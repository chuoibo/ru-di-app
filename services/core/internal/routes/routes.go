// Package routes holds the Go implementations of API routes, keyed by
// manifest id. A route here is served only when the manifest gives it to Go or
// MOBILE_CORE_CANDIDATE_ROUTES names it (package ownership).
package routes

import (
	"fmt"
	"net/http"

	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyval"
	staticweb "mobile/services/core/internal/web/static"
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
		createPost(),
		listPosts(),
		listPersonPosts(),
		readPost(),
		reactToPost(),
		unreactToPost(),
		listPostComments(),
		postComment(),
		deletePostComment(),
		mintPersonID(),
		findPersonByPhone(),
		createContext(),
		updateContext(),
		inviteContextMember(),
		acceptContextMembership(),
		leaveContext(),
		listContextMembers(),
		getContextBalances(),
		getContext(),
		postContextMemory(),
		postContextCheckin(),
		listContextMemories(),
		readContextWidget(),
		postMemoryReaction(),
		deleteMemoryReaction(),
		postMemoryComment(),
		listMemoryComments(),
		uploadContextPhoto(),
		readContextPhoto(),
		setPersonAvatar(),
		readPersonAvatar(),
		uploadPersonalPhoto(),
		readPersonPhoto(),
		listMyContexts(),
		getMyProfile(),
		updateMyProfile(),
		listSavedPlaces(),
		savePlace(),
		unsavePlace(),
		listBlocked(),
		deleteMyAccount(),
		blockPerson(),
		unblockPerson(),
		openDirectMessage(),
		getPersonProfile(),
		registerPerson(),
		createBill(),
		getBill(),
		confirmBillAssignments(),
		claimBillItems(),
		splitBill(),
		createBatch(),
		publishBatch(),
		listBatchObligations(),
		listContextBatches(),
		proposeExpense(),
		confirmExpense(),
		confirmReceipt(),
		readPersonFinance(),
		readGroupBudget(),
		guestPage(),
		guestReportPayment(),
		guestNotMePage(),
		guestNotMeSubmit(),
		guestWrongAmountPage(),
		guestWrongAmountSubmit(),
		guestRequestEvidence(),
		readPairNotebook(),
		proposePairConsent(),
		grantPairConsent(),
		revokePairConsent(),
		putPairConstraint(),
		deletePairConstraint(),
		previewClosePairNotebook(),
		closePairNotebook(),
		listPairPapers(),
		draftPairPaper(),
		readPairPaper(),
		editPairDraft(),
		sendPairPaper(),
		markPairPaperViewed(),
		respondPairPaper(),
		withdrawPairPaper(),
		skipPairWeek(),
		recordPairOutingDone(),
		keepPairPaperLine(),
		createSession(),
		listSessions(),
		revokeCurrentSession(),
		revokeSession(),
		requestOTP(),
		verifyOTP(),
		loginGoogle(),
		previewOutingItinerary(),
		replaceOutingItinerary(),
		createOuting(),
		listContextOutings(),
		replaceOutingTimeline(),
		checkInToStop(),
		listOutingCheckins(),
		createOutingInvite(),
		revokeOutingInvite(),
		rotateOutingInvite(),
		acceptOutingInvite(),
		postContextMessage(),
		listContextMessages(),
		deleteOwnMessage(),
		reactToMessage(),
		unreactToMessage(),
		createChatExpenseDraft(),
		takeCompanionTurnRoute(),
		setContextMemberRole(),
		markContextRead(),
		listDestinations(),
		listPlacesWAI(),
		listPlacePhotos(),
		listPlaceGroupPhotos(),
		readPlacePhoto(),
		getPlaceWAI(),
		searchPlacesWAI(),
		scanReceipt(),
		scanScreenshot(),
		readGroupSuggestion(),
		readContextualSuggestion(),
		listTripAlbums(),
		readTripAlbum(),
		readTripReel(),
		detectFaces(),
		healthz(),
	}
}

// ImplementedIDs lists every manifest id this binary implements: the routes in
// All(), plus the mounts served outside the endpoint pipeline. `core routes
// --json` and the handler map must agree on exactly this list, or the ownership
// gate compares the manifest against a list missing a route the binary really
// does answer -- which is how a Go-served route can look unimplemented.
func ImplementedIDs() []string {
	all := All()
	out := make([]string, 0, len(all)+1)
	for _, route := range all {
		out = append(out, route.ID)
	}
	return append(out, staticweb.RouteID)
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
		handler, err := endpoint.New(bound, route.Status, authorizeChatReplay(route.Serve), env)
		if err != nil {
			return nil, fmt.Errorf("routes: %s: %w", route.ID, err)
		}
		handlers[route.ID] = handler
	}
	// MOUNT /static does not go through the endpoint pipeline: it has no
	// pydantic contract to bind, no body model and no reply to frame. It still
	// runs inside the same chain dispatch wraps every Go route in.
	if _, dup := handlers[staticweb.RouteID]; dup {
		return nil, fmt.Errorf("routes: %s is implemented twice", staticweb.RouteID)
	}
	handlers[staticweb.RouteID] = staticweb.Handler()
	return handlers, nil
}
