package routes

import (
	"context"

	"mobile/services/core/internal/domain/areas"
	"mobile/services/core/internal/domain/interests"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
)

// interestVocabulary is GET /interests (routes/preferences.py
// read_interest_vocabulary, ApiService.interest_vocabulary): the product's
// taste words and budget bands, straight off the domain lists. No actor and no
// query; the repository dependency is declared but never used.
func interestVocabulary() Route {
	return Route{ID: "GET /interests", Status: 200, Serve: func(context.Context, *endpoint.Call) (endpoint.Reply, error) {
		tags := pyjson.List{}
		for _, tag := range interests.InterestTags() {
			entry := pyjson.NewOrderedMap()
			entry.Set("id", pyjson.String(tag.ID))
			entry.Set("label", pyjson.String(tag.Label))
			tags = append(tags, entry)
		}
		bands := pyjson.List{}
		for _, band := range interests.BudgetBands() {
			entry := pyjson.NewOrderedMap()
			entry.Set("id", pyjson.String(band.ID))
			entry.Set("label", pyjson.String(band.Label))
			entry.Set("min_vnd", pyjson.NewInt(band.MinVND))
			if band.MaxVND == nil {
				entry.Set("max_vnd", pyjson.Null{})
			} else {
				entry.Set("max_vnd", pyjson.NewInt(*band.MaxVND))
			}
			bands = append(bands, entry)
		}
		body := pyjson.NewOrderedMap()
		body.Set("interests", tags)
		body.Set("budget_bands", bands)
		return endpoint.Reply{Body: body}, nil
	}}
}

// listAreas is GET /areas (routes/social_map.py list_areas): every area as
// area_summary shapes it, in declaration order, as a top-level JSON array.
func listAreas() Route {
	return Route{ID: "GET /areas", Status: 200, Serve: func(context.Context, *endpoint.Call) (endpoint.Reply, error) {
		list := pyjson.List{}
		for _, area := range areas.All() {
			entry := pyjson.NewOrderedMap()
			entry.Set("id", pyjson.String(area.ID))
			entry.Set("label", pyjson.String(area.Label))
			entry.Set("lat", pyjson.Float(area.Lat))
			entry.Set("lng", pyjson.Float(area.Lng))
			list = append(list, entry)
		}
		return endpoint.Reply{Body: list}, nil
	}}
}

func healthz() Route {
	return Route{ID: "GET /healthz", Status: 200, Serve: func(context.Context, *endpoint.Call) (endpoint.Reply, error) {
		body := pyjson.NewOrderedMap()
		body.Set("status", pyjson.String("ok"))
		return endpoint.Reply{Body: body}, nil
	}}
}
