package chatassist

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"mobile/services/core/internal/chatv2"
)

// The group AI in an end-to-end room (ADR-0057 §6).
//
// The server reads no message of a v2 room: it has no key. What it knows of
// the room is what the lane records in the clear -- which device sent which
// logical send at which sequence, and whose device that is. So the shared
// turns and the `@Rủ Đi` message are checked against that, never against what
// they say; the words are only the caller's bundle, as in a legacy room. The
// answer is never posted: it waits in v2_result for the caller's device,
// which seals it into the room as an `ai_card`, and every member checks the
// card against result_digest (receipt).

// maxTheV2 is the largest answer a device can seal into one message
// (chat-crypto MAX_AI_CARD).
const maxTheV2 = 12 * 1024

// cacTacGiaV2 maps each logical send id of the room to the one person whose
// device sent it. A logical id is the sender's choice and travels in the
// clear, so an id two people used names nobody: it is left out.
const cauTacGiaV2 = `SELECT s.logical_send_id::text, min(e.actor_id::text) FROM chat_v2_sends s JOIN chat_v2_events e ON e.context_id=s.context_id AND e.sequence=s.sequence WHERE s.context_id=$1 AND s.logical_send_id = ANY($2::uuid[]) AND e.kind='envelope' GROUP BY s.logical_send_id HAVING count(DISTINCT e.actor_id)=1`

func tacGiaV2(ctx context.Context, tx pgx.Tx, room string, bc *bundle) (map[string]string, error) {
	out := map[string]string{}
	if len(bc.Luot) == 0 {
		return out, nil
	}
	ids := make([]string, 0, len(bc.Luot))
	for _, l := range bc.Luot {
		ids = append(ids, l.ID)
	}
	rows, err := tx.Query(ctx, cauTacGiaV2, room, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, author string
		if err = rows.Scan(&id, &author); err != nil {
			return nil, err
		}
		out[id] = author
	}
	return out, rows.Err()
}

// thuocPhongV2 is thuocPhong on the lane: every shared turn is a send of this
// room with one sender. One code for every miss, as on the legacy lane.
func thuocPhongV2(ctx context.Context, tx pgx.Tx, room string, bc *bundle) (int, error) {
	if len(bc.Luot) == 0 {
		return 0, nil
	}
	rieng := map[string]bool{}
	for _, l := range bc.Luot {
		rieng[l.ID] = true
	}
	tacGia, err := tacGiaV2(ctx, tx, room, bc)
	if err != nil {
		return 0, err
	}
	if len(tacGia) != len(rieng) {
		return 0, &denied{422, "boi_canh_mismatch"}
	}
	return len(tacGia), nil
}

// kiemTriggerV2 is kiemTrigger on the lane: the `@Rủ Đi` message was sent by
// the caller's device into this room, recently. That it says `@Rủ Đi` the
// server cannot see; the caller's own device asked.
func kiemTriggerV2(ctx context.Context, tx pgx.Tx, room, person, trigger string) error {
	var fresh bool
	err := tx.QueryRow(ctx, `SELECT e.created_at>clock_timestamp()-make_interval(secs => $4) FROM chat_v2_sends s JOIN chat_v2_events e ON e.context_id=s.context_id AND e.sequence=s.sequence WHERE s.context_id=$1 AND s.logical_send_id=$2 AND e.actor_id=$3 AND e.kind='envelope' ORDER BY e.sequence LIMIT 1`, room, trigger, person, tuoiTrigger.Seconds()).Scan(&fresh)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !fresh) {
		return &denied{422, "trigger_khong_hop_le"}
	}
	return err
}

// digestThe is what result_digest holds and what a member recomputes from
// the card it received (chat-crypto ai_card_digest).
func digestThe(card []byte) []byte {
	d := sha256.Sum256(card)
	return d[:]
}

