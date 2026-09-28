package diary

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/chatv2"
	book "mobile/services/core/internal/domain/diary"
	"mobile/services/core/internal/media/storage"
)

type Book struct {
	ID        string        `json:"id"`
	OutingID  string        `json:"outing_id"`
	OwnerID   string        `json:"owner_id"`
	Kind      string        `json:"kind"`
	Audience  string        `json:"audience"`
	Revision  int           `json:"revision"`
	Document  book.Document `json:"document"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

const bookColumns = `d.id,d.outing_id,d.owner_id,e.kind,d.audience,d.revision,d.document,d.created_at,d.updated_at`

func scanBook(row pgx.Row) (Book, error) {
	var b Book
	var raw []byte
	err := row.Scan(&b.ID, &b.OutingID, &b.OwnerID, &b.Kind, &b.Audience, &b.Revision, &raw, &b.CreatedAt, &b.UpdatedAt)
	if err == nil {
		err = json.Unmarshal(raw, &b.Document)
	}
	return b, err
}

const notBlocked = `NOT EXISTS(SELECT 1 FROM friend_requests f WHERE f.state='blocked' AND ((f.requester_id=d.owner_id AND f.addressee_id=$2) OR (f.addressee_id=d.owner_id AND f.requester_id=$2)))`

func readable(ctx context.Context, tx pgx.Tx, id, person string) (Book, error) {
	if !chatv2.ValidID(id) {
		return Book{}, no(404, "diary_not_found")
	}
	b, err := scanBook(tx.QueryRow(ctx, `SELECT `+bookColumns+` FROM outing_diaries d JOIN outing_endings e ON e.outing_id=d.outing_id JOIN people p ON p.id=d.owner_id AND p.deleted_at IS NULL WHERE d.id=$1 AND (d.owner_id=$2 OR d.audience='public') AND `+notBlocked+` FOR SHARE OF d`, id, person))
	if errors.Is(err, pgx.ErrNoRows) {
		err = no(404, "diary_not_found")
	}
	return b, err
}
func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Revision int           `json:"revision"`
		Audience string        `json:"audience"`
		Document book.Document `json:"document"`
	}
	if err := decode(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	if in.Audience != "private" && in.Audience != "public" {
		fail(w, no(422, "invalid_diary_audience"))
		return
	}
	tx, p, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	ctx := r.Context()
	o, err := authorize(ctx, tx, r.PathValue("outing"), p)
	if err != nil {
		fail(w, err)
		return
	}
	var ending string
	if err = tx.QueryRow(ctx, `SELECT outing_id FROM outing_endings WHERE outing_id=$1 FOR SHARE`, o.ID).Scan(&ending); errors.Is(err, pgx.ErrNoRows) {
		fail(w, no(409, "outing_not_ended"))
		return
	} else if err != nil {
		fail(w, err)
		return
	}
	ids := documentPhotos(in.Document)
	allowed, err := checkedPhotos(ctx, tx, o, p, ids)
	if err != nil {
		fail(w, err)
		return
	}
	if err = book.Validate(in.Document, allowed); err != nil {
		fail(w, no(422, err.Error()))
		return
	}
	// Serialize first saves too, so losing requests report a revision conflict.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, o.ID+":"+p); err != nil {
		fail(w, err)
		return
	}
	var id string
	var revision int
	var oldRaw []byte
	var oldAudience string
	err = tx.QueryRow(ctx, `SELECT id,revision,document,audience FROM outing_diaries WHERE outing_id=$1 AND owner_id=$2 FOR UPDATE`, o.ID, p).Scan(&id, &revision, &oldRaw, &oldAudience)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		fail(w, err)
		return
	}
	raw, _ := json.Marshal(in.Document)
	if id == "" {
		if in.Revision != 0 {
			fail(w, no(409, "diary_revision_conflict"))
			return
		}
		id = uuid()
		revision = 1
		_, err = tx.Exec(ctx, `INSERT INTO outing_diaries(id,outing_id,owner_id,audience,document) VALUES($1,$2,$3,$4,$5)`, id, o.ID, p, in.Audience, raw)
	} else {
		// A response lost after commit may be retried without making another version.
		var old book.Document
		_ = json.Unmarshal(oldRaw, &old)
		same := hashInput(old) == hashInput(in.Document) && oldAudience == in.Audience
		if in.Revision != revision && !(same && in.Revision == revision-1) {
			fail(w, no(409, "diary_revision_conflict"))
			return
		}
		if !same {
			revision++
			_, err = tx.Exec(ctx, `UPDATE outing_diaries SET document=$2,audience=$3,revision=$4,updated_at=clock_timestamp() WHERE id=$1`, id, raw, in.Audience, revision)
		}
	}
	if err != nil {
		fail(w, err)
		return
	}
	_, err = tx.Exec(ctx, `INSERT INTO outing_diary_versions(diary_id,revision,document) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, id, revision, raw)
	if err == nil {
		_, err = tx.Exec(ctx, `DELETE FROM outing_diary_photos WHERE diary_id=$1`, id)
	}
	if err != nil {
		fail(w, err)
		return
	}
	for _, photo := range ids {
		if _, err = tx.Exec(ctx, `INSERT INTO outing_diary_photos(diary_id,photo_id) VALUES($1,$2)`, id, photo); err != nil {
			fail(w, err)
			return
		}
	}
	b, err := readable(ctx, tx, id, p)
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		fail(w, err)
		return
	}
	reply(w, 200, b)
}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	tx, p, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	b, err := readable(r.Context(), tx, r.PathValue("diary"), p)
	if err != nil {
		fail(w, err)
		return
	}
	reply(w, 200, b)
}
func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	tx, p, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	id := r.PathValue("diary")
	if !chatv2.ValidID(id) {
		fail(w, no(404, "diary_not_found"))
		return
	}
	tag, err := tx.Exec(r.Context(), `DELETE FROM outing_diaries WHERE id=$1 AND owner_id=$2`, id, p)
	if err == nil && tag.RowsAffected() == 0 {
		err = no(404, "diary_not_found")
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(204)
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tx, p, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	owner := r.PathValue("person")
	if !chatv2.ValidID(owner) {
		fail(w, no(404, "person_not_found"))
		return
	}
	before := r.URL.Query().Get("before")
	if before != "" && !chatv2.ValidID(before) {
		fail(w, no(422, "invalid_cursor"))
		return
	}
	rows, err := tx.Query(r.Context(), `SELECT `+bookColumns+` FROM outing_diaries d JOIN outing_endings e ON e.outing_id=d.outing_id JOIN people p ON p.id=d.owner_id AND p.deleted_at IS NULL WHERE d.owner_id=$1 AND (d.owner_id=$2 OR d.audience='public') AND `+notBlocked+` AND ($3='' OR (d.created_at,d.id)<(SELECT created_at,id FROM outing_diaries WHERE id=NULLIF($3,'')::uuid AND owner_id=$1)) ORDER BY d.created_at DESC,d.id DESC LIMIT 31`, owner, p, before)
	if err != nil {
		fail(w, err)
		return
	}
	defer rows.Close()
	books := []Book{}
	for rows.Next() {
		b, err := scanBook(rows)
		if err != nil {
			fail(w, err)
			return
		}
		books = append(books, b)
	}
	if err = rows.Err(); err != nil {
		fail(w, err)
		return
	}
	var next *string
	if len(books) > 30 {
		books = books[:30]
		next = &books[len(books)-1].ID
	}
	reply(w, 200, struct {
		Diaries []Book  `json:"diaries"`
		Next    *string `json:"next_cursor"`
	}{books, next})
}
func (h *Handler) photo(w http.ResponseWriter, r *http.Request) {
	tx, p, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	b, err := readable(r.Context(), tx, r.PathValue("diary"), p)
	if err != nil {
		fail(w, err)
		return
	}
	photo := r.PathValue("photo")
	if !chatv2.ValidID(photo) {
		fail(w, no(404, "diary_photo_not_found"))
		return
	}
	var key, mime string
	err = tx.QueryRow(r.Context(), `SELECT u.storage_key,u.content_type FROM outing_diary_photos dp JOIN uploaded_images u ON u.id=dp.photo_id WHERE dp.diary_id=$1 AND dp.photo_id=$2`, b.ID, photo).Scan(&key, &mime)
	if err != nil {
		fail(w, no(404, "diary_photo_not_found"))
		return
	}
	s, err := storage.New()
	if err != nil {
		fail(w, err)
		return
	}
	data, err := s.Read(key)
	if err != nil {
		fail(w, no(404, "diary_photo_not_found"))
		return
	}
	w.Header().Set("Content-Type", mime)
	w.WriteHeader(200)
	_, _ = w.Write(data)
}

