package aieval

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aiharness/obs"
)

// Roles a run plays in a corpus.
const (
	// VaiDung: a case's correct script; it must pass every check.
	VaiDung = "dung"
	// VaiSai: a wrong script; it must fail the check it names.
	VaiSai = "sai"
	// VaiCanary: the canary's correct script; it must fail exactly the checks
	// the case names, and nothing else.
	VaiCanary = "canary"
)

// KetQuaChay is one run of one script on one case, the line the binary
// prints for it (design 06 §3.2). The trace (vet) of §3.2 comes with the
// trace plugin, slice 18.
type KetQuaChay struct {
	CaID       string     `json:"case_id"`
	KichBan    string     `json:"kich_ban"`
	Vai        string     `json:"vai"`
	Lap        int        `json:"lap"`
	SuKien     []SuKien   `json:"su_kien"`
	KetQua     KetQuaLuot `json:"ket_qua"`
	YeuCauHash []string   `json:"yeu_cau_hash"`
	SoGoiModel int        `json:"so_goi_model"`
	Loi        string     `json:"loi,omitempty"`
	// Truot is every failed check, invariants first.
	Truot []Truot `json:"truot"`
	// Dat says the run did what its role asks; LyDo says why not.
	Dat  bool   `json:"dat"`
	LyDo string `json:"ly_do,omitempty"`
}

// KetQuaLuot is how the turn ended.
type KetQuaLuot struct {
	KetThuc string         `json:"ket_thuc"`
	Ma      string         `json:"ma,omitempty"`
	Chu     string         `json:"chu,omitempty"`
	BanGhi  map[string]any `json:"ban_ghi"`
}

// banGhiMap is the record as the log line and the metrics row carry it.
func banGhiMap(r obs.TurnRecord) map[string]any {
	cols, vals := obs.Columns(), r.Values()
	out := make(map[string]any, len(cols))
	for i, c := range cols {
		out[c] = vals[i]
	}
	return out
}

// ChayLuot runs one script on one case through Engine.Run and scores it.
// The clock is fixed at the case's instant: durations are zero and the run
// is repeatable byte for byte, which is what T1 asks of a scripted run.
func ChayLuot(ctx context.Context, c Ca, kb KichBan, lap int) (KetQuaChay, error) {
	l, out, err := chayLuot(ctx, c, kb, lap)
	if err != nil {
		return KetQuaChay{}, err
	}
	out.Truot = append(append(out.Truot, KiemBatBien(l.LuotDaChay)...), Cham(c.KyVong, l)...)
	return out, nil
}

// chayLuot runs the turn and gathers what the checks read, unscored.
func chayLuot(ctx context.Context, c Ca, kb KichBan, lap int) (LuotDaCham, KetQuaChay, error) {
	g, err := GieoCa(c, lap)
	if err != nil {
		return LuotDaCham{}, KetQuaChay{}, err
	}
	stub := kb.Stub(g.MaKiem)
	var nhatKy bytes.Buffer
	dongHo := func() time.Time { return g.Turn.Luc }
	e, err := aiharness.New(
		aiharness.WithModel(stub),
		aiharness.WithLogger(slog.New(slog.NewJSONHandler(&nhatKy, nil))),
		aiharness.WithMaKiem(g.MaKiem),
		aiharness.WithRetryWait(func(int) time.Duration { return 0 }),
		aiharness.WithClock(dongHo),
	)
	if err != nil {
		return LuotDaCham{}, KetQuaChay{}, err
	}
	sink := NewGhiLai(dongHo)
	res, runErr := e.Run(ctx, g.Turn, sink)
	if errors.Is(runErr, aiharness.ErrHuy) {
		// Nothing outside stops a scripted turn but the caller's context:
		// that is the harness failing, not the turn.
		return LuotDaCham{}, KetQuaChay{}, runErr
	}
	l := LuotDaCham{
		LuotDaChay: LuotDaChay{Turn: g.Turn, SuKien: sink.SuKien(), KetThuc: res.Record.KetThuc, Chu: res.Text, BanGhi: res.Record},
		MaKiem:     g.MaKiem,
		NhatKy:     nhatKy.String(),
		// How many replies the script holds: a turn that asked for more ran
		// off its script.
		SoBuocKichBan: len(kb.Buoc),
	}
	if runErr != nil {
		l.Ma = aiharness.MaCua(runErr)
	}
	// Empty lists print as [], never null: a reader of the line should not
	// have to know which fields Go leaves nil.
	out := KetQuaChay{CaID: c.CaID, KichBan: kb.Ten, Lap: lap, SuKien: l.SuKien, YeuCauHash: []string{}, Truot: []Truot{}}
	for _, raw := range stub.YeuCau() {
		y, err := DocYeuCau(raw)
		if err != nil {
			return LuotDaCham{}, KetQuaChay{}, fmt.Errorf("đọc lại yêu cầu: %w", err)
		}
		l.YeuCau = append(l.YeuCau, y)
		sum := sha256.Sum256(raw)
		out.YeuCauHash = append(out.YeuCauHash, hex.EncodeToString(sum[:]))
	}
	bg := banGhiMap(res.Record)
	rawBG, _ := json.Marshal(bg)
	rawSK, _ := json.Marshal(l.SuKien)
	l.BanGhiJSON, l.SuKienJSON = string(rawBG), string(rawSK)
	out.KetQua = KetQuaLuot{KetThuc: string(l.KetThuc), Ma: string(l.Ma), Chu: l.Chu, BanGhi: bg}
	out.SoGoiModel = len(l.YeuCau)
	if err := res.Record.Valid(); err != nil {
		out.Loi = err.Error()
	}
	return l, out, nil
}

