package profilemedia

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/repo"
)

type Handler struct {
	pool  *pgxpool.Pool
	mode  string
	proxy Proxy
	mux   *http.ServeMux
}

func RouteIDs() []string {
	return []string{
		"POST /me/profile-videos",
		"GET /me/profile-videos",
		"GET /me/profile-videos/credits",
		"GET /me/profile-videos/{job_id}",
		"GET /me/profile-videos/{job_id}/file",
	}
}

func Matches(path string) bool {
	return path == "/me/profile-videos" || strings.HasPrefix(path, "/me/profile-videos/")
}

func New(pool *pgxpool.Pool, mode string, proxy Proxy) *Handler {
	h := &Handler{pool: pool, mode: mode, proxy: proxy, mux: http.NewServeMux()}
	h.mux.HandleFunc("POST /me/profile-videos", h.create)
	h.mux.HandleFunc("GET /me/profile-videos", h.list)
	h.mux.HandleFunc("GET /me/profile-videos/credits", h.balance)
	h.mux.HandleFunc("GET /me/profile-videos/{job_id}", h.status)
	h.mux.HandleFunc("GET /me/profile-videos/{job_id}/file", h.file)
	return h
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		refuse(w, err)
		return
	}
	rows, err := h.pool.Query(r.Context(), `SELECT id,kind,status,created_at FROM profile_media_jobs
	 WHERE person_id=$1::uuid ORDER BY created_at DESC,id DESC LIMIT 50`, actor.ID)
	if err != nil {
		refuse(w, err)
		return
	}
	defer rows.Close()
	jobs := make([]map[string]any, 0)
	for rows.Next() {
		var id, kind, status string
		var created time.Time
		if err := rows.Scan(&id, &kind, &status, &created); err != nil {
			refuse(w, err)
			return
		}
		jobs = append(jobs, map[string]any{"job_id": id, "kind": kind, "status": status, "created_at": created.UTC().Format(time.RFC3339Nano)})
	}
	if err := rows.Err(); err != nil {
		refuse(w, err)
		return
	}
	respond(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) actor(r *http.Request) (*auth.Actor, error) {
	if h.mode == "dev" {
		actor, problem := auth.DevActor(r.Header)
		if problem != nil {
			return nil, &Error{problem.Status, problem.Code}
		}
		return actor, nil
	}
	actor, problem, err := auth.ProdActor(r.Context(), r.Header, repo.Sessions{Q: h.pool}, time.Now())
	if err != nil {
		return nil, err
	}
	if problem != nil {
		return nil, &Error{problem.Status, problem.Code}
	}
	return actor, nil
}

func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func refuse(w http.ResponseWriter, err error) {
	mediaErr := asMediaError(err)
	respond(w, mediaErr.Status, map[string]string{"code": mediaErr.Code, "detail": mediaErr.Code})
}

func decode(w http.ResponseWriter, r *http.Request, target any) error {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		return &Error{http.StatusUnsupportedMediaType, "json_required"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return &Error{http.StatusBadRequest, "invalid_body"}
	}
	if decoder.Decode(new(any)) != io.EOF {
		return &Error{http.StatusBadRequest, "invalid_body"}
	}
	return nil
}

