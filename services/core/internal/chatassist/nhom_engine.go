package chatassist

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/domain/chatintent"
	"mobile/services/core/internal/domain/companion"
	"mobile/services/core/internal/domain/tree"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
	"mobile/services/core/internal/treejson"
)

// The group assistant on the Go engine (slice 9, MOBILE_AI_ENGINE_GROUP=go;
// the default stays the brain). The same queue, lease, stream, one publish
// as a reply to the tag message, and the same failure codes: only the
// inference step differs. The engine gets what the caller explicitly shared
// (the bundle's turns, the reply chain in it) and what the server owns and
// never encrypted (who is in the room under the roster's labels, who wrote
// each shared turn), never a word of the conversation read from the
// database; it reads the catalogue through its own tool ports.

// WithNhomEngine runs the group's jobs on the Go engine.
func (h *Handler) WithNhomEngine(e *aiharness.Engine) *Handler {
	h.nhomEngine, h.nhomGo = e, e != nil
	return h
}

// WithNhomGo marks a process that serves the group's routes while its jobs
// run on the Go engine in `core work`: `hoi` is taken.
func (h *Handler) WithNhomGo() *Handler {
	h.nhomGo = true
	return h
}

// obsLenh is the job's command as the engine's closed set names it.
func obsLenh(command string) (obs.Lenh, bool) {
	switch command {
	case lenhPlan:
		return obs.LenhPlan, true
	case lenhChiaBill:
		return obs.LenhChiaBill, true
	case lenhHoi:
		return obs.LenhHoi, true
	}
	return "", false
}

// loiNhoNhom is the request's words without the explicit command syntax the
// person typed (/plan, /chia-bill, @Rủ Đi at the start): the command is
// already the job's, so the router reads what is asked, not how the app was
// called. chatintent.Parse is that syntax's one reader (contract §8: a
// consent gate, never intent guessing).
func loiNhoNhom(prompt string) string {
	if p := chatintent.Parse(prompt); p != nil {
		return p.Args
	}
	return prompt
}

// luotNhom is the bundle's turns for the engine, each with its author as
// messages.author_id has it (tacGia) and a friend's label read through the
// same test the roster applies (tenDoc).
func luotNhom(goi []byte, authors map[string]string) ([]aiharness.LuotNhom, error) {
	if len(goi) == 0 {
		return nil, nil
	}
	var bc bundle
	if err := json.Unmarshal(goi, &bc); err != nil {
		return nil, err
	}
	out := make([]aiharness.LuotNhom, 0, len(bc.Luot))
	for _, l := range bc.Luot {
		if l.Loai != "chu" {
			continue
		}
		x := aiharness.LuotNhom{ID: l.ID, Vai: l.Vai, Chu: l.Chu, TacGia: authors[l.ID]}
		if l.Vai == "ban" {
			x.Ten = tenDoc(l.BiDanh)
		}
		if l.Vai == "ai" {
			// An earlier answer has no author: nothing of it is ever billed.
			x.TacGia = ""
		}
		out = append(out, x)
	}
	return out, nil
}

// thanhVienNhom is the room's active members under the labels the room
// knows them by (tenThanhVien: a display name safe for a model, "" when
// none), the caller first.
func thanhVienNhom(ms []repo.Membership, caller string) []aiharness.ThanhVienNhom {
	var out []aiharness.ThanhVienNhom
	for _, m := range ms {
		if m.State == "active" && m.PersonID == caller {
			out = append(out, aiharness.ThanhVienNhom{ID: m.PersonID, Ten: tenThanhVien(m)})
		}
	}
	for _, m := range ms {
		if m.State == "active" && m.PersonID != caller {
			out = append(out, aiharness.ThanhVienNhom{ID: m.PersonID, Ten: tenThanhVien(m)})
		}
	}
	return out
}

