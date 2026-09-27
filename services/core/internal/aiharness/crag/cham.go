package crag

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"google.golang.org/adk/model"

	"mobile/services/core/internal/aiharness/cautruc"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// chamHe is the grader's static system instruction.
//
//go:embed cham.txt
var chamHe string

// Data block names of the grader's user turn. Ours, never data.
const (
	KhoiYeuCau    prompts.Nguon = "yeu_cau"
	KhoiBangChung prompts.Nguon = "bang_chung"
	KhoiBiLoai    prompts.Nguon = "bi_loai"
)

// MaxTokensCham bounds the grader's output: four short fields.
const MaxTokensCham = 256

// ChamLLM is the grader: one flash-lite call with LuocDo as its response
// schema, read strictly by Doc. It sees the request's constraints as ids and
// the evidence under aliases; it never sees the conversation.
type ChamLLM struct{}

var _ Cham = ChamLLM{}

// BiDanhCham is the alias the grader shows evidence i under when the caller
// has none (the loop passes the ledger's aliases through Vao.BiDanh).
func BiDanhCham(i int, _ truyhoi.BangChung) string { return fmt.Sprintf("b%d", i+1) }

// YeuCauCham is the grader's request for v, byte-stable for the same v.
func YeuCauCham(v Vao) *model.LLMRequest {
	return cautruc.YeuCau(strings.TrimSpace(chamHe), NoiDungCham(v), LuocDo(), MaxTokensCham)
}

// NoiDungCham is the grader's user turn: the request, the evidence, the
// counts of what the hard filters removed.
func NoiDungCham(v Vao) string {
	bi := v.BiDanh
	if bi == nil {
		bi = BiDanhCham
	}
	parts := []string{
		prompts.BocDuLieu(KhoiYeuCau, MoTaYeuCau(v.YeuCau)),
		cautruc.KhoiBangChung(KhoiBangChung, v.BangChung, bi),
	}
	if len(v.BiLoai) > 0 {
		parts = append(parts, prompts.BocDuLieu(KhoiBiLoai, moTaBiLoai(v.BiLoai)))
	}
	parts = append(parts, "Chấm bằng chứng trên theo schema.")
	return strings.Join(parts, "\n\n")
}

// MoTaYeuCau writes a request's constraints for a model: ids, integers and
// an RFC 3339 instant as they are, the query text datamarked.
func MoTaYeuCau(y truyhoi.YeuCau) string {
	var b strings.Builder
	w := func(k, v string) {
		if v != "" {
			b.WriteString(k + ": " + v + "\n")
		}
	}
	w("nguon", string(y.Nguon))
	w("truy_van", prompts.DanhDau(y.Cau))
	w("cung.diem_den", y.Cung.DiemDenID)
	w("cung.di_ung", strings.Join(y.Cung.DiUng, ", "))
	w("cung.an_kieng", strings.Join(y.Cung.AnKieng, ", "))
	if y.Cung.MoLuc != nil {
		w("cung.mo_luc", y.Cung.MoLuc.Format(time.RFC3339))
	}
	if k := y.Cung.MoTrong; k != nil {
		w("cung.mo_trong", k.Tu.Format(time.RFC3339)+"/"+k.Den.Format(time.RFC3339))
	}
	if y.DiUngNgoaiDanhMuc {
		w("cung.di_ung_ngoai_danh_muc", "true")
	}
	if y.Cung.NganSachVND != nil {
		w("cung.ngan_sach_vnd_moi_nguoi", fmt.Sprint(*y.Cung.NganSachVND))
	}
	w("mem.loai_cho", strings.Join(y.Mem.LoaiCho, ", "))
	w("mem.khi_chat", strings.Join(y.Mem.KhiChat, ", "))
	w("mem.khu_vuc", y.Mem.KhuVuc)
	return strings.TrimRight(b.String(), "\n")
}

func moTaBiLoai(m map[truyhoi.RangBuoc]int) string {
	var lines []string
	for _, r := range RangBuocs.Values() {
		if n := m[truyhoi.RangBuoc(r)]; n > 0 {
			lines = append(lines, fmt.Sprintf("%s: %d", r, n))
		}
	}
	return strings.Join(lines, "\n")
}

// DanhGia makes the one grader call through dem.
func (ChamLLM) DanhGia(ctx context.Context, v Vao, dem *llm.Dem) (DanhGia, error) {
	raw, err := cautruc.Goi(ctx, dem, YeuCauCham(v))
	if err != nil {
		return DanhGia{}, err
	}
	return Doc([]byte(raw))
}
