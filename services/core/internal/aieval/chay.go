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

	"google.golang.org/adk/v2/model"

	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/testkit"
	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/aiharness/truyhoi"
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
	// LoiHaTang marks, in the model modes, a run that ended on the
	// provider's 429 or 5xx after the retries: an infrastructure error,
	// neither passed nor failed (design 06 §5).
	LoiHaTang bool `json:"loi_ha_tang,omitempty"`
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
	l, out, err := chayKichBan(ctx, c, kb, lap)
	if err != nil {
		return KetQuaChay{}, err
	}
	out.Truot = append(append(out.Truot, KiemBatBien(l.LuotDaChay)...), Cham(c.KyVong, l)...)
	return out, nil
}

// chayKichBan runs one script on one case, unscored: the stub, the script's
// length, the clock fixed at the case's instant, no wait between retries.
func chayKichBan(ctx context.Context, c Ca, kb KichBan, lap int) (LuotDaCham, KetQuaChay, error) {
	l, out, err := chayLuot(ctx, c, lap, cachChay{
		moHinh: kb.Stub(maKiemCua(c.CaID)),
		soBuoc: len(kb.Buoc),
		buoc:   kb.Buoc,
		cho:    khongCho,
	})
	out.KichBan = kb.Ten
	return l, out, err
}

// moHinhLuot is the model one turn runs on, which keeps the canonical form
// of every request it was handed: the scripted stub, or the cassette.
type moHinhLuot interface {
	model.LLM
	YeuCau() [][]byte
}

// cachChay is how one turn is run.
type cachChay struct {
	moHinh moHinhLuot
	// soBuoc is how many replies the script holds (kich_ban_lech reads
	// it); the model modes have no script and do not read it.
	soBuoc int
	// buoc is the script's steps (the chặng each request is scored as);
	// nil in the model modes.
	buoc []BuocKichBan
	// dongHo is the clock the engine and the recorder time the turn with.
	// nil fixes it at the case's instant: every duration is zero and the
	// run repeats byte for byte.
	dongHo func() time.Time
	// cho is the wait before model retry n; nil keeps the engine's own.
	cho func(int) time.Duration
	// them are further engine options (the model modes' router with its
	// example bank, their reranker).
	them []aiharness.Option
}

func khongCho(int) time.Duration { return 0 }

