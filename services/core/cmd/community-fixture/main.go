// community-fixture seeds an empty, explicitly synthetic loopback database.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/community"
)

type actor struct {
	ID    string `json:"id"`
	Token string `json:"token"`
}

func id() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}
func main() {
	n := flag.Int("actors", 20000, "Synthetic accounts (4..100000)")
	out := flag.String("out", "", "New JSON file outside all Git worktrees")
	flag.Parse()
	if err := run(*n, *out); err != nil {
		fmt.Fprintln(os.Stderr, "synthetic fixture refused or failed; check empty loopback database, migrations, and output path")
		os.Exit(1)
	}
}
func run(n int, out string) error {
	raw := os.Getenv("MOBILE_DATABASE_URL")
	u, err := url.Parse(raw)
	if err != nil || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") || !strings.HasPrefix(strings.TrimPrefix(u.Path, "/"), "community_synthetic_") || n < 4 || n > 100000 || out == "" {
		return fmt.Errorf("synthetic scope required")
	}
	absolute, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return err
	}
	for dir := parent; ; dir = filepath.Dir(dir) {
		if _, e := os.Stat(filepath.Join(dir, ".git")); e == nil {
			return fmt.Errorf("output inside worktree")
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	f, err := os.OpenFile(filepath.Join(parent, filepath.Base(absolute)), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, raw)
	if err != nil {
		return err
	}
	defer pool.Close()
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM people`).Scan(&count); err != nil || count != 0 {
		return fmt.Errorf("empty database required")
	}
	if err = community.Migrate(ctx, pool); err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	actors, people, sessions := []actor{}, [][]any{}, [][]any{}
	for i := 0; i < n; i++ {
		a := actor{id(), "synthetic-load-" + id() + id()}
		actors = append(actors, a)
		people = append(people, []any{a.ID, "Synthetic community actor"})
		sessions = append(sessions, []any{id(), a.ID, auth.TokenDigest(a.Token), "genesis", time.Now().Add(48 * time.Hour)})
	}
	if _, err = tx.CopyFrom(ctx, pgx.Identifier{"people"}, []string{"id", "display_name"}, pgx.CopyFromRows(people)); err != nil {
		return err
	}
	if _, err = tx.CopyFrom(ctx, pgx.Identifier{"account_sessions"}, []string{"id", "person_id", "token_digest", "issued_via", "expires_at"}, pgx.CopyFromRows(sessions)); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO friend_requests(id,requester_id,addressee_id,state,decided_at) VALUES($1,$2,$3,'accepted',clock_timestamp())`, id(), actors[0].ID, actors[1].ID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO community_moderators VALUES($1)`, actors[3].ID); err != nil {
		return err
	}
	posts := []string{}
	for i := 0; i < 500; i++ {
		pid, topic := id(), []string{"đi bộ", "cà phê", "du lịch", "biển"}[i%4]
		body := strings.Repeat("Chuyến đi tổng hợp dành riêng cho kiểm thử tải. ", 12)
		if _, err = tx.Exec(ctx, `INSERT INTO posts(id,author_id,audience,body) VALUES($1,$2,'public',$3)`, pid, actors[i%min(n, 100)].ID, body); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO community_posts(post_id,revision,published_revision,requested_audience,status,topics,published_at) VALUES($1,1,1,'public','approved',$2,clock_timestamp()-make_interval(hours=>$3))`, pid, []string{topic}, i%48); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO community_revisions(post_id,revision,body,topics,mentions,media_ids) VALUES($1,1,$2,$3,'{}','{}')`, pid, body, []string{topic}); err != nil {
			return err
		}
		posts = append(posts, pid)
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if err = json.NewEncoder(f).Encode(map[string]any{"actors": actors, "posts": posts}); err != nil {
		return err
	}
	fmt.Printf("Created %d synthetic accounts and 500 posts; no inference quality measured.\n", n)
	return nil
}
