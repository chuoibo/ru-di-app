// Package congdong is the model step of the community (internal/community):
// a first reading of a public post or comment, and Nếp's draft of a member's
// own post. The Python brain only forwarded both to an operator endpoint that
// was never in the repository, so these instructions (duyet.txt, nep.txt) are
// new with ADR-0052; the contract they answer is the one community already
// held: relevant, safe, confidence_milli, reason for a reading, draft for Nếp.
//
// It decides nothing. community.decision publishes only from a confident,
// complete reading and sends everything else to a human; a reading of media
// the model never saw says so (MediaChecked false). Every image goes, shrunk
// to fit (anh.go), and a video goes as its review cut, one piece a call.
package congdong

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/cautruc"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/motluot"
)

//go:embed duyet.txt
var huongDanDuyet string

//go:embed nep.txt
var huongDanNep string

const (
	// MaxByteAnh bounds the raw bytes of the images one reading sends; see
	// aiharness/nhatky for why 14 MiB fits agy-proxy's request body.
	MaxByteAnh = 14 << 20
	// GiayDoan is the length in seconds of one piece of a video's review
	// cut, and MaxDoan the most pieces a video has: community caps a video
	// at 180 s. A piece is one call. Measured through agy-proxy on
	// 2026-10-02: a whole 180 s cut took 171 s once and had no answer in
	// 400 s the next, past the client's 90 s; two 45 s pieces read side by
	// side took 30 s each (4161 prompt tokens), and the model placed shapes
	// it was not told about at the right seconds of each.
	GiayDoan   = 45
	MaxDoan    = 4
	maxRaDuyet = 256
	maxRaNep   = 4096
	// moiLoiDuyet is one reading's wait: a piece of video, or every image
	// of a post, under agy-proxy's 90 s client. The reading is a queued
	// job, not a request someone waits on; Nếp's draft is.
	moiLoiDuyet = 85 * time.Second
	moiLoiNep   = 45 * time.Second
	nhietDoNep  = 0.7
	maxLyDoRune = 200
)

// ErrKetQua: the answer is not the shape asked for.
var ErrKetQua = errors.New("congdong: invalid inference result")

// Media is one attachment of the submission: its stored type and what of it
// a reading can send. An image sends its bytes (Data), shrunk to fit. A video
// sends its review cut (Doan): pieces of GiayDoan seconds at one frame a
// second, low resolution, with their sound, which the media runtime made
// when it processed the upload. A video without a cut is never sent.
type Media struct {
	MIME string
	Data []byte
	Doan [][]byte
}

var anhDuocPhep = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}

const mimeDoan = "video/mp4"

// Doc is a reading, in the field names community's verdict decodes.
type Doc struct {
	Relevant     bool   `json:"relevant"`
	Safe         bool   `json:"safe"`
	Confidence   int    `json:"confidence_milli"`
	Reason       string `json:"reason"`
	MediaChecked bool   `json:"media_checked"`
}

// LuocDoDuyet is the reading's response schema. media_checked is not in it:
// whether the media was checked is a fact about what was sent, which Go
// knows and the model does not.
func LuocDoDuyet() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"relevant":         {Type: genai.TypeBoolean},
			"safe":             {Type: genai.TypeBoolean},
			"confidence_milli": {Type: genai.TypeInteger, Minimum: genai.Ptr(0.0), Maximum: genai.Ptr(1000.0)},
			"reason":           {Type: genai.TypeString},
		},
		Required:         []string{"relevant", "safe", "confidence_milli", "reason"},
		PropertyOrdering: []string{"relevant", "safe", "confidence_milli", "reason"},
	}
}

// CacYeuCau are a submission's reading requests, and whether they carry all
// of its media. When every attachment can go: one request with the text and
// every image (left out when there are none but a video), then one per
// piece of each video, each with the text. When one cannot: the text alone,
// since a reading of half the media is not a reading of the post and it
// goes to a human either way.
func CacYeuCau(body string, comment bool, media []Media) ([]*model.LLMRequest, bool) {
	kind := "POST"
	if comment {
		kind = "COMMENT"
	}
	submission := map[string]any{"kind": kind, "text": body, "attachments": len(media)}
	images := []Media{}
	videos := []Media{}
	all := true
	for _, m := range media {
		switch {
		case anhDuocPhep[m.MIME] && len(m.Data) > 0:
			images = append(images, m)
		case strings.HasPrefix(m.MIME, "video/") && len(m.Doan) > 0 && len(m.Doan) <= MaxDoan:
			videos = append(videos, m)
		default:
			all = false
		}
	}
	shrunk, fit := vuaAnh(images)
	if !all || !fit {
		return []*model.LLMRequest{yeuCau(submission)}, false
	}
	reqs := []*model.LLMRequest{}
	if len(images) > 0 || len(videos) == 0 {
		parts := []*genai.Part{}
		for _, b := range shrunk {
			parts = append(parts, cautruc.Anh("image/jpeg", b))
		}
		reqs = append(reqs, yeuCau(submission, parts...))
	}
	for i, v := range videos {
		for j, piece := range v.Doan {
			part := map[string]any{}
			for k, x := range submission {
				part[k] = x
			}
			part["video"] = map[string]any{"number": i + 1, "piece": j + 1, "pieces": len(v.Doan), "starts_at_second": j * GiayDoan}
			reqs = append(reqs, yeuCau(part, cautruc.Anh(mimeDoan, piece)))
		}
	}
	return reqs, len(media) > 0
}

