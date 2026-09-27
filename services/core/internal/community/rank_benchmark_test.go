package community

import (
	"fmt"
	"testing"
	"time"
)

func BenchmarkRank500(b *testing.B) {
	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	items := make([]candidate, 500)
	for i := range items {
		items[i] = candidate{ID: fmt.Sprint(i), Author: fmt.Sprint(i % 100), Created: now.Add(-time.Duration(i%48) * time.Hour), Likes: i % 30, Affinity: i % 4}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if len(rank(append([]candidate(nil), items...), "for_you", true, now)) != len(items) {
			b.Fatal("ranking lost candidates")
		}
	}
}
