// Package tools is the engine's one tool registry (ADR-0044 §4: no second
// registry): the closed list of tool names, what each is for, its side-effect
// class and scope, its argument schema, the per-bot permission table loaded
// from testdata/quyen.golden.json, and the per-turn ledger (SoCai) that
// records which tool produced which evidence.
//
// The MODEL chooses tools (function calling). Go never picks a tool from the
// words of the message; it only filters the toolset by permission and
// policy, checks arguments against the schema and the turn's closed id
// lists, enforces MaxToolCallsPerTurn and timeouts, and records evidence
// (docs/architecture/03-ai-engine-hop-dong.md, «Luật không heuristic»).
//
// No tool writes money, obligations, outings, votes or messages. There is no
// side-effect class for any of those, so a tool that would need one cannot
// be declared. Drafts (Nhap) touch no database and reach the room only after
// a human taps them.
package tools

import (
	"mobile/services/core/internal/aiharness/dong"
)

// Ten is a tool name.
type Ten string

const (
	SearchPlaces      Ten = "search_places"
	GetPlace          Ten = "get_place"
	ListDestinations  Ten = "list_destinations"
	NearestArea       Ten = "nearest_area"
	SearchAppManual   Ten = "search_app_manual"
	ExplainScreen     Ten = "explain_screen"
	SuggestScreen     Ten = "suggest_screen"
	ProposePlaces     Ten = "propose_places"
	ProposeItinerary  Ten = "propose_itinerary"
	DraftPoll         Ten = "draft_poll"
	GroupSnapshot     Ten = "group_snapshot"
	ListGroupOutings  Ten = "list_group_outings"
	GuHaiBan          Ten = "gu_hai_ban"
	MyUpcomingOutings Ten = "my_upcoming_outings"
	RecallMemory      Ten = "recall_memory"
	RememberFact      Ten = "remember_fact"
	ForgetFact        Ten = "forget_fact"
	WhatYouRemember   Ten = "what_you_remember"
	SetReminder       Ten = "set_reminder"
)

// Tens is the closed set of tool names, in registry order.
var Tens = dong.Moi("cong_cu",
	SearchPlaces, GetPlace, ListDestinations, NearestArea, SearchAppManual,
	ExplainScreen, SuggestScreen, ProposePlaces, ProposeItinerary, DraftPoll,
	GroupSnapshot, ListGroupOutings, GuHaiBan, MyUpcomingOutings, RecallMemory,
	RememberFact, ForgetFact, WhatYouRemember, SetReminder)

// Lop is a tool's side-effect class. The set is closed and has no class
// that writes money, obligations, outings, votes or messages.
type Lop string

const (
	// Doc reads, inside a ReadOnly transaction (the database refuses a write,
	// SQLSTATE 25006), or from an in-process store.
	Doc Lop = "doc"
	// Nhap builds a draft part of the answer in the turn's state; no
	// database. A human tap is the only way it becomes anything.
	Nhap Lop = "nhap"
	// TriNho writes the asking person's own memory through its one writer
	// (nepnho), scope me only.
	TriNho Lop = "tri_nho"
	// Nhac schedules a reminder for the asking person through its one
	// writer (nepnhac), scope me only.
	Nhac Lop = "nhac"
)

// Lops is the closed set of Lop.
var Lops = dong.Moi("lop_cong_cu", Doc, Nhap, TriNho, Nhac)

// Pham is the scope a tool reads or writes.
type Pham string

const (
	// Chung: shared catalogue and manual; either bot.
	Chung Pham = "chung"
	// Me: the asking person's own data (memory, own outings, the screen card
	// of their panel); reachable only from a scope=me root, i.e. Nếp.
	Me Pham = "me"
	// Nhom: the group the question was asked in; the group assistant only.
	Nhom Pham = "nhom"
	// Doi: the couple's room the question was asked in (a chat of two whose
	// two people both said yes to «Một đôi»); the group assistant, and only
	// on a couple's turn (BoiCanh.Doi). A room of friends -- a group or an
	// ordinary chat of two -- never declares it (ADR-0048).
	Doi Pham = "doi"
)

