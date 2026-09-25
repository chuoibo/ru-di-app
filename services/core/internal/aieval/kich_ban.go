package aieval

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/llm"
)

// KichBan is one stub script (design 01 §7, design 06 §1): the model's
// replies in call order, each tagged with the stage it answers. The stub
// never opens a connection; it only answers from here.
type KichBan struct {
	Ten  string        `json:"ten"`
	MoTa string        `json:"mo_ta"`
	Buoc []BuocKichBan `json:"buoc"`
}

// BuocKichBan is one scripted reply: exactly one of Chu, Goi and Loi.
type BuocKichBan struct {
	// Chang is the stage the call belongs to. S1 has one: the answer.
	Chang string `json:"chang"`
	// Chu is a text answer. MaKiemCho in it is replaced by the run's canary
	// marker, so a script can play a model that leaks it.
	Chu *string `json:"chu,omitempty"`
	// Goi is a function call.
	Goi *GoiKichBan `json:"goi,omitempty"`
	// Loi is a provider error.
	Loi *LoiKichBan `json:"loi,omitempty"`
	// Finish is the finish reason; empty means STOP.
	Finish string        `json:"finish,omitempty"`
	Usage  *UsageKichBan `json:"usage,omitempty"`
}

// GoiKichBan is a scripted function call.
type GoiKichBan struct {
	Ten   string         `json:"ten"`
	DoiSo map[string]any `json:"doi_so,omitempty"`
}

// LoiKichBan is a scripted provider error: an HTTP status from the API, or
// the provider answering with no candidate at all (a blocked prompt).
type LoiKichBan struct {
	Code int  `json:"code,omitempty"`
	Rong bool `json:"khong_ung_vien,omitempty"`
}

// UsageKichBan is a scripted token account.
type UsageKichBan struct {
	Vao   int32 `json:"vao"`
	Ra    int32 `json:"ra"`
	Cache int32 `json:"cache"`
	Nghi  int32 `json:"nghi"`
}

// MaKiemCho is where a script puts the run's canary marker.
const MaKiemCho = "{{MA_KIEM}}"

// ChangTraLoi is the answer stage, the only stage at S1.
const ChangTraLoi = "tra_loi"

var finishHopLe = map[string]bool{
	"": true, string(genai.FinishReasonStop): true, string(genai.FinishReasonMaxTokens): true,
	string(genai.FinishReasonSafety): true, string(genai.FinishReasonRecitation): true,
	string(genai.FinishReasonProhibitedContent): true, string(genai.FinishReasonBlocklist): true,
	string(genai.FinishReasonSPII): true,
}

// Kiem checks a script.
func (k KichBan) Kiem() error {
	if !dangNhom.MatchString(k.Ten) {
		return fmt.Errorf("tên kịch bản %q sai dạng", k.Ten)
	}
	if strings.TrimSpace(k.MoTa) == "" {
		return fmt.Errorf("kịch bản %s thiếu mo_ta", k.Ten)
	}
	for i, b := range k.Buoc {
		n := 0
		if b.Chu != nil {
			n++
		}
		if b.Goi != nil {
			n++
			if b.Goi.Ten == "" {
				return fmt.Errorf("kịch bản %s bước %d: lời gọi hàm thiếu tên", k.Ten, i+1)
			}
		}
		if b.Loi != nil {
			n++
			if (b.Loi.Code == 0) == !b.Loi.Rong || (b.Loi.Code != 0 && (b.Loi.Code < 400 || b.Loi.Code > 599)) {
				return fmt.Errorf("kịch bản %s bước %d: lỗi phải là một mã HTTP 4xx/5xx hoặc khong_ung_vien", k.Ten, i+1)
			}
		}
		if n != 1 {
			return fmt.Errorf("kịch bản %s bước %d: cần đúng một trong chu, goi, loi", k.Ten, i+1)
		}
		if b.Chang != ChangTraLoi {
			return fmt.Errorf("kịch bản %s bước %d: chặng %q không có ở S1", k.Ten, i+1, b.Chang)
		}
		if !finishHopLe[b.Finish] {
			return fmt.Errorf("kịch bản %s bước %d: finish %q lạ", k.Ten, i+1, b.Finish)
		}
	}
	return nil
}

// Stub builds the llm.Stub this script plays, with maKiem put in place.
func (k KichBan) Stub(maKiem string) *llm.Stub {
	buoc := make([]llm.Buoc, 0, len(k.Buoc))
	for _, b := range k.Buoc {
		var s llm.Buoc
		switch {
		case b.Chu != nil:
			s.Text = strings.ReplaceAll(*b.Chu, MaKiemCho, maKiem)
		case b.Goi != nil:
			s.Goi = &genai.FunctionCall{Name: b.Goi.Ten, Args: b.Goi.DoiSo}
		case b.Loi != nil && b.Loi.Rong:
			s.Loi = llm.ErrKhongUngVien
		case b.Loi != nil:
			s.Loi = genai.APIError{Code: b.Loi.Code}
		}
		s.Finish = genai.FinishReason(b.Finish)
		if u := b.Usage; u != nil {
			s.Usage = &genai.GenerateContentResponseUsageMetadata{PromptTokenCount: u.Vao, CandidatesTokenCount: u.Ra, CachedContentTokenCount: u.Cache, ThoughtsTokenCount: u.Nghi}
		}
		buoc = append(buoc, s)
	}
	return llm.NewStub(buoc...)
}

// DocKichBan reads every script in dir. A file's name is its script's name.
func DocKichBan(dir string) (map[string]KichBan, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("%s: không có kịch bản nào", dir)
	}
	sort.Strings(paths)
	out := map[string]KichBan{}
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var k KichBan
		if err := giaiMaChat(raw, &k); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if want := strings.TrimSuffix(filepath.Base(p), ".json"); k.Ten != want {
			return nil, fmt.Errorf("%s: tên kịch bản %q khác tên file", p, k.Ten)
		}
		if err := k.Kiem(); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		out[k.Ten] = k
	}
	if len(out) == 0 {
		return nil, errors.New("không đọc được kịch bản nào")
	}
	return out, nil
}
