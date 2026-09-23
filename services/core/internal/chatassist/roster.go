package chatassist

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
)

// toiLa is how the caller appears everywhere the model reads, matching
// nhanNguoiNoi so the roster and the transcript name the same person the
// same way.
const toiLa = "Mình"

// roster is who is in the room, spoken in the caller's own vocabulary.
//
// The model needs the roster for what it can plan with: how many people are
// going, and who among them has not said anything yet. It does not need their
// account names, and it must not get them. The client mints a pseudonym per
// bundle ("Bạn 1", "Bạn 2") and the screen that sends it says, above the send
// button, «Tên tài khoản không đi kèm». The server holds the real names and
// could lay them over the top, but that would make the sentence the person just
// read false, and ADR-0034 §2.5 forbids withdrawing a stated privacy promise
// from one side. It would also hand the model two vocabularies for the same
// people with no key between them, which invites it to guess that "Bạn 1" is
// Lan and put the wrong person's allergy under her name on a card the whole
// room reads.
//
// So every active member appears exactly once, and each is labelled the way
// the bundle already labels them: the caller as toiLa, a friend who spoke by
// the alias their turns carry, and a friend who did not speak by a fresh alias
// that collides with none the bundle used. A member who left is not listed,
// even if their words are in the bundle: they are not coming.
//
// Mapping an alias to a member reads `id` and `author_id` of the shared turns.
// Never `body`: the gate in khong_doc_chat_test.go holds that.
func roster(ctx context.Context, tx pgx.Tx, store repo.Repository, room, caller string, goi []byte) (pyjson.List, error) {
	memberships, err := store.ListMembers(ctx, room)
	if err != nil {
		return nil, err
	}
	used := map[string]bool{toiLa: true}
	alias := map[string]string{}
	if len(goi) > 0 {
		var bc bundle
		if err = json.Unmarshal(goi, &bc); err != nil {
			return nil, err
		}
		authors, err := tacGia(ctx, tx, room, &bc)
		if err != nil {
			return nil, err
		}
		for _, l := range bc.Luot {
			if l.Vai != "ban" || l.BiDanh == "" {
				continue
			}
			// Reserve every alias the model will read, including a departed
			// member's, so a fresh one can never make two people one.
			taken := used[l.BiDanh]
			used[l.BiDanh] = true
			person, ok := authors[l.ID]
			if !ok || person == caller || taken {
				continue
			}
			if _, named := alias[person]; !named {
				alias[person] = l.BiDanh
			}
		}
	}
	next := 1
	fresh := func() string {
		for {
			label := fmt.Sprintf("Bạn %d", next)
			next++
			if !used[label] {
				used[label] = true
				return label
			}
		}
	}
	out := pyjson.List{entry(toiLa)}
	for _, m := range memberships {
		// The same test GroupTaste uses, so the roster and the taste it was
		// summed from describe the same people.
		if m.State != "active" || m.PersonID == caller {
			continue
		}
		label, ok := alias[m.PersonID]
		if !ok {
			label = fresh()
		}
		out = append(out, entry(label))
	}
	return out, nil
}

func entry(label string) *pyjson.OrderedMap {
	e := pyjson.NewOrderedMap()
	e.Set("display_name", pyjson.String(label))
	return e
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
