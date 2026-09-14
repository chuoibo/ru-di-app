// Package routes holds the Go implementations of API routes, keyed by
// manifest id. A route here is served only when the manifest gives it to Go or
// MOBILE_CORE_CANDIDATE_ROUTES names it (package ownership).
package routes

import (
	"fmt"
	"net/http"

	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyval"
)

// Route is one Go implementation.
type Route struct {
	// ID is the manifest id, "METHOD /path".
	ID string
	// Status is the route decorator's status_code; 200 when it has none.
	Status int
	Serve  endpoint.Serve
}

// All lists every route this binary implements.
func All() []Route {
	return []Route{
		interestVocabulary(),
		listAreas(),
		createReport(),
		putMyInterests(),
		groupRecap(),
		preferenceProfile(),
		groupHeatmap(),
		postMeetingPoint(),
		socialMap(),
		listFriendRequests(),
		listFriends(),
		sendFriendRequest(),
		respondToFriendRequest(),
		createVote(),
		listContextVotes(),
		getVoteResults(),
		castVoteBallot(),
		closeVote(),
		createStory(),
		listStories(),
		markStorySeen(),
		deleteStory(),
	}
}

// Handlers binds every route to its request contract and builds its handler.
// A route whose contract pyval cannot bind refuses to start the binary.
func Handlers(contract *pyval.Contract, registry *pyval.Registry, env endpoint.Env) (map[string]http.Handler, error) {
	handlers := map[string]http.Handler{}
	for _, route := range All() {
		if _, dup := handlers[route.ID]; dup {
			return nil, fmt.Errorf("routes: %s is implemented twice", route.ID)
		}
		bound, err := contract.Bind(route.ID, registry)
		if err != nil {
			return nil, fmt.Errorf("routes: %s: %w", route.ID, err)
		}
		handler, err := endpoint.New(bound, route.Status, route.Serve, env)
		if err != nil {
			return nil, fmt.Errorf("routes: %s: %w", route.ID, err)
		}
		handlers[route.ID] = handler
	}
	return handlers, nil
}
