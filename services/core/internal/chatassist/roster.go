package chatassist

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/domain/promptsafety"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
)

// toiLa is the caller's label when their own display name cannot be used:
// empty, unsafe to put in front of a model, or already carried by somebody
// else's turns in the bundle.
const toiLa = "Mình"

// maxTenDoc bounds a display name the model may read. It is the catalogue's
// item bound (promptsafety maxItem): long enough for any real name, short
// enough that a name cannot carry a paragraph.
const maxTenDoc = 60

// tenDoc is a display name as the model may read it, or "" when it may not
// and the caller has to fall back to a neutral label.
//
// A display name is text a person typed about themselves, and since
// 2026-09-24 it reaches the model (ADR-0034 §5). So it goes through the same
// test a catalogue row does: a name that tries to talk to the model is not
// quoted more carefully, it is not quoted at all.
func tenDoc(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || !promptsafety.TextSafe(name, maxTenDoc) {
		return ""
	}
	return name
}

// roster is who is in the room, by the names the room knows them by, and the
// label the caller goes by in the same vocabulary.
//
// The model needs the roster for what it can plan with: how many people are
// going, and who among them has not said anything yet. Since the product
// decision of 2026-09-24 (ADR-0034 §5) it gets their display names too: a
// plan that says «Lan dị ứng hải sản» is one the room can act on, and one that
// says «Bạn 1 dị ứng hải sản» makes everybody work out who Bạn 1 was. The
// screen that sends the bundle says so above the send button, and that
// sentence changed in the same change as this code, because ADR-0034 §2.5
// forbids withdrawing a stated promise from one side.
//
// The roster and the transcript still have to speak ONE vocabulary. Two labels
// for the same person with no key between them invites the model to guess,
// and a wrong guess puts somebody else's allergy under a name on a card the
// whole room reads. So:
//
//   - the caller is their own display name, or toiLa when that name is not
//     safe or some other speaker in the bundle already carries it;
//   - a friend who spoke keeps the label their turns carry (the client now
//     sends the display name; an older client sent «Bạn N»), unless that label
//     is not safe, in which case the turns read «Một người trong nhóm» and the
//     member is labelled as if silent;
//   - a friend who did not speak gets their display name, «Tên (2)» when that
//     name is already taken, or a fresh «Bạn N» when the name is empty or not
//     safe.
//
// Every active member appears exactly once. A member who left is not listed,
// even if their words are in the bundle: they are not coming. Their label is
// still reserved, so nobody else can be handed it.
//
// Mapping a label to a member reads `id` and `author_id` of the shared turns.
// Never `body`: the gate in khong_doc_chat_test.go holds that.
func roster(ctx context.Context, tx pgx.Tx, store repo.Repository, room, caller string, goi []byte) (pyjson.List, string, error) {
	memberships, err := store.ListMembers(ctx, room)
	if err != nil {
		return nil, "", err
	}
	var luot []turn
	authors := map[string]string{}
	if len(goi) > 0 {
		var bc bundle
		if err = json.Unmarshal(goi, &bc); err != nil {
			return nil, "", err
		}
		if authors, err = tacGia(ctx, tx, room, &bc); err != nil {
			return nil, "", err
		}
		luot = bc.Luot
	}
	members, toi := xepRoster(memberships, caller, luot, authors)
	return members, toi, nil
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

// xepRoster is roster without the database: the labelling rules on their own,
// so they can be tested without PostgreSQL.
func xepRoster(memberships []repo.Membership, caller string, luot []turn, authors map[string]string) (pyjson.List, string) {
	// Labels other speakers carry in the bundle. The caller may not take one:
	// that would make the caller and a friend one speaker.
	cuaBan := map[string]bool{}
	for _, l := range luot {
		if l.Vai == "ban" {
			if label := tenDoc(l.BiDanh); label != "" {
				cuaBan[label] = true
			}
		}
	}
	toi := toiLa
	for _, m := range memberships {
		if m.State == "active" && m.PersonID == caller {
			if name := tenThanhVien(m); name != "" && !cuaBan[name] {
				toi = name
			}
			break
		}
	}
	used := map[string]bool{toi: true}
	alias := map[string]string{}
	for _, l := range luot {
		label := tenDoc(l.BiDanh)
		if l.Vai != "ban" || label == "" {
			continue
		}
		// Reserve every label the model will read, including a departed
		// member's, so a silent member can never be handed it.
		taken := used[label]
		used[label] = true
		person, ok := authors[l.ID]
		if !ok || person == caller || taken {
			continue
		}
		if _, named := alias[person]; !named {
			alias[person] = label
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
	rieng := func(name string) string {
		label := name
		for k := 2; used[label]; k++ {
			label = fmt.Sprintf("%s (%d)", name, k)
		}
		used[label] = true
		return label
	}
	out := pyjson.List{entry(toi)}
	for _, m := range memberships {
		// The same test GroupTaste uses, so the roster and the taste it was
		// summed from describe the same people.
		if m.State != "active" || m.PersonID == caller {
			continue
		}
		label, ok := alias[m.PersonID]
		if !ok {
			if name := tenThanhVien(m); name != "" {
				label = rieng(name)
			} else {
				label = fresh()
			}
		}
		out = append(out, entry(label))
	}
	return out, toi
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
