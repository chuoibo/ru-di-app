// Package kiemchung verifies an answer before any byte of it is released,
// in two independent layers:
//
//  1. KiemTra, deterministic grounding by SET MEMBERSHIP and exact equality:
//     every id the answer cites is evidence of this turn (tools.SoCai), every
//     number or hour it states for an id equals that evidence's field, every
//     button label it names is a label the manual evidence carries. The
//     claims come from the answer step's STRUCTURED output (cited ids, the
//     field each quoted value belongs to, button labels), never from a regex
//     over the prose.
//  2. PhanTu, the LLM verifier: one model call that judges each claim
//     supported or not by the evidence, and flags two things only a reader
//     of meaning can: a promise of an action no tool performed («I booked»,
//     «I transferred», «I sent the reminder») and any money action. This
//     replaces the output guard's phrase rules about «I transferred money»
//     (docs/architecture/03-ai-engine-hop-dong.md, «Luật không heuristic»).
//
// What stays deterministic beside them, in aiharness/guard, is FORMAT
// validation for privacy: phone numbers, bank account numbers, emails and
// payment card numbers in the output. That is data-format checking, not
// language understanding, and it keeps running on every released byte.
package kiemchung

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/dong"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// TruongNhanNut is the evidence field that carries a manual button label.
const TruongNhanNut = "nhan_nut"

// TrichSo is one value the answer states for an evidence item: the field it
// belongs to (gia, gio, …) and the value exactly as stated.
type TrichSo struct {
	ID     string
	Truong string
	GiaTri string
}

// TuyenBo is what an answer claims, from the answer step's structured
// output.
type TuyenBo struct {
	IDs     []string
	So      []TrichSo
	NhanNut []string
}

// Lech is a stated value that differs from the evidence.
type Lech struct {
	TrichSo
	// BangChung is the evidence's value; "" when the field is absent.
	BangChung string
}

// KiemTra is the deterministic grounding result.
type KiemTra struct {
	// IDNgoai are cited ids that are not evidence of this turn.
	IDNgoai []string
	// SoLech are numbers or hours that differ from the evidence.
	SoLech []Lech
	// NhanNutKhongCo are button labels no manual evidence carries.
	NhanNutKhongCo []string
}

// Sach reports whether the answer passed grounding.
func (k KiemTra) Sach() bool {
	return len(k.IDNgoai) == 0 && len(k.SoLech) == 0 && len(k.NhanNutKhongCo) == 0
}

// Kiem checks tb against the turn's ledger. Pure set membership and exact
// string equality; it interprets nothing.
func Kiem(tb TuyenBo, sc *tools.SoCai) KiemTra {
	var k KiemTra
	for _, id := range tb.IDs {
		if !sc.Co(id) {
			k.IDNgoai = append(k.IDNgoai, id)
		}
	}
	for _, s := range tb.So {
		bc, _, ok := sc.Lay(s.ID)
		if !ok {
			k.IDNgoai = append(k.IDNgoai, s.ID)
			continue
		}
		if v, co := bc.Truong[s.Truong]; !co || v != s.GiaTri {
			k.SoLech = append(k.SoLech, Lech{TrichSo: s, BangChung: v})
		}
	}
	if len(tb.NhanNut) > 0 {
		// Exact identity of two texts, the label the answer names and one a
		// manual section carries: set membership by equality, never a map
		// keyed by evidence text.
		var nhan []string
		for _, id := range sc.IDs() {
			if bc, _, _ := sc.Lay(id); bc.Truong[TruongNhanNut] != "" {
				nhan = append(nhan, bc.Truong[TruongNhanNut])
			}
		}
		for _, n := range tb.NhanNut {
			co := false
			for _, l := range nhan {
				co = co || l == n
			}
			if !co {
				k.NhanNutKhongCo = append(k.NhanNutKhongCo, n)
			}
		}
	}
	return k
}

// HoTro is the verifier's verdict on one claim.
type HoTro string

const (
	CoHoTro    HoTro = "ho_tro"
	KhongHoTro HoTro = "khong_ho_tro"
	// KhongThongTin: the sentence states nothing to check (a greeting, a
	// question back to the person, an invitation to look at the card).
	// Every sentence gets a verdict, so a sentence the verifier skipped is
	// never read as a pass: saying that one carries no claim is itself a
	// judgement the verifier must write down.
	KhongThongTin HoTro = "khong_thong_tin"
)

// HoTros is the closed set of HoTro.
var HoTros = dong.Moi("ho_tro", CoHoTro, KhongHoTro, KhongThongTin)

// MenhDe is the verifier's verdict on one sentence of the answer. The
// answer step emits its text as numbered sentences (structured output), so
// the verifier returns an index and an enum, never free text: nothing it
// writes can carry an instruction into another prompt.
type MenhDe struct {
	// So is the sentence's 1-based index in the answer.
	So int
	// BangChungIDs are the evidence the verifier read it against.
	BangChungIDs []string
	Ket          HoTro
}

// PhanTu is the verifier's checked output.
type PhanTu struct {
	MenhDe []MenhDe
	// HuaHanhDongKhongCo: the answer says or promises an action no tool of
	// this turn performed.
	HuaHanhDongKhongCo bool
	// Tien: the answer creates, splits, settles, records or reminds about
	// money, or says it did.
	Tien bool
}

// Dat reports whether the answer may be released: no flag, and no sentence
// judged unsupported. Doc has already refused an output that leaves a
// sentence without a verdict, so every sentence was judged.
func (p PhanTu) Dat() bool {
	if p.HuaHanhDongKhongCo || p.Tien {
		return false
	}
	for _, m := range p.MenhDe {
		if m.Ket == KhongHoTro {
			return false
		}
	}
	return true
}

