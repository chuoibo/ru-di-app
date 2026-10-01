package routes

import (
	"context"
	"errors"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"mobile/services/core/internal/aiharness/timquan"
	"mobile/services/core/internal/domain/areas"
	"mobile/services/core/internal/domain/catalog"
	"mobile/services/core/internal/domain/promptsafety"
	"mobile/services/core/internal/domain/taste"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
	"mobile/services/core/internal/treejson"
	guestweb "mobile/services/core/internal/web/guest"
)

const (
	nearLimitKM      = 60.0
	maxReasonRows    = 12
	groupPhotoLimit  = 20
	publicPhotoCache = "public, max-age=86400"
)

// nearerDestination is the sort key Python spells `(distance, id)`. The id half
// decides only a bit-exact tie, which needs two destinations at one point, so
// the parity corpus cannot reach it: `GET-destinations.yaml` stays EQUAL when
// this half is deleted. It is kept because production may hold such a pair, and
// `places_wai_test.go` is what proves it still works.
func nearerDestination(iKM float64, iID string, jKM float64, jID string) bool {
	if iKM != jKM {
		return iKM < jKM
	}
	return iID < jID
}

func listDestinations() Route {
	return Route{ID: "GET /destinations", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		lat, err := optionalFloatParam(call, "lat")
		if err != nil {
			return endpoint.Reply{}, err
		}
		lng, err := optionalFloatParam(call, "lng")
		if err != nil {
			return endpoint.Reply{}, err
		}
		if (lat == nil) != (lng == nil) {
			return endpoint.Reply{}, endpoint.Refuse(422, "coordinates_incomplete", "Cần cả lat và lng, hoặc không gửi gì.")
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		rows, err := store.ListDestinations(ctx)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if lat == nil {
			list := pyjson.List{}
			for _, row := range rows {
				list = append(list, wireDestination(row, nil))
			}
			body := pyjson.NewOrderedMap()
			body.Set("destinations", list)
			body.Set("nearest", pyjson.Null{})
			return endpoint.Reply{Body: body}, nil
		}
		type ranked struct {
			km  float64
			row repo.Destination
		}
		pairs := make([]ranked, 0, len(rows))
		for _, row := range rows {
			km, err := areas.HaversineKm(*lat, *lng, row.Lat, row.Lng)
			if err != nil {
				return endpoint.Reply{}, err
			}
			pairs = append(pairs, ranked{km: km, row: row})
		}
		sort.Slice(pairs, func(i, j int) bool {
			return nearerDestination(pairs[i].km, pairs[i].row.ID, pairs[j].km, pairs[j].row.ID)
		})
		list := pyjson.List{}
		for _, pair := range pairs {
			km := pythonRound1(pair.km)
			list = append(list, wireDestination(pair.row, &km))
		}
		var nearest pyjson.Value = pyjson.Null{}
		if len(pairs) > 0 && pairs[0].km <= nearLimitKM {
			km := pythonRound1(pairs[0].km)
			nearest = wireDestination(pairs[0].row, &km)
		}
		body := pyjson.NewOrderedMap()
		body.Set("destinations", list)
		body.Set("nearest", nearest)
		return endpoint.Reply{Body: body}, nil
	}}
}

