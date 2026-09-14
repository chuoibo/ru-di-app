// Package storyvisibility ports app.domain.story_visibility (L4, ADR-0022
// §2.3): how long a 24-hour story lives, who sees it, and the order of the
// story rail.
//
// Parity, not correctness, is the contract (ADR-0029 §2.4): oracle_test.go
// replays testdata/python_*.json, rendered by
// scripts/render_domain_w2_goldens.py from the real module in the parity API
// image.
//
// # Values
//
// Python refuses a naive datetime with NAIVE_DATETIME; a time.Time always
// carries a location, so that refusal has no Go spelling and the functions
// that could raise it do not return an error. Datetimes should carry a fixed
// offset (UTC, or time.FixedZone), as every datetime the service handles
// does: Python adds a timedelta to the wall clock, Go to the instant, and the
// two agree only when the offset cannot change. The caption is `str | None`,
// the shape the service passes, so CAPTION_NOT_TEXT has no Go spelling either.
package storyvisibility

import (
	"cmp"
	"math/big"
	"slices"
	"time"
	"unicode/utf8"
)

// StoryTTL is STORY_TTL.
const StoryTTL = 24 * time.Hour

// DefaultStoryAudience is DEFAULT_STORY_AUDIENCE.
const DefaultStoryAudience = "friends"

// MaxCaptionLength is MAX_CAPTION_LENGTH, in code points.
const MaxCaptionLength = 200

// Codes StoryError carries. CodeNaiveDatetime and CodeCaptionNotText cannot
// arise from Go values; see the package comment.
const (
	CodeNaiveDatetime  = "NAIVE_DATETIME"
	CodeCaptionNotText = "CAPTION_NOT_TEXT"
	CodeCaptionTooLong = "CAPTION_TOO_LONG"
)

var storyAudiences = [...]string{DefaultStoryAudience}

// StoryAudiences returns STORY_AUDIENCES.
func StoryAudiences() []string { return slices.Clone(storyAudiences[:]) }

// StoryError is Python's StoryError: `str(exc)` is the code.
type StoryError struct {
	Code string
}

func (e *StoryError) Error() string { return e.Code }

// OverflowError is the OverflowError datetime arithmetic raises past year
// 9999.
type OverflowError struct {
	Message string
}

func (e *OverflowError) Error() string { return e.Message }

// maxYear is datetime.MAXYEAR.
const maxYear = 9999

// Story is the dict the service builds for this module (`_story_dict`).
type Story struct {
	AuthorID  string
	Audience  string
	ExpiresAt time.Time
}

// ExpiresAtFor is expires_at_for: created_at + 24 hours, in created_at's own
// offset. A wall clock that lands in year 10000 is Python's OverflowError.
func ExpiresAtFor(createdAt time.Time) (time.Time, error) {
	expires := createdAt.Add(StoryTTL)
	if expires.Year() > maxYear {
		return time.Time{}, &OverflowError{Message: "date value out of range"}
	}
	return expires, nil
}

// IsLive is is_live: strictly before the deadline.
func IsLive(story Story, now time.Time) bool {
	return story.ExpiresAt.After(now)
}

// CheckCaption is check_caption: nil, or at most MaxCaptionLength code points.
func CheckCaption(caption *string) error {
	if caption == nil {
		return nil
	}
	if utf8.RuneCountInString(*caption) > MaxCaptionLength {
		return &StoryError{Code: CodeCaptionTooLong}
	}
	return nil
}

// CanView is can_view: an unknown audience fails closed even for the author;
// then the author always; then not blocked, live, and a friend.
func CanView(story Story, readerID string, isFriend, isBlocked bool, now time.Time) bool {
	if !slices.Contains(storyAudiences[:], story.Audience) {
		return false
	}
	if readerID == story.AuthorID {
		return true
	}
	if isBlocked {
		return false
	}
	if !IsLive(story, now) {
		return false
	}
	return isFriend
}

// AuthorFacts are the three keys order_authors reads from one rail group.
type AuthorFacts struct {
	Mine     bool
	AllSeen  bool
	LatestAt time.Time
}

// OrderAuthors is order_authors: one's own first, then authors with something
// unseen, newest first within each, stable for ties. Python sorts on
// `-latest_at.timestamp()`, a float, so two instants a few microseconds apart
// far from the epoch can tie; the port sorts on the same float.
func OrderAuthors[G any](groups []G, facts func(G) AuthorFacts) []G {
	type keyed struct {
		group  G
		mine   int
		seen   int
		latest float64
	}
	rows := make([]keyed, len(groups))
	for i, group := range groups {
		f := facts(group)
		rows[i] = keyed{group: group, mine: 1, seen: 0, latest: -timestamp(f.LatestAt)}
		if f.Mine {
			rows[i].mine = 0
		}
		if f.AllSeen {
			rows[i].seen = 1
		}
	}
	slices.SortStableFunc(rows, func(a, b keyed) int {
		if c := cmp.Compare(a.mine, b.mine); c != 0 {
			return c
		}
		if c := cmp.Compare(a.seen, b.seen); c != 0 {
			return c
		}
		return cmp.Compare(a.latest, b.latest)
	})
	ordered := make([]G, len(rows))
	for i, row := range rows {
		ordered[i] = row.group
	}
	return ordered
}

// timestamp is datetime.timestamp() for an aware datetime: whole microseconds
// since the epoch, divided by a million with the correct rounding of CPython's
// int true division.
func timestamp(t time.Time) float64 {
	micros := t.Unix()*1_000_000 + int64(t.Nanosecond()/1_000)
	seconds, _ := new(big.Rat).SetFrac(big.NewInt(micros), big.NewInt(1_000_000)).Float64()
	return seconds
}
