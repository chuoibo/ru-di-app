package repo

// Ending an account (ADR-0023 §2.1): erase_person and
// revoke_all_account_sessions of SqlAlchemyApiRepository. The policy is
// app.domain.account_lifecycle.ERASURE; this file is the repository's
// spelling of it as statements, in the repository's order.

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"mobile/services/core/internal/domain/accountlifecycle"
)

// The branches of accountlifecycle.Erasure() this file writes statements for.
// ERASURE is the policy (ADR-0023 §2.1) and the Python repository reads the
// names and the anonymised values through app.domain.account_lifecycle; so
// does this file, rather than spelling either of them a second time.
var (
	erasureDeleteTables   = erasureTablesFor(accountlifecycle.ActionDelete)
	erasureRevokeTable    = erasureOnlyTable(accountlifecycle.ActionRevoke)
	erasureLeaveTable     = erasureOnlyTable(accountlifecycle.ActionLeave)
	erasureAnonymiseTable = erasureOnlyTable(accountlifecycle.ActionAnonymise)
)

func erasureTablesFor(action string) []string {
	tables, err := accountlifecycle.TablesFor(action)
	if err != nil {
		panic(err)
	}
	return tables
}

// erasureOnlyTable is the one table of a branch that names one. The actions
// are the domain's own constants, so anything else is a programming error
// here, not a fact about a request.
func erasureOnlyTable(action string) string {
	tables := erasureTablesFor(action)
	if len(tables) != 1 {
		panic("repo: " + action + " does not name exactly one table")
	}
	return tables[0]
}

// ErrErasureMapChanged is what ErasePerson answers when
// accountlifecycle.Erasure() and the statements below no longer name the same
// tables: the map is the policy, so a repository that cannot spell it refuses
// rather than ending an account by a stale list of its own.
var ErrErasureMapChanged = errors.New("repo: the erasure map and the repository name different tables")

// ErasureCount is one entry of ErasureReport.counts, in insertion order.
type ErasureCount struct {
	Table string
	Rows  int64
}

// ErasureReport is ErasureReport: how many rows went from each table, and the
// storage keys of the photographs whose rows went, for the caller to unlink.
type ErasureReport struct {
	Counts      []ErasureCount
	StorageKeys []string
}