func listPlacesWAI() Route {
	return Route{ID: "GET /places", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := optionalUUIDParam(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		category, err := optionalStringParam(call, "category")
		if err != nil {
			return endpoint.Reply{}, err
		}
		query, err := optionalStringParam(call, "q")
		if err != nil {
			return endpoint.Reply{}, err
		}
		destinationID, err := optionalStringParam(call, "destination")
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
		diemDen, err := service.DestinationOrDefault(ctx, store, destinationID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if diemDen == nil {
			return endpoint.Reply{}, endpoint.Refuse(404, "destination_not_found", "Không có điểm đến nào với mã này.")
		}
		// No known taste means no personal ranking and no reasons: the body is
		// the same for every such caller, so a cached one is served as is.
		replyKey := ""
		if !group.Known() {
			replyKey = diemDen.ID + "\x00" + deref(category) + "\x00" + deref(query)
			if body := catalogue.reply(replyKey); body != nil {
				return endpoint.Reply{Body: body}, nil
			}
		}
		snap, err := catalogue.load(ctx, store, diemDen.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		rows := snap.rows
		selected := make([]repo.Place, 0, len(rows))
		q := ""
		if query != nil {
			q = *query
		}
		for _, row := range rows {
			if category != nil && row.Category != *category {
				continue
			}
			if !placeMatches(q, row) {
				continue
			}
			selected = append(selected, row)
		}
		cards := cardsWithPhotos(selected, snap.covers, snap.counts)
		written := map[string]reasonPair{}
		if group.Known() {
			safe := treejson.MapsFrom(promptsafety.Filter(treejson.MapsTo(cards)))
			sort.SliceStable(safe, func(i, j int) bool {
				si, sj := service.ScoreOrZero(safe[i], group), service.ScoreOrZero(safe[j], group)
				if si != sj {
					return si > sj
				}
				return service.PlaceID(safe[i]) < service.PlaceID(safe[j])
			})
			if len(safe) > maxReasonRows {
				safe = safe[:maxReasonRows]
			}
			// The list rows were read without description, reviews and
			// activities; the reasons prompt quotes whole cards, so its few
			// rows are read in full and rebuilt exactly as before.
			safe, err = fullCards(ctx, store, safe, snap)
			if err != nil {
				return endpoint.Reply{}, err
			}
			written = fetchReasons(ctx, call, safe, group)
		}
		out := make([]*pyjson.OrderedMap, 0, len(cards))
		for _, card := range cards {
			id := service.PlaceID(card)
			pair := written[id]
			wired, err := wirePlaceCard(card, pair.reason, pair.verdict, group)
			if err != nil {
				return endpoint.Reply{}, err
			}
			out = append(out, wired)
		}
		sort.SliceStable(out, func(i, j int) bool {
			return placeSortLess(out[i], out[j])
		})
		places := pyjson.List{}
		for _, card := range out {
			places = append(places, card)
		}
		body := pyjson.NewOrderedMap()
		body.Set("places", places)
		body.Set("categories", wireCategories())
		body.Set("group", wireGroupSummary(group))
		body.Set("destination", wireDestination(*diemDen, nil))
		if replyKey != "" {
			catalogue.keepReply(replyKey, body, snap)
		}
		return endpoint.Reply{Body: body}, nil
	}}
}

func deref(s *string) string {
	if s == nil {
		return "\x01" // absent, distinct from an empty value
	}
	return *s
}

func listPlacePhotos() Route {
	return Route{ID: "GET /places/{place_id}/photos", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		placeID, err := stringParam(call, "place_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		place, err := store.GetPlace(ctx, placeID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if place == nil {
			return endpoint.Reply{}, endpoint.Refuse(404, "place_not_found", "Không có địa điểm này trong danh mục.")
		}
		photos, err := store.ListPlacePhotos(ctx, placeID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		list := pyjson.List{}
		for _, photo := range photos {
			list = append(list, wirePlacePhoto(placeID, photo))
		}
		body := pyjson.NewOrderedMap()
		body.Set("place_id", pyjson.String(placeID))
		body.Set("photos", list)
		return endpoint.Reply{Body: body}, nil
	}}
}

func listPlaceGroupPhotos() Route {
	return Route{ID: "GET /places/{place_id}/group-photos", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		placeID, err := stringParam(call, "place_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireFacts(call, "view_own_contexts", map[string]bool{"is_self": true}); err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		rows, err := store.GroupPhotosAtPlace(ctx, placeID, call.Actor.ID, groupPhotoLimit)
		if err != nil {
			return endpoint.Reply{}, err
		}
		list := pyjson.List{}
		for _, row := range rows {
			list = append(list, wireMemory(row))
		}
		body := pyjson.NewOrderedMap()
		body.Set("place_id", pyjson.String(placeID))
		body.Set("photos", list)
		return endpoint.Reply{Body: body}, nil
	}}
}

func readPlacePhoto() Route {
	return Route{ID: "GET /places/{place_id}/photos/{photo_id}", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		placeID, err := stringParam(call, "place_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		photoID, err := pathUUID(call, "photo_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		record, err := store.GetPlacePhoto(ctx, placeID, photoID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if record == nil {
			return endpoint.Reply{}, endpoint.Refuse(404, "photo_not_found", "Không có ảnh này.")
		}
		content, err := call.Photos.Read(record.StorageKey)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return endpoint.Reply{}, endpoint.Refuse(404, "photo_not_found", "Không có ảnh này.")
			}
			return endpoint.Reply{}, err
		}
		if len(content) == 0 {
			return endpoint.Reply{}, endpoint.Refuse(404, "photo_not_found", "Không có ảnh này.")
		}
		return endpoint.Reply{Raw: &guestweb.Response{
			Status: 200,
			Headers: [][2]string{
				{"cache-control", publicPhotoCache},
				{"content-length", strconv.Itoa(len(content))},
				{"content-type", record.ContentType},
			},
			Body: content,
		}}, nil
	}}
}

func getPlaceWAI() Route {
	return Route{ID: "GET /places/{place_id}", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		placeID, err := stringParam(call, "place_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		place, err := store.GetPlace(ctx, placeID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if place == nil {
			return endpoint.Reply{}, endpoint.Refuse(404, "place_not_found", "Không tìm thấy địa điểm này.")
		}
		cards, err := withPhotos(ctx, store, []repo.Place{*place})
		if err != nil {
			return endpoint.Reply{}, err
		}
		group, err := tasteProfile(ctx, store, call, nil)
		if err != nil {
			return endpoint.Reply{}, err
		}
		written := map[string]reasonPair{}
		if group.Known() {
			written = fetchReasons(ctx, call, cards, group)
		}
		pair := written[placeID]
		card, err := wirePlaceCard(cards[0], pair.reason, pair.verdict, group)
		if err != nil {
			return endpoint.Reply{}, err
		}
		card.Set("description", textOrNull(place.Description))
		card.Set("activities", service.JSONListOrEmpty(place.Activities))
		card.Set("reviews", service.WireReviews(place.Reviews))
		count := int64(0)
		if v, ok := cards[0].Get("photo_count"); ok {
			if n, ok := v.(pyjson.Int); ok {
				if parsed, ok := n.Int64(); ok {
					count = parsed
				}
			}
		}
		card.Set("photos_available", pyjson.Bool(count > 0))
		return endpoint.Reply{Body: card}, nil
	}}
}

