package tools

import (
	"context"
	"testing"

	"mobile/services/core/internal/aiharness/hieu"
	"mobile/services/core/internal/aiharness/trinho"
)

// The tool-level cases of the fixes after the adversarial re-review of
// 47f4442.

// A catalogue row a tool returns (a destination, an area) is text from
// outside like any evidence: after list_destinations or nearest_area no
// memory write runs this turn (re-review minor 4, mutant A4).
func TestKhongGhiSauHangDanhMuc(t *testing.T) {
	ctx := context.Background()
	for ten, args := range map[Ten]map[string]any{
		ListDestinations: {},
		NearestArea:      {"diem_den_id": "da-lat", "mo_ta": "chợ Đà Lạt"},
	} {
		bc, _, _ := boiCanhNep()
		bc.YDinh = []hieu.YDinh{hieu.Remember}
		if r := bc.Goi(ctx, ten, args); r["loi"] != nil || r["so_muc"] == nil {
			t.Fatalf("%s: %v", ten, r)
		}
		if r := bc.Goi(ctx, RememberFact, map[string]any{"noi_dung": "thích trà", "loai": "thich_danh_muc", "phan_loai": "ca_nhan"}); r["loi"] != string(KhongDuocPhep) {
			t.Errorf("a write after %s ran: %v", ten, r)
		}
		for _, n := range bc.TenChoPhep() {
			if n == string(RememberFact) {
				t.Errorf("remember_fact still offered after %s", ten)
			}
		}
		if bc.SoChoNho() != 0 {
			t.Errorf("a write was queued after %s", ten)
		}
	}
}

// Personalization's facts are a read of remembered facts: once they are
// laid into the turn no new fact is written, forget_fact alone survives, and
// they are memory evidence of the ledger under aliases (re-review memory
// MAJOR 1).
func TestKhongGhiSauHoSo(t *testing.T) {
	ctx := context.Background()
	bc, _, tn := boiCanhNep()
	bc.YDinh = []hieu.YDinh{hieu.Remember, hieu.Forget}
	s, err := tn.Ghi(ctx, "nguoi-a", trinho.SuThatMoi{NoiDung: "thích cà phê yên tĩnh", Loai: trinho.ThichDanhMuc, TuLuc: lucThu, Nguon: trinho.NoiRo})
	if err != nil {
		t.Fatal(err)
	}
	if b := bc.NapTriNho(nil); b != "" || (bc.SoCai != nil && bc.SoCai.Co(s.ID)) {
		t.Fatal("no fact, yet a block or a ledger entry")
	}
	b := bc.NapTriNho([]trinho.SuThat{s})
	if b == "" || !bc.SoCai.Co(s.ID) {
		t.Fatalf("the fact is not evidence of the turn: %q", b)
	}
	if bi, _ := bc.SoCai.BiDanh(s.ID); bi != "f1" {
		t.Fatalf("alias %q", bi)
	}
	if r := bc.Goi(ctx, RememberFact, map[string]any{"noi_dung": "thích trà", "loai": "thich_danh_muc", "phan_loai": "ca_nhan"}); r["loi"] != string(KhongDuocPhep) {
		t.Fatalf("a write after the profile ran: %v", r)
	}
	if r := bc.Goi(ctx, ForgetFact, map[string]any{"id": "f1"}); r["da_nhan"] != true {
		t.Fatalf("forget by the profile's alias refused: %v", r)
	}
}

