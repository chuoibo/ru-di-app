//go:build eval

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"mobile/services/core/internal/aieval"
)

// coMoHinh are the flags of a model-mode run.
type coMoHinh struct {
	cheDo, bo, kichBan, chiBuoc string
	lap, tranGoi                int
	duToan                      bool
	out, bang, gia, gitSHA, cay string
}

// thongTinBuild is the SHA and the dirty flag the go tool stamped into the
// binary (go build inside a git checkout); empty when it did not.
func thongTinBuild() (string, *bool) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "", nil
	}
	var sha string
	var ban *bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			sha = s.Value
		case "vcs.modified":
			b := s.Value == "true"
			ban = &b
		}
	}
	return sha, ban
}

// chayMoHinh runs `that`, `ghi` or `phat-lai`. Exit 0: the run is valid (a
// replay: and reproduced its source); 1: it ran and is unfinished, void or
// differs; 2: it could not run (flags, estimate over the ceiling, key,
// files).
func chayMoHinh(ctx context.Context, mh coMoHinh, out, errw io.Writer) int {
	if mh.lap < 1 {
		fmt.Fprintln(errw, "rudi-eval: --lap phải ≥ 1")
		return raSai
	}
	if mh.chiBuoc != "" && mh.chiBuoc != aieval.ChiBuocHieu {
		fmt.Fprintf(errw, "rudi-eval: --chi-buoc chỉ nhận hieu (có %q)\n", mh.chiBuoc)
		return raSai
	}
	sha, ban := thongTinBuild()
	if mh.gitSHA != "" {
		sha = mh.gitSHA
	}
	switch mh.cay {
	case "":
	case "sach", "ban":
		b := mh.cay == "ban"
		ban = &b
	default:
		fmt.Fprintf(errw, "rudi-eval: --cay là sach hoặc ban (có %q)\n", mh.cay)
		return raSai
	}
	y := aieval.YeuCauLuot{CheDo: mh.cheDo, ChiBuoc: mh.chiBuoc, BoDuongDan: mh.bo, KichBanDir: mh.kichBan, Lap: mh.lap,
		TranGoi: mh.tranGoi, Goc: mh.out, GitSHA: sha, CayBan: ban, Log: errw}

	if mh.cheDo == aieval.MoHinhPhatLai {
		if mh.bang == "" {
			fmt.Fprintln(errw, "rudi-eval: phat-lai cần --bang <thư mục lượt gốc>")
			return raSai
		}
		goc, err := aieval.DocManifest(mh.bang)
		if err != nil {
			fmt.Fprintf(errw, "rudi-eval: lượt gốc: %v\n", err)
			return raSai
		}
		y.BangTu = mh.bang
		if y.BoDuongDan == "" {
			y.BoDuongDan = goc.Bo.DuongDan
		}
		if mh.chiBuoc == "" {
			y.ChiBuoc = goc.ChiBuoc
		}
		y.Lap = goc.Lap
	}
	if y.BoDuongDan == "" {
		fmt.Fprintln(errw, "rudi-eval: chế độ "+mh.cheDo+" cần --bo")
		return raSai
	}
	if y.KichBanDir == "" {
		y.KichBanDir = filepath.Join(filepath.Dir(y.BoDuongDan), "..", "kich_ban")
	}
	giaPath := mh.gia
	if giaPath == "" {
		giaPath = filepath.Join(filepath.Dir(y.BoDuongDan), "..", "gia-model.json")
	}
	if g, err := aieval.DocGia(giaPath); err == nil {
		y.Gia = g
	} else if mh.gia != "" {
		fmt.Fprintf(errw, "rudi-eval: %v\n", err)
		return raSai
	} else {
		fmt.Fprintf(errw, "rudi-eval: không đọc được %s (%v): chi phí sẽ in «%s»\n", giaPath, err, aieval.ChuaCoGia)
	}

	if mh.cheDo != aieval.MoHinhPhatLai {
		// The estimate, then the ceiling -- both before any client exists.
		du, err := duToan(y, coPhuTuMoiTruong())
		if err != nil {
			fmt.Fprintf(errw, "rudi-eval: %v\n", err)
			return raSai
		}
		raw, _ := json.Marshal(du)
		if mh.duToan {
			fmt.Fprintf(out, "{\"du_toan\":%s}\n", raw)
			return raXanh
		}
		fmt.Fprintf(errw, "dự toán (trần trên chính xác): %d lượt × (%d model + %d nhúng + %d xếp lại) + %d nhúng kho ví dụ = %d lời gọi; kỳ vọng ở mục tiêu p95: %d\n",
			du.SoLuot, du.GoiMoiLuot, du.NhungMoiLuot, du.XepLaiMoiLuot, du.NhungChung, du.Tran, du.KyVongP95)
		if mh.tranGoi < 1 {
			fmt.Fprintf(errw, "rudi-eval: --tran-goi N bắt buộc ở chế độ %s (không có mặc định): số lời gọi Lead đã duyệt\n", mh.cheDo)
			return raSai
		}
		if du.Tran > mh.tranGoi {
			fmt.Fprintf(errw, "rudi-eval: TỪ CHỐI: dự toán %d > trần --tran-goi %d. Xin Lead duyệt ít nhất %d, hoặc chạy ít ca/lần hơn.\n", du.Tran, mh.tranGoi, du.Tran)
			return raSai
		}
		noi, nguon, err := dungNhaCungCap(ctx, mh.cheDo)
		if err != nil {
			fmt.Fprintf(errw, "rudi-eval: %v\n", err)
			return raSai
		}
		y.Noi, y.Nguon = noi, nguon
	}

	kq, err := aieval.ChayLuotMoHinh(ctx, y)
	if err != nil {
		fmt.Fprintf(errw, "rudi-eval: %v\n", err)
		return raSai
	}
	m := kq.Manifest
	fmt.Fprintf(errw, "lượt %s: %s; lời gọi nhà cung cấp %d/%d (model %d, nhúng %d, xếp lại %d), trả từ cassette %d\n",
		m.RunID, m.TrangThai, m.Goi.DaDung, m.Goi.TranDuyet, m.Goi.MoHinh, m.Goi.Nhung, m.Goi.XepLai, m.Goi.TuBang)
	if len(m.LyDo) > 0 {
		fmt.Fprintf(errw, "lý do: %s\n", strings.Join(m.LyDo, "; "))
	}
	fmt.Fprintf(errw, "bảng điểm: %s\n", filepath.Join(kq.Dir, aieval.TepBangDiem))
	_ = json.NewEncoder(out).Encode(map[string]string{"thu_muc": kq.Dir, "trang_thai": m.TrangThai, "run_id": m.RunID})
	if m.TrangThai != aieval.TrangThaiHopLe {
		return raDo
	}
	return raXanh
}

// duToan is the estimate of y's corpus (or router set).
func duToan(y aieval.YeuCauLuot, p aieval.CoPhu) (aieval.DuToan, error) {
	if y.ChiBuoc == aieval.ChiBuocHieu {
		raw, err := os.ReadFile(y.BoDuongDan)
		if err != nil {
			return aieval.DuToan{}, err
		}
		b, err := aieval.DocBoHieu(raw)
		if err != nil {
			return aieval.DuToan{}, err
		}
		return aieval.TinhDuToanHieu(b, p), nil
	}
	kbs, err := aieval.DocKichBan(y.KichBanDir)
	if err != nil {
		return aieval.DuToan{}, err
	}
	b, _, err := aieval.DocBo(y.BoDuongDan, kbs)
	if err != nil {
		return aieval.DuToan{}, err
	}
	return aieval.TinhDuToan(b, y.Lap, p), nil
}