func searchPlacesWAI() Route {
	return Route{ID: "POST /places/search", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		if err := spendActorWindow(call, call.Limits.SearchLimiter); err != nil {
			return endpoint.Reply{}, err
		}
		request, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		query, err := stringField(request, "query")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		group, err := tasteProfile(ctx, store, call, nil)
		if err != nil {
			return endpoint.Reply{}, err
		}
		unavailable := func() endpoint.Reply {
			body := pyjson.NewOrderedMap()
			body.Set("query", pyjson.String(query))
			body.Set("understood", pyjson.Null{})
			body.Set("places", pyjson.List{})
			body.Set("source", pyjson.String("none"))
			body.Set("group", wireGroupSummary(group))
			return endpoint.Reply{Body: body}
		}
		// `?destination=` narrows the search to one destination; without it
		// the whole catalogue is searched, as before. Read off the raw query
		// string: the route's contract (from Python, which has no such
		// parameter) does not declare it, so call.Values never carries it --
		// reading it there answered 500. An unknown destination is ignored
		// rather than refused, so this Go-only narrowing can never turn an
		// answer Python gives into a different status.
		filter := repo.PlaceFilter{}
		if wanted := destinationQuery(call); wanted != "" {
			diemDen, err := service.DestinationOrDefault(ctx, store, &wanted)
			if err != nil {
				return endpoint.Reply{}, err
			}
			if diemDen != nil && diemDen.ID == wanted {
				filter.DestinationID = &diemDen.ID
			}
		}
		// Light rows for the whole (or the held) catalogue, then full rows
		// only for the few searchCandidates keeps: at most rag.ToiDaNgan, the
		// most any search hands the model (design 04 §7).
		// places_search_shortlist_postgres_test.go reads the prompt the model
		// receives; parity has nothing to compare since ADR-0051.
		slim, err := store.ListPlaceCards(ctx, filter)
		if err != nil {
			return endpoint.Reply{}, err
		}
		rows, err := fullRowsInOrder(ctx, store, searchCandidates(slim, query, group))
		if err != nil {
			return endpoint.Reply{}, err
		}
		cards, err := withPhotos(ctx, store, rows)
		if err != nil {
			return endpoint.Reply{}, err
		}
		shortlist := []*pyjson.OrderedMap{}
		for _, item := range modelShortlist(cards) {
			shortlist = append(shortlist, item.(*pyjson.OrderedMap))
		}
		// An empty catalogue would ask the model to pick from nothing, which
		// it can only answer by inventing: refused before the call.
		if !call.AI.CoMay() || len(shortlist) == 0 {
			return unavailable(), nil
		}
		prompt, err := timquan.PromptTimQuan(query, shortlist, group)
		if err != nil {
			return unavailable(), nil
		}
		raw, err := timquan.Tim(ctx, call.AI.Luot(1), prompt)
		if err != nil {
			return unavailable(), nil
		}
		understood, results, err := timquan.Ground(raw, shortlist)
		if err != nil {
			return unavailable(), nil
		}
		if !group.Known() {
			group = taste.Profile{Basis: taste.BasisPerson, Interests: []string{}, People: 1}
			if n := service.IntPtrOf(understood, "budget_per_person_vnd"); n != nil {
				group.BudgetPerPersonVND = n
			}
			if n := service.IntPtrOf(understood, "group_size"); n != nil {
				size := *n
				group.Size = &size
			}
		}
		places := pyjson.List{}
		for _, result := range results {
			reason, verdict := result.Reason, result.Verdict
			// Two per-row gates: a sentence quoting a figure the model was
			// not shown, or repeating the caller's own sentence, is dropped
			// with its verdict; the real place stays, under the server's
			// own words.
			if reason != nil {
				stray, err := timquan.UngroundedNumbers(*reason, result.Place, group)
				if err != nil {
					return unavailable(), nil
				}
				if len(stray) > 0 || timquan.EchoesTheQuery(reason, query) {
					reason, verdict = nil, nil
				}
			}
			card, err := wirePlaceCard(result.Place, reason, verdict, group)
			if err != nil {
				return endpoint.Reply{}, err
			}
			places = append(places, card)
		}
		body := pyjson.NewOrderedMap()
		body.Set("query", pyjson.String(query))
		body.Set("understood", understood)
		body.Set("places", places)
		body.Set("source", pyjson.String("ai"))
		body.Set("group", wireGroupSummary(group))
		return endpoint.Reply{Body: body}, nil
	}}
}

