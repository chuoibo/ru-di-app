package diary

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"mobile/services/core/internal/aiharness/nhatky"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/chatv2"
	book "mobile/services/core/internal/domain/diary"
	"mobile/services/core/internal/media/storage"
	"mobile/services/core/internal/pyjson"
)

type Job struct {
	ID     string         `json:"id"`
	Status string         `json:"status"`
	Code   *string        `json:"code"`
	Result *book.Document `json:"result"`
}

func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	var in struct {
		LogicalID string      `json:"logical_id"`
		Confirmed bool        `json:"confirmed"`
		UseAI     bool        `json:"use_ai"`
		Source    book.Source `json:"source"`
	}
	if err := decode(w, r, &in); err != nil {
		fail(w, err)
		return
	}
	s := in.Source
	if !in.Confirmed || !chatv2.ValidID(in.LogicalID) || !book.ValidKind(s.Kind) || len(s.Photos) > book.MaxPhotos || len(s.Excerpts) > 40 || len(s.Places) > 100 || utf8.RuneCountInString(s.Title) > 200 || strings.TrimSpace(s.Title) == "" {
		fail(w, no(422, "invalid_diary_request"))
		return
	}
	total := 0
	for _, v := range s.Excerpts {
		total += utf8.RuneCountInString(v)
		if utf8.RuneCountInString(v) > 2000 {
			fail(w, no(422, "diary_context_too_large"))
			return
		}
	}
	for _, v := range s.Places {
		if utf8.RuneCountInString(v) > 200 {
			fail(w, no(422, "diary_context_too_large"))
			return
		}
	}
	if total > 16000 {
		fail(w, no(422, "diary_context_too_large"))
		return
	}
	ids := []string{}
	for _, photo := range s.Photos {
		if utf8.RuneCountInString(photo.Caption) > 2000 || len(photo.Day) != 10 {
			fail(w, no(422, "invalid_diary_photo"))
			return
		}
		ids = append(ids, photo.ID)
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
	e, err := readEnding(ctx, tx, o, p)
	if err != nil {
		fail(w, err)
		return
	}
	if e.EndedAt == nil {
		fail(w, no(409, "outing_not_ended"))
		return
	}
	if s.StartsOn != o.Start || s.EndsOn != o.End || s.Kind != e.Kind {
		fail(w, no(409, "diary_source_changed"))
		return
	}
	if _, err = checkedPhotos(ctx, tx, o, p, ids); err != nil {
		fail(w, err)
		return
	}
	// URLs are display-only; the worker reads only authorized storage keys.
	for i := range s.Photos {
		s.Photos[i].URL = ""
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,1))`, p); err != nil {
		fail(w, err)
		return
	}
	digest := hashInput(struct {
		Outing string
		AI     bool
		Source book.Source
	}{o.ID, in.UseAI, s})
	var existing Job
	var oldDigest string
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT id,status,code,result,digest FROM outing_diary_jobs WHERE owner_id=$1 AND logical_id=$2`, p, in.LogicalID).Scan(&existing.ID, &existing.Status, &existing.Code, &raw, &oldDigest)
	if err == nil {
		if oldDigest != digest {
			fail(w, no(409, "diary_job_conflict"))
			return
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &existing.Result)
		}
		reply(w, 200, existing)
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		fail(w, err)
		return
	}
	var n int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM outing_diary_jobs WHERE owner_id=$1 AND created_at>clock_timestamp()-interval '1 hour'`, p).Scan(&n); err != nil {
		fail(w, err)
		return
	}
	if n >= 12 && in.UseAI {
		fail(w, no(429, "diary_rate_limited"))
		return
	}
	if n >= 120 {
		fail(w, no(429, "diary_manual_rate_limited"))
		return
	}
	j := Job{ID: uuid(), Status: "queued"}
	sourceRaw, _ := json.Marshal(s)
	var resultRaw []byte
	if !in.UseAI {
		d := book.Compose(s)
		j.Result = &d
		j.Status = "succeeded"
		resultRaw, _ = json.Marshal(d)
		sourceRaw = nil
	}
	token, _ := auth.BearerToken(r.Header)
	_, err = tx.Exec(ctx, `INSERT INTO outing_diary_jobs(id,outing_id,owner_id,logical_id,session_digest,digest,source,status,result) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, j.ID, o.ID, p, in.LogicalID, auth.TokenDigest(token), digest, sourceRaw, j.Status, resultRaw)
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		fail(w, err)
		return
	}
	reply(w, 202, j)
}
func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
	tx, p, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	id := r.PathValue("job")
	if !chatv2.ValidID(id) {
		fail(w, no(404, "diary_job_not_found"))
		return
	}
	var j Job
	var raw []byte
	var outingID string
	err = tx.QueryRow(r.Context(), `SELECT id,status,code,result,outing_id FROM outing_diary_jobs WHERE id=$1 AND owner_id=$2 AND expires_at>clock_timestamp()`, id, p).Scan(&j.ID, &j.Status, &j.Code, &raw, &outingID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = no(404, "diary_job_not_found")
	}
	if err != nil {
		fail(w, err)
		return
	}
	if _, err = authorize(r.Context(), tx, outingID, p); err != nil {
		fail(w, err)
		return
	}
	if len(raw) > 0 {
		if err = json.Unmarshal(raw, &j.Result); err != nil {
			fail(w, err)
			return
		}
	}
	reply(w, 200, j)
}

