package chatassist

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/brain"
	"mobile/services/core/internal/chatv2"
	"mobile/services/core/internal/pyjson"
)

// Nếp's words (ADR-0036 §2.6-§2.8, ADR-0033 §2.5-§2.6).
//
// Nếp is the person's own assistant, so its questions ride the same queue,
// lease, retry, idempotency and rate limit as the group AI, under
// `scope='me'`: no room, no membership, and a result that goes back to the
// caller alone and is never written into any room.
//
// What reaches the model is exactly what the device sends, and nothing the
// server looks up: the screen's context slip (already through `donPhieu` on the
// device, re-checked here against the same closed vocabulary), the turns of the
// panel session that is open right now, and the question. The server reads no
// chat, no taste, no history and no table for context -- `nep_khong_doc_test.go`
// walks every function this path can reach and holds that to the SQL.

const (
	scopeMe = "me"
	// The one command the schema allows for `scope='me'` (chat_ai_command_scope).
	lenhHoi = "hoi"
)

// Bounds on the panel session. They refuse rather than truncate, for the
// reason `kiemBoiCanh` gives: the device prints how many turns go along, and a
// server that silently dropped some would make that sentence false.
const (
	maxLuotNep     = 24
	maxChuLuotNep  = 2000
	maxHoiNep      = 2000
	maxTongRuneNep = 16000
	// Nếp answers in a few sentences; anything past this is not an answer the
	// panel was built to show.
	maxTraLoiNep = 2000
)

// luotNep is one turn of the open panel session, as the device keeps it in
// memory. Two voices only: the person, and Nếp.
type luotNep struct {
	Vai string `json:"vai"`
	Chu string `json:"chu"`
}

// nhipPhieu mirrors `NhipKeo` (keo/nhip-keo.ts).
type nhipPhieu struct {
	Kieu      string `json:"kieu"`
	ConNgay   *int   `json:"conNgay,omitempty"`
	TruocNgay *int   `json:"truocNgay,omitempty"`
}

// phieuNep mirrors `PhieuNguCanh` (nep/phieu.ts). The decoder refuses unknown
// fields, so the top level is closed here exactly as `donPhieu` closes it.
type phieuNep struct {
	Man    string         `json:"man"`
	TieuDe string         `json:"tieuDe,omitempty"`
	Nhip   *nhipPhieu     `json:"nhip,omitempty"`
	LoaiSo string         `json:"loaiSo,omitempty"`
	SoLieu map[string]any `json:"soLieu,omitempty"`
	GoiY   []string       `json:"goiY,omitempty"`
}

// goiNep is what is stored in `boi_canh` for the life of the job, and digested.
type goiNep struct {
	Phieu *phieuNep `json:"phieu"`
	Luot  []luotNep `json:"luot"`
}

var (
	// The same closed lists as phieu.ts. `nep_phieu_test.go` reads phieu.ts and
	// fails when the two drift.
	manNepLui   = []string{"finance", "settlements", "batches", "smart-split"}
	khoaSoLieu  = map[string]bool{"soNguoi": true, "soChang": true, "soAnh": true, "soNgay": true, "soMuc": true, "soViec": true}
	kieuNhip    = map[string]bool{"sap-toi": true, "hom-nay": true, "dang-dien-ra": true, "da-qua": true, "khong-ro": true}
	loaiSoHopLe = map[string]bool{"hoi": true, "hai-nguoi": true, "doi": true}
	vaiNep      = map[string]bool{"toi": true, "nep": true}
)

// nepPhaiLui is `nepPhaiLui` of phieu.ts: whole first segment, never a prefix,
// so `financial-report` is not a money screen.
func nepPhaiLui(man string) bool {
	dau := strings.Split(strings.TrimLeft(man, "/"), "/")[0]
	for _, m := range manNepLui {
		if dau == m {
			return true
		}
	}
	return false
}

func chuTrongHan(s string, han int) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) <= han
}

