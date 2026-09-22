//go:build postgres

package chatassist

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The shared sheet's invariants that live in the schema rather than in Go,
// pinned here because a later migration could quietly drop any of them and
// every higher tier would still look green for a while.
//
// The end-to-end tier drives the same rules through HTTP as cases D2 and D5;
// this test is the close-range version, so a regression names the constraint
// instead of naming a status code.
func TestSharedDraftConstraints(t *testing.T) {
	f := setup(t, nil)
	ctx := context.Background()

	anchor := func() string {
		id := newID()
		if _, err := f.pool.Exec(ctx,
			`INSERT INTO messages (id,context_id,author_id,kind,card,created_at)
			 VALUES ($1,$2,$3,'ai_card','{}'::jsonb,clock_timestamp())`,
			id, f.context, f.person); err != nil {
			t.Fatal(err)
		}
		return id
	}
	insert := func(pool *pgxpool.Pool, vote *string, message, status string) error {
		_, err := pool.Exec(ctx,
			`INSERT INTO chat_shared_drafts
			  (id,context_id,message_id,source_vote_id,revision,title,stops,status,created_by)
			 VALUES ($1,$2,$3,$4,1,'Tờ hẹn mẫu','[]'::jsonb,$5,$6)`,
			newID(), f.context, message, vote, status, f.person)
		return err
	}

	t.Run("một bình chọn chỉ nuôi một tờ đang mở", func(t *testing.T) {
		vote := newID()
		if err := insert(f.pool, &vote, anchor(), "open"); err != nil {
			t.Fatalf("tờ đầu phải vào được: %v", err)
		}
		if err := insert(f.pool, &vote, anchor(), "open"); err == nil {
			t.Fatal("tờ thứ hai cho cùng bình chọn phải bị chỉ mục riêng phần chặn")
		}
		// A sheet that is no longer open frees the poll: after discarding one,
		// the group may start over from the same decision.
		if _, err := f.pool.Exec(ctx,
			`UPDATE chat_shared_drafts SET status='discarded' WHERE source_vote_id=$1`, vote); err != nil {
			t.Fatal(err)
		}
		if err := insert(f.pool, &vote, anchor(), "open"); err != nil {
			t.Fatalf("sau khi bỏ tờ cũ phải mở lại được: %v", err)
		}
	})

	t.Run("một neo chỉ mang một tờ", func(t *testing.T) {
		message := anchor()
		if err := insert(f.pool, nil, message, "open"); err != nil {
			t.Fatalf("tờ đầu phải vào được: %v", err)
		}
		if err := insert(f.pool, nil, message, "open"); err == nil {
			t.Fatal("hai tờ trên một neo phải bị chặn: luồng không được hiện một tờ hai lần")
		}
	})

	t.Run("trạng thái và tiêu đề rỗng bị từ chối", func(t *testing.T) {
		if err := insert(f.pool, nil, anchor(), "dang-nghi"); err == nil {
			t.Fatal("trạng thái ngoài open/promoted/discarded phải bị CHECK chặn")
		}
		if _, err := f.pool.Exec(ctx,
			`INSERT INTO chat_shared_drafts
			  (id,context_id,message_id,source_vote_id,revision,title,stops,status,created_by)
			 VALUES ($1,$2,$3,NULL,1,'   ','[]'::jsonb,'open',$4)`,
			newID(), f.context, anchor(), f.person); err == nil {
			t.Fatal("tiêu đề chỉ có khoảng trắng phải bị CHECK chặn")
		}
	})
}
