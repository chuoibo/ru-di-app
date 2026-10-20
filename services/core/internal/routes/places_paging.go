package routes

import (
	"context"
	"strconv"
	"strings"

	"mobile/services/core/internal/domain/promptsafety"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/rag"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/treejson"
)

// Paging of GET /places. Both parameters are read off the raw query string:
// the route's contract comes from Python, which has no paging, so
// call.Values never carries them. A request with neither keeps the whole
// list and the body Python sends; one with either gets a page plus `total`
// and `next_offset`.
const (
	defaultPageLimit = 30
	maxPageLimit     = 100
)

type paging struct {
	paged  bool
	limit  int
	offset int
}

type window struct{ start, end int }

func pagingOf(call *endpoint.Call) (paging, error) {
	if call.Request == nil || call.Request.URL == nil {
		return paging{}, nil
	}
	values := call.Request.URL.Query()
	rawLimit, hasLimit := values["limit"]
	rawOffset, hasOffset := values["offset"]
	if !hasLimit && !hasOffset {
		return paging{}, nil
	}
	out := paging{paged: true, limit: defaultPageLimit}
	refuse := func() (paging, error) {
		return paging{}, endpoint.Refuse(422, "invalid_paging",
			"limit phải là số nguyên từ 1 đến "+strconv.Itoa(maxPageLimit)+", offset phải là số nguyên không âm.")
	}
	if hasLimit {
		n, err := strconv.Atoi(strings.TrimSpace(rawLimit[0]))
		if err != nil || n < 1 {
			return refuse()
		}
		out.limit = min(n, maxPageLimit)
	}
	if hasOffset {
		n, err := strconv.Atoi(strings.TrimSpace(rawOffset[0]))
		if err != nil || n < 0 {
			return refuse()
		}
		out.offset = n
	}
	return out, nil
}

// cut is the window of a list of total rows and the offset of the next page,
// nil on the last one. Without paging it is the whole list.
func (p paging) cut(total int) (window, *int) {
	if !p.paged {
		return window{0, total}, nil
	}
	start := min(p.offset, total)
	end := min(start+p.limit, total)
	if end >= total {
		return window{start, end}, nil
	}
	return window{start, end}, &end
}

// includes is the comma-separated `?include=` of a request. The names are
// Go-only additions to a body Python also sends (see listDestinations).
func includes(call *endpoint.Call) map[string]bool {
	out := map[string]bool{}
	if call.Request == nil || call.Request.URL == nil {
		return out
	}
	for _, value := range call.Request.URL.Query()["include"] {
		for _, name := range strings.Split(value, ",") {
			if name = strings.TrimSpace(name); name != "" {
				out[name] = true
			}
		}
	}
	return out
}

