// Package crag is corrective retrieval: after a retrieval, ONE model call
// (the grader) judges whether the evidence answers the request, names the
// constraints it leaves unmet, and may propose ONE corrective move: relax
// soft constraints, or rewrite the query. Go applies the move exactly as
// proposed, never touches a hard constraint, and allows at most
// llm.MaxCorrectiveRounds rounds per turn.
//
// Whether evidence is enough is the model's judgement; Go reads no words to
// decide it (docs/architecture/03-ai-engine-hop-dong.md, «Luật không
// heuristic»). A shortfall after the round is answered honestly with what
// there is, never by widening a hard filter.
package crag

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/dong"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// KetLuan is the grader's verdict on the evidence.
type KetLuan string

const (
	// Du: the evidence answers the request.
	Du KetLuan = "du"
	// Thieu: some constraint is unmet or there is too little.
	Thieu KetLuan = "thieu"
	// MauThuan: the evidence contradicts the request or itself.
	MauThuan KetLuan = "mau_thuan"
)

// KetLuans is the closed set of KetLuan.
var KetLuans = dong.Moi("ket_luan", Du, Thieu, MauThuan)

// RangBuocs is every constraint name the grader may report as unmet.
var RangBuocs = dong.Moi("rang_buoc",
	truyhoi.RBDiemDen, truyhoi.RBDiUng, truyhoi.RBAnKieng, truyhoi.RBMoLuc, truyhoi.RBNganSach,
	truyhoi.RBKhiChat, truyhoi.RBLoaiCho, truyhoi.RBKhuVuc)

// MaxVietLai is a rewritten query's ceiling in runes.
const MaxVietLai = 200

// DanhGia is the grader's checked output.
type DanhGia struct {
	KetLuan KetLuan
	// RangBuocThieu are the constraints the evidence leaves unmet.
	RangBuocThieu []truyhoi.RangBuoc
	// NoiLong are SOFT constraints to relax (truyhoi.RangBuocMems); a hard
	// name is refused by Doc.
	NoiLong []truyhoi.RangBuoc
	// VietLai is a rewritten query. At most one of NoiLong and VietLai is
	// set, and neither when KetLuan is Du.
	VietLai string
}

// Vao is what the grader reads: the request as built (its hard and soft
// constraints) and the evidence it returned, both inside <du_lieu> blocks.
type Vao struct {
	YeuCau    truyhoi.YeuCau
	BangChung []truyhoi.BangChung
	// BiLoai is the retrieval's count of candidates each hard constraint
	// removed (counts only), so the grader can tell «nothing matches the
	// allergy» from «the query missed».
	BiLoai map[truyhoi.RangBuoc]int
	// BiDanh names evidence i as the grader sees it: the turn ledger's
	// alias (tools.SoCai.BiDanh), so no real id enters the prompt. Nil
	// means b1, b2, … in order.
	BiDanh func(i int, b truyhoi.BangChung) string
}

// Cham is the grader.
type Cham interface {
	DanhGia(ctx context.Context, v Vao, dem *llm.Dem) (DanhGia, error)
}

func i64(n int64) *int64 { return &n }

// LuocDo is the grader's response schema, the one source of truth.
func LuocDo() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"ket_luan": {Type: genai.TypeString, Enum: KetLuans.Values()},
			"rang_buoc_thieu": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString, Enum: RangBuocs.Values()},
				MaxItems: i64(int64(RangBuocs.Len()))},
			"noi_long": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString, Enum: truyhoi.RangBuocMems.Values()},
				MaxItems: i64(int64(truyhoi.RangBuocMems.Len()))},
			"viet_lai": {Type: genai.TypeString, MaxLength: i64(MaxVietLai)},
		},
		PropertyOrdering: []string{"ket_luan", "rang_buoc_thieu", "noi_long", "viet_lai"},
		Required:         []string{"ket_luan", "rang_buoc_thieu"},
	}
}

type tho struct {
	KetLuan       *string  `json:"ket_luan"`
	RangBuocThieu []string `json:"rang_buoc_thieu"`
	NoiLong       []string `json:"noi_long"`
	VietLai       string   `json:"viet_lai"`
}

// ErrCauTruc is what every structural refusal wraps.
var ErrCauTruc = errors.New("crag: grader output refused")

// Doc reads the grader's JSON strictly.
func Doc(raw []byte) (DanhGia, error) {
	var t tho
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil || dec.More() {
		return DanhGia{}, fmt.Errorf("%w: json: %v", ErrCauTruc, err)
	}
	if t.KetLuan == nil || t.RangBuocThieu == nil {
		return DanhGia{}, fmt.Errorf("%w: a required field is missing", ErrCauTruc)
	}
	var d DanhGia
	var err error
	if d.KetLuan, err = KetLuans.Parse(*t.KetLuan); err != nil {
		return DanhGia{}, fmt.Errorf("%w: %v", ErrCauTruc, err)
	}
	if d.RangBuocThieu, err = RangBuocs.ParseAll(t.RangBuocThieu); err != nil {
		return DanhGia{}, fmt.Errorf("%w: %v", ErrCauTruc, err)
	}
	if d.NoiLong, err = truyhoi.RangBuocMems.ParseAll(t.NoiLong); err != nil {
		return DanhGia{}, fmt.Errorf("%w: %v", ErrCauTruc, err)
	}
	if utf8.RuneCountInString(t.VietLai) > MaxVietLai {
		return DanhGia{}, fmt.Errorf("%w: viet_lai too long", ErrCauTruc)
	}
	d.VietLai = t.VietLai
	if len(d.NoiLong) > 0 && d.VietLai != "" {
		return DanhGia{}, fmt.Errorf("%w: both a relaxation and a rewrite", ErrCauTruc)
	}
	if d.KetLuan == Du && (len(d.NoiLong) > 0 || d.VietLai != "" || len(d.RangBuocThieu) > 0) {
		return DanhGia{}, fmt.Errorf("%w: a sufficient verdict with a correction", ErrCauTruc)
	}
	if len(d.NoiLong) == 0 {
		d.NoiLong = nil
	}
	if len(d.RangBuocThieu) == 0 {
		d.RangBuocThieu = nil
	}
	return d, nil
}

// ErrHetVong: the turn has run MaxCorrectiveRounds rounds.
var ErrHetVong = errors.New("crag: corrective round budget spent")

// SuaYeuCau applies the grader's move to y for round vong (1-based): drops
// the soft constraints it named, or replaces the query text. The hard
// constraints are copied unchanged. ok is false when there is no move.
func SuaYeuCau(y truyhoi.YeuCau, d DanhGia, vong int) (truyhoi.YeuCau, bool, error) {
	if d.KetLuan == Du || (len(d.NoiLong) == 0 && d.VietLai == "") {
		return y, false, nil
	}
	if vong < 1 || vong > llm.MaxCorrectiveRounds {
		return y, false, ErrHetVong
	}
	out := y
	for _, r := range d.NoiLong {
		m, err := out.Mem.Bo(r)
		if err != nil {
			return y, false, err
		}
		out.Mem = m
	}
	if d.VietLai != "" {
		out.Cau = d.VietLai
	}
	return out, true, nil
}
