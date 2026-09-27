package aiharness

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"mobile/services/core/internal/aiharness/agent"
	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/crag"
	"mobile/services/core/internal/aiharness/hieu"
	"mobile/services/core/internal/aiharness/kiemchung"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/preprocess"
	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/aiharness/tactu"
	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/aiharness/traloi"
	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/domain/nepphieu"
	"mobile/services/core/internal/domain/thoigian"
)

// duTruKiem is the model call every released prose keeps for its verifier.
const duTruKiem = 1

// nep is Nếp's turn (the owner's rule, 2026-09-25). After the structural
// stages (the money screen of the slip, NFC, invisible characters, the
// @mention) the MODEL reads the question once, through the router, and its
// labels decide: a money action is refused with the fixed sentence and no
// further call; an injection label keeps the person's text as data but
// leaves only the read tools; a question back ends the turn with the
// model's one question. Otherwise the path the router chose runs:
//
//   - the retrieval path, when the router's one-step path asked for the
//     places catalogue or the app manual with its own query: crag's grade
//     and one corrective round the grader chooses, then traloi's grounded
//     structured answer, its deterministic checks and its verifier;
//   - a direct answer (tra_loi_thang): one call with no tool;
//   - the tools (tactu): the fast path for explain_screen, or the bounded
//     ADK loop where the model chooses the tools.
//
// Every prose answer then goes to the verifier in a fresh context
// (xacMinh), which carries the judgement of claimed actions and money, and
// last to the output guard's structural checks. No word list, pattern or
// keyword reader looks at the question or the answer on this path.
func (e *Engine) nep(ctx context.Context, t Turn, s Sink, rec *obs.TurnRecord, batDau time.Time) (Result, error) {
	// The money screen first, before anything else is read (ADR-0033 §2.2):
	// a structural check of the route the device sent.
	if t.PhieuNep != nil && nepphieu.PhaiLui(t.PhieuNep.Man) {
		rec.Guard = obs.GuardRefused
		return Result{}, &Loi{Ma: cau.NepLuiManTien}
	}
	hoi := preprocess.LamSach(t.LoiNho)
	rec.KyTuAn += hoi.KyTuAn
	rec.KhongDau = hoi.KhongDau
	if hoi.Chu == "" {
		return Result{}, &Loi{Ma: cau.InvalidAIResult}
	}
	conLai := llm.MaxModelCallsPerTurn - t.DaGoiTruoc
	if conLai <= 0 {
		return Result{}, &Loi{Ma: cau.HetNganSach}
	}
	dem := llm.NewDem(e.model, conLai, t.GiuLuot).WithGioiHan(e.gioiHan)
	if e.cho != nil {
		dem.WithWait(e.cho)
	}
	runCtx, cancel := context.WithTimeout(ctx, e.han)
	defer cancel()
	// One embedding budget for the whole turn: the router's example choice
	// and every retrieval (an embedder shared across turns reads it from the
	// context, nhung.TheoLuot).
	demNhung := nhung.MoiDemLuot()
	runCtx = nhung.VoiDemLuot(runCtx, demNhung)
	loi := func(err error) error {
		return loiMoHinh(err, errors.Is(ctx.Err(), context.Canceled), errors.Is(runCtx.Err(), context.DeadlineExceeded), rec)
	}
	luot := nganHanNep(t.LuotNep, rec)
	dsDiemDen := e.danhSachDiemDen(runCtx)
	v := hieu.Vao{
		Bot:             obs.BotNep,
		Cau:             hoi.Chu,
		Luc:             t.Luc,
		NganHan:         luot,
		PhieuNep:        phieuJSON(t.PhieuNep),
		DanhSachDiemDen: dsDiemDen,
		DemNhung:        demNhung,
	}
	rec.MsTienXuLy = ms(e.now().Sub(batDau))
	s.TrangThai(cau.DangNghi, soTinNep)

	moHinh := e.now()
	sc := tools.MoiSoCai(obs.BotNep)
	defer func() {
		rec.MsMoHinh = ms(e.now().Sub(moHinh))
		rec.SoGoiMoHinh = dem.SoGoi()
		rec.SoCongCu = sc.TongGoi()
		rec.CongCu = obs.CacCongCu{}
		for _, c := range sc.DaChay() {
			rec.CongCu = append(rec.CongCu, obs.CongCu(c))
		}
	}()

	kq, err := e.hieu.Hieu(runCtx, v, dem)
	if err != nil {
		if ma, ok := hieu.MaLoi(err); ok {
			// The router's own failure: the fixed sentence asking the person
			// to rephrase, never a guess.
			rec.LoiMoHinh = obs.LoiBadResp
			if errors.Is(err, hieu.ErrBiChan) {
				rec.LoiMoHinh = obs.LoiSafety
			}
			return Result{}, &Loi{Ma: ma}
		}
		return Result{}, loi(err)
	}
	ghiNhanRouter(rec, kq)
	q := hieu.QuyetDinhCho(obs.BotNep, kq)
	if q.TuChoiTien {
		rec.Guard, rec.Duong = obs.GuardRefused, obs.DuongTuChoiTien
		return Result{}, &Loi{Ma: q.TuChoi}
	}
	if q.HanChe {
		rec.Guard = obs.GuardRestricted
	}
	if q.HoiLai {
		rec.Duong = obs.DuongHoiLai
		res, err := e.hoiLai(runCtx, rec, dem, kq)
		if err != nil && !isLoi(err) {
			return Result{}, loi(err)
		}
		return res, err
	}
	cung, mem, err := tools.RangBuocTuRouter(kq.Slots, t.Luc)
	if err != nil {
		rec.LoiMoHinh = obs.LoiBadResp
		return Result{}, &Loi{Ma: cau.InvalidAIResult}
	}
	bc := &tools.BoiCanh{
		Bot: obs.BotNep, NguoiHoi: t.NguoiHoi, Man: manCua(t.PhieuNep), Luc: t.Luc, HanChe: q.HanChe, YDinh: kq.YDinh,
		Cung: cung, Mem: mem, DiUngNgoaiDanhMuc: kq.Slots.DiUngNgoaiDanhMuc, DiemDen: idsDiemDen(dsDiemDen),
		Nguon: e.nguon, Quyen: e.quyen, SoCai: sc,
	}
	khoi := e.khoiThem(t, rec, kq)
	if b := e.hoSoNep(runCtx, t, hoi.Chu); b != "" {
		khoi = append(khoi, b)
	}
	if ten, _, ok := tactu.Nhanh(kq, bc); ok && !q.KhongCongCu && (ten == tools.SearchPlaces || ten == tools.SearchAppManual) {
		if res, chay, err := e.nepTruyHoi(runCtx, rec, dem, kq, bc, hoi.Chu, ten); chay {
			if err != nil {
				return Result{}, loi(err)
			}
			return e.luuYDiUng(res, kq, rec)
		}
	}
	if dem.ConLai() < 1+duTruKiem {
		return Result{}, &Loi{Ma: cau.HetNganSach}
	}
	var td agent.TheoDoi
	var text string
	ngan, phien, xong := e.bamPhien(runCtx, t, luot)
	defer xong()
	if kq.Huong == hieu.TraLoiThang || q.KhongCongCu {
		rec.Duong = obs.DuongThang
		blocks := khoi
		if s := tactu.KhoiLichSu(luot, nil); s != "" {
			blocks = append([]string{s}, blocks...)
		}
		blocks = append(blocks, prompts.BocDuLieuDanhDau(prompts.CauHoi, hoi.Chu))
		instruction := prompts.NepAgent(e.maKiem)
		if q.KhongCongCu {
			instruction += "\n\n" + prompts.LoiDanNhan(string(kq.NhanGuard))
		}
		cfg := agent.CauHinh{
			Ten:             nepTen,
			Instruction:     instruction,
			NhietDo:         nepNhietDo,
			MaxOutputTokens: nepMaxTokens,
			MaxBuoc:         nepMaxBuoc,
			ConLai:          func() int { return dem.ConLai() - duTruKiem },
		}
		text, err = agent.Chay(runCtx, dem, cfg, nil, strings.Join(blocks, "\n\n"), &td)
	} else {
		var ra tactu.Ra
		ra, err = tactu.Chay(runCtx, dem, tactu.Vao{
			Ten: nepTen, Instruction: prompts.NepAgent(e.maKiem), NhietDo: nepNhietDo, MaxTokens: nepMaxTokens,
			Router: kq, Cau: hoi.Chu, KhoiThem: khoi, NganHan: ngan, Phien: phien, BoiCanh: bc,
			DuTru: duTruKiem,
		}, &td)
		text = ra.Text
		rec.Duong = obs.DuongTacTu
		if ra.Nhanh {
			rec.Duong = obs.DuongNhanh
		}
	}
	snap := td.Snapshot()
	rec.Buoc, rec.TokensIn, rec.TokensOut, rec.TokensCache, rec.TokensNghi = snap.Buoc, snap.TokensIn, snap.TokensOut, snap.TokensCache, snap.TokensNghi
	if err != nil {
		return Result{}, loi(err)
	}
	if llm.BiChanAnToan(snap.Finish) {
		rec.LoiMoHinh = obs.LoiSafety
		return Result{}, &Loi{Ma: cau.InvalidAIResult}
	}
	res, err := e.xacMinh(runCtx, rec, dem, text, sc)
	if err != nil && !isLoi(err) {
		return Result{}, loi(err)
	}
	if err != nil {
		// Nothing is released, so nothing the tools queued is written.
		return Result{}, err
	}
	// The allergen caveat's structural check is the last check that can
	// withhold the answer, so it runs before anything is written.
	res, err = e.luuYDiUng(res, kq, rec)
	if err != nil {
		return Result{}, err
	}
	// The answer passed every check: only now do the memory writes the
	// model queued this turn reach the store (tools.BoiCanh.CamKet). A
	// failed write withholds the answer, which may say it was done.
	if err := bc.CamKet(runCtx); err != nil {
		if errors.Is(err, context.Canceled) && errors.Is(ctx.Err(), context.Canceled) {
			return Result{}, ErrHuy
		}
		return Result{}, &Loi{Ma: cau.ProviderUnavailable}
	}
	return res, nil
}

