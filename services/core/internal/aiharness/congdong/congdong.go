// Package congdong is the model step of the community (internal/community):
// a first reading of a public post or comment, and Nếp's draft of a member's
// own post. The Python brain only forwarded both to an operator endpoint that
// was never in the repository, so these instructions (duyet.txt, nep.txt) are
// new with ADR-0052; the contract they answer is the one community already
// held: relevant, safe, confidence_milli, reason for a reading, draft for Nếp.
//
// It decides nothing. community.decision publishes only from a confident,
// complete reading and sends everything else to a human; a reading of media
// the model never saw says so (MediaChecked false).
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
	MaxByteAnh  = 14 << 20
	maxRaDuyet  = 256
	maxRaNep    = 4096
	moiLoi      = 45 * time.Second
	nhietDoNep  = 0.7
	maxLyDoRune = 200
)

// ErrKetQua: the answer is not the shape asked for.
var ErrKetQua = errors.New("congdong: invalid inference result")

// Media is one attachment of the submission: its stored type and bytes.
// Only images reach the model; a video is never sent.
type Media struct {
	MIME string
	Data []byte
}

var anhDuocPhep = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}

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

// YeuCauDuyet is the reading request: the submission as one JSON text, then
// every image that fits, and whether all media went with it.
func YeuCauDuyet(body string, comment bool, media []Media) (*model.LLMRequest, bool) {
	kind := "POST"
	if comment {
		kind = "COMMENT"
	}
	data := jsonText(map[string]any{"kind": kind, "text": body, "attachments": len(media)})
	parts := []*genai.Part{{Text: "SUBMISSION (JSON):\n" + data}}
	all := true
	total := 0
	for _, m := range media {
		total += len(m.Data)
		if !anhDuocPhep[m.MIME] || total > MaxByteAnh {
			all = false
			continue
		}
		parts = append(parts, cautruc.Anh(m.MIME, m.Data))
	}
	if !all {
		// Nothing but the text: a reading of half the media is not a
		// reading of the post, and it goes to a human either way.
		parts = parts[:1]
	}
	req := cautruc.YeuCauPhan(llm.BuocTrichXuat, huongDanDuyet, parts, LuocDoDuyet(), maxRaDuyet)
	return cautruc.NhietDo(req, 0), all && len(media) > 0
}

// Duyet reads one submission. MediaChecked is true only when the submission
// had media and every attachment was an image sent with the request.
func Duyet(ctx context.Context, l *motluot.Luot, body string, comment bool, media []Media) (Doc, error) {
	req, checked := YeuCauDuyet(body, comment, media)
	ctx, cancel := context.WithTimeout(ctx, moiLoi)
	defer cancel()
	text, err := l.Goi(ctx, req)
	if err != nil {
		return Doc{}, err
	}
	raw, err := motluot.DocDoiTuong(text)
	if err != nil {
		return Doc{}, ErrKetQua
	}
	var d Doc
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
	d = Doc{Relevant: relevant, Safe: safe, Confidence: int(confidence), Reason: reason, MediaChecked: checked}
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
	ctx, cancel := context.WithTimeout(ctx, moiLoi)
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
