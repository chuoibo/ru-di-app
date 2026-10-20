package routes

import (
	"context"
	"os"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
)

// CatalogueCacheTTLEnv turns on the in-process catalogue snapshot. Unset or 0
// means every request reads the database, which is what tests and the parity
// harness need: they write a place and read it back in the next step.
const CatalogueCacheTTLEnv = "MOBILE_CATALOGUE_CACHE_TTL"

// catalogueSnapshot is one destination's list rows and photo summary: the
// part of GET /places that is the same for every caller. Filtering, taste
// scoring and ordering are cached next to it per (destination, taste,
// category): see rankedEntry.
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
	// ranked holds whole ordered lists of wired cards, built once per TTL for
	// a (destination, taste, category) and cut into pages per request.
	ranked  map[string]rankedEntry
	flight  singleflight.Group
	rflight singleflight.Group
	// covers is GET /destinations' cover photo per destination.
	coverMu sync.RWMutex
	cover   *coverEntry
	cflight singleflight.Group
	reasons reasonCache
}

// rankedEntry is the final order of one catalogue view. at is the snapshot's
// read time, so an entry can never outlive the rows it shows.
type rankedEntry struct {
	list []*pyjson.OrderedMap
	at   time.Time
}

type coverEntry struct {
	byDestination map[string]repo.PlacePhoto
	at            time.Time
}

var catalogue = &catalogueCache{
	byDest: map[string]*catalogueSnapshot{},
	ranked: map[string]rankedEntry{},
}

// maxRankedViews and maxRankedCards bound the ordered-list cache. An entry is
// a whole city's cards (9,514 for TP.HCM), so the card budget is what bounds
// memory: about six of the biggest lists. Free-text q is never cached, which
// keeps the key space to destination x taste x category.
const (
	maxRankedViews = 48
	maxRankedCards = 60000
)

// rankedView returns a cached ordered list for key, or nil. Only while the
// cache is on.
func (c *catalogueCache) rankedView(key string) []*pyjson.OrderedMap {
	ttl := c.configuredTTL()
	if ttl == 0 {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if hit, ok := c.ranked[key]; ok && time.Since(hit.at) < ttl {
		return hit.list
	}
	return nil
}

// keepRanked stores list under key, stamped with the snapshot it was built
// from. At the bound it drops expired entries first, then the oldest.
func (c *catalogueCache) keepRanked(key string, list []*pyjson.OrderedMap, snap *catalogueSnapshot) {
	ttl := c.configuredTTL()
	if ttl == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(list) > maxRankedCards {
		return
	}
	delete(c.ranked, key)
	cards := func() int {
		n := 0
		for _, e := range c.ranked {
			n += len(e.list)
		}
		return n
	}
	full := func() bool { return len(c.ranked) >= maxRankedViews || cards()+len(list) > maxRankedCards }
	if full() {
		for k, e := range c.ranked {
			if time.Since(e.at) >= ttl {
				delete(c.ranked, k)
			}
		}
		for len(c.ranked) > 0 && full() {
			oldest, oldestAt := "", time.Time{}
			for k, e := range c.ranked {
				if oldest == "" || e.at.Before(oldestAt) {
					oldest, oldestAt = k, e.at
				}
			}
			delete(c.ranked, oldest)
		}
	}
	c.ranked[key] = rankedEntry{list: list, at: snap.at}
}

// configuredTTL is MOBILE_CATALOGUE_CACHE_TTL read once as a Go duration
// ("30s", "2m"); unset, 0 or unreadable means the cache is off.
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
