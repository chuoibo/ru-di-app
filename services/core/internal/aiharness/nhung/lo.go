package nhung

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/llm"
)

// Lo is the embedder's batch door: the Gemini Batch API, at half the online
// price (ADR-0049 §2.7). Same model, same dimensionality, same in-text prefix
// (DinhDang) and the same normalisation as the online door, so a vector from
// a batch is the vector the online door returns for the same text (measured
// 2026-09-28 on gemini-embedding-2: cosine 1.0, max |difference| 0). Only the
// ingest's documents go through it; a query is always online.
type Lo struct {
	client *genai.Client
}

// TaiLieuLo is one document of a batch, keyed by the caller (the content hash).
type TaiLieuLo struct {
	Khoa    string
	TieuDe  string
	NoiDung string
}

// TrangThaiLo is where a batch job is.
type TrangThaiLo string

const (
	LoDangChay TrangThaiLo = "dang_chay"
	LoXong     TrangThaiLo = "xong"
	LoHong     TrangThaiLo = "hong"
)

// KetQuaLo is a job read back. Vecs is set once the job is done: key →
// normalised vector. LoiDong counts lines the provider answered with an
// error or a vector of the wrong size; they are left out.
type KetQuaLo struct {
	TrangThai TrangThaiLo
	Vecs      map[string][]float32
	LoiDong   int
	// Loi is the provider's job error, when it failed.
	Loi string
}

// MaxDongLo bounds one batch: the whole catalogue (~25k chunks) fits many
// times over, and the input file stays far under the API's 2 GB.
const MaxDongLo = 200_000

// NewLo builds the batch door under the same guard as NewGemini: a test
// binary may only reach a loopback server.
func NewLo(ctx context.Context, apiKey, baseURL string) (*Lo, error) {
	if apiKey == "" {
		return nil, llm.ErrNotConfigured
	}
	if baseURL != "" {
		if err := llm.CheckBaseURL(baseURL); err != nil {
			return nil, err
		}
	} else {
		if testing.Testing() {
			return nil, errors.New("nhung: a test binary may only reach a loopback Gemini")
		}
		baseURL = llm.DefaultBaseURL
	}
	c, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:      apiKey,
		Backend:     genai.BackendGeminiAPI,
		HTTPOptions: genai.HTTPOptions{BaseURL: baseURL},
		// The input file of a whole catalogue is tens of MB.
		HTTPClient: &http.Client{Timeout: 10 * time.Minute},
	})
	if err != nil {
		return nil, err
	}
	return &Lo{client: c}, nil
}

// LoFromEnv reads GEMINI_API_KEY and MOBILE_GEMINI_BASE_URL.
func LoFromEnv(ctx context.Context, getenv func(string) string) (*Lo, error) {
	return NewLo(ctx, getenv(llm.EnvAPIKey), getenv(llm.EnvBaseURL))
}

func (l *Lo) Model() string { return Model }
func (l *Lo) Dims() int     { return Dims }

type dongVao struct {
	Key     string `json:"key"`
	Request struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		OutputDimensionality int `json:"output_dimensionality"`
	} `json:"request"`
}

// FileVao is the JSONL input file of docs: one request per line, the text
// already prefixed as the online door prefixes it, the output dimensionality
// and nothing else (no TaskType, no title field: research §Kiểm chứng).
func FileVao(docs []TaiLieuLo) ([]byte, error) {
	if len(docs) == 0 {
		return nil, ErrRong
	}
	if len(docs) > MaxDongLo {
		return nil, fmt.Errorf("nhung: %d documents in one batch, at most %d", len(docs), MaxDongLo)
	}
	var b bytes.Buffer
	seen := map[string]bool{}
	for _, d := range docs {
		if d.Khoa == "" || seen[d.Khoa] {
			return nil, fmt.Errorf("nhung: batch key %q empty or repeated", d.Khoa)
		}
		seen[d.Khoa] = true
		p, err := DinhDang(TaiLieu, d.TieuDe, d.NoiDung)
		if err != nil {
			return nil, err
		}
		var line dongVao
		line.Key = d.Khoa
		line.Request.Content.Parts = []struct {
			Text string `json:"text"`
		}{{Text: p}}
		line.Request.OutputDimensionality = Dims
		raw, err := json.Marshal(line)
		if err != nil {
			return nil, err
		}
		b.Write(raw)
		b.WriteByte('\n')
	}
	return b.Bytes(), nil
}

// Gui uploads the documents and starts one batch job; it returns the job's
// name. ten is the job's display name.
func (l *Lo) Gui(ctx context.Context, ten string, docs []TaiLieuLo) (string, error) {
	body, err := FileVao(docs)
	if err != nil {
		return "", err
	}
	f, err := l.client.Files.Upload(ctx, bytes.NewReader(body), &genai.UploadFileConfig{
		MIMEType: "application/jsonl", DisplayName: ten})
	if err != nil {
		return "", fmt.Errorf("nhung: batch upload: %w", err)
	}
	model := Model
	job, err := l.client.Batches.CreateEmbeddings(ctx, &model, &genai.EmbeddingsBatchJobSource{FileName: f.Name},
		&genai.CreateEmbeddingsBatchJobConfig{DisplayName: ten})
	if err != nil {
		return "", fmt.Errorf("nhung: batch create: %w", err)
	}
	return job.Name, nil
}

// Xem reads a job; when it is done, its vectors.
func (l *Lo) Xem(ctx context.Context, job string) (KetQuaLo, error) {
	j, err := l.client.Batches.Get(ctx, job, nil)
	if err != nil {
		return KetQuaLo{}, err
	}
	switch j.State {
	case genai.JobStateSucceeded:
	case genai.JobStateFailed, genai.JobStateCancelled, genai.JobStateExpired:
		kq := KetQuaLo{TrangThai: LoHong, Loi: string(j.State)}
		if j.Error != nil {
			kq.Loi += ": " + j.Error.Message
		}
		return kq, nil
	default:
		return KetQuaLo{TrangThai: LoDangChay}, nil
	}
	if j.Dest == nil || j.Dest.FileName == "" {
		return KetQuaLo{}, errors.New("nhung: a finished batch named no result file")
	}
	raw, err := l.client.Files.Download(ctx, genai.NewDownloadURIFromFile(&genai.File{Name: j.Dest.FileName}), nil)
	if err != nil {
		return KetQuaLo{}, fmt.Errorf("nhung: batch download: %w", err)
	}
	return DocKetQuaLo(raw)
}

// DocKetQuaLo reads the JSONL result file: one line per request, the key the
// input carried, the embedding or an error.
func DocKetQuaLo(raw []byte) (KetQuaLo, error) {
	kq := KetQuaLo{TrangThai: LoXong, Vecs: map[string][]float32{}}
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var d struct {
			Key      string `json:"key"`
			Response *struct {
				Embedding *struct {
					Values []float32 `json:"values"`
				} `json:"embedding"`
			} `json:"response"`
		}
		if err := json.Unmarshal([]byte(line), &d); err != nil || d.Key == "" {
			return KetQuaLo{}, fmt.Errorf("nhung: unreadable batch result line: %v", err)
		}
		if d.Response == nil || d.Response.Embedding == nil || len(d.Response.Embedding.Values) != Dims {
			kq.LoiDong++
			continue
		}
		v, err := ChuanHoa(d.Response.Embedding.Values)
		if err != nil {
			kq.LoiDong++
			continue
		}
		kq.Vecs[d.Key] = v
	}
	return kq, sc.Err()
}
