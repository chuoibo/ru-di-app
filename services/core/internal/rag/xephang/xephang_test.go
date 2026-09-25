package xephang

import (
	"reflect"
	"testing"
)

func TestThuatFoldsAndPairsSyllables(t *testing.T) {
	got := Thuat("Cà phê, ĐÀ LẠT!")
	want := []string{"ca", "phe", "da", "lat", "ca_phe", "phe_da", "da_lat"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Thuat = %q, want %q", got, want)
	}
	if got := ChuTimKiem("bún chả"); got != "bun cha bun_cha" {
		t.Fatalf("ChuTimKiem = %q", got)
	}
	if got := Thuat("  ...  "); len(got) != 0 {
		t.Fatalf("punctuation alone has no terms: %q", got)
	}
}

var quan = []Doan{
	{ID: "p1", Chu: "Cà phê view đồi thông, mở sớm"},
	{ID: "p2", Chu: "Bún chả Hà Nội, quán nhỏ"},
	{ID: "p3", Chu: "Chả giò, bún"},
	{ID: "p4", Chu: "Phê duyệt kế hoạch chuyến đi"},
	{ID: "p5", Chu: "Quán cà phê sách yên tĩnh"},
}

func ids(hits []KetQua) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.ID
	}
	return out
}

func TestTimFindsWithoutDiacritics(t *testing.T) {
	m := Dung(quan)
	withMarks := ids(m.Tim("cà phê", 3))
	without := ids(m.Tim("ca phe", 3))
	if !reflect.DeepEqual(withMarks, without) {
		t.Fatalf("with marks %v, without %v", withMarks, without)
	}
	if len(without) < 2 || (without[0] != "p1" && without[0] != "p5") {
		t.Fatalf("coffee query ranked %v", without)
	}
}

func TestTimSyllablePairSeparatesDishes(t *testing.T) {
	m := Dung(quan)
	// «bún chả» shares both syllables with p3, which is also the shorter text
	// and so wins on syllables alone; only p2 has the pair.
	got := ids(m.Tim("bún chả", 5))
	if len(got) == 0 || got[0] != "p2" {
		t.Fatalf("bún chả ranked %v, want p2 first", got)
	}
	// «cà phê» must outrank the text that only shares «phê».
	got = ids(m.Tim("cà phê", 5))
	for i, id := range got {
		if id == "p4" && i < 2 {
			t.Fatalf("phê duyệt ranked %d in %v", i, got)
		}
	}
}

func TestTimIsDeterministicAndBounded(t *testing.T) {
	m := Dung([]Doan{{ID: "b", Chu: "hồ xuân hương"}, {ID: "a", Chu: "hồ xuân hương"}, {ID: "c", Chu: "chợ đêm"}})
	got := m.Tim("hồ xuân hương", 5)
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" || got[0].Diem != got[1].Diem {
		t.Fatalf("equal scores must break ties by id: %+v", got)
	}
	if got := m.Tim("hồ xuân hương", 1); len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("k=1: %+v", got)
	}
	if got := m.Tim("không có chữ nào khớp", 5); got != nil {
		t.Fatalf("no shared term must return nothing: %+v", got)
	}
	if got := m.Tim("chợ", 0); got != nil {
		t.Fatalf("k=0: %+v", got)
	}
	if got := Dung(nil).Tim("chợ", 3); got != nil {
		t.Fatalf("empty index: %+v", got)
	}
}

func TestRepeatedQueryWordCountsOnce(t *testing.T) {
	m := Dung(quan)
	once := m.Tim("bún", 5)
	thrice := m.Tim("bún bún bún", 5)
	if len(once) == 0 || once[0].ID != thrice[0].ID {
		t.Fatalf("once %+v, thrice %+v", once, thrice)
	}
	// The pair «bun_bun» exists in no text, so the scores are the same.
	if once[0].Diem != thrice[0].Diem {
		t.Fatalf("repetition changed the score: %v vs %v", once[0].Diem, thrice[0].Diem)
	}
}

func TestDuplicateIDPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a repeated id must panic")
		}
	}()
	Dung([]Doan{{ID: "x", Chu: "a"}, {ID: "x", Chu: "b"}})
}
