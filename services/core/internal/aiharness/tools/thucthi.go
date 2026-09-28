package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"

	"mobile/services/core/internal/aiharness/guard"
	"mobile/services/core/internal/aiharness/hieu"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/domain/areas"
	"mobile/services/core/internal/domain/nepphieu"
	"mobile/services/core/internal/huongdan"
)

// congCu is one tool's implementation: a strict decode and the turn's
// membership checks (kiem), the run (chay), and its ADK functiontool (moi).
type congCu struct {
	kiem func(bc *BoiCanh, raw []byte) (any, *loiTS)
	chay func(ctx context.Context, bc *BoiCanh, a any) (ketQuaTho, error)
	moi  func(bc *BoiCanh) (tool.Tool, error)
}

// Default k of each reading tool when the model gives none.
const (
	kQuanMacDinh     = 8
	kNhoMacDinh      = 5
	kChuyenMacDinh   = 5
	kAnhNhomChuyenDi = 3
)

// dk registers a tool whose arguments decode into A.
func dk[A any](t Ten, kiem func(*BoiCanh, *A) *loiTS, chay func(context.Context, *BoiCanh, *A) (ketQuaTho, error)) congCu {
	var cc congCu
	cc.kiem = func(bc *BoiCanh, raw []byte) (any, *loiTS) {
		a := new(A)
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(a); err != nil {
			return nil, thamSoSai("", "arguments must match the declared schema")
		}
		if kiem != nil {
			if l := kiem(bc, a); l != nil {
				return nil, l
			}
		}
		return a, nil
	}
	cc.chay = func(ctx context.Context, bc *BoiCanh, a any) (ketQuaTho, error) {
		return chay(ctx, bc, a.(*A))
	}
	cc.moi = func(bc *BoiCanh) (tool.Tool, error) {
		r, _ := LuocDoJSON(t)
		return functiontool.New(functiontool.Config{
			Name:        string(t),
			Description: MoTaDay(t),
			InputSchema: r.Schema(),
		}, func(tc agent.Context, a A) (map[string]any, error) {
			// The arguments already passed truoc (BeforeTool). They are
			// checked once more here because the check also resolves
			// aliases and merges constraints into fields ADK's decode
			// cannot fill; then the tool runs and its raw result is
			// parked for AfterTool to record and render.
			raw, err := json.Marshal(a)
			if err != nil {
				return nil, err
			}
			v, l := cc.kiem(bc, raw)
			if l != nil {
				return nil, l
			}
			kq, err := bc.chay(tc, cc, v)
			if err != nil {
				return nil, err
			}
			bc.gac(tc.FunctionCallID(), t, kq)
			return map[string]any{"cho_ghi": true}, nil
		})
	}
	return cc
}

// ---- search_places --------------------------------------------------------

type thamSoTim struct {
	TruyVan     string   `json:"truy_van"`
	DiemDenID   string   `json:"diem_den_id,omitempty"`
	DiUng       []string `json:"di_ung,omitempty"`
	AnKieng     []string `json:"an_kieng,omitempty"`
	NgayISO     string   `json:"ngay_iso,omitempty"`
	Gio         string   `json:"gio,omitempty"`
	NganSachVND *int64   `json:"ngan_sach_vnd,omitempty"`
	LoaiCho     []string `json:"loai_cho,omitempty"`
	KhiChat     []string `json:"khi_chat,omitempty"`
	KhuVuc      string   `json:"khu_vuc,omitempty"`
	K           *int     `json:"k,omitempty"`

	// Filled by kiem: the merged request.
	cung truyhoi.Cung
	mem  truyhoi.Mem
}

// HopRangBuoc merges a search's own arguments into the router's
// constraints. Hard: stricter wins (truyhoi.Cung.HopChat), so an argument
// can add an allergen, a diet, a lower budget, a destination or an open
// instant the router left empty, and can never remove or loosen one the
// router set; a destination or an instant that disagrees with the router's
// is refused. Soft: the model's lists are added to the router's, and its
// area replaces the router's (soft only ranks).
func HopRangBuoc(rc truyhoi.Cung, rm truyhoi.Mem, tc truyhoi.Cung, tm truyhoi.Mem) (truyhoi.Cung, truyhoi.Mem, error) {
	c, err := rc.HopChat(tc)
	if err != nil {
		return truyhoi.Cung{}, truyhoi.Mem{}, err
	}
	m := truyhoi.Mem{LoaiCho: hopMem(rm.LoaiCho, tm.LoaiCho), KhiChat: hopMem(rm.KhiChat, tm.KhiChat), KhuVuc: rm.KhuVuc}
	if tm.KhuVuc != "" {
		m.KhuVuc = tm.KhuVuc
	}
	return c, m, nil
}

