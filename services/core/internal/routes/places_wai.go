package routes

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"mobile/services/core/internal/brain"
	"mobile/services/core/internal/domain/areas"
	"mobile/services/core/internal/domain/catalog"
	"mobile/services/core/internal/domain/promptsafety"
	"mobile/services/core/internal/domain/scoring"
	"mobile/services/core/internal/domain/taste"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/treejson"
	guestweb "mobile/services/core/internal/web/guest"
)

const (
	nearLimitKM      = 60.0
	maxReasonRows    = 12
	maxModelPlaces   = 40
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
		diemDen, err := destinationOrDefault(ctx, store, destinationID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if diemDen == nil {
			return endpoint.Reply{}, endpoint.Refuse(404, "destination_not_found", "Không có điểm đến nào với mã này.")
		}
		filter := repo.PlaceFilter{DestinationID: &diemDen.ID}
		rows, err := store.ListPlaces(ctx, filter)
		if err != nil {
			return endpoint.Reply{}, err
		}
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
		cards, err := withPhotos(ctx, store, selected)
		if err != nil {
			return endpoint.Reply{}, err
		}
		written := map[string]reasonPair{}
		if group.Known() {
			safe := treejson.MapsFrom(promptsafety.Filter(treejson.MapsTo(cards)))
			sort.SliceStable(safe, func(i, j int) bool {
				si, sj := scoreOrZero(safe[i], group), scoreOrZero(safe[j], group)
				if si != sj {
					return si > sj
				}
				return placeID(safe[i]) < placeID(safe[j])
			})
			if len(safe) > maxReasonRows {
				safe = safe[:maxReasonRows]
			}
			written = fetchReasons(safe, group)
		}
		out := make([]*pyjson.OrderedMap, 0, len(cards))
		for _, card := range cards {
			id := placeID(card)
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
		return endpoint.Reply{Body: body}, nil
	}}
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
			written = fetchReasons(cards, group)
		}
		pair := written[placeID]
		card, err := wirePlaceCard(cards[0], pair.reason, pair.verdict, group)
		if err != nil {
			return endpoint.Reply{}, err
		}
		card.Set("description", textOrNull(place.Description))
		card.Set("activities", jsonListOrEmpty(place.Activities))
		card.Set("reviews", wireReviews(place.Reviews))
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
		rows, err := store.ListPlaces(ctx, repo.PlaceFilter{})
		if err != nil {
			return endpoint.Reply{}, err
		}
		cards, err := withPhotos(ctx, store, rows)
		if err != nil {
			return endpoint.Reply{}, err
		}
		safe := treejson.MapsFrom(promptsafety.Filter(treejson.MapsTo(cards)))
		payload := pyjson.NewOrderedMap()
		payload.Set("query", pyjson.String(query))
		list := pyjson.List{}
		for _, card := range safe {
			list = append(list, card)
		}
		payload.Set("catalogue", list)
		payload.Set("group", wireTaste(group))
		client := brain.Configured()
		raw, err := client.PostJSON("place-search", payload)
		if err != nil {
			return unavailable(), nil
		}
		obj, err := brain.AsObject(raw)
		if err != nil {
			return unavailable(), nil
		}
		if _, found := obj.Get("source"); found {
			return unavailable(), nil
		}
		understoodValue, _ := obj.Get("understood")
		understood, ok := understoodValue.(*pyjson.OrderedMap)
		if !ok {
			return unavailable(), nil
		}
		rawResults, _ := obj.Get("results")
		results, _ := rawResults.(pyjson.List)
		if !group.Known() {
			group = taste.Profile{Basis: taste.BasisPerson, Interests: []string{}, People: 1}
			if n := intPtrOf(understood, "budget_per_person_vnd"); n != nil {
				group.BudgetPerPersonVND = n
			}
			if n := intPtrOf(understood, "group_size"); n != nil {
				size := *n
				group.Size = &size
			}
		}
		places := pyjson.List{}
		for _, item := range results {
			row, ok := item.(*pyjson.OrderedMap)
			if !ok {
				continue
			}
			placeValue, _ := row.Get("place")
			place, ok := placeValue.(*pyjson.OrderedMap)
			if !ok {
				continue
			}
			reasonValue, _ := row.Get("reason")
			verdictValue, _ := row.Get("verdict")
			var reason, verdict *string
			if text, ok := reasonValue.(pyjson.String); ok {
				s := string(text)
				reason = &s
			}
			if text, ok := verdictValue.(pyjson.String); ok {
				s := string(text)
				verdict = &s
			}
			card, err := wirePlaceCard(place, reason, verdict, group)
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

func wireTaste(group taste.Profile) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("basis", pyjson.String(string(group.Basis)))
	interests := pyjson.List{}
	for _, tag := range group.Interests {
		interests = append(interests, pyjson.String(tag))
	}
	out.Set("interests", interests)
	if group.BudgetPerPersonVND == nil {
		out.Set("budget_per_person_vnd", pyjson.Null{})
	} else {
		out.Set("budget_per_person_vnd", pyjson.NewInt(*group.BudgetPerPersonVND))
	}
	if group.Size == nil {
		out.Set("size", pyjson.Null{})
	} else {
		out.Set("size", pyjson.NewInt(*group.Size))
	}
	out.Set("people", pyjson.NewInt(group.People))
	out.Set("people_answered", pyjson.NewInt(group.PeopleAnswered))
	return out
}

