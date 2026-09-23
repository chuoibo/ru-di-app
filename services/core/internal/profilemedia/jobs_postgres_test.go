package profilemedia

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/achievementv1"
	"mobile/services/core/internal/testdb"
)

func newTestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func mediaFixture(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	ctx := context.Background()
	base := testdb.Pool(t)
	schema := "profile_media_test_" + strings.ReplaceAll(newTestID(), "-", "")
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := base.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	config := base.Config().Copy()
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, "CREATE TABLE people (LIKE public.people INCLUDING ALL)"); err != nil {
		t.Fatal(err)
	}
	if err := achievementv1.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	person := "00000000-0000-0000-0000-000000000001"
	if _, err := pool.Exec(ctx, `INSERT INTO people(id,display_name) VALUES($1,'Synthetic media user')`, person); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO achievement_mp4_credits(person_id,source) VALUES($1,'route:dau_chan')`, person); err != nil {
		t.Fatal(err)
	}
	return pool, person
}

func TestCreditReservationIsAtomicAndFailureReleasesIt(t *testing.T) {
	pool, person := mediaFixture(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := Reserve(ctx, pool, person, fmt.Sprintf("click-%d", i), fmt.Sprintf("job-%d", i), "nep_video", []byte(`{"synthetic":true}`))
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	accepted, refused := 0, 0
	for err := range results {
		if err == nil {
			accepted++
		} else if err == ErrNoCredit {
			refused++
		} else {
			t.Fatal(err)
		}
	}
	if accepted != 1 || refused != 1 {
		t.Fatalf("accepted=%d refused=%d", accepted, refused)
	}
	var winningID string
	if err := pool.QueryRow(ctx, `SELECT id FROM profile_media_jobs WHERE status='reserved'`).Scan(&winningID); err != nil {
		t.Fatal(err)
	}
	if err := SetStatus(ctx, pool, person, winningID, "failed", "proxy_failed", ""); err != nil {
		t.Fatal(err)
	}
	job, err := Reserve(ctx, pool, person, "retry-after-failure", "job-retry", "nep_video", []byte(`{}`))
	if err != nil || job.CreditSource != "route:dau_chan" {
		t.Fatalf("released credit not reused: job=%+v err=%v", job, err)
	}
	if err := SetStatus(ctx, pool, person, job.ID, "ready", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := Reserve(ctx, pool, person, "third-click", "job-third", "nep_video", []byte(`{}`)); err != ErrNoCredit {
		t.Fatalf("completed credit was spent again: %v", err)
	}
	credits, err := CreditBalance(ctx, pool, person)
	if err != nil || credits.Granted != 1 || credits.Used != 1 || credits.Available != 0 {
		t.Fatalf("credit balance=%+v err=%v", credits, err)
	}
}

func TestSameClickConcurrentlyReservesOneJobAndRejectsChangedPayload(t *testing.T) {
	pool, person := mediaFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO achievement_mp4_credits(person_id,source) VALUES($1,'route:ky_niem')`, person); err != nil {
		t.Fatal(err)
	}
	const callers = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	type answer struct {
		job Job
		err error
	}
	results := make(chan answer, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			job, err := Reserve(ctx, pool, person, "same-click", fmt.Sprintf("same-job-%d", i), "nep_video", []byte(`{"image_job_ids":["synthetic-a"]}`))
			results <- answer{job, err}
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	var first string
	for result := range results {
		if result.err != nil {
			t.Fatalf("same click failed: %v", result.err)
		}
		if first == "" {
			first = result.job.ID
		}
		if result.job.ID != first {
			t.Fatalf("same click created %q and %q", first, result.job.ID)
		}
	}
	if _, err := Reserve(ctx, pool, person, "same-click", "changed-job", "nep_video", []byte(`{"image_job_ids":["synthetic-b"]}`)); err == nil {
		t.Fatal("same key accepted a changed media request")
	}
	if err := SetStatus(ctx, pool, person, first, "running", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := SetStatus(ctx, pool, person, first, "queued", "", ""); err == nil {
		t.Fatal("running media job moved back to queued")
	}
}