func hopMem(a, b []string) []string {
	var out []string
	for _, s := range append(append([]string(nil), a...), b...) {
		if !coTrong(out, s) {
			out = append(out, s)
		}
	}
	return out
}

func kiemTim(bc *BoiCanh, a *thamSoTim) *loiTS {
	if a.DiemDenID != "" && !coTrong(bc.diemDen(), a.DiemDenID) {
		return thamSoSai("diem_den_id", "must be a destination id offered this turn")
	}
	if a.NgayISO != "" && !hieu.NgayHopLe(a.NgayISO) {
		return thamSoSai("ngay_iso", "must be a calendar date YYYY-MM-DD")
	}
	if a.Gio != "" && !hieu.GioHopLe(a.Gio) {
		return thamSoSai("gio", "must be HH:MM")
	}
	if a.KhuVuc != "" {
		if _, ok := areas.Find(a.KhuVuc); !ok {
			return thamSoSai("khu_vuc", "must be an area id from nearest_area")
		}
	}
	tc := truyhoi.Cung{DiemDenID: a.DiemDenID, DiUng: a.DiUng, AnKieng: a.AnKieng, NganSachVND: a.NganSachVND}
	if a.Gio != "" {
		t, err := moLuc(a.NgayISO, a.Gio, bc.Luc)
		if err != nil {
			return thamSoSai("gio", "must be HH:MM on a valid date")
		}
		tc.MoLuc = &t
	}
	c, m, err := HopRangBuoc(bc.Cung, bc.Mem, tc, truyhoi.Mem{LoaiCho: a.LoaiCho, KhiChat: a.KhiChat, KhuVuc: a.KhuVuc})
	if err != nil {
		return thamSoSai("diem_den_id", "must agree with the destination and time the person already stated")
	}
	a.cung, a.mem = c, m
	return nil
}

func chayTim(ctx context.Context, bc *BoiCanh, a *thamSoTim) (ketQuaTho, error) {
	if bc.Nguon.Quan == nil {
		return ketQuaTho{}, &loiTS{loi: LoiNguon}
	}
	k := kQuanMacDinh
	if a.K != nil {
		k = *a.K
	}
	// A search text that is one of the router's queries goes with the
	// router's diacritics-restored form of it; the model's own text has
	// none (restoring marks is the router's writing, never Go's).
	y := truyhoi.YeuCau{Nguon: truyhoi.Places, Cau: a.TruyVan, CauCoDau: bc.CoDauCua(a.TruyVan), Cung: a.cung, Mem: a.mem, K: k}
	kq, err := bc.Nguon.Quan.Tim(ctx, y)
	if err != nil {
		return ketQuaTho{}, err
	}
	var bc2 []truyhoi.BangChung
	for _, b := range kq.BangChung {
		if b.Nguon == truyhoi.Places && len(bc2) < k {
			bc2 = append(bc2, b)
		}
	}
	them := map[string]any{"rang_buoc_cung": moTaCung(a.cung)}
	if len(kq.BiLoai) > 0 {
		bl := map[string]int{}
		for r, n := range kq.BiLoai {
			bl[string(r)] = n
		}
		them["bi_loai"] = bl
	}
	if len(kq.Degraded) > 0 {
		var d []string
		for _, f := range kq.Degraded {
			d = append(d, string(f))
		}
		them["suy_giam"] = d
	}
	if bc.DiUngNgoaiDanhMuc {
		them["di_ung_ngoai_danh_muc"] = true
	}
	return ketQuaTho{bangChung: bc2, them: them, coDuLieu: true}, nil
}

// moTaCung is the applied hard constraints as ids, an ISO instant and an
// integer: what the model may tell the person the search respected.
func moTaCung(c truyhoi.Cung) map[string]any {
	out := map[string]any{}
	if c.DiemDenID != "" {
		out["diem_den_id"] = c.DiemDenID
	}
	if len(c.DiUng) > 0 {
		out["di_ung"] = sapXep(c.DiUng)
	}
	if len(c.AnKieng) > 0 {
		out["an_kieng"] = sapXep(c.AnKieng)
	}
	if c.MoLuc != nil {
		out["mo_luc"] = c.MoLuc.In(gioVN).Format("2006-01-02T15:04")
	}
	if c.MoTrong != nil {
		out["mo_trong"] = []string{c.MoTrong.Tu.In(gioVN).Format("2006-01-02T15:04"), c.MoTrong.Den.In(gioVN).Format("2006-01-02T15:04")}
	}
	if c.NganSachVND != nil {
		out["ngan_sach_vnd"] = *c.NganSachVND
	}
	return out
}

