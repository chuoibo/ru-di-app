package aieval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"google.golang.org/adk/v2/model"

	"mobile/services/core/internal/aiharness/hieu"
)

// One model-mode run from start to evidence (design 06 §8.1, §8.2): the
// binary builds the provider doors (only in `that` and `ghi`, in the one
// file of cmd/rudi-eval allowed to), and everything after that lives here,
// where the tests drive it against loopback stand-ins. A run:
//
//  1. puts every provider door under the run's ceiling (TranGoi), whose
//     watchdog cancels the run when the call that reaches N comes back;
//  2. wraps the doors in a recording cassette (that, ghi), or opens the
//     cassette of an earlier run with nothing behind it (phat-lai);
//  3. runs the corpus (or, with ChiBuoc "hieu", a router set);
//  4. writes the run directory: vet.jsonl, cham.jsonl, bang-ghi.json,
//     manifest.json, bang-diem.md, and, for a replay that reproduced a
//     valid real run at the same SHA with no call, trailer.txt.

// ChiBuocHieu is the router-only mode (--chi-buoc hieu).
const ChiBuocHieu = "hieu"

// YeuCauLuot is one model-mode run.
type YeuCauLuot struct {
	CheDo   string
	ChiBuoc string
	// BoDuongDan is the corpus (or router set) file; KichBanDir the scripts
	// the corpus names (read to check it, never to answer).
	BoDuongDan string
	KichBanDir string
	Lap        int
	// TranGoi is `--tran-goi`: required in that and ghi, ignored in
	// phat-lai (nothing can be called).
	TranGoi int
	// Goc is the evidence root (default ~/ThuMucGocTuongDoi).
	Goc    string
	GitSHA string
	CayBan *bool
	Gia    *BangGia
	// Noi are the provider doors (that, ghi), unwrapped; Nguon says what
	// answers behind them (NguonGeminiAPI, NguonLoopback).
	Noi   *NoiGhi
	Nguon string
	// BangTu is the run directory whose cassette a replay serves.
	BangTu string
	// Cho is the wait before model retry n; nil keeps the engine's (a
	// replay always waits 0: waiting changes no request).
	Cho func(int) time.Duration
	// TruocCa hears each case before it runs (tests).
	TruocCa func(c Ca, lap int)
	// Log hears the progress lines, in Vietnamese.
	Log io.Writer
}

// KetQuaLuotMoHinh is what a run left.
type KetQuaLuotMoHinh struct {
	Dir      string
	Manifest Manifest
}

func (y YeuCauLuot) log(f string, a ...any) {
	if y.Log != nil {
		fmt.Fprintf(y.Log, f+"\n", a...)
	}
}

// CoPhuCua is which doors a run has, from its request: in phat-lai, the
// doors its cassette recorded.
func CoPhuCua(n *NoiGhi) CoPhu {
	if n == nil {
		return CoPhu{}
	}
	return CoPhu{Nhung: n.Nhung != nil, SoViDu: len(hieu.ViDuMacDinh), XepLai: n.XepLai != nil}
}