// luuYDiUng opens a released answer with the fixed caveat when the router
// flagged an allergen outside the closed list: no filter could check it,
// whatever path answered. The caveat is ours; the structural checks run
// once more on the whole text (its length cap).
func (e *Engine) luuYDiUng(res Result, kq hieu.KetQua, rec *obs.TurnRecord) (Result, error) {
	if !kq.Slots.DiUngNgoaiDanhMuc {
		return res, nil
	}
	out, err := e.kiemDauRa(cau.DiUngNgoaiDanhMuc+" "+res.Text, rec)
	if err != nil {
		return Result{}, err
	}
	res.Text = out.Text
	return res, nil
}

// hoiLai is the router's one question back, released only after the same
// checks as any prose: the structural checks on the question and on each
// option (an option that fails them is dropped), then the verifier in a
// fresh context on the question's sentences and the kept options, with no
// evidence. A question that claims an action or money, or a sentence the
// verifier judges unsupported, ends the turn with the fixed sentence.
func (e *Engine) hoiLai(ctx context.Context, rec *obs.TurnRecord, dem *llm.Dem, kq hieu.KetQua) (Result, error) {
	res, err := e.kiemDauRa(kq.CauHoiLai, rec)
	if err != nil {
		return Result{}, err
	}
	for _, c := range kq.LuaChonHoiLai {
		if r, err := e.kiemDauRa(c, rec); err == nil {
			res.LuaChon = append(res.LuaChon, r.Text)
		}
	}
	cauRa := append(traloi.TachCauVanXuoi(res.Text), res.LuaChon...)
	if err := e.phanXu(ctx, rec, dem, cauRa, nil); err != nil {
		return Result{}, err
	}
	return res, nil
}