// delivered records where the caller's device posted the answer, and drops
// the answer: from here it lives only in the room, sealed. The sequence must
// be an envelope the caller sent into this room; the same call again is a
// replay.
func (h *Handler) delivered(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Sequence int64 `json:"sequence"`
	}
	if !chatv2.ValidID(r.PathValue("id")) {
		failure(w, invalid("invalid_invocation"))
		return
	}
	if err := readBody(w, r, &in, 1024); err != nil {
		failure(w, err)
		return
	}
	if in.Sequence < 1 {
		failure(w, invalid("invalid_sequence"))
		return
	}
	tx, g, err := h.beginV2(r)
	if err != nil {
		failure(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	var lane, status string
	var result *string
	var daGiao *int64
	err = tx.QueryRow(r.Context(), `SELECT lane,status,v2_result,delivered_sequence FROM chat_ai_invocations WHERE id=$1 AND context_id=$2 AND person_id=$3 AND membership_id=$4 FOR UPDATE`, r.PathValue("id"), r.PathValue("context"), g.person, g.member).Scan(&lane, &status, &result, &daGiao)
	if errors.Is(err, pgx.ErrNoRows) {
		refuse(w, 404, "invocation_not_found")
		return
	}
	if err != nil {
		failure(w, err)
		return
	}
	if daGiao != nil {
		if *daGiao != in.Sequence {
			refuse(w, 409, "invocation_delivered")
			return
		}
	} else {
		if lane != laneV2 || status != "succeeded" || result == nil {
			refuse(w, 409, "invocation_not_deliverable")
			return
		}
		var cuaNguoiGoi bool
		if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM chat_v2_events WHERE context_id=$1 AND sequence=$2 AND actor_id=$3 AND kind='envelope')`, r.PathValue("context"), in.Sequence, g.person).Scan(&cuaNguoiGoi); err != nil {
			failure(w, err)
			return
		}
		if !cuaNguoiGoi {
			refuse(w, 422, "delivery_mismatch")
			return
		}
		if _, err = tx.Exec(r.Context(), `UPDATE chat_ai_invocations SET delivered_sequence=$2,v2_result=NULL,updated_at=clock_timestamp() WHERE id=$1`, r.PathValue("id"), in.Sequence); err != nil {
			failure(w, err)
			return
		}
	}
	v, err := scan(tx.QueryRow(r.Context(), `SELECT `+columns+` FROM chat_ai_invocations WHERE id=$1`, r.PathValue("id")))
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

// receipt lets any member of the room check an `ai_card` that claims to be
// the assistant's: the digest of the answer the server made for that
// invocation, and who asked. Only a v2 room's answered invocations have one.
func (h *Handler) receipt(w http.ResponseWriter, r *http.Request) {
	if !chatv2.ValidID(r.PathValue("id")) {
		failure(w, invalid("invalid_invocation"))
		return
	}
	tx, g, err := h.beginV2(r)
	if err != nil {
		failure(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if g.lane != laneV2 {
		refuse(w, 404, "invocation_not_found")
		return
	}
	var out struct {
		ID       string `json:"id"`
		PersonID string `json:"person_id"`
		Digest   string `json:"the_digest"`
		Trigger  string `json:"trigger_v2"`
	}
	err = tx.QueryRow(r.Context(), `SELECT id::text,person_id::text,encode(result_digest,'hex'),trigger_v2::text FROM chat_ai_invocations WHERE id=$1 AND context_id=$2 AND lane='v2' AND status='succeeded' AND result_digest IS NOT NULL AND trigger_v2 IS NOT NULL`, r.PathValue("id"), r.PathValue("context")).Scan(&out.ID, &out.PersonID, &out.Digest, &out.Trigger)
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
	reply(w, 200, out)
}

// theV2 is the card a v2 answer seals: the card exactly as the legacy lane
// would have posted it, compact JSON.
func theV2(card json.RawMessage) ([]byte, error) {
	var v any
	if err := json.Unmarshal(card, &v); err != nil {
		return nil, err
	}
	if _, ok := v.(map[string]any); !ok {
		return nil, errors.New("chatassist: card is not an object")
	}
	return card, nil
}
