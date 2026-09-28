package hieu

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/agent"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/domain/thoigian"
)

// The router call's settings (research intent-routing §5.4, agentic-rag-tools
// §4.4). Temperature is left to the provider: Gemini 3.5 ignores it, and the
// schema plus the minimal thinking level is what keeps the output steady.
const (
	// MaxTokensRa bounds the router's output; the longest valid object
	// (three queries, three options, every slot) is well under it.
	MaxTokensRa = 1024
	// SoNgayLich is how many days the calendar in the «now» block lists,
	// today included: enough for «tuần sau» from any weekday.
	SoNgayLich = 14
	// DuTru is how many model calls must stay unspent after a repair re-ask:
	// the answer still needs one. A repair that would leave fewer is not
	// made; the turn falls back instead.
	DuTru = 1
	// maxChuSua bounds how much of a refused output goes back to the model
	// in the repair request.
	maxChuSua = 4000
)

var (
	// ErrKhongHieu: the router's output was refused and the one repair
	// re-ask was refused too, or there was no budget left to make it. The
	// caller answers with the fixed sentence asking the person to rephrase
	// (MaLoi).
	ErrKhongHieu = errors.New("hieu: no usable router output")
	// ErrBiChan: the provider withheld the router's output for safety.
	ErrBiChan = errors.New("hieu: the provider withheld the router output")
	// ErrVao: the turn gave the router nothing to route (no message, no
	// «now», no counter).
	ErrVao = errors.New("hieu: incomplete router input")
)

// Router is the production Hieu: one flash-lite call with the response
// schema of LuocDo, and at most one repair re-ask.
type Router struct {
	viDu chonViDu
}

// chonViDu picks the worked examples of one turn (KhoViDu, KhoViDuLuoi).
type chonViDu interface {
	Chon(ctx context.Context, v Vao) ([]ViDu, error)
}

// TuyChon configures a Router.
type TuyChon func(*Router)

// WithViDu gives the router a bank of worked examples, chosen per turn by
// embedding similarity (never by words).
func WithViDu(k *KhoViDu) TuyChon { return func(r *Router) { r.viDu = k } }

// WithViDuLuoi gives the router a bank embedded on first use (KhoViDuLuoi):
// a process starts without a call to the embedding provider.
func WithViDuLuoi(k *KhoViDuLuoi) TuyChon { return func(r *Router) { r.viDu = k } }

// Moi builds a router.
func Moi(opts ...TuyChon) *Router {
	r := &Router{}
	for _, o := range opts {
		o(r)
	}
	return r
}

// DauVet is what one routing did, as counts and flags only: no text.
type DauVet struct {
	// SoGoi is the model calls this routing made (1, or 2 with a repair).
	SoGoi int
	// DaSua: the first output was refused and a repair was asked for.
	DaSua bool
	// SoViDu is how many worked examples went into the request.
	SoViDu int
	// ViDuHong: the example bank could not embed the message; the request
	// went without examples.
	ViDuHong bool
}

// Hieu implements the interface.
func (r *Router) Hieu(ctx context.Context, v Vao, dem *llm.Dem) (KetQua, error) {
	kq, _, err := r.HieuVet(ctx, v, dem)
	return kq, err
}

