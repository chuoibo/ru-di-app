package routes

import (
	"fmt"
	"testing"
	"time"

	"mobile/services/core/internal/pyjson"
)

// The ordered-list cache is bounded by cards, not by entries: one TP.HCM list
// is 9,514 cards, so 48 of them would hold the whole city 48 times over.
func TestRankedCacheStaysWithinItsCardBudget(t *testing.T) {
	c := &catalogueCache{byDest: map[string]*catalogueSnapshot{}, ranked: map[string]rankedEntry{}}
	c.once.Do(func() {})
	c.ttl = time.Minute
	city := make([]*pyjson.OrderedMap, 9514)
	start := time.Now()
	for i := range 10 {
		snap := &catalogueSnapshot{at: start.Add(time.Duration(i) * time.Millisecond)}
		c.keepRanked(fmt.Sprintf("d-tinh-79\x00taste-%d\x00", i), city, snap)
		total := 0
		for _, e := range c.ranked {
			total += len(e.list)
		}
		if total > maxRankedCards {
			t.Fatalf("after %d lists the cache holds %d cards, over the budget of %d", i+1, total, maxRankedCards)
		}
	}
	if c.rankedView("d-tinh-79\x00taste-9\x00") == nil {
		t.Fatal("the newest list was evicted instead of the oldest")
	}
	if c.rankedView("d-tinh-79\x00taste-0\x00") != nil {
		t.Fatal("the oldest list survived past the budget")
	}
	c.keepRanked("too-big", make([]*pyjson.OrderedMap, maxRankedCards+1), &catalogueSnapshot{at: time.Now()})
	if c.rankedView("too-big") != nil {
		t.Fatal("a list larger than the whole budget was kept")
	}
}