// chayLuot runs the turn and gathers what the checks read, unscored.
func chayLuot(ctx context.Context, c Ca, lap int, cc cachChay) (LuotDaCham, KetQuaChay, error) {
	g, err := GieoCa(c, lap)
	if err != nil {
		return LuotDaCham{}, KetQuaChay{}, err
	}
	var nhatKy bytes.Buffer
	dongHo := cc.dongHo
	if dongHo == nil {
		dongHo = func() time.Time { return g.Turn.Luc }
	}
	opts := []aiharness.Option{
		aiharness.WithModel(cc.moHinh),
		aiharness.WithLogger(slog.New(slog.NewJSONHandler(&nhatKy, nil))),
		aiharness.WithMaKiem(g.MaKiem),
		aiharness.WithClock(dongHo),
		aiharness.WithNguon(nguonCua(c.DauVao.TheGioi, g.Turn.Luc)),
	}
	if cc.cho != nil {
		opts = append(opts, aiharness.WithRetryWait(cc.cho))
	}
	opts = append(opts, cc.them...)
	e, err := aiharness.New(opts...)
	if err != nil {
		return LuotDaCham{}, KetQuaChay{}, err
	}
	sink := NewGhiLai(dongHo)
	res, runErr := e.Run(ctx, g.Turn, sink)
	if errors.Is(runErr, aiharness.ErrHuy) {
		// Nothing outside stops a turn but the caller's context: that is
		// the harness failing (or, in a real run, the watchdog), not the
		// turn.
		return LuotDaCham{}, KetQuaChay{}, runErr
	}
	l := LuotDaCham{
		LuotDaChay:  LuotDaChay{Turn: g.Turn, SuKien: sink.SuKien(), KetThuc: res.Record.KetThuc, Chu: res.Text, BanGhi: res.Record, MaKiem: g.MaKiem},
		BuocKichBan: cc.buoc,
		NhatKy:      nhatKy.String(),
		// How many replies the script holds: a turn that asked for more ran
		// off its script.
		SoBuocKichBan: cc.soBuoc,
	}
	if runErr != nil {
		l.Ma = aiharness.MaCua(runErr)
	}
	// Empty lists print as [], never null: a reader of the line should not
	// have to know which fields Go leaves nil.
	out := KetQuaChay{CaID: c.CaID, Lap: lap, SuKien: l.SuKien, YeuCauHash: []string{}, Truot: []Truot{}}
	for _, raw := range cc.moHinh.YeuCau() {
		y, err := DocYeuCau(raw)
		if err != nil {
			return LuotDaCham{}, KetQuaChay{}, fmt.Errorf("đọc lại yêu cầu: %w", err)
		}
		l.YeuCau = append(l.YeuCau, y)
		c := ""
		if i := len(l.YeuCau) - 1; i < len(cc.buoc) {
			c = cc.buoc[i].Chang
		} else if cc.buoc == nil {
			// A model mode: no script, the stage is read off the request.
			c = ChangTuYeuCau(y.SystemInstruction)
		}
		l.Chang = append(l.Chang, c)
		sum := sha256.Sum256(raw)
		out.YeuCauHash = append(out.YeuCauHash, hex.EncodeToString(sum[:]))
	}
	if cc.buoc == nil {
		// A model mode: the answer steps are what the model wrote.
		if d, ok := cc.moHinh.(interface{ DapAn() []string }); ok {
			for i, t := range d.DapAn() {
				t := t
				l.BuocKichBan = append(l.BuocKichBan, BuocKichBan{Chang: changCua(l.LuotDaChay, i), Chu: &t})
			}
		}
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

// nguonCua builds the engine's data ports over a case's world, as the
// in-memory fakes of aiharness/testkit. Every port is set: an empty world
// answers with nothing, never with loi_nguon.
func nguonCua(g *TheGioi, luc time.Time) tools.NguonDuLieu {
	if g == nil {
		g = &TheGioi{}
	}
	bang := func(ms []MucTheGioi, n truyhoi.Nguon) []truyhoi.BangChung {
		var out []truyhoi.BangChung
		for _, m := range ms {
			tr := make(map[string]string, len(m.Truong))
			for k, v := range m.Truong {
				tr[k] = v
			}
			out = append(out, truyhoi.BangChung{ID: m.ID, Nguon: n, Truong: tr})
		}
		return out
	}
	cho := testkit.Cho{DiemDens: bang(g.DiemDen, "")}
	quan := &testkit.TheoLuot{}
	for _, l := range g.TruyHoi {
		kq := truyhoi.KetQuaTruyHoi{BangChung: bang(l.Quan, truyhoi.Places)}
		if len(l.BiLoai) > 0 {
			kq.BiLoai = map[truyhoi.RangBuoc]int{}
			for r, n := range l.BiLoai {
				kq.BiLoai[truyhoi.RangBuoc(r)] = n
			}
		}
		quan.KetQua = append(quan.KetQua, kq)
		cho.Quans = append(cho.Quans, kq.BangChung...)
	}
	tn := testkit.MoiTriNho()
	for _, f := range g.TriNho {
		// A world is checked (TheGioi.kiem) before it is built.
		_, _ = tn.Ghi(context.Background(), g.NguoiHoi, trinho.SuThatMoi{NoiDung: f.NoiDung, Loai: trinho.LoaiSuThat(f.Loai), TuLuc: luc, Nguon: trinho.NoiRo})
	}
	return tools.NguonDuLieu{Quan: quan, Cho: cho, TriNho: tn}
}