func isLoi(err error) bool {
	var l *Loi
	return errors.As(err, &l)
}

// nepTruyHoi is the retrieval path: the source the router asked for, the
// query the router wrote, the hard constraints it extracted as filters that
// never relax, crag's grade and one corrective round, then traloi's
// grounded, verified answer. chay is false when the source has no port
// (the tools then answer loi_nguon to the model).
func (e *Engine) nepTruyHoi(ctx context.Context, rec *obs.TurnRecord, dem *llm.Dem, kq hieu.KetQua, bc *tools.BoiCanh, cauHoi string, ten tools.Ten) (Result, bool, error) {
	y := truyhoi.YeuCau{Cung: bc.Cung, Mem: bc.Mem}
	var tim truyhoi.Retriever
	switch ten {
	case tools.SearchPlaces:
		if e.nguon.Quan == nil {
			return Result{}, false, nil
		}
		y.Nguon, tim = truyhoi.Places, e.nguon.Quan
	case tools.SearchAppManual:
		// The manual has no hard constraint to apply.
		y = truyhoi.YeuCau{Nguon: truyhoi.Manual}
		tim = tools.SoTay{Man: bc.Man}
	default:
		return Result{}, false, nil
	}
	var caus []string
	for _, tv := range kq.TruyVan {
		if tv.Nguon == y.Nguon {
			caus = append(caus, tv.Cau)
		}
	}
	if len(caus) == 0 {
		return Result{}, false, nil
	}
	// Every query the router wrote for the source runs (it may write up to
	// hieu.MaxTruyVan); their results are merged by Go (nhieuTruyVan), the
	// first query standing for them in the request the grader reads.
	y.Cau = caus[0]
	y.DiUngNgoaiDanhMuc = bc.DiUngNgoaiDanhMuc
	tim = nhieuTruyVan{tim: tim, caus: caus}
	rec.Duong = obs.DuongTruyHoi
	r, err := traloi.Chay(ctx, traloi.Vao{Cau: cauHoi, YeuCau: y}, traloi.BoPhan{
		BoPhan:   crag.BoPhan{Tim: tim, XepLai: e.xepLai, Cham: e.cham, NganSach: &crag.NganSachXepLai{}},
		Verifier: e.kiem,
	}, bc.SoCai, dem)
	rec.VongSua, rec.SoXepLai, rec.SinhLai = r.Crag.Vong, r.Crag.SoXepLai, r.Vet.SinhLai
	switch {
	case r.Vet.KiemHong:
		rec.KetKiem = obs.KiemHong
	case r.Vet.SoKiem > 0 && r.KetThuc == traloi.DaTraLoi:
		rec.KetKiem = obs.KiemDat
	case r.Vet.SoKiem > 0:
		rec.KetKiem = obs.KiemKhongDat
	}
	if err != nil && errors.Is(err, crag.ErrNguon) && r.Crag.SoTruyHoi <= 1 && r.Vet.SoGoi == 0 {
		// The retriever failed before any evidence or grade: the turn is
		// handed to the tool path, whose tool answers the model loi_nguon
		// (a data failure, never recorded as the model's).
		rec.Duong = obs.DuongKhong
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, true, err
	}
	if r.KetThuc == traloi.BiChan {
		rec.OutGuard = obs.OutChan
		return Result{}, true, &Loi{Ma: cau.TraLoiBiChan}
	}
	res, err := e.kiemDauRa(r.Chu, rec)
	return res, true, err
}