// RevokeAllAccountSessions is revoke_all_account_sessions: every session of
// the person whose revoked_at is NULL (no lock, no ORDER BY), then the flush,
// one `UPDATE ... SET revoked_at` per row in primary key order. Answers how
// many there were.
func (r Repository) RevokeAllAccountSessions(ctx context.Context, personID string, now time.Time) (int64, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT account_sessions.id, account_sessions.person_id, account_sessions.token_digest,
		        account_sessions.issued_from_invite_id, account_sessions.issued_via, account_sessions.created_at,
		        account_sessions.expires_at, account_sessions.revoked_at
		   FROM account_sessions
		  WHERE account_sessions.person_id = $1::UUID AND account_sessions.revoked_at IS NULL`, personID)
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id, person, via string
		var digest []byte
		var invite *string
		var created, expires time.Time
		var revoked *time.Time
		if err := rows.Scan(&id, &person, &digest, &invite, &via, &created, &expires, &revoked); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	sortStrings(ids)
	at := pythonInstant(now)
	for _, id := range ids {
		if err := r.execUpdate(ctx,
			`UPDATE account_sessions SET revoked_at=$1::TIMESTAMP WITH TIME ZONE WHERE account_sessions.id = $2::UUID`,
			at, id); err != nil {
			return 0, err
		}
	}
	return int64(len(ids)), nil
}

// erasureWipe is one `wipe(model, *conditions)` of erase_person: an ORM
// DELETE whose person id is bound `binds` times. SQLAlchemy's
// synchronize_session="auto" evaluates a criterion it can in Python and
// fetches the deleted primary keys with RETURNING otherwise (an IN
// subquery), which changes the statement text but not the row count. For
// story_views, whose primary key has two columns, SQLAlchemy lists them in an
// order that changes from one Python process to the next; the rows are only
// counted, so Go fixes one order.
type erasureWipe struct {
	table string
	sql   string
	binds int
}

var erasureWipes, erasureWipesSpellTheMap = buildErasureWipes()

// buildErasureWipes renders the fourteen DELETEs in the repository's order --
// children before the rows they point at, which is not the map's order -- from
// the map's own names. A name the map no longer has renders empty and answers
// false, as does a map naming a table no statement here wipes; ErasePerson
// reads that answer before its first statement.
func buildErasureWipes() ([]erasureWipe, bool) {
	named := map[string]bool{}
	for _, table := range erasureDeleteTables {
		named[table] = true
	}
	spellsTheMap := true
	table := func(name string) string {
		if !named[name] {
			spellsTheMap = false
			return ""
		}
		return name
	}
	// mine is `DELETE FROM t WHERE t.<column> = $1::UUID`, which
	// synchronize_session="auto" evaluates in Python, so it has no RETURNING.
	mine := func(name, column string) erasureWipe {
		t := table(name)
		return erasureWipe{table: t, binds: 1,
			sql: `DELETE FROM ` + t + ` WHERE ` + t + `.` + column + ` = $1::UUID`}
	}
	// eitherEnd is the same for a row that names the person at either end.
	eitherEnd := func(name, first, second string) erasureWipe {
		t := table(name)
		return erasureWipe{table: t, binds: 2,
			sql: `DELETE FROM ` + t + ` WHERE ` + t + `.` + first + ` = $1::UUID
		OR ` + t + `.` + second + ` = $2::UUID`}
	}
	// underMine is a child of the person's own rows: theirs by `column`, or
	// pointing through `parentColumn` at a `parent` row they authored. The
	// evaluator cannot read that subquery, so SQLAlchemy fetches the keys of
	// the deleted rows with RETURNING.
	underMine := func(name, column, parent, parentColumn string, returning ...string) erasureWipe {
		t := table(name)
		keys := make([]string, len(returning))
		for i, key := range returning {
			keys[i] = t + `.` + key
		}
		return erasureWipe{table: t, binds: 2,
			sql: `DELETE FROM ` + t + ` WHERE ` + t + `.` + column + ` = $1::UUID
		OR ` + t + `.` + parentColumn + ` IN (SELECT ` + parent + `.id FROM ` + parent + ` WHERE ` + parent + `.author_id = $2::UUID)
		RETURNING ` + strings.Join(keys, ", ")}
	}
	posts, stories := table("posts"), table("stories")
	wipes := []erasureWipe{
		underMine("post_comments", "author_id", posts, "post_id", "id"),
		underMine("post_reactions", "person_id", posts, "post_id", "id"),
		underMine("story_views", "viewer_id", stories, "story_id", "viewer_id", "story_id"),
		mine("posts", "author_id"),
		mine("stories", "author_id"),
		mine("uploaded_images", "owner_person_id"),
		mine("person_interests", "person_id"),
		mine("saved_places", "person_id"),
		mine("context_read_marks", "person_id"),
		mine("pair_paper_views", "person_id"),
		mine("pair_shared_constraints", "owner_id"),
		mine("active_couple_members", "person_id"),
		mine("account_identities", "person_id"),
		eitherEnd("friend_requests", "requester_id", "addressee_id"),
	}
	return wipes, spellsTheMap && len(wipes) == len(erasureDeleteTables)
}

// ErasePerson is erase_person: end one account inside the caller's
// transaction. Photograph files are NOT touched: their keys come back in the
// report, in the order the first statement read them.
//
// Statements, in Python's order:
//  1. the person's uploaded_images (every column, no ORDER BY, no lock);
//  2. the fourteen DELETEs of erasureWipes, in that order, children before the
//     rows they point at; no money table is among them;
//  3. RevokeAllAccountSessions;
//  4. the person's memberships with left_at NULL, every column, FOR UPDATE;
//  5. the autoflush of those memberships (state left, left_at now, one UPDATE
//     each in primary key order), issued by the next statement's execute;
//  6. the people row FOR UPDATE; none is Conflict PERSON_NOT_FOUND, raised
//     from nothing AFTER every statement above (the caller's transaction must
//     roll them back);
//  7. the flush of anonymised_person: an UPDATE of the columns whose value
//     changes (display_name, bio, city, budget_band, discoverable_by_phone,
//     deleted_at, wall_comment_policy, in mapper order), none when an account
//     erased at the same instant is erased again;
//  8. the INSERT of the account.deleted audit event, actor and aggregate the
//     person, event_data {"counts": counts} and occurred_at now.
//
// counts carries each wiped table's row count, then account_sessions, then
// memberships (the rows found in 4, set before 6 can refuse); the three names
// are the map's. Before statement 1, a repository whose statements no longer
// name the map's `delete` tables answers ErrErasureMapChanged and writes
// nothing.
func (r Repository) ErasePerson(ctx context.Context, personID string, now time.Time) (ErasureReport, error) {
	if !erasureWipesSpellTheMap {
		return ErasureReport{}, ErrErasureMapChanged
	}
	at := pythonInstant(now)
	report := ErasureReport{Counts: []ErasureCount{}, StorageKeys: []string{}}
	// Every wiped table appears once, so `counts.get(table, 0) + rowcount` is
	// the row count itself.
	count := func(table string, rows int64) {
		report.Counts = append(report.Counts, ErasureCount{Table: table, Rows: rows})
	}

	images, err := r.Q.Query(ctx,
		`SELECT uploaded_images.id, uploaded_images.storage_key, uploaded_images.context_id,
		        uploaded_images.owner_person_id, uploaded_images.uploaded_by_id, uploaded_images.purpose,
		        uploaded_images.content_type, uploaded_images.byte_size, uploaded_images.width, uploaded_images.height,
		        uploaded_images.created_at
		   FROM uploaded_images
		  WHERE uploaded_images.owner_person_id = $1::UUID`, personID)
	if err != nil {
		return ErasureReport{}, err
	}
	for images.Next() {
		var m UploadedImage
		if err := images.Scan(&m.ID, &m.StorageKey, &m.ContextID, &m.OwnerPersonID, &m.UploadedByID, &m.Purpose,
			&m.ContentType, &m.ByteSize, &m.Width, &m.Height, &m.CreatedAt); err != nil {
			images.Close()
			return ErasureReport{}, err
		}
		report.StorageKeys = append(report.StorageKeys, m.StorageKey)
	}
	images.Close()
	if err := images.Err(); err != nil {
		return ErasureReport{}, err
	}

	for _, wipe := range erasureWipes {
		args := make([]any, wipe.binds)
		for i := range args {
			args[i] = personID
		}
		if strings.Contains(wipe.sql, "RETURNING") {
			rows, err := r.Q.Query(ctx, wipe.sql, args...)
			if err != nil {
				return ErasureReport{}, err
			}
			var n int64
			for rows.Next() {
				n++
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return ErasureReport{}, err
			}
			count(wipe.table, n)
			continue
		}
		tag, err := r.Q.Exec(ctx, wipe.sql, args...)
		if err != nil {
			return ErasureReport{}, err
		}
		count(wipe.table, tag.RowsAffected())
	}

	revoked, err := r.RevokeAllAccountSessions(ctx, personID, at)
	if err != nil {
		return ErasureReport{}, err
	}
	report.Counts = append(report.Counts, ErasureCount{Table: erasureRevokeTable, Rows: revoked})

	memberships, err := r.Q.Query(ctx,
		`SELECT memberships.id, memberships.context_id, memberships.person_id, memberships.state, memberships.role,
		        memberships.origin, memberships.invited_by_id, memberships.joined_at, memberships.left_at,
		        memberships.created_at
		   FROM memberships
		  WHERE memberships.person_id = $1::UUID AND memberships.left_at IS NULL FOR UPDATE`, personID)
	if err != nil {
		return ErasureReport{}, err
	}
	var leaving []string
	for memberships.Next() {
		var m Membership
		if err := memberships.Scan(&m.ID, &m.ContextID, &m.PersonID, &m.State, &m.Role, &m.Origin, &m.InvitedByID,
			&m.JoinedAt, &m.LeftAt, &m.CreatedAt); err != nil {
			memberships.Close()
			return ErasureReport{}, err
		}
		leaving = append(leaving, m.ID)
	}
	memberships.Close()
	if err := memberships.Err(); err != nil {
		return ErasureReport{}, err
	}
	report.Counts = append(report.Counts, ErasureCount{Table: erasureLeaveTable, Rows: int64(len(leaving))})
	sortStrings(leaving)
	for _, id := range leaving {
		if err := r.execUpdate(ctx,
			`UPDATE memberships SET state=$1, left_at=$2::TIMESTAMP WITH TIME ZONE WHERE memberships.id = $3::UUID`,
			"left", at, id); err != nil {
			return ErasureReport{}, err
		}
	}

	person, err := scanPerson(r.Q.QueryRow(ctx,
		`SELECT `+personColumns+`
		   FROM people
		  WHERE people.id = $1::UUID FOR UPDATE`, personID))
	if err != nil {
		return ErasureReport{}, err
	}
	if person == nil {
		return ErasureReport{}, &Conflict{Code: "PERSON_NOT_FOUND"}
	}
	if err := r.anonymise(ctx, person, at); err != nil {
		return ErasureReport{}, err
	}

	counts := map[string]any{}
	for _, c := range report.Counts {
		counts[c.Table] = c.Rows
	}
	actor := personID
	if err := r.insertAudit(ctx, auditEvent{actorID: &actor, eventType: "account.deleted", aggregateType: "person",
		aggregateID: personID, eventData: map[string]any{"counts": counts}, occurredAt: at}); err != nil {
		return ErasureReport{}, err
	}
	return report, nil
}

// anonymise is the flush of anonymised_person's values set on the loaded row.
// The values are the domain's: the six keys the service hands
// account_lifecycle.anonymised_person go in, and what comes back is written.
// The SET list stays in the mapper's column order (deleted_at before
// wall_comment_policy, which is not the dict's order), and a column whose
// value does not change is left out, exactly as the flush leaves it out.
func (r Repository) anonymise(ctx context.Context, p *Person, at time.Time) error {
	wanted := map[string]any{}
	for _, column := range accountlifecycle.AnonymisedPerson([]accountlifecycle.Column{
		{Name: "display_name", Value: p.DisplayName},
		{Name: "bio", Value: p.Bio},
		{Name: "city", Value: p.City},
		{Name: "budget_band", Value: p.BudgetBand},
		{Name: "discoverable_by_phone", Value: p.DiscoverableByPhone},
		{Name: "wall_comment_policy", Value: p.WallCommentPolicy},
	}, at) {
		wanted[column.Name] = column.Value
	}
	var sets []string
	var args []any
	set := func(column, cast string, value any) {
		args = append(args, value)
		sets = append(sets, column+"=$"+strconv.Itoa(len(args))+cast)
	}
	// cleared is one of the three columns the map empties: NULL when the
	// domain says so, and the domain's text otherwise.
	cleared := func(column string, stored *string) {
		switch value := wanted[column].(type) {
		case nil:
			if stored != nil {
				set(column, "::VARCHAR", nil)
			}
		case string:
			if stored == nil || *stored != value {
				set(column, "::VARCHAR", value)
			}
		}
	}
	if name, ok := wanted["display_name"].(string); ok && p.DisplayName != name {
		set("display_name", "::VARCHAR", name)
	}
	cleared("bio", p.Bio)
	cleared("city", p.City)
	cleared("budget_band", p.BudgetBand)
	if discoverable, ok := wanted["discoverable_by_phone"].(bool); ok && p.DiscoverableByPhone != discoverable {
		set("discoverable_by_phone", "", discoverable)
	}
	if deleted, ok := wanted["deleted_at"].(time.Time); ok && (p.DeletedAt == nil || !p.DeletedAt.Equal(deleted)) {
		set("deleted_at", "::TIMESTAMP WITH TIME ZONE", deleted)
	}
	if policy, ok := wanted["wall_comment_policy"].(string); ok && p.WallCommentPolicy != policy {
		set("wall_comment_policy", "::VARCHAR", policy)
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, p.ID)
	return r.execUpdate(ctx,
		`UPDATE `+erasureAnonymiseTable+` SET `+strings.Join(sets, ", ")+` WHERE `+erasureAnonymiseTable+
			`.id = $`+strconv.Itoa(len(args))+`::UUID`, args...)
}
