package chatassist

import (
	"context"

	"mobile/services/core/internal/domain/promptsafety"
	"mobile/services/core/internal/repo"

	"github.com/jackc/pgx/v5"
)

// maxTenDoc bounds a display name the model may read (promptsafety.MaxTenDoc).
const maxTenDoc = promptsafety.MaxTenDoc

// tenDoc is a display name as the model may read it, or "" when it may not
// and the caller has to fall back to a neutral label (promptsafety.TenDoc).
func tenDoc(name string) string { return promptsafety.TenDoc(name) }

// tenThanhVien is a member's display name as the model may read it; the
// person-id fallback of ListMembers counts as no name (promptsafety.TenNguoi).
func tenThanhVien(m repo.Membership) string { return promptsafety.TenNguoi(m.DisplayName, m.PersonID) }

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