// Phams is the closed set of Pham.
var Phams = dong.Moi("pham_cong_cu", Chung, Me, Nhom, Doi)

// Muc is one registry entry.
type Muc struct {
	Ten Ten
	// MoTa is the one-line purpose; it is also the function declaration's
	// description the model reads.
	MoTa string
	Lop  Lop
	Pham Pham
}

// DangKy is the registry, in Tens order. Its Lop and Pham are the code's
// declaration; quyen.golden.json only grants bots, and the loader refuses a
// grant that contradicts them (NapQuyen).
var DangKy = []Muc{
	{SearchPlaces, "Search the place catalogue with a query you write plus hard filters (destination, allergens, diets, open time, budget) and soft preferences; returns evidence ids.", Doc, Chung},
	{GetPlace, "Get one place's evidence fields (name, price, hours, address) by id.", Doc, Chung},
	{ListDestinations, "List the destinations the app covers, with their ids.", Doc, Chung},
	{NearestArea, "Resolve a landmark or area the person described to the nearest known area of a destination.", Doc, Chung},
	{SearchAppManual, "Search the app manual for how to do something in the app; returns section ids and button labels.", Doc, Chung},
	{ExplainScreen, "Explain the screen the person has open, from its screen card and the manual.", Doc, Me},
	{SuggestScreen, "Suggest a screen of the app to open, as a chip the person taps; opens nothing by itself.", Nhap, Me},
	{ProposePlaces, "Put up to five places from the evidence into the answer as a places part.", Nhap, Chung},
	{ProposeItinerary, "Put an ordered itinerary of places from the evidence into the answer as a draft.", Nhap, Chung},
	{DraftPoll, "Draft a poll for the group; nothing is posted until a member taps it.", Nhap, Nhom},
	{GroupSnapshot, "Read the group's shared context: members' shared tastes and the current plan.", Doc, Nhom},
	{ListGroupOutings, "List the group's outings, upcoming or past.", Doc, Nhom},
	{GuHaiBan, "Read the taste each of the two chose to share with Rủ Đi AI in this chat, and what they both like.", Doc, Doi},
	{MyUpcomingOutings, "List the asking person's own upcoming outings.", Doc, Me},
	{RecallMemory, "Recall facts the person asked Nếp to remember, relevant to a query you write.", Doc, Me},
	{RememberFact, "Remember one fact the person stated about themself, with its kind and valid time.", TriNho, Me},
	{ForgetFact, "Forget facts by id, or by the person's description of them.", TriNho, Me},
	{WhatYouRemember, "List everything Nếp remembers about the person.", Doc, Me},
	{SetReminder, "Schedule a reminder for the person about their own outing.", Nhac, Me},
}

// Tra returns the registry entry of t.
func Tra(t Ten) (Muc, bool) {
	for _, m := range DangKy {
		if m.Ten == t {
			return m, true
		}
	}
	return Muc{}, false
}

// LoiTool is the closed error code a failed tool returns to the model as
// {"loi":"<code>"}, so it can correct itself (OnToolError). It never carries
// the arguments or any data.
type LoiTool string

const (
	ThamSoSai     LoiTool = "tham_so_sai"
	KhongDuocPhep LoiTool = "khong_duoc_phep"
	HetLuotGoi    LoiTool = "het_luot_goi"
	HetHan        LoiTool = "het_han"
	KhongThay     LoiTool = "khong_thay"
	LoiNguon      LoiTool = "loi_nguon"
)

// LoiTools is the closed set of LoiTool.
var LoiTools = dong.Moi("loi_cong_cu", ThamSoSai, KhongDuocPhep, HetLuotGoi, HetHan, KhongThay, LoiNguon, ChuaCo)