// xacMinh releases a prose answer only after the verifier, in a fresh
// context, judged it: its place tokens rendered from the ledger, its
// sentences numbered by punctuation, the turn's evidence under local
// aliases. The output guard's structural checks run before it.
func (e *Engine) xacMinh(ctx context.Context, rec *obs.TurnRecord, dem *llm.Dem, text string, sc *tools.SoCai) (Result, error) {
	chu, _, _ := traloi.GhepVanXuoi(strings.TrimSpace(text), sc)
	// The structural checks first: they cost no call, and a leaked marker,
	// a quoted instruction or a phone number is never sent on to another
	// model, not even to the verifier.
	res, err := e.kiemDauRa(chu, rec)
	if err != nil {
		return Result{}, err
	}
	var bcs []truyhoi.BangChung
	for _, id := range sc.IDs() {
		if b, _, ok := sc.Lay(id); ok {
			bcs = append(bcs, b)
		}
	}
	if err := e.phanXu(ctx, rec, dem, traloi.TachCauVanXuoi(res.Text), bcs); err != nil {
		return Result{}, err
	}
	return res, nil
}

// phanXu is the verifier's one call on sentences cauRa against the turn's
// evidence bcs (none on a direct answer or a question back). The verifier
// must judge every sentence (kiemchung.Doc); a claimed action, money, or
// any sentence judged unsupported fails it, evidence or not: a direct
// answer that states a place, a price or an hour nothing in the turn backs
// is withheld like any other. A verifier output that fails its schema
// releases nothing.
func (e *Engine) phanXu(ctx context.Context, rec *obs.TurnRecord, dem *llm.Dem, cauRa []string, bcs []truyhoi.BangChung) error {
	if dem.ConLai() < duTruKiem {
		return &Loi{Ma: cau.HetNganSach}
	}
	p, err := e.kiem.PhanTu(ctx, cauRa, bcs, dem)
	if err != nil {
		if errors.Is(err, kiemchung.ErrCauTruc) {
			rec.KetKiem, rec.OutGuard = obs.KiemHong, obs.OutChan
			return &Loi{Ma: cau.TraLoiBiChan}
		}
		return err
	}
	if !p.Dat() {
		rec.KetKiem, rec.OutGuard = obs.KiemKhongDat, obs.OutChan
		return &Loi{Ma: cau.TraLoiBiChan}
	}
	rec.KetKiem = obs.KiemDat
	return nil
}