// kiemPhieu re-checks the slip against the whitelist the device applied. It
// refuses instead of dropping: a device that sends a field `donPhieu` would
// have dropped is not the shipped app, and quietly forwarding the rest would
// make «Mình đang thấy» a claim nobody checked.
func kiemPhieu(p *phieuNep) error {
	sai := &denied{400, "boi_canh_sai_dang"}
	if strings.TrimSpace(p.Man) == "" || !chuTrongHan(p.Man, 120) || !chuTrongHan(p.TieuDe, 80) {
		return sai
	}
	if p.Nhip != nil && !kieuNhip[p.Nhip.Kieu] {
		return sai
	}
	if p.LoaiSo != "" && !loaiSoHopLe[p.LoaiSo] {
		return sai
	}
	for khoa, v := range p.SoLieu {
		if !khoaSoLieu[khoa] {
			return sai
		}
		switch x := v.(type) {
		case float64:
		case string:
			if strings.TrimSpace(x) == "" || !chuTrongHan(x, 24) {
				return sai
			}
		default:
			return sai
		}
	}
	if len(p.GoiY) > 3 {
		return sai
	}
	for _, g := range p.GoiY {
		if !chuTrongHan(g, 80) {
			return sai
		}
	}
	return nil
}

// kiemNep is every check on a Nếp request that needs no database, in the order
// that decides the answer. The money law comes before the bounds: a request
// from a money screen is refused as such however it is shaped, and it never
// costs a model call (ADR-0033 §2.2, ADR-0036 §2.9).
func kiemNep(prompt string, g *goiNep) error {
	if g.Phieu != nil && nepPhaiLui(g.Phieu.Man) {
		return &denied{403, "nep_lui_man_tien"}
	}
	if strings.TrimSpace(prompt) == "" || !chuTrongHan(prompt, maxHoiNep) {
		return invalid("invalid_invocation")
	}
	if g.Phieu != nil {
		if err := kiemPhieu(g.Phieu); err != nil {
			return err
		}
	}
	if len(g.Luot) > maxLuotNep {
		return &denied{413, "boi_canh_qua_lon"}
	}
	tong := utf8.RuneCountInString(prompt)
	for _, l := range g.Luot {
		if !vaiNep[l.Vai] || !utf8.ValidString(l.Chu) || strings.TrimSpace(l.Chu) == "" {
			return &denied{400, "boi_canh_sai_dang"}
		}
		n := utf8.RuneCountInString(l.Chu)
		if n > maxChuLuotNep {
			return &denied{413, "boi_canh_qua_lon"}
		}
		tong += n
	}
	if tong > maxTongRuneNep {
		return &denied{413, "boi_canh_qua_lon"}
	}
	return nil
}

