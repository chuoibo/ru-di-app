package chatassist

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"mobile/services/core/internal/domain/companion"
	"mobile/services/core/internal/domain/tree"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/treejson"
)

// The group AI answers inside the thread (ADR-0039, proposed; design 03).
//
// The person's `@Rủ Đi …` words are an ordinary message, posted through the
// ordinary send queue. The client then invokes the AI explicitly and names that
// message as the trigger; the server never starts anything from message text
// (routes.actOnMessageIntent is unchanged). The answer is an `ai_card` of kind
// `tra_loi`, published as a reply to the trigger.

const (
	// tuoiTrigger is how old a trigger may be. Asking the AI about a message
	// from last week is a new question, and it should be asked as one.
	tuoiTrigger = 24 * time.Hour
	// Room limits: how many jobs may be in flight in one room, and how many
	// one room may start per hour. The per-person limit (8 a minute) stays.
	maxDangChayMoiPhong = 3
	maxMoiPhongMoiGio   = 30
)

// kiemTrigger checks the message an invocation answers. It reads the five
// columns that decide the question and never `body`: the server cannot read
// the words, and does not need to (ADR-0036 §2.4). The row is held with a KEY
// SHARE lock until the job row is inserted, so a deletion that races this
// create either finishes first (and the check refuses) or waits for it (and
// chat_ai_trigger_deleted then cancels the new job).
//
// One code for every way it can be wrong, including "no such message":
// telling them apart would answer "does message X exist" for anybody in the
// room.
func kiemTrigger(ctx context.Context, tx pgx.Tx, room, person, trigger string) error {
	var author *string
	var kind string
	var created time.Time
	var deleted *time.Time
	var fresh bool
	err := tx.QueryRow(ctx, `SELECT author_id::text,kind::text,created_at,deleted_at,created_at>clock_timestamp()-make_interval(secs => $3) FROM messages WHERE id=$1 AND context_id=$2 FOR KEY SHARE`, trigger, room, tuoiTrigger.Seconds()).Scan(&author, &kind, &created, &deleted, &fresh)
	if errors.Is(err, pgx.ErrNoRows) {
		return &denied{422, "trigger_khong_hop_le"}
	}
	if err != nil {
		return err
	}
	if author == nil || *author != person || kind != "text" || deleted != nil || !fresh {
		return &denied{422, "trigger_khong_hop_le"}
	}
	return nil
}

// gioiHanPhong holds one room to maxDangChayMoiPhong jobs in flight and
// maxMoiPhongMoiGio jobs an hour. The per-person advisory lock does not
// serialise two people in one room, so the room takes its own lock first;
// every caller takes the person lock and then the room lock, never the other
// way round.
func gioiHanPhong(ctx context.Context, tx pgx.Tx, room string) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('chat_ai_room:'||$1,0))`, room); err != nil {
		return err
	}
	var active, hour int
	if err := tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE status IN ('queued','running')),count(*) FILTER (WHERE created_at>clock_timestamp()-interval '1 hour') FROM chat_ai_invocations WHERE context_id=$1`, room).Scan(&active, &hour); err != nil {
		return err
	}
	if active >= maxDangChayMoiPhong {
		return &denied{429, "invocation_room_busy"}
	}
	if hour >= maxMoiPhongMoiGio {
		return &denied{429, "invocation_room_rate_limited"}
	}
	return nil
}

// daCoTraLoi reports whether err is the one-answer-per-trigger index refusing
// a second job on the same message.
func daCoTraLoi(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505" && pg.ConstraintName == "chat_ai_one_answer_per_trigger"
}

// theCuaViec is the card a job publishes. A job with a trigger answers inside
// the thread with a `tra_loi` reply around exactly the card it published
// before. A job without one comes from a client that cannot draw a reply (an
// app from before this change, still in people's pockets), so it gets that
// card unchanged -- the same bytes GroundCard has always produced.
func theCuaViec(j work, part tree.Value, places []*tree.OrderedMap) ([]byte, error) {
	if j.trigger == "" {
		grounded, err := companion.GroundCard(part, places)
		if err != nil {
			return nil, err
		}
		return pyjson.Dumps(treejson.From(grounded))
	}
	grounded, err := companion.GroundReply(companion.ReplyMeta{InvocationID: j.id, Command: j.command, Read: j.soTin}, []tree.Value{part}, places)
	if err != nil {
		return nil, err
	}
	return pyjson.Dumps(treejson.From(grounded))
}

// giuTrigger takes a KEY SHARE lock on the trigger. publish calls it AFTER it
// holds the room's feed head (lockFeed) and BEFORE it locks the job, which is
// the order every Go chat write takes: chatlegacychange.BeforeWrite locks the
// head, then a delete, an edit or a reaction locks the message FOR UPDATE, and
// a delete then reaches the job through chat_ai_trigger_deleted. Holding the
// head first makes publish and those writes queue on the head.
//
// The order this replaced (the trigger first, then the head) deadlocked: a
// write held the head and waited for FOR UPDATE on the message while publish
// held the KEY SHARE and waited for the head. The note inside schema_luong.sql
// still states that old order; the file is checksummed, so it stays and this
// comment is the correct one (TestDangTraLoiKhongKhoaCheoVoiXoaVaCamXuc).
//
// False means the message is gone outright, not merely taken back.
func giuTrigger(ctx context.Context, tx pgx.Tx, j work) (bool, error) {
	if j.trigger == "" {
		return true, nil
	}
	var id string
	err := tx.QueryRow(ctx, `SELECT id FROM messages WHERE id=$1 AND context_id=$2 FOR KEY SHARE`, j.trigger, j.conversation).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
