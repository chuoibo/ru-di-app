package tools

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"mobile/services/core/internal/aiharness/dong"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/truyhoi"
)

func TestDangKyKhopTens(t *testing.T) {
	if len(DangKy) != Tens.Len() {
		t.Fatalf("registry %d, names %d", len(DangKy), Tens.Len())
	}
	for i, m := range DangKy {
		if string(m.Ten) != Tens.Values()[i] || m.MoTa == "" || !Lops.Co(m.Lop) || !Phams.Co(m.Pham) {
			t.Errorf("entry %d: %+v", i, m)
		}
		if s, ok := ThamSo(m.Ten); !ok || s == nil {
			t.Errorf("%s has no argument schema", m.Ten)
		}
		if d, ok := KhaiBao(m.Ten); !ok || d.Name != string(m.Ten) || d.Parameters == nil {
			t.Errorf("%s: declaration %+v", m.Ten, d)
		}
		// Every ordering names a property, and every required one exists.
		s, _ := ThamSo(m.Ten)
		for _, p := range append(append([]string{}, s.PropertyOrdering...), s.Required...) {
			if s.Properties[p] == nil {
				t.Errorf("%s names %q, absent", m.Ten, p)
			}
		}
	}
	if len(thamSo) != Tens.Len() {
		t.Errorf("%d schemas for %d tools", len(thamSo), Tens.Len())
	}
	if _, err := Tens.Parse("transfer_money"); !errors.Is(err, dong.ErrLa) {
		t.Error("an invented tool name parsed")
	}
	if _, err := Lops.Parse("tien"); !errors.Is(err, dong.ErrLa) {
		t.Error("a money class parsed")
	}
}

func TestQuyenGolden(t *testing.T) {
	q := MacDinh
	nep := q.DuocPhep(obs.BotNep, false)
	nhom := q.DuocPhep(obs.BotNhom, false)
	wantNep := []Ten{SearchPlaces, GetPlace, ListDestinations, NearestArea, SearchAppManual, ExplainScreen, SuggestScreen,
		ProposePlaces, ProposeItinerary, MyUpcomingOutings, RecallMemory, RememberFact, ForgetFact, WhatYouRemember}
	wantNhom := []Ten{SearchPlaces, GetPlace, ListDestinations, NearestArea, SearchAppManual, ProposePlaces, ProposeItinerary,
		DraftPoll, GroupSnapshot, ListGroupOutings}
	if !reflect.DeepEqual(nep, wantNep) {
		t.Errorf("Nếp: %v", nep)
	}
	if !reflect.DeepEqual(nhom, wantNhom) {
		t.Errorf("group: %v", nhom)
	}
	for _, b := range []obs.Bot{obs.BotNep, obs.BotNhom} {
		for _, ten := range q.DuocPhep(b, true) {
			if m, _ := Tra(ten); m.Lop != Doc {
				t.Errorf("%s restricted still has %s (%s)", b, ten, m.Lop)
			}
		}
	}
	if q.ChoPhep(obs.BotNep, SetReminder) || q.ChoPhep(obs.BotNhom, RecallMemory) || q.ChoPhep("khach", SearchPlaces) {
		t.Error("a grant the table does not make")
	}
}

// Each case edits the golden table in one place; every one must refuse.
func TestNapQuyenTuChoi(t *testing.T) {
	cases := map[string][2]string{
		"memory to the group": {`{"ten": "recall_memory", "lop": "doc", "pham": "me", "bot": ["nep"]}`, `{"ten": "recall_memory", "lop": "doc", "pham": "me", "bot": ["nep", "nhom"]}`},
		"poll to Nếp":         {`{"ten": "draft_poll", "lop": "nhap", "pham": "nhom", "bot": ["nhom"]}`, `{"ten": "draft_poll", "lop": "nhap", "pham": "nhom", "bot": ["nhom", "nep"]}`},
		"class changed":       {`{"ten": "get_place", "lop": "doc"`, `{"ten": "get_place", "lop": "nhap"`},
		"scope changed":       {`{"ten": "remember_fact", "lop": "tri_nho", "pham": "me"`, `{"ten": "remember_fact", "lop": "tri_nho", "pham": "chung"`},
		"invented tool":       {`{"ten": "set_reminder"`, `{"ten": "transfer_money"`},
		"tool missing": {`,
    {"ten": "set_reminder", "lop": "nhac", "pham": "me", "bot": []}`, ``},
		"tool twice":    {`{"ten": "set_reminder"`, `{"ten": "get_place"`},
		"unknown bot":   {`"bot": ["nep", "nhom"]}`, `"bot": ["nep", "khach"]}`},
		"repeated bot":  {`"bot": ["nep", "nhom"]}`, `"bot": ["nep", "nep"]}`},
		"unknown field": {`"phien_ban": 1,`, `"phien_ban": 1, "ghi_chu": "",`},
		"other version": {`"phien_ban": 1,`, `"phien_ban": 2,`},
	}
	for name, c := range cases {
		if !bytes.Contains(quyenGolden, []byte(c[0])) {
			t.Fatalf("%s: %q not in the golden", name, c[0])
		}
		raw := strings.Replace(string(quyenGolden), c[0], c[1], 1)
		if _, err := NapQuyen([]byte(raw)); !errors.Is(err, ErrQuyen) {
			t.Errorf("%s: accepted (%v)", name, err)
		}
	}
	if !json.Valid(quyenGolden) {
		t.Fatal("golden is not JSON")
	}
}