func destinationOrDefault(ctx context.Context, store repo.Repository, destinationID *string) (*repo.Destination, error) {
	if destinationID != nil {
		return store.GetDestination(ctx, *destinationID)
	}
	rows, err := store.ListDestinations(ctx)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	row := rows[0]
	return &row, nil
}

func tasteProfile(ctx context.Context, store repo.Repository, call *endpoint.Call, contextID *string) (taste.Profile, error) {
	if contextID != nil {
		if call.Actor == nil {
			return taste.Profile{}, endpoint.Refuse(401, "authentication_required", "Cần đăng nhập để đọc gu nhóm.")
		}
		if err := requireGroupMember(ctx, call, store, "view_group_preference_profile", *contextID); err != nil {
			return taste.Profile{}, err
		}
		return groupTaste(ctx, store, *contextID, time.Now().UTC())
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
	covers, err := store.PhotoCovers(ctx, ids)
	if err != nil {
		return nil, err
	}
	counts, err := store.PhotoCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]*pyjson.OrderedMap, 0, len(rows))
	for _, row := range rows {
		card := placeRow(row)
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
	return out, nil
}

func placeRow(row repo.Place) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(row.ID))
	out.Set("destination_id", pyjson.String(row.DestinationID))
	out.Set("name", pyjson.String(row.Name))
	out.Set("category", pyjson.String(row.Category))
	out.Set("kinds", stringList(row.Kinds))
	out.Set("rating", floatOrNull(row.Rating))
	if row.RatingCount == nil {
		out.Set("rating_count", pyjson.Null{})
	} else {
		out.Set("rating_count", pyjson.NewInt(*row.RatingCount))
	}
	out.Set("distance_km", floatOrNull(row.DistanceKM))
	out.Set("price_min_vnd", intOrNull(row.PriceMinVND))
	out.Set("price_max_vnd", intOrNull(row.PriceMaxVND))
	out.Set("address", textOrNull(row.Address))
	if row.OpenNow == nil {
		out.Set("open_now", pyjson.Null{})
	} else {
		out.Set("open_now", pyjson.Bool(*row.OpenNow))
	}
	out.Set("open_hours", textOrNull(row.OpenHours))
	out.Set("travel_minutes", intOrNull(row.TravelMinutes))
	out.Set("photo_count", pyjson.NewInt(row.PhotoCount))
	out.Set("traits", stringList(row.Traits))
	out.Set("group_fit", wireGroupFit(row.GroupFit))
	out.Set("activities", jsonListOrEmpty(row.Activities))
	out.Set("flag", textOrNull(row.Flag))
	out.Set("lat", floatOrNull(row.Lat))
	out.Set("lng", floatOrNull(row.Lng))
	out.Set("geo_precision", textOrNull(row.GeoPrecision))
	out.Set("description", textOrNull(row.Description))
	out.Set("reviews", wireReviews(row.Reviews))
	out.Set("source", pyjson.String(row.Source))
	out.Set("license", textOrNull(row.License))
	return out
}

