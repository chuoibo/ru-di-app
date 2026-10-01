//go:build postgres

package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/motluot"
	"mobile/services/core/internal/db"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/limit"
	"mobile/services/core/internal/testdb"
)

// POST /contexts/{id}/messages/{id}/expense-draft on its own, now that its
// Python twin is gone (ADR-0052) and parity has nothing to compare it with:
// every refusal before the model, the keyless answer, and a draft that bills
// the message's author and never a name the model wrote.

const (
	draftOwner    = "0d00aaaa-bbbb-4ccc-8ddd-eeeeeeeeee01"
	draftMember   = "0d00aaaa-bbbb-4ccc-8ddd-eeeeeeeeee02"
	draftStranger = "0d00aaaa-bbbb-4ccc-8ddd-eeeeeeeeee03"
	draftRoom     = "0d00aaaa-bbbb-4ccc-8ddd-eeeeeeeeeec0"
	draftMessage  = "0d00aaaa-bbbb-4ccc-8ddd-eeeeeeeeeee0"
)

func draftSchema(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	base := testdb.Pool(t)
	schema := fmt.Sprintf("expense_draft_test_%d", time.Now().UnixNano())
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE") })
	cfg := base.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for _, table := range []string{"people", "contexts", "memberships", "messages"} {
		if _, err := pool.Exec(ctx, fmt.Sprintf("CREATE TABLE %s (LIKE public.%s INCLUDING ALL)", table, table)); err != nil {
			t.Fatal(err)
		}
	}
	for _, stmt := range []string{
		`INSERT INTO people(id,display_name) VALUES ('` + draftOwner + `','Chủ nhóm'),('` + draftMember + `','Bạn'),('` + draftStranger + `','Người lạ')`,
		`INSERT INTO contexts(id,display_name,kind,created_by_id) VALUES ('` + draftRoom + `','Nhóm thử','group','` + draftOwner + `')`,
		`INSERT INTO memberships(id,context_id,person_id,state,role,origin) VALUES
		 (gen_random_uuid(),'` + draftRoom + `','` + draftOwner + `','active','admin','named'),
		 (gen_random_uuid(),'` + draftRoom + `','` + draftMember + `','active','member','named')`,
		`INSERT INTO messages(id,context_id,author_id,kind,body) VALUES ('` + draftMessage + `','` + draftRoom + `','` + draftMember + `','text','Tao trả 300k tiền nước')`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	return pool
}

func draftHandler(t *testing.T, pool *pgxpool.Pool, may *motluot.May) http.Handler {
	env := endpoint.Env{Mode: endpoint.ModeDev, NewUnit: func() *db.Unit { return db.NewUnit(pool) }, Now: time.Now,
		Limits: limit.NewSet(limit.Monotonic), AI: may}
	return coreWithEnv(t, env, func(next http.Handler) http.Handler { return next })
}

func postDraft(h http.Handler, actor, message string) (int, map[string]any) {
	r := httptest.NewRequest(http.MethodPost, "/contexts/"+draftRoom+"/messages/"+message+"/expense-draft", nil)
	if actor != "" {
		r.Header.Set("X-Actor-ID", actor)
		r.Header.Set("X-Actor-Roles", "member")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestExpenseDraftTuChoiTruocModel(t *testing.T) {
	pool := draftSchema(t)
	stub := llm.NewStub()
	h := draftHandler(t, pool, motluot.Moi(stub, 1))
	for _, c := range []struct {
		name, actor, message string
		status               int
		code                 string
	}{
		{"ẩn danh", "", draftMessage, 401, ""},
		{"người lạ", draftStranger, draftMessage, 403, ""},
		{"tin không có", draftOwner, "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", 404, "message_not_found"},
	} {
		status, body := postDraft(h, c.actor, c.message)
		if status != c.status || (c.code != "" && body["code"] != c.code) {
			t.Errorf("%s: %d %v", c.name, status, body)
		}
	}
	if _, err := pool.Exec(context.Background(), `UPDATE messages SET kind='deleted',body=NULL,deleted_at=now() WHERE id=$1`, draftMessage); err != nil {
		t.Fatal(err)
	}
	if status, body := postDraft(h, draftOwner, draftMessage); status != 409 || body["code"] != "message_deleted" {
		t.Errorf("tin đã xoá: %d %v", status, body)
	}
	if stub.SoGoi() != 0 {
		t.Fatalf("a refusal reached the model %d times", stub.SoGoi())
	}
}

func TestExpenseDraftKhongKhoaVaCoModel(t *testing.T) {
	pool := draftSchema(t)
	if status, body := postDraft(draftHandler(t, pool, nil), draftOwner, draftMessage); status != 503 || body["code"] != "chat_reader_not_configured" {
		t.Fatalf("keyless: %d %v", status, body)
	}
	stub := llm.NewStub(llm.Buoc{Text: `{"is_expense":true,"title":"Tiền nước","amount_text":"300k"}`})
	status, body := postDraft(draftHandler(t, pool, motluot.Moi(stub, 1)), draftOwner, draftMessage)
	if status != 200 || body["detected"] != true {
		t.Fatalf("%d %v", status, body)
	}
	draft := body["draft"].(map[string]any)
	if draft["title"] != "Tiền nước" || draft["amount_vnd"] != float64(300000) || draft["paid_by_id"] != draftMember || draft["needs_review"] != true {
		t.Fatalf("draft: %v", draft)
	}
	if shared := draft["shared_by"].([]any); len(shared) != 2 {
		t.Fatalf("shared_by: %v", shared)
	}
	if req := string(stub.YeuCau()[0]); strings.Contains(req, draftMember) || strings.Contains(req, "Chủ nhóm") || !strings.Contains(req, "300k") {
		t.Fatal("the model saw a person or missed the message")
	}
}

func postScan(t *testing.T, h http.Handler, path, actor, mime string, body []byte) (int, map[string]any) {
	t.Helper()
	var buf strings.Builder
	buf.WriteString("--b\r\nContent-Disposition: form-data; name=\"image\"; filename=\"x\"\r\nContent-Type: " + mime + "\r\n\r\n")
	buf.Write(body)
	buf.WriteString("\r\n--b--\r\n")
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(buf.String()))
	r.Header.Set("Content-Type", "multipart/form-data; boundary=b")
	if actor != "" {
		r.Header.Set("X-Actor-ID", actor)
		r.Header.Set("X-Actor-Roles", "member")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

// The two scan routes through the whole Go pipeline: no session is 401
// before the body is read, a keyless process 503 after the upload checks,
// and a reading the model returns reaches the wire without its confidence.
func TestScanRoutesQuaCuaTruocGo(t *testing.T) {
	pool := draftSchema(t)
	anh := anhThu(t)
	keyless := draftHandler(t, pool, nil)
	for _, path := range []string{"/receipts/scan", "/screenshots/scan"} {
		if status, _ := postScan(t, keyless, path, "", "image/png", anh); status != 401 {
			t.Errorf("%s anonymous: %d", path, status)
		}
		if status, body := postScan(t, keyless, path, draftOwner, "image/gif", anh); status != 415 || body["code"] != "unsupported_image_type" {
			t.Errorf("%s gif: %d %v", path, status, body)
		}
		status, body := postScan(t, keyless, path, draftOwner, "image/png", anh)
		if status != 503 || !strings.HasSuffix(body["code"].(string), "_reader_not_configured") {
			t.Errorf("%s keyless: %d %v", path, status, body)
		}
	}
	stub := llm.NewStub(llm.Buoc{Text: billDoc})
	status, body := postScan(t, draftHandler(t, pool, motluot.Moi(stub, 1)), "/receipts/scan", draftOwner, "image/png", anh)
	if status != 200 || body["items_total_vnd"] != float64(105000) || body["needs_review"] != false {
		t.Fatalf("%d %v", status, body)
	}
	if _, ok := body["confidence"]; ok {
		t.Fatal("the confidence reached the wire")
	}
}