// HieuVet routes v and says what it did.
func (r *Router) HieuVet(ctx context.Context, v Vao, dem *llm.Dem) (KetQua, DauVet, error) {
	var vet DauVet
	if dem == nil {
		return KetQua{}, vet, fmt.Errorf("%w: no call counter", ErrVao)
	}
	var viDu []ViDu
	if r.viDu != nil {
		var err error
		if viDu, err = r.viDu.Chon(ctx, v); err != nil {
			// Examples help; a routing without them is still a routing.
			vet.ViDuHong, viDu = true, nil
		}
	}
	vet.SoViDu = len(viDu)
	req, err := YeuCau(v, viDu)
	if err != nil {
		return KetQua{}, vet, err
	}
	raw, err := goi(ctx, dem, req, &vet)
	if err != nil {
		return KetQua{}, vet, err
	}
	kq, err := Doc([]byte(raw), v)
	if err == nil {
		return kq, vet, nil
	}
	if !errors.Is(err, ErrCauTruc) {
		return KetQua{}, vet, err
	}
	// One repair, only if the answer would still have its call after it.
	if dem.ConLai() < 1+DuTru {
		return KetQua{}, vet, fmt.Errorf("%w: %w", ErrKhongHieu, err)
	}
	vet.DaSua = true
	raw2, err2 := goi(ctx, dem, SuaLai(req, raw, err), &vet)
	if err2 != nil {
		return KetQua{}, vet, err2
	}
	kq, err = Doc([]byte(raw2), v)
	if err != nil {
		return KetQua{}, vet, fmt.Errorf("%w: %w", ErrKhongHieu, err)
	}
	return kq, vet, nil
}

// goi makes one call through dem and returns the answer's text.
func goi(ctx context.Context, dem *llm.Dem, req *model.LLMRequest, vet *DauVet) (string, error) {
	vet.SoGoi++
	var b strings.Builder
	for resp, err := range dem.GenerateContent(ctx, req, false) {
		if err != nil {
			return "", err
		}
		if resp == nil {
			continue
		}
		if llm.BiChanAnToan(resp.FinishReason) {
			return "", ErrBiChan
		}
		if resp.Content == nil {
			continue
		}
		for _, p := range resp.Content.Parts {
			if p != nil && !p.Thought {
				b.WriteString(p.Text)
			}
		}
	}
	return b.String(), nil
}

// YeuCau is the router request for v: the system instruction of v's bot,
// one user turn of data blocks, and the response schema of LuocDo(v). The
// same v and examples give the same request, byte for byte.
func YeuCau(v Vao, viDu []ViDu) (*model.LLMRequest, error) {
	instr, err := LoiNhacCua(v)
	if err != nil {
		return nil, err
	}
	schema, err := LuocDo(v)
	if err != nil {
		return nil, err
	}
	body, err := NoiDung(v, viDu)
	if err != nil {
		return nil, err
	}
	return &model.LLMRequest{
		Model:    llm.Model,
		Contents: []*genai.Content{{Role: "user", Parts: []*genai.Part{{Text: body}}}},
		Config: &genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: instr}}},
			ResponseMIMEType:  "application/json",
			ResponseSchema:    schema,
			MaxOutputTokens:   MaxTokensRa,
			ThinkingConfig:    llm.CauHinhNghi(llm.BuocRouter),
			SafetySettings:    agent.AnToan(),
		},
	}, nil
}

// SuaLai is the repair request: the first request, the refused output as
// the model's turn, and why it was refused as data.
func SuaLai(req *model.LLMRequest, raw string, loi error) *model.LLMRequest {
	if utf8.RuneCountInString(raw) > maxChuSua {
		raw = string([]rune(raw)[:maxChuSua])
	}
	sua := *req
	sua.Contents = append(append([]*genai.Content(nil), req.Contents...),
		&genai.Content{Role: "model", Parts: []*genai.Part{{Text: raw}}},
		&genai.Content{Role: "user", Parts: []*genai.Part{{Text: prompts.BocDuLieuDanhDau(prompts.LoiCauTruc, loi.Error()) +
			"\n\nThe JSON above was refused for the reason in the loi_cau_truc block. Write the whole JSON object again, following the schema and the rules exactly."}}},
	)
	return &sua
}

// tenVai is how a short-term turn's speaker reads in the ngan_han block.
var tenVai = map[trinho.VaiLuot]string{trinho.Toi: "nguoi_hoi", trinho.TroLy: "tro_ly", trinho.Ban: "thanh_vien"}

// MaxLuotCua is how many short-term turns bot's router reads: Nếp's panel
// session (trinho.MaxLuotNganHan), or every turn the caller shared with the
// group assistant (trinho.MaxLuotNhom), since the card says how many it read.
func MaxLuotCua(bot obs.Bot) int {
	if bot == obs.BotNhom {
		return trinho.MaxLuotNhom
	}
	return trinho.MaxLuotNganHan
}

