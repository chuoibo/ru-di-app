// Package chiabill is the group assistant's one structured reading of the
// shared messages for a bill split (design 03 §4.3, slice 9): ONE flash-lite
// call replaces the brain's eight sequential chat-expense calls. The MODEL
// decides which messages state an expense, what was paid for and how much;
// Go checks the structure only:
//
//   - every item names a message of the turn's closed list by its alias (or
//     the request itself), and the payer is the author the server confirmed
//     for that message, never a name the model wrote;
//   - the amount is a JSON integer of đồng within the ledger's bounds: a
//     fraction or an exponent is refused, never rounded (money law 1);
//   - the amount is SUPPORTED by the message it names, structurally: the
//     model quotes the words that state it (so_tien_goc), the quote must be
//     a substring of that message, and the integer the quote's digits spell
//     must be the item's amount up to a power of ten (850k, 850.000đ and
//     850000 all spell 850 → 850000; 1tr2 spells 12 → 1200000). An amount
//     the message does not support (a number the model invented, or read off
//     another message) refuses the whole reading with ErrSoTienKhongKhop,
//     and no draft is built: nothing unverified leaves the server;
//   - the title is kept only when it is a run of whole words of that same
//     message, found by exact identity and taken from the message (the way
//     tools.kiemGhiNho takes a fact): a title the model invented or
//     paraphrased never reaches the room.
//
// Then the LLM verifier (Kiem, kiem.go), in a fresh context, reads each
// item against the one message it names and must find that the message's
// writer paid that amount for that thing; any item it does not support, or
// an output it cannot read, withholds the whole draft (fail closed).
//
// Nothing here splits anything: the engine's chiaBillParts builds the draft
// and its equal-split preview with the domain allocator, and people confirm
// it in the app.
package chiabill

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/cautruc"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/domain/allocator"
)

//go:embed chia_bill.txt
var he string

// He is the step's static system instruction (the eval reads a request's
// stage off it).
func He() string { return strings.TrimSpace(he) }

const (
	// BiDanhLoiNho is the alias of the request itself.
	BiDanhLoiNho = "loi_nho"
	// MaxKhoan bounds the items (companion.MaxReplyKhoan).
	MaxKhoan = 8
	// MaxChuTieuDe bounds a title in runes.
	MaxChuTieuDe = 60
	// MaxChuSoTienGoc bounds the quoted amount in runes.
	MaxChuSoTienGoc = 32
	// maxTokensRa bounds the output: eight short objects.
	maxTokensRa = 1024
	// KhoiTin names the shared messages' block. Ours, never data.
	KhoiTin prompts.Nguon = "tin_nhan"
)

// Tin is one shared message offered to the model.
type Tin struct {
	// BiDanh is t1, t2, …: the only name the model sees for it.
	BiDanh string
	// Ten is the writer's label as the room knows them (the roster's).
	Ten string
	// Chu is the message's text, cleaned structurally.
	Chu string
}

// Vao is one reading.
type Vao struct {
	Tin []Tin
	// LoiNho is the request of the member who asked, cleaned structurally.
	LoiNho string
}

// BiDanhTin is the alias of message i (0-based).
func BiDanhTin(i int) string { return "t" + strconv.Itoa(i+1) }

// Khoan is one checked item.
type Khoan struct {
	// Tin is the alias of the message that states it, or BiDanhLoiNho.
	Tin string
	// TieuDe is a run of whole words of that message; "" when the model's
	// title was not one (TieuDeBo).
	TieuDe string
	// TieuDeBo marks a title refused because it was not copied from the
	// message.
	TieuDeBo  bool
	SoTienVND int64
	// SoTienGoc is the message's own words for the amount, as the message
	// has them (a checked substring of it).
	SoTienGoc string
}

var (
	// ErrCauTruc wraps every structural refusal of Doc.
	ErrCauTruc = errors.New("chiabill: output refused")
	// ErrVao: nothing to read.
	ErrVao = errors.New("chiabill: no message and no request")
	// ErrSoTienKhongKhop: an item's amount is not supported by the message it
	// names (its quote is not in the message, or does not spell the amount).
	// The engine asks back instead of drafting; it is not a malformed
	// output, so it does not wrap ErrCauTruc.
	ErrSoTienKhongKhop = errors.New("chiabill: an amount the message does not state")
)

func loi(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrCauTruc}, a...)...)
}

func (v Vao) biDanh() []string {
	out := make([]string, 0, len(v.Tin)+1)
	for _, t := range v.Tin {
		out = append(out, t.BiDanh)
	}
	return append(out, BiDanhLoiNho)
}

