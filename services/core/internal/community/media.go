package community

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mobile/services/core/internal/aiharness/congdong"
	"mobile/services/core/internal/media/storage"
)

// uploadTimeout bounds one upload end to end; uploadReadTimeout bounds
// receiving its body.
const (
	uploadTimeout     = 90 * time.Second
	uploadReadTimeout = 60 * time.Second
)

// upload keeps every slow step out of a database transaction (audit
// 2026-10-05, PER-COMMUNITY-01). It used to open the transaction, lock the
// person, session and rate row, and only then receive, decode and store the
// body: four slow clients held all four slots, four connections and their
// locks for as long as they cared to take. Now the caller is checked, the body
// received under its own deadline, decoded and stored with no transaction
// open, and one short transaction re-checks the session and records the row.
// The rate is spent up front (see below).
func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	select {
	case h.uploadSlots <- struct{}{}:
		defer func() { <-h.uploadSlots }()
	default:
		fail(w, no(429, "media_busy"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), uploadTimeout)
	defer cancel()
	r = r.WithContext(ctx)
	// Who is asking, and the per-minute allowance spent, before a byte of
	// the body is read, in one short transaction that commits at once: an
	// attempt counts whether or not it succeeds, so a caller over the limit
	// is refused here and never makes the server receive or decode anything.
	check, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	err = rate(ctx, check, person)
	if err == nil {
		err = check.Commit(ctx)
	}
	_ = check.Rollback(context.WithoutCancel(ctx))
	if err != nil {
		fail(w, err)
		return
	}
	mime := strings.Split(r.Header.Get("Content-Type"), ";")[0]
	max := int64(12 << 20)
	if mime == "video/mp4" || mime == "video/quicktime" {
		max = 64 << 20
	} else if mime != "image/jpeg" && mime != "image/png" {
		fail(w, no(422, "unsupported_media"))
		return
	}
	rc := http.NewResponseController(w)
	deadlineSet := rc.SetReadDeadline(time.Now().Add(uploadReadTimeout)) == nil
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, max))
	if err != nil || len(raw) == 0 {
		fail(w, no(413, "media_too_large"))
		return
	}
	if deadlineSet {
		_ = rc.SetReadDeadline(time.Time{})
	}
	if strings.HasPrefix(mime, "video/") && (len(raw) < 12 || string(raw[4:8]) != "ftyp") {
		fail(w, no(422, "invalid_video"))
		return
	}
	width, height := 0, 0
	state := "processing"
	if strings.HasPrefix(mime, "image/") {
		config, format, e := image.DecodeConfig(bytes.NewReader(raw))
		if e != nil || "image/"+format != mime || config.Width < 1 || config.Height < 1 || config.Width > 6000 || config.Height > 6000 || int64(config.Width)*int64(config.Height) > 24000000 {
			fail(w, no(422, "invalid_image"))
			return
		}
		img, _, e := image.Decode(bytes.NewReader(raw))
		if e != nil {
			fail(w, no(422, "invalid_image"))
			return
		}
		var clean bytes.Buffer
		if mime == "image/png" {
			err = png.Encode(&clean, img)
		} else {
			err = jpeg.Encode(&clean, img, &jpeg.Options{Quality: 88})
		}
		if err != nil {
			fail(w, err)
			return
		}
		raw = clean.Bytes()
		width, height = config.Width, config.Height
		state = "ready"
	}
	st, err := storage.New()
	if err != nil {
		fail(w, err)
		return
	}
	key, err := storage.NewStorageKey()
	if err == nil {
		err = st.Write(key, raw)
	}
	if err != nil {
		fail(w, err)
		return
	}
	id := uuid()
	tx, owner, err := h.begin(r)
	if err == nil && owner != person {
		err = no(401, "authentication_required")
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO community_media(id,owner_id,storage_key,content_type,byte_size,width,height,state) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, person, key, mime, len(raw), width, height, state)
		if err == nil {
			err = tx.Commit(ctx)
		}
		_ = tx.Rollback(context.WithoutCancel(ctx))
	}
	if err != nil {
		if removed, delErr := st.Delete(key); delErr != nil || !removed {
			forgetLater(h, key)
		}
		fail(w, err)
		return
	}
	reply(w, 201, map[string]any{"id": id, "state": state, "url": "/v2/community/media/" + id, "type": mime, "width": width, "height": height})
}

