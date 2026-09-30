package tuvung

import (
	"reflect"
	"testing"

	"mobile/services/core/internal/domain/catalog"
)

// The ten categories the owner fixed on 2026-09-29, in order, each with a
// Vietnamese label and a definition, and khong_ro never one of them.
func TestDanhMucDongVaCoDinhNghia(t *testing.T) {
	want := []string{"quan_an", "an_vat", "cafe", "bar_nhau", "cho_am_thuc", "thien_nhien", "van_hoa", "vui_choi", "trai_nghiem", "luu_tru"}
	if got := DanhMuc.IDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("DanhMuc ids %v", got)
	}
	if DanhMuc.Ten() != "danh_muc" || DanhMuc.Co(KhongRo) {
		t.Fatal("khong_ro is a category, or the vocabulary is misnamed")
	}
	for _, m := range DanhMuc.Muc() {
		if m.Nhan == "" || len([]rune(m.DinhNghia)) < 40 || len(m.Cum) != 0 {
			t.Errorf("%s: label %q, definition %q, %d phrases", m.ID, m.Nhan, m.DinhNghia, len(m.Cum))
		}
		if len(m.ID) > 32 {
			t.Errorf("%s is longer than the index's 32-byte tag", m.ID)
		}
	}
}

// Every one of the ten maps to exactly one UI category, and every group is
// a catalogue category.
func TestDanhMucMoiMucMotNhomUI(t *testing.T) {
	ui := map[string]bool{}
	for _, c := range catalog.Categories {
		ui[c.ID] = true
	}
	count := map[string]int{}
	for nhom, ids := range DanhMucTheoNhomUI {
		if !ui[nhom] {
			t.Errorf("group %q is not a catalogue category", nhom)
		}
		for _, id := range ids {
			if !DanhMuc.Co(id) {
				t.Errorf("group %s holds %q, not a DanhMuc id", nhom, id)
			}
			count[id]++
		}
	}
	for _, id := range DanhMuc.IDs() {
		if count[id] != 1 {
			t.Errorf("%s is in %d UI groups", id, count[id])
		}
		if g := NhomUI(id); g == "" || !ui[g] {
			t.Errorf("NhomUI(%s) = %q", id, g)
		}
	}
	if len(DanhMucTheoNhomUI) != len(catalog.Categories) {
		t.Fatalf("%d groups for %d UI categories", len(DanhMucTheoNhomUI), len(catalog.Categories))
	}
	if NhomUI(KhongRo) != "" || NhomUI("bịa") != "" {
		t.Fatal("an unknown category mapped to a UI group")
	}
	if got := DanhMucCuaNhomUI("quan-an-local"); !reflect.DeepEqual(got, []string{"quan_an", "an_vat", "cho_am_thuc"}) {
		t.Fatalf("quan-an-local covers %v", got)
	}
}
