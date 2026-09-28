package huongdan

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

var reBanTS = regexp.MustCompile(`(?m)^export const HUONG_DAN_BAN = "([0-9a-f]{12})";$`)

// banTrongTS reads the constant the extractor writes into huong-dan-ban.ts.
func banTrongTS(src string) (string, error) {
	m := reBanTS.FindAllStringSubmatch(src, -1)
	if len(m) != 1 {
		return "", errors.New("huong-dan-ban.ts must hold exactly one HUONG_DAN_BAN of 12 hex")
	}
	return m[0][1], nil
}

func TestBanDungLaBamCuaRut(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("data", "_rut.json"))
	if err != nil {
		t.Fatal(err)
	}
	tong := sha256.Sum256(raw)
	if want := hex.EncodeToString(tong[:])[:12]; BanDung() != want {
		t.Fatalf("BanDung() = %q, sha256 of data/_rut.json starts %q", BanDung(), want)
	}
}

// The app carries the same value, written by apps/mobile/tools/rut-huong-dan.mjs.
// Regenerating _rut.json without the constant, or editing either by hand,
// turns this red.
func TestBanDungKhopHangSoTS(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "apps", "mobile", "src", "rudi", "nep", "huong-dan-ban.ts"))
	if err != nil {
		t.Fatal(err)
	}
	ts, err := banTrongTS(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if ts != BanDung() {
		t.Fatalf("huong-dan-ban.ts says %s, the embedded _rut.json hashes to %s: run node tools/rut-huong-dan.mjs in apps/mobile", ts, BanDung())
	}
}

// The reader above must not go blind: a stale constant reads as a different
// value, and a file without one is an error rather than an empty match.
func TestBanTrongTSCanary(t *testing.T) {
	if got, err := banTrongTS(`export const HUONG_DAN_BAN = "ffffffffffff";` + "\n"); err != nil || got == BanDung() {
		t.Fatalf("stale constant read as %q, %v", got, err)
	}
	for _, xau := range []string{"", `export const HUONG_DAN_BAN = "4ee7";`, `export const HUONG_DAN_BAN = "ABCDEF123456";`,
		"export const HUONG_DAN_BAN = \"aaaaaaaaaaaa\";\nexport const HUONG_DAN_BAN = \"bbbbbbbbbbbb\";\n"} {
		if _, err := banTrongTS(xau); err == nil {
			t.Errorf("%q read as a constant", xau)
		}
	}
}