// ChayLuotMoHinh runs y to its evidence. The error is for a run that could
// not be made (a bad input, an unwritable directory); a run that went badly
// -- unfinished, void, red -- is reported in the manifest, which the caller
// reads for its exit code.
func ChayLuotMoHinh(ctx context.Context, y YeuCauLuot) (KetQuaLuotMoHinh, error) {
	batDau := time.Now()
	if y.Lap < 1 {
		y.Lap = 1
	}
	var tran *TranGoi
	var bang *Bang
	var goc Manifest
	var bangSha string
	ctx, huy := context.WithCancel(ctx)
	defer huy()
	dongHo := time.Now
	switch y.CheDo {
	case MoHinhThat, MoHinhGhi:
		if y.Noi == nil || y.Noi.MoHinh == nil {
			return KetQuaLuotMoHinh{}, errors.New("chế độ " + y.CheDo + " cần mô hình")
		}
		if y.TranGoi < 1 {
			return KetQuaLuotMoHinh{}, errors.New("--tran-goi bắt buộc ở chế độ " + y.CheDo)
		}
		tran = NewTranGoi(y.TranGoi, huy)
		noi := NoiGhi{MoHinh: tran.BocLLM(y.Noi.MoHinh), XepLaiModel: y.Noi.XepLaiModel}
		if y.Noi.Nhung != nil {
			noi.Nhung = tran.BocNhung(y.Noi.Nhung)
		}
		if y.Noi.XepLai != nil {
			noi.XepLai = tran.BocXepLai(y.Noi.XepLai)
		}
		bang = NewBangGhi(noi, time.Now)
	case MoHinhPhatLai:
		if y.BangTu == "" {
			return KetQuaLuotMoHinh{}, errors.New("phat-lai cần thư mục lượt gốc (--bang)")
		}
		var err error
		if goc, err = DocManifest(y.BangTu); err != nil {
			return KetQuaLuotMoHinh{}, fmt.Errorf("lượt gốc: %w", err)
		}
		t, sha, err := DocBang(filepath.Join(y.BangTu, TepBangGhi))
		if err != nil {
			return KetQuaLuotMoHinh{}, fmt.Errorf("cassette lượt gốc: %w", err)
		}
		if bang, err = NewBangPhatLai(t); err != nil {
			return KetQuaLuotMoHinh{}, err
		}
		bangSha = sha
		if goc.ChiBuoc != y.ChiBuoc {
			return KetQuaLuotMoHinh{}, fmt.Errorf("lượt gốc chạy --chi-buoc %q, lượt này %q", goc.ChiBuoc, y.ChiBuoc)
		}
		if goc.Lap != y.Lap {
			y.log("lap theo lượt gốc: %d", goc.Lap)
			y.Lap = goc.Lap
		}
		y.Cho = func(int) time.Duration { return 0 }
		dongHo = nil
	default:
		return KetQuaLuotMoHinh{}, fmt.Errorf("chế độ %q không phải that, ghi hay phat-lai", y.CheDo)
	}
	rawBo, err := os.ReadFile(y.BoDuongDan)
	if err != nil {
		return KetQuaLuotMoHinh{}, err
	}
	shaBo := ShaCua(rawBo)
	if y.CheDo == MoHinhPhatLai && goc.Bo.Sha != shaBo {
		return KetQuaLuotMoHinh{}, fmt.Errorf("bộ %s có sha %s, lượt gốc chạy sha %s: phát lại phải trên đúng bộ đó", y.BoDuongDan, shaBo[:12], goc.Bo.Sha[:min(12, len(goc.Bo.Sha))])
	}
	if y.Goc == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return KetQuaLuotMoHinh{}, err
		}
		y.Goc = filepath.Join(home, ThuMucGocTuongDoi)
	}
	dir, runID, err := TaoThuMucLuot(y.Goc, RunIDCua(batDau, y.CheDo, y.GitSHA))
	if err != nil {
		return KetQuaLuotMoHinh{}, err
	}
	y.log("thư mục lượt: %s", dir)
	vet, err := os.OpenFile(filepath.Join(dir, TepVet), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return KetQuaLuotMoHinh{}, err
	}
	defer vet.Close()
	enc := json.NewEncoder(vet)
	enc.SetEscapeHTML(false)

	m := Manifest{RunID: runID, GitSHA: y.GitSHA, CayBan: y.CayBan, MoHinh: y.CheDo, ChiBuoc: y.ChiBuoc,
		Nguon: y.Nguon, Lap: y.Lap, BatDau: batDau.UTC().Format(time.RFC3339), ChuaDo: ChuaDoMacDinh, LyDo: []string{}}
	if y.CheDo == MoHinhPhatLai {
		m.Nguon = "bang:" + goc.RunID
	}
	m.Bo = BoManifest{DuongDan: y.BoDuongDan, Sha: shaBo}
	m.Model = bang.tep.Model
	m.NhungModel, m.XepLaiModel = bang.tep.NhungModel, bang.tep.XepLaiModel
	coPhu := CoPhu{Nhung: bang.tep.NhungModel != "", SoViDu: len(hieu.ViDuMacDinh), XepLai: bang.tep.XepLaiModel != ""}

	var cham []byte
	var soLuot int
	switch y.ChiBuoc {
	case "":
		kbs, err := DocKichBan(y.KichBanDir)
		if err != nil {
			return KetQuaLuotMoHinh{}, err
		}
		b, _, err := DocBo(y.BoDuongDan, kbs)
		if err != nil {
			return KetQuaLuotMoHinh{}, err
		}
		m.Bo.Ten, m.Bo.PhienBan = b.Bo, b.PhienBan
		m.Goi.DuToan = TinhDuToan(b, y.Lap, coPhu)
		cfg := CauHinhMoHinh{CheDo: y.CheDo, Bang: bang, Tran: tran, DongHo: dongHo, Cho: y.Cho, Lap: y.Lap, TruocCa: y.TruocCa}
		tk, rs, err := ChayBoMoHinh(ctx, b, shaBo, cfg, func(r KetQuaChay) error {
			if !r.Dat && !r.LoiHaTang {
				y.log("ĐỎ %s lần %d: %s", r.CaID, r.Lap, strings.Join(tenTruot(r.Truot), ", "))
			}
			return enc.Encode(r)
		})
		if err != nil {
			return KetQuaLuotMoHinh{}, err
		}
		m.PromptVersionNep = tk.PromptVersionNep
		if cham, err = MaHoaCham(rs); err != nil {
			return KetQuaLuotMoHinh{}, err
		}
		sd, vs, err := TinhSoDo(rs, bang.Tep(), dongHo != nil, y.Gia)
		if err != nil {
			return KetQuaLuotMoHinh{}, err
		}
		m.TongKet, m.SoDo, m.ModelVersions = tk, sd, vs
		soLuot = tk.SoLuot
		m.LyDo = append(m.LyDo, tk.LyDo...)
		switch {
		case tk.ChuaXong:
			m.TrangThai = TrangThaiChuaXong
		case !tk.HopLe:
			m.TrangThai = TrangThaiVoHieu
		default:
			m.TrangThai = TrangThaiHopLe
		}
	case ChiBuocHieu:
		b, err := DocBoHieu(rawBo)
		if err != nil {
			return KetQuaLuotMoHinh{}, err
		}
		m.Bo.Ten, m.Bo.PhienBan = b.Bo, b.PhienBan
		m.Lap = 1
		m.Goi.DuToan = TinhDuToanHieu(b, coPhu)
		kq, err := ChayBoHieuMoHinh(ctx, b, bang, tran, func(r KetQuaHieu) error {
			if !r.Dat {
				y.log("ĐỎ %s: %s", r.ID, strings.Join(tenTruotHieu(r.Truot), ", "))
			}
			return enc.Encode(r)
		})
		if err != nil {
			return KetQuaLuotMoHinh{}, err
		}
		cham = kq.Cham
		m.Hieu = &kq.TongKet
		m.PromptVersionNep = kq.TongKet.PhienBan
		sd, vs, err := TinhSoDo(nil, bang.Tep(), dongHo != nil, y.Gia)
		if err != nil {
			return KetQuaLuotMoHinh{}, err
		}
		sd.ChiPhi = TinhChiPhiBang(y.Gia, bang.Tep(), kq.SoCaXong)
		m.SoDo, m.ModelVersions = sd, vs
		soLuot = kq.SoCaXong
		m.LyDo = append(m.LyDo, kq.LyDo...)
		switch {
		case kq.ChuaXong:
			m.TrangThai = TrangThaiChuaXong
		case len(kq.LyDo) > 0:
			m.TrangThai = TrangThaiVoHieu
		default:
			m.TrangThai = TrangThaiHopLe
		}
	default:
		return KetQuaLuotMoHinh{}, fmt.Errorf("--chi-buoc %q: chỉ nhận hieu", y.ChiBuoc)
	}
	if y.ChiBuoc == "" && soLuot > 0 && m.SoDo.ChiPhi.DonVi == "" {
		m.SoDo.ChiPhi = TinhChiPhiBang(y.Gia, bang.Tep(), soLuot)
	}

	// Calls, from the ceiling's own count (the provider side), and what
	// the cassette answered.
	if tran != nil {
		m.Goi.TranDuyet = tran.Max()
		m.Goi.DaDung, m.Goi.MoHinh, m.Goi.Nhung, m.Goi.XepLai = tran.DaDung(), tran.MoHinh(), tran.Nhung(), tran.XepLai()
		if tran.DaCham() && m.TrangThai == TrangThaiHopLe {
			m.TrangThai = TrangThaiChuaXong
			m.LyDo = append(m.LyDo, fmt.Sprintf("chạm trần %d lời gọi", tran.Max()))
		}
	}
	m.Goi.TuBang = bang.SoTuBang()
	if y.CheDo == MoHinhPhatLai && bang.SoGoiRa() != 0 {
		// Unreachable by construction (a replay cassette has nothing to
		// forward to); said rather than assumed.
		m.TrangThai = TrangThaiVoHieu
		m.LyDo = append(m.LyDo, fmt.Sprintf("phát lại đã gọi ra %d lần", bang.SoGoiRa()))
	}
	m.ChamSha = ShaCua(cham)
	if err := GhiTep(dir, TepCham, cham); err != nil {
		return KetQuaLuotMoHinh{}, err
	}
	if y.CheDo != MoHinhPhatLai {
		raw, err := MaHoaBang(bang.Tep())
		if err != nil {
			return KetQuaLuotMoHinh{}, err
		}
		if err := GhiTep(dir, TepBangGhi, raw); err != nil {
			return KetQuaLuotMoHinh{}, err
		}
		bangSha = ShaCua(raw)
	}
	m.BangSha = bangSha
	if y.CheDo == MoHinhPhatLai {
		gocCham, err := os.ReadFile(filepath.Join(y.BangTu, TepCham))
		if err != nil {
			return KetQuaLuotMoHinh{}, fmt.Errorf("điểm lượt gốc: %w", err)
		}
		trung, khac := SoSanhCham(gocCham, cham)
		m.SoVoiNguon = &SoVoi{RunID: goc.RunID, ChamSha: ShaCua(gocCham), Trung: trung, Khac: khac}
		if !trung && m.TrangThai == TrangThaiHopLe {
			m.TrangThai = TrangThaiVoHieu
			m.LyDo = append(m.LyDo, "điểm khác lượt gốc")
		}
		if goc.GitSHA != "" && goc.GitSHA != y.GitSHA {
			m.LyDo = append(m.LyDo, fmt.Sprintf("SHA khác lượt gốc (%s): phép so T2, không in trailer", short(goc.GitSHA)))
		}
	}
	m.KetThuc = time.Now().UTC().Format(time.RFC3339)
	raw, err := MaHoaManifest(m)
	if err != nil {
		return KetQuaLuotMoHinh{}, err
	}
	if err := GhiTep(dir, TepManifest, raw); err != nil {
		return KetQuaLuotMoHinh{}, err
	}
	if err := GhiTep(dir, TepBangDiem, []byte(BangDiem(m))); err != nil {
		return KetQuaLuotMoHinh{}, err
	}
	if y.CheDo == MoHinhPhatLai {
		if tr, ok := Trailer(goc, m); ok {
			if err := GhiTep(dir, TepTrailer, []byte(tr)); err != nil {
				return KetQuaLuotMoHinh{}, err
			}
		}
	}
	return KetQuaLuotMoHinh{Dir: dir, Manifest: m}, nil
}

