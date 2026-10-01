package chatassist

import (
	"context"
	"strings"

	"mobile/services/core/internal/domain/promptsafety"
	"mobile/services/core/internal/repo"

	"github.com/jackc/pgx/v5"
)

// maxTenDoc bounds a display name the model may read. It is the catalogue's
// item bound (promptsafety maxItem): long enough for any real name, short
// enough that a name cannot carry a paragraph.
const maxTenDoc = 60

// tenDoc is a display name as the model may read it, or "" when it may not
// and the caller has to fall back to a neutral label.
//
// A display name is text a person typed about themselves, and since
// 2026-09-24 it reaches the model (ADR-0036 §5). So it goes through the same
// test a catalogue row does: a name that tries to talk to the model is not
// quoted more carefully, it is not quoted at all.
func tenDoc(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || !promptsafety.TextSafe(name, maxTenDoc) {
		return ""
	}
	return name
}

// tenThanhVien is a member's display name as the model may read it.
// ListMembers falls back to the person id when display_name is empty, and an
// account id is exactly what must never reach the model, so that fallback
// counts as no name.
func tenThanhVien(m repo.Membership) string {
	if m.DisplayName == m.PersonID {
		return ""
	}
	return tenDoc(m.DisplayName)
}

// tacGia maps each shared turn to its author. Ownership was already checked
// when the invocation was accepted (thuocPhong); a turn whose message has
// since gone simply maps to nobody.
func tacGia(ctx context.Context, tx pgx.Tx, room string, bc *bundle) (map[string]string, error) {
	out := map[string]string{}
	if len(bc.Luot) == 0 {
		return out, nil
	}
	ids := make([]string, 0, len(bc.Luot))
	for _, l := range bc.Luot {
		ids = append(ids, l.ID)
	}
	rows, err := tx.Query(ctx, `SELECT id::text, author_id::text FROM messages WHERE context_id=$1 AND author_id IS NOT NULL AND id = ANY($2::uuid[])`, room, ids)
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
