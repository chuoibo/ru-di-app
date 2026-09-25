package jobs

import (
	"strings"
	"testing"
)

func TestMessageRoundTripAndBound(t *testing.T) {
	m := Message{V: 1, Ref: "0b8f1c9e-aaaa-4bbb-8ccc-dddddddddd01", Seq: 3}
	body, err := m.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(body) > 128 {
		t.Fatalf("body %d bytes", len(body))
	}
	got, err := Decode(body)
	if err != nil || got != m {
		t.Fatalf("%+v %v", got, err)
	}
	if m.ID("ai.nep") != "ai.nep:0b8f1c9e-aaaa-4bbb-8ccc-dddddddddd01:3" {
		t.Fatal(m.ID("ai.nep"))
	}
}

func TestDecodeRefusesAnythingButIDs(t *testing.T) {
	for _, bad := range []string{
		`{"v":1,"ref":"0b8f1c9e-aaaa-4bbb-8ccc-dddddddddd01","seq":1,"prompt":"x"}`,
		`{"v":2,"ref":"0b8f1c9e-aaaa-4bbb-8ccc-dddddddddd01","seq":1}`,
		`{"v":1,"ref":"not-a-uuid","seq":1}`,
		`{"v":1,"ref":"0b8f1c9e-aaaa-4bbb-8ccc-dddddddddd01","seq":-1}`,
		`{"v":1,"ref":"0b8f1c9e-aaaa-4bbb-8ccc-dddddddddd01","seq":1} {}`,
		`{"v":1,"ref":"0b8f1c9e-aaaa-4bbb-8ccc-dddddddddd01","seq":1,"x":"` + strings.Repeat("a", 200) + `"}`,
		``,
	} {
		if _, err := Decode([]byte(bad)); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestTopologyNames(t *testing.T) {
	if _, err := NewTopology("Rudi prod"); err == nil {
		t.Fatal("bad namespace accepted")
	}
	top, _ := NewTopology("rudi")
	if top.Exchange() != "rudi.jobs" || top.Queue("ai.nep") != "rudi.ai.nep" || top.DeadQueue("ai.nep") != "rudi.ai.nep.dlq" {
		t.Fatal("names")
	}
}