type createInput struct {
	Kind            string   `json:"kind"`
	ImageJobIDs     []string `json:"image_job_ids"`
	SecondsPerImage *int     `json:"seconds_per_image"`
	IdempotencyKey  string   `json:"idempotency_key"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		refuse(w, err)
		return
	}
	var input createInput
	if err := decode(w, r, &input); err != nil {
		refuse(w, err)
		return
	}
	if input.Kind != "nep_video" || len(input.ImageJobIDs) == 0 || input.IdempotencyKey == "" {
		refuse(w, &Error{http.StatusUnprocessableEntity, "invalid_video_request"})
		return
	}
	if err := h.proxy.configured(); err != nil {
		refuse(w, err)
		return
	}
	for _, imageID := range input.ImageJobIDs {
		if !OwnsJob(imageID, actor.ID, h.proxy.PersonKey) {
			refuse(w, &Error{http.StatusNotFound, "khong_thay_job"})
			return
		}
	}
	jobID, err := NewJobID(actor.ID, h.proxy.PersonKey)
	if err != nil {
		refuse(w, err)
		return
	}
	payload, _ := json.Marshal(input)
	job, err := Reserve(r.Context(), h.pool, actor.ID, input.IdempotencyKey, jobID, "nep_video", payload)
	if errors.Is(err, ErrNoCredit) {
		refuse(w, &Error{http.StatusConflict, "video_credit_required"})
		return
	}
	if errors.Is(err, ErrKeyReuse) {
		refuse(w, &Error{http.StatusConflict, "idempotency_key_reuse"})
		return
	}
	if err != nil {
		refuse(w, err)
		return
	}
	if job.Status == "reserved" {
		if err := SetStatus(r.Context(), h.pool, actor.ID, job.ID, "queued", "", ""); err != nil {
			refuse(w, err)
			return
		}
		if err := h.proxy.SubmitVideo(r.Context(), actor.ID, job.ID, input.ImageJobIDs, input.SecondsPerImage); err != nil {
			_ = SetStatus(r.Context(), h.pool, actor.ID, job.ID, "failed", "proxy_refused", "")
			refuse(w, err)
			return
		}
		job.Status = "queued"
	}
	respond(w, http.StatusAccepted, map[string]any{"job_id": job.ID, "status": job.Status, "kind": job.Kind})
}

func (h *Handler) load(ctx context.Context, personID, jobID string) (Job, error) {
	var job Job
	err := h.pool.QueryRow(ctx, `SELECT id,person_id::text,credit_source,kind,status,idempotency_key,created_at
	 FROM profile_media_jobs WHERE person_id=$1::uuid AND id=$2`, personID, jobID).
		Scan(&job.ID, &job.PersonID, &job.CreditSource, &job.Kind, &job.Status, &job.IdempotencyKey, &job.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, &Error{http.StatusNotFound, "khong_thay_job"}
	}
	return job, err
}

func (h *Handler) recoverSubmission(ctx context.Context, job Job) error {
	var payload []byte
	if err := h.pool.QueryRow(ctx, `SELECT payload FROM profile_media_jobs WHERE person_id=$1::uuid AND id=$2`, job.PersonID, job.ID).Scan(&payload); err != nil {
		return err
	}
	var input createInput
	if err := json.Unmarshal(payload, &input); err != nil {
		return err
	}
	if err := h.proxy.SubmitVideo(ctx, job.PersonID, job.ID, input.ImageJobIDs, input.SecondsPerImage); err != nil {
		_ = SetStatus(ctx, h.pool, job.PersonID, job.ID, "failed", "proxy_refused", "")
		return err
	}
	if job.Status == "reserved" {
		return SetStatus(ctx, h.pool, job.PersonID, job.ID, "queued", "", "")
	}
	return nil
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		refuse(w, err)
		return
	}
	job, err := h.load(r.Context(), actor.ID, r.PathValue("job_id"))
	if err != nil {
		refuse(w, err)
		return
	}
	if job.Kind == "nep_video" && (job.Status == "reserved" || job.Status == "queued" || job.Status == "running") {
		upstream, err := h.proxy.Status(r.Context(), actor.ID, job.ID)
		if err != nil {
			var mediaErr *Error
			if !errors.As(err, &mediaErr) || mediaErr.Status != http.StatusNotFound {
				refuse(w, err)
				return
			}
			if err = h.recoverSubmission(r.Context(), job); err != nil {
				refuse(w, err)
				return
			}
			job.Status = "queued"
			respond(w, http.StatusOK, map[string]any{"job_id": job.ID, "status": job.Status, "kind": job.Kind})
			return
		}
		switch upstream.State {
		case "dang-cho":
			if job.Status == "reserved" {
				err = SetStatus(r.Context(), h.pool, actor.ID, job.ID, "queued", "", "")
			}
		case "dang-chay":
			err = SetStatus(r.Context(), h.pool, actor.ID, job.ID, "running", "", "")
		case "hong":
			err = SetStatus(r.Context(), h.pool, actor.ID, job.ID, "failed", "render_failed", "")
		case "xong":
			content, mediaType, fileErr := h.proxy.File(r.Context(), actor.ID, job.ID)
			if fileErr == nil && validMP4(content, mediaType) {
				err = SetStatus(r.Context(), h.pool, actor.ID, job.ID, "ready", "", "")
			} else if fileErr == nil {
				err = SetStatus(r.Context(), h.pool, actor.ID, job.ID, "failed", "invalid_video", "")
			}
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			refuse(w, err)
			return
		}
		job, err = h.load(r.Context(), actor.ID, job.ID)
		if err != nil {
			refuse(w, err)
			return
		}
	}
	respond(w, http.StatusOK, map[string]any{"job_id": job.ID, "status": job.Status, "kind": job.Kind})
}

// validMP4 checks a complete, non-fragmented H.264/H.265 container, not codec decoding.
func validMP4(content []byte, mediaType string) bool {
	if mediaType != "video/mp4" {
		return false
	}
	boxes, ok := mp4Boxes(content)
	if !ok || len(boxes) < 3 || boxes[0].kind != "ftyp" || len(boxes[0].data) < 8 {
		return false
	}
	var video, samples bool
	for _, box := range boxes {
		switch box.kind {
		case "moov":
			video = video || mp4VideoTrack(box.data)
		case "mdat":
			samples = samples || len(box.data) > 0
		}
	}
	return video && samples
}

type mp4Box struct {
	kind string
	data []byte
}

func mp4Boxes(content []byte) ([]mp4Box, bool) {
	var boxes []mp4Box
	for len(content) > 0 {
		if len(content) < 8 {
			return nil, false
		}
		size := uint64(binary.BigEndian.Uint32(content[:4]))
		header := uint64(8)
		if size == 1 {
			if len(content) < 16 {
				return nil, false
			}
			size = binary.BigEndian.Uint64(content[8:16])
			header = 16
		} else if size == 0 {
			size = uint64(len(content))
		}
		if size < header || size > uint64(len(content)) {
			return nil, false
		}
		boxes = append(boxes, mp4Box{kind: string(content[4:8]), data: content[header:size]})
		content = content[size:]
	}
	return boxes, true
}

func mp4VideoTrack(content []byte) bool {
	children, ok := mp4Boxes(content)
	if !ok {
		return false
	}
	for _, track := range children {
		if track.kind != "trak" {
			continue
		}
		trackChildren, ok := mp4Boxes(track.data)
		if !ok {
			continue
		}
		for _, media := range trackChildren {
			if media.kind != "mdia" {
				continue
			}
			mediaChildren, ok := mp4Boxes(media.data)
			if !ok {
				continue
			}
			var isVideo, hasSamples bool
			for _, child := range mediaChildren {
				switch child.kind {
				case "hdlr":
					isVideo = len(child.data) >= 12 && string(child.data[8:12]) == "vide"
				case "minf":
					hasSamples = mp4HasVideoSamples(child.data)
				}
			}
			if isVideo && hasSamples {
				return true
			}
		}
	}
	return false
}

func mp4HasVideoSamples(content []byte) bool {
	children, ok := mp4Boxes(content)
	if !ok {
		return false
	}
	for _, child := range children {
		if child.kind != "stbl" {
			continue
		}
		table, ok := mp4Boxes(child.data)
		if !ok {
			return false
		}
		var codec, samples bool
		for _, box := range table {
			switch box.kind {
			case "stsd":
				if len(box.data) >= 16 && binary.BigEndian.Uint32(box.data[4:8]) > 0 {
					switch string(box.data[12:16]) {
					case "avc1", "avc3", "hvc1", "hev1":
						codec = true
					}
				}
			case "stsz":
				samples = len(box.data) >= 12 && binary.BigEndian.Uint32(box.data[8:12]) > 0
			}
		}
		return codec && samples
	}
	return false
}

func (h *Handler) file(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		refuse(w, err)
		return
	}
	job, err := h.load(r.Context(), actor.ID, r.PathValue("job_id"))
	if err != nil {
		refuse(w, err)
		return
	}
	if job.Status != "ready" {
		refuse(w, &Error{http.StatusNotFound, "chua_co_media"})
		return
	}
	content, mediaType, err := h.proxy.File(r.Context(), actor.ID, job.ID)
	if err != nil {
		refuse(w, err)
		return
	}
	if !validMP4(content, mediaType) {
		refuse(w, &Error{http.StatusBadGateway, "invalid_video"})
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeContent(w, r, job.ID+".mp4", time.Time{}, bytes.NewReader(content))
}

func (h *Handler) balance(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		refuse(w, err)
		return
	}
	credits, err := CreditBalance(r.Context(), h.pool, actor.ID)
	if err != nil {
		refuse(w, err)
		return
	}
	respond(w, http.StatusOK, credits)
}
