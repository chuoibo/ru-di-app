package nepnho

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// MucNho is one memory the sidecar holds: its mem0 id, its words and the
// kind recorded when it was written. Score is the sidecar's similarity for a
// search and zero for a listing; it orders, it never decides.
type MucNho struct {
	ID    string
	Text  string
	Score float64
	Loai  string
}

// KhoNho is the port to the memory sidecar (services/ai-infer, mem0ai 2.2.1
// embedded as a library behind our own FastAPI surface). Every call names
// the owner; the sidecar filters by its owner field and checks the owner of
// every id, and Go filters again by the receipts in Postgres, so a sidecar
// that ever returned another person's row would still show nothing.
//
// The sidecar is the only writer of the memory collection in Milvus: Go
// never deletes there itself. A deletion is believed only when the sidecar
// answered remaining 0 AND Go's own listing, at the collection's Strong
// consistency, no longer holds what was deleted.
type KhoNho interface {
	// Them sends the person's own sentence for extraction and returns what
	// was stored (0..n memories; the extraction model itself refuses
	// sentences about money, other people or sensitive traits) and how
	// many candidates it refused.
	Them(ctx context.Context, owner, cau string) ([]MucNho, int, error)
	// Tim returns at most k memories of owner similar to query, best first.
	Tim(ctx context.Context, owner, query string, k int) ([]MucNho, error)
	// LietKe returns every memory of owner.
	LietKe(ctx context.Context, owner string) ([]MucNho, error)
	// Xoa deletes one memory of owner; a memory already gone (or never the
	// owner's: the sidecar answers both with 404) counts 0.
	Xoa(ctx context.Context, owner, id string) (int, error)
	// XoaHet deletes every memory of owner (delete_all, repeated inside the
	// sidecar until its Strong count reads zero).
	XoaHet(ctx context.Context, owner string) (deleted int, err error)
	// XoaSach is account deletion (purge_user): every row of owner deleted,
	// flushed and compacted, then counted.
	XoaSach(ctx context.Context, owner string) (deleted int, err error)
}

// ErrConHang: the sidecar deleted but still counts rows (its 500 with
// "remaining"), or Go's own listing still holds what was deleted. Never a
// receipt: the saga retries.
var ErrConHang = errors.New("nepnho: rows remain after a delete")

// ErrDichVuVang: the sidecar did not answer, answered 5xx, or is not
// configured. The caller degrades (no memory in this turn) or retries later
// (a deletion); it never pretends the call succeeded.
var ErrDichVuVang = errors.New("nepnho: memory sidecar unavailable")

// ErrDichVuSai: the sidecar answered something outside the contract.
var ErrDichVuSai = errors.New("nepnho: memory sidecar answered outside the contract")

// Per-call bounds. Search sits on the hot path of a Nếp turn (design 05 §8:
// recall_memory 1 s); a purge compacts Milvus and may take a while.
const (
	HanTim     = time.Second
	HanThem    = 5 * time.Second
	HanLietKe  = 3 * time.Second
	HanXoa     = 10 * time.Second
	HanXoaSach = 50 * time.Second
	// maxTraLoi bounds a sidecar answer read into memory.
	maxTraLoi = 1 << 20
	// LietKeToiDa is the most memories a listing asks for; the ledger holds
	// at most MaxSuThat live facts, so a full listing never truncates.
	LietKeToiDa = 1000
)

// KhachHTTP calls the sidecar over HTTP with its internal bearer token. No
// retry inside a call: a turn degrades, a deletion is retried by the pass.
type KhachHTTP struct {
	Goc    string // base URL, e.g. http://127.0.0.1:8090
	Token  string
	Nguong float64 // search similarity floor sent to the sidecar
	HTTP   *http.Client
	// ThemHan overrides HanThem (0 keeps it). The local Milvus bring-up
	// showed inserts stalling about 5 s at Strong consistency now and then;
	// a host that measures the same sets this rather than losing writes.
	ThemHan time.Duration
}