func (bc *BoiCanh) diemDen() []string {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return append([]string(nil), bc.DiemDen...)
}

// ---- get_place, list_destinations, nearest_area ---------------------------

type thamSoQuan struct {
	ID string `json:"id"`
	id string
}

func kiemQuan(bc *BoiCanh, a *thamSoQuan) *loiTS {
	if id, ok := bc.giaiBiDanh(a.ID, truyhoi.Places); ok {
		a.id = id
		return nil
	}
	if id, ok := bc.giaiThamChieu(a.ID); ok {
		a.id = id
		return nil
	}
	return thamSoSai("id", "must be a place alias (p1…) or an earlier-turn alias (t1…) you were shown")
}

func chayQuan(ctx context.Context, bc *BoiCanh, a *thamSoQuan) (ketQuaTho, error) {
	if bc.Nguon.Cho == nil {
		return ketQuaTho{}, &loiTS{loi: LoiNguon}
	}
	bs, err := bc.Nguon.Cho.Quan(ctx, []string{a.id})
	if err != nil {
		return ketQuaTho{}, err
	}
	var out []truyhoi.BangChung
	for _, b := range bs {
		if b.ID == a.id {
			b.Nguon = truyhoi.Places
			out = append(out, b)
		}
	}
	if len(out) == 0 {
		return ketQuaTho{}, errKhongThay
	}
	return ketQuaTho{bangChung: out}, nil
}

type khongThamSo struct{}

func chayDiemDen(ctx context.Context, bc *BoiCanh, _ *khongThamSo) (ketQuaTho, error) {
	if bc.Nguon.Cho == nil {
		return ketQuaTho{}, &loiTS{loi: LoiNguon}
	}
	ds, err := bc.Nguon.Cho.DiemDen(ctx)
	if err != nil {
		return ketQuaTho{}, err
	}
	hang := []map[string]string{}
	bc.mu.Lock()
	for _, d := range ds {
		hang = append(hang, map[string]string{"id": d.ID, "ten": d.Truong["ten"]})
		// The catalogue's own ids join the turn's closed list: the model
		// may now pick one as diem_den_id.
		if !coTrong(bc.DiemDen, d.ID) {
			bc.DiemDen = append(bc.DiemDen, d.ID)
		}
	}
	bc.mu.Unlock()
	return ketQuaTho{hang: hang}, nil
}

type thamSoKhuVuc struct {
	DiemDenID string `json:"diem_den_id"`
	MoTa      string `json:"mo_ta"`
}

func kiemKhuVuc(bc *BoiCanh, a *thamSoKhuVuc) *loiTS {
	if !coTrong(bc.diemDen(), a.DiemDenID) {
		return thamSoSai("diem_den_id", "must be a destination id offered this turn")
	}
	return nil
}

// chayKhuVuc lists the destination's areas; the MODEL picks the one the
// description means. mo_ta is not read by Go: matching a description to an
// area by its words would be a keyword reader.
func chayKhuVuc(ctx context.Context, bc *BoiCanh, a *thamSoKhuVuc) (ketQuaTho, error) {
	if bc.Nguon.Cho == nil {
		return ketQuaTho{}, &loiTS{loi: LoiNguon}
	}
	ks, err := bc.Nguon.Cho.KhuVuc(ctx, a.DiemDenID)
	if err != nil {
		return ketQuaTho{}, err
	}
	hang := []map[string]string{}
	for _, k := range ks {
		hang = append(hang, map[string]string{"id": k.ID, "ten": k.Truong["ten"]})
	}
	return ketQuaTho{hang: hang}, nil
}

// ---- manual ---------------------------------------------------------------

type thamSoSoTay struct {
	TruyVan string `json:"truy_van"`
	K       *int   `json:"k,omitempty"`
}

