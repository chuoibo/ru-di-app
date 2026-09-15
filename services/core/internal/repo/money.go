package repo

// Shared machinery of the W4 slice of SqlAlchemyApiRepository (the money
// routes): expenses.go, bills.go, batches.go and receipts.go.
//
// Three SQLAlchemy behaviours every write here reproduces:
//
//   - Flush order. None of the money models declares a relationship(), so the
//     unit of work has no dependency between mappers and emits a flush's
//     pending rows mapper by mapper in the order of the mapped class names
//     (AuditEvent < BillDiscount < BillItemShare < BillSurcharge <
//     CollectionBatch < CollectionEnvelope < CollectionObligation <
//     CollectionObligationSource < ConfirmedAllocation < ExpenseDiscount <
//     ExpenseItemShare < ExpenseSurcharge), UPDATEs of a mapper before its
//     INSERTs. Each method below lists its flushes in that order.
//   - Batching. Several new rows of one table whose primary key comes from a
//     Python default (uuid4) go out as insertmanyvalues: one multi-row INSERT
//     per page of 1000. Rows whose primary key is given in full
//     (collection_obligation_sources) and DELETEs by primary key go out as a
//     true executemany: one statement per row.
//   - Bind-time refusals. An enum column (native_enum=False,
//     validate_strings=True) refuses a string outside its values while the
//     statement's parameters are processed, before anything is sent: the
//     statement is never issued and Python raises StatementError.

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Errors the Python money methods raise themselves rather than from PostgreSQL.
var (
	// ErrUnknownPayerAcknowledgement is `PayerAcknowledgement(value)` refusing (ValueError).
	ErrUnknownPayerAcknowledgement = errors.New("repo: not a payer acknowledgement")
	// ErrUnknownVerificationScope is `VerificationScope(value)` refusing (ValueError).
	ErrUnknownVerificationScope = errors.New("repo: not a verification scope")
	// ErrUnknownBatchStatus is `CollectionBatchStatus(value)` refusing (ValueError).
	ErrUnknownBatchStatus = errors.New("repo: not a collection batch status")
	// ErrNotAnEnumValue is the StatementError an Enum column's bind processor
	// raises for a string outside its values; the statement is not issued.
	ErrNotAnEnumValue = errors.New("repo: value is not among the enum's values")
	// ErrEventDataNotAnObject is the AttributeError `event_data.get(...)`
	// raises when an audit event's JSON is not an object.
	ErrEventDataNotAnObject = errors.New("repo: audit event_data is not a JSON object")
	// ErrDisputeReasonNotText marks a stored objection reason that is JSON but
	// not a string. Python carries the value into BatchObligationRow and the
	// route's response model refuses it; a Go record cannot hold it.
	ErrDisputeReasonNotText = errors.New("repo: stored objection reason is not text")
)

var (
	payerAcknowledgements = map[string]bool{"pending": true, "acknowledged": true, "disputed": true}
	verificationScopes    = map[string]bool{"totals_only": true, "items_reviewed": true}
	surchargeModes        = map[string]bool{"proportional": true, "even": true}
	discountScopes        = map[string]bool{"global_proportional": true, "item": true}
	batchStatuses         = map[string]bool{"accruing": true, "frozen": true, "published": true, "collecting": true,
		"completed": true, "closed_with_exceptions": true, "cancelled": true}
)

// sqlInteger binds a Python int to an INTEGER parameter. psycopg sends any
// int and PostgreSQL refuses one outside INTEGER with 22003; pgx would refuse
// it in the client instead, so such a value travels as text (pgx sends a Go
// string in text format) for the server to refuse the same way.
func sqlInteger(n int64) any {
	if n >= math.MinInt32 && n <= math.MaxInt32 {
		return int32(n)
	}
	return strconv.FormatInt(n, 10)
}

// insertColumn is one column of an INSERT and the bind cast the psycopg
// dialect renders on its parameter ("" for none).
type insertColumn struct {
	name, cast string
}

