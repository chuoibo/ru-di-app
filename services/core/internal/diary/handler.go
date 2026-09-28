package diary

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/brain"
	"mobile/services/core/internal/chatv2"
	book "mobile/services/core/internal/domain/diary"
)

type Handler struct {
	pool  *pgxpool.Pool
	brain *brain.Client
	mux   *http.ServeMux
}

var Patterns = []string{
	"GET /outings/{outing}/ending",
	"POST /outings/{outing}/ending",
	"GET /outings/{outing}/diary-sources",
	"POST /outings/{outing}/diary-jobs",
	"GET /diary-jobs/{job}",
	"PUT /outings/{outing}/diary",
	"GET /diaries/{diary}",
	"DELETE /diaries/{diary}",
	"GET /diaries/{diary}/photos/{photo}",
	"GET /people/{person}/diaries",
	"PATCH /diaries/{diary}/audience",
}

func New(pool *pgxpool.Pool, client *brain.Client) *Handler {
	h := &Handler{pool: pool, brain: client, mux: http.NewServeMux()}
	h.mux.HandleFunc("GET /outings/{outing}/ending", h.ending)
	h.mux.HandleFunc("POST /outings/{outing}/ending", h.end)
	h.mux.HandleFunc("GET /outings/{outing}/diary-sources", h.sources)
	h.mux.HandleFunc("POST /outings/{outing}/diary-jobs", h.createJob)
	h.mux.HandleFunc("GET /diary-jobs/{job}", h.getJob)
	h.mux.HandleFunc("PUT /outings/{outing}/diary", h.save)
	h.mux.HandleFunc("GET /diaries/{diary}", h.get)
	h.mux.HandleFunc("DELETE /diaries/{diary}", h.remove)
	h.mux.HandleFunc("GET /diaries/{diary}/photos/{photo}", h.photo)
	h.mux.HandleFunc("GET /people/{person}/diaries", h.list)
	h.mux.HandleFunc("PATCH /diaries/{diary}/audience", h.visibility)
	return h
}
func Matches(path string) bool {
	p := strings.Split(strings.Trim(path, "/"), "/")
	if len(p) >= 2 && (p[0] == "diaries" || p[0] == "diary-jobs") {
		return true
	}
	return len(p) == 3 && (p[0] == "outings" && (p[2] == "ending" || p[2] == "diary-sources" || p[2] == "diary-jobs" || p[2] == "diary") || p[0] == "people" && p[2] == "diaries")
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	h.mux.ServeHTTP(w, r)
}

type denial struct {
	status int
	code   string
}

func (e *denial) Error() string        { return e.code }
func no(status int, code string) error { return &denial{status, code} }
func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}
func fail(w http.ResponseWriter, err error) {
	var d *denial
	if errors.As(err, &d) {
		reply(w, d.status, map[string]string{"code": d.code})
	} else {
		reply(w, 503, map[string]string{"code": "diary_unavailable"})
	}
}
func uuid() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		return no(415, "json_required")
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		return no(422, "invalid_diary_request")
	}
	return nil
}

// begin takes the same person -> session lock order as the existing AI engine.
func (h *Handler) begin(r *http.Request) (pgx.Tx, string, error) {
	token, p := auth.BearerToken(r.Header)
	if p != nil {
		return nil, "", no(401, "authentication_required")
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		return nil, "", err
	}
	person, err := session(r.Context(), tx, auth.TokenDigest(token))
	if err != nil {
		tx.Rollback(r.Context())
		return nil, "", err
	}
	return tx, person, nil
}
func session(ctx context.Context, tx pgx.Tx, digest []byte) (string, error) {
	var person, found string
	err := tx.QueryRow(ctx, `SELECT person_id FROM account_sessions WHERE token_digest=$1`, digest).Scan(&person)
	if err == nil {
		err = tx.QueryRow(ctx, `SELECT id FROM people WHERE id=$1 AND deleted_at IS NULL FOR SHARE`, person).Scan(&found)
	}
	if err == nil {
		err = tx.QueryRow(ctx, `SELECT id FROM account_sessions WHERE token_digest=$1 AND revoked_at IS NULL AND expires_at>clock_timestamp() FOR SHARE`, digest).Scan(&found)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return "", no(401, "authentication_required")
	}
	return person, err
}

