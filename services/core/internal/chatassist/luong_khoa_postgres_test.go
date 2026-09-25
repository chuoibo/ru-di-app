//go:build postgres

package chatassist

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"mobile/services/core/internal/brain"
	"mobile/services/core/internal/chatlegacychange"
	"mobile/services/core/internal/db"
	"mobile/services/core/internal/httpapi/dispatch"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/httpapi/mw/cors"
	"mobile/services/core/internal/httpapi/router"
	"mobile/services/core/internal/pyval"
	"mobile/services/core/internal/routes"
	"mobile/services/core/ownership"
)

// Publishing an answer against the chat write routes that lock its trigger,
// in the lock order production imposes.
//
// Every Go chat write goes through chatlegacychange.BeforeWrite (cmd/core sets
// it as env.BeforeServe when chat is on): the room's sequence head first, then
// the message FOR UPDATE. publish() has to take the same head before it
// touches the trigger at all. The order this replaced took a KEY SHARE on the
// trigger first and then waited for the head, while a delete or a reaction
// held the head and waited for FOR UPDATE on the trigger: a cycle, answered by
// PostgreSQL with 40P01 about a second later.

// banKhoa is the fixture plus what the real write routes need: the reaction
// and vote tables, change capture with its head, and the Go front door with
// BeforeServe wired as in production. The routes and the publisher each get a
// pool of their own under a distinct application_name, so the test can see
// which of them is waiting on a lock.
type banKhoa struct {
	fixture
	cua             http.Handler
	dang            *Handler
	tenGhi, tenDang string
}

func setupKhoa(t *testing.T) banKhoa {
	t.Helper()
	f := setup(t, nil)
	ctx := context.Background()
	// Every table change capture puts a trigger on must exist in the test
	// schema first; otherwise CREATE TRIGGER would resolve to public.
	for _, table := range []string{"message_reactions", "votes", "vote_options", "vote_ballots"} {
		if _, err := f.pool.Exec(ctx, "CREATE TABLE "+table+" (LIKE public."+table+" INCLUDING ALL)"); err != nil {
			t.Fatal(err)
		}
	}
	// LIKE copies the unique definition under a fresh name; AddReaction knows
	// the production name.
	var unique string
	if err := f.pool.QueryRow(ctx, `SELECT conname FROM pg_constraint WHERE conrelid='message_reactions'::regclass AND contype='u'`).Scan(&unique); err != nil {
		t.Fatal(err)
	}
	if unique != "uq_message_reactions_one_per_kind" {
		if _, err := f.pool.Exec(ctx, "ALTER TABLE message_reactions RENAME CONSTRAINT "+pgx.Identifier{unique}.Sanitize()+" TO uq_message_reactions_one_per_kind"); err != nil {
			t.Fatal(err)
		}
	}
	if err := chatlegacychange.Migrate(ctx, f.pool); err != nil {
		t.Fatal(err)
	}
	tag := strings.ReplaceAll(newID(), "-", "")[:12]
	b := banKhoa{fixture: f, tenGhi: "khoa_ghi_" + tag, tenDang: "khoa_dang_" + tag}
	b.dang = New(b.poolTen(t, b.tenDang), brain.Configured())
	ghi := b.poolTen(t, b.tenGhi)
	env := endpoint.Env{Mode: endpoint.ModeDev, NewUnit: func() *db.Unit { return db.NewUnit(ghi) }, Now: time.Now, BeforeServe: chatlegacychange.BeforeWrite}
	b.cua = cuaTruocGo(t, env)
	return b
}

