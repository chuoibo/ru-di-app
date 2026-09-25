package pairpaper

import (
	"encoding/json"
	"os"
	"testing"
)

// The weekly ceiling is the shared file's number (ADR-0034 §2.5).
func TestToMoiNguoiMoiTuanMatchesSharedFile(t *testing.T) {
	raw, err := os.ReadFile("../../../../../packages/shared/nep-nhip.json")
	if err != nil {
		t.Fatalf("read shared nep-nhip.json: %v", err)
	}
	var shared struct {
		ToMoiNguoiMoiTuan int `json:"to_moi_nguoi_moi_tuan"`
	}
	if err := json.Unmarshal(raw, &shared); err != nil {
		t.Fatal(err)
	}
	if shared.ToMoiNguoiMoiTuan != ToMoiNguoiMoiTuan {
		t.Fatalf("nep-nhip.json says %d, Go says %d", shared.ToMoiNguoiMoiTuan, ToMoiNguoiMoiTuan)
	}
}
