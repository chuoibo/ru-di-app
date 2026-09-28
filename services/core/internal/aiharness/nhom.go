package aiharness

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"mobile/services/core/internal/aiharness/agent"
	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/hieu"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/preprocess"
	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/aiharness/tactu"
	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/domain/thoigian"
	"mobile/services/core/internal/rerank"
)

// The group assistant «Rủ Đi AI» in the thread (design 03, slice 9): the
// same engine stages as Nếp's, with the group's policy.
//
//  1. preprocess (structural): NFC, invisible characters out, the @mention
//     out; the shared turns cleaned the same way. A v2 (end-to-end
//     encrypted) room gives the model no shared turn and the short-term
//     store nothing.
//  2. the router (hieu), the group's schema: no memory source, no memory
//     intent; the members as aliases m1, m2, …; the shared turns as its
//     only history (the tag message is the question).
//  3. policy, a pure function of the model's labels (hieu.QuyetDinhCho) and
//     the person's explicit command: money_action is answered with the
//     fixed refusal and no further call; split_draft (or /chia-bill) becomes
//     a draft only (chiaBillParts); chen_lenh keeps the read tools only.
//  4. the retrieval fast path (find_places), a direct answer, or the bounded
//     agent loop with the group's toolset (draft_poll masked until a poll
//     draft has a card kind).
//  5. the verifier in a fresh context on every released prose, the output
//     guard's structural checks, then the parts of the `tra_loi` card
//     (text, places, itinerary, expense_draft), which the worker grounds
//     again with companion.GroundReply before it publishes once.
//
// The group's path names none of Nếp's memory: no HoSo, no NganHanLuot, no
// memory tool (its BoiCanh holds the group's own tool table, ChoNhom), and
// nothing a tool writes is ever committed (the group has no write tool).
// internal/aigate walks it from the worker's group root.

// The group assistant's generation settings and bounds.
const (
	nhomTen       = "rudi_ai"
	nhomNhietDo   = 0.0
	nhomMaxTokens = 1024
	// NhomMaxChu is the group answer's ceiling in runes, the card's text
	// ceiling (companion.MaxReplyText; a test holds them equal).
	NhomMaxChu = 1500
	// maxLuotNhom is how many shared turns reach the model:
	// trinho.MaxLuotNhom, the bundle's ceiling.
	maxLuotNhom = trinho.MaxLuotNhom
)

// khuonNhom is the output shape of a room's answer; a couple's (doi) never
// quotes the couple's instruction, a room of friends' never the group's.
func khuonNhom(doi bool) khuon {
	loiNhac := prompts.LoiNhacNhom()
	if doi {
		loiNhac = prompts.LoiNhacDoi()
	}
	return khuon{maxChu: NhomMaxChu, loiNhac: loiNhac, cauChan: cau.CauNhom(cau.TuChoiNhom)}
}

// The two classes of a room (decision 2026-09-28, ADR-0046 §8.4). Every
// room runs the group's path whole: the same router schema, tools, policy,
// split draft and card. What a couple (Turn.Doi: a chat of two whose two
// people both turned on «Một đôi») changes is only who the words speak to:
// its system instruction, its out-of-scope clause, the fixed sentences that
// name the audience, and the record (bot doi, the couple's prompt version);
// and one tool, the couple's shared taste (gu_hai_ban, ADR-0048), declared on
// a couple's turn only (tools.BoiCanh.Doi). A chat of two among friends is
// not a couple: it reads the group's words and never sees that tool.

// agentPhong is the room's system instruction carrying the canary marker.
func agentPhong(t Turn, maKiem string) string {
	if t.Doi {
		return prompts.DoiAgent(maKiem)
	}
	return prompts.NhomAgent(maKiem)
}

// loiDanNhanPhong is the room's clause for the router's label nhan.
func loiDanNhanPhong(t Turn, nhan string) string {
	if t.Doi {
		return prompts.LoiDanNhanDoi(nhan)
	}
	return prompts.LoiDanNhanNhom(nhan)
}

// phienBanPhong is the prompt version the room's record carries.
func phienBanPhong(t Turn) obs.PromptVersion {
	if t.Doi {
		return obs.PromptVersion(prompts.VersionDoi())
	}
	return obs.PromptVersion(prompts.VersionNhom())
}