type reasonPair struct{ reason, verdict *string }

// modelShortlist is what the search model may read of the shortlist: the
// rows promptsafety.Filter keeps (the oracle's rule), each then cut by
// promptsafety.SafeDeep -- a row it drops is gone, a quarantined review,
// activity or description is emptied -- in the shortlist's order.
func modelShortlist(cards []*pyjson.OrderedMap) pyjson.List {
	list := pyjson.List{}
	for _, card := range promptsafety.Filter(treejson.MapsTo(cards)) {
		deep, report := promptsafety.SafeDeep(card)
		if report.Bo {
			continue
		}
		list = append(list, treejson.MapFrom(deep))
	}
	return list
}

func wireDestination(row repo.Destination, km *float64) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(row.ID))
	out.Set("name", pyjson.String(row.Name))
	out.Set("province", textOrNull(row.Province))
	out.Set("blurb", textOrNull(row.Blurb))
	out.Set("lat", pyjson.Float(row.Lat))
	out.Set("lng", pyjson.Float(row.Lng))
	if km == nil {
		out.Set("distance_km", pyjson.Null{})
	} else {
		out.Set("distance_km", pyjson.Float(*km))
	}
	return out
}

func wireCategories() pyjson.List {
	list := pyjson.List{}
	for _, row := range catalog.Categories {
		entry := pyjson.NewOrderedMap()
		entry.Set("id", pyjson.String(row.ID))
		entry.Set("label", pyjson.String(row.Label))
		list = append(list, entry)
	}
	return list
}

