package routes

import (
	"context"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"

	"mobile/services/core/internal/domain/scoring"
	"mobile/services/core/internal/domain/socialmap"
	"mobile/services/core/internal/domain/taste"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
)

// mapRecommended is _MAP_RECOMMENDED.
const mapRecommended = 8

const savedUnavailableReason = "Chưa có chỗ lưu địa điểm yêu thích, nên lớp này chưa có gì để hiện."

// socialMap is GET /contexts/{context_id}/map (routes/social_map.py
// get_social_map, ApiService.get_social_map): the visited and trending layers,
// places the group has not been to ranked against its taste, and the saved
// layer declared missing. The reads run in Python's order: membership, the
// check-in scan, the group's taste (pair consent first), then one catalogue
// read, which Python caches for both layers that need it.
func socialMap() Route {
	return Route{ID: "GET /contexts/{context_id}/map", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		tx, err := call.Unit.Tx(ctx)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store := repo.Repository{Q: tx}
		member, err := store.IsMember(ctx, contextID, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		refused, err := service.RequirePermission("view_social_map", *call.Actor,
			service.Resource{Proven: map[string]bool{"is_group_member": member}})
		if err != nil {
			return endpoint.Reply{}, err
		}
		if refused != nil {
			return endpoint.Reply{}, &endpoint.Refusal{Problem: *refused}
		}
		rows, truncated, err := scanCheckins(ctx, store, contextID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		visited := socialmap.VisitedLayer(rows)
		seen := map[string]bool{}
		for _, entry := range visited {
			seen[entry.PlaceID] = true
		}
		group, err := groupTaste(ctx, store, contextID, time.Now().UTC())
		if err != nil {
			return endpoint.Reply{}, err
		}
		catalogue, err := store.ListPlaces(ctx, repo.PlaceFilter{})
		if err != nil {
			return endpoint.Reply{}, err
		}

		type scoredPlace struct {
			place repo.Place
			score *big.Int
		}
		var scored []scoredPlace
		for _, place := range catalogue {
			if seen[place.ID] {
				continue
			}
			fit, deferred := groupFit(place.GroupFit)
			if deferred != nil && group.Size != nil {
				return endpoint.Reply{}, deferred
			}
			score, _, err := scoring.ScorePlace(scoring.Place{
				Category: place.Category, Kinds: place.Kinds, Traits: place.Traits,
				PriceMinVND: place.PriceMinVND, PriceMaxVND: place.PriceMaxVND,
				DistanceKM: place.DistanceKM, TravelMinutes: place.TravelMinutes, GroupFit: fit,
			}, group)
			if err != nil {
				return endpoint.Reply{}, err
			}
			if score == nil {
				score = new(big.Int)
			}
			scored = append(scored, scoredPlace{place: place, score: score})
		}
		// sorted() is stable, keyed by (-(score or 0), id).
		slices.SortStableFunc(scored, func(a, b scoredPlace) int {
			if c := b.score.Cmp(a.score); c != 0 {
				return c
			}
			return strings.Compare(a.place.ID, b.place.ID)
		})

		visitedList := pyjson.List{}
		for _, entry := range visited {
			item := pyjson.NewOrderedMap()
			item.Set("place_id", pyjson.String(entry.PlaceID))
			item.Set("place_name", pyjson.String(entry.PlaceName))
			item.Set("lat", pyjson.Float(entry.Lat))
			item.Set("lng", pyjson.Float(entry.Lng))
			item.Set("visit_count", pyjson.NewInt(int64(entry.VisitCount)))
			visitedList = append(visitedList, item)
		}
		trendingInput := make([]socialmap.Place, 0, len(catalogue))
		for _, place := range catalogue {
			// A map layer: a place with nowhere to put a pin is not on it.
			if place.Lat == nil || place.Lng == nil {
				continue
			}
			trendingInput = append(trendingInput, socialmap.Place{
				ID: place.ID, Name: place.Name, Lat: *place.Lat, Lng: *place.Lng,
				Rating: place.Rating, RatingCount: place.RatingCount, Flag: place.Flag,
			})
		}
		trendingList := pyjson.List{}
		for _, entry := range socialmap.TrendingLayer(trendingInput) {
			item, err := mapPlace(entry.PlaceID, entry.PlaceName, entry.Lat, entry.Lng, entry.Rating, entry.RatingCount)
			if err != nil {
				return endpoint.Reply{}, err
			}
			trendingList = append(trendingList, item)
		}
		recommendedList := pyjson.List{}
		for _, entry := range scored {
			if len(recommendedList) == mapRecommended {
				break
			}
			// Counted by pins placed rather than by position in the ranking:
			// skipping a place with no coordinates must not also shorten the
			// layer by one.
			if entry.place.Lat == nil || entry.place.Lng == nil {
				continue
			}
			item, err := mapPlace(entry.place.ID, entry.place.Name, *entry.place.Lat, *entry.place.Lng, entry.place.Rating, entry.place.RatingCount)
			if err != nil {
				return endpoint.Reply{}, err
			}
			recommendedList = append(recommendedList, item)
		}
		saved := pyjson.NewOrderedMap()
		saved.Set("layer", pyjson.String("saved"))
		saved.Set("reason", pyjson.String(savedUnavailableReason))

		body := pyjson.NewOrderedMap()
		body.Set("context_id", pyjson.String(contextID))
		body.Set("visited", visitedList)
		body.Set("trending", trendingList)
		body.Set("recommended", recommendedList)
		body.Set("unavailable", pyjson.List{saved})
		body.Set("scanned_checkins", pyjson.NewInt(int64(len(rows))))
		body.Set("truncated", pyjson.Bool(truncated))
		return endpoint.Reply{Body: body}, nil
	}}
}

// mapPlace is MapPlace. rating and rating_count are required there, so a place
// without them fails pydantic while the service builds the response: a 500.
func mapPlace(id, name string, lat, lng float64, rating *float64, ratingCount *int64) (*pyjson.OrderedMap, error) {
	if rating == nil || ratingCount == nil {
		return nil, fmt.Errorf("routes: place %s has no rating for MapPlace", id)
	}
	item := pyjson.NewOrderedMap()
	item.Set("place_id", pyjson.String(id))
	item.Set("place_name", pyjson.String(name))
	item.Set("lat", pyjson.Float(lat))
	item.Set("lng", pyjson.Float(lng))
	item.Set("rating", pyjson.Float(*rating))
	item.Set("rating_count", pyjson.NewInt(*ratingCount))
	return item, nil
}

// groupTaste is ApiService.group_taste: a pair whose chat consent is not active
// has no readable taste; otherwise the active members' own answers, summed.
func groupTaste(ctx context.Context, store repo.Repository, contextID string, now time.Time) (taste.Profile, error) {
	consent, err := service.PairChatConsent(ctx, store, contextID, now)
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