// NoiDung is the router's user turn for v, laid out for Gemini's implicit
// cache (SOTA gap #5): what changes least comes first -- the turn's closed
// lists (the same for every turn of a deployment), then the worked
// examples, the slip and the history -- and what changes on every call
// last: the server's clock and calendar («bây giờ»), then the message.
// Every text that came from outside is datamarked.
func NoiDung(v Vao, viDu []ViDu) (string, error) {
	if strings.TrimSpace(v.Cau) == "" || v.Luc.IsZero() {
		return "", ErrVao
	}
	var blocks []string
	if len(v.DanhSachDiemDen) > 0 {
		var lines []string
		for _, d := range v.DanhSachDiemDen {
			lines = append(lines, d.ID+" | "+d.Ten)
		}
		blocks = append(blocks, prompts.BocDuLieuDanhDau(prompts.DiemDen, strings.Join(lines, "\n")))
	}
	if v.Bot == obs.BotNhom && len(v.DanhSachThanhVien) > 0 {
		var lines []string
		for _, m := range v.DanhSachThanhVien {
			lines = append(lines, m.ID+" | "+m.Ten)
		}
		blocks = append(blocks, prompts.BocDuLieuDanhDau(prompts.ThanhVien, strings.Join(lines, "\n")))
	}
	if len(viDu) > 0 {
		var parts []string
		for _, d := range viDu {
			parts = append(parts, "cau: "+d.Cau+"\njson: "+string(d.Ra))
		}
		blocks = append(blocks, prompts.BocDuLieu(prompts.ViDu, strings.Join(parts, "\n\n")))
	}
	if v.Bot == obs.BotNep && len(v.PhieuNep) > 0 {
		blocks = append(blocks, prompts.BocDuLieuDanhDau(prompts.PhieuManHinh, string(v.PhieuNep)))
	}
	luot := v.NganHan
	if n := MaxLuotCua(v.Bot); len(luot) > n {
		luot = luot[len(luot)-n:]
	}
	if len(luot) > 0 {
		var lines []string
		for _, l := range luot {
			vai, ok := tenVai[l.Vai]
			if !ok || (l.Vai == trinho.Ban && v.Bot != obs.BotNhom) {
				return "", fmt.Errorf("%w: short-term turn by %q", ErrVao, l.Vai)
			}
			lines = append(lines, vai+": "+strings.Join(strings.Fields(l.Chu), " "))
			if len(l.BangChungIDs) > 0 {
				lines = append(lines, "bang_chung: "+strings.Join(l.BangChungIDs, ", "))
			}
		}
		blocks = append(blocks, prompts.BocDuLieuDanhDau(prompts.NganHan, strings.Join(lines, "\n")))
	}
	blocks = append(blocks, prompts.BocDuLieu(prompts.MayChu, DongLich(v)))
	blocks = append(blocks, prompts.BocDuLieuDanhDau(prompts.CauHoi, v.Cau))
	return strings.Join(blocks, "\n\n"), nil
}

// DongLich is the «now» line and a calendar of the next SoNgayLich days on
// Vietnam's clock, computed from the turn's instant: plain arithmetic the
// model reads to write ngay_iso. Nothing here reads the message.
func DongLich(v Vao) string {
	here := thoigian.Now(v.Luc)
	homNay := thoigian.Ngay{Nam: here.Year(), Thang: int(here.Month()), Ngay: here.Day()}
	lines := []string{thoigian.DongBayGio(v.Luc), "Lịch:"}
	for i := 0; i < SoNgayLich; i++ {
		d := homNay.Cong(i)
		line := fmt.Sprintf("- %04d-%02d-%02d: %s", d.Nam, d.Thang, d.Ngay, d)
		switch i {
		case 0:
			line += " (hôm nay)"
		case 1:
			line += " (ngày mai)"
		case 2:
			line += " (ngày mốt)"
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
