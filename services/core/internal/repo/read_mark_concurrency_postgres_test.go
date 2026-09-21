//go:build postgres

package repo

import (
	"context"
	"testing"
	"time"

	"mobile/services/core/internal/testdb"
)

func TestReadMarkConcurrentWritersNeverMoveBackwards(t *testing.T) {
	for _, firstRead := range []bool{false, true} {
		name := "existing"
		if firstRead {
			name = "first-read"
		}
		t.Run(name, func(t *testing.T) {
			pool := testdb.Pool(t)
			person := committedPerson(t, pool)
			r := Repository{Q: pool}
			group, err := r.CreateContext(bg, "Chat race (dữ liệu mẫu)", person)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_, _ = pool.Exec(bg, "DELETE FROM context_read_marks WHERE context_id=$1", group.ID)
				_, _ = pool.Exec(bg, "DELETE FROM messages WHERE context_id=$1", group.ID)
				_, _ = pool.Exec(bg, "DELETE FROM contexts WHERE id=$1", group.ID)
			})
			now := time.Now().UTC().Truncate(time.Microsecond)
			body := "Tin thử (dữ liệu mẫu)"
			messages := make([]Message, 3)
			for i := range messages {
				messages[i], err = r.CreateMessage(bg, MessageInput{ContextID: group.ID, AuthorID: &person, Kind: "text", Body: &body, Now: now.Add(time.Duration(i) * time.Second)})
				if err != nil {
					t.Fatal(err)
				}
			}
			if !firstRead {
				if _, err := r.SetReadMark(bg, group.ID, person, messages[0], now); err != nil {
					t.Fatal(err)
				}
			}
			winner := begin(t, pool)
			if _, err := (Repository{Q: winner}).SetReadMark(bg, group.ID, person, messages[2], now); err != nil {
				t.Fatal(err)
			}
			loser := begin(t, pool)
			ctx, cancel := context.WithTimeout(bg, 5*time.Second)
			defer cancel()
			result := make(chan error, 1)
			go func() {
				mark, err := (Repository{Q: loser}).SetReadMark(ctx, group.ID, person, messages[1], now.Add(time.Second))
				if err == nil && mark.LastReadMessageID != messages[2].ID {
					result <- &wrongReadMark{}
					return
				}
				if err == nil {
					err = loser.Commit(ctx)
				}
				result <- err
			}()
			if !waitsOnLock(t, pool, loser.Conn().PgConn().PID(), 2*time.Second) {
				t.Fatal("second writer did not contend on the row")
			}
			if err := winner.Commit(bg); err != nil {
				t.Fatal(err)
			}
			if err := <-result; err != nil {
				t.Fatal(err)
			}
			var id string
			if err := pool.QueryRow(bg, "SELECT last_read_message_id FROM context_read_marks WHERE context_id=$1 AND person_id=$2", group.ID, person).Scan(&id); err != nil {
				t.Fatal(err)
			}
			if id != messages[2].ID {
				t.Fatal("committed read mark regressed")
			}
		})
	}
}

type wrongReadMark struct{}

func (*wrongReadMark) Error() string { return "concurrent older read replaced the newer mark" }

func TestReadMarkEqualTimestampUsesUUIDAndPreservesNoOp(t *testing.T) {
	w := newStdWorld()
	tx, _ := seeded(t, w.sql)
	r := Repository{Q: tx}
	at := time.Now().UTC().Truncate(time.Microsecond)
	body := "Tin thử (dữ liệu mẫu)"
	low, err := r.CreateMessage(bg, MessageInput{ContextID: w.group, AuthorID: &w.owner, Kind: "text", Body: &body, Now: at})
	if err != nil {
		t.Fatal(err)
	}
	high, err := r.CreateMessage(bg, MessageInput{ContextID: w.group, AuthorID: &w.owner, Kind: "text", Body: &body, Now: at})
	if err != nil {
		t.Fatal(err)
	}
	if low.ID > high.ID {
		low, high = high, low
	}
	if _, err = r.SetReadMark(bg, w.group, w.owner, low, at); err != nil {
		t.Fatal(err)
	}
	expected, err := r.SetReadMark(bg, w.group, w.owner, high, at.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	for _, msg := range []Message{low, high} {
		got, err := r.SetReadMark(bg, w.group, w.owner, msg, at.Add(time.Minute))
		if err != nil || got.LastReadMessageID != high.ID || !got.UpdatedAt.Equal(expected.UpdatedAt) {
			t.Fatalf("equal-time watermark changed: %+v %v", got, err)
		}
	}
}