// Run shares the process's model door with the chat engine. Inference is
// bounded and outside database transactions; a lease recovers a crashed worker.
func (h *Handler) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t := time.NewTicker(time.Second)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					_, _ = h.ProcessOne(ctx)
				}
			}
		}()
	}
	wg.Wait()
}
func (h *Handler) ProcessOne(ctx context.Context) (bool, error) {
	// Up to four model calls through agy-proxy (aiharness/nhatky); the lease
	// below outlives this bound so a live job is never taken twice.
	ctx, cancel := context.WithTimeout(ctx, 150*time.Second)
	defer cancel()
	_, err := h.pool.Exec(ctx, `UPDATE outing_diary_jobs SET source=NULL,result=NULL,status=CASE WHEN status IN ('queued','running') THEN 'failed' ELSE status END,code=CASE WHEN status IN ('queued','running') THEN 'sharing_expired' ELSE code END WHERE expires_at<=clock_timestamp() AND (source IS NOT NULL OR result IS NOT NULL)`)
	if err != nil {
		return false, err
	}
	_, err = h.pool.Exec(ctx, `UPDATE outing_diary_jobs SET source=NULL,status='failed',code='worker_interrupted' WHERE status='running' AND lease_until<clock_timestamp() AND attempts>=3`)
	if err != nil {
		return false, err
	}
	var id, outingID, owner string
	var digest, raw []byte
	lease := uuid()
	err = h.pool.QueryRow(ctx, `WITH c AS (SELECT id FROM outing_diary_jobs WHERE (status='queued' OR (status='running' AND lease_until<clock_timestamp())) AND attempts<3 AND expires_at>clock_timestamp() ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE outing_diary_jobs j SET status='running',attempts=attempts+1,lease_id=$1,lease_until=clock_timestamp()+interval '160 seconds' FROM c WHERE j.id=c.id RETURNING j.id,j.outing_id,j.owner_id,j.session_digest,j.source`, lease).Scan(&id, &outingID, &owner, &digest, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	finish := func(result []byte, code string) error {
		tx, e := h.pool.Begin(ctx)
		if e != nil {
			return e
		}
		defer tx.Rollback(ctx)
		person, e := session(ctx, tx, digest)
		if e == nil && person == owner {
			_, e = authorize(ctx, tx, outingID, owner)
		}
		if e != nil || person != owner {
			code = "sharing_revoked"
			result = nil
		}
		status := "succeeded"
		var c *string
		if code != "" {
			status = "failed"
			c = &code
		}
		_, e = tx.Exec(ctx, `UPDATE outing_diary_jobs SET status=$3,result=$4,code=$5,source=NULL,lease_id=NULL,lease_until=NULL WHERE id=$1 AND lease_id=$2 AND expires_at>clock_timestamp()`, id, lease, status, result, c)
		if e != nil {
			return e
		}
		return tx.Commit(ctx)
	}
	var s book.Source
	if err = json.Unmarshal(raw, &s); err != nil {
		return true, finish(nil, "invalid_diary_source")
	}
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return true, err
	}
	person, err := session(ctx, tx, digest)
	if err != nil || person != owner {
		tx.Rollback(ctx)
		return true, finish(nil, "sharing_revoked")
	}
	o, err := authorize(ctx, tx, outingID, owner)
	if err != nil {
		tx.Rollback(ctx)
		return true, finish(nil, "sharing_revoked")
	}
	ids := []string{}
	for _, photo := range s.Photos {
		ids = append(ids, photo.ID)
	}
	allowed, err := checkedPhotos(ctx, tx, o, owner, ids)
	if err != nil {
		tx.Rollback(ctx)
		return true, finish(nil, "sharing_revoked")
	}
	if !h.ai.CoMay() {
		tx.Rollback(ctx)
		return true, finish(nil, "diary_ai_unavailable")
	}
	images := []nhatky.Anh{}
	st, err := storage.New()
	if err != nil {
		tx.Rollback(ctx)
		return true, finish(nil, "diary_media_unavailable")
	}
	size := 0
	tooLarge := false
	for _, photo := range s.Photos {
		var key, mime string
		err = tx.QueryRow(ctx, `SELECT storage_key,content_type FROM uploaded_images WHERE id=$1`, photo.ID).Scan(&key, &mime)
		if err != nil {
			break
		}
		var data []byte
		data, err = st.Read(key)
		if err != nil {
			break
		}
		size += len(data)
		if size > nhatky.MaxByteAnh {
			tooLarge = true
			break
		}
		images = append(images, nhatky.Anh{ID: photo.ID, MIME: mime, Data: data})
	}
	tx.Rollback(ctx)
	if tooLarge {
		return true, finish(nil, "diary_images_too_large")
	}
	if err != nil {
		return true, finish(nil, "diary_media_unavailable")
	}
	sourceRaw, _ := json.Marshal(s)
	v, err := pyjson.Loads(sourceRaw)
	if err != nil {
		return true, finish(nil, "invalid_diary_source")
	}
	parts, err := nhatky.Phan(v, images)
	if err != nil {
		return true, finish(nil, "invalid_diary_source")
	}
	answer, err := nhatky.Viet(ctx, h.ai.Luot(nhatky.Luot), parts)
	if err != nil {
		return true, finish(nil, "diary_ai_unavailable")
	}
	answerRaw, err := pyjson.Dumps(answer)
	if err != nil {
		return true, finish(nil, "invalid_diary_result")
	}
	var d book.Document
	if err = json.Unmarshal(answerRaw, &d); err != nil {
		return true, finish(nil, "invalid_diary_result")
	}
	d.AIGenerated = true
	if err = book.Validate(d, allowed); err != nil {
		return true, finish(nil, "invalid_diary_result")
	}
	answerRaw, _ = json.Marshal(d)
	return true, finish(answerRaw, "")
}
