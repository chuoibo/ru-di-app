package tools

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/testkit"
	"mobile/services/core/internal/gudoi"
)

// A couple's shared taste (ADR-0048): gu_hai_ban is declared, offered and run
// only on a couple's turn of the group assistant; it takes no argument; it
// shows each person's taste under the turn's roster label and flags what
// both like; what it returned is what the worker re-checks and names.

const (
	nguoiA = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	nguoiB = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
)

func boiCanhDoi(doi bool, gu ...gudoi.Gu) (*BoiCanh, *testkit.GuDoi) {
	port := &testkit.GuDoi{Gu: gu}
	bc := &BoiCanh{Bot: obs.BotNhom, NguoiHoi: nguoiA, NhomID: "phong-doi", Luc: lucThu, Doi: doi,
		NhanDoi: map[string]string{nguoiA: "Tú", nguoiB: "Linh"}, Nguon: NguonDuLieu{Doi: port}}
	bc.ChoNhom()
	return bc, port
}

func coTrongBo(ts []Ten, t Ten) bool {
	for _, x := range ts {
		if x == t {
			return true
		}
	}
	return false
}

func khaiBao(t *testing.T, bc *BoiCanh) []string {
	t.Helper()
	ts, err := bc.BoCongCu()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, x := range ts {
		out = append(out, x.Name())
	}
	return out
}

// duLieu decodes the data block a tool result carries.
func duLieu(t *testing.T, r map[string]any) []map[string]string {
	t.Helper()
	s, _ := r["du_lieu"].(string)
	i, j := strings.Index(s, "["), strings.LastIndex(s, "]")
	if i < 0 || j < i {
		t.Fatalf("no data block: %v", r)
	}
	var out []map[string]string
	if err := json.Unmarshal([]byte(s[i:j+1]), &out); err != nil {
		t.Fatalf("%v: %s", err, s)
	}
	return out
}

// A room of friends (a group, or a chat of two without «Một đôi») never sees
// the tool: not declared, not offered, refused if called anyway, and its
// port is never read.
func TestGuHaiBanChiChoCapDoi(t *testing.T) {
	bc, port := boiCanhDoi(false, gudoi.Gu{NguoiID: nguoiA, The: []string{"cafe"}})
	if coTrongBo(bc.DuocPhep(), GuHaiBan) || strings.Contains(strings.Join(khaiBao(t, bc), ","), string(GuHaiBan)) {
		t.Fatal("a room of friends offers or declares gu_hai_ban")
	}
	if coTrongBo(MacDinh.DuocPhep(obs.BotNhom, false), GuHaiBan) || coTrongBo(MacDinh.DuocPhep(obs.BotNep, false), GuHaiBan) {
		t.Fatal("DuocPhep carries a couple's tool")
	}
	if r := bc.Goi(context.Background(), GuHaiBan, map[string]any{}); r["loi"] != string(KhongDuocPhep) {
		t.Fatalf("a room of friends ran gu_hai_ban: %v", r)
	}
	// Even with the couple's table forced in, the run itself refuses a turn
	// that is not a couple's.
	bc.rieng = congCusDoi
	if _, err := chayGuHaiBan(context.Background(), bc, &khongThamSo{}); err == nil {
		t.Fatal("chayGuHaiBan ran on a turn that is not a couple's")
	}
	if port.SoLanHoi() != 0 || len(bc.SoCai.GuDaDung()) != 0 {
		t.Fatal("the taste port was read for a room of friends")
	}
	// A couple's turn declares and offers it, read-only class.
	doi, _ := boiCanhDoi(true)
	if !coTrongBo(doi.DuocPhep(), GuHaiBan) || !strings.Contains(strings.Join(khaiBao(t, doi), ","), string(GuHaiBan)) {
		t.Fatal("a couple's turn does not offer gu_hai_ban")
	}
	if m, _ := Tra(GuHaiBan); m.Lop != Doc || m.Pham != Doi {
		t.Fatalf("gu_hai_ban is %s/%s", m.Lop, m.Pham)
	}
	// proceed_restricted keeps read tools: the taste stays readable.
	doi.HanChe = true
	if !coTrongBo(doi.DuocPhep(), GuHaiBan) {
		t.Fatal("a restricted couple's turn lost a read tool")
	}
}

