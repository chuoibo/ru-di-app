package service

import (
	"context"
	"encoding/json"
	"math"
	"math/big"
	"sort"
	"time"

	"mobile/services/core/internal/domain/promptsafety"
	"mobile/services/core/internal/domain/scoring"
	"mobile/services/core/internal/domain/taste"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/treejson"
)

// The catalogue a model is handed, and the group taste that orders it.
//
// Moved here from internal/routes unchanged, so that the one AI path that
// survives ADR-0034 (internal/chatassist) ranks places exactly the way the
// routes that Python still oracles do. Two copies of the ranking would be two
// answers to "which forty places does the model see", and only one of them
// would be under the parity gate.

// MaxModelPlaces is how many catalogue rows a model is ever handed.
const MaxModelPlaces = 40

// The three nil-to-null spellings placeRow needs. They are the same one-line
// rule as routes' own; copied rather than exported because they carry no
// field order or parsing decision that could drift.
func textOrNull(value *string) pyjson.Value {
	if value == nil {
		return pyjson.Null{}
	}
	return pyjson.String(*value)
}

func intOrNull(n *int64) pyjson.Value {
	if n == nil {
		return pyjson.Null{}
	}
	return pyjson.NewInt(*n)
}

func floatOrNull(value *float64) pyjson.Value {
	if value == nil {
		return pyjson.Null{}
	}
	return pyjson.Float(*value)
}

// GroupTaste is ApiService.group_taste: a pair whose chat consent is not active
// has no readable taste; otherwise the active members' own answers, summed.
func GroupTaste(ctx context.Context, store repo.Repository, contextID string, now time.Time) (taste.Profile, error) {
	consent, err := PairChatConsent(ctx, store, contextID, now)
	if err != nil {
		return taste.Profile{}, err
	}
	if consent != nil && !*consent {
		return taste.Unknown(), nil
	}
	members, err := store.ListMembers(ctx, contextID)
	if err != nil {
		return taste.Profile{}, err
	}
	var people []string
	for _, membership := range members {
		if membership.State == "active" {
			people = append(people, membership.PersonID)
		}
	}
	interestRows, err := store.InterestsByPerson(ctx, people)
	if err != nil {
		return taste.Profile{}, err
	}
	bandRows, err := store.BudgetBandsByPerson(ctx, people)
	if err != nil {
		return taste.Profile{}, err
	}
	interests := map[string][]string{}
	for _, row := range interestRows {
		interests[row.PersonID] = row.Tags
	}
	bands := map[string]string{}
	for _, row := range bandRows {
		bands[row.PersonID] = row.Band
	}
	profiles := make([]taste.Member, 0, len(people))
	for _, person := range people {
		member := taste.Member{Interests: interests[person]}
		if interests[person] == nil {
			member.Interests = []string{}
		}
		if band, ok := bands[person]; ok {
			member.BandID = &band
		}
		profiles = append(profiles, member)
	}
	return taste.ProfileForGroup(profiles), nil
}

