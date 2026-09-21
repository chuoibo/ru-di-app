package repo

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// VoteOption is VoteOptionRecord.
type VoteOption struct {
	ID        string
	VoteID    string
	Position  int64
	Label     string
	PlaceName *string
}

// VoteBallot is VoteBallotRecord.
type VoteBallot struct {
	ID        string
	VoteID    string
	OptionID  string
	VoterID   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Vote is VoteRecord. Options and Ballots are never nil.
type Vote struct {
	ID          string
	ContextID   string
	OutingID    *string
	CreatedByID string
	Question    string
	CreatedAt   time.Time
	ClosedAt    *time.Time
	ClosedByID  *string
	Options     []VoteOption
	Ballots     []VoteBallot
}

// VoteOptionInput is one of create_vote's option dicts.
type VoteOptionInput struct {
	Label     string
	PlaceName *string
}

// VoteInput is create_vote's keyword arguments.
type VoteInput struct {
	ContextID   string
	OutingID    *string
	CreatedByID string
	Question    string
	Options     []VoteOptionInput
	Now         time.Time
}

const voteColumns = `votes.id, votes.context_id, votes.outing_id, votes.created_by_id, votes.question,
       votes.created_at, votes.closed_at, votes.closed_by_id`

func scanVote(row pgx.Row) (*Vote, error) {
	var v Vote
	err := row.Scan(&v.ID, &v.ContextID, &v.OutingID, &v.CreatedByID, &v.Question, &v.CreatedAt, &v.ClosedAt, &v.ClosedByID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v.CreatedAt, v.ClosedAt = v.CreatedAt.UTC(), utcOptional(v.ClosedAt)
	return &v, nil
}

// voteRecord is `_vote_record(vote)`: the options by position, then the
// ballots by (created_at, id), two statements per vote in that order.
func (r Repository) voteRecord(ctx context.Context, v *Vote) error {
	rows, err := r.Q.Query(ctx,
		`SELECT vote_options.id, vote_options.vote_id, vote_options.position, vote_options.label, vote_options.place_name
		   FROM vote_options
		  WHERE vote_options.vote_id = $1::UUID
		  ORDER BY vote_options.position`, v.ID)
	if err != nil {
		return err
	}
	v.Options = []VoteOption{}
	for rows.Next() {
		var o VoteOption
		if err := rows.Scan(&o.ID, &o.VoteID, &o.Position, &o.Label, &o.PlaceName); err != nil {
			rows.Close()
			return err
		}
		v.Options = append(v.Options, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	rows, err = r.Q.Query(ctx,
		`SELECT vote_ballots.id, vote_ballots.vote_id, vote_ballots.option_id, vote_ballots.voter_id,
		        vote_ballots.created_at, vote_ballots.updated_at
		   FROM vote_ballots
		  WHERE vote_ballots.vote_id = $1::UUID
		  ORDER BY vote_ballots.created_at, vote_ballots.id`, v.ID)
	if err != nil {
		return err
	}
	v.Ballots = []VoteBallot{}
	for rows.Next() {
		b, err := scanBallot(rows)
		if err != nil {
			rows.Close()
			return err
		}
		v.Ballots = append(v.Ballots, *b)
	}
	rows.Close()
	return rows.Err()
}

func scanBallot(row pgx.Row) (*VoteBallot, error) {
	var b VoteBallot
	err := row.Scan(&b.ID, &b.VoteID, &b.OptionID, &b.VoterID, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	b.CreatedAt, b.UpdatedAt = b.CreatedAt.UTC(), b.UpdatedAt.UTC()
	return &b, nil
}

// CreateVote is create_vote: two flushes, so two kinds of statement in order.
// First the vote's INSERT (client-side uuid4, the caller's clock, every mapped
// column listed with the closing pair as NULL); then the options as ONE
// multi-row INSERT per 1000 rows (insertmanyvalues), each option with a
// client-side uuid4 and its list index as position, no statement at all for an
// empty list; then `_vote_record`'s two reads. The record carries the vote
// fields handed in and the options and ballots as read back.
func (r Repository) CreateVote(ctx context.Context, in VoteInput) (Vote, error) {
	id, err := newUUID()
	if err != nil {
		return Vote{}, err
	}
	created := pythonInstant(in.Now)
	if _, err := r.Q.Exec(ctx,
		`INSERT INTO votes (id, context_id, outing_id, created_by_id, question, created_at, closed_at, closed_by_id)
		 VALUES ($1::UUID, $2::UUID, $3::UUID, $4::UUID, $5::VARCHAR, $6::TIMESTAMP WITH TIME ZONE,
		         $7::TIMESTAMP WITH TIME ZONE, $8::UUID)`,
		id, in.ContextID, in.OutingID, in.CreatedByID, in.Question, created, nil, nil); err != nil {
		return Vote{}, err
	}
	for start := 0; start < len(in.Options); start += insertManyValuesPageSize {
		page := in.Options[start:min(start+insertManyValuesPageSize, len(in.Options))]
		values := make([]string, 0, len(page))
		args := make([]any, 0, 5*len(page))
		for i, option := range page {
			optionID, err := newUUID()
			if err != nil {
				return Vote{}, err
			}
			n := len(args)
			values = append(values, "($"+strconv.Itoa(n+1)+"::UUID, $"+strconv.Itoa(n+2)+"::UUID, $"+
				strconv.Itoa(n+3)+"::INTEGER, $"+strconv.Itoa(n+4)+"::VARCHAR, $"+strconv.Itoa(n+5)+"::VARCHAR)")
			args = append(args, optionID, id, start+i, option.Label, option.PlaceName)
		}
		if _, err := r.Q.Exec(ctx,
			`INSERT INTO vote_options (id, vote_id, position, label, place_name) VALUES `+strings.Join(values, ", "),
			args...); err != nil {
			return Vote{}, err
		}
	}
	vote := Vote{ID: id, ContextID: in.ContextID, OutingID: in.OutingID, CreatedByID: in.CreatedByID,
		Question: in.Question, CreatedAt: created}
	if err := r.voteRecord(ctx, &vote); err != nil {
		return Vote{}, err
	}
	return vote, nil
}

// GetVote is get_vote: `session.get(Vote, id)` (labelled table_column), then
// `_vote_record`.
//
// SQLAlchemy note: a vote the same session already loaded (list_votes, an
// earlier get_vote) answers from the identity map without the first SELECT.
func (r Repository) GetVote(ctx context.Context, voteID string) (*Vote, error) {
	v, err := scanVote(r.Q.QueryRow(ctx,
		`SELECT votes.id AS votes_id, votes.context_id AS votes_context_id, votes.outing_id AS votes_outing_id,
		        votes.created_by_id AS votes_created_by_id, votes.question AS votes_question,
		        votes.created_at AS votes_created_at, votes.closed_at AS votes_closed_at,
		        votes.closed_by_id AS votes_closed_by_id
		   FROM votes
		  WHERE votes.id = $1::UUID`, voteID))
	if err != nil || v == nil {
		return nil, err
	}
	if err := r.voteRecord(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

// ListVotes is list_votes: the context's votes by (created_at, id), all read
// first, then `_vote_record`'s two reads per vote.
func (r Repository) ListVotes(ctx context.Context, contextID string) ([]Vote, error) {
	rows, err := r.Q.Query(ctx,
		`SELECT `+voteColumns+`
		   FROM votes
		  WHERE votes.context_id = $1::UUID
		  ORDER BY votes.created_at, votes.id`, contextID)
	if err != nil {
		return nil, err
	}
	out := []Vote{}
	for rows.Next() {
		v, err := scanVote(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if err := r.voteRecord(ctx, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// lockVote is `select(Vote).where(Vote.id == vote_id).with_for_update()`.
func (r Repository) lockVote(ctx context.Context, voteID string) (*Vote, error) {
	return scanVote(r.Q.QueryRow(ctx,
		`SELECT `+voteColumns+`
		   FROM votes
		  WHERE votes.id = $1::UUID FOR UPDATE`, voteID))
}

// UpsertBallot is upsert_ballot. Statements and refusals, in Python's order:
// the vote FOR UPDATE (Conflict VOTE_NOT_FOUND, then VOTE_CLOSED); the option
// on that vote with LIMIT 1 (Conflict UNKNOWN_OPTION); the voter's ballot FOR
// UPDATE; then either one INSERT (created_at = updated_at = now) or one UPDATE
// of the columns whose value changes, option_id before updated_at, and no
// UPDATE at all when the same option is cast at the stored updated_at instant.
// The bool is replaced_previous_ballot.
//
// Not reproduced, because it lives in the session and not in the method: the
// service calls get_vote first in the same session, so SQLAlchemy's FOR
// UPDATE re-read keeps that earlier, unlocked copy's closed_at (no
// populate_existing) and a vote closed in between is not seen as closed. Go
// judges the row as read under the lock.
func (r Repository) UpsertBallot(ctx context.Context, voteID, optionID, voterID string, now time.Time) (VoteBallot, bool, error) {
	vote, err := r.lockVote(ctx, voteID)
	if err != nil {
		return VoteBallot{}, false, err
	}
	if vote == nil {
		return VoteBallot{}, false, &Conflict{Code: "VOTE_NOT_FOUND"}
	}
	if vote.ClosedAt != nil {
		return VoteBallot{}, false, &Conflict{Code: "VOTE_CLOSED"}
	}
	var known string
	err = r.Q.QueryRow(ctx,
		`SELECT vote_options.id
		   FROM vote_options
		  WHERE vote_options.vote_id = $1::UUID AND vote_options.id = $2::UUID
		  LIMIT $3::INTEGER`, voteID, optionID, 1).Scan(&known)
	if errors.Is(err, pgx.ErrNoRows) {
		return VoteBallot{}, false, &Conflict{Code: "UNKNOWN_OPTION"}
	}
	if err != nil {
		return VoteBallot{}, false, err
	}
	ballot, err := scanBallot(r.Q.QueryRow(ctx,
		`SELECT vote_ballots.id, vote_ballots.vote_id, vote_ballots.option_id, vote_ballots.voter_id,
		        vote_ballots.created_at, vote_ballots.updated_at
		   FROM vote_ballots
		  WHERE vote_ballots.vote_id = $1::UUID AND vote_ballots.voter_id = $2::UUID FOR UPDATE`, voteID, voterID))
	if err != nil {
		return VoteBallot{}, false, err
	}
	instant := pythonInstant(now)
	if ballot == nil {
		id, err := newUUID()
		if err != nil {
			return VoteBallot{}, false, err
		}
		if _, err := r.Q.Exec(ctx,
			`INSERT INTO vote_ballots (id, vote_id, option_id, voter_id, created_at, updated_at)
			 VALUES ($1::UUID, $2::UUID, $3::UUID, $4::UUID, $5::TIMESTAMP WITH TIME ZONE, $6::TIMESTAMP WITH TIME ZONE)`,
			id, voteID, optionID, voterID, instant, instant); err != nil {
			return VoteBallot{}, false, err
		}
		return VoteBallot{ID: id, VoteID: voteID, OptionID: optionID, VoterID: voterID,
			CreatedAt: instant, UpdatedAt: instant}, false, nil
	}
	var sets []string
	var args []any
	if ballot.OptionID != optionID {
		args = append(args, optionID)
		sets = append(sets, "option_id=$"+strconv.Itoa(len(args))+"::UUID")
	}
	if !ballot.UpdatedAt.Equal(instant) {
		args = append(args, instant)
		sets = append(sets, "updated_at=$"+strconv.Itoa(len(args))+"::TIMESTAMP WITH TIME ZONE")
	}
	ballot.OptionID, ballot.UpdatedAt = optionID, instant
	if len(sets) > 0 {
		args = append(args, ballot.ID)
		if _, err := r.Q.Exec(ctx,
			`UPDATE vote_ballots SET `+strings.Join(sets, ", ")+` WHERE vote_ballots.id = $`+strconv.Itoa(len(args))+`::UUID`,
			args...); err != nil {
			return VoteBallot{}, false, err
		}
	}
	return *ballot, true, nil
}

// CloseVote is close_vote: the vote FOR UPDATE (Conflict VOTE_NOT_FOUND, then
// VOTE_ALREADY_CLOSED), one UPDATE of closed_at and closed_by_id, then
// `_vote_record`'s two reads. The record carries the closing values handed in.
// A closer with no people row fails the UPDATE on fk_votes_closed_by and that
// *pgconn.PgError is returned as is.
func (r Repository) CloseVote(ctx context.Context, voteID, closedByID string, now time.Time) (Vote, error) {
	vote, err := r.lockVote(ctx, voteID)
	if err != nil {
		return Vote{}, err
	}
	if vote == nil {
		return Vote{}, &Conflict{Code: "VOTE_NOT_FOUND"}
	}
	if vote.ClosedAt != nil {
		return Vote{}, &Conflict{Code: "VOTE_ALREADY_CLOSED"}
	}
	closed := pythonInstant(now)
	if _, err := r.Q.Exec(ctx,
		`UPDATE votes SET closed_at=$1::TIMESTAMP WITH TIME ZONE, closed_by_id=$2::UUID WHERE votes.id = $3::UUID`,
		closed, closedByID, voteID); err != nil {
		return Vote{}, err
	}
	vote.ClosedAt, vote.ClosedByID = &closed, &closedByID
	if err := r.voteRecord(ctx, vote); err != nil {
		return Vote{}, err
	}
	return *vote, nil
}
