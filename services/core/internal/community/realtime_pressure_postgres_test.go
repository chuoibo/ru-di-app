package community

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresCommunityRelaySurvivesRequestPoolPressure(t *testing.T) {
	f := setup(t)
	p := f.post(t, "public")
	f.approve(t, p)
	config := f.pool.Config().Copy()
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var workers sync.WaitGroup
	defer func() { cancel(); workers.Wait(); pool.Close() }()
	reader := New(pool, nil, nil)
	server := httptest.NewServer(reader)
	defer server.Close()
	deadline, done := context.WithTimeout(ctx, 8*time.Second)
	defer done()
	conn, _, err := websocket.Dial(deadline, "ws"+strings.TrimPrefix(server.URL, "http")+"/v2/community/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if err = wsjson.Write(deadline, conn, map[string]any{"token": f.tokens[1], "posts": []string{p.ID}}); err != nil {
		t.Fatal(err)
	}
	var msg frame
	if err = wsjson.Read(deadline, conn, &msg); err != nil {
		t.Fatal(err)
	}
	workers.Add(1)
	go func() { defer workers.Done(); reader.runEvents(ctx) }()
	// Let the relay start before a slow request attempts to hold the entire
	// pool. Without its reservation, the next reconciliation cannot proceed.
	time.Sleep(100 * time.Millisecond)
	workers.Add(1)
	go func() {
		defer workers.Done()
		request, e := pool.Acquire(ctx)
		if e == nil {
			defer request.Release()
			<-ctx.Done()
		}
	}()
	time.Sleep(100 * time.Millisecond)
	requireStatus(t, f.call("PUT", "/v2/community/posts/"+p.ID+"/like", 2, nil), 200)
	var committed int64
	if err = f.pool.QueryRow(ctx, `SELECT max(id) FROM community_events WHERE post_id=$1`, p.ID).Scan(&committed); err != nil {
		t.Fatal(err)
	}
	arrival, stop := context.WithTimeout(ctx, 2*time.Second)
	defer stop()
	for {
		if err = wsjson.Read(arrival, conn, &msg); err != nil {
			t.Fatal("request pool pressure starved committed realtime invalidation:", err)
		}
		if msg.Cursor >= committed && (msg.Kind == "sync" || msg.Kind == "post.changed" && msg.PostID == p.ID) {
			break
		}
	}
}
