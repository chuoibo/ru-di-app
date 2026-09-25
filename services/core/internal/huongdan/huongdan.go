// Package huongdan is the app manual Nếp answers «how do I …, what do I tap?»
// from (design 05 §3, design 04 §4b).
//
// The data is two things, both embedded:
//
//   - data/*.md, one file per screen, written by people. Front matter between a
//     `---json` line and a `---` line: {man, tieu_de, nhanUI, di_toi[{nhan,
//     man}], tien}. Every `## ` section is one task and one retrievable
//     passage (a Doan).
//   - data/_rut.json, the map of routes, labels and navigation edges pulled out
//     of the app's own source by apps/mobile/tools/rut-huong-dan.mjs. The drift
//     gate apps/mobile/tests/huong-dan-khop-ma.test.mjs holds the manuals to it
//     and it to the code.
//
// Pure: no database, no model, no network, no clock. Everything is parsed and
// validated once, at package init; a manual that does not parse, names a
// screen the app does not have, declares a way the app does not have (not an
// edge of _rut.json, not the tab bar, not a named exception), or lets a money
// screen say more than how to get there and on, stops the process at start
// instead of becoming a wrong answer.
//
// Four questions are answered here: TheoMan (the manual of one screen), Tim
// (which passages answer a question, lexically, through rag/xephang), DuongToi
// (the shortest way from one screen to another, and what to tap on each step)
// and BanDung (which build of the app map this binary carries).
package huongdan

import (
	"context"
	"embed"
)

//go:embed data/*.md data/_rut.json
var duLieu embed.FS

// soTay is the manual this binary carries. Built at init; a bad manual panics
// here, which is the point: see phaiNap.
var soTay = phaiNap(duLieu)

// KMacDinh is how many passages Tim returns when the question does not say.
const KMacDinh = 4

// MaxBuoc is the longest way DuongToi gives. A route longer than this is not
// directions a person can follow from a chat bubble (design 05 §3).
const MaxBuoc = 5

// Doan is one section of a screen's manual: one task, one passage.
type Doan struct {
	// ID is «<file>/<heading slug>», e.g. «chat-nhom/tao-mot-binh-chon». It is
	// what an answer cites in nguon[], so it only changes when the heading does.
	ID string
	// Man is the route id of the screen, the same string PhieuNguCanh.man
	// carries (`outings/[id]`, `plan`).
	Man string
	// TieuDeMan is the screen's title from its front matter.
	TieuDeMan string
	// TieuDe is the section heading: the task, in the words of the manual.
	TieuDe string
	// Chu is the section body as written, trimmed.
	Chu string
	// Buoc are the steps: the list items of the section, markers stripped, in
	// order.
	Buoc []string
	// Nhan are the labels the section quotes in «…», in order of first use.
	// Each one is declared in the screen's nhanUI, which the drift gate holds
	// to a literal in the app's source.
	Nhan []string
	// Tien marks a money screen (its route's first segment is in manTienDau).
	// Its manual has exactly one section, headed tieuDeManTien, and every step
	// of it names a way in or out that is a button of the code (see
	// kiemManTien); nothing on a money screen says how to pay, split or settle.
	Tien bool
}

// Hoi is a question to the manual.
type Hoi struct {
	// Cau is the question as the person typed it, with or without diacritics.
	Cau string
	// Man is the screen the person is on, as declared (`outings/[id]`) or as
	// walked (`/outings/7`). Its sections come first when they score at least
	// half the best match (tyLeGhim).
	Man string
	// K is how many passages to return; zero or less means KMacDinh.
	K int
}

// Buoc is one step of a way between two screens.
type Buoc struct {
	Tu  string // route the step starts on
	Den string // route the step lands on
	// Nhan is the label to tap, when a manual declares one for this edge (its
	// di_toi, or the tab's title for the tab bar). Empty when only the code
	// knows the edge: the answer then names the screen, not a button.
	Nhan string
	// TieuDe is the title of the screen the step lands on, when it has a
	// manual; empty otherwise.
	TieuDe string
}

// TheoMan returns the sections of one screen's manual, in file order. The
// screen may be given as declared or as walked; an unknown screen, or one with
// no manual, returns nil. On a money screen this is the single navigation
// section and nothing else.
func TheoMan(man string) []Doan { return soTay.theoMan(man) }

// Tim returns at most h.K passages (KMacDinh when h.K ≤ 0) that answer h.Cau,
// best first. Only the first MaxRuneCau runes of h.Cau are read; its
// teencode syllables the manual does not use are rewritten first (see teen).
// Ranking is BM25 over rag/xephang terms of the section heading and body (see
// chuDeXep). A passage must share at least one content syllable with the
// question (see tuDem); passages of h.Man scoring at least tyLeGhim of the
// best come first, then the rest, each group in score order, ties broken by
// id. A done context returns nil.
func Tim(ctx context.Context, h Hoi) []Doan { return soTay.tim(ctx, h) }

// DuongToi returns the shortest way from screen tu to screen den, one Buoc per
// tap, and whether there is one within MaxBuoc steps. Screens may be given as
// declared or as walked. tu == den is a way of zero steps. Among ways of the
// same length it prefers the one passing through the fewest money screens
// (manTienDau), then, step by step from tu, an edge a manual labels, then the
// smaller route id, so the same graph gives the same way on every run. The
// sign-in screens (manVao) are never passed through.
func DuongToi(tu, den string) ([]Buoc, bool) { return soTay.duongToi(tu, den) }

// BanDung is the first 12 hex characters of the sha256 of the embedded
// _rut.json: which build of the app map this binary answers from. The app
// carries the same constant in src/rudi/nep/huong-dan-ban.ts, written by the
// extractor, so a phiếu from a different build can be told apart.
func BanDung() string { return soTay.ban }

// ChuanMan maps a screen as walked (`/outings/7?ctx=x`) or as declared
// (`outings/[id]`) to its route id, the way expo-router matches: a static
// segment beats a dynamic one. Empty when no route matches.
func ChuanMan(man string) string { return soTay.chuanMan(man) }