var _ KhoNho = (*KhachHTTP)(nil)

// MoiKhachHTTP validates the configuration.
func MoiKhachHTTP(goc, token string, nguong float64) (*KhachHTTP, error) {
	if !strings.HasPrefix(goc, "http://") && !strings.HasPrefix(goc, "https://") {
		return nil, errors.New("nepnho: sidecar URL must be http(s)")
	}
	if len(token) < 32 {
		return nil, errors.New("nepnho: sidecar token must be at least 32 characters")
	}
	if nguong < 0 || nguong > 1 {
		return nil, errors.New("nepnho: search threshold must be within [0,1]")
	}
	return &KhachHTTP{Goc: strings.TrimRight(goc, "/"), Token: token, Nguong: nguong,
		HTTP: &http.Client{Timeout: HanXoaSach + 5*time.Second}}, nil
}

type mucJSON struct {
	ID    string   `json:"id"`
	Text  string   `json:"text"`
	Score *float64 `json:"score"`
	Loai  *string  `json:"loai"`
}

func (m mucJSON) muc() (MucNho, error) {
	if m.ID == "" {
		return MucNho{}, fmt.Errorf("%w: item without id", ErrDichVuSai)
	}
	out := MucNho{ID: m.ID, Text: m.Text}
	if m.Score != nil {
		out.Score = *m.Score
	}
	if m.Loai != nil {
		out.Loai = *m.Loai
	}
	return out, nil
}

func (k *KhachHTTP) goi(ctx context.Context, han time.Duration, path string, in, out any) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, han)
	defer cancel()
	body, err := json.Marshal(in)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, k.Goc+path, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+k.Token)
	resp, err := k.HTTP.Do(req)
	if err != nil {
		// The error names the URL and the transport failure, never a body.
		return 0, fmt.Errorf("%w: %s", ErrDichVuVang, path)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxTraLoi+1))
	if err != nil {
		return resp.StatusCode, fmt.Errorf("%w: %s", ErrDichVuVang, path)
	}
	if len(raw) > maxTraLoi {
		return resp.StatusCode, fmt.Errorf("%w: %s answer too large", ErrDichVuSai, path)
	}
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return resp.StatusCode, nil
	case resp.StatusCode == http.StatusInternalServerError:
		// A delete that left rows is a 500 carrying "remaining" (the
		// sidecar's DeleteIncomplete): not an outage, a count above zero.
		var inc struct {
			Remaining *int `json:"remaining"`
		}
		if json.Unmarshal(raw, &inc) == nil && inc.Remaining != nil {
			return resp.StatusCode, fmt.Errorf("%w: %s left %d", ErrConHang, path, *inc.Remaining)
		}
		return resp.StatusCode, fmt.Errorf("%w: %s %d", ErrDichVuVang, path, resp.StatusCode)
	case resp.StatusCode > 500:
		return resp.StatusCode, fmt.Errorf("%w: %s %d", ErrDichVuVang, path, resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return resp.StatusCode, fmt.Errorf("%w: %s %d", ErrDichVuSai, path, resp.StatusCode)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return resp.StatusCode, fmt.Errorf("%w: %s undecodable", ErrDichVuSai, path)
	}
	return resp.StatusCode, nil
}

type tinNhan struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type themVao struct {
	UserID   string    `json:"user_id"`
	Messages []tinNhan `json:"messages"`
}

// Them implements KhoNho.
func (k *KhachHTTP) Them(ctx context.Context, owner, cau string) ([]MucNho, int, error) {
	var out struct {
		Added   []mucJSON `json:"added"`
		Refused *int      `json:"refused"`
	}
	han := HanThem
	if k.ThemHan > 0 {
		han = k.ThemHan
	}
	status, err := k.goi(ctx, han, "/v1/memory/add", themVao{UserID: owner,
		Messages: []tinNhan{{Role: "user", Content: cau}}}, &out)
	if err != nil {
		return nil, 0, err
	}
	if status == http.StatusNotFound || out.Refused == nil || *out.Refused < 0 {
		return nil, 0, fmt.Errorf("%w: add answer incomplete", ErrDichVuSai)
	}
	ms, err := cacMuc(out.Added, MaxThemMotLan)
	return ms, *out.Refused, err
}

