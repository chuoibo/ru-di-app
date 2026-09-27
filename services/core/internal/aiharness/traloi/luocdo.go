// Package traloi is the answer step of a retrieval turn (find_places, the
// app manual): the CRAG loop (crag.TruyHoi), then ONE structured answer
// call, the deterministic grounding checks, the LLM verifier, at most one
// regeneration with the findings as data, and a fixed fallback. Nothing is
// released before it has passed all of them (draft-then-verify): there is no
// retract event, so a check that could lead to a regeneration must finish
// before the first byte leaves.
//
// Who decides what (docs/architecture/03-ai-engine-hop-dong.md §8):
//   - the MODEL decides whether the evidence is enough (the grader), which
//     corrective move to make, whether to answer, ask back or refuse, which
//     constraints are unmet (enums), what the prose says, and whether each
//     sentence is supported, promises an action or acts on money (the
//     verifier);
//   - Go checks structure (schemas, closed enums, alias membership), budgets,
//     the relaxation order and that hard constraints never move; it resolves
//     place tokens from the ledger, checks cited ids, stated values and
//     button labels by set membership and exact equality, and checks the
//     released text's DATA FORMATS for privacy (guard.DinhDang: phones,
//     account and card numbers, emails). No word list or regex reads the
//     meaning of the question or the answer.
package traloi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/crag"
	"mobile/services/core/internal/aiharness/dong"
	"mobile/services/core/internal/aiharness/kiemchung"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// HanhDong is what the answer step chose to do.
type HanhDong string

const (
	// TraLoi: answer from the evidence.
	TraLoi HanhDong = "tra_loi"
	// HoiLai: ask the person ONE question about the unmet constraints (a
	// fixed sentence, cau.HoiLai).
	HoiLai HanhDong = "hoi_lai"
	// TuChoi: say which constraints nothing meets (a fixed sentence,
	// cau.TuChoi).
	TuChoi HanhDong = "tu_choi"
)

// HanhDongs is the closed set of HanhDong.
var HanhDongs = dong.Moi("hanh_dong", TraLoi, HoiLai, TuChoi)

// Truong is an evidence field an answer may state a value of.
type Truong string

const (
	TruongGia Truong = "gia"
	TruongGio Truong = "gio"
)

// Truongs is the closed set of Truong.
var Truongs = dong.Moi("truong_trich", TruongGia, TruongGio)

// Limits of the answer's structured output.
const (
	// MaxCau is the sentences of one answer: what one verification covers.
	MaxCau = kiemchung.MaxMenhDe
	// MaxRuneCau bounds one sentence.
	MaxRuneCau = 300
	// MaxBangChungMoiCau bounds the evidence one sentence cites.
	MaxBangChungMoiCau = 5
	// MaxTrichMoiCau bounds the values one sentence states.
	MaxTrichMoiCau = 4
	// MaxRuneGiaTri bounds one stated value.
	MaxRuneGiaTri = 40
	// MaxTokensTraLoi bounds the answer's output.
	MaxTokensTraLoi = 2048
)

// Trich is one value a sentence states: the evidence alias it belongs to,
// the field, and the value exactly as written in the sentence.
type Trich struct {
	BiDanh string
	Truong Truong
	GiaTri string
}

// Cau is one sentence of the draft.
type Cau struct {
	// Chu is the sentence. Places appear only as [[p:<alias>]] tokens and
	// button labels only inside «…».
	Chu string
	// BangChung are the aliases of the evidence the sentence rests on.
	BangChung []string
	Trich     []Trich
}

// Nhap is the answer step's checked output.
type Nhap struct {
	HanhDong HanhDong
	// RangBuocKhongDat are the constraints the evidence leaves unmet.
	RangBuocKhongDat []truyhoi.RangBuoc
	Cau              []Cau
}

func i64(n int64) *int64 { return &n }

// LuocDo is the answer step's response schema, the one source of truth.
// biDanh are the aliases of this turn's evidence, offered as a closed enum:
// the model cannot cite a real id at all. It must not be empty (with no
// evidence there is nothing to answer from, and no call is made).
func LuocDo(biDanh []string) *genai.Schema {
	alias := &genai.Schema{Type: genai.TypeString, Enum: append([]string(nil), biDanh...)}
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"hanh_dong": {Type: genai.TypeString, Enum: HanhDongs.Values()},
			"rang_buoc_khong_dat": {Type: genai.TypeArray, MaxItems: i64(int64(crag.RangBuocs.Len())),
				Items: &genai.Schema{Type: genai.TypeString, Enum: crag.RangBuocs.Values()}},
			"cau": {Type: genai.TypeArray, MaxItems: i64(MaxCau), Items: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"chu":        {Type: genai.TypeString, MaxLength: i64(MaxRuneCau)},
					"bang_chung": {Type: genai.TypeArray, MaxItems: i64(MaxBangChungMoiCau), Items: alias},
					"trich": {Type: genai.TypeArray, MaxItems: i64(MaxTrichMoiCau), Items: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"bi_danh": alias,
							"truong":  {Type: genai.TypeString, Enum: Truongs.Values()},
							"gia_tri": {Type: genai.TypeString, MaxLength: i64(MaxRuneGiaTri)},
						},
						PropertyOrdering: []string{"bi_danh", "truong", "gia_tri"},
						Required:         []string{"bi_danh", "truong", "gia_tri"},
					}},
				},
				PropertyOrdering: []string{"chu", "bang_chung", "trich"},
				Required:         []string{"chu", "bang_chung", "trich"},
			}},
		},
		PropertyOrdering: []string{"hanh_dong", "rang_buoc_khong_dat", "cau"},
		Required:         []string{"hanh_dong", "rang_buoc_khong_dat", "cau"},
	}
}