// tenTruot is the set of failed check names.
func tenTruot(ts []Truot) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range ts {
		if !seen[t.Kiem] {
			seen[t.Kiem] = true
			out = append(out, t.Kiem)
		}
	}
	sort.Strings(out)
	return out
}

// xet sets Dat and LyDo from the run's role.
func (r *KetQuaChay) xet(vai string, phai []string) {
	r.Vai = vai
	got := tenTruot(r.Truot)
	switch vai {
	case VaiDung:
		r.Dat = len(got) == 0 && r.Loi == ""
		if !r.Dat {
			r.LyDo = "kịch bản đúng trượt: " + strings.Join(got, ", ")
			if r.Loi != "" {
				r.LyDo += "; bản ghi hỏng: " + r.Loi
			}
		}
	case VaiCanary:
		want := append([]string(nil), phai...)
		sort.Strings(want)
		r.Dat = strings.Join(got, ",") == strings.Join(want, ",")
		if !r.Dat {
			r.LyDo = fmt.Sprintf("canary phải đỏ đúng ở %v, đỏ ở %v", want, got)
		}
	case VaiSai:
		for _, g := range got {
			if g == phai[0] {
				r.Dat = true
			}
		}
		if !r.Dat {
			r.LyDo = fmt.Sprintf("kịch bản sai phải trượt %s, trượt ở %v", phai[0], got)
		}
	}
}

// ChayDung runs a case's correct script as run lap and judges it by the
// case's role: the canary's must fail exactly where the case says, any other
// case's must pass.
func ChayDung(ctx context.Context, c Ca, kbs map[string]KichBan, lap int) (KetQuaChay, error) {
	r, err := ChayLuot(ctx, c, kbs[c.KichBan.Dung], lap)
	if err != nil {
		return r, err
	}
	vai := VaiDung
	if c.CanhGac == CanhGacCanary {
		vai = VaiCanary
	}
	r.xet(vai, c.PhaiDoO)
	return r, nil
}

// CanhGac is a sentinel case's outcome.
type CanhGac struct {
	CaID  string   `json:"case_id"`
	CoMat bool     `json:"co_mat"`
	Dat   bool     `json:"dat"`
	Truot []string `json:"truot"`
}

// TongKet is a corpus run's verdict, the binary's last line.
type TongKet struct {
	Bo       string `json:"bo"`
	PhienBan int    `json:"phien_ban"`
	// ShaBo is the sha256 of the corpus file as read.
	ShaBo            string  `json:"sha_bo"`
	MoHinh           string  `json:"mo_hinh"`
	PromptVersionNep string  `json:"prompt_version_nep"`
	SoCa             int     `json:"so_ca"`
	SoLuot           int     `json:"so_luot"`
	Dat              int     `json:"dat"`
	KhongDat         int     `json:"khong_dat"`
	SoSai            int     `json:"so_sai"`
	SaiDat           int     `json:"sai_dat"`
	Canary           CanhGac `json:"canary"`
	DongNhat         CanhGac `json:"dong_nhat"`
	// Xanh: every run did what its role asks, and both sentinels are present
	// and did theirs. There is no skip to count: a case that cannot run stops
	// the whole corpus with an error (ChayBo), which the binary exits 2 on.
	Xanh bool `json:"xanh"`
}

// ChayBo runs every case of a corpus: each correct script once per lap, each
// wrong script once. emit hears each run as it finishes.
func ChayBo(ctx context.Context, b Bo, shaBo string, kbs map[string]KichBan, lap int, emit func(KetQuaChay) error) (TongKet, error) {
	h := DocHang()
	tk := TongKet{Bo: b.Bo, PhienBan: b.PhienBan, ShaBo: shaBo, MoHinh: h.MoHinh, PromptVersionNep: h.PromptVersionNep, SoCa: len(b.Ca)}
	tk.Canary = CanhGac{CaID: CaCanary, Truot: []string{}}
	tk.DongNhat = CanhGac{CaID: CaDongNhat, Truot: []string{}}
	for _, c := range b.Ca {
		for n := 1; n <= lap; n++ {
			r, err := ChayDung(ctx, c, kbs, n)
			if err != nil {
				return tk, fmt.Errorf("ca %s: %w", c.CaID, err)
			}
			tk.dem(r)
			switch c.CanhGac {
			case CanhGacCanary:
				tk.Canary = CanhGac{CaID: c.CaID, CoMat: true, Dat: r.Dat && (n == 1 || tk.Canary.Dat), Truot: tenTruot(r.Truot)}
			case CanhGacDongNhat:
				tk.DongNhat = CanhGac{CaID: c.CaID, CoMat: true, Dat: r.Dat && (n == 1 || tk.DongNhat.Dat), Truot: tenTruot(r.Truot)}
			}
			if err := emit(r); err != nil {
				return tk, err
			}
		}
		for _, s := range c.KichBan.Sai {
			r, err := ChayLuot(ctx, c, kbs[s.KichBan], 1)
			if err != nil {
				return tk, fmt.Errorf("ca %s, kịch bản %s: %w", c.CaID, s.KichBan, err)
			}
			r.xet(VaiSai, []string{s.PhaiTruot})
			tk.dem(r)
			tk.SoSai++
			if r.Dat {
				tk.SaiDat++
			}
			if err := emit(r); err != nil {
				return tk, err
			}
		}
	}
	tk.Xanh = tk.KhongDat == 0 && tk.Canary.CoMat && tk.Canary.Dat && tk.DongNhat.CoMat && tk.DongNhat.Dat
	return tk, nil
}

func (tk *TongKet) dem(r KetQuaChay) {
	tk.SoLuot++
	if r.Dat {
		tk.Dat++
	} else {
		tk.KhongDat++
	}
}
