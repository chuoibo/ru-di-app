package tools

import (
	"context"
	"testing"

	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/testkit"
)

// tatCaCongCu is every implementation of the four tables, for the tests
// that walk the whole registry. A name in two tables is red.
func tatCaCongCu(t *testing.T) map[Ten]congCu {
	t.Helper()
	out := map[Ten]congCu{}
	for _, bang := range []map[Ten]congCu{congCusChung, congCusNhom, congCusCapDoi, congCusNep} {
		for ten, cc := range bang {
			if _, ok := out[ten]; ok {
				t.Fatalf("%s is in two tables", ten)
			}
			out[ten] = cc
		}
	}
	return out
}

// A turn's context runs only the tools of its own table: the group's (ChoNhom)
// has no memory tool to run, even past the permission table; a context built
// without a table offers the common tools only, and BoCongCu refuses to
// declare a tool the table grants but the turn cannot run.
func TestBangTheoBot(t *testing.T) {
	nhom := (&BoiCanh{Bot: obs.BotNhom, NhomID: "g", Luc: lucThu}).ChoNhom()
	for _, ten := range []Ten{RecallMemory, RememberFact, ForgetFact, WhatYouRemember, MyUpcomingOutings, ExplainScreen, SuggestScreen, SetReminder} {
		if _, ok := nhom.congCu(ten); ok {
			t.Errorf("the group's context has %s", ten)
		}
	}
	for _, ten := range []Ten{SearchPlaces, GroupSnapshot, ListGroupOutings, ProposeItinerary} {
		if _, ok := nhom.congCu(ten); !ok {
			t.Errorf("the group's context lacks %s", ten)
		}
	}
	nep := (&BoiCanh{Bot: obs.BotNep, NguoiHoi: "nguoi-a", Luc: lucThu}).ChoNep()
	for _, ten := range []Ten{GroupSnapshot, ListGroupOutings, DraftPoll} {
		if _, ok := nep.congCu(ten); ok {
			t.Errorf("Nếp's context has %s", ten)
		}
	}
	if _, ok := nep.congCu(RecallMemory); !ok {
		t.Fatal("Nếp's context lacks recall_memory")
	}
	tran := &BoiCanh{Bot: obs.BotNep, NguoiHoi: "nguoi-a", Luc: lucThu, Nguon: NguonDuLieu{TriNho: testkit.MoiTriNho()}}
	if r := tran.Goi(context.Background(), RecallMemory, map[string]any{"truy_van": "a"}); r["loi"] != string(KhongDuocPhep) {
		t.Fatalf("a context without a table ran a me tool: %v", r)
	}
	if _, err := tran.BoCongCu(); err == nil {
		t.Fatal("BoCongCu declared tools the turn cannot run")
	}
}

// The group masks draft_poll: declared, never allowed on a step, refused if
// called.
func TestCheCongCu(t *testing.T) {
	bc := (&BoiCanh{Bot: obs.BotNhom, NhomID: "g", Luc: lucThu, Che: []Ten{DraftPoll}}).ChoNhom()
	ts, err := bc.BoCongCu()
	if err != nil {
		t.Fatal(err)
	}
	khai := false
	for _, tt := range ts {
		khai = khai || tt.Name() == string(DraftPoll)
	}
	if !khai {
		t.Fatal("a masked tool is no longer declared")
	}
	for _, n := range bc.TenChoPhep() {
		if n == string(DraftPoll) {
			t.Fatal("a masked tool is allowed on a step")
		}
	}
	if r := bc.Goi(context.Background(), DraftPoll, map[string]any{"cau_hoi": "a", "lua_chon": []any{"x", "y"}}); r["loi"] != string(KhongDuocPhep) {
		t.Fatalf("a masked tool ran: %v", r)
	}
}