func short(s string) string { return s[:min(12, len(s))] }

// TinhChiPhiBang prices every recorded model call of t.
func TinhChiPhiBang(g *BangGia, t TepBang, soLuot int) ChiPhi {
	var tok Token
	for _, goi := range t.Goi {
		x, _, _ := TokenCuaGoi(goi)
		tok.Cong(x)
	}
	if g == nil {
		return ChiPhi{Tong: ChuaCoGia, MoiLuot: ChuaCoGia, ThieuGia: []string{"không có gia-model.json"}}
	}
	return TinhChiPhi(g, t.Model, tok, soLuot)
}

// KetQuaBoHieuMoHinh is a router set's model-mode run.
type KetQuaBoHieuMoHinh struct {
	TongKet  TongKetHieu
	Cham     []byte
	SoCaXong int
	ChuaXong bool
	LyDo     []string
}

// DongChamHieu is one router case's grade line: closed labels only.
type DongChamHieu struct {
	ID    string   `json:"id"`
	Dat   bool     `json:"dat"`
	SoGoi int      `json:"so_goi"`
	Truot []string `json:"truot"`
}

func tenTruotHieu(ts []TruotHieu) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range ts {
		if !seen[t.Kiem] {
			seen[t.Kiem] = true
			out = append(out, t.Kiem)
		}
	}
	sort.Strings(out)
	return out
}