// bangChungDoan is one manual section as evidence. nhan_nut is its first
// quoted label, the field the grounding check reads; cac_nhan lists them all.
func bangChungDoan(d huongdan.Doan) truyhoi.BangChung {
	t := map[string]string{"man": d.Man, "tieu_de_man": d.TieuDeMan, "tieu_de": d.TieuDe}
	if len(d.Buoc) > 0 {
		t["buoc"] = strings.Join(d.Buoc, "\n")
	} else {
		t["chu"] = d.Chu
	}
	if len(d.Nhan) > 0 {
		t["nhan_nut"] = d.Nhan[0]
		t["cac_nhan"] = strings.Join(d.Nhan, " | ")
	}
	return truyhoi.BangChung{ID: d.ID, Nguon: truyhoi.Manual, Truong: t, PhienBanChiMuc: huongdan.BanDung()}
}

func chaySoTay(ctx context.Context, bc *BoiCanh, a *thamSoSoTay) (ketQuaTho, error) {
	k := 0
	if a.K != nil {
		k = *a.K
	}
	var out []truyhoi.BangChung
	for _, d := range huongdan.Tim(ctx, huongdan.Hoi{Cau: a.TruyVan, Man: bc.Man, K: k}) {
		out = append(out, bangChungDoan(d))
	}
	return ketQuaTho{bangChung: out, coDuLieu: true}, ctx.Err()
}

func chayGiaiThich(_ context.Context, bc *BoiCanh, _ *khongThamSo) (ketQuaTho, error) {
	if bc.Man == "" {
		return ketQuaTho{}, errKhongThay
	}
	var out []truyhoi.BangChung
	for _, d := range huongdan.TheoMan(bc.Man) {
		out = append(out, bangChungDoan(d))
	}
	if len(out) == 0 {
		return ketQuaTho{}, errKhongThay
	}
	return ketQuaTho{bangChung: out}, nil
}

type thamSoMan struct {
	Man string `json:"man"`
	man string
}

// laManTien: a money screen of Nếp's slip rule, or one whose manual marks
// it. A chip never leads there (ai_khong_cham_tien).
func laManTien(man string) bool {
	if nepphieu.PhaiLui(man) {
		return true
	}
	for _, d := range huongdan.TheoMan(man) {
		if d.Tien {
			return true
		}
	}
	return false
}

func kiemMan(_ *BoiCanh, a *thamSoMan) *loiTS {
	a.man = huongdan.ChuanMan(a.Man)
	if a.man == "" {
		return thamSoSai("man", "must be a screen route of the app, as the manual names it")
	}
	if laManTien(a.man) {
		return &loiTS{loi: KhongDuocPhep, truong: "man"}
	}
	return nil
}

func chayMan(_ context.Context, bc *BoiCanh, a *thamSoMan) (ketQuaTho, error) {
	chip := &ChipMan{Man: a.man}
	if bc.Man != "" {
		if b, ok := huongdan.DuongToi(bc.Man, a.man); ok {
			chip.Buoc = b
		}
	}
	bc.mu.Lock()
	bc.nhap.Chip = chip
	bc.mu.Unlock()
	return ketQuaTho{them: map[string]any{"da_them": true, "so_buoc": len(chip.Buoc)}}, nil
}

// ---- drafts ---------------------------------------------------------------

type thamSoDeXuat struct {
	IDs []string `json:"ids"`
	ids []string
}

func kiemDeXuat(bc *BoiCanh, a *thamSoDeXuat) *loiTS {
	a.ids = nil
	for _, bi := range a.IDs {
		id, ok := bc.giaiBiDanh(bi, truyhoi.Places)
		if !ok {
			return thamSoSai("ids", "every item must be a place alias (p1…) from this turn's evidence")
		}
		a.ids = append(a.ids, id)
	}
	return nil
}

func chayDeXuat(_ context.Context, bc *BoiCanh, a *thamSoDeXuat) (ketQuaTho, error) {
	bc.mu.Lock()
	bc.nhap.Quan = append([]string(nil), a.ids...)
	bc.mu.Unlock()
	return ketQuaTho{them: map[string]any{"da_them": len(a.ids)}}, nil
}

type thamSoLichTrinh struct {
	NgayISO string `json:"ngay_iso,omitempty"`
	Chang   []struct {
		ID  string `json:"id"`
		Gio string `json:"gio,omitempty"`
	} `json:"chang"`
	chang []Chang
}