type outing struct {
	ID, Context, Creator, Title, Start, End, ContextKind, Role string
	CanEnd                                                     bool
}

func authorize(ctx context.Context, tx pgx.Tx, id, person string) (outing, error) {
	o := outing{ID: id}
	if !chatv2.ValidID(id) {
		return o, no(404, "outing_not_found")
	}
	err := tx.QueryRow(ctx, `SELECT o.context_id,o.created_by_id,o.title,o.starts_on::text,o.ends_on::text,c.kind,m.role FROM outings o JOIN contexts c ON c.id=o.context_id JOIN memberships m ON m.context_id=c.id AND m.person_id=$2 AND m.state='active' AND m.left_at IS NULL WHERE o.id=$1 FOR SHARE OF m`, id, person).Scan(&o.Context, &o.Creator, &o.Title, &o.Start, &o.End, &o.ContextKind, &o.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return o, no(404, "outing_not_found")
	}
	if err != nil {
		return o, err
	}
	var creatorPresent, couple bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM memberships WHERE context_id=$1 AND person_id=$2 AND state='active' AND left_at IS NULL),EXISTS(SELECT 1 FROM active_couple_members ac JOIN pair_notebook_cycles cy ON cy.id=ac.cycle_id JOIN pair_notebooks pn ON pn.id=cy.notebook_id WHERE pn.context_id=$1 AND ac.person_id=$3)`, o.Context, o.Creator, person).Scan(&creatorPresent, &couple)
	o.CanEnd = o.Creator == person || couple || o.Role == "admin" && !creatorPresent
	return o, err
}

type Ending struct {
	Title   string     `json:"title"`
	Start   string     `json:"starts_on"`
	End     string     `json:"ends_on"`
	Kind    string     `json:"kind"`
	EndedAt *time.Time `json:"ended_at"`
	CanEnd  bool       `json:"can_end"`
	DiaryID *string    `json:"diary_id"`
}

func readEnding(ctx context.Context, tx pgx.Tx, o outing, person string) (Ending, error) {
	e := Ending{Title: o.Title, Start: o.Start, End: o.End, Kind: book.SuggestedKind(o.Start, o.End), CanEnd: o.CanEnd}
	err := tx.QueryRow(ctx, `SELECT kind,ended_at FROM outing_endings WHERE outing_id=$1`, o.ID).Scan(&e.Kind, &e.EndedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	if err != nil {
		return e, err
	}
	err = tx.QueryRow(ctx, `SELECT id FROM outing_diaries WHERE outing_id=$1 AND owner_id=$2`, o.ID, person).Scan(&e.DiaryID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	return e, err
}
func (h *Handler) ending(w http.ResponseWriter, r *http.Request) {
	tx, p, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	o, err := authorize(r.Context(), tx, r.PathValue("outing"), p)
	if err != nil {
		fail(w, err)
		return
	}
	e, err := readEnding(r.Context(), tx, o, p)
	if err != nil {
		fail(w, err)
		return
	}
	reply(w, 200, e)
}
func (h *Handler) end(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind string `json:"kind"`
	}
	if err := decode(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	if !book.ValidKind(in.Kind) {
		fail(w, no(422, "invalid_outing_kind"))
		return
	}
	tx, p, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	o, err := authorize(r.Context(), tx, r.PathValue("outing"), p)
	if err != nil {
		fail(w, err)
		return
	}
	if !o.CanEnd {
		fail(w, no(403, "organizer_required"))
		return
	}
	// A date passing only suggests an ending; this transition is always deliberate.
	if o.Start > time.Now().In(time.FixedZone("Vietnam", 7*3600)).Format("2006-01-02") {
		fail(w, no(409, "outing_not_started"))
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO outing_endings(outing_id,kind,ended_by) VALUES($1,$2,$3) ON CONFLICT(outing_id) DO NOTHING`, o.ID, in.Kind, p)
	if err != nil {
		fail(w, err)
		return
	}
	e, err := readEnding(r.Context(), tx, o, p)
	if err == nil && e.Kind != in.Kind {
		err = no(409, "ending_already_confirmed")
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		fail(w, err)
		return
	}
	reply(w, 200, e)
}