func wireGroupSummary(group taste.Profile) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("basis", pyjson.String(string(group.Basis)))
	if group.Size == nil {
		out.Set("size", pyjson.Null{})
	} else {
		out.Set("size", pyjson.NewInt(*group.Size))
	}
	if group.BudgetPerPersonVND == nil {
		out.Set("budget_per_person_vnd", pyjson.Null{})
	} else {
		out.Set("budget_per_person_vnd", pyjson.NewInt(*group.BudgetPerPersonVND))
	}
	interests := pyjson.List{}
	for _, tag := range group.Interests {
		interests = append(interests, pyjson.String(tag))
	}
	out.Set("interests", interests)
	out.Set("people", pyjson.NewInt(group.People))
	out.Set("people_answered", pyjson.NewInt(group.PeopleAnswered))
	uncovered := pyjson.List{}
	for _, tag := range taste.Uncovered(group.Interests) {
		uncovered = append(uncovered, pyjson.String(tag))
	}
	out.Set("uncovered_interests", uncovered)
	return out
}

func tasteProfile(ctx context.Context, store repo.Repository, call *endpoint.Call, contextID *string) (taste.Profile, error) {
	if contextID != nil {
		if call.Actor == nil {
			return taste.Profile{}, endpoint.Refuse(401, "authentication_required", "Cần đăng nhập để đọc gu nhóm.")
		}
		if err := requireGroupMember(ctx, call, store, "view_group_preference_profile", *contextID); err != nil {
			return taste.Profile{}, err
		}
		return service.GroupTaste(ctx, store, *contextID, time.Now().UTC())
	}
	if call.Actor == nil {
		return taste.Unknown(), nil
	}
	person, err := store.GetPerson(ctx, call.Actor.ID)
	if err != nil {
		return taste.Profile{}, err
	}
	interests, err := store.ListPersonInterests(ctx, call.Actor.ID)
	if err != nil {
		return taste.Profile{}, err
	}
	var band *string
	if person != nil {
		band = person.BudgetBand
	}
	return taste.ProfileForPerson(interests, band), nil
}

func placeMatches(text string, place repo.Place) bool {
	needle := strings.ToLower(strings.TrimSpace(text))
	if needle == "" {
		return true
	}
	address := ""
	if place.Address != nil {
		address = *place.Address
	}
	parts := append([]string{place.Name, address}, place.Kinds...)
	parts = append(parts, place.Traits...)
	return strings.Contains(strings.ToLower(strings.Join(parts, " ")), needle)
}

func withPhotos(ctx context.Context, store repo.Repository, rows []repo.Place) ([]*pyjson.OrderedMap, error) {
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	covers, counts, err := store.PhotoCoversAndCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	return cardsWithPhotos(rows, covers, counts), nil
}

// fullCards replaces each card with one built from the place's full row, in
// the same order and through the same conversion the list applied, so what a
// prompt sees is byte-for-byte what it saw when the list read every column.
func fullCards(ctx context.Context, store repo.Repository, cards []*pyjson.OrderedMap,
	snap *catalogueSnapshot) ([]*pyjson.OrderedMap, error) {
	ids := make([]string, len(cards))
	for i, card := range cards {
		ids[i] = service.PlaceID(card)
	}
	rows, err := store.PlacesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]repo.Place, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	ordered := make([]repo.Place, 0, len(ids))
	for _, id := range ids {
		if row, ok := byID[id]; ok {
			ordered = append(ordered, row)
		}
	}
	return treejson.MapsFrom(treejson.MapsTo(cardsWithPhotos(ordered, snap.covers, snap.counts))), nil
}