func TestSoCai(t *testing.T) {
	nep := MoiSoCai(obs.BotNep)
	for i := 0; i < llm.MaxToolCallsNep; i++ {
		if err := nep.Giu(RecallMemory); err != nil {
			t.Fatal(err)
		}
	}
	if err := nep.Giu(RecallMemory); !errors.Is(err, ErrHetLuotGoi) || nep.TongGoi() != llm.MaxToolCallsNep {
		t.Fatalf("Nếp's ceiling: %v, %d", err, nep.TongGoi())
	}
	if nep.LapLai(SearchPlaces, []byte(`{"truy_van":"a"}`)) || !nep.LapLai(SearchPlaces, []byte(`{"truy_van":"a"}`)) ||
		nep.LapLai(SearchPlaces, []byte(`{"truy_van":"b"}`)) || nep.LapLai(GetPlace, []byte(`{"truy_van":"a"}`)) {
		t.Fatal("repeat detection")
	}

	s := MoiSoCai(obs.BotNhom)
	var wg sync.WaitGroup
	var mu sync.Mutex
	refused := 0
	for i := 0; i < llm.MaxToolCallsPerTurn+5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Giu(SearchPlaces); errors.Is(err, ErrHetLuotGoi) {
				mu.Lock()
				refused++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if s.TongGoi() != llm.MaxToolCallsPerTurn || refused != 5 || s.SoGoi(SearchPlaces) != llm.MaxToolCallsPerTurn {
		t.Fatalf("counted %d, refused %d", s.TongGoi(), refused)
	}
	s.Ghi(SearchPlaces, []truyhoi.BangChung{{ID: "p1", Truong: map[string]string{"ten": "A"}}, {ID: ""}, {ID: "p2"}})
	s.Ghi(GetPlace, []truyhoi.BangChung{{ID: "p1", Truong: map[string]string{"ten": "B"}}, {ID: "p3"}})
	if !reflect.DeepEqual(s.IDs(), []string{"p1", "p2", "p3"}) || s.Co("") || s.Co("p9") {
		t.Fatalf("ids %v", s.IDs())
	}
	if bc, tu, ok := s.Lay("p1"); !ok || tu != SearchPlaces || bc.Truong["ten"] != "A" {
		t.Fatalf("p1 = %+v %s %v", bc, tu, ok)
	}

	// Aliases: per source prefix, in record order, both ways.
	a := MoiSoCai(obs.BotNep)
	a.Ghi(SearchPlaces, []truyhoi.BangChung{{ID: "plc-9", Nguon: truyhoi.Places}, {ID: "plc-4", Nguon: truyhoi.Places}})
	a.Ghi(SearchAppManual, []truyhoi.BangChung{{ID: "man-1", Nguon: truyhoi.Manual}, {ID: "plc-9", Nguon: truyhoi.Places}})
	for id, want := range map[string]string{"plc-9": "p1", "plc-4": "p2", "man-1": "m1"} {
		if got, ok := a.BiDanh(id); !ok || got != want {
			t.Errorf("alias of %s = %q", id, got)
		}
		if back, ok := a.TuBiDanh(want); !ok || back != id {
			t.Errorf("%s -> %q", want, back)
		}
	}
	if _, ok := a.TuBiDanh("p3"); ok {
		t.Error("an alias never given resolved")
	}
}
