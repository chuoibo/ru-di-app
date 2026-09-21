package repo

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Person is PersonRecord.
type Person struct {
	ID                  string
	DisplayName         string
	CreatedAt           time.Time
	Bio                 *string
	City                *string
	BudgetBand          *string
	WallCommentPolicy   string
	DiscoverableByPhone bool
	DeletedAt           *time.Time
}

// personColumns is what `session.get(Person, id)` selects: every mapped column
// in declaration order, notify_prefs included although PersonRecord drops it.
// Session.get labels them table_column.
const personColumns = `people.id AS people_id, people.display_name AS people_display_name,
       people.bio AS people_bio, people.city AS people_city, people.budget_band AS people_budget_band,
       people.discoverable_by_phone AS people_discoverable_by_phone, people.deleted_at AS people_deleted_at,
       people.notify_prefs AS people_notify_prefs, people.wall_comment_policy AS people_wall_comment_policy,
       people.created_at AS people_created_at`

func scanPerson(row pgx.Row) (*Person, error) {
	var p Person
	var notifyPrefs []byte
	err := row.Scan(&p.ID, &p.DisplayName, &p.Bio, &p.City, &p.BudgetBand,
		&p.DiscoverableByPhone, &p.DeletedAt, &notifyPrefs, &p.WallCommentPolicy, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.CreatedAt = p.CreatedAt.UTC()
	if p.DeletedAt != nil {
		deleted := p.DeletedAt.UTC()
		p.DeletedAt = &deleted
	}
	return &p, nil
}

// GetPerson is get_person. An erased account (deleted_at set) is still
// returned: the row stays for the ledger and callers decide what it means.
//
// SQLAlchemy note: session.get answers from the identity map without a
// statement when the row was already loaded in the same session. A request
// that reads a person twice issues this SELECT twice in Go.
func (r Repository) GetPerson(ctx context.Context, personID string) (*Person, error) {
	return scanPerson(r.Q.QueryRow(ctx,
		`SELECT `+personColumns+`
		   FROM people
		  WHERE people.id = $1::UUID`, personID))
}

// OptionalText is one nullable text field of a partial update: Set says the
// key is present in Python's `changes` dict, Value nil is its None.
type OptionalText struct {
	Set   bool
	Value *string
}

// SetText builds a present OptionalText.
func SetText(value *string) OptionalText { return OptionalText{Set: true, Value: value} }

// ProfileChanges is the `changes` dict the service hands update_person_profile.
// A nil pointer or an unset OptionalText is a key that is absent.
type ProfileChanges struct {
	DisplayName         *string
	Bio                 OptionalText
	City                OptionalText
	BudgetBand          OptionalText
	DiscoverableByPhone *bool
	WallCommentPolicy   *string
}

func sameText(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// UpdatePersonProfile is update_person_profile.
//
// SQLAlchemy behaviour reproduced:
//   - `session.get(Person, id, with_for_update=True)` always emits
//     SELECT ... FOR UPDATE (with_for_update bypasses the identity-map lookup),
//     so the row is locked even when nothing changes;
//   - setattr of a value equal to the loaded one is not a change: the flush
//     compares with `==` and leaves the column out, and a flush with no
//     changed column emits no UPDATE at all;
//   - the SET list follows the mapper's column order, not the dict's.
//
// Not reproduced, because it lives in the session and not in the method: when
// the same session loaded the person earlier (set_my_interests calls
// get_person first), SQLAlchemy keeps that earlier, unlocked copy's
// attributes and the returned record shows them for the fields not changed.
// Go returns the row as read under the lock.
func (r Repository) UpdatePersonProfile(ctx context.Context, personID string, changes ProfileChanges) (*Person, error) {
	person, err := scanPerson(r.Q.QueryRow(ctx,
		`SELECT `+personColumns+`
		   FROM people
		  WHERE people.id = $1::UUID FOR UPDATE`, personID))
	if err != nil || person == nil {
		return nil, err
	}
	var sets []string
	var args []any
	set := func(column, cast string, value any) {
		args = append(args, value)
		sets = append(sets, column+"=$"+strconv.Itoa(len(args))+cast)
	}
	if changes.DisplayName != nil && *changes.DisplayName != person.DisplayName {
		person.DisplayName = *changes.DisplayName
		set("display_name", "::VARCHAR", person.DisplayName)
	}
	if changes.Bio.Set && !sameText(changes.Bio.Value, person.Bio) {
		person.Bio = changes.Bio.Value
		set("bio", "::VARCHAR", person.Bio)
	}
	if changes.City.Set && !sameText(changes.City.Value, person.City) {
		person.City = changes.City.Value
		set("city", "::VARCHAR", person.City)
	}
	if changes.BudgetBand.Set && !sameText(changes.BudgetBand.Value, person.BudgetBand) {
		person.BudgetBand = changes.BudgetBand.Value
		set("budget_band", "::VARCHAR", person.BudgetBand)
	}
	if changes.DiscoverableByPhone != nil && *changes.DiscoverableByPhone != person.DiscoverableByPhone {
		person.DiscoverableByPhone = *changes.DiscoverableByPhone
		set("discoverable_by_phone", "", person.DiscoverableByPhone)
	}
	if changes.WallCommentPolicy != nil && *changes.WallCommentPolicy != person.WallCommentPolicy {
		person.WallCommentPolicy = *changes.WallCommentPolicy
		set("wall_comment_policy", "::VARCHAR", person.WallCommentPolicy)
	}
	if len(sets) == 0 {
		return person, nil
	}
	args = append(args, personID)
	if _, err := r.Q.Exec(ctx,
		`UPDATE people SET `+strings.Join(sets, ", ")+` WHERE people.id = $`+strconv.Itoa(len(args))+`::UUID`,
		args...); err != nil {
		return nil, err
	}
	return person, nil
}

// ListPersonInterests is list_person_interests: tags ordered by the database
// (the column's collation), which the service re-orders by vocabulary.
func (r Repository) ListPersonInterests(ctx context.Context, personID string) ([]string, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT person_interests.tag
		   FROM person_interests
		  WHERE person_interests.person_id = $1::UUID
		  ORDER BY person_interests.tag`, personID)
	if err != nil {
		return nil, err
	}
	tags, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	if tags == nil {
		tags = []string{}
	}
	return tags, nil
}

// SetPersonInterests is set_person_interests: replace the whole set, leaving
// the rows of tags that stay (their ids and created_at) untouched.
//
// SQLAlchemy behaviour reproduced:
//   - the rows are read without ORDER BY and without a lock;
//   - one flush writes the new rows first and deletes second (the unit of
//     work runs SaveUpdateAll before DeleteAll for one mapper);
//   - new rows go in sorted(wanted - current) order (code point order, which is
//     byte order for UTF-8), each with a client-side uuid4 and created_at=now,
//     never the column's server default, as ONE multi-row INSERT per 1000 rows
//     (SQLAlchemy 2.0's insertmanyvalues, insertmanyvalues_page_size);
//   - deleted rows go in primary-key order, one DELETE per row;
//   - a DELETE that matches no row (a concurrent request removed it first) is
//     only a warning in SQLAlchemy (confirm_deleted_rows without a version
//     column), so it is not an error here either;
//   - the answer is list_person_interests read back after the flush.
func (r Repository) SetPersonInterests(ctx context.Context, personID string, tags []string, now time.Time) ([]string, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT person_interests.id, person_interests.person_id, person_interests.tag, person_interests.created_at
		   FROM person_interests
		  WHERE person_interests.person_id = $1::UUID`, personID)
	if err != nil {
		return nil, err
	}
	type interestRow struct {
		id  string
		tag string
	}
	var existing []interestRow
	for rows.Next() {
		var row interestRow
		var person string
		var created time.Time
		if err := rows.Scan(&row.id, &person, &row.tag, &created); err != nil {
			rows.Close()
			return nil, err
		}
		existing = append(existing, row)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	current := map[string]bool{}
	for _, row := range existing {
		current[row.tag] = true
	}
	wanted := map[string]bool{}
	for _, tag := range tags {
		wanted[tag] = true
	}
	var leaving []string
	for _, row := range existing {
		if !wanted[row.tag] {
			leaving = append(leaving, row.id)
		}
	}
	var arriving []string
	for tag := range wanted {
		if !current[tag] {
			arriving = append(arriving, tag)
		}
	}
	sort.Strings(arriving)
	sort.Strings(leaving)

	created := pythonInstant(now)
	for start := 0; start < len(arriving); start += insertManyValuesPageSize {
		page := arriving[start:min(start+insertManyValuesPageSize, len(arriving))]
		values := make([]string, 0, len(page))
		args := make([]any, 0, 4*len(page))
		for _, tag := range page {
			id, err := newUUID()
			if err != nil {
				return nil, err
			}
			n := len(args)
			values = append(values, "($"+strconv.Itoa(n+1)+"::UUID, $"+strconv.Itoa(n+2)+"::UUID, $"+
				strconv.Itoa(n+3)+"::VARCHAR, $"+strconv.Itoa(n+4)+"::TIMESTAMP WITH TIME ZONE)")
			args = append(args, id, personID, tag, created)
		}
		if _, err := r.Q.Exec(ctx,
			`INSERT INTO person_interests (id, person_id, tag, created_at) VALUES `+strings.Join(values, ", "),
			args...); err != nil {
			return nil, err
		}
	}
	for _, id := range leaving {
		if _, err := r.Q.Exec(ctx,
			`DELETE FROM person_interests WHERE person_interests.id = $1::UUID`, id); err != nil {
			return nil, err
		}
	}
	return r.ListPersonInterests(ctx, personID)
}

// insertManyValuesPageSize is SQLAlchemy's default insertmanyvalues_page_size.
const insertManyValuesPageSize = 1000

// PersonTags is one entry of interests_by_person's dict, in insertion order.
type PersonTags struct {
	PersonID string
	Tags     []string
}

// InterestsByPerson is interests_by_person. People with no tags are absent,
// entries keep the dict's insertion order (person_id, then tag, as the
// database orders them), and an empty id list answers without a statement.
func (r Repository) InterestsByPerson(ctx context.Context, personIDs []string) ([]PersonTags, error) {
	out := []PersonTags{}
	if len(personIDs) == 0 {
		return out, nil
	}
	rows, err := r.Q.Query(ctx,
		`SELECT person_interests.person_id, person_interests.tag
		   FROM person_interests
		  WHERE person_interests.person_id IN (`+uuidPlaceholders(1, len(personIDs))+`)
		  ORDER BY person_interests.person_id, person_interests.tag`,
		uuidArgs(personIDs)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	index := map[string]int{}
	for rows.Next() {
		var person, tag string
		if err := rows.Scan(&person, &tag); err != nil {
			return nil, err
		}
		i, seen := index[person]
		if !seen {
			i = len(out)
			index[person] = i
			out = append(out, PersonTags{PersonID: person})
		}
		out[i].Tags = append(out[i].Tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
