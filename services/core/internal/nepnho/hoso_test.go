package nepnho

import (
	"context"
	"errors"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// triNhoDem is a trinho.TriNho whose Nho returns fixed facts and counts calls.
type triNhoDem struct {
	ds   []trinho.SuThat
	goi  int
	cauK int
}

func (t *triNhoDem) Nho(_ context.Context, _, _ string, k int) ([]trinho.SuThat, error) {
	t.goi++
	t.cauK = k
	return t.ds, nil
}
func (t *triNhoDem) Ghi(context.Context, string, trinho.SuThatMoi) (trinho.SuThat, error) {
	return trinho.SuThat{}, errors.New("unused")
}
func (t *triNhoDem) Quen(context.Context, string, trinho.QuenGi) (int, error) { return 0, nil }
func (t *triNhoDem) LietKe(context.Context, string) (trinho.TatCa, error)     { return trinho.TatCa{}, nil }

// xepNguoc reverses and cuts; bia adds an id it was never given.
type xepNguoc struct {
	bia  bool
	loi  bool
	goiN int
}

func (x *xepNguoc) XepLai(_ context.Context, _ string, bc []truyhoi.BangChung, topN int) ([]truyhoi.BangChung, error) {
	x.goiN = topN
	if x.loi {
		return nil, errors.New("rerank down")
	}
	var out []truyhoi.BangChung
	if x.bia {
		out = append(out, truyhoi.BangChung{ID: "bia", Nguon: truyhoi.Memory})
	}
	for i := len(bc) - 1; i >= 0; i-- {
		out = append(out, bc[i])
	}
	if len(out) > topN {
		out = out[:topN]
	}
	return out, nil
}

func facts(n int) []trinho.SuThat {
	var out []trinho.SuThat
	for i := 0; i < n; i++ {
		out = append(out, trinho.SuThat{ID: string(rune('a' + i)), NoiDung: "sự thật " + string(rune('A'+i)), Loai: trinho.ThichDanhMuc,
			TuLuc: time.Date(2026, 9, 25, 7, 0, 0, 0, time.UTC), Nguon: trinho.NoiRo})
	}
	return out
}

// Only the person's own words (noi_ro) reach the sidecar's extraction,
// which reads its input as theirs: a fact learnt any other way is refused
// before any call (re-review memory MAJOR 2).
func TestChiLoiNguoiDenTrichXuat(t *testing.T) {
	gia := moiKhoGia()
	k, err := Moi(nil, gia, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []trinho.NguonSuThat{trinho.HanhVi, trinho.HoiDap} {
		moi := trinho.SuThatMoi{NoiDung: "thích trà", Loai: trinho.ThichDanhMuc, Nguon: n, TuLuc: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}
		if _, err := k.Ghi(context.Background(), "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", moi); !errors.Is(err, ErrKhongLoiNguoi) {
			t.Fatalf("%s: %v", n, err)
		}
	}
	if gia.goi["them"] != 0 {
		t.Fatal("the sidecar heard a fact that is not the person's words")
	}
}

// The group assistant gets nothing, and memory is not even asked.
func TestHoSoNhomKhongCoGi(t *testing.T) {
	tn := &triNhoDem{ds: facts(3)}
	h, err := DungHoSo(context.Background(), BotNhom, tn, &xepNguoc{}, "p", "q")
	if !errors.Is(err, ErrBotNhom) || len(h.SuThat) != 0 {
		t.Fatalf("group profile: %+v %v", h, err)
	}
	if tn.goi != 0 {
		t.Fatal("the group path asked memory")
	}
	if _, err := DungHoSo(context.Background(), Bot(""), tn, &xepNguoc{}, "p", "q"); !errors.Is(err, ErrBotNhom) {
		t.Fatal("an unnamed bot got a profile")
	}
}

// At most five facts, in the reranker's order (the engine renders them).
func TestHoSoNamSuThatTheoRerank(t *testing.T) {
	tn := &triNhoDem{ds: facts(9)}
	x := &xepNguoc{}
	h, err := DungHoSo(context.Background(), BotNep, tn, x, "p", "cà phê")
	if err != nil {
		t.Fatal(err)
	}
	if tn.cauK != UngVienHoSo || x.goiN != MaxSuThatHoSo {
		t.Fatalf("recall asked %d, rerank kept %d", tn.cauK, x.goiN)
	}
	if len(h.SuThat) != 5 || h.SuThat[0].ID != "i" || h.SuThat[4].ID != "e" || h.KhongRerank {
		t.Fatalf("profile %+v", h.SuThat)
	}
}

// A reranker cannot add an item; a failed rerank keeps the recall order and
// says so.
func TestHoSoRerankBiaVaLoi(t *testing.T) {
	tn := &triNhoDem{ds: facts(3)}
	h, _ := DungHoSo(context.Background(), BotNep, tn, &xepNguoc{bia: true}, "p", "q")
	for _, s := range h.SuThat {
		if s.ID == "bia" {
			t.Fatal("an id the reranker invented reached the prompt")
		}
	}
	h, err := DungHoSo(context.Background(), BotNep, tn, &xepNguoc{loi: true}, "p", "q")
	if err != nil || !h.KhongRerank || len(h.SuThat) != 3 || h.SuThat[0].ID != "a" {
		t.Fatalf("rerank failure: %+v %v", h, err)
	}
}

// Nothing recalled (memory off, or nothing held): no fact at all.
func TestHoSoRongKhiKhongNho(t *testing.T) {
	h, err := DungHoSo(context.Background(), BotNep, &triNhoDem{}, &xepNguoc{}, "p", "q")
	if err != nil || len(h.SuThat) != 0 {
		t.Fatalf("%+v %v", h, err)
	}
}