// cauPhong picks the room's fixed sentence: the couple's when t is a
// couple's turn.
func cauPhong(t Turn, nhom, doi string) string {
	if t.Doi {
		return doi
	}
	return nhom
}

// RunNhom runs one group turn to its end (Run sends Bot nhom here). The
// worker's group path calls it directly, so what it can reach holds
// nothing of Nếp's path. A couple's turn (t.Doi) runs the same path; its
// record names bot doi and the couple's prompt version, while the turn
// itself stays Bot nhom for every routing decision.
func (e *Engine) RunNhom(ctx context.Context, t Turn, s Sink) (Result, error) {
	return e.boc(ctx, t, s, phienBanPhong(t), t.SoTin, e.nhomVaPhat)
}

// nhomVaPhat is the group's turn, then the release of its card's text the
// way Nếp's answer is released (draft, verify, stream: phatRa through the
// output guard's window, paced), and the text part set to exactly what the
// Deltas carried.
func (e *Engine) nhomVaPhat(ctx context.Context, t Turn, s Sink, rec *obs.TurnRecord, batDau time.Time) (Result, error) {
	if t.Doi && t.Bot == obs.BotNhom {
		// The record only: nothing downstream routes on rec.Bot.
		rec.Bot = obs.BotDoi
	}
	res, err := e.nhom(ctx, t, s, rec, batDau)
	if err != nil {
		return res, err
	}
	phat, err := e.phatRa(ctx, res, s, rec, khuonNhom(t.Doi))
	if err != nil {
		return Result{}, err
	}
	if phat.Text != res.Text {
		for i, p := range phat.Phan {
			var v phanTho
			if json.Unmarshal(p, &v) == nil && v.Kind == string(PhanText) {
				phat.Phan[i] = phan(PhanText, map[string]string{"text": phat.Text})
			}
		}
	}
	return phat, nil
}

// luotNhomSach is one shared turn after structural cleaning.
type luotNhomSach struct {
	luot trinho.Luot
	// chu is the message's own words, without the speaker's label.
	id, chu, ten, tacGia string
	// chuMayChu is the server's stored text of the message, cleaned the same
	// way; "" when the server has none (LuotNhom.ChuMayChu).
	chuMayChu string
}

// nganHanNhom cleans the shared turns structurally (NFC, invisible
// characters) and keeps the newest maxLuotNhom; a turn with no text or an
// unknown speaker is left out and counted (rec.LuotBo). Nothing is dropped
// for its words. A v2 room's turns are all left out: the server keeps and
// shows nothing of an end-to-end encrypted room.
func nganHanNhom(t Turn, rec *obs.TurnRecord) []luotNhomSach {
	if t.Lane != LaneLegacy {
		rec.LuotBo += len(t.LuotNhom)
		return nil
	}
	var out []luotNhomSach
	for _, l := range t.LuotNhom {
		c := preprocess.LamSach(l.Chu)
		rec.KyTuAn += c.KyTuAn
		if c.Chu == "" {
			rec.LuotBo++
			continue
		}
		s := luotNhomSach{id: l.ID, chu: c.Chu, tacGia: l.TacGia, chuMayChu: preprocess.LamSach(l.ChuMayChu).Chu}
		switch l.Vai {
		case "toi":
			s.luot = trinho.Luot{Vai: trinho.Toi, Chu: c.Chu, Luc: t.Luc}
		case "ban":
			s.ten = preprocess.LamSach(l.Ten).Chu
			chu := c.Chu
			if s.ten != "" {
				chu = s.ten + ": " + c.Chu
			}
			s.luot = trinho.Luot{Vai: trinho.Ban, Chu: chu, Luc: t.Luc}
		case "ai":
			s.luot = trinho.Luot{Vai: trinho.TroLy, Chu: c.Chu, Luc: t.Luc}
		default:
			rec.LuotBo++
			continue
		}
		out = append(out, s)
	}
	if len(out) > maxLuotNhom {
		rec.LuotBo += len(out) - maxLuotNhom
		out = out[len(out)-maxLuotNhom:]
	}
	return out
}