func kiemLichTrinh(bc *BoiCanh, a *thamSoLichTrinh) *loiTS {
	if a.NgayISO != "" && !hieu.NgayHopLe(a.NgayISO) {
		return thamSoSai("ngay_iso", "must be a calendar date YYYY-MM-DD")
	}
	a.chang = nil
	for _, c := range a.Chang {
		id, ok := bc.giaiBiDanh(c.ID, truyhoi.Places)
		if !ok {
			return thamSoSai("chang", "every stop must be a place alias (p1…) from this turn's evidence")
		}
		if c.Gio != "" && !hieu.GioHopLe(c.Gio) {
			return thamSoSai("chang", "a stop's time must be HH:MM")
		}
		a.chang = append(a.chang, Chang{ID: id, Gio: c.Gio})
	}
	return nil
}

func chayLichTrinh(_ context.Context, bc *BoiCanh, a *thamSoLichTrinh) (ketQuaTho, error) {
	bc.mu.Lock()
	bc.nhap.LichTrinh = &LichTrinh{NgayISO: a.NgayISO, Chang: append([]Chang(nil), a.chang...)}
	bc.mu.Unlock()
	return ketQuaTho{them: map[string]any{"da_them": len(a.chang)}}, nil
}

type thamSoBinhChon struct {
	CauHoi  string   `json:"cau_hoi"`
	LuaChon []string `json:"lua_chon"`
}

func kiemBinhChon(_ *BoiCanh, a *thamSoBinhChon) *loiTS {
	if strings.TrimSpace(a.CauHoi) == "" {
		return thamSoSai("cau_hoi", "must not be empty")
	}
	for _, l := range a.LuaChon {
		if strings.TrimSpace(l) == "" {
			return thamSoSai("lua_chon", "options must not be empty")
		}
	}
	return nil
}

func chayBinhChon(_ context.Context, bc *BoiCanh, a *thamSoBinhChon) (ketQuaTho, error) {
	bc.mu.Lock()
	bc.nhap.BinhChon = &BinhChon{CauHoi: a.CauHoi, LuaChon: append([]string(nil), a.LuaChon...)}
	bc.mu.Unlock()
	return ketQuaTho{them: map[string]any{"da_them": true}}, nil
}

// ---- group and own outings ------------------------------------------------

// laNhom holds a group tool to the group bot with a group from the job, and
// never a pair's turn (ChoCap): defence in depth behind the permission
// table.
func (bc *BoiCanh) laNhom() *loiTS {
	if bc.Bot != obs.BotNhom || bc.NhomID == "" || bc.botQuyen() == BotCap {
		return &loiTS{loi: KhongDuocPhep}
	}
	return nil
}

// laNep holds a me-scoped tool to Nếp with a person from the job.
func (bc *BoiCanh) laNep() *loiTS {
	if bc.Bot != obs.BotNep || bc.NguoiHoi == "" {
		return &loiTS{loi: KhongDuocPhep}
	}
	return nil
}

func (bc *BoiCanh) ngay() time.Time {
	y, m, d := bc.Luc.In(gioVN).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, gioVN)
}

func voiNguon(bs []truyhoi.BangChung, n truyhoi.Nguon) []truyhoi.BangChung {
	out := make([]truyhoi.BangChung, 0, len(bs))
	for _, b := range bs {
		b.Nguon = n
		out = append(out, b)
	}
	return out
}

func chayAnhNhom(ctx context.Context, bc *BoiCanh, _ *khongThamSo) (ketQuaTho, error) {
	if l := bc.laNhom(); l != nil {
		return ketQuaTho{}, l
	}
	if bc.Nguon.Nhom == nil {
		return ketQuaTho{}, &loiTS{loi: LoiNguon}
	}
	n, err := bc.Nguon.Nhom.SoThanhVien(ctx, bc.NhomID)
	if err != nil {
		return ketQuaTho{}, err
	}
	cs, err := bc.Nguon.Nhom.ChuyenDi(ctx, bc.NhomID, bc.ngay(), true, kAnhNhomChuyenDi)
	if err != nil {
		return ketQuaTho{}, err
	}
	return ketQuaTho{bangChung: voiNguon(cs, truyhoi.GroupHistory), them: map[string]any{"so_thanh_vien": n}, coDuLieu: true}, nil
}

type thamSoChuyenNhom struct {
	Khi string `json:"khi"`
	K   *int   `json:"k,omitempty"`
}

