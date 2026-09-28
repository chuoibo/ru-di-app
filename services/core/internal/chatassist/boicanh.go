package chatassist

import (
	"context"
	"encoding/json"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/chatv2"
	"mobile/services/core/internal/pyjson"
)

// The context bundle the caller hands over (ADR-0036 §2.2).
//
// The server does not read the conversation, so everything here arrives from
// the client. That makes the bounds below a refusal surface rather than a
// formality, and it makes the ownership check further down the only statement
// about this payload the server can actually stand behind.
//
// Field names match the client's own vocabulary rather than being translated on
// the way in. A second spelling would be a second thing to keep in step.
type turn struct {
	ID     string `json:"id"`
	Vai    string `json:"vai"`
	BiDanh string `json:"biDanh,omitempty"`
	Loai   string `json:"loai"`
	Luc    string `json:"luc"`
	Chu    string `json:"chu"`
}

type bundle struct {
	Ban      int    `json:"ban"`
	Nguon    string `json:"nguon"`
	Luot     []turn `json:"luot"`
	TongLuot int    `json:"tongLuot"`
	DaCat    bool   `json:"daCat"`
}

const (
	maxLuot       = 40
	maxChuMoiLuot = 300
	// The only bound that is encoding independent, and therefore the one that
	// actually decides. A client may send `\uXXXX` escapes, so counting bytes on
	// the wire would cap a Vietnamese sentence six times tighter than an English
	// one for no reason anybody could explain.
	maxTongRune = 20000
	// 20000 runes at the worst case 6 bytes per escaped rune, plus envelope.
	maxBodyWithBundle = 128 << 10
)

var (
	vaiHopLe  = map[string]bool{"toi": true, "ban": true, "ai": true}
	loaiHopLe = map[string]bool{"chu": true, "anh": true, "sticker": true, "the": true, "da-xoa": true}
)

// kiemBoiCanh refuses rather than truncates. Silently dropping the oldest turns
// server-side would make the sentence the person just read above the send button
// ("24 tin gần nhất") false at the moment it mattered most.
func kiemBoiCanh(bc *bundle, prompt string) error {
	if bc.Ban != 1 || bc.Nguon != "chat-nhom" {
		return &denied{400, "boi_canh_sai_dang"}
	}
	if len(bc.Luot) > maxLuot || bc.TongLuot < len(bc.Luot) {
		return &denied{413, "boi_canh_qua_lon"}
	}
	tong := utf8.RuneCountInString(prompt)
	for i := range bc.Luot {
		l := &bc.Luot[i]
		if !chatv2.ValidID(l.ID) || !vaiHopLe[l.Vai] || !loaiHopLe[l.Loai] {
			return &denied{400, "boi_canh_sai_dang"}
		}
		if !utf8.ValidString(l.Chu) || !utf8.ValidString(l.BiDanh) {
			return &denied{400, "boi_canh_sai_dang"}
		}
		if utf8.RuneCountInString(l.Chu) > maxChuMoiLuot {
			return &denied{413, "boi_canh_qua_lon"}
		}
		tong += utf8.RuneCountInString(l.Chu)
	}
	if tong > maxTongRune {
		return &denied{413, "boi_canh_qua_lon"}
	}
	return nil
}