// ChayBoHieuMoHinh runs a router set on the cassette's model, one case at a
// time so each case has its own scope and so the ceiling stops the set
// between two cases, never inside one. The router gets the example bank
// when the cassette has an embedder (production's router does).
func ChayBoHieuMoHinh(ctx context.Context, b BoHieu, bang *Bang, tran *TranGoi, moiCa func(KetQuaHieu) error) (KetQuaBoHieuMoHinh, error) {
	var out KetQuaBoHieuMoHinh
	out.TongKet = TongKetHieu{Bo: b.Bo, TruotTheo: map[string]int{}}
	kho, err := KhoViDuCua(ctx, bang)
	if err != nil {
		if tran != nil && tran.DaCham() {
			out.ChuaXong = true
			out.LyDo = append(out.LyDo, "chạm trần khi dựng kho ví dụ")
			return out, nil
		}
		return out, err
	}
	var opts []hieu.TuyChon
	if kho != nil {
		opts = append(opts, hieu.WithViDu(kho))
	}
	router := hieu.Moi(opts...)
	var cham bytes.Buffer
	enc := json.NewEncoder(&cham)
	enc.SetEscapeHTML(false)
	var loiMoCa error
	for _, c := range b.Ca {
		if tran != nil && tran.DaCham() {
			out.ChuaXong = true
			out.LyDo = append(out.LyDo, fmt.Sprintf("chạm trần %d lời gọi trước ca %s", tran.Max(), c.ID))
			break
		}
		if ctx.Err() != nil {
			out.ChuaXong = true
			out.LyDo = append(out.LyDo, "bị dừng từ bên ngoài trước ca "+c.ID)
			break
		}
		mot := b
		mot.Ca = []CaHieu{c}
		var r KetQuaHieu
		ctxCa := VoiPham(ctx, PhamCua(c.ID, 1))
		tk := ChayBoHieu(ctxCa, router, mot, func(c CaHieu) model.LLM { return bang.LLM(c.ID, 1) }, func(x KetQuaHieu) { r = x })
		if tran != nil && tran.DaCham() && !r.Dat {
			// The watchdog cut this case short: not a case of the run.
			out.ChuaXong = true
			out.LyDo = append(out.LyDo, fmt.Sprintf("chạm trần %d lời gọi giữa ca %s", tran.Max(), c.ID))
			break
		}
		gopHieu(&out.TongKet, tk)
		out.SoCaXong++
		if err := enc.Encode(DongChamHieu{ID: r.ID, Dat: r.Dat, SoGoi: r.SoGoi, Truot: tenTruotHieu(r.Truot)}); err != nil {
			return out, err
		}
		if err := moiCa(r); err != nil && loiMoCa == nil {
			loiMoCa = err
		}
	}
	out.Cham = cham.Bytes()
	if lech := bang.Lech(); len(lech) > 0 {
		out.LyDo = append(out.LyDo, fmt.Sprintf("bang_lech: %d", len(lech)))
	}
	if bang.CheDo() == CheDoPhatLai && !out.ChuaXong {
		if cd := bang.ChuaDung(); len(cd) > 0 {
			out.LyDo = append(out.LyDo, fmt.Sprintf("bang_lech: %d bản ghi không ca nào hỏi", len(cd)))
		}
	}
	if out.SoCaXong == 0 {
		out.LyDo = append(out.LyDo, "không ca nào chạy xong")
	}
	return out, loiMoCa
}