func chayChuyenNhom(ctx context.Context, bc *BoiCanh, a *thamSoChuyenNhom) (ketQuaTho, error) {
	if l := bc.laNhom(); l != nil {
		return ketQuaTho{}, l
	}
	if bc.Nguon.Nhom == nil {
		return ketQuaTho{}, &loiTS{loi: LoiNguon}
	}
	k := kChuyenMacDinh
	if a.K != nil {
		k = *a.K
	}
	cs, err := bc.Nguon.Nhom.ChuyenDi(ctx, bc.NhomID, bc.ngay(), a.Khi == "sap_toi", k)
	if err != nil {
		return ketQuaTho{}, err
	}
	return ketQuaTho{bangChung: voiNguon(cs, truyhoi.GroupHistory), coDuLieu: true}, nil
}

type thamSoK struct {
	K *int `json:"k,omitempty"`
}

func chayChuyenCuaToi(ctx context.Context, bc *BoiCanh, a *thamSoK) (ketQuaTho, error) {
	if l := bc.laNep(); l != nil {
		return ketQuaTho{}, l
	}
	if bc.Nguon.CaNhan == nil {
		return ketQuaTho{}, &loiTS{loi: LoiNguon}
	}
	k := kChuyenMacDinh
	if a.K != nil {
		k = *a.K
	}
	cs, err := bc.Nguon.CaNhan.ChuyenDiSapToi(ctx, bc.NguoiHoi, bc.ngay(), k)
	if err != nil {
		return ketQuaTho{}, err
	}
	return ketQuaTho{bangChung: voiNguon(cs, truyhoi.GroupHistory), coDuLieu: true}, nil
}

// ---- memory (Nếp, scope me) -----------------------------------------------

func bangChungSuThat(s trinho.SuThat) truyhoi.BangChung {
	t := map[string]string{"noi_dung": s.NoiDung, "loai": string(s.Loai), "tu_ngay": s.TuLuc.In(gioVN).Format(time.DateOnly)}
	if s.DenLuc != nil {
		t["den_ngay"] = s.DenLuc.In(gioVN).Format(time.DateOnly)
	}
	return truyhoi.BangChung{ID: s.ID, Nguon: truyhoi.Memory, Truong: t}
}

type thamSoNho struct {
	TruyVan string `json:"truy_van"`
	K       *int   `json:"k,omitempty"`
}

func chayNho(ctx context.Context, bc *BoiCanh, a *thamSoNho) (ketQuaTho, error) {
	if l := bc.laNep(); l != nil {
		return ketQuaTho{}, l
	}
	if bc.Nguon.TriNho == nil {
		return ketQuaTho{}, &loiTS{loi: LoiNguon}
	}
	k := kNhoMacDinh
	if a.K != nil {
		k = *a.K
	}
	ss, err := bc.Nguon.TriNho.Nho(ctx, bc.NguoiHoi, a.TruyVan, k)
	if err != nil {
		return ketQuaTho{}, err
	}
	var out []truyhoi.BangChung
	for _, s := range ss {
		out = append(out, bangChungSuThat(s))
	}
	return ketQuaTho{bangChung: out, coDuLieu: true}, nil
}

type thamSoGhiNho struct {
	NoiDung  string `json:"noi_dung"`
	Loai     string `json:"loai"`
	PhanLoai string `json:"phan_loai"`
	TuNgay   string `json:"tu_ngay,omitempty"`
	DenNgay  string `json:"den_ngay,omitempty"`
	moi      trinho.SuThatMoi
}

// kiemGhiNho enforces the MODEL's own classification (only ca_nhan is
// stored) and the data-format privacy check, then that the fact is the
// person's own words of this turn, then the fact's structure. None reads
// the fact's meaning.
//
// The words stored are the person's (re-review MAJOR 2): noi_dung must be a
// whole span of their message (BoiCanh.LoiNguoiHoi), found by exact
// identity of words, and the span is taken from the message itself, not
// from what the model wrote. Text that reached the model any other way (the
// panel history, the slip, a tool's data) can therefore never become a
// fact, whatever the model was told; neither can its paraphrase.
func kiemGhiNho(bc *BoiCanh, a *thamSoGhiNho) *loiTS {
	if PhanLoaiSuThat(a.PhanLoai) != CaNhan {
		return &loiTS{loi: KhongDuocPhep, truong: "phan_loai"}
	}
	if guard.DinhDang(a.NoiDung) != guard.RaSach {
		return &loiTS{loi: KhongDuocPhep, truong: "noi_dung"}
	}
	doan, ok := doanCuaLoi(bc.LoiNguoiHoi, prompts.BoDanhDau(a.NoiDung))
	if !ok {
		return thamSoSai("noi_dung", "must be copied word for word from the person's message in the cau_hoi block")
	}
	a.NoiDung = doan
	tu := bc.ngay()
	if a.TuNgay != "" {
		if !hieu.NgayHopLe(a.TuNgay) {
			return thamSoSai("tu_ngay", "must be a calendar date YYYY-MM-DD")
		}
		tu, _ = time.ParseInLocation(time.DateOnly, a.TuNgay, gioVN)
	}
	a.moi = trinho.SuThatMoi{NoiDung: a.NoiDung, Loai: trinho.LoaiSuThat(a.Loai), TuLuc: tu, Nguon: trinho.NoiRo}
	if a.DenNgay != "" {
		if !hieu.NgayHopLe(a.DenNgay) {
			return thamSoSai("den_ngay", "must be a calendar date YYYY-MM-DD")
		}
		den, _ := time.ParseInLocation(time.DateOnly, a.DenNgay, gioVN)
		a.moi.DenLuc = &den
	}
	if err := a.moi.Kiem(); err != nil {
		return thamSoSai("den_ngay", "the fact must be 1-160 characters and den_ngay after tu_ngay")
	}
	return nil
}