// Source reads never query messages. The device owns selecting chat excerpts.
func source(ctx context.Context, tx pgx.Tx, o outing, person string) (book.Source, error) {
	s := book.Source{Title: o.Title, StartsOn: o.Start, EndsOn: o.End, Photos: []book.Photo{}, Places: []string{}, Excerpts: []string{}}
	err := tx.QueryRow(ctx, `SELECT kind FROM outing_endings WHERE outing_id=$1`, o.ID).Scan(&s.Kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, no(409, "outing_not_ended")
	}
	if err != nil {
		return s, err
	}
	rows, err := tx.Query(ctx, `SELECT u.id, '/contexts/'||u.context_id::text||'/photos/'||u.id::text,COALESCE((SELECT m.caption FROM memories m WHERE m.context_id=u.context_id AND m.image_url='/contexts/'||u.context_id::text||'/photos/'||u.id::text ORDER BY m.created_at LIMIT 1),''),(u.created_at AT TIME ZONE 'Asia/Ho_Chi_Minh')::date::text FROM uploaded_images u WHERE u.context_id=$1 AND u.purpose='group' ORDER BY CASE WHEN (u.created_at AT TIME ZONE 'Asia/Ho_Chi_Minh')::date BETWEEN $2::date AND $3::date THEN 0 ELSE 1 END,u.created_at DESC,u.id LIMIT 200`, o.Context, o.Start, o.End)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var p book.Photo
		if err = rows.Scan(&p.ID, &p.URL, &p.Caption, &p.Day); err != nil {
			return s, err
		}
		s.Photos = append(s.Photos, p)
	}
	if err = rows.Err(); err != nil {
		return s, err
	}
	rows.Close()
	rows, err = tx.Query(ctx, `SELECT DISTINCT st.label FROM outing_stops st JOIN outing_stop_checkins c ON c.stop_id=st.id WHERE st.outing_id=$1 ORDER BY st.label LIMIT 100`, o.ID)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var label string
		if err = rows.Scan(&label); err != nil {
			return s, err
		}
		s.Places = append(s.Places, label)
	}
	return s, rows.Err()
}
func (h *Handler) sources(w http.ResponseWriter, r *http.Request) {
	tx, p, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	o, err := authorize(r.Context(), tx, r.PathValue("outing"), p)
	if err != nil {
		fail(w, err)
		return
	}
	s, err := source(r.Context(), tx, o, p)
	if err != nil {
		fail(w, err)
		return
	}
	reply(w, 200, s)
}

// checkedPhotos permits group photos and personal photos owned by this caller.
func checkedPhotos(ctx context.Context, tx pgx.Tx, o outing, person string, ids []string) (map[string]bool, error) {
	if len(ids) > book.MaxPhotos {
		return nil, no(422, "too_many_diary_photos")
	}
	allowed := map[string]bool{}
	for _, id := range ids {
		if !chatv2.ValidID(id) {
			return nil, no(422, "invalid_diary_photo")
		}
		var found string
		err := tx.QueryRow(ctx, `SELECT id FROM uploaded_images WHERE id=$1 AND ((context_id=$2 AND purpose='group') OR (owner_person_id=$3 AND purpose='personal')) FOR SHARE`, id, o.Context, person).Scan(&found)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, no(404, "diary_photo_not_found")
		}
		if err != nil {
			return nil, err
		}
		allowed[id] = true
	}
	return allowed, nil
}
func documentPhotos(d book.Document) []string {
	ids := []string{}
	seen := map[string]bool{}
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	add(d.CoverID)
	for _, p := range d.Pages {
		for _, id := range p.PhotoIDs {
			add(id)
		}
	}
	return ids
}

// hashInput is stable over the exact JSON bundle the user confirmed.
func hashInput(v any) string {
	b, _ := json.Marshal(v)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