// fullRowsInOrder reads the given places in full, keeping their order; a row
// deleted since the slim read is dropped.
func fullRowsInOrder(ctx context.Context, store repo.Repository, slim []repo.Place) ([]repo.Place, error) {
	ids := make([]string, len(slim))
	for i, row := range slim {
		ids[i] = row.ID
	}
	rows, err := store.PlacesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]repo.Place, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	out := make([]repo.Place, 0, len(ids))
	for _, id := range ids {
		if row, ok := byID[id]; ok {
			out = append(out, row)
		}
	}
	return out, nil
}

// cardsWithPhotos builds list cards from rows and a photo summary already read.
func cardsWithPhotos(rows []repo.Place, covers map[string]repo.PlacePhoto, counts map[string]int64) []*pyjson.OrderedMap {
	out := make([]*pyjson.OrderedMap, 0, len(rows))
	for _, row := range rows {
		card := service.PlaceRow(row)
		count := counts[row.ID]
		card.Set("photo_count", pyjson.NewInt(count))
		if cover, ok := covers[row.ID]; ok {
			url := "/places/" + row.ID + "/photos/" + cover.ID
			card.Set("photo_url", pyjson.String(url))
			card.Set("photo_author", textOrNull(cover.Author))
			card.Set("photo_license", textOrNull(cover.License))
		} else {
			card.Set("photo_url", pyjson.Null{})
			card.Set("photo_author", pyjson.Null{})
			card.Set("photo_license", pyjson.Null{})
		}
		out = append(out, card)
	}
	return out
}

func wirePlaceCard(place *pyjson.OrderedMap, reason, verdict *string, group taste.Profile) (*pyjson.OrderedMap, error) {
	if reason == nil || verdict == nil {
		reason, verdict = nil, nil
	}
	score, factors, err := service.ScoreCard(place, group)
	if err != nil {
		return nil, err
	}
	out := pyjson.NewOrderedMap()
	for _, key := range []string{
		"id", "name", "category", "kinds", "rating", "rating_count", "distance_km",
		"price_min_vnd", "price_max_vnd", "address", "open_now", "open_hours",
		"travel_minutes", "photo_count", "photo_url", "photo_author", "photo_license",
		"traits", "group_fit", "flag", "lat", "lng", "geo_precision", "source", "license",
	} {
		if value, ok := place.Get(key); ok {
			out.Set(key, value)
		} else if key == "photo_url" || key == "photo_author" || key == "photo_license" {
			out.Set(key, pyjson.Null{})
		}
	}
	if score == nil {
		out.Set("match", pyjson.Null{})
		return out, nil
	}
	match := pyjson.NewOrderedMap()
	match.Set("score", pyjson.NewBigInt(score))
	if reason == nil {
		match.Set("reason", pyjson.String(fallbackReason(place)))
		match.Set("source", pyjson.String("none"))
		match.Set("verdict", pyjson.Null{})
	} else {
		match.Set("reason", pyjson.String(*reason))
		match.Set("source", pyjson.String("ai"))
		match.Set("verdict", pyjson.String(*verdict))
	}
	lines := pyjson.List{}
	for _, factor := range factors {
		entry := pyjson.NewOrderedMap()
		entry.Set("label", pyjson.String(factor.Label))
		entry.Set("detail", pyjson.String(factor.Detail))
		lines = append(lines, entry)
	}
	match.Set("factors", lines)
	out.Set("match", match)
	return out, nil
}