// chuanBiNhom confirms the job is still the caller's in a room they are in,
// and reads what the server lays on top of the bundle: the members and the
// author of each shared turn. Never `body`.
func (h *Handler) chuanBiNhom(ctx context.Context, j work) ([]repo.Membership, map[string]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)
	g, err := authority(ctx, tx, j.conversation, j.digest)
	if err != nil {
		return nil, nil, err
	}
	if g.member != j.member || g.person != j.person || g.kind != "group" {
		return nil, nil, &denied{403, "sharing_unavailable"}
	}
	var live bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat_ai_invocations WHERE id=$1 AND status='running' AND lease_id=$2 AND lease_until>clock_timestamp() AND share_expires_at>clock_timestamp())`, j.id, j.lease).Scan(&live); err != nil {
		return nil, nil, err
	}
	if !live {
		return nil, nil, &denied{409, "invocation_cancelled"}
	}
	ms, err := repo.Repository{Q: tx}.ListMembers(ctx, j.conversation)
	if err != nil {
		return nil, nil, err
	}
	authors := map[string]string{}
	if len(j.goi) > 0 {
		var bc bundle
		if err = json.Unmarshal(j.goi, &bc); err != nil {
			return nil, nil, err
		}
		if authors, err = tacGia(ctx, tx, j.conversation, &bc); err != nil {
			return nil, nil, err
		}
	}
	return ms, authors, tx.Commit(ctx)
}

// processNhomEngine runs one group job on the Go engine: the turn from the
// stored job and the room, the engine (its verified text streamed to the
// job's stream through the output guard's window as it is released), the
// card grounded by companion.GroundReply against the catalogue rows of the
// places it names, and one publish as a reply to the tag message. A turn
// stopped from outside touches nothing (aiharness.ErrHuy). One metrics row
// follows the terminal transition and never decides it.
func (h *Handler) processNhomEngine(ctx context.Context, j work) error {
	lenh, ok := obsLenh(j.command)
	if !ok {
		return h.finishFailure(ctx, j, "invalid_ai_result")
	}
	ms, authors, err := h.chuanBiNhom(ctx, j)
	if err != nil {
		return h.finishFailure(ctx, j, "sharing_unavailable")
	}
	luot, err := luotNhom(j.goi, authors)
	if err != nil {
		return h.finishFailure(ctx, j, "invalid_ai_result")
	}
	lane := j.lane
	if lane == "" {
		lane = laneLegacy
	}
	turn := aiharness.Turn{
		Bot: obs.BotNhom, InvocationID: j.id, LanThu: j.attempt, Lenh: lenh, Luc: j.createdAt,
		LoiNho: loiNhoNhom(j.prompt), NguoiHoi: j.person, DaGoiTruoc: j.modelCalls, GiuLuot: h.giuLuot(j),
		Phong: j.conversation, Lane: lane, SoTin: j.soTin, LuotNhom: luot, ThanhVien: thanhVienNhom(ms, j.person),
	}
	res, runErr := h.nhomEngine.RunNhom(ctx, turn, j.luong.sink())
	if errors.Is(runErr, aiharness.ErrHuy) {
		return runErr
	}
	switch {
	case runErr == nil:
		var card []byte
		card, err = h.theNhomEngine(ctx, j, res)
		if err != nil {
			err = h.finishFailure(ctx, j, "invalid_ai_result")
			break
		}
		// The text already reached the stream through the engine's window
		// (draft, verify, stream); publish posts the card and ends the
		// stream with its id, releasing nothing again.
		err = h.publishCo(ctx, j, card, res.KetQuaNhap, false)
	case aiharness.TamThoi(runErr) && !dangDung(ctx):
		var later bool
		if later, err = h.retryLater(ctx, j); err == nil && !later {
			err = h.finishFailure(ctx, j, maNhom(runErr))
		}
	default:
		err = h.finishFailure(ctx, j, maNhom(runErr))
	}
	h.ghiSoDo(ctx, res.Record)
	return err
}

// maNhom is the job code of a group turn that ended without an answer: the
// engine's code, except a stopped answer, which the job row and the room see
// only as the generic ai_tu_choi (design 01 §3.3).
func maNhom(err error) string {
	if aiharness.MaCua(err) == cau.TraLoiBiChan {
		return maChanChung
	}
	return string(aiharness.MaCua(err))
}

// theNhomEngine grounds the engine's card: the catalogue rows of the places
// it names, read in a READ ONLY transaction and shaped as the model was
// allowed to see them (service.ClientPlaces), then companion.GroundReply --
// every place id must be one of those rows, every part within its bounds.
// A job from a client that names no trigger gets the one text card it has
// always drawn (GroundCard).
func (h *Handler) theNhomEngine(ctx context.Context, j work, res aiharness.Result) ([]byte, error) {
	places, err := h.hangCatalogue(ctx, res.QuanIDs)
	if err != nil {
		return nil, err
	}
	parts := make([]tree.Value, 0, len(res.Phan))
	for _, p := range res.Phan {
		v, err := pyjson.Loads(p)
		if err != nil {
			return nil, err
		}
		parts = append(parts, treejson.To(v))
	}
	if j.trigger == "" {
		for _, p := range parts {
			if g, err := companion.GroundCard(p, places); err == nil {
				if k, _ := g.Get("kind"); k == tree.String("text") {
					return pyjson.Dumps(treejson.From(g))
				}
			}
		}
		return nil, errors.New("chatassist: no text part for a client without a trigger")
	}
	grounded, err := companion.GroundReply(companion.ReplyMeta{InvocationID: j.id, Command: j.command, Read: j.soTin}, parts, places)
	if err != nil {
		return nil, err
	}
	return pyjson.Dumps(treejson.From(grounded))
}

// hangCatalogue reads the catalogue rows of ids.
func (h *Handler) hangCatalogue(ctx context.Context, ids []string) ([]*tree.OrderedMap, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := repo.Repository{Q: tx}.PlacesByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	cards := make([]*pyjson.OrderedMap, 0, len(rows))
	for _, r := range rows {
		cards = append(cards, service.PlaceRow(r))
	}
	return treejson.MapsTo(service.ClientPlaces(cards)), tx.Commit(ctx)
}
