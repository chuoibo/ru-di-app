package chiabill

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/cautruc"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/prompts"
)

// The draft's verifier (review of slices 9/11, finding 2.1: the owner's rule
// that nothing unverified leaves the server). The reading's amounts and
// titles are the model's; before any of them reaches the room, ONE more
// flash-lite call in a fresh context -- it sees no instruction of the
// reading, no history, only each item beside the one message it names --
// judges whether that message says its writer paid that amount for that
// thing. It returns an index and an enum per item, never free text, and it
// must judge every item exactly once: an output that skips one is refused,
// and a refused output releases nothing (fail closed, like kiemchung).
//
// It is a separate verifier from kiemchung's on purpose: kiemchung flags any
// sentence that splits money (a split draft would rightly fail it), while
// this one judges only whether the message supports each item.

//go:embed kiem_chia_bill.txt
var heKiem string

// HeKiem is the verifier's static system instruction (the eval reads a
// request's stage off it).
func HeKiem() string { return strings.TrimSpace(heKiem) }

const (
	// KhoiKhoanKiem names the verifier's one data block. Ours, never data.
	KhoiKhoanKiem prompts.Nguon = "khoan_can_kiem"
	// maxTokensKiem bounds the verifier's output: MaxKhoan short objects.
	maxTokensKiem = 512
)

// KetKiem is the verifier's verdict on one item.
type KetKiem string

const (
	CoHoTro    KetKiem = "ho_tro"
	KhongHoTro KetKiem = "khong_ho_tro"
)

// LuocDoKiem is the verifier's schema for n items: exactly n entries, each
// an index in 1..n and a verdict of the closed set.
func LuocDoKiem(n int) *genai.Schema {
	i64 := func(x int64) *int64 { return &x }
	f64 := func(x float64) *float64 { return &x }
	return &genai.Schema{Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"khoan": {Type: genai.TypeArray, MinItems: i64(int64(n)), MaxItems: i64(int64(n)), Items: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"so":  {Type: genai.TypeInteger, Minimum: f64(1), Maximum: f64(float64(n))},
					"ket": {Type: genai.TypeString, Enum: []string{string(CoHoTro), string(KhongHoTro)}},
				},
				PropertyOrdering: []string{"so", "ket"},
				Required:         []string{"so", "ket"},
			}},
		},
		PropertyOrdering: []string{"khoan"},
		Required:         []string{"khoan"},
	}
}

// DongTien writes whole đồng with dot grouping: 1250000 -> "1.250.000đ".
func DongTien(n int64) string {
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return b.String() + "đ"
}

// NoiDungKiem is the verifier's user turn: each item beside the message it
// names and that message's writer, datamarked (the message, the writer's
// label, the title and the quote came from outside; the amount is ours).
func NoiDungKiem(v Vao, ks []Khoan, tenLoiNho string) (string, error) {
	if len(ks) == 0 || len(ks) > MaxKhoan {
		return "", fmt.Errorf("%w: %d items to verify", ErrCauTruc, len(ks))
	}
	lines := make([]string, 0, len(ks))
	for i, k := range ks {
		chu, ok := v.chuCua(k.Tin)
		if !ok {
			return "", fmt.Errorf("%w: tin %q is not an alias of this reading", ErrCauTruc, k.Tin)
		}
		ten := tenLoiNho
		for _, t := range v.Tin {
			if t.BiDanh == k.Tin {
				ten = t.Ten
			}
		}
		tieuDe := k.TieuDe
		if tieuDe == "" {
			tieuDe = "-"
		}
		lines = append(lines, strconv.Itoa(i+1)+" | tin: "+strings.Join(strings.Fields(chu), " ")+
			" | nguoi_viet: "+strings.Join(strings.Fields(ten), " ")+
			" | khoan: "+tieuDe+" | so_tien: "+DongTien(k.SoTienVND)+" | chu_so_tien: "+k.SoTienGoc)
	}
	return prompts.BocDuLieuDanhDau(KhoiKhoanKiem, strings.Join(lines, "\n")) + "\n\nChấm từng khoản theo schema.", nil
}

// YeuCauKiem is the verifier's request, byte-stable for the same input.
func YeuCauKiem(v Vao, ks []Khoan, tenLoiNho string) (*model.LLMRequest, error) {
	body, err := NoiDungKiem(v, ks, tenLoiNho)
	if err != nil {
		return nil, err
	}
	return cautruc.YeuCau(llm.BuocKiemChiaBill, HeKiem(), body, LuocDoKiem(len(ks)), maxTokensKiem), nil
}

// DocKiem reads the verifier's JSON strictly: unknown fields, a missing
// field, an index outside 1..n, an item judged twice or not at all, or a
// verdict outside the closed set refuse the WHOLE output (ErrCauTruc). It
// reports whether every item was supported.
func DocKiem(raw []byte, n int) (bool, error) {
	var tho struct {
		Khoan *[]struct {
			So  *int    `json:"so"`
			Ket *string `json:"ket"`
		} `json:"khoan"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&tho); err != nil || dec.More() {
		return false, loi("verifier json: %v", err)
	}
	if tho.Khoan == nil {
		return false, loi("verifier: khoan is missing")
	}
	seen := map[int]bool{}
	dat := true
	for _, k := range *tho.Khoan {
		if k.So == nil || k.Ket == nil {
			return false, loi("verifier: a field is missing")
		}
		if *k.So < 1 || *k.So > n || seen[*k.So] {
			return false, loi("verifier: item %d", *k.So)
		}
		seen[*k.So] = true
		switch KetKiem(*k.Ket) {
		case CoHoTro:
		case KhongHoTro:
			dat = false
		default:
			return false, loi("verifier: verdict %q", *k.Ket)
		}
	}
	if len(seen) != n {
		return false, loi("verifier: %d of %d items judged", len(seen), n)
	}
	return dat, nil
}

// Kiem makes the verifier's one call through dem on the checked items ks of
// reading v (tenLoiNho is the requester's label, the writer of the
// request). It reports whether the messages support every item; a provider
// or budget error is returned as it came, a refused output wraps
// ErrCauTruc. Either way, only a true with a nil error may release a draft.
func Kiem(ctx context.Context, dem *llm.Dem, v Vao, ks []Khoan, tenLoiNho string) (bool, error) {
	req, err := YeuCauKiem(v, ks, tenLoiNho)
	if err != nil {
		return false, err
	}
	raw, err := cautruc.Goi(ctx, dem, req)
	if err != nil {
		return false, err
	}
	return DocKiem([]byte(raw), len(ks))
}
