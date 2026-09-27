package routes

import (
	"context"
	"os"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"mobile/services/core/internal/repo"
)

// CatalogueCacheTTLEnv turns on the in-process catalogue snapshot. Unset or 0
// means every request reads the database, which is what tests and the parity
// harness need: they write a place and read it back in the next step.
const CatalogueCacheTTLEnv = "MOBILE_CATALOGUE_CACHE_TTL"

// catalogueSnapshot is one destination's list rows and photo summary: the
// part of GET /places that is the same for every caller. Filtering, taste
// scoring and reasons stay per request.
type catalogueSnapshot struct {
	rows   []repo.Place
	covers map[string]repo.PlacePhoto
	counts map[string]int64
	at     time.Time
}

type catalogueCache struct {
	once   sync.Once
	ttl    time.Duration
	mu     sync.RWMutex
	byDest map[string]*catalogueSnapshot
	flight singleflight.Group
}

var catalogue = &catalogueCache{byDest: map[string]*catalogueSnapshot{}}

func (c *catalogueCache) configuredTTL() time.Duration {
	c.once.Do(func() {
		if d, err := time.ParseDuration(os.Getenv(CatalogueCacheTTLEnv)); err == nil && d > 0 {
			c.ttl = d
		}
	})
	return c.ttl
}

// load returns the destination's snapshot, reading the database at most once
// per TTL per destination.
//
// Concurrent misses for one destination share one read (singleflight): when a
// snapshot expires under load, a thousand waiting requests must not become a
// thousand copies of the same 5,000-row query. The catalogue changes only when
// `rudi-ingest sync` applies a batch (every ~10 minutes), so a TTL of tens of
// seconds is invisible to people and removes nearly all catalogue reads.
func (c *catalogueCache) load(ctx context.Context, store repo.Repository, destinationID string) (*catalogueSnapshot, error) {
	ttl := c.configuredTTL()
	if ttl == 0 {
		return readCatalogue(ctx, store, destinationID)
	}
	c.mu.RLock()
	snap := c.byDest[destinationID]
	c.mu.RUnlock()
	if snap != nil && time.Since(snap.at) < ttl {
		return snap, nil
	}
	value, err, _ := c.flight.Do(destinationID, func() (any, error) {
		// The request that wins the flight must not be the one whose
		// cancellation fails everyone waiting on it.
		fresh, err := readCatalogue(context.WithoutCancel(ctx), store, destinationID)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.byDest[destinationID] = fresh
		c.mu.Unlock()
		return fresh, nil
	})
	if err != nil {
		return nil, err
	}
	return value.(*catalogueSnapshot), nil
}

func readCatalogue(ctx context.Context, store repo.Repository, destinationID string) (*catalogueSnapshot, error) {
	rows, err := store.ListPlaceCards(ctx, repo.PlaceFilter{DestinationID: &destinationID})
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	covers, counts, err := store.PhotoCoversAndCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	return &catalogueSnapshot{rows: rows, covers: covers, counts: counts, at: time.Now()}, nil
}