// MaxThemMotLan bounds how many memories one sentence may yield.
const MaxThemMotLan = 8

// Tim implements KhoNho.
func (k *KhachHTTP) Tim(ctx context.Context, owner, query string, n int) ([]MucNho, error) {
	var out struct {
		Items []mucJSON `json:"items"`
	}
	status, err := k.goi(ctx, HanTim, "/v1/memory/search", map[string]any{"user_id": owner, "query": query, "top_k": n, "threshold": k.Nguong}, &out)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, fmt.Errorf("%w: search 404", ErrDichVuSai)
	}
	return cacMuc(out.Items, n)
}

// LietKe implements KhoNho.
func (k *KhachHTTP) LietKe(ctx context.Context, owner string) ([]MucNho, error) {
	var out struct {
		Items []mucJSON `json:"items"`
		Count int       `json:"count"`
	}
	status, err := k.goi(ctx, HanLietKe, "/v1/memory/list", map[string]any{"user_id": owner, "limit": LietKeToiDa}, &out)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound || out.Count != len(out.Items) {
		return nil, fmt.Errorf("%w: list count %d for %d items", ErrDichVuSai, out.Count, len(out.Items))
	}
	return cacMuc(out.Items, LietKeToiDa)
}

func cacMuc(items []mucJSON, n int) ([]MucNho, error) {
	if len(items) > n {
		return nil, fmt.Errorf("%w: %d items for a limit of %d", ErrDichVuSai, len(items), n)
	}
	out := make([]MucNho, 0, len(items))
	for _, it := range items {
		m, err := it.muc()
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

type daXoa struct {
	Deleted   *int `json:"deleted"`
	Remaining *int `json:"remaining"`
}

// daXoaHet reads a delete answer: remaining must be present and zero.
func daXoaHet(path string, out daXoa) (int, error) {
	if out.Deleted == nil || out.Remaining == nil || *out.Deleted < 0 {
		return 0, fmt.Errorf("%w: %s answer incomplete", ErrDichVuSai, path)
	}
	if *out.Remaining != 0 {
		return 0, fmt.Errorf("%w: %s left %d", ErrConHang, path, *out.Remaining)
	}
	return *out.Deleted, nil
}

// Xoa implements KhoNho.
func (k *KhachHTTP) Xoa(ctx context.Context, owner, id string) (int, error) {
	var out daXoa
	status, err := k.goi(ctx, HanXoa, "/v1/memory/delete", map[string]any{"user_id": owner, "memory_id": id}, &out)
	if err != nil {
		return 0, err
	}
	if status == http.StatusNotFound {
		return 0, nil
	}
	return daXoaHet("delete", out)
}

// XoaHet implements KhoNho.
func (k *KhachHTTP) XoaHet(ctx context.Context, owner string) (int, error) {
	var out daXoa
	status, err := k.goi(ctx, HanXoa, "/v1/memory/delete_all", map[string]any{"user_id": owner}, &out)
	if err != nil {
		return 0, err
	}
	if status == http.StatusNotFound {
		return 0, fmt.Errorf("%w: delete_all 404", ErrDichVuSai)
	}
	return daXoaHet("delete_all", out)
}

// XoaSach implements KhoNho.
func (k *KhachHTTP) XoaSach(ctx context.Context, owner string) (int, error) {
	var out daXoa
	status, err := k.goi(ctx, HanXoaSach, "/v1/memory/purge_user", map[string]any{"user_id": owner}, &out)
	if err != nil {
		return 0, err
	}
	if status == http.StatusNotFound {
		return 0, fmt.Errorf("%w: purge_user 404", ErrDichVuSai)
	}
	return daXoaHet("purge_user", out)
}
