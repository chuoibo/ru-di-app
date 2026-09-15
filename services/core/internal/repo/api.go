package repo

// The pilot wave's slice of SqlAlchemyApiRepository
// (services/api/app/api/repository.py). Each method issues the statements the
// Python method issues, in the same order, and returns what the Python method
// returns. Where SQLAlchemy does something a reader of the Python would not
// guess, the Go method says so next to the statement that reproduces it.
//
// Conventions shared by every method here:
//   - ids travel as canonical lowercase UUID strings, both ways;
//   - timestamptz columns come back as time.Time with PostgreSQL's microsecond
//     precision; DATE columns come back as midnight UTC of that calendar day;
//   - a Python `None` is a nil pointer (or a nil json.RawMessage), never a zero
//     value, so NULL and "" stay different;
//   - PostgreSQL errors are returned untouched (*pgconn.PgError), because the
//     Python method lets IntegrityError and friends propagate to a 500.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // WallClockDate must not depend on the host's zoneinfo.

	"github.com/jackc/pgx/v5"
)

// Repository is SqlAlchemyApiRepository over one request's transaction.
type Repository struct {
	Q Querier
}

// WallClockZone is repository.py's WALL_CLOCK_ZONE: trip days are Vietnam's
// calendar days.
const WallClockZone = "Asia/Ho_Chi_Minh"

// wallClockDate is `_wall_clock_date(column)`: SQLAlchemy renders
// cast(func.timezone(WALL_CLOCK_ZONE, column), Date) with the zone as a bound
// VARCHAR parameter, so the fold happens in PostgreSQL whatever TimeZone the
// session carries.
func wallClockDate(zoneParam, column string) string {
	return "CAST(timezone(" + zoneParam + "::VARCHAR, " + column + ") AS DATE)"
}

var wallClockLocation = func() *time.Location {
	location, err := time.LoadLocation(WallClockZone)
	if err != nil {
		panic(err)
	}
	return location
}()

// WallClockDate is how the service computes the `today` it hands group_recap:
// `_now().astimezone(ZoneInfo(WALL_CLOCK_ZONE)).date()`. The result is that
// calendar day at midnight UTC, the shape every DATE in this package has.
func WallClockDate(instant time.Time) time.Time {
	year, month, day := instant.In(wallClockLocation).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// calendarDay drops the clock and the location of a date argument, keeping
// the calendar fields as written (a Python `date` has nothing else).
func calendarDay(day time.Time) time.Time {
	year, month, date := day.Date()
	return time.Date(year, month, date, 0, 0, 0, 0, time.UTC)
}

// pythonInstant is what a Python `datetime` can hold: microseconds, rounded
// toward the past exactly as datetime.now() and pgx's binary encoding both do.
// Returning the truncated value keeps the record equal to the stored row.
func pythonInstant(instant time.Time) time.Time {
	return instant.Truncate(time.Microsecond)
}

// NewUUID is uuid.uuid4() for a route that mints an id itself, as
// ApiService.report_payment mints a missing idempotency key.
func NewUUID() (string, error) { return newUUID() }

// newUUID is uuid.uuid4(), the client-side default SQLAlchemy runs at flush.
func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	var out [36]byte
	hex.Encode(out[0:8], b[0:4])
	out[8] = '-'
	hex.Encode(out[9:13], b[4:6])
	out[13] = '-'
	hex.Encode(out[14:18], b[6:8])
	out[18] = '-'
	hex.Encode(out[19:23], b[8:10])
	out[23] = '-'
	hex.Encode(out[24:], b[10:])
	return string(out[:]), nil
}

// placeholders renders SQLAlchemy's expanded IN list ("IN (__[POSTCOMPILE_x])"
// becomes one bound parameter per element) starting at $first.
func placeholders(first, count int) string {
	var b strings.Builder
	for i := 0; i < count; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("$")
		b.WriteString(strconv.Itoa(first + i))
	}
	return b.String()
}

// uuidPlaceholders is placeholders with the ::UUID bind cast the psycopg
// dialect renders on every UUID parameter.
func uuidPlaceholders(first, count int) string {
	var b strings.Builder
	for i := 0; i < count; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("$")
		b.WriteString(strconv.Itoa(first + i))
		b.WriteString("::UUID")
	}
	return b.String()
}

func uuidArgs(ids []string) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

// ErrPythonTypeError marks a stored value the Python method would have raised
// TypeError on while building its record.
var ErrPythonTypeError = errors.New("repo: python would raise TypeError on this stored value")

// IsMember is is_member: an ACTIVE membership row whose left_at is NULL. It
// does not look at people.deleted_at or at the context's existence.
func (r Repository) IsMember(ctx context.Context, contextID, personID string) (bool, error) {
	var id string
	err := r.Q.QueryRow(ctx,
		`SELECT memberships.id
		   FROM memberships
		  WHERE memberships.context_id = $1::UUID
		    AND memberships.person_id = $2::UUID
		    AND memberships.state = $3
		    AND memberships.left_at IS NULL
		  LIMIT $4::INTEGER`,
		contextID, personID, "active", 1).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