// doanCuaLoi finds doan in loi as a run of whole words (runs of white
// space are one space in both) and returns that run as loi has it. Exact
// identity of words, compared one by one; no word is read. false when doan
// has no word or is not such a run.
func doanCuaLoi(loi, doan string) (string, bool) {
	lw, dw := strings.Fields(loi), strings.Fields(doan)
	if len(dw) == 0 {
		return "", false
	}
	for i := 0; i+len(dw) <= len(lw); i++ {
		khop := true
		for j := range dw {
			khop = khop && lw[i+j] == dw[j]
		}
		if khop {
			return strings.Join(lw[i:i+len(dw)], " "), true
		}
	}
	return "", false
}

func chayGhiNho(ctx context.Context, bc *BoiCanh, a *thamSoGhiNho) (ketQuaTho, error) {
	if l := bc.laNep(); l != nil {
		return ketQuaTho{}, l
	}
	if bc.Nguon.TriNho == nil {
		return ketQuaTho{}, &loiTS{loi: LoiNguon}
	}
	// Queued, not written: the fact reaches the store only when the answer
	// is released (BoiCanh.CamKet). A turn that fails writes nothing.
	moi := a.moi
	if l := bc.xepNho(thaoTacNho{ghi: &moi}); l != nil {
		return ketQuaTho{}, l
	}
	return ketQuaTho{them: map[string]any{"da_nhan": true, "ghi_khi_tra_loi": true}}, ctx.Err()
}

type thamSoQuen struct {
	ID   string `json:"id,omitempty"`
	MoTa string `json:"mo_ta,omitempty"`
	q    trinho.QuenGi
}

func kiemQuen(bc *BoiCanh, a *thamSoQuen) *loiTS {
	a.q = trinho.QuenGi{MoTa: a.MoTa}
	if a.ID != "" {
		id, ok := bc.giaiBiDanh(a.ID, truyhoi.Memory)
		if !ok {
			return thamSoSai("id", "must be a fact alias (f1…) you were shown this turn")
		}
		a.q.ID = id
	}
	if a.q.Kiem() != nil {
		return thamSoSai("id", "give exactly one of id or mo_ta")
	}
	return nil
}

func chayQuen(ctx context.Context, bc *BoiCanh, a *thamSoQuen) (ketQuaTho, error) {
	if l := bc.laNep(); l != nil {
		return ketQuaTho{}, l
	}
	if bc.Nguon.TriNho == nil {
		return ketQuaTho{}, &loiTS{loi: LoiNguon}
	}
	// Queued like remember_fact: deleted only when the answer is released.
	q := a.q
	_ = bc.xepNho(thaoTacNho{quen: &q})
	return ketQuaTho{them: map[string]any{"da_nhan": true, "ghi_khi_tra_loi": true}}, ctx.Err()
}

func chayNhoGi(ctx context.Context, bc *BoiCanh, _ *khongThamSo) (ketQuaTho, error) {
	if l := bc.laNep(); l != nil {
		return ketQuaTho{}, l
	}
	if bc.Nguon.TriNho == nil {
		return ketQuaTho{}, &loiTS{loi: LoiNguon}
	}
	tc, err := bc.Nguon.TriNho.LietKe(ctx, bc.NguoiHoi)
	if err != nil {
		return ketQuaTho{}, err
	}
	var out []truyhoi.BangChung
	for _, s := range tc.SuThat {
		out = append(out, bangChungSuThat(s))
	}
	return ketQuaTho{bangChung: out, coDuLieu: true}, nil
}