// Nếp never reaches it, whatever the turn says: the table grants it to the
// group assistant only, and the run refuses any other bot.
func TestGuHaiBanNepBiTuChoi(t *testing.T) {
	port := &testkit.GuDoi{Gu: []gudoi.Gu{{NguoiID: nguoiA, The: []string{"cafe"}}}}
	nep := (&BoiCanh{Bot: obs.BotNep, NguoiHoi: nguoiA, NhomID: "phong-doi", Luc: lucThu, Doi: true, Nguon: NguonDuLieu{Doi: port}}).ChoNep()
	if coTrongBo(nep.DuocPhep(), GuHaiBan) || MacDinh.ChoPhep(obs.BotNep, GuHaiBan) {
		t.Fatal("Nếp may call gu_hai_ban")
	}
	if r := nep.Goi(context.Background(), GuHaiBan, map[string]any{}); r["loi"] != string(KhongDuocPhep) {
		t.Fatalf("Nếp ran gu_hai_ban: %v", r)
	}
	nep.Bot = obs.BotNep
	if _, err := chayGuHaiBan(context.Background(), nep, &khongThamSo{}); err == nil || port.SoLanHoi() != 0 {
		t.Fatal("the run does not refuse Nếp")
	}
	// A table granting it to Nếp, or to bot doi, is refused at load.
	for _, bot := range []string{"nep", "doi"} {
		raw := strings.Replace(string(quyenGolden), `{"ten": "gu_hai_ban", "lop": "doc", "pham": "doi", "bot": ["nhom"]}`,
			`{"ten": "gu_hai_ban", "lop": "doc", "pham": "doi", "bot": ["nhom", "`+bot+`"]}`, 1)
		if raw == string(quyenGolden) {
			t.Fatal("the golden table has no gu_hai_ban line to rewrite")
		}
		if _, err := NapQuyen([]byte(raw)); err == nil {
			t.Fatalf("a table granting gu_hai_ban to %s loaded", bot)
		}
	}
}

// The tool takes no argument: anything is refused as a schema error and
// the port is not read.
func TestGuHaiBanThuaThamSo(t *testing.T) {
	for _, args := range []map[string]any{{"nguoi": nguoiB}, {"phong": "khac"}, {"k": 3}} {
		bc, port := boiCanhDoi(true, gudoi.Gu{NguoiID: nguoiA, The: []string{"cafe"}})
		if r := bc.Goi(context.Background(), GuHaiBan, args); r["loi"] != string(ThamSoSai) {
			t.Fatalf("%v: %v", args, r)
		}
		if port.SoLanHoi() != 0 || len(bc.SoCai.GuDaDung()) != 0 {
			t.Fatal("a refused call read the port")
		}
	}
}

func TestGuHaiBanTraGuTheoNhan(t *testing.T) {
	// Only A shared under the chat's wording: only A's taste, no overlap.
	bc, port := boiCanhDoi(true, gudoi.Gu{NguoiID: nguoiA, The: []string{"cafe", "outdoor"}})
	r := bc.Goi(context.Background(), GuHaiBan, map[string]any{})
	items := duLieu(t, r)
	if len(items) != 1 || items[0]["id"] != "d1" || !strings.Contains(items[0]["nguoi"], "Tú") || !strings.Contains(items[0]["thich"], "Cafe") ||
		!strings.Contains(items[0]["thich"], "Ngoài") || items[0]["cung_thich"] != "" || r["so_nguoi_chia"] != 1 {
		t.Fatalf("%v", r)
	}
	if got := bc.SoCai.GuDaDung(); !reflect.DeepEqual(got, []string{nguoiA}) || port.Hoi[0] != "phong-doi" {
		t.Fatalf("ledger %v, room %v", got, port.Hoi)
	}
	for _, it := range items {
		for _, v := range it {
			if strings.Contains(v, nguoiA) || strings.Contains(v, nguoiB) {
				t.Fatalf("a person id reached the model: %v", it)
			}
		}
	}
	// Both: each person's, and what both like flagged.
	bc, _ = boiCanhDoi(true, gudoi.Gu{NguoiID: nguoiA, The: []string{"cafe", "outdoor"}}, gudoi.Gu{NguoiID: nguoiB, The: []string{"outdoor", "karaoke"}})
	items = duLieu(t, bc.Goi(context.Background(), GuHaiBan, map[string]any{}))
	if len(items) != 3 || !strings.Contains(items[1]["nguoi"], "Linh") || !strings.Contains(items[2]["cung_thich"], "Ngoài") || strings.Contains(items[2]["cung_thich"], "Cafe") {
		t.Fatalf("%v", items)
	}
	if got := bc.SoCai.GuDaDung(); !reflect.DeepEqual(got, []string{nguoiA, nguoiB}) {
		t.Fatalf("ledger %v", got)
	}
	// Nobody shared: an empty list, not a missing one; nothing to name.
	bc, _ = boiCanhDoi(true)
	r = bc.Goi(context.Background(), GuHaiBan, map[string]any{})
	if items := duLieu(t, r); len(items) != 0 || r["so_muc"] != 0 || r["so_nguoi_chia"] != 0 || len(bc.SoCai.GuDaDung()) != 0 {
		t.Fatalf("%v", r)
	}
	// A person the roster has no label for is left out: the tool never
	// names anybody by a name of its own.
	bc, _ = boiCanhDoi(true, gudoi.Gu{NguoiID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", The: []string{"cafe"}})
	if items := duLieu(t, bc.Goi(context.Background(), GuHaiBan, map[string]any{})); len(items) != 0 {
		t.Fatalf("%v", items)
	}
	// Without its port the tool answers loi_nguon.
	bc, _ = boiCanhDoi(true)
	bc.Nguon.Doi = nil
	if r := bc.Goi(context.Background(), GuHaiBan, map[string]any{}); r["loi"] != string(LoiNguon) {
		t.Fatalf("%v", r)
	}
}
