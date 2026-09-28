package giomo

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

type golden struct {
	DocDuoc []struct {
		Kieu string   `json:"kieu"`
		Gio  string   `json:"gio"`
		Luc  [][3]any `json:"luc"`
	} `json:"doc_duoc"`
	KhongRo []string `json:"khong_ro"`
}

var thu = map[string]int{"Mo": 0, "Tu": 1, "We": 2, "Th": 3, "Fr": 4, "Sa": 5, "Su": 6}

func load(t *testing.T) golden {
	t.Helper()
	raw, err := os.ReadFile("testdata/mo_cua_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var g golden
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

func TestGoldenOpenAt(t *testing.T) {
	g := load(t)
	checked := 0
	for _, c := range g.DocDuoc {
		lich, ok := Doc(c.Gio)
		if !ok {
			t.Errorf("%q: not read", c.Gio)
			continue
		}
		for _, l := range c.Luc {
			day, hhmm, want := l[0].(string), l[1].(string), l[2].(bool)
			var h, m int
			if _, err := time.Parse("15:04", hhmm); err != nil {
				t.Fatal(err)
			}
			h, m = int(hhmm[0]-'0')*10+int(hhmm[1]-'0'), int(hhmm[3]-'0')*10+int(hhmm[4]-'0')
			minute := thu[day]*24*60 + h*60 + m
			if got := lich.MoLuc(minute); got != want {
				t.Errorf("%q at %s %s: open=%v, want %v (%s)", c.Gio, day, hhmm, got, want, lich.SQL())
			}
			checked++
		}
	}
	if checked < 30 {
		t.Fatalf("only %d instants checked", checked)
	}
}

func TestUnknownIsNotClosed(t *testing.T) {
	for _, s := range load(t).KhongRo {
		if l, ok := Doc(s); ok {
			t.Errorf("%q read as %s; unknown hours must stay unknown", s, l.SQL())
		}
	}
}

func TestWeekMinuteIsVietnamTime(t *testing.T) {
	// Friday 15:30 UTC (the twenty-fifth of September) is Friday 22:30 in Vietnam.
	utc := time.Date(2026, 9, 25, 15, 30, 0, 0, time.UTC)
	if got, want := PhutCuaTuan(utc), 4*24*60+22*60+30; got != want {
		t.Fatalf("minute %d, want %d", got, want)
	}
	// Sunday 23:30 in Vietnam is Sunday 16:30 UTC; Monday 00:10 VN is Sunday 17:10 UTC.
	if PhutCuaTuan(time.Date(2026, 9, 27, 17, 10, 0, 0, time.UTC)) != 10 {
		t.Fatal("Monday 00:10 in Vietnam must be minute 10 of the week")
	}
}

func TestMergedAndSQL(t *testing.T) {
	l, ok := Doc("Mo 08:00-12:00,11:00-14:00")
	if !ok || l.SQL() != "{[480,840)}" {
		t.Fatalf("overlapping spans must merge: %v %s", ok, l.SQL())
	}
	if !l.MoSuot(480, 840) || l.MoSuot(480, 841) || l.MoSuot(500, 500) {
		t.Fatal("MoSuot must require the whole window")
	}
}