// gopHieu adds one case's tally to the set's.
func gopHieu(a *TongKetHieu, b TongKetHieu) {
	a.PhienBan = b.PhienBan
	a.SoCa += b.SoCa
	a.Dat += b.Dat
	a.LoiHieu += b.LoiHieu
	a.Tien.TP += b.Tien.TP
	a.Tien.FP += b.Tien.FP
	a.Tien.FN += b.Tien.FN
	a.Tien.TN += b.Tien.TN
	a.DiUngThieu += b.DiUngThieu
	a.DiUngThua += b.DiUngThua
	a.AnKiengSai += b.AnKiengSai
	for k, v := range b.TruotTheo {
		a.TruotTheo[k] += v
	}
	a.TongGoi += b.TongGoi
	a.MaxGoi = max(a.MaxGoi, b.MaxGoi)
	a.CaVuotTran += b.CaVuotTran
}

// Wilson is a proportion k/n with its 95% Wilson score interval
// (z = 1.96). Statistics, not money: floats are fine here.
type Wilson struct {
	K  int     `json:"k"`
	N  int     `json:"n"`
	P  float64 `json:"p"`
	Lo float64 `json:"lo"`
	Hi float64 `json:"hi"`
}

// WilsonCua is k/n with its interval; n = 0 is all zeros.
func WilsonCua(k, n int) Wilson {
	if n == 0 {
		return Wilson{}
	}
	const z = 1.96
	p := float64(k) / float64(n)
	nn := float64(n)
	d := 1 + z*z/nn
	c := (p + z*z/(2*nn)) / d
	h := z * math.Sqrt(p*(1-p)/nn+z*z/(4*nn*nn)) / d
	return Wilson{K: k, N: n, P: p, Lo: math.Max(0, c-h), Hi: math.Min(1, c+h)}
}

func (w Wilson) String() string {
	if w.N == 0 {
		return "— (n = 0)"
	}
	return strings.ReplaceAll(fmt.Sprintf("%d/%d = %.3f (KTC 95%% Wilson %.3f–%.3f)", w.K, w.N, w.P, w.Lo, w.Hi), ".", ",")
}

// ChiSoTien is the router's money label on a set: recall of the refused
// class (money_action or split_draft, which Nếp refuses alike) and the
// false refusals of the rest.
type ChiSoTien struct {
	Recall    Wilson `json:"recall"`
	TuChoiSai Wilson `json:"tu_choi_sai"`
}

// ChiSoTienCua reads the money metrics off a set's tally.
func ChiSoTienCua(t TongKetHieu) ChiSoTien {
	return ChiSoTien{Recall: WilsonCua(t.Tien.TP, t.Tien.TP+t.Tien.FN), TuChoiSai: WilsonCua(t.Tien.FP, t.Tien.FP+t.Tien.TN)}
}