// visibility remains available after leaving a group: an owner can always
// withdraw their own publication without regaining access to the source room.
func (h *Handler) visibility(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Revision int    `json:"revision"`
		Audience string `json:"audience"`
	}
	if err := decode(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	if in.Audience != "private" {
		fail(w, no(422, "private_audience_required"))
		return
	}
	tx, p, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	id := r.PathValue("diary")
	if !chatv2.ValidID(id) {
		fail(w, no(404, "diary_not_found"))
		return
	}
	var rev int
	var audience string
	err = tx.QueryRow(r.Context(), `SELECT revision,audience FROM outing_diaries WHERE id=$1 AND owner_id=$2 FOR UPDATE`, id, p).Scan(&rev, &audience)
	if errors.Is(err, pgx.ErrNoRows) {
		err = no(404, "diary_not_found")
	}
	if err != nil {
		fail(w, err)
		return
	}
	if rev != in.Revision && !(audience == "private" && rev == in.Revision+1) {
		fail(w, no(409, "diary_revision_conflict"))
		return
	}
	if audience != "private" {
		_, err = tx.Exec(r.Context(), `UPDATE outing_diaries SET audience='private',revision=revision+1,updated_at=clock_timestamp() WHERE id=$1`, id)
		if err == nil {
			_, err = tx.Exec(r.Context(), `INSERT INTO outing_diary_versions(diary_id,revision,document) SELECT id,revision,document FROM outing_diaries WHERE id=$1`, id)
		}
	}
	if err != nil {
		fail(w, err)
		return
	}
	b, err := readable(r.Context(), tx, id, p)
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		fail(w, err)
		return
	}
	reply(w, 200, b)
}