func fallbackReason(place *pyjson.OrderedMap) string {
	var parts []string
	low := service.IntPtrOf(place, "price_min_vnd")
	high := service.IntPtrOf(place, "price_max_vnd")
	if low != nil && high != nil {
		lowK, highK := *low/1000, *high/1000
		band := strconv.FormatInt(lowK, 10) + "k"
		if lowK != highK {
			band = strconv.FormatInt(lowK, 10) + "–" + strconv.FormatInt(highK, 10) + "k"
		}
		parts = append(parts, "Khoảng "+band+"/người")
	}
	if km := service.FloatPtrOf(place, "distance_km"); km != nil {
		parts = append(parts, "cách "+strconv.FormatFloat(*km, 'f', -1, 64)+"km")
	}
	head := "Chưa có giá và khoảng cách cho chỗ này. "
	if len(parts) > 0 {
		head = strings.Join(parts, ", ") + ". "
	}
	return head + "Điểm dưới đây do máy tính từ ngân sách, sở thích và khoảng cách đã biết; chưa có nhận xét của AI cho chỗ này."
}

func placeSortLess(a, b *pyjson.OrderedMap) bool {
	openA, openB := openNowRank(a), openNowRank(b)
	if openA != openB {
		return openA < openB
	}
	scoreA, scoreB := matchScore(a), matchScore(b)
	if scoreA != scoreB {
		return scoreA > scoreB
	}
	ratingA, ratingB := ratingOf(a), ratingOf(b)
	if ratingA != ratingB {
		return ratingA > ratingB
	}
	return service.PlaceID(a) < service.PlaceID(b)
}

func openNowRank(place *pyjson.OrderedMap) int {
	value, _ := place.Get("open_now")
	flag, ok := value.(pyjson.Bool)
	if ok && bool(flag) {
		return 0
	}
	return 1
}

func matchScore(place *pyjson.OrderedMap) int64 {
	value, _ := place.Get("match")
	obj, ok := value.(*pyjson.OrderedMap)
	if !ok {
		return -1
	}
	score, _ := obj.Get("score")
	n, ok := score.(pyjson.Int)
	if !ok {
		return -1
	}
	return n.Big().Int64()
}

func ratingOf(place *pyjson.OrderedMap) float64 {
	value, _ := place.Get("rating")
	switch v := value.(type) {
	case pyjson.Float:
		return float64(v)
	case pyjson.Int:
		n, _ := v.Big().Float64()
		return n
	}
	return -1
}

func wirePlacePhoto(placeID string, photo repo.PlacePhoto) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(photo.ID))
	out.Set("url", pyjson.String("/places/"+placeID+"/photos/"+photo.ID))
	out.Set("author", textOrNull(photo.Author))
	out.Set("license", textOrNull(photo.License))
	out.Set("source_url", pyjson.String(photo.SourceURL))
	out.Set("title", textOrNull(photo.Title))
	out.Set("width", pyjson.NewInt(photo.Width))
	out.Set("height", pyjson.NewInt(photo.Height))
	return out
}

// fetchReasons asks the model, in one call, whether each place fits the
// group (aiharness/timquan). Every way there is no answer -- no model, a
// failed call, an unreadable reply -- is an empty map: the cards are then
// served with the server's own sentence and no AI label.
func fetchReasons(ctx context.Context, call *endpoint.Call, places []*pyjson.OrderedMap, group taste.Profile) map[string]reasonPair {
	out := map[string]reasonPair{}
	if len(places) == 0 || !call.AI.CoMay() {
		return out
	}
	prompt, err := timquan.PromptLyDo(places, group)
	if err != nil {
		return out
	}
	text, err := timquan.VietLyDo(ctx, call.AI.Luot(1), prompt)
	if err != nil {
		return out
	}
	written, err := timquan.ParseReasons(text, places, group)
	if err != nil {
		return out
	}
	for id, r := range written {
		reason, verdict := r.Reason, r.Verdict
		out[id] = reasonPair{reason: &reason, verdict: &verdict}
	}
	return out
}

// destinationQuery is the raw `?destination=` of a request, "" when absent.
func destinationQuery(call *endpoint.Call) string {
	if call.Request == nil || call.Request.URL == nil {
		return ""
	}
	return strings.TrimSpace(call.Request.URL.Query().Get("destination"))
}
