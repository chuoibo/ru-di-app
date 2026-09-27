package community

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if len(q) > 100 || len(q) < 2 {
		fail(w, no(422, "invalid_search"))
		return
	}
	rows, err := tx.Query(r.Context(), `SELECT a.id,a.display_name,EXISTS(SELECT 1 FROM community_follows WHERE person_id=$1 AND kind='person' AND target=a.id::text) FROM people a WHERE a.deleted_at IS NULL AND strpos(lower(a.display_name),$2)>0 AND EXISTS(SELECT 1 FROM community_posts c JOIN posts p ON p.id=c.post_id WHERE p.author_id=a.id AND p.audience='public' AND c.published_revision IS NOT NULL AND c.deleted_at IS NULL AND (`+readableSQL+`)) ORDER BY a.display_name,a.id LIMIT 30`, person, q)
	if err != nil {
		fail(w, err)
		return
	}
	out := []map[string]any{}
	for rows.Next() {
		var id, name string
		var following bool
		if err = rows.Scan(&id, &name, &following); err != nil {
			break
		}
		out = append(out, map[string]any{"id": id, "name": name, "following": following})
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 200, map[string]any{"people": out})
}
func (h *Handler) keep(w http.ResponseWriter, r *http.Request) {
	var in struct {
		LogicalID string `json:"logical_id"`
		PostID    string `json:"post_id"`
		Body      string `json:"body"`
		AI        bool   `json:"ai_generated"`
	}
	if err := decode(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	in.Body = strings.TrimSpace(in.Body)
	if in.Body == "" || utf8.RuneCountInString(in.Body) > 5000 {
		fail(w, no(422, "invalid_body"))
		return
	}
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	ctx := r.Context()
	if _, err = readable(ctx, tx, person, in.PostID); err != nil {
		fail(w, err)
		return
	}
	digest := hash(in)
	id, err := replay(ctx, tx, person, in.LogicalID, digest)
	if err != nil {
		fail(w, err)
		return
	}
	if id != "" {
		commit(w, r, tx, 200, map[string]string{"id": id})
		return
	}
	if err = rate(ctx, tx, person); err != nil {
		fail(w, err)
		return
	}
	id = uuid()
	_, err = tx.Exec(ctx, `INSERT INTO community_keeps(id,person_id,post_id,body,ai_generated) VALUES($1,$2,$3,$4,$5)`, id, person, in.PostID, in.Body, in.AI)
	if err == nil {
		err = remember(ctx, tx, person, in.LogicalID, digest, id)
	}
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 201, map[string]string{"id": id})
}
func (h *Handler) keeps(w http.ResponseWriter, r *http.Request) {
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	rows, err := tx.Query(r.Context(), `SELECT id,body,ai_generated,created_at FROM community_keeps WHERE person_id=$1 ORDER BY created_at DESC LIMIT 100`, person)
	if err != nil {
		fail(w, err)
		return
	}
	out := []map[string]any{}
	for rows.Next() {
		var id, body string
		var ai bool
		var at time.Time
		if err = rows.Scan(&id, &body, &ai, &at); err != nil {
			break
		}
		out = append(out, map[string]any{"id": id, "body": body, "ai_generated": ai, "created_at": at})
	}
	rows.Close()
	if err == nil {
		err = rows.Err()
	}
	if err != nil {
		fail(w, err)
		return
	}
	commit(w, r, tx, 200, map[string]any{"keeps": out})
}
func (h *Handler) removeKeep(w http.ResponseWriter, r *http.Request) {
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	id := r.PathValue("keep")
	if !validID(id) {
		fail(w, no(404, "keep_not_found"))
		return
	}
	tag, err := tx.Exec(r.Context(), `DELETE FROM community_keeps WHERE id=$1 AND person_id=$2`, id, person)
	if err != nil {
		fail(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, no(404, "keep_not_found"))
		return
	}
	commit(w, r, tx, 204, nil)
}