// khoiThem are the engine's data blocks for the answer step: the slip
// (text from the device, datamarked) and the server's facts (ours).
func (e *Engine) khoiThem(t Turn, rec *obs.TurnRecord, kq hieu.KetQua) []string {
	var blocks []string
	if phieu := renderPhieu(t.PhieuNep, rec); phieu != "" {
		blocks = append(blocks, prompts.BocDuLieuDanhDau(prompts.PhieuManHinh, phieu))
	}
	return append(blocks, prompts.BocDuLieu(prompts.MayChu, strings.Join(dongMayChuHieu(t.Luc, kq.Slots), "\n")))
}

// danhSachDiemDen is the closed list of destinations the router may pick
// from, read through the catalogue port. A failed or missing port offers no
// list: the slot is then not offered, and nothing is guessed.
func (e *Engine) danhSachDiemDen(ctx context.Context) []hieu.DiemDen {
	if e.nguon.Cho == nil {
		return nil
	}
	ds, err := e.nguon.Cho.DiemDen(ctx)
	if err != nil {
		return nil
	}
	out := make([]hieu.DiemDen, 0, len(ds))
	for _, d := range ds {
		out = append(out, hieu.DiemDen{ID: d.ID, Ten: d.Truong["ten"]})
	}
	return out
}

func idsDiemDen(ds []hieu.DiemDen) []string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.ID)
	}
	return out
}

func manCua(p *PhieuNep) string {
	if p == nil {
		return ""
	}
	return preprocess.LamSach(p.Man).Chu
}

// ghiNhanRouter records the router's labels, enums and a count only.
func ghiNhanRouter(rec *obs.TurnRecord, kq hieu.KetQua) {
	rec.NhanGuard, rec.Tien, rec.Huong = obs.NhanGuard(kq.NhanGuard), obs.Tien(kq.Tien), obs.Huong(kq.Huong)
	if kq.NhanGuard == hieu.NhayCam {
		// A sensitive label is never stored against the person, not even
		// as a closed value: the row names the invocation, and the
		// invocation names the person (chat_ai_invocations.person_id).
		rec.NhanGuard = ""
	}
	rec.SoYDinh = len(kq.YDinh)
	if len(kq.YDinh) > 0 {
		rec.YDinh = obs.YDinh(kq.YDinh[0])
	}
}

// dongMayChuHieu is the answer's server facts on the router path: the «now»
// line, then the date and the time window the ROUTER resolved, laid on the
// calendar by arithmetic (hieu.Doc has already checked their form).
func dongMayChuHieu(luc time.Time, s hieu.Slots) []string {
	lines := []string{thoigian.DongBayGio(luc)}
	var ngay []string
	if d, err := time.Parse(time.DateOnly, s.NgayISO); err == nil {
		ngay = append(ngay, "- ngày: "+thoigian.Ngay{Nam: d.Year(), Thang: int(d.Month()), Ngay: d.Day()}.String())
	}
	if k := s.KhungGio; k != nil {
		g := "- giờ: " + k.Tu
		if k.Den != "" {
			g += "–" + k.Den
		}
		ngay = append(ngay, g)
	}
	if len(ngay) > 0 {
		lines = append(lines, "Ngày giờ trong câu hỏi, đã quy ra lịch giờ Việt Nam:")
		lines = append(lines, ngay...)
	}
	return lines
}

// nganHanNep is the panel session as short-term turns, cleaned
// structurally (NFC, invisible characters); a turn with no text or an
// unknown speaker is left out and counted (rec.LuotBo). Nothing is dropped
// for its words: the router and the answer read the turns as data.
func nganHanNep(ls []LuotNep, rec *obs.TurnRecord) []trinho.Luot {
	var out []trinho.Luot
	for _, l := range ls {
		c := preprocess.LamSach(l.Chu)
		rec.KyTuAn += c.KyTuAn
		var vai trinho.VaiLuot
		switch l.Vai {
		case vaiToi:
			vai = trinho.Toi
		case vaiNep:
			vai = trinho.TroLy
		default:
			rec.LuotBo++
			continue
		}
		if c.Chu == "" {
			rec.LuotBo++
			continue
		}
		out = append(out, trinho.Luot{Vai: vai, Chu: c.Chu})
	}
	if len(out) > trinho.MaxLuotNganHan {
		out = out[len(out)-trinho.MaxLuotNganHan:]
	}
	return out
}