// yeuCau is one reading request: the submission as one JSON text, then the
// media parts.
func yeuCau(submission map[string]any, media ...*genai.Part) *model.LLMRequest {
	parts := append([]*genai.Part{{Text: "SUBMISSION (JSON):\n" + jsonText(submission)}}, media...)
	return cautruc.NhietDo(cautruc.YeuCauPhan(llm.BuocTrichXuat, huongDanDuyet, parts, LuocDoDuyet(), maxRaDuyet), 0)
}

// Duyet reads one submission, one call per request of CacYeuCau, and joins
// the readings: relevant when one is, safe when all are, as confident as the
// least confident, with the reason of the first unsafe reading or else of
// the least confident. MediaChecked is true only when the submission had
// media and every attachment went with the readings. A call that fails
// fails the whole reading, so the job waits and retries. mo opens the call
// budget, sized to the requests (a process's May.Luot).
func Duyet(ctx context.Context, mo func(tran int) *motluot.Luot, body string, comment bool, media []Media) (Doc, error) {
	reqs, checked := CacYeuCau(body, comment, media)
	l := mo(len(reqs))
	var out Doc
	unsafe := false
	for i, req := range reqs {
		d, err := doc(ctx, l, req)
		if err != nil {
			return Doc{}, err
		}
		if i == 0 {
			out = d
			unsafe = !d.Safe
			continue
		}
		out.Relevant = out.Relevant || d.Relevant
		out.Safe = out.Safe && d.Safe
		switch {
		case !d.Safe && !unsafe:
			out.Reason, unsafe = d.Reason, true
		case !unsafe && d.Confidence < out.Confidence:
			out.Reason = d.Reason
		}
		out.Confidence = min(out.Confidence, d.Confidence)
	}
	out.MediaChecked = checked
	return out, nil
}

// doc is one reading's answer, in Doc's shape.
func doc(ctx context.Context, l *motluot.Luot, req *model.LLMRequest) (Doc, error) {
	ctx, cancel := context.WithTimeout(ctx, moiLoiDuyet)
	defer cancel()
	text, err := l.Goi(ctx, req)
	if err != nil {
		return Doc{}, err
	}
	raw, err := motluot.DocDoiTuong(text)
	if err != nil {
		return Doc{}, ErrKetQua
	}
	relevant, ok1 := raw["relevant"].(bool)
	safe, ok2 := raw["safe"].(bool)
	n, ok3 := raw["confidence_milli"].(json.Number)
	reason, _ := raw["reason"].(string)
	confidence, err := n.Int64()
	if !ok1 || !ok2 || !ok3 || err != nil {
		return Doc{}, ErrKetQua
	}
	if r := []rune(strings.TrimSpace(reason)); len(r) > maxLyDoRune {
		reason = string(r[:maxLyDoRune])
	}
	d := Doc{Relevant: relevant, Safe: safe, Confidence: int(confidence), Reason: reason}
	if int64(d.Confidence) != confidence {
		return Doc{}, ErrKetQua
	}
	return d, nil
}

// Nep drafts a rewrite of the member's own confirmed excerpt.
func Nep(ctx context.Context, l *motluot.Luot, excerpt, request string) (string, error) {
	data := jsonText(map[string]string{"excerpt": excerpt, "request": request})
	schema := &genai.Schema{
		Type:       genai.TypeObject,
		Properties: map[string]*genai.Schema{"draft": {Type: genai.TypeString}},
		Required:   []string{"draft"},
	}
	req := cautruc.NhietDo(cautruc.YeuCau(llm.BuocViet, huongDanNep, "NGUỒN ĐÃ XÁC NHẬN (JSON):\n"+data, schema, maxRaNep), nhietDoNep)
	ctx, cancel := context.WithTimeout(ctx, moiLoiNep)
	defer cancel()
	text, err := l.Goi(ctx, req)
	if err != nil {
		return "", err
	}
	raw, err := motluot.DocDoiTuong(text)
	if err != nil {
		return "", ErrKetQua
	}
	draft, ok := raw["draft"].(string)
	if !ok || strings.TrimSpace(draft) == "" {
		return "", ErrKetQua
	}
	return strings.TrimSpace(draft), nil
}

// jsonText is v as one line of JSON with <, > and & left as written.
func jsonText(v any) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimSuffix(b.String(), "\n")
}