type thamSoNhac struct {
	OutingID string `json:"outing_id"`
	NgayISO  string `json:"ngay_iso"`
	Gio      string `json:"gio,omitempty"`
}

func chayNhac(context.Context, *BoiCanh, *thamSoNhac) (ketQuaTho, error) {
	return ketQuaTho{them: map[string]any{"loi": string(ChuaCo)}}, nil
}

// Every tool's implementation, one per registry name, in three tables by
// scope (TestCongCuDuMoiTen): the tools either bot may be granted, the
// group's own and Nếp's own (scope me: the screen, the person's own outings,
// memory and reminders). A turn's BoiCanh carries its bot's own table as a
// value (ChoNep, ChoNhom), so nothing on the group's path names Nếp's table
// -- not at run time, and not in what a static walk of the code can reach
// (internal/aigate: the group never reaches a memory tool).
var congCusChung = map[Ten]congCu{
	SearchPlaces:     dk(SearchPlaces, kiemTim, chayTim),
	GetPlace:         dk(GetPlace, kiemQuan, chayQuan),
	ListDestinations: dk[khongThamSo](ListDestinations, nil, chayDiemDen),
	NearestArea:      dk(NearestArea, kiemKhuVuc, chayKhuVuc),
	SearchAppManual:  dk[thamSoSoTay](SearchAppManual, nil, chaySoTay),
	ProposePlaces:    dk(ProposePlaces, kiemDeXuat, chayDeXuat),
	ProposeItinerary: dk(ProposeItinerary, kiemLichTrinh, chayLichTrinh),
}

var congCusNhom = map[Ten]congCu{
	DraftPoll:        dk(DraftPoll, kiemBinhChon, chayBinhChon),
	GroupSnapshot:    dk[khongThamSo](GroupSnapshot, nil, chayAnhNhom),
	ListGroupOutings: dk[thamSoChuyenNhom](ListGroupOutings, nil, chayChuyenNhom),
}

var congCusNep = map[Ten]congCu{
	ExplainScreen:     dk[khongThamSo](ExplainScreen, nil, chayGiaiThich),
	SuggestScreen:     dk(SuggestScreen, kiemMan, chayMan),
	MyUpcomingOutings: dk[thamSoK](MyUpcomingOutings, nil, chayChuyenCuaToi),
	RecallMemory:      dk[thamSoNho](RecallMemory, nil, chayNho),
	RememberFact:      dk(RememberFact, kiemGhiNho, chayGhiNho),
	ForgetFact:        dk(ForgetFact, kiemQuen, chayQuen),
	WhatYouRemember:   dk[khongThamSo](WhatYouRemember, nil, chayNhoGi),
	SetReminder:       dk[thamSoNhac](SetReminder, nil, chayNhac),
}

// ChoNep gives the turn Nếp's own tools beside the common ones. The engine
// calls it on every Nếp turn's context.
func (bc *BoiCanh) ChoNep() *BoiCanh {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.rieng = congCusNep
	return bc
}

// ChoNhom gives the turn the group's own tools beside the common ones. The
// engine calls it on every group turn's context.
func (bc *BoiCanh) ChoNhom() *BoiCanh {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.rieng = congCusNhom
	return bc
}

// congCusCap is the pair's own table: empty. A chat of two gets the common
// tools and nothing of the group's (no poll draft, no group snapshot, no
// group outings) or of Nếp's.
var congCusCap = map[Ten]congCu{}

// ChoCap gives a pair's turn (the room assistant in a chat of two) the
// common tools only, and reads the permission table under BotCap, so a
// group tool is neither declared nor callable. The engine calls it instead
// of ChoNhom on every pair turn's context.
func (bc *BoiCanh) ChoCap() *BoiCanh {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.rieng = congCusCap
	bc.bang = BotCap
	return bc
}

// congCu is tool t's implementation for this turn: a common tool, or one of
// the bot's own table; false for a tool this turn has no table for (a
// context built without ChoNep, ChoNhom or ChoCap offers the common tools
// only).
func (bc *BoiCanh) congCu(t Ten) (congCu, bool) {
	if cc, ok := congCusChung[t]; ok {
		return cc, true
	}
	bc.mu.Lock()
	rieng := bc.rieng
	bc.mu.Unlock()
	cc, ok := rieng[t]
	return cc, ok
}
