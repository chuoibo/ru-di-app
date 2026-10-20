package routes

import (
	"context"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"mobile/services/core/internal/domain/taste"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
)

// CatalogueWarmEnv lists the destinations kept hot: read and ranked at start
// and again before their cache entries expire, so the biggest cities never
// answer a cold request. "off" turns it off; unset keeps the default. It does
// nothing while the cache itself is off (MOBILE_CATALOGUE_CACHE_TTL).
const CatalogueWarmEnv = "MOBILE_CATALOGUE_WARM"

// defaultWarm are the largest provinces: TP.HCM and Hà Nội.
var defaultWarm = []string{"d-tinh-79", "d-tinh-1"}

// rankPlaces is the whole ordered list a caller pages through: the rows of
// the snapshot that match category and q, scored for group, in the one order
// every page is cut from. The order is total (placeSortLess ends on the id),
// so two pages of one list never overlap or skip.
func rankPlaces(snap *catalogueSnapshot, category *string, q string, group taste.Profile) ([]*pyjson.OrderedMap, error) {
	selected := make([]repo.Place, 0, len(snap.rows))
	for _, row := range snap.rows {
		if category != nil && row.Category != *category {
			continue
		}
		if !placeMatches(q, row) {
			continue
		}
		selected = append(selected, row)
	}
	cards := cardsWithPhotos(selected, snap.covers, snap.counts)
	out := make([]*pyjson.OrderedMap, 0, len(cards))
	for _, card := range cards {
		wired, err := wirePlaceCard(card, nil, nil, group)
		if err != nil {
			return nil, err
		}
		out = append(out, wired)
	}
	sort.SliceStable(out, func(i, j int) bool { return placeSortLess(out[i], out[j]) })
	return out, nil
}

func rankKey(destinationID string, group taste.Profile, category *string) string {
	return destinationID + "\x00" + group.CacheKey() + "\x00" + deref(category)
}

// rankedFor returns the ordered list for a view, from the cache when the
// view can be cached (no free-text q) and the cache is on. Concurrent misses
// for one view share one build.
func (c *catalogueCache) rankedFor(snap *catalogueSnapshot, destinationID string, category *string, q string,
	group taste.Profile) ([]*pyjson.OrderedMap, error) {
	if q != "" || c.configuredTTL() == 0 {
		return rankPlaces(snap, category, q, group)
	}
	key := rankKey(destinationID, group, category)
	if hit := c.rankedView(key); hit != nil {
		return hit, nil
	}
	value, err, _ := c.rflight.Do(key, func() (any, error) {
		list, err := rankPlaces(snap, category, q, group)
		if err != nil {
			return nil, err
		}
		c.keepRanked(key, list, snap)
		return list, nil
	})
	if err != nil {
		return nil, err
	}
	return value.([]*pyjson.OrderedMap), nil
}

// refresh reads a destination unconditionally and replaces its snapshot; the
// warmer's way of renewing before expiry.
func (c *catalogueCache) refresh(ctx context.Context, store repo.Repository, destinationID string) (*catalogueSnapshot, error) {
	fresh, err := readCatalogue(ctx, store, destinationID)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.byDest[destinationID] = fresh
	c.mu.Unlock()
	return fresh, nil
}

// warmOnce renews the snapshot and the anonymous, all-category ordered list
// of each destination, and the destination covers.
func warmOnce(ctx context.Context, store repo.Repository, ids []string) {
	for _, id := range ids {
		snap, err := catalogue.refresh(ctx, store, id)
		if err != nil {
			slog.Warn("catalogue warm-up failed", "destination", id, "error", err.Error())
			continue
		}
		group := taste.Unknown()
		list, err := rankPlaces(snap, nil, "", group)
		if err != nil {
			slog.Warn("catalogue warm-up failed", "destination", id, "error", err.Error())
			continue
		}
		catalogue.keepRanked(rankKey(id, group, nil), list, snap)
	}
	if _, err := catalogue.destinationCovers(ctx, store, true); err != nil {
		slog.Warn("destination covers warm-up failed", "error", err.Error())
	}
}

// StartCatalogueWarmer keeps the biggest destinations hot until ctx ends. It
// returns at once: the first pass runs in the background so startup is never
// held by it, and a failed pass only means those requests read the database
// as they would have anyway.
func StartCatalogueWarmer(ctx context.Context, q repo.Querier) {
	ttl := catalogue.configuredTTL()
	if ttl == 0 {
		return
	}
	ids := defaultWarm
	if raw, set := os.LookupEnv(CatalogueWarmEnv); set {
		ids = nil
		if strings.TrimSpace(raw) != "off" {
			for _, id := range strings.Split(raw, ",") {
				if id = strings.TrimSpace(id); id != "" {
					ids = append(ids, id)
				}
			}
		}
	}
	if len(ids) == 0 {
		return
	}
	store := repo.Repository{Q: q}
	go func() {
		warmOnce(ctx, store, ids)
		// Renew at half the TTL: an entry is replaced well before it ages out.
		ticker := time.NewTicker(max(ttl/2, time.Second))
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				warmOnce(ctx, store, ids)
			}
		}
	}()
}

// destinationCovers is the cover photograph per destination, cached for the
// catalogue TTL. force renews it regardless of age.
func (c *catalogueCache) destinationCovers(ctx context.Context, store repo.Repository, force bool) (map[string]repo.PlacePhoto, error) {
	ttl := c.configuredTTL()
	if ttl > 0 && !force {
		c.coverMu.RLock()
		hit := c.cover
		c.coverMu.RUnlock()
		if hit != nil && time.Since(hit.at) < ttl {
			return hit.byDestination, nil
		}
	}
	read := func() (map[string]repo.PlacePhoto, error) {
		rows, err := store.DestinationCovers(context.WithoutCancel(ctx))
		if err != nil {
			return nil, err
		}
		if ttl > 0 {
			c.coverMu.Lock()
			c.cover = &coverEntry{byDestination: rows, at: time.Now()}
			c.coverMu.Unlock()
		}
		return rows, nil
	}
	if ttl == 0 || force {
		return read()
	}
	value, err, _ := c.cflight.Do("covers", func() (any, error) { return read() })
	if err != nil {
		return nil, err
	}
	return value.(map[string]repo.PlacePhoto), nil
}

// reasonCache remembers the model's sentence for a place under one taste:
// the key is the taste (taste.Profile.CacheKey) and the place id, which fixes
// the destination. Only answers are kept; "no answer" is asked again.
type reasonCache struct {
	mu     sync.Mutex
	m      map[string]reasonStamp
	flight singleflight.Group
}

type reasonStamp struct {
	pair reasonPair
	at   time.Time
}

const (
	// reasonTTL is how long a written reason is served again. It is far longer
	// than the catalogue TTL: a model call costs money and seconds, and the
	// sentence only changes when the place's text does.
	reasonTTL = 30 * time.Minute
	// maxCachedReasons bounds the map; a full one starts over.
	maxCachedReasons = 20000
)

func reasonKey(group taste.Profile, id string) string { return group.CacheKey() + "\x00" + id }

func (r *reasonCache) get(key string, enabled bool) (reasonPair, bool) {
	if !enabled {
		return reasonPair{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	hit, ok := r.m[key]
	if !ok || time.Since(hit.at) >= reasonTTL {
		return reasonPair{}, false
	}
	return hit.pair, true
}

func (r *reasonCache) put(key string, pair reasonPair, enabled bool) {
	if !enabled {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.m == nil || len(r.m) >= maxCachedReasons {
		r.m = map[string]reasonStamp{}
	}
	r.m[key] = reasonStamp{pair: pair, at: time.Now()}
}