func (b banKhoa) poolTen(t *testing.T, name string) *pgxpool.Pool {
	t.Helper()
	config := b.pool.Config().Copy()
	config.ConnConfig.RuntimeParams["application_name"] = name
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// cuaTruocGo is the Go front door the binary builds: dispatch, the endpoint
// pipeline and every Go route, with BeforeServe from env.
func cuaTruocGo(t *testing.T, env endpoint.Env) http.Handler {
	t.Helper()
	contract, err := pyval.Load()
	if err != nil {
		t.Fatal(err)
	}
	handlers, err := routes.Handlers(contract, pyval.NewRegistry(), env)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := ownership.Load()
	if err != nil {
		t.Fatal(err)
	}
	table, err := router.New(manifest.Routes)
	if err != nil {
		t.Fatal(err)
	}
	var served []ownership.Route
	for _, row := range manifest.Routes {
		if handlers[row.ID] != nil {
			served = append(served, row)
		}
	}
	python := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("%s %s went to Python", r.Method, r.RequestURI)
	})
	h, err := dispatch.New(dispatch.Options{
		Router: table, Served: served, Handlers: handlers, Python: python,
		CORS: cors.New("", false), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Idempotency: func(next http.Handler) http.Handler { return next },
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// ghi runs one real chat write on the trigger through the front door: the
// author deletes it, or the other member reacts to it.
func (b banKhoa) ghi(act, trigger string) *httptest.ResponseRecorder {
	var r *http.Request
	actor := b.person
	if act == "xoa" {
		r = httptest.NewRequest(http.MethodDelete, "/contexts/"+b.context+"/messages/"+trigger, nil)
	} else {
		actor = b.peer
		r = httptest.NewRequest(http.MethodPost, "/contexts/"+b.context+"/messages/"+trigger+"/reactions", strings.NewReader(`{"kind":"heart"}`))
		r.Header.Set("Content-Type", "application/json")
	}
	r.Header.Set("X-Actor-ID", actor)
	r.Header.Set("X-Actor-Roles", "member")
	w := httptest.NewRecorder()
	b.cua.ServeHTTP(w, r)
	return w
}

// doiKhoa waits until a backend of the named pool is waiting on a lock.
func (b banKhoa) doiKhoa(t *testing.T, name string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		if err := b.pool.QueryRow(context.Background(), `SELECT count(*) FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock'`, name).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("%s never waited on a lock", name)
}

// ketQua is one round: what the write route answered, what publish returned,
// and what the database holds afterwards.
type ketQua struct {
	ma        int
	than      string
	loiDang   error
	trangThai string
	traLoi    int
	camXuc    int
}

func laKhoaCheo(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "40P01"
}

// mot runs one round. With ep set, the order is forced through a gate that
// holds the head: the side named first queues on it before the other starts,
// then the gate lets go. ghiTruoc puts the write route first in that queue,
// which is the interleaving that deadlocked. Without ep both start together.
func (b banKhoa) mot(t *testing.T, act string, ep, ghiTruoc bool) ketQua {
	t.Helper()
	ctx := context.Background()
	trigger := b.tinTag(t, b.context, b.person)
	w := b.goiTag(b.token, newID(), trigger, nil)
	requireCode(t, w, 202)
	var v Invocation
	_ = json.Unmarshal(w.Body.Bytes(), &v)
	// The job this round created, at its first entry into the queue.
	j, ok, err := b.handler.claimTin(ctx, v.ID, 1, "")
	if err != nil || !ok {
		t.Fatalf("claim %v %v", ok, err)
	}
	card, err := theCuaViec(j, phanChu("Synthetic answer"), nil)
	if err != nil {
		t.Fatal(err)
	}
	var gate pgx.Tx
	if ep {
		if gate, err = b.pool.Begin(ctx); err != nil {
			t.Fatal(err)
		}
		defer gate.Rollback(ctx)
		if err = lockFeed(ctx, gate, b.context); err != nil {
			t.Fatal(err)
		}
	}
	ghi := make(chan *httptest.ResponseRecorder, 1)
	dang := make(chan error, 1)
	batDauGhi := func() { go func() { ghi <- b.ghi(act, trigger) }() }
	batDauDang := func() { go func() { dang <- b.dang.publish(ctx, j, card, nil) }() }
	switch {
	case !ep:
		batDauGhi()
		batDauDang()
	case ghiTruoc:
		batDauGhi()
		b.doiKhoa(t, b.tenGhi)
		batDauDang()
		b.doiKhoa(t, b.tenDang)
	default:
		batDauDang()
		b.doiKhoa(t, b.tenDang)
		batDauGhi()
		b.doiKhoa(t, b.tenGhi)
	}
	if ep {
		if err = gate.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
	r := <-ghi
	out := ketQua{ma: r.Code, than: r.Body.String(), loiDang: <-dang}
	if err = b.pool.QueryRow(ctx, `SELECT status FROM chat_ai_invocations WHERE id=$1`, v.ID).Scan(&out.trangThai); err != nil {
		t.Fatal(err)
	}
	if err = b.pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE reply_to_id=$1 AND kind='ai_card'`, trigger).Scan(&out.traLoi); err != nil {
		t.Fatal(err)
	}
	if err = b.pool.QueryRow(ctx, `SELECT count(*) FROM message_reactions WHERE message_id=$1`, trigger).Scan(&out.camXuc); err != nil {
		t.Fatal(err)
	}
	// The limits (8 a minute per person, 30 an hour per room) are not what
	// this test is about; age the round out of both windows.
	if _, err = b.pool.Exec(ctx, `UPDATE chat_ai_invocations SET created_at=created_at-interval '2 hours' WHERE context_id=$1`, b.context); err != nil {
		t.Fatal(err)
	}
	return out
}

// kiem holds one round to the only outcomes that are right: no deadlock on
// either side, the route answered as it would alone, and exactly one fate for
// the question -- answered once, or (the trigger taken back first) cancelled
// with no answer. Never two answers, never a job left running.
func kiem(t *testing.T, name, act string, k ketQua) {
	t.Helper()
	if laKhoaCheo(k.loiDang) || strings.Contains(k.than, "deadlock") {
		t.Fatalf("%s: KHOÁ CHÉO (40P01): publish=%v route=%d %s", name, k.loiDang, k.ma, k.than)
	}
	if k.loiDang != nil {
		t.Fatalf("%s: publish: %v", name, k.loiDang)
	}
	switch act {
	case "xoa":
		if k.ma != 204 {
			t.Fatalf("%s: xoá tin tag: %d %s", name, k.ma, k.than)
		}
		answered := k.trangThai == "succeeded" && k.traLoi == 1
		taken := k.trangThai == "cancelled" && k.traLoi == 0
		if !answered && !taken {
			t.Fatalf("%s: xoá đua với publish: status=%s, %d câu trả lời", name, k.trangThai, k.traLoi)
		}
	default:
		if k.ma != 201 || k.camXuc != 1 {
			t.Fatalf("%s: thả cảm xúc: %d %s (%d cảm xúc)", name, k.ma, k.than, k.camXuc)
		}
		if k.trangThai != "succeeded" || k.traLoi != 1 {
			t.Fatalf("%s: cảm xúc đua với publish: status=%s, %d câu trả lời, cần đúng một", name, k.trangThai, k.traLoi)
		}
	}
}

func TestDangTraLoiKhongKhoaCheoVoiXoaVaCamXuc(t *testing.T) {
	b := setupKhoa(t)
	const lanEp, lanTuDo = 8, 40
	for _, act := range []string{"xoa", "tha"} {
		for i := 0; i < lanEp; i++ {
			// The write route queued on the head first: the order that
			// deadlocked. Then the publisher first, for the other fate.
			kiem(t, act+"/ghi-truoc", act, b.mot(t, act, true, true))
			kiem(t, act+"/dang-truoc", act, b.mot(t, act, true, false))
		}
		for i := 0; i < lanTuDo; i++ {
			kiem(t, act+"/tu-do", act, b.mot(t, act, false, false))
		}
	}
	// The forced orders decide the fate of a deletion: the route first takes
	// the question back, the publisher first keeps its answer.
	if k := b.mot(t, "xoa", true, true); k.trangThai != "cancelled" || k.traLoi != 0 {
		t.Fatalf("xoá trước publish: status=%s, %d câu trả lời", k.trangThai, k.traLoi)
	}
	if k := b.mot(t, "xoa", true, false); k.trangThai != "succeeded" || k.traLoi != 1 {
		t.Fatalf("publish trước xoá: status=%s, %d câu trả lời", k.trangThai, k.traLoi)
	}
}