// forgetLater queues a stored object that lost its row for the janitor's
// reaper, when the store could not remove it at once. Best effort: a database
// without the queue (a community-only stack) leaves it to the media GC.
func forgetLater(h *Handler, key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = h.pool.Exec(ctx, `INSERT INTO pending_object_deletes(storage_key,reason) VALUES($1,'community_upload_failed') ON CONFLICT (storage_key) DO NOTHING`, key)
}
func (h *Handler) mediaStatus(w http.ResponseWriter, r *http.Request) {
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	id := r.PathValue("media")
	if !validID(id) {
		fail(w, no(404, "media_not_found"))
		return
	}
	var state string
	err = tx.QueryRow(r.Context(), `SELECT state FROM community_media WHERE id=$1 AND owner_id=$2`, id, person).Scan(&state)
	if err != nil {
		fail(w, no(404, "media_not_found"))
		return
	}
	commit(w, r, tx, 200, map[string]string{"id": id, "state": state})
}
func (h *Handler) media(w http.ResponseWriter, r *http.Request) {
	tx, person, err := h.begin(r)
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	id := r.PathValue("media")
	if !validID(id) {
		fail(w, no(404, "media_not_found"))
		return
	}
	var key, mime string
	err = tx.QueryRow(r.Context(), `SELECT m.storage_key,m.content_type FROM community_media m WHERE m.id=$2 AND m.state='ready' AND (
 m.owner_id=$1 OR EXISTS(SELECT 1 FROM community_posts c JOIN posts p ON p.id=c.post_id JOIN community_revisions v ON v.post_id=c.post_id AND v.revision=c.published_revision WHERE $2=ANY(v.media_ids) AND c.deleted_at IS NULL AND (`+readableSQL+`)) OR
 EXISTS(SELECT 1 FROM community_comment_meta cm JOIN post_comments cc ON cc.id=cm.comment_id JOIN people ca ON ca.id=cc.author_id JOIN posts p ON p.id=cc.post_id JOIN people pa ON pa.id=p.author_id WHERE cm.media_id=$2 AND ca.deleted_at IS NULL AND pa.deleted_at IS NULL AND NOT EXISTS(SELECT 1 FROM friend_requests f WHERE f.state='blocked' AND ((f.requester_id=$1 AND f.addressee_id=cc.author_id) OR (f.addressee_id=$1 AND f.requester_id=cc.author_id))) AND (`+readableSQL+`)) OR
 EXISTS(SELECT 1 FROM community_moderators WHERE person_id=$1))`, person, id).Scan(&key, &mime)
	if err != nil {
		fail(w, no(404, "media_not_found"))
		return
	}
	st, err := storage.New()
	if err != nil {
		fail(w, err)
		return
	}
	path, err := st.PathFor(key)
	if err != nil {
		fail(w, err)
		return
	}
	file, err := os.Open(path)
	if err != nil {
		fail(w, no(404, "media_not_found"))
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		fail(w, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", "inline")
	http.ServeContent(w, r, "media", info.ModTime(), file)
}
func (h *Handler) processVideo(ctx context.Context) {
	var id, key string
	err := h.pool.QueryRow(ctx, `UPDATE community_media SET lease_until=clock_timestamp()+interval '6 minutes' WHERE id=(SELECT id FROM community_media WHERE state='processing' AND (lease_until IS NULL OR lease_until<clock_timestamp()) ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id,storage_key`).Scan(&id, &key)
	if err != nil {
		return
	}
	duration, width, height, newKey, reviewKeys, err := transcode(ctx, key)
	if err != nil {
		_, _ = h.pool.Exec(ctx, `UPDATE community_media SET state='failed',lease_until=NULL WHERE id=$1`, id)
		return
	}
	st, e := storage.New()
	if e != nil {
		return
	}
	drop := func(keys ...string) {
		for _, k := range keys {
			_, _ = st.Delete(k)
		}
	}
	path, e := st.PathFor(newKey)
	if e != nil {
		return
	}
	info, e := os.Stat(path)
	if e != nil {
		drop(append(reviewKeys, newKey)...)
		return
	}
	tag, err := h.pool.Exec(ctx, `UPDATE community_media SET storage_key=$2,content_type='video/mp4',state='ready',duration_ms=$3,width=$4,height=$5,byte_size=$6,review_keys=$7,lease_until=NULL WHERE id=$1 AND state='processing'`, id, newKey, duration, width, height, info.Size(), reviewKeys)
	if err == nil && tag.RowsAffected() == 1 {
		drop(key)
	} else {
		drop(append(reviewKeys, newKey)...)
	}
}

// RunMedia is a separate process so codec CPU and memory do not compete with API work.
func (h *Handler) RunMedia(ctx context.Context) error {
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("community media requires %s", name)
		}
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			h.processVideo(ctx)
		}
	}
}