// phienNep names the device's session inside one turn.
const phienNep = "panel"

// hoSoNep is the personalization block of a Nếp turn: the person's own
// recalled facts, at most five, only while their memory toggle is on (the
// adapter returns "" otherwise). A failed recall answers without it: memory
// is an aid, never a reason to fail the turn.
func (e *Engine) hoSoNep(ctx context.Context, t Turn, cau string) string {
	if e.hoSo == nil || t.Bot != obs.BotNep || t.NguoiHoi == "" {
		return ""
	}
	b, err := e.hoSo.HoSoNep(ctx, t.NguoiHoi, cau)
	if err != nil {
		return ""
	}
	return b
}

// bamPhien buffers the device's session of this turn in the short-term
// store, one key per turn (stm-personalization §5.1, option A: the device
// stays the session's source), and returns the port and key the tool part
// reads, and the release that drops the key when the turn ends. Without a
// store, or when the buffer cannot be written, the turns stay in memory.
func (e *Engine) bamPhien(ctx context.Context, t Turn, luot []trinho.Luot) (trinho.NganHan, string, func()) {
	trongRam := func() (trinho.NganHan, string, func()) { return phienThietBi(luot), phienNep, func() {} }
	if e.nganHan == nil || t.NguoiHoi == "" || t.InvocationID == "" {
		return trongRam()
	}
	phien, err := e.nganHan.PhienLuot(t.NguoiHoi, t.InvocationID)
	if err != nil {
		return trongRam()
	}
	xoa := func() { _ = e.nganHan.Xoa(context.WithoutCancel(ctx), phien) }
	for _, l := range luot {
		if l.Luc.IsZero() {
			l.Luc = t.Luc
		}
		if err := e.nganHan.Them(ctx, phien, l); err != nil {
			xoa()
			return trongRam()
		}
	}
	return e.nganHan, phien, xoa
}

// phienThietBi is the panel session the device sent, as the short-term
// memory port of the tool part (stm-personalization §5.1, option A: the
// device is the session's source and sends it with every question; the
// server keeps nothing of it between turns, ADR-0036 §4). Writes are
// dropped: the next question brings the session again.
type phienThietBi []trinho.Luot

func (p phienThietBi) Doc(context.Context, string) ([]trinho.Luot, error) {
	return append([]trinho.Luot(nil), p...), nil
}
func (phienThietBi) Them(context.Context, string, trinho.Luot) error { return nil }
func (phienThietBi) Xoa(context.Context, string) error               { return nil }

// phieuJSON is the slip as the router reads it, under the device's own key
// names, with every string cleaned structurally. Keys marshal sorted, so
// the request is stable byte for byte.
func phieuJSON(p *PhieuNep) json.RawMessage {
	if p == nil {
		return nil
	}
	sach := func(s string) string { return preprocess.LamSach(s).Chu }
	m := map[string]any{}
	if v := sach(p.Man); v != "" {
		m["man"] = v
	}
	if v := sach(p.TieuDe); v != "" {
		m["tieuDe"] = v
	}
	if p.Nhip != nil {
		n := map[string]any{"kieu": sach(p.Nhip.Kieu)}
		if p.Nhip.ConNgay != nil {
			n["conNgay"] = *p.Nhip.ConNgay
		}
		if p.Nhip.TruocNgay != nil {
			n["truocNgay"] = *p.Nhip.TruocNgay
		}
		m["nhip"] = n
	}
	if v := sach(p.LoaiSo); v != "" {
		m["loaiSo"] = v
	}
	if len(p.SoLieu) > 0 {
		so := map[string]any{}
		for k, v := range p.SoLieu {
			switch x := v.(type) {
			case string:
				so[k] = sach(x)
			case float64, int:
				so[k] = x
			}
		}
		m["soLieu"] = so
	}
	var goiY []string
	for _, g := range p.GoiY {
		if v := sach(g); v != "" {
			goiY = append(goiY, v)
		}
	}
	if len(goiY) > 0 {
		m["goiY"] = goiY
	}
	raw, err := json.Marshal(m)
	if err != nil || len(m) == 0 {
		return nil
	}
	return raw
}