func stringList(values []string) pyjson.List {
	list := pyjson.List{}
	for _, value := range values {
		list = append(list, pyjson.String(value))
	}
	return list
}

func jsonListOrEmpty(raw json.RawMessage) pyjson.Value {
	if len(raw) == 0 {
		return pyjson.List{}
	}
	value, err := pyjson.Loads(raw)
	if err != nil {
		return pyjson.List{}
	}
	if list, ok := value.(pyjson.List); ok {
		return list
	}
	return pyjson.List{}
}

func placeID(place *pyjson.OrderedMap) string {
	value, _ := place.Get("id")
	text, _ := value.(pyjson.String)
	return string(text)
}

func scoreCard(place *pyjson.OrderedMap, group taste.Profile) (*big.Int, []scoring.Factor, error) {
	scored, err := scoringPlace(place)
	if err != nil {
		return nil, nil, err
	}
	return scoring.ScorePlace(scored, group)
}

func scoreOrZero(place *pyjson.OrderedMap, group taste.Profile) int64 {
	score, _, err := scoreCard(place, group)
	if err != nil || score == nil {
		return 0
	}
	if score.IsInt64() {
		return score.Int64()
	}
	return math.MaxInt64
}

func scoringPlace(place *pyjson.OrderedMap) (scoring.Place, error) {
	category, _ := place.Get("category")
	cat, _ := category.(pyjson.String)
	out := scoring.Place{
		Category:      string(cat),
		Kinds:         stringsOf(place, "kinds"),
		Traits:        stringsOf(place, "traits"),
		PriceMinVND:   intPtrOf(place, "price_min_vnd"),
		PriceMaxVND:   intPtrOf(place, "price_max_vnd"),
		DistanceKM:    floatPtrOf(place, "distance_km"),
		TravelMinutes: intPtrOf(place, "travel_minutes"),
	}
	fitValue, _ := place.Get("group_fit")
	if fit, ok := fitValue.(*pyjson.OrderedMap); ok {
		minP := intPtrOf(fit, "min_people")
		maxP := intPtrOf(fit, "max_people")
		if minP != nil && maxP != nil {
			out.GroupFit = &scoring.GroupFit{MinPeople: *minP, MaxPeople: *maxP}
		}
	}
	return out, nil
}

func stringsOf(place *pyjson.OrderedMap, key string) []string {
	value, _ := place.Get(key)
	list, ok := value.(pyjson.List)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if text, ok := item.(pyjson.String); ok {
			out = append(out, string(text))
		} else {
			out = append(out, "")
		}
	}
	return out
}

func intPtrOf(place *pyjson.OrderedMap, key string) *int64 {
	value, ok := place.Get(key)
	if !ok {
		return nil
	}
	switch v := value.(type) {
	case pyjson.Null:
		return nil
	case pyjson.Int:
		n, ok := v.Int64()
		if !ok {
			return nil
		}
		return &n
	}
	return nil
}

func floatPtrOf(place *pyjson.OrderedMap, key string) *float64 {
	value, ok := place.Get(key)
	if !ok {
		return nil
	}
	switch v := value.(type) {
	case pyjson.Null:
		return nil
	case pyjson.Float:
		n := float64(v)
		return &n
	case pyjson.Int:
		n, _ := v.Big().Float64()
		return &n
	}
	return nil
}