// NepInvocation is the only shape a personal invocation leaves the server in.
// `text` is the sealed answer: present on the caller's own read, never on any
// room's feed, and gone once the sharing window closes.
type NepInvocation struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Code      *string   `json:"code"`
	Text      *string   `json:"text"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const nepColumns = `id,status,code,result->>'text',created_at,updated_at`

func scanNep(row pgx.Row) (NepInvocation, error) {
	var v NepInvocation
	err := row.Scan(&v.ID, &v.Status, &v.Code, &v.Text, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

// beginNep opens a transaction as the bearer's person, and nothing more: a
// personal invocation has no room to authorise against.
func (h *Handler) beginNep(r *http.Request) (pgx.Tx, string, []byte, error) {
	token, p := auth.BearerToken(r.Header)
	if p != nil {
		return nil, "", nil, &denied{401, "authentication_required"}
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		return nil, "", nil, err
	}
	digest := auth.TokenDigest(token)
	person, err := phien(r.Context(), tx, digest)
	if err != nil {
		_ = tx.Rollback(r.Context())
		return nil, "", nil, err
	}
	return tx, person, digest, nil
}

func (h *Handler) nepCreate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		LogicalID string    `json:"logical_id"`
		Prompt    string    `json:"prompt"`
		Phieu     *phieuNep `json:"phieu"`
		Luot      []luotNep `json:"luot"`
	}
	if err := readBody(w, r, &in, maxBodyWithBundle); err != nil {
		failure(w, err)
		return
	}
	if !chatv2.ValidID(in.LogicalID) {
		failure(w, invalid("invalid_invocation"))
		return
	}
	g := goiNep{Phieu: in.Phieu, Luot: in.Luot}
	if g.Luot == nil {
		g.Luot = []luotNep{}
	}
	// Pure, so before any transaction and before the provider probe: neither
	// a money screen nor an oversized session ever reaches the database.
	if err := kiemNep(in.Prompt, &g); err != nil {
		failure(w, err)
		return
	}
	goi, err := json.Marshal(g)
	if err != nil {
		failure(w, err)
		return
	}
	// Authenticate before the internal inference service hears about anyone.
	tx, _, _, err := h.beginNep(r)
	if err != nil {
		failure(w, err)
		return
	}
	_ = tx.Rollback(r.Context())
	available := h.available(r.Context())
	tx, person, digest, err := h.beginNep(r)
	if err != nil {
		failure(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	// The same per-person lock as the group path, so the shared rate limit
	// below counts both kinds of question in one line.
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended('chat_ai:'||$1,0))`, person); err != nil {
		failure(w, err)
		return
	}
	sum := sha256.Sum256(append([]byte(lenhHoi+"\x00"+in.Prompt+"\x00"), goi...))
	var oldHash []byte
	var oldID string
	err = tx.QueryRow(r.Context(), `SELECT id,input_digest FROM chat_ai_invocations WHERE context_id IS NULL AND person_id=$1 AND logical_id=$2`, person, in.LogicalID).Scan(&oldID, &oldHash)
	if err == nil {
		if !bytes.Equal(oldHash, sum[:]) {
			refuse(w, 409, "invocation_conflict")
			return
		}
		v, e := scanNep(tx.QueryRow(r.Context(), `SELECT `+nepColumns+` FROM chat_ai_invocations WHERE id=$1`, oldID))
		if e != nil {
			failure(w, e)
			return
		}
		if e = tx.Commit(r.Context()); e != nil {
			failure(w, e)
			return
		}
		reply(w, 200, v)
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		failure(w, err)
		return
	}
	if !available {
		refuse(w, 503, "provider_unavailable")
		return
	}
	var recent int
	if err = tx.QueryRow(r.Context(), `SELECT count(*) FROM chat_ai_invocations WHERE person_id=$1 AND created_at>clock_timestamp()-interval '1 minute'`, person).Scan(&recent); err != nil {
		failure(w, err)
		return
	}
	if recent >= 8 {
		refuse(w, 429, "invocation_rate_limited")
		return
	}
	v, err := scanNep(tx.QueryRow(r.Context(), `INSERT INTO chat_ai_invocations(id,scope,context_id,person_id,membership_id,session_digest,logical_id,input_digest,command,prompt,boi_canh,share_expires_at,status) VALUES($1,'me',NULL,$2,NULL,$3,$4,$5,$6,$7,$8,clock_timestamp()+interval '15 minutes','queued') RETURNING `+nepColumns, newID(), person, digest, in.LogicalID, sum[:], lenhHoi, in.Prompt, json.RawMessage(goi)))
	if err != nil {
		failure(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		failure(w, err)
		return
	}
	reply(w, 202, v)
}