// LuocDo is the response schema: the closed alias list of this reading, the
// title's bound and the amount's integer range.
func LuocDo(v Vao) *genai.Schema {
	i64 := func(n int64) *int64 { return &n }
	f64 := func(n float64) *float64 { return &n }
	khoan := &genai.Schema{Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"tin":         {Type: genai.TypeString, Enum: v.biDanh()},
			"tieu_de":     {Type: genai.TypeString, MaxLength: i64(MaxChuTieuDe), Description: "copied exactly from the message"},
			"so_tien_goc": {Type: genai.TypeString, MaxLength: i64(MaxChuSoTienGoc), Description: "the words of the message that state the amount, copied exactly"},
			"so_tien_vnd": {Type: genai.TypeInteger, Minimum: f64(1), Maximum: f64(float64(allocator.MaxAmountVND)), Description: "whole đồng"},
		},
		PropertyOrdering: []string{"tin", "tieu_de", "so_tien_goc", "so_tien_vnd"},
		Required:         []string{"tin", "tieu_de", "so_tien_goc", "so_tien_vnd"},
	}
	return &genai.Schema{Type: genai.TypeObject,
		Properties:       map[string]*genai.Schema{"khoan": {Type: genai.TypeArray, Items: khoan, MaxItems: i64(MaxKhoan)}},
		PropertyOrdering: []string{"khoan"},
		Required:         []string{"khoan"},
	}
}

// NoiDung is the user turn: the shared messages under their aliases, then
// the request, every text datamarked.
func NoiDung(v Vao) (string, error) {
	if len(v.Tin) == 0 && strings.TrimSpace(v.LoiNho) == "" {
		return "", ErrVao
	}
	var blocks []string
	if len(v.Tin) > 0 {
		lines := make([]string, 0, len(v.Tin))
		for _, t := range v.Tin {
			lines = append(lines, t.BiDanh+" | "+strings.Join(strings.Fields(t.Ten), " ")+": "+strings.Join(strings.Fields(t.Chu), " "))
		}
		blocks = append(blocks, prompts.BocDuLieuDanhDau(KhoiTin, strings.Join(lines, "\n")))
	}
	blocks = append(blocks, prompts.BocDuLieuDanhDau(prompts.CauHoi, v.LoiNho))
	return strings.Join(blocks, "\n\n"), nil
}

// YeuCau is the reading's request, byte-stable for the same input.
func YeuCau(v Vao) (*model.LLMRequest, error) {
	body, err := NoiDung(v)
	if err != nil {
		return nil, err
	}
	return cautruc.YeuCau(llm.BuocChiaBill, He(), body, LuocDo(v), maxTokensRa), nil
}

// Goi makes the one call through dem and reads its answer strictly. A
// provider or budget error is returned as it came; a refused output wraps
// ErrCauTruc.
func Goi(ctx context.Context, dem *llm.Dem, v Vao) ([]Khoan, error) {
	req, err := YeuCau(v)
	if err != nil {
		return nil, err
	}
	raw, err := cautruc.Goi(ctx, dem, req)
	if err != nil {
		return nil, err
	}
	return Doc([]byte(raw), v)
}

type khoanTho struct {
	Tin       string      `json:"tin"`
	TieuDe    string      `json:"tieu_de"`
	SoTienGoc string      `json:"so_tien_goc"`
	SoTienVND json.Number `json:"so_tien_vnd"`
}

// truongKhoan are an item's fields, every one required.
var truongKhoan = []string{"tin", "tieu_de", "so_tien_goc", "so_tien_vnd"}

// chuCua is the text of the message alias bi names in this reading (the
// request for BiDanhLoiNho); false for an alias the reading did not offer.
func (v Vao) chuCua(bi string) (string, bool) {
	if bi == BiDanhLoiNho {
		return v.LoiNho, true
	}
	for _, t := range v.Tin {
		if t.BiDanh == bi {
			return t.Chu, true
		}
	}
	return "", false
}

func giaiMaChat(raw []byte, out any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	if err := dec.Decode(out); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("trailing data")
	}
	return nil
}