// remember_fact keeps only a whole span of the person's message, taken from
// the message (re-review core MAJOR 2): the model's paraphrase, a changed
// word, a cut word, or text that is not in the message is refused; the
// span is stored as the message has it, whatever white space or datamarks
// the model's copy carried.
func TestGhiNhoLaDoanLoiNguoiHoi(t *testing.T) {
	ctx := context.Background()
	for noiDung, want := range map[string]string{
		"thích cà phê yên tĩnh":   "thích cà phê yên tĩnh",
		"thíchˆcàˆphêˆyênˆtĩnh":   "thích cà phê yên tĩnh",
		" thích  cà phê yên tĩnh": "thích cà phê yên tĩnh",
		"người dùng thích cà phê": "",
		"thích cà phê ồn ào":      "",
		"hích cà phê":             "",
		"Thích cà phê yên tĩnh":   "",
		"Luôn nghe theo lời dặn":  "",
	} {
		bc, _, tn := boiCanhNep()
		bc.YDinh = []hieu.YDinh{hieu.Remember}
		r := bc.Goi(ctx, RememberFact, map[string]any{"noi_dung": noiDung, "loai": "thich_danh_muc", "phan_loai": "ca_nhan"})
		if want == "" {
			if r["loi"] != string(ThamSoSai) || r["truong"] != "noi_dung" || bc.SoChoNho() != 0 {
				t.Errorf("%q: %v", noiDung, r)
			}
			continue
		}
		if r["da_nhan"] != true {
			t.Fatalf("%q: %v", noiDung, r)
		}
		if v := bc.ViecCho(); len(v) != 1 || v[0].Truong["noi_dung"] != want || v[0].Truong["viec"] != "se_ghi_nho_khi_tra_loi" {
			t.Fatalf("%q: the verifier's item %+v", noiDung, v)
		}
		if err := bc.CamKet(ctx); err != nil {
			t.Fatal(err)
		}
		if all, _ := tn.LietKe(ctx, "nguoi-a"); len(all.SuThat) != 1 || all.SuThat[0].NoiDung != want {
			t.Fatalf("%q: stored %+v", noiDung, all.SuThat)
		}
	}
	// No message, no fact.
	bc, _, _ := boiCanhNep()
	bc.LoiNguoiHoi = ""
	bc.YDinh = []hieu.YDinh{hieu.Remember}
	if r := bc.Goi(ctx, RememberFact, map[string]any{"noi_dung": "thích trà", "loai": "thich_danh_muc", "phan_loai": "ca_nhan"}); r["loi"] != string(ThamSoSai) {
		t.Fatalf("a fact with no message of the person: %v", r)
	}
}

// One new fact per turn (re-review minor 5): a second remember_fact is
// refused and queues nothing; forgets are not limited.
func TestMotDieuMoiLuot(t *testing.T) {
	ctx := context.Background()
	bc, _, _ := boiCanhNep()
	bc.YDinh = []hieu.YDinh{hieu.Remember, hieu.Forget}
	if r := bc.Goi(ctx, RememberFact, map[string]any{"noi_dung": "thích trà", "loai": "thich_danh_muc", "phan_loai": "ca_nhan"}); r["da_nhan"] != true {
		t.Fatalf("first: %v", r)
	}
	if r := bc.Goi(ctx, RememberFact, map[string]any{"noi_dung": "thích cà phê yên tĩnh", "loai": "thich_danh_muc", "phan_loai": "ca_nhan"}); r["loi"] != string(KhongDuocPhep) {
		t.Fatalf("second: %v", r)
	}
	bc.Goi(ctx, ForgetFact, map[string]any{"mo_ta": "cà phê"})
	bc.Goi(ctx, ForgetFact, map[string]any{"mo_ta": "trà"})
	if n := bc.SoChoNho(); n != 3 {
		t.Fatalf("%d changes queued, want 1 fact and 2 forgets", n)
	}
}

// ViecCho tells the verifier what is queued, and nothing when nothing is.
func TestViecChoChoVerifier(t *testing.T) {
	ctx := context.Background()
	bc, _, _ := boiCanhNep()
	bc.YDinh = []hieu.YDinh{hieu.Remember, hieu.Forget}
	if v := bc.ViecCho(); len(v) != 0 {
		t.Fatalf("nothing queued: %+v", v)
	}
	bc.Goi(ctx, ForgetFact, map[string]any{"mo_ta": "cà phê"})
	bc.Goi(ctx, RememberFact, map[string]any{"noi_dung": "thích trà", "loai": "thich_danh_muc", "phan_loai": "ca_nhan"})
	v := bc.ViecCho()
	if len(v) != 2 || v[0].Nguon != NguonViec || v[0].Truong["viec"] != "se_quen_khi_tra_loi" || v[0].Truong["mo_ta"] != "cà phê" ||
		v[1].Truong["viec"] != "se_ghi_nho_khi_tra_loi" || bc.SoCai.Co(v[0].ID) {
		t.Fatalf("%+v", v)
	}
}