// nepGet answers only the person who asked. Anyone else's id reads as absent,
// never as forbidden: whether someone else asked Nếp something is not theirs
// to learn.
func (h *Handler) nepGet(w http.ResponseWriter, r *http.Request) {
	if !chatv2.ValidID(r.PathValue("id")) {
		failure(w, invalid("invalid_invocation"))
		return
	}
	tx, person, _, err := h.beginNep(r)
	if err != nil {
		failure(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	v, err := scanNep(tx.QueryRow(r.Context(), `SELECT `+nepColumns+` FROM chat_ai_invocations WHERE id=$1 AND scope='me' AND context_id IS NULL AND person_id=$2`, r.PathValue("id"), person))
	if errors.Is(err, pgx.ErrNoRows) {
		refuse(w, 404, "invocation_not_found")
		return
	}
	if err != nil {
		failure(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		failure(w, err)
		return
	}
	reply(w, 200, v)
}

// nepPayload is the brain body: the slip, the session, the question. Built from
// the stored column alone.
func nepPayload(goi []byte, prompt string) (pyjson.Value, error) {
	var g goiNep
	if err := json.Unmarshal(goi, &g); err != nil {
		return nil, err
	}
	if g.Luot == nil {
		g.Luot = []luotNep{}
	}
	raw, err := json.Marshal(map[string]any{"slip": g.Phieu, "turns": g.Luot, "prompt": prompt})
	if err != nil {
		return nil, err
	}
	return pyjson.Loads(raw)
}

// docTraLoi takes the one field the brain may return and holds it to the
// panel's bounds. Anything else is an invalid result, not a partial one.
func docTraLoi(raw pyjson.Value) (string, bool) {
	obj, err := brain.AsObject(raw)
	if err != nil {
		return "", false
	}
	v, ok := obj.Get("text")
	if !ok {
		return "", false
	}
	s, ok := v.(pyjson.String)
	if !ok {
		return "", false
	}
	text := strings.TrimSpace(string(s))
	if text == "" || !chuTrongHan(text, maxTraLoiNep) {
		return "", false
	}
	return text, true
}

// processNep runs one personal job. It never calls prepare: nothing the
// server owns about a room -- roster, taste, budget, catalogue -- belongs in a
// question the person asked on their own.
func (h *Handler) processNep(ctx context.Context, j work) error {
	if err := h.nepConSong(ctx, j); err != nil {
		return h.nepThatBai(ctx, j, "sharing_unavailable")
	}
	payload, err := nepPayload(j.goi, j.prompt)
	if err != nil {
		return h.nepThatBai(ctx, j, "invalid_ai_result")
	}
	inference, cancel := context.WithTimeout(ctx, 60*time.Second)
	raw, err := h.brain.PostJSONContext(inference, "nep-reply", payload)
	cancel()
	if err != nil {
		return h.nepThatBai(ctx, j, "provider_unavailable")
	}
	text, ok := docTraLoi(raw)
	if !ok {
		return h.nepThatBai(ctx, j, "invalid_ai_result")
	}
	return h.nepXong(ctx, j, text)
}

// nepConSong confirms the session that asked is still live and the job is
// still this worker's.
func (h *Handler) nepConSong(ctx context.Context, j work) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	person, err := phien(ctx, tx, j.digest)
	if err != nil {
		return err
	}
	if person != j.person {
		return &denied{403, "sharing_unavailable"}
	}
	var live bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat_ai_invocations WHERE id=$1 AND status='running' AND lease_id=$2 AND lease_until>clock_timestamp() AND share_expires_at>clock_timestamp())`, j.id, j.lease).Scan(&live); err != nil {
		return err
	}
	if !live {
		return &denied{409, "invocation_cancelled"}
	}
	return tx.Commit(ctx)
}

// nepXong closes the job with the sealed answer. The question and the session
// go in the same statement that stores the answer; nothing is published.
func (h *Handler) nepXong(ctx context.Context, j work, text string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	person, err := phien(ctx, tx, j.digest)
	if err != nil || person != j.person {
		_ = tx.Rollback(ctx)
		return h.nepThatBai(ctx, j, "sharing_unavailable")
	}
	result, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE chat_ai_invocations SET status='succeeded',result=$3,prompt=NULL,boi_canh=NULL,lease_id=NULL,lease_until=NULL,updated_at=clock_timestamp() WHERE id=$1 AND lease_id=$2 AND status='running' AND scope='me' AND share_expires_at>clock_timestamp()`, j.id, j.lease, json.RawMessage(result))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// nepThatBai fails a personal job and scrubs what it was given at once. The
// group path keeps a failed prompt for its retry route; Nếp has none, so a
// question the device will simply ask again has no reason to stay.
func (h *Handler) nepThatBai(ctx context.Context, j work, code string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err := h.pool.Exec(ctx, `UPDATE chat_ai_invocations SET status='failed',code=$3,prompt=NULL,boi_canh=NULL,lease_id=NULL,lease_until=NULL,updated_at=clock_timestamp() WHERE id=$1 AND status='running' AND lease_id=$2`, j.id, j.lease, code)
	return err
}