// Verifier is the LLM verifier: ONE model call through dem for the whole
// answer (not one per sentence), in a fresh context: it sees the numbered
// sentences and the evidence (under aliases, tools.SoCai.BiDanh) only, never
// the generation's history or instructions, and the answer is presented as
// another assistant's, since a model reviewing its own context is the
// weakest reviewer (research reflection-verification: cross-context review,
// external verifier over intrinsic self-correction). When the engine's
// policy runs it on an answer (answers that cite evidence or could promise
// an action), it runs before the first byte of that answer is released:
// nothing is streamed first and withdrawn. Which answers it runs on is
// budget policy, decided by the router's labels (Huong, YDinh, Tien), never
// by reading the answer's words; an optional call runs only while the turn
// keeps one call in reserve for the final answer.
type Verifier interface {
	PhanTu(ctx context.Context, cau []string, bc []truyhoi.BangChung, dem *llm.Dem) (PhanTu, error)
}

// MaxMenhDe bounds the sentences one verification covers.
const MaxMenhDe = 12

func i64(n int64) *int64     { return &n }
func f64(n float64) *float64 { return &n }

// LuocDo is the verifier's response schema, the one source of truth. soCau
// is how many sentences the answer has; bangChungIDs is the evidence (as
// the aliases the verifier is shown), offered as a closed enum; empty drops
// the enum (no id may then be cited).
func LuocDo(soCau int, bangChungIDs []string) *genai.Schema {
	id := &genai.Schema{Type: genai.TypeString}
	if len(bangChungIDs) > 0 {
		id.Enum = append([]string(nil), bangChungIDs...)
	}
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"menh_de": {Type: genai.TypeArray, MinItems: i64(int64(soCau)), MaxItems: i64(int64(soCau)), Items: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"so":             {Type: genai.TypeInteger, Minimum: f64(1), Maximum: f64(float64(soCau))},
					"bang_chung_ids": {Type: genai.TypeArray, Items: id},
					"ket":            {Type: genai.TypeString, Enum: HoTros.Values()},
				},
				PropertyOrdering: []string{"so", "bang_chung_ids", "ket"},
				Required:         []string{"so", "bang_chung_ids", "ket"},
			}},
			"hua_hanh_dong_khong_co": {Type: genai.TypeBoolean},
			"tien":                   {Type: genai.TypeBoolean},
		},
		PropertyOrdering: []string{"menh_de", "hua_hanh_dong_khong_co", "tien"},
		Required:         []string{"menh_de", "hua_hanh_dong_khong_co", "tien"},
	}
}

type menhDeTho struct {
	So           *int     `json:"so"`
	BangChungIDs []string `json:"bang_chung_ids"`
	Ket          *string  `json:"ket"`
}

type tho struct {
	MenhDe             []menhDeTho `json:"menh_de"`
	HuaHanhDongKhongCo *bool       `json:"hua_hanh_dong_khong_co"`
	Tien               *bool       `json:"tien"`
}

// ErrCauTruc is what every structural refusal wraps.
var ErrCauTruc = errors.New("kiemchung: verifier output refused")

// Doc reads the verifier's JSON strictly: an index outside 1..soCau, a
// sentence judged twice, a sentence left without a verdict, or a cited id
// outside bangChungIDs is refused. Every sentence 1..soCau must be judged
// exactly once: an output that judges fewer (a lazy model, or an
// instruction hidden in the evidence telling it to return nothing) would
// otherwise release the sentences it skipped unchecked. A
// refused verifier output means the answer is NOT released: the verifier
// failing is never read as a pass, because it carries the money and
// promised-action judgement the old phrase rules made.
func Doc(raw []byte, soCau int, bangChungIDs []string) (PhanTu, error) {
	var t tho
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil || dec.More() {
		return PhanTu{}, fmt.Errorf("%w: json: %v", ErrCauTruc, err)
	}
	if t.MenhDe == nil || t.HuaHanhDongKhongCo == nil || t.Tien == nil {
		return PhanTu{}, fmt.Errorf("%w: a required field is missing", ErrCauTruc)
	}
	if len(t.MenhDe) > MaxMenhDe {
		return PhanTu{}, fmt.Errorf("%w: %d claims", ErrCauTruc, len(t.MenhDe))
	}
	co := make(map[string]bool, len(bangChungIDs))
	for _, id := range bangChungIDs {
		co[id] = true
	}
	seen := map[int]bool{}
	p := PhanTu{HuaHanhDongKhongCo: *t.HuaHanhDongKhongCo, Tien: *t.Tien}
	for _, m := range t.MenhDe {
		if m.So == nil || m.Ket == nil || m.BangChungIDs == nil {
			return PhanTu{}, fmt.Errorf("%w: a claim field is missing", ErrCauTruc)
		}
		if *m.So < 1 || *m.So > soCau || seen[*m.So] {
			return PhanTu{}, fmt.Errorf("%w: sentence %d", ErrCauTruc, *m.So)
		}
		seen[*m.So] = true
		ket, err := HoTros.Parse(*m.Ket)
		if err != nil {
			return PhanTu{}, fmt.Errorf("%w: %v", ErrCauTruc, err)
		}
		for _, id := range m.BangChungIDs {
			if !co[id] {
				return PhanTu{}, fmt.Errorf("%w: claim cites %q, not shown", ErrCauTruc, id)
			}
		}
		p.MenhDe = append(p.MenhDe, MenhDe{So: *m.So, BangChungIDs: append([]string(nil), m.BangChungIDs...), Ket: ket})
	}
	if len(seen) != soCau {
		return PhanTu{}, fmt.Errorf("%w: %d of %d sentences judged", ErrCauTruc, len(seen), soCau)
	}
	return p, nil
}
