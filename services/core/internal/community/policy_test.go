package community

import (
	"testing"
	"time"
)

func TestModerationFailsClosed(t *testing.T) {
	cases := []struct {
		v              verdict
		comment, media bool
		want           string
	}{
		{verdict{Relevant: true, Safe: true, Confidence: 999}, false, false, "approved"},
		{verdict{Relevant: false, Safe: true, Confidence: 999}, false, false, "rejected"},
		{verdict{Relevant: false, Safe: true, Confidence: 999}, true, false, "approved"},
		{verdict{Relevant: true, Safe: false, Confidence: 999}, false, false, "rejected"},
		{verdict{Relevant: true, Safe: true, Confidence: 899}, false, false, "review"},
		{verdict{Relevant: true, Safe: true, Confidence: 999}, false, true, "review"},
		{verdict{Relevant: true, Safe: true, Confidence: 999, MediaChecked: true}, false, true, "approved"},
	}
	for _, c := range cases {
		got, _ := decision(c.v, c.comment, c.media)
		if got != c.want {
			t.Fatalf("%+v: %s", c, got)
		}
	}
}
func TestRankUsesConsentAndFollowingChronology(t *testing.T) {
	now := time.Now()
	a := candidate{ID: "a", Author: "a", Created: now.Add(-time.Hour)}
	b := candidate{ID: "b", Author: "b", Created: now.Add(-2 * time.Hour), Affinity: 100}
	if got := rank([]candidate{a, b}, "for_you", false, now); got[0] != "a" {
		t.Fatal(got)
	}
	if got := rank([]candidate{a, b}, "for_you", true, now); got[0] != "b" {
		t.Fatal(got)
	}
	if got := rank([]candidate{a, b}, "following", true, now); got[0] != "a" {
		t.Fatal(got)
	}
}
func TestTopicNormalization(t *testing.T) {
	got, err := normalizeTopics([]string{"  #Đi BỘ  ", "đi bộ"})
	if err != nil || len(got) != 1 || got[0] != "đi bộ" {
		t.Fatal(got, err)
	}
	if _, err = normalizeTopics([]string{"<script>"}); err == nil {
		t.Fatal("invalid topic accepted")
	}
}