// biDanhThanhVien is member i's alias in the router's closed list.
func biDanhThanhVien(i int) string { return "m" + strconv.Itoa(i+1) }

// thanhVienRouter is the members as the router sees them: aliases and the
// roster's labels, never a person id.
func thanhVienRouter(ts []ThanhVienNhom) []hieu.ThanhVien {
	out := make([]hieu.ThanhVien, 0, len(ts))
	for i, m := range ts {
		ten := preprocess.LamSach(m.Ten).Chu
		if ten == "" {
			ten = "Bạn " + strconv.Itoa(i+1)
		}
		out = append(out, hieu.ThanhVien{ID: biDanhThanhVien(i), Ten: ten})
	}
	return out
}

// epLenh applies the person's explicit command to the router's result, a
// lookup on closed values: /plan asks for a plan, whatever else the message
// holds, so plan is the first intent; /chia-bill asks for a split draft.
// It never lowers a money label: money_action stays refused.
func epLenh(lenh obs.Lenh, kq hieu.KetQua) hieu.KetQua {
	switch lenh {
	case obs.LenhPlan:
		if !coYDinh(kq.YDinh, hieu.Plan) {
			kq.YDinh = append([]hieu.YDinh{hieu.Plan}, kq.YDinh...)
			if len(kq.YDinh) > hieu.MaxYDinh {
				kq.YDinh = kq.YDinh[:hieu.MaxYDinh]
			}
		}
	case obs.LenhChiaBill:
		if kq.Tien == hieu.TienNone {
			kq.Tien = hieu.SplitDraft
		}
	}
	return kq
}