func wirePlaceCard(place *pyjson.OrderedMap, reason, verdict *string, group taste.Profile) (*pyjson.OrderedMap, error) {
	if reason == nil || verdict == nil {
		reason, verdict = nil, nil
	}
	score, factors, err := scoreCard(place, group)
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
	low := intPtrOf(place, "price_min_vnd")
	high := intPtrOf(place, "price_max_vnd")
	if low != nil && high != nil {
		lowK, highK := *low/1000, *high/1000
		band := strconv.FormatInt(lowK, 10) + "k"
		if lowK != highK {
			band = strconv.FormatInt(lowK, 10) + "–" + strconv.FormatInt(highK, 10) + "k"
		}
		parts = append(parts, "Khoảng "+band+"/người")
	}
	if km := floatPtrOf(place, "distance_km"); km != nil {
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
	return placeID(a) < placeID(b)
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

func fetchReasons(places []*pyjson.OrderedMap, group taste.Profile) map[string]reasonPair {
	out := map[string]reasonPair{}
	if len(places) == 0 {
		return out
	}
	body := pyjson.NewOrderedMap()
	rows := pyjson.List{}
	for _, place := range places {
		row := pyjson.NewOrderedMap()
		row.Set("place", place)
		rows = append(rows, row)
	}
	body.Set("rows", rows)
	body.Set("group", wireTaste(group))
	raw, err := brain.Configured().PostJSON("place-reasons", body)
	if err != nil {
		return out
	}
	obj, err := brain.AsObject(raw)
	if err != nil {
		return out
	}
	for key, value := range obj.All() {
		entry, ok := value.(*pyjson.OrderedMap)
		if !ok {
			continue
		}
		reasonValue, _ := entry.Get("reason")
		verdictValue, _ := entry.Get("verdict")
		reason, rok := reasonValue.(pyjson.String)
		verdict, vok := verdictValue.(pyjson.String)
		pair := reasonPair{}
		if rok {
			s := string(reason)
			pair.reason = &s
		}
		if vok {
			s := string(verdict)
			pair.verdict = &s
		}
		out[key] = pair
	}
	return out
}

func clientPlaces(places []*pyjson.OrderedMap) []*pyjson.OrderedMap {
	fields := []string{"id", "name", "address", "price_min_vnd", "price_max_vnd", "rating", "distance_km", "open_hours", "category"}
	out := []*pyjson.OrderedMap{}
	for _, place := range treejson.MapsFrom(promptsafety.Filter(treejson.MapsTo(places))) {
		entry := pyjson.NewOrderedMap()
		for _, field := range fields {
			if value, ok := place.Get(field); ok {
				entry.Set(field, value)
			} else {
				entry.Set(field, pyjson.Null{})
			}
		}
		out = append(out, entry)
	}
	return out
}

func modelPlaceRows(ctx context.Context, store repo.Repository, group taste.Profile) ([]*pyjson.OrderedMap, error) {
	diemDen, err := destinationOrDefault(ctx, store, nil)
	if err != nil {
		return nil, err
	}
	filter := repo.PlaceFilter{}
	if diemDen != nil {
		filter.DestinationID = &diemDen.ID
	}
	rows, err := store.ListPlaces(ctx, filter)
	if err != nil {
		return nil, err
	}
	cards := make([]*pyjson.OrderedMap, 0, len(rows))
	for _, row := range rows {
		cards = append(cards, placeRow(row))
	}
	sort.SliceStable(cards, func(i, j int) bool {
		si, sj := scoreOrZero(cards[i], group), scoreOrZero(cards[j], group)
		if si != sj {
			return si > sj
		}
		return placeID(cards[i]) < placeID(cards[j])
	})
	if len(cards) > maxModelPlaces {
		cards = cards[:maxModelPlaces]
	}
	return clientPlaces(cards), nil
}
