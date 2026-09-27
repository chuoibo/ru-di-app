package tools

// khiNao is, per tool, what it returns and when to call it or not. It is
// appended to the registry's one-line purpose in the function declaration
// the model reads (MoTaDay), so the MODEL can decide whether a tool is worth
// a call (research agentic-rag-tools §2.3: long descriptions with «when not
// to use»). Go never reads these sentences; they are the model's manual.
var khiNao = map[Ten]string{
	SearchPlaces: "Returns up to k places as evidence aliases (p1, p2…) with name, price, hours and address, plus how many candidates each hard filter removed. " +
		"The hard constraints the router already extracted are always applied; your arguments can only add constraints, never remove or loosen one. " +
		"Call it when the person wants concrete places to go. Do not call it for general advice that needs no specific place, for a place already in the evidence (use get_place), or twice with the same arguments.",
	GetPlace: "Returns the evidence fields of one place alias from this turn (p1…) or from an earlier turn (t1…). " +
		"Call it when the person asks about a place already named. Do not call it to find new places, and never with an id you did not see.",
	ListDestinations: "Returns the destinations the app covers with their ids. " +
		"Call it only when the person names or asks about a destination you cannot find among the ids you already have. Do not call it for every search.",
	NearestArea: "Returns the known areas of one destination (ids and labels) so you can pick the one the person's description means. " +
		"Call it when the person describes a part of town and you need an area id. Do not call it when no area was mentioned.",
	SearchAppManual: "Returns manual sections as aliases (m1…) with their heading, steps and the button labels to tap. " +
		"Call it when the person asks how to do something in the app. Do not call it for questions about places or plans, and never invent a button label that is not in a returned section.",
	ExplainScreen: "Returns the manual sections of the screen the person has open, as aliases (m1…). " +
		"Call it when the person asks what the current screen is or does. Do not call it when they ask about another screen (use search_app_manual).",
	SuggestScreen: "Adds a chip the person may tap to open a screen, with the taps that lead there; it opens nothing by itself and returns only whether the chip was added. " +
		"Call it after answering a how-to question when opening a screen helps. Do not call it for money screens (it is refused) or more than once per answer.",
	ProposePlaces: "Adds up to five places from the evidence (aliases p1…) to the answer as a places part; returns how many were added. " +
		"Call it once, after a search, with the places your answer recommends. Do not call it with places you did not see in the evidence.",
	ProposeItinerary: "Adds an ordered itinerary draft of evidence places (aliases p1…), each with an optional HH:MM time, to the answer; returns how many stops were added. " +
		"Call it when the person asks for a schedule or plan of several stops. Nothing is booked or saved.",
	DraftPoll: "Adds a poll draft to the answer; nothing is posted until a member taps it. Returns whether the draft was added. " +
		"Call it when the group has to choose between options. Do not call it for questions about money or payments.",
	GroupSnapshot: "Returns the group's current plan (upcoming outings, as aliases g1…) and how many members it has. " +
		"Call it when the answer depends on what the group already plans. Do not call it for questions that do not involve the group's own plans.",
	ListGroupOutings: "Returns the group's outings, upcoming or past, as aliases (g1…) with title and dates. " +
		"Call it when the person asks about the group's outings. Do not call it for place suggestions.",
	MyUpcomingOutings: "Returns the person's own upcoming outings across their groups, as aliases (g1…) with title and dates. " +
		"Call it when the person asks what they have coming up. Do not call it for anyone else.",
	RecallMemory: "Returns facts the person asked Nếp to remember that are relevant to your query, as aliases (f1…). " +
		"Call it when the answer should respect the person's own stated preferences. Do not call it for facts about other people.",
	RememberFact: "Stores one fact the person stated about themself and asked you to remember; returns its alias (f1…). " +
		"Classify it honestly in phan_loai: anything about money, other people, health or sensitive traits is refused. Call it only when the person asked to be remembered.",
	ForgetFact: "Deletes facts: either one alias (f1…) you saw, or the person's own description of what to forget. Returns how many were deleted. " +
		"Call it only when the person asks you to forget something.",
	WhatYouRemember: "Returns every fact Nếp remembers about the person, as aliases (f1…). " +
		"Call it when the person asks what you remember about them.",
	SetReminder: "Not available yet: it returns that reminders cannot be set.",
}

// MoTaDay is the function declaration's description: the registry's purpose
// followed by what the tool returns and when to call it or not.
func MoTaDay(t Ten) string {
	m, ok := Tra(t)
	if !ok {
		return ""
	}
	return m.MoTa + " " + khiNao[t]
}
