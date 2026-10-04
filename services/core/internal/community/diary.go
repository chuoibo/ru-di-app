package community

import (
	"encoding/json"
	"net/http"
	"strings"

	book "mobile/services/core/internal/domain/diary"
	"mobile/services/core/internal/media/storage"
)

// shareDiary copies only an explicitly selected, already-public edition. The
// community copy never grants access to the original group's photograph URL.
func (h *Handler) shareDiary(w http.ResponseWriter, r *http.Request) {
	var in struct {
		LogicalID string   `json:"logical_id"`
		Revision  int      `json:"revision"`
		Confirmed bool     `json:"confirmed"`
		Topics    []string `json:"topics"`
	}
	if err := decode(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	if !in.Confirmed {
		fail(w, no(422, "consent_required"))
		return
	}
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	ctx := r.Context()
	id := r.PathValue("diary")
	if !validID(id) {
		fail(w, no(404, "diary_not_found"))
		return
	}
	digest := hash(struct {
		ID    string
		Input any
	}{id, in})
	pid, err := replay(ctx, tx, person, in.LogicalID, digest)
	if err != nil {
		fail(w, err)
		return
	}
	if pid != "" {
		commit(w, r, tx, 200, map[string]string{"id": pid})
		return
	}
	var raw []byte
	var kind string
	var rev int
	err = tx.QueryRow(ctx, `SELECT d.document,d.kind,d.revision FROM (SELECT od.*,oe.kind FROM outing_diaries od JOIN outing_endings oe ON oe.outing_id=od.outing_id) d JOIN outings o ON o.id=d.outing_id JOIN memberships m ON m.context_id=o.context_id AND m.person_id=$2 AND m.state='active' AND m.left_at IS NULL WHERE d.id=$1 AND d.owner_id=$2 AND d.audience='public' FOR SHARE OF m`, id, person).Scan(&raw, &kind, &rev)
	if err != nil {
		fail(w, no(404, "diary_not_found"))
		return
	}
	if rev != in.Revision {
		fail(w, no(409, "revision_conflict"))
		return
	}
	// Lock the edition itself as well as membership until the copy commits.
	var locked int
	err = tx.QueryRow(ctx, `SELECT revision FROM outing_diaries WHERE id=$1 AND audience='public' FOR SHARE`, id).Scan(&locked)
	if err != nil || locked != rev {
		fail(w, no(409, "revision_conflict"))
		return
	}
	var doc book.Document
	if json.Unmarshal(raw, &doc) != nil {
		fail(w, no(422, "invalid_diary"))
		return
	}
	topics, err := normalizeTopics(in.Topics)
	if err != nil {
		fail(w, err)
		return
	}
	if err = rate(ctx, tx, person); err != nil {
		fail(w, err)
		return
	}
	rows, err := tx.Query(ctx, `SELECT u.id,u.storage_key,u.content_type,u.width,u.height FROM outing_diary_photos dp JOIN uploaded_images u ON u.id=dp.photo_id WHERE dp.diary_id=$1 LIMIT 41`, id)
	if err != nil {
		fail(w, err)
		return
	}
	type source struct {
		id, key, mime string
		width, height int
	}
	sources := []source{}
	for rows.Next() {
		var s source
		if err = rows.Scan(&s.id, &s.key, &s.mime, &s.width, &s.height); err != nil {
			break
		}
		sources = append(sources, s)
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err != nil || len(sources) > 40 {
		fail(w, no(422, "invalid_diary"))
		return
	}
	st, err := storage.New()
	if err != nil {
		fail(w, err)
		return
	}
	keys := []string{}
	success := false
	defer func() {
		if !success {
			for _, key := range keys {
				_, _ = st.Delete(key)
			}
		}
	}()
	mapping := map[string]string{}
	mediaIDs := []string{}
	total := 0
	for _, s := range sources {
		data, e := st.Read(s.key)
		if e != nil {
			fail(w, no(404, "media_not_found"))
			return
		}
		total += len(data)
		if total > 64<<20 {
			fail(w, no(422, "media_bundle_too_large"))
			return
		}
		key, e := storage.NewStorageKey()
		if e == nil {
			e = st.Write(key, data)
		}
		if e != nil {
			fail(w, e)
			return
		}
		keys = append(keys, key)
		mid := uuid()
		mapping[s.id] = mid
		mediaIDs = append(mediaIDs, mid)
		_, err = tx.Exec(ctx, `INSERT INTO community_media(id,owner_id,storage_key,content_type,byte_size,width,height,state) VALUES($1,$2,$3,$4,$5,$6,$7,'ready')`, mid, person, key, s.mime, len(data), s.width, s.height)
		if err != nil {
			fail(w, err)
			return
		}
	}
	doc.CoverID = mapping[doc.CoverID]
	var words strings.Builder
	words.WriteString(doc.Title + "\n" + doc.Subtitle)
	for i := range doc.Pages {
		page := &doc.Pages[i]
		words.WriteString("\n\n" + book.TenTrang(page.Heading) + "\n" + page.Text)
		for j, sourceID := range page.PhotoIDs {
			mid, ok := mapping[sourceID]
			if !ok {
				fail(w, no(422, "diary_source_changed"))
				return
			}
			page.PhotoIDs[j] = mid
		}
	}
	body := words.String()
	if len([]rune(body)) > 5000 {
		fail(w, no(422, "diary_excerpt_too_long"))
		return
	}
	encoded, _ := json.Marshal(doc)
	pid = uuid()
	_, err = tx.Exec(ctx, `INSERT INTO posts(id,author_id,audience,body,created_at) VALUES($1,$2,'only_me',$3,clock_timestamp())`, pid, person, body)
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO community_posts(post_id,requested_audience,status,diary_document,diary_kind,diary_id,diary_revision) VALUES($1,'public','pending',$2,$3,$4,$5)`, pid, encoded, kind, id, rev)
	}
	if err == nil {
		err = writeRevision(ctx, tx, pid, 1, PostInput{Body: body, Topics: topics, Mentions: []string{}, MediaIDs: mediaIDs})
	}
	if err == nil {
		err = enqueue(ctx, tx, pid, 1)
	}
	if err == nil {
		err = remember(ctx, tx, person, in.LogicalID, digest, pid)
	}
	if err == nil {
		err = emit(ctx, tx, pid, "post.changed")
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		fail(w, err)
		return
	}
	success = true
	reply(w, 201, map[string]string{"id": pid, "status": "pending"})
}