// thuocPhong is the one claim about this payload the server can verify, and the
// reason it is worth verifying is that other people read the result.
//
// The published card says «Nếp đã đọc N tin». Everyone in the room sees that
// line. Without this check N is a number the caller asserted about itself. With
// it, every turn is a real message of THIS room before that sentence may be
// published.
//
// It deliberately reads only `id`, never `body`. "The server does not read the
// conversation" is then a property of the SQL rather than a promise in a
// comment, and it stays true after the cutover, when there is no body to read.
//
// The count it returns is the number of distinct messages it confirmed: the
// N a reply in the thread says it read (`so_tin_doc`).
func thuocPhong(ctx context.Context, tx pgx.Tx, room string, bc *bundle) (int, error) {
	if len(bc.Luot) == 0 {
		return 0, nil
	}
	ids := make([]string, 0, len(bc.Luot))
	rieng := make(map[string]bool, len(bc.Luot))
	for _, l := range bc.Luot {
		if !rieng[l.ID] {
			rieng[l.ID] = true
			ids = append(ids, l.ID)
		}
	}
	var thay int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM messages WHERE context_id=$1 AND id = ANY($2::uuid[])`, room, ids).Scan(&thay); err != nil {
		return 0, err
	}
	if thay != len(ids) {
		// One code for "not this room" and for "no such message": telling them
		// apart would answer "does message X exist" for anybody who can post here.
		return 0, &denied{422, "boi_canh_mismatch"}
	}
	return thay, nil
}

// canonical is what goes into the input digest and into the stored column.
//
// Idempotency has to cover the bundle on BOTH sides. Keyed on the prompt alone,
// the same question asked again over newer messages lands on the same digest,
// the server replays the previous answer with a 200, and the person believes the
// AI just read what they just said. That failure is silent, so the key is the
// place to stop it.
func canonical(bc *bundle) ([]byte, error) { return json.Marshal(bc) }

// goiHoacNull keeps the column NULL instead of storing the JSON literal `null`.
// The difference is not cosmetic: `chat_ai_context_needs_prompt` is written
// against NULL, and a stored `null` would satisfy the column while defeating the
// constraint that exists to make scrubbing verifiable.
func goiHoacNull(goi []byte) any {
	if len(goi) == 0 {
		return nil
	}
	return json.RawMessage(goi)
}

// hoiThoai turns the stored bundle into the `conversation` the brain already
// accepts, with the caller's own words as the last turn.
//
// The brain contract does not change by one line: `companion-reply` has always
// taken a list of turns, v1 filled it from the database, and this fills it from
// what the caller shared. That is why the live quality gate written for v1
// (`test_companion_gemini_live.py`, which checks the model reads a real
// conversation and respects a constraint the group typed) still measures this
// path without being rewritten.
//
// toi is the caller's label, the one roster returned, so the transcript and
// the roster name the caller the same way.
func hoiThoai(goi []byte, prompt, toi string) (pyjson.List, error) {
	out := pyjson.List{}
	if len(goi) > 0 {
		var bc bundle
		if err := json.Unmarshal(goi, &bc); err != nil {
			return nil, err
		}
		for _, l := range bc.Luot {
			row := pyjson.NewOrderedMap()
			kind := "human"
			if l.Vai == "ai" {
				kind = "ai"
			}
			row.Set("author_kind", pyjson.String(kind))
			row.Set("kind", pyjson.String("text"))
			row.Set("speaker", pyjson.String(nhanNguoiNoi(l, toi)))
			row.Set("body", pyjson.String(l.Chu))
			row.Set("created_at", pyjson.String(l.Luc))
			out = append(out, row)
		}
	}
	last := pyjson.NewOrderedMap()
	last.Set("author_kind", pyjson.String("human"))
	last.Set("kind", pyjson.String("text"))
	last.Set("speaker", pyjson.String(toi))
	last.Set("body", pyjson.String(prompt))
	out = append(out, last)
	return out, nil
}

// nhanNguoiNoi is the speaker label of one shared turn.
//
// A friend's label is the display name the client put on the turn (ADR-0036
// §5), and it is text somebody typed about themselves. It goes through tenDoc,
// the same test roster applies, so a name written at the model is never
// quoted; the turn is still attributed to someone in the room rather than
// dropped.
func nhanNguoiNoi(l turn, toi string) string {
	switch l.Vai {
	case "toi":
		return toi
	case "ai":
		return "Rủ Đi AI"
	default:
		if label := tenDoc(l.BiDanh); label != "" {
			return label
		}
		return "Một người trong nhóm"
	}
}