// reasonsRequested is the place ids of `?reasons_for=a,b,c` and whether the
// parameter was sent at all. At most maxReasonRows ids are kept, duplicates
// dropped, in the order asked.
func reasonsRequested(call *endpoint.Call) ([]string, bool) {
	if call.Request == nil || call.Request.URL == nil {
		return nil, false
	}
	values, sent := call.Request.URL.Query()["reasons_for"]
	if !sent {
		return nil, false
	}
	var ids []string
	seen := map[string]bool{}
	for _, value := range values {
		for _, id := range strings.Split(value, ",") {
			if id = strings.TrimSpace(id); id != "" && !seen[id] && len(ids) < maxReasonRows {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids, true
}

// serveReasons answers `GET /places?reasons_for=`: the model's one sentence
// for each asked place, for the caller's taste. It is the only path that
// waits for the model, and it holds no database transaction while it waits.
// A place with no answer is simply absent -- the card already carries the
// server's own sentence.
func serveReasons(ctx context.Context, call *endpoint.Call, ids []string) (endpoint.Reply, error) {
	reasons := pyjson.NewOrderedMap()
	reply := func() endpoint.Reply {
		body := pyjson.NewOrderedMap()
		body.Set("reasons", reasons)
		return endpoint.Reply{Body: body}
	}
	if len(ids) == 0 {
		return reply(), nil
	}
	contextID, err := optionalUUIDParam(call, "context_id")
	if err != nil {
		return endpoint.Reply{}, err
	}
	store, err := groupStore(ctx, call)
	if err != nil {
		return endpoint.Reply{}, err
	}
	group, err := tasteProfile(ctx, store, call, contextID)
	if err != nil {
		return endpoint.Reply{}, err
	}
	// No known taste means nobody to write a reason for.
	if !group.Known() {
		return reply(), nil
	}
	enabled := catalogue.configuredTTL() > 0
	found := map[string]reasonPair{}
	var missing []string
	for _, id := range ids {
		if pair, ok := catalogue.reasons.get(reasonKey(group, id), enabled); ok {
			found[id] = pair
		} else {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 && call.AI.CoMay() {
		rows, err := store.PlacesByIDs(ctx, missing)
		if err != nil {
			return endpoint.Reply{}, err
		}
		byID := make(map[string]repo.Place, len(rows))
		for _, row := range rows {
			byID[row.ID] = row
		}
		ordered := make([]repo.Place, 0, len(missing))
		for _, id := range missing {
			if row, ok := byID[id]; ok {
				ordered = append(ordered, row)
			}
		}
		cards, err := withPhotos(ctx, store, ordered)
		if err != nil {
			return endpoint.Reply{}, err
		}
		safe := treejson.MapsFrom(promptsafety.Filter(treejson.MapsTo(cards)))
		// Everything the model is shown has been read. Give the connection
		// back before the wait: a model call can take tens of seconds, and a
		// transaction held through it pins a pooled connection.
		if err := call.Unit.Release(ctx); err != nil {
			return endpoint.Reply{}, err
		}
		if len(safe) > 0 {
			asked := make([]string, len(safe))
			for i, card := range safe {
				asked[i] = placeIDOf(card)
			}
			flightKey := group.CacheKey() + "\x00" + strings.Join(asked, ",")
			value, _, _ := catalogue.reasons.flight.Do(flightKey, func() (any, error) {
				written := fetchReasons(ctx, call, safe, group)
				for id, pair := range written {
					catalogue.reasons.put(reasonKey(group, id), pair, enabled)
				}
				return written, nil
			})
			for id, pair := range value.(map[string]reasonPair) {
				found[id] = pair
			}
		}
	}
	for _, id := range ids {
		pair, ok := found[id]
		if !ok || pair.reason == nil || pair.verdict == nil {
			continue
		}
		entry := pyjson.NewOrderedMap()
		entry.Set("reason", pyjson.String(*pair.reason))
		entry.Set("verdict", pyjson.String(*pair.verdict))
		entry.Set("source", pyjson.String("ai"))
		reasons.Set(id, entry)
	}
	return reply(), nil
}

func placeIDOf(card *pyjson.OrderedMap) string {
	if v, ok := card.Get("id"); ok {
		if s, ok := v.(pyjson.String); ok {
			return string(s)
		}
	}
	return ""
}

// destinationCovers picks the cover photograph of each destination.
type destinationCovers struct {
	byDestination map[string]repo.PlacePhoto
	destinations  []rag.DiemDen
}

func coversFor(ctx context.Context, store repo.Repository, rows []repo.Destination) (*destinationCovers, error) {
	byDestination, err := catalogue.destinationCovers(ctx, store, false)
	if err != nil {
		return nil, err
	}
	return (&destinationCovers{byDestination: byDestination}).withRows(rows), nil
}

func (c *destinationCovers) withRows(rows []repo.Destination) *destinationCovers {
	c.destinations = rag.DiemDenTuRepo(rows)
	return c
}

// coverOf is the destination's own cover; a curated destination with none
// (d-tphcm beside the province d-tinh-79) takes the cover of the province it
// lies in, the relation rag/phamvi.go uses to merge their places.
func (c *destinationCovers) coverOf(id string) (repo.PlacePhoto, bool) {
	if photo, ok := c.byDestination[id]; ok {
		return photo, true
	}
	if rag.LaTinh(id) {
		return repo.PlacePhoto{}, false
	}
	if province := rag.TinhCua(c.destinations, id); province != "" {
		photo, ok := c.byDestination[province]
		return photo, ok
	}
	return repo.PlacePhoto{}, false
}

// decorate adds the three cover fields to a wired destination; nil-safe, so a
// caller that did not ask for covers passes a nil receiver.
func (c *destinationCovers) decorate(out *pyjson.OrderedMap, id string) {
	if c == nil {
		return
	}
	photo, ok := c.coverOf(id)
	if !ok {
		out.Set("cover_photo_url", pyjson.Null{})
		out.Set("cover_photo_author", pyjson.Null{})
		out.Set("cover_photo_license", pyjson.Null{})
		return
	}
	out.Set("cover_photo_url", pyjson.String("/places/"+photo.PlaceID+"/photos/"+photo.ID))
	out.Set("cover_photo_author", textOrNull(photo.Author))
	out.Set("cover_photo_license", textOrNull(photo.License))
}
