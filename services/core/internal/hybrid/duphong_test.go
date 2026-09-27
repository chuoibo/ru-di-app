package hybrid

import (
	"context"
	"errors"
	"testing"

	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/thuoctinh"
)

type phuGhi struct{ n int }

func (p *phuGhi) Tim(context.Context, truyhoi.YeuCau) (truyhoi.KetQuaTruyHoi, error) {
	p.n++
	return truyhoi.KetQuaTruyHoi{Degraded: []truyhoi.CoSuyGiam{truyhoi.LexicalOnly}}, nil
}

type hetNhung struct{ nhung.Stub }

func (hetNhung) Nhung(context.Context, []string, nhung.TacVu) ([][]float32, error) {
	return nil, nhung.ErrHetLuotNhung
}

// The lexical index answers only when the hybrid has no leg at all; a
// failed re-check is never replaced by an unchecked answer.
func TestDuPhongChiKhiKhongConNhanh(t *testing.T) {
	_, k := moiThu(t)
	k.Nhung, k.Thua = hetNhung{}, nil
	p := &phuGhi{}
	kq, err := DuPhong{Chinh: k, Phu: p}.Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu"})
	if err != nil || p.n != 1 || len(kq.Degraded) != 1 || kq.Degraded[0] != truyhoi.LexicalOnly {
		t.Fatalf("both legs down: %v, fallback %d, %v", err, p.n, kq.Degraded)
	}
	_, k2 := moiThu(t)
	k2.DocSong = DocSongHam(func(context.Context, []string) (map[string]thuoctinh.Hang, error) { return nil, errors.New("reset") })
	if _, err := (DuPhong{Chinh: k2, Phu: p}).Tim(context.Background(), truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: "lẩu"}); err == nil || p.n != 1 {
		t.Fatalf("a failed re-check fell back to the lexical index: %v, fallback %d", err, p.n)
	}
}
