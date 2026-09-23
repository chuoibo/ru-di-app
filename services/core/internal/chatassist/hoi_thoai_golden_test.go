package chatassist

import (
	"encoding/json"
	"os"
	"testing"

	"mobile/services/core/internal/pyjson"
)

// The conversation shape the brain receives, pinned by a file the Python
// quality harness reads too (services/api/tests/skills/tra_loi_trong_nhom.py).
//
// A harness that builds a different conversation than the worker measures a
// model reading something production never sends. The only way to keep the two
// honest without running Go from Python is to make both answer to one
// handwritten fixture, byte for byte, key order included: the brain serialises
// the payload with json.dumps, so key order is part of what the model reads.
func TestHoiThoaiKhopGoldenDungChungVoiBoDo(t *testing.T) {
	raw, err := os.ReadFile("testdata/hoi_thoai_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Cases []struct {
			Ten          string          `json:"ten"`
			Goi          json.RawMessage `json:"goi"`
			Prompt       string          `json:"prompt"`
			Conversation json.RawMessage `json:"conversation"`
		} `json:"cases"`
	}
	if err = json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Cases) < 2 {
		t.Fatalf("golden chỉ có %d ca; cổng đang nhìn vào chỗ trống", len(file.Cases))
	}
	for _, ca := range file.Cases {
		t.Run(ca.Ten, func(t *testing.T) {
			goi := []byte(ca.Goi)
			if string(goi) == "null" {
				goi = nil
			}
			got, err := hoiThoai(goi, ca.Prompt)
			if err != nil {
				t.Fatal(err)
			}
			gotBytes, err := pyjson.Dumps(got)
			if err != nil {
				t.Fatal(err)
			}
			want, err := pyjson.Loads(ca.Conversation)
			if err != nil {
				t.Fatal(err)
			}
			wantBytes, err := pyjson.Dumps(want)
			if err != nil {
				t.Fatal(err)
			}
			if string(gotBytes) != string(wantBytes) {
				t.Fatalf("hoiThoai lệch golden:\n got %s\nwant %s", gotBytes, wantBytes)
			}
		})
	}
}