func (e *Engine) nhom(ctx context.Context, t Turn, s Sink, rec *obs.TurnRecord, batDau time.Time) (Result, error) {
	hoi := preprocess.LamSach(t.LoiNho)
	rec.KyTuAn += hoi.KyTuAn
	rec.KhongDau = hoi.KhongDau
	if hoi.Chu == "" {
		// A bare «@Rủ Đi» or «/plan» still asks for something: the fixed
		// request of the command stands in for the words.
		switch t.Lenh {
		case obs.LenhPlan:
			hoi.Chu = cauPhong(t, cau.NhomLoiNhoPlan, cau.DoiLoiNhoPlan)
		case obs.LenhChiaBill:
			hoi.Chu = cau.NhomLoiNhoChiaBill
		default:
			return Result{}, &Loi{Ma: cau.InvalidAIResult}
		}
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
	demNhung := nhung.MoiDemLuot()
	runCtx = nhung.VoiDemLuot(runCtx, demNhung)
	var demXL *rerank.Dem
	if e.xepLai != nil {
		demXL = rerank.NewDem(e.xepLai, llm.MaxRerankCallsPerTurn)
		runCtx = truyhoi.VoiXepLai(runCtx, demXL)
	}
	loi := func(err error) error {
		return loiMoHinh(err, errors.Is(ctx.Err(), context.Canceled), errors.Is(runCtx.Err(), context.DeadlineExceeded), rec)
	}
	chung := nganHanNhom(t, rec)
	luot := make([]trinho.Luot, 0, len(chung))
	for _, c := range chung {
		luot = append(luot, c.luot)
	}
	dsDiemDen := e.danhSachDiemDen(runCtx)
	v := hieu.Vao{
		Bot:               obs.BotNhom,
		Cau:               hoi.Chu,
		Luc:               t.Luc,
		NganHan:           luot,
		DanhSachDiemDen:   dsDiemDen,
		DanhSachThanhVien: thanhVienRouter(t.ThanhVien),
		Doi:               t.Doi,
		DemNhung:          demNhung,
	}
	rec.MsTienXuLy = ms(e.now().Sub(batDau))
	s.TrangThai(cau.DangNghi, t.SoTin)

	moHinh := e.now()
	sc := tools.MoiSoCai(obs.BotNhom)
	defer func() {
		rec.MsMoHinh = ms(e.now().Sub(moHinh))
		rec.SoGoiMoHinh = dem.SoGoi()
		tok := dem.Token()
		rec.TokensIn, rec.TokensOut, rec.TokensCache, rec.TokensNghi = tok.In, tok.Out, tok.Cache, tok.Thought
		rec.SoCongCu = sc.TongGoi()
		if demXL != nil {
			rec.SoXepLai = demXL.SoGoi()
		}
		rec.CongCu = obs.CacCongCu{}
		for _, c := range sc.DaChay() {
			rec.CongCu = append(rec.CongCu, obs.CongCu(c))
		}
	}()

	kq, err := e.hieu.Hieu(runCtx, v, dem)
	if err != nil {
		if ma, ok := hieu.MaLoi(err); ok {
			rec.LoiMoHinh = obs.LoiBadResp
			if errors.Is(err, hieu.ErrBiChan) {
				rec.LoiMoHinh = obs.LoiSafety
			}
			return Result{}, &Loi{Ma: ma}
		}
		return Result{}, loi(err)
	}
	ghiNhanRouter(rec, kq)
	kq = epLenh(t.Lenh, kq)
	q := hieu.QuyetDinhCho(obs.BotNhom, kq)
	if q.TuChoiTien {
		// The model classed a money action: the fixed refusal, no further
		// call, no tool, no draft. It is an answer the room reads, not a
		// failure.
		rec.Guard, rec.Duong = obs.GuardRefused, obs.DuongTuChoiTien
		return theMotChu(cauPhong(t, cau.NhomKhongChamTien, cau.DoiKhongChamTien)), nil
	}
	if q.NhapTien || coYDinh(kq.YDinh, hieu.ChiaBillDraft) {
		if q.HanChe {
			rec.Guard = obs.GuardRestricted
		}
		rec.Duong = obs.DuongNhapChiaBill
		res, err := e.nhapChiaBill(runCtx, t, rec, dem, hoi.Chu, chung, kq)
		if err != nil && !isLoi(err) {
			return Result{}, loi(err)
		}
		return res, err
	}
	if q.HanChe {
		rec.Guard = obs.GuardRestricted
	}
	if q.HoiLai {
		rec.Duong = obs.DuongHoiLai
		res, err := e.hoiLai(runCtx, rec, dem, kq, khuonNhom(t.Doi))
		if err != nil {
			if !isLoi(err) {
				return Result{}, loi(err)
			}
			return Result{}, err
		}
		chu := res.Text
		for _, c := range res.LuaChon {
			chu += "\n• " + c
		}
		out := theMotChu(chu)
		out.LuaChon = res.LuaChon
		return out, nil
	}
	cung, mem, err := tools.RangBuocTuRouter(kq.Slots, t.Luc)
	if err != nil {
		rec.LoiMoHinh = obs.LoiBadResp
		return Result{}, &Loi{Ma: cau.InvalidAIResult}
	}
	bc := &tools.BoiCanh{
		Bot: obs.BotNhom, NguoiHoi: t.NguoiHoi, NhomID: t.Phong, LoiNguoiHoi: hoi.Chu, Luc: t.Luc, HanChe: q.HanChe, YDinh: kq.YDinh,
		Cung: cung, Mem: mem, DiUngNgoaiDanhMuc: kq.Slots.DiUngNgoaiDanhMuc, DiemDen: idsDiemDen(dsDiemDen),
		Nguon: nguonNhom(e.nguon), Quyen: e.quyen, SoCai: sc, Che: []tools.Ten{tools.DraftPoll},
		Doi: t.Doi, NhanDoi: nhanThanhVien(t.ThanhVien),
	}
	bc.ChoNhom()
	khoi := []string{prompts.BocDuLieu(prompts.MayChu, strings.Join(dongMayChuHieu(t.Luc, kq.Slots), "\n"))}
	k := khuonNhom(t.Doi)
	if ten, _, ok := tactu.Nhanh(kq, bc); ok && !q.KhongCongCu && ten == tools.SearchPlaces && t.Lenh != obs.LenhPlan {
		if res, chay, err := e.nepTruyHoi(runCtx, rec, dem, kq, bc, hoi.Chu, ten, k); chay {
			if err != nil {
				if !isLoi(err) {
					return Result{}, loi(err)
				}
				return Result{}, err
			}
			res, err = e.luuYDiUng(res, kq, rec, k)
			if err != nil {
				return Result{}, err
			}
			out := phanNhom(res.Text, res.QuanIDs, nil)
			out.GuDung = guDaDung(sc, t)
			return out, nil
		}
	}
	if dem.ConLai() < 1+duTruKiem {
		return Result{}, &Loi{Ma: cau.HetNganSach}
	}
	var td agent.TheoDoi
	var text string
	var nhap tools.BanNhap
	ngan, phien, xong := e.bamPhienNhom(runCtx, t, luot)
	defer xong()
	// A /plan always goes to the tools (a plan is an itinerary the model
	// builds from evidence); otherwise the router's path decides.
	thang := (kq.Huong == hieu.TraLoiThang && t.Lenh != obs.LenhPlan) || q.KhongCongCu
	if thang {
		rec.Duong = obs.DuongThang
		blocks := khoi
		if s := tactu.KhoiLichSu(luot, nil); s != "" {
			blocks = append([]string{s}, blocks...)
		}
		blocks = append(blocks, prompts.BocDuLieuDanhDau(prompts.CauHoi, hoi.Chu))
		instruction := agentPhong(t, e.maKiem) + "\n\n" + prompts.LoiDanThang()
		if c := loiDanNhanPhong(t, string(kq.NhanGuard)); q.KhongCongCu && c != "" {
			instruction += "\n\n" + c
		}
		cfg := agent.CauHinh{
			Ten:             nhomTen,
			Instruction:     instruction,
			NhietDo:         nhomNhietDo,
			MaxOutputTokens: nhomMaxTokens,
			MaxBuoc:         1,
			ConLai:          func() int { return dem.ConLai() - duTruKiem },
		}
		text, err = agent.Chay(runCtx, dem, cfg, nil, strings.Join(blocks, "\n\n"), &td)
	} else {
		var ra tactu.Ra
		ra, err = tactu.Chay(runCtx, dem, tactu.Vao{
			Ten: nhomTen, Instruction: agentPhong(t, e.maKiem), NhietDo: nhomNhietDo, MaxTokens: nhomMaxTokens,
			Router: kq, Cau: hoi.Chu, KhoiThem: khoi, NganHan: ngan, Phien: phien, BoiCanh: bc,
			DuTru: duTruKiem,
		}, &td)
		text, nhap = ra.Text, ra.Nhap
		rec.Duong = obs.DuongTacTu
		if ra.Nhanh {
			rec.Duong = obs.DuongNhanh
		}
	}
	snap := td.Snapshot()
	rec.Buoc = snap.Buoc
	if err != nil {
		return Result{}, loi(err)
	}
	if llm.BiChanAnToan(snap.Finish) {
		rec.LoiMoHinh = obs.LoiSafety
		return Result{}, &Loi{Ma: cau.InvalidAIResult}
	}
	res, err := e.xacMinh(runCtx, rec, dem, text, sc, nil, k)
	if err != nil {
		if !isLoi(err) {
			return Result{}, loi(err)
		}
		return Result{}, err
	}
	res, err = e.luuYDiUng(res, kq, rec, k)
	if err != nil {
		return Result{}, err
	}
	out := phanNhom(res.Text, nhap.Quan, nhap.LichTrinh)
	out.GuDung = guDaDung(sc, t)
	return out, nil
}

// nhanThanhVien is each member's label as the router reads it
// (thanhVienRouter: the roster's label, or «Bạn n»), by person id.
func nhanThanhVien(ts []ThanhVienNhom) map[string]string {
	out := map[string]string{}
	for i, v := range thanhVienRouter(ts) {
		out[ts[i].ID] = v.Ten
	}
	return out
}

// guDaDung are the people whose shared taste the ledger holds, with the
// label the model read beside it. Only a couple's turn can have any: the
// tool is declared and run nowhere else (tools.BoiCanh.Doi).
func guDaDung(sc *tools.SoCai, t Turn) []NguoiGu {
	nhan := nhanThanhVien(t.ThanhVien)
	var out []NguoiGu
	for _, id := range sc.GuDaDung() {
		out = append(out, NguoiGu{ID: id, Nhan: nhan[id]})
	}
	return out
}

// bamPhienNhom buffers a legacy-lane group turn's shared messages in the
// short-term store, one key per invocation, and returns the port and key
// the tool part reads and the release that drops the key when the turn
// ends. A v2 room has no key (the store refuses its lane; the turns are
// already none); without a store, or when the buffer cannot be written, the
// turns stay in memory.
func (e *Engine) bamPhienNhom(ctx context.Context, t Turn, luot []trinho.Luot) (trinho.NganHan, string, func()) {
	trongRam := func() (trinho.NganHan, string, func()) { return phienThietBi(luot), phienNep, func() {} }
	if e.nganHanNhom == nil || t.Phong == "" || t.InvocationID == "" || len(luot) == 0 {
		return trongRam()
	}
	phien, err := e.nganHanNhom.PhienLuotNhom(t.Phong, t.InvocationID, t.Lane)
	if err != nil {
		return trongRam()
	}
	xoa := func() { _ = e.nganHanNhom.Xoa(context.WithoutCancel(ctx), phien) }
	for _, l := range luot {
		if err := e.nganHanNhom.Them(ctx, phien, l); err != nil {
			xoa()
			return trongRam()
		}
	}
	return e.nganHanNhom, phien, xoa
}

// phanTho is one part of a `tra_loi` card in the raw form
// companion.GroundReply grounds.
type phanTho struct {
	Kind    string `json:"kind"`
	Payload any    `json:"payload"`
}

type chang struct {
	PlaceID  string `json:"place_id"`
	TimeText string `json:"time_text"`
	Note     string `json:"note"`
}

func phan(kind PhanKind, payload any) json.RawMessage {
	raw, _ := json.Marshal(phanTho{Kind: string(kind), Payload: payload})
	return raw
}

// theMotChu is a card of one text part.
func theMotChu(chu string) Result {
	return Result{Text: chu, Phan: []json.RawMessage{phan(PhanText, map[string]string{"text": chu})}}
}

// phanNhom lays out the card of a prose answer: the places part and the
// itinerary part first, when the turn's drafts or its grounded retrieval
// made them (ids from the ledger, never from the model's words), then the
// verified text. Every id is named in QuanIDs for the worker's catalogue
// read.
func phanNhom(chu string, quan []string, lt *tools.LichTrinh) Result {
	res := Result{Text: chu}
	them := func(id string) {
		if !coChuoi(res.QuanIDs, id) {
			res.QuanIDs = append(res.QuanIDs, id)
		}
	}
	if len(quan) > 0 {
		for _, id := range quan {
			them(id)
		}
		res.Phan = append(res.Phan, phan(PhanPlaces, map[string]any{"intro": "", "place_ids": quan}))
	}
	if lt != nil && len(lt.Chang) > 0 {
		stops := make([]chang, 0, len(lt.Chang))
		for _, c := range lt.Chang {
			them(c.ID)
			stops = append(stops, chang{PlaceID: c.ID, TimeText: c.Gio, Note: ""})
		}
		res.Phan = append(res.Phan, phan(PhanItinerary, map[string]any{"title": tieuDeLichTrinh(lt.NgayISO), "stops": stops}))
	}
	res.Phan = append(res.Phan, phan(PhanText, map[string]string{"text": chu}))
	return res
}

// tieuDeLichTrinh is the itinerary's title: the date the model wrote, laid
// on Vietnam's calendar by arithmetic, or a fixed title.
func tieuDeLichTrinh(ngayISO string) string {
	if d, err := time.Parse(time.DateOnly, ngayISO); err == nil {
		return "Lịch trình " + thoigian.Ngay{Nam: d.Year(), Thang: int(d.Month()), Ngay: d.Day()}.String()
	}
	return "Lịch trình đề xuất"
}

func coChuoi(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// nguonNhom is the group's read ports: the catalogue, the destinations, the
// room and a couple's shared taste (read only on a couple's turn, only for
// people who shared it with the chat: ADR-0048), and nothing of a person's
// own (no CaNhan, no TriNho). One engine
// serves both bots, so the group's tool context is built from a copy that
// never holds Nếp's ports: a tool mis-registered for the group still finds
// no memory to read or write (review of slices 9/11, finding 2.5).
func nguonNhom(n tools.NguonDuLieu) tools.NguonDuLieu {
	return tools.NguonDuLieu{Quan: n.Quan, Cho: n.Cho, Nhom: n.Nhom, Doi: n.Doi}
}