// transcode makes the served video and its review cut (cutReview) from the
// upload, and stores both.
func transcode(ctx context.Context, key string) (int, int, int, string, []string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	st, err := storage.New()
	if err != nil {
		return 0, 0, 0, "", nil, err
	}
	source, err := st.PathFor(key)
	if err != nil {
		return 0, 0, 0, "", nil, err
	}
	raw, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-protocol_whitelist", "file,pipe", "-show_entries", "format=duration:stream=codec_type,width,height", "-of", "json", source).Output()
	if err != nil {
		return 0, 0, 0, "", nil, err
	}
	var info struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			Type   string `json:"codec_type"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"streams"`
	}
	if json.Unmarshal(raw, &info) != nil {
		return 0, 0, 0, "", nil, no(422, "invalid_video")
	}
	seconds, err := strconv.ParseFloat(info.Format.Duration, 64)
	if err != nil || seconds <= 0 || seconds > 180 {
		return 0, 0, 0, "", nil, no(422, "video_too_long")
	}
	width, height := 0, 0
	for _, s := range info.Streams {
		if s.Type == "video" {
			width, height = s.Width, s.Height
			break
		}
	}
	if width < 1 || height < 1 || width > 7680 || height > 7680 {
		return 0, 0, 0, "", nil, no(422, "invalid_video")
	}
	dir, err := os.MkdirTemp("", "rudi-community-video-")
	if err != nil {
		return 0, 0, 0, "", nil, err
	}
	defer os.RemoveAll(dir)
	dest := filepath.Join(dir, "processed.mp4")
	err = exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-v", "error", "-protocol_whitelist", "file,pipe", "-i", source, "-map", "0:v:0", "-map", "0:a:0?", "-map_metadata", "-1", "-vf", "scale='min(1280,iw)':'min(1280,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2", "-c:v", "libx264", "-preset", "fast", "-crf", "24", "-threads", "2", "-c:a", "aac", "-b:a", "128k", "-movflags", "+faststart", "-t", "180", dest).Run()
	if err != nil {
		return 0, 0, 0, "", nil, err
	}
	processed, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-protocol_whitelist", "file,pipe", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "json", dest).Output()
	if err != nil || json.Unmarshal(processed, &info) != nil || len(info.Streams) != 1 {
		return 0, 0, 0, "", nil, no(422, "invalid_video")
	}
	width, height = info.Streams[0].Width, info.Streams[0].Height
	data, err := os.ReadFile(dest)
	if err != nil || len(data) > 64<<20 {
		return 0, 0, 0, "", nil, no(422, "video_too_large")
	}
	pieces, err := cutReview(ctx, dest, seconds, dir)
	if err != nil {
		return 0, 0, 0, "", nil, err
	}
	keys := []string{}
	for _, b := range append([][]byte{data}, pieces...) {
		k, e := storage.NewStorageKey()
		if e == nil {
			e = st.Write(k, b)
		}
		if e != nil {
			for _, written := range keys {
				_, _ = st.Delete(written)
			}
			return 0, 0, 0, "", nil, e
		}
		keys = append(keys, k)
	}
	return int(seconds * 1000), width, height, keys[0], keys[1:], nil
}

// cutReview is the review cut of a processed video: one piece per
// congdong.GiayDoan seconds, at one frame a second, at most 360 pixels high,
// with its sound in mono, which is what a moderation reading sends the model
// in place of the video. A tail under a second gets no piece of its own.
// Each piece is its own encode: ffmpeg's segment muxer split the first piece
// at 97 s instead of 45 when tried.
func cutReview(ctx context.Context, source string, seconds float64, dir string) ([][]byte, error) {
	pieces := [][]byte{}
	for start := 0; start == 0 || float64(start) < seconds-1; start += congdong.GiayDoan {
		out := filepath.Join(dir, fmt.Sprintf("review-%d.mp4", start))
		err := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-v", "error", "-protocol_whitelist", "file,pipe", "-ss", strconv.Itoa(start), "-i", source, "-t", strconv.Itoa(congdong.GiayDoan), "-map", "0:v:0", "-map", "0:a:0?", "-map_metadata", "-1", "-vf", "fps=1,scale=-2:'min(360,ih)'", "-c:v", "libx264", "-preset", "veryfast", "-crf", "32", "-threads", "2", "-c:a", "aac", "-ac", "1", "-ar", "16000", "-b:a", "32k", "-movflags", "+faststart", out).Run()
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(out)
		if err != nil {
			return nil, err
		}
		pieces = append(pieces, b)
	}
	if len(pieces) > congdong.MaxDoan {
		return nil, no(422, "video_too_long")
	}
	return pieces, nil
}