func renderInsert(table string, columns []insertColumn, rows int) string {
	var b strings.Builder
	b.WriteString("INSERT INTO " + table + " (")
	for i, c := range columns {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(c.name)
	}
	b.WriteString(") VALUES ")
	n := 0
	for row := 0; row < rows; row++ {
		if row > 0 {
			b.WriteString(", ")
		}
		b.WriteString("(")
		for i, c := range columns {
			if i > 0 {
				b.WriteString(", ")
			}
			n++
			b.WriteString("$" + strconv.Itoa(n) + c.cast)
		}
		b.WriteString(")")
	}
	return b.String()
}

// insertManyValues is the flush of new rows of one uuid4-keyed table: one
// INSERT per page of insertManyValuesPageSize rows, rows in the order they
// were added; nothing for no rows.
func (r Repository) insertManyValues(ctx context.Context, table string, columns []insertColumn, rows [][]any) error {
	for start := 0; start < len(rows); start += insertManyValuesPageSize {
		page := rows[start:min(start+insertManyValuesPageSize, len(rows))]
		args := make([]any, 0, len(page)*len(columns))
		for _, row := range page {
			args = append(args, row...)
		}
		if _, err := r.Q.Exec(ctx, renderInsert(table, columns, len(page)), args...); err != nil {
			return err
		}
	}
	return nil
}

// insertEach is a true executemany: one single-row INSERT per row.
func (r Repository) insertEach(ctx context.Context, table string, columns []insertColumn, rows [][]any) error {
	sql := renderInsert(table, columns, 1)
	for _, row := range rows {
		if _, err := r.Q.Exec(ctx, sql, row...); err != nil {
			return err
		}
	}
	return nil
}

// auditEvent is one AuditEvent the methods add.
type auditEvent struct {
	actorID       *string
	eventType     string
	aggregateType string
	aggregateID   string
	requestID     *string
	eventData     map[string]any
	occurredAt    time.Time
}

var auditColumns = []insertColumn{{"id", "::UUID"}, {"actor_id", "::UUID"}, {"event_type", "::VARCHAR"},
	{"aggregate_type", "::VARCHAR"}, {"aggregate_id", "::UUID"}, {"request_id", "::UUID"}, {"event_data", "::JSONB"},
	{"occurred_at", "::TIMESTAMP WITH TIME ZONE"}}

// insertAudit is the flush of one AuditEvent: every column listed, event_data
// as JSON text (JSONB keeps the value, not the spelling).
func (r Repository) insertAudit(ctx context.Context, e auditEvent) error {
	id, err := newUUID()
	if err != nil {
		return err
	}
	data, err := json.Marshal(e.eventData)
	if err != nil {
		return err
	}
	_, err = r.Q.Exec(ctx, renderInsert("audit_events", auditColumns, 1),
		id, e.actorID, e.eventType, e.aggregateType, e.aggregateID, e.requestID, string(data), e.occurredAt)
	return err
}

// wholeNumber reads one numeric (a SUM of BIGINT, coalesced) as the exact
// integer Python's int() makes of it.
func wholeNumber(row pgx.Row) (*big.Int, error) {
	var n pgtype.Numeric
	if err := row.Scan(&n); err != nil {
		return nil, err
	}
	return numericInteger(n)
}

// deleteByID is the flush of `session.delete(row)` for each row: a true
// executemany of DELETE by primary key, rows in primary key order (the unit
// of work sorts persistent states by identity), each matching one row.
func (r Repository) deleteByID(ctx context.Context, table string, ids []string) error {
	sorted := append([]string{}, ids...)
	sortStrings(sorted)
	for _, id := range sorted {
		tag, err := r.Q.Exec(ctx, `DELETE FROM `+table+` WHERE `+table+`.id = $1::UUID`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrStaleUpdate
		}
	}
	return nil
}
