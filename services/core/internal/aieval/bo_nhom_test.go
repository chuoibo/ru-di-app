package aieval

import (
	"bytes"
	"strings"
	"testing"
)

const fileBoNhom = "testdata/corpus/nhom-kich-ban.json"

func napBoNhom(t *testing.T) (Bo, string, map[string]KichBan) {
	t.Helper()
	kbs, err := DocKichBan(thuMucKichBan)
	if err != nil {
		t.Fatal(err)
	}
	b, sha, err := DocBo(fileBoNhom, kbs)
	if err != nil {
		t.Fatal(err)
	}
	return b, sha, kbs
}

// T1 of the group assistant in the thread (slice 9): every correct script
// passes -- every invariant, the group's 4 (Nếp's memory never in a group
// request) and 9 (the `tra_loi` card GroundReply accepts whole) included --
// every wrong script fails the check it names, the canary is red exactly at
// khong_bia_dia_diem and the identity case is green; twice, the same bytes.
func TestBoNhomKichBan(t *testing.T) {
	b, sha, kbs := napBoNhom(t)
	tk, rs, raw := chayCaBo(t, b, sha, kbs, 1)
	for _, r := range rs {
		if !r.Dat {
			t.Errorf("%s [%s, %s]: %s %+v", r.CaID, r.Vai, r.KichBan, r.LyDo, r.Truot)
		}
	}
	if !tk.Xanh || tk.KhongDat != 0 || tk.SaiDat != tk.SoSai || tk.SoSai == 0 {
		t.Fatalf("tổng kết: %+v", tk)
	}
	if !tk.Canary.CoMat || !tk.Canary.Dat || strings.Join(tk.Canary.Truot, ",") != KiemKhongBiaDiaDiem {
		t.Fatalf("canary: %+v", tk.Canary)
	}
	if !tk.DongNhat.CoMat || !tk.DongNhat.Dat || len(tk.DongNhat.Truot) != 0 {
		t.Fatalf("đồng nhất: %+v", tk.DongNhat)
	}
	for _, c := range b.Ca {
		if c.BeMat != "nhom" || c.KyVong.The == nil {
			t.Fatalf("ca %s không phải ca nhóm có kỳ vọng thẻ", c.CaID)
		}
	}
	_, _, raw2 := chayCaBo(t, b, sha, kbs, 1)
	if !bytes.Equal(raw, raw2) {
		t.Fatal("hai lần chạy cùng bộ nhóm ra hai báo cáo khác nhau")
	}
	t.Logf("%d ca nhóm, %d lượt chạy, %d kịch bản sai trượt đúng chỗ", tk.SoCa, tk.SoLuot, tk.SaiDat)
}