// DestinationOrDefault is the named destination, or the first by sort order
// when none is named; nil when the table is empty.
func DestinationOrDefault(ctx context.Context, store repo.Repository, destinationID *string) (*repo.Destination, error) {
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

// PlaceRow is `PlaceRecord.to_row()`: one catalogue row in pydantic's field
// order, the shape both the scorer and the client read.
func PlaceRow(row repo.Place) *pyjson.OrderedMap {
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
	out.Set("activities", JSONListOrEmpty(row.Activities))
	out.Set("flag", textOrNull(row.Flag))
	out.Set("lat", pyjson.Float(row.Lat))
	out.Set("lng", pyjson.Float(row.Lng))
	out.Set("description", textOrNull(row.Description))
	out.Set("reviews", WireReviews(row.Reviews))
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

// JSONListOrEmpty decodes a jsonb array column; anything else reads as [].
func JSONListOrEmpty(raw json.RawMessage) pyjson.Value {
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

// PlaceID is a catalogue row's id, or "" when the row has none.
func PlaceID(place *pyjson.OrderedMap) string {
	value, _ := place.Get("id")
	text, _ := value.(pyjson.String)
	return string(text)
}

// ScoreCard scores one catalogue row against a taste profile.
func ScoreCard(place *pyjson.OrderedMap, group taste.Profile) (*big.Int, []scoring.Factor, error) {
	scored, err := scoringPlace(place)
	if err != nil {
		return nil, nil, err
	}
	return scoring.ScorePlace(scored, group)
}

// ScoreOrZero is ScoreCard's score as a sort key: 0 when there is none.
func ScoreOrZero(place *pyjson.OrderedMap, group taste.Profile) int64 {
	score, _, err := ScoreCard(place, group)
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
		PriceMinVND:   IntPtrOf(place, "price_min_vnd"),
		PriceMaxVND:   IntPtrOf(place, "price_max_vnd"),
		DistanceKM:    FloatPtrOf(place, "distance_km"),
		TravelMinutes: IntPtrOf(place, "travel_minutes"),
	}
	fitValue, _ := place.Get("group_fit")
	if fit, ok := fitValue.(*pyjson.OrderedMap); ok {
		minP := IntPtrOf(fit, "min_people")
		maxP := IntPtrOf(fit, "max_people")
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

// IntPtrOf reads an optional integer field of a decoded row.
func IntPtrOf(place *pyjson.OrderedMap, key string) *int64 {
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

// FloatPtrOf reads an optional number field of a decoded row.
func FloatPtrOf(place *pyjson.OrderedMap, key string) *float64 {
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

// ClientPlaces keeps the fields a model may read, after promptsafety has
// dropped every row whose text could carry an instruction.
func ClientPlaces(places []*pyjson.OrderedMap) []*pyjson.OrderedMap {
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

// ModelPlaceRows is the catalogue a model is handed: the default
// destination's places, best match for the group first, at most
// MaxModelPlaces of them, filtered by promptsafety.
func ModelPlaceRows(ctx context.Context, store repo.Repository, group taste.Profile) ([]*pyjson.OrderedMap, error) {
	diemDen, err := DestinationOrDefault(ctx, store, nil)
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
		cards = append(cards, PlaceRow(row))
	}
	sort.SliceStable(cards, func(i, j int) bool {
		si, sj := ScoreOrZero(cards[i], group), ScoreOrZero(cards[j], group)
		if si != sj {
			return si > sj
		}
		return PlaceID(cards[i]) < PlaceID(cards[j])
	})
	if len(cards) > MaxModelPlaces {
		cards = cards[:MaxModelPlaces]
	}
	return ClientPlaces(cards), nil
}

// The orders `class GroupFit` and `class Review` declare in routes/places.py,
// and therefore the orders pydantic writes.
var (
	groupFitFields = []string{"min_people", "max_people", "relation"}
	reviewFields   = []string{"author", "rating", "body"}
)

// inFieldOrder rewrites one decoded jsonb object into a model's declared field
// order. A value that is not an object, or an object missing any of the
// fields, is returned untouched: pydantic would refuse such a row, and
// half-building one here would answer with a shape Python never serves.
func inFieldOrder(value pyjson.Value, fields []string) pyjson.Value {
	obj, ok := value.(*pyjson.OrderedMap)
	if !ok {
		return value
	}
	out := pyjson.NewOrderedMap()
	for _, field := range fields {
		got, present := obj.Get(field)
		if !present {
			return value
		}
		out.Set(field, got)
	}
	return out
}

// wireGroupFit rewrites a catalogue row's group_fit into pydantic's field
// order instead of echoing the column.
//
// The column is `jsonb`, and jsonb does not keep the order it was written in:
// it sorts keys by length and then bytewise, so what went in as
// {min_people, max_people, relation} comes back out of Postgres as
// {relation, max_people, min_people} -- `relation` is eight characters, the
// other two are ten. Python never sees that order because the row passes
// through a pydantic model on its way out, and a model writes its fields as
// declared. Echoing the column instead reproduces the storage order, which is
// the same JSON and a different wire.
//
// Measured: identical body lengths, 5694 against 5694, differing from byte 489.
// Nothing but a byte comparison would have noticed.
//
// A row missing one of the three is left alone rather than half-built: pydantic
// would refuse it, and inventing a shape here would answer with a card Python
// never serves. No catalogue row is like that, so the corpus cannot reach it.
func wireGroupFit(raw json.RawMessage) pyjson.Value {
	return inFieldOrder(JSONValue(raw), groupFitFields)
}

// WireReviews is the same rule one level down: `reviews` is a jsonb ARRAY of
// objects, so the order has to be restored inside every element. Postgres puts
// `body` first there -- four characters against six -- while `class Review`
// declares author, rating, body.
func WireReviews(raw json.RawMessage) pyjson.Value {
	value := JSONValue(raw)
	list, ok := value.(pyjson.List)
	if !ok {
		return pyjson.List{}
	}
	out := make(pyjson.List, 0, len(list))
	for _, item := range list {
		out = append(out, inFieldOrder(item, reviewFields))
	}
	return out
}

// JSONValue decodes a jsonb column; an empty or unreadable one is null.
func JSONValue(raw json.RawMessage) pyjson.Value {
	if len(raw) == 0 {
		return pyjson.Null{}
	}
	value, err := pyjson.Loads(raw)
	if err != nil {
		return pyjson.Null{}
	}
	return value
}