type trichTho struct {
	BiDanh *string `json:"bi_danh"`
	Truong *string `json:"truong"`
	GiaTri *string `json:"gia_tri"`
}

type cauTho struct {
	Chu       *string    `json:"chu"`
	BangChung []string   `json:"bang_chung"`
	Trich     []trichTho `json:"trich"`
}

type tho struct {
	HanhDong         *string  `json:"hanh_dong"`
	RangBuocKhongDat []string `json:"rang_buoc_khong_dat"`
	Cau              []cauTho `json:"cau"`
}

// ErrCauTruc is what every structural refusal wraps.
var ErrCauTruc = errors.New("traloi: answer output refused")

// Doc reads the answer step's JSON strictly against biDanh. An answer has
// 1..MaxCau non-empty sentences; a question back or a refusal has none (the
// sentence is ours) and names at least one unmet constraint. Every alias is
// one of biDanh; a sentence cites an alias once.
func Doc(raw []byte, biDanh []string) (Nhap, error) {
	var t tho
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil || dec.More() {
		return Nhap{}, fmt.Errorf("%w: json: %v", ErrCauTruc, err)
	}
	if t.HanhDong == nil || t.RangBuocKhongDat == nil || t.Cau == nil {
		return Nhap{}, fmt.Errorf("%w: a required field is missing", ErrCauTruc)
	}
	var n Nhap
	var err error
	if n.HanhDong, err = HanhDongs.Parse(*t.HanhDong); err != nil {
		return Nhap{}, fmt.Errorf("%w: %v", ErrCauTruc, err)
	}
	if n.RangBuocKhongDat, err = crag.RangBuocs.ParseAll(t.RangBuocKhongDat); err != nil {
		return Nhap{}, fmt.Errorf("%w: %v", ErrCauTruc, err)
	}
	switch n.HanhDong {
	case TraLoi:
		if len(t.Cau) == 0 || len(t.Cau) > MaxCau {
			return Nhap{}, fmt.Errorf("%w: %d sentences", ErrCauTruc, len(t.Cau))
		}
	default:
		if len(t.Cau) > 0 || len(n.RangBuocKhongDat) == 0 {
			return Nhap{}, fmt.Errorf("%w: %s needs unmet constraints and no sentence", ErrCauTruc, n.HanhDong)
		}
	}
	if len(biDanh) == 0 {
		return Nhap{}, fmt.Errorf("%w: no evidence to cite", ErrCauTruc)
	}
	bi := dong.Moi("bi_danh", biDanh...)
	for i, c := range t.Cau {
		if c.Chu == nil || c.BangChung == nil || c.Trich == nil {
			return Nhap{}, fmt.Errorf("%w: sentence %d: a field is missing", ErrCauTruc, i+1)
		}
		chu := strings.TrimSpace(*c.Chu)
		if chu == "" || utf8.RuneCountInString(chu) > MaxRuneCau {
			return Nhap{}, fmt.Errorf("%w: sentence %d: length", ErrCauTruc, i+1)
		}
		if len(c.BangChung) > MaxBangChungMoiCau || len(c.Trich) > MaxTrichMoiCau {
			return Nhap{}, fmt.Errorf("%w: sentence %d: too many citations", ErrCauTruc, i+1)
		}
		bc, err := bi.ParseAll(c.BangChung)
		if err != nil {
			return Nhap{}, fmt.Errorf("%w: sentence %d: %v", ErrCauTruc, i+1, err)
		}
		out := Cau{Chu: chu, BangChung: bc}
		for _, tr := range c.Trich {
			if tr.BiDanh == nil || tr.Truong == nil || tr.GiaTri == nil {
				return Nhap{}, fmt.Errorf("%w: sentence %d: a value field is missing", ErrCauTruc, i+1)
			}
			a, err := bi.Parse(*tr.BiDanh)
			if err != nil {
				return Nhap{}, fmt.Errorf("%w: sentence %d: %v", ErrCauTruc, i+1, err)
			}
			f, err := Truongs.Parse(*tr.Truong)
			if err != nil {
				return Nhap{}, fmt.Errorf("%w: sentence %d: %v", ErrCauTruc, i+1, err)
			}
			if *tr.GiaTri == "" || utf8.RuneCountInString(*tr.GiaTri) > MaxRuneGiaTri {
				return Nhap{}, fmt.Errorf("%w: sentence %d: value length", ErrCauTruc, i+1)
			}
			out.Trich = append(out.Trich, Trich{BiDanh: a, Truong: f, GiaTri: *tr.GiaTri})
		}
		n.Cau = append(n.Cau, out)
	}
	return n, nil
}