// Doc reads the model's JSON strictly: unknown fields, a missing field, an
// alias outside the reading, more than MaxKhoan items, a title or a quote
// too long, or an amount that is not a JSON integer in
// [1, allocator.MaxAmountVND] refuse the WHOLE output (ErrCauTruc); an
// amount its message does not support (soTienCoTrongTin) refuses it with
// ErrSoTienKhongKhop. A title that is not a run of words of its message is
// dropped from that item, never repaired.
func Doc(raw []byte, v Vao) ([]Khoan, error) {
	var tho struct {
		Khoan *[]json.RawMessage `json:"khoan"`
	}
	if err := giaiMaChat(raw, &tho); err != nil {
		return nil, loi("json: %v", err)
	}
	if tho.Khoan == nil {
		return nil, loi("khoan is missing")
	}
	if len(*tho.Khoan) > MaxKhoan {
		return nil, loi("%d items", len(*tho.Khoan))
	}
	out := make([]Khoan, 0, len(*tho.Khoan))
	for _, item := range *tho.Khoan {
		// Every field present: read by key first, so a missing title is told
		// apart from an empty one without reading the title.
		var co map[string]json.RawMessage
		if err := json.Unmarshal(item, &co); err != nil {
			return nil, loi("an item is not an object")
		}
		for _, f := range truongKhoan {
			if _, ok := co[f]; !ok {
				return nil, loi("an item has no %s", f)
			}
		}
		// The amount is a JSON number, never a string that holds one.
		if so := bytes.TrimSpace(co["so_tien_vnd"]); len(so) == 0 || so[0] < '0' || so[0] > '9' {
			return nil, loi("so_tien_vnd is not a JSON number")
		}
		var k khoanTho
		if err := giaiMaChat(item, &k); err != nil {
			return nil, loi("item: %v", err)
		}
		nguon, ok := v.chuCua(k.Tin)
		if !ok {
			return nil, loi("tin %q is not an alias of this reading", k.Tin)
		}
		n, err := soNguyenDong(k.SoTienVND)
		if err != nil {
			return nil, err
		}
		if !utf8.ValidString(k.TieuDe) || utf8.RuneCountInString(k.TieuDe) > MaxChuTieuDe {
			return nil, loi("a title is too long")
		}
		if !utf8.ValidString(k.SoTienGoc) || utf8.RuneCountInString(k.SoTienGoc) > MaxChuSoTienGoc {
			return nil, loi("a quoted amount is too long")
		}
		goc, ok := soTienCoTrongTin(nguon, prompts.BoDanhDau(k.SoTienGoc), n)
		if !ok {
			return nil, fmt.Errorf("%w: tin %s", ErrSoTienKhongKhop, k.Tin)
		}
		kq := Khoan{Tin: k.Tin, SoTienVND: n, SoTienGoc: goc}
		if doan, ok := doanCuaTin(nguon, prompts.BoDanhDau(k.TieuDe)); ok {
			kq.TieuDe = doan
		} else {
			kq.TieuDeBo = true
		}
		out = append(out, kq)
	}
	return out, nil
}

// soNguyenDong is a JSON number as whole đồng: digits only (no sign, point
// or exponent), within [1, allocator.MaxAmountVND]. Nothing is rounded.
func soNguyenDong(n json.Number) (int64, error) {
	s := n.String()
	if s == "" || strings.ContainsAny(s, ".eE+-") {
		return 0, loi("so_tien_vnd %q is not a whole number of đồng", s)
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v < 1 || v > int64(allocator.MaxAmountVND) {
		return 0, loi("so_tien_vnd %q is outside [1, %d]", s, int64(allocator.MaxAmountVND))
	}
	return v, nil
}

// soTienCoTrongTin is the structural support of an amount by its message:
// the quote, its runs of white space as one space, is a substring of the
// message read the same way, and the ASCII digits of the quote, read in
// order as one integer d, spell the amount: n = d·10^k for some k ≥ 0.
// Nothing else is read: no unit word, no language. A quote whose digits do
// not spell the amount («2 triệu rưỡi» for 2500000, where the words carry
// the half) is not supported, and the reading is refused rather than
// guessed at. It returns the quote as the message has it.
func soTienCoTrongTin(tin, goc string, n int64) (string, bool) {
	g := strings.Join(strings.Fields(goc), " ")
	if g == "" || !chuoiConKhongCatSo(strings.Join(strings.Fields(tin), " "), g) {
		return "", false
	}
	var d int64
	coSo := false
	for i := 0; i < len(g); i++ {
		c := g[i]
		if c < '0' || c > '9' {
			continue
		}
		if d > (int64(allocator.MaxAmountVND))/10 {
			return "", false
		}
		d = d*10 + int64(c-'0')
		coSo = true
	}
	if !coSo || d < 1 {
		return "", false
	}
	for x := d; x <= n; x *= 10 {
		if x == n {
			return g, true
		}
		if x > int64(allocator.MaxAmountVND)/10 {
			break
		}
	}
	return "", false
}

// chuoiConKhongCatSo reports whether g occurs in tin at a place where it
// cuts no number: the byte before it and the byte after it are not ASCII
// digits, so «850k» is not read out of «2850k».
func chuoiConKhongCatSo(tin, g string) bool {
	laSo := func(c byte) bool { return c >= '0' && c <= '9' }
	for i := 0; i+len(g) <= len(tin); {
		j := strings.Index(tin[i:], g)
		if j < 0 {
			return false
		}
		a, b := i+j, i+j+len(g)
		if (a == 0 || !laSo(tin[a-1])) && (b == len(tin) || !laSo(tin[b])) {
			return true
		}
		i = a + 1
	}
	return false
}

// doanCuaTin finds doan in tin as a run of whole words (runs of white space
// are one space in both) and returns that run as tin has it. Exact identity
// of words, compared one by one; no word is read.
func doanCuaTin(tin, doan string) (string, bool) {
	tw, dw := strings.Fields(tin), strings.Fields(doan)
	if len(dw) == 0 {
		return "", false
	}
	for i := 0; i+len(dw) <= len(tw); i++ {
		khop := true
		for j := range dw {
			khop = khop && tw[i+j] == dw[j]
		}
		if khop {
			return strings.Join(tw[i:i+len(dw)], " "), true
		}
	}
	return "", false
}
