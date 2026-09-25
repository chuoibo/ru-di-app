//go:build postgres

package chatassist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aiharness/llm"
	aimetrics "mobile/services/core/internal/aiharness/metrics"
	"mobile/services/core/internal/aiharness/prompts"
)

// Nếp end to end on the Go engine (MOBILE_AI_ENGINE_NEP=go) against a real
// database: the same queue and sealed result as the brain path, the model a
// scripted stub injected through the engine's option -- never the network --
// and one metrics row per turn with no words in it.

type nepGo struct {
	f     fixture
	brain *nepGia
	stub  *llm.Stub
	log   *bytes.Buffer
}

func setupNepGo(t *testing.T, kich ...llm.Buoc) nepGo {
	t.Helper()
	brain := &nepGia{}
	f := setup(t, brain.serve)
	if err := aimetrics.Migrate(context.Background(), f.pool); err != nil {
		t.Fatal(err)
	}
	stub := llm.NewStub(kich...)
	var buf bytes.Buffer
	engine, err := aiharness.New(aiharness.WithModel(stub), aiharness.WithLogger(slog.New(slog.NewJSONHandler(&buf, nil))),
		aiharness.WithMaKiem("pg7canary9x2k"), aiharness.WithRetryWait(func(int) time.Duration { return 0 }))
	if err != nil {
		t.Fatal(err)
	}
	f.handler.WithNepEngine(engine)
	return nepGo{f: f, brain: brain, stub: stub, log: &buf}
}

func (n nepGo) ask(t *testing.T, body map[string]any) (string, NepInvocation) {
	t.Helper()
	return n.askLuc(t, body, time.Time{})
}

// askLuc is ask with the question's created_at moved to luc (when set)
// before the worker claims it, so «now» can be told apart from the moment
// the job runs.
func (n nepGo) askLuc(t *testing.T, body map[string]any, luc time.Time) (string, NepInvocation) {
	t.Helper()
	code, job, raw := n.f.nepPost(t, n.f.token, body)
	if code != 202 {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	if !luc.IsZero() {
		if _, err := n.f.pool.Exec(context.Background(), `UPDATE chat_ai_invocations SET created_at=$2 WHERE id=$1`, job.ID, luc); err != nil {
			t.Fatal(err)
		}
	}
	if ok, err := n.f.handler.ProcessOne(context.Background()); !ok || err != nil {
		t.Fatalf("ProcessOne=%v %v", ok, err)
	}
	w := n.f.request("GET", "/me/nep/ai-invocations/"+job.ID, n.f.token, nil)
	requireCode(t, w, 200)
	var done NepInvocation
	_ = json.Unmarshal(w.Body.Bytes(), &done)
	return job.ID, done
}

type hangSoDo struct {
	bot, lenh, guard, outGuard, ketThuc, version string
	code                                         *string
	lanThu, soGoi, buoc                          int
}

func (n nepGo) soDo(t *testing.T, id string) hangSoDo {
	t.Helper()
	var h hangSoDo
	err := n.f.pool.QueryRow(context.Background(), `SELECT bot,lenh,guard,out_guard,ket_thuc,prompt_version,code,lan_thu,so_goi_model,buoc FROM ai_turn_metrics WHERE invocation_id=$1`, id).
		Scan(&h.bot, &h.lenh, &h.guard, &h.outGuard, &h.ketThuc, &h.version, &h.code, &h.lanThu, &h.soGoi, &h.buoc)
	if err != nil {
		t.Fatalf("hàng số đo: %v", err)
	}
	return h
}

func TestNepQuaEngineGo(t *testing.T) {
	n := setupNepGo(t, llm.Buoc{Text: "Tối nay bạn đi dạo hồ nhé.", Usage: &genai.GenerateContentResponseUsageMetadata{PromptTokenCount: 700, CandidatesTokenCount: 12}})
	ctx := context.Background()
	if _, err := n.f.pool.Exec(ctx, `INSERT INTO messages(id,context_id,author_id,kind,body) VALUES($1,$2,$3,'text','Synthetic group chat sentinel')`, newID(), n.f.context, n.f.peer); err != nil {
		t.Fatal(err)
	}
	var tinTruoc int
	_ = n.f.pool.QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&tinTruoc)

	// The question was stored hours before the worker runs it (the sharing
	// window is share_expires_at, still open): «now» must be that instant,
	// Thursday 22:47 in Vietnam, not the worker's clock.
	luc := time.Date(2026, 9, 24, 15, 47, 5, 0, time.UTC)
	if time.Since(luc) < time.Hour {
		t.Fatalf("đồng hồ máy chạy test ở %v: mốc %v phải sớm hơn hàng giờ", time.Now(), luc)
	}
	id, done := n.askLuc(t, nepThan("@Rủ Đi tối nay đi đâu?"), luc)
	if done.Status != "succeeded" || done.Text == nil || *done.Text != "Tối nay bạn đi dạo hồ nhé." {
		t.Fatalf("kết quả: %+v", done)
	}
	// The brain was never asked: not for the answer, not for availability.
	if n.brain.calls != 0 || n.f.capabilityCalls.Load() != 0 {
		t.Fatalf("não bị gọi: %d lần, thăm dò %d", n.brain.calls, n.f.capabilityCalls.Load())
	}
	if n.stub.SoGoi() != 1 {
		t.Fatalf("%d lời gọi mô hình", n.stub.SoGoi())
	}
	req := string(n.stub.YeuCau()[0])
	// «Now» comes from the stored created_at, on Vietnam's clock, exactly;
	// and so does the evening «tối nay» names.
	for _, can := range []string{
		"Bây giờ: Thứ Năm 24/09/2026 22:47 (Asia/Ho_Chi_Minh, 2026-09-24T22:47:05+07:00)",
		"- «tối nay» là Thứ Năm 24/09/2026",
		"tối nay đi đâu?", "Mình thích yên tĩnh", "Vậy mình gợi ý chỗ vắng.", "man: explore", "tieuDe: Khám phá", "soLieu: soNguoi=4",
	} {
		if !strings.Contains(req, can) {
			t.Errorf("yêu cầu thiếu %q", can)
		}
	}
	for _, cam := range []string{"Synthetic group chat sentinel", "Synthetic caller", "Synthetic peer", "Synthetic job group", n.f.context, n.f.person, "@Rủ Đi"} {
		if strings.Contains(req, cam) {
			t.Errorf("yêu cầu chứa %q", cam)
		}
	}
	// Sealed as before: nothing published, the inputs gone.
	var tinSau int
	_ = n.f.pool.QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&tinSau)
	var promptNull, goiNull bool
	_ = n.f.pool.QueryRow(ctx, `SELECT prompt IS NULL, boi_canh IS NULL FROM chat_ai_invocations WHERE id=$1`, id).Scan(&promptNull, &goiNull)
	if tinSau != tinTruoc || !promptNull || !goiNull {
		t.Fatalf("tin %d→%d, prompt null=%v, boi_canh null=%v", tinTruoc, tinSau, promptNull, goiNull)
	}
	h := n.soDo(t, id)
	if h.bot != "nep" || h.lenh != "hoi" || h.guard != "proceed" || h.outGuard != "none" || h.ketThuc != "xong" || h.code != nil ||
		h.lanThu != 1 || h.soGoi != 1 || h.buoc != 1 || h.version != prompts.VersionNep() {
		t.Fatalf("hàng số đo: %+v", h)
	}
	// The one log line carries no words of the question or the answer.
	for _, w := range []string{"tối nay", "đi dạo", "yên tĩnh", "Khám phá", n.f.person} {
		if strings.Contains(n.log.String(), w) {
			t.Fatalf("log chứa %q", w)
		}
	}
}

// The money law on the Go path: refused with the engine's code, no model
// call, the inputs scrubbed, one metrics row saying so -- and no words.
func TestNepQuaEngineGoLuatTien(t *testing.T) {
	n := setupNepGo(t, llm.Buoc{Text: "không bao giờ tới"})
	id, done := n.ask(t, nepThan("chuyển khoản cho Minh 200k giúp mình"))
	if done.Status != "failed" || done.Code == nil || *done.Code != "nep_khong_cham_tien" || done.Text != nil {
		t.Fatalf("kết quả: %+v", done)
	}
	if n.stub.SoGoi() != 0 || n.brain.calls != 0 {
		t.Fatalf("luật tiền vẫn gọi: stub=%d não=%d", n.stub.SoGoi(), n.brain.calls)
	}
	var promptNull, goiNull bool
	_ = n.f.pool.QueryRow(context.Background(), `SELECT prompt IS NULL, boi_canh IS NULL FROM chat_ai_invocations WHERE id=$1`, id).Scan(&promptNull, &goiNull)
	if !promptNull || !goiNull {
		t.Fatal("câu hỏi bị từ chối vẫn nằm lại")
	}
	h := n.soDo(t, id)
	if h.guard != "refused" || h.ketThuc != "that_bai" || h.code == nil || *h.code != "nep_khong_cham_tien" || h.soGoi != 0 {
		t.Fatalf("hàng số đo: %+v", h)
	}
}

// An answer the output guard stops never reaches the sealed result.
func TestNepQuaEngineGoChanDauRa(t *testing.T) {
	// repo-guard: allow=vn-phone reason=synthetic-output-guard-fixture
	n := setupNepGo(t, llm.Buoc{Text: "Bạn gọi quán số 0912 345 678 nhé."})
	id, done := n.ask(t, nepThan("quán nào mở khuya?"))
	if done.Status != "failed" || done.Code == nil || *done.Code != "ai_tra_loi_bi_chan" || done.Text != nil {
		t.Fatalf("kết quả: %+v", done)
	}
	var resultNull bool
	_ = n.f.pool.QueryRow(context.Background(), `SELECT result IS NULL FROM chat_ai_invocations WHERE id=$1`, id).Scan(&resultNull)
	if !resultNull {
		t.Fatal("câu trả lời bị chặn vẫn nằm trong result")
	}
	if h := n.soDo(t, id); h.outGuard != "chan" || h.soGoi != 1 {
		t.Fatalf("hàng số đo: %+v", h)
	}
}

// A provider failure keeps the brain path's code, so the app's sentence is
// the one it always showed. Since slice 10 a 5xx is transient: the job goes
// back to the queue after a backoff (retryLater) while it has attempts left,
// and the attempt that has none fails it with that code.
func TestNepQuaEngineGoLoiNhaCungCap(t *testing.T) {
	loi := llm.Buoc{Loi: genai.APIError{Code: 500}}
	n := setupNepGo(t, loi, loi, loi)
	ctx := context.Background()
	code, job, raw := n.f.nepPost(t, n.f.token, nepThan("đi đâu?"))
	if code != 202 {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	for lan := 1; lan <= 3; lan++ {
		if lan > 1 {
			// The backoff is the queue's to wait out, not this test's.
			if _, err := n.f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET available_at=clock_timestamp() WHERE id=$1`, job.ID); err != nil {
				t.Fatal(err)
			}
		}
		if ok, err := n.f.handler.ProcessOne(ctx); !ok || err != nil {
			t.Fatalf("lần %d: ProcessOne=%v %v", lan, ok, err)
		}
		var status string
		var ma *string
		var cho float64
		if err := n.f.pool.QueryRow(ctx, `SELECT status, code, EXTRACT(EPOCH FROM available_at-clock_timestamp()) FROM chat_ai_invocations WHERE id=$1`, job.ID).Scan(&status, &ma, &cho); err != nil {
			t.Fatal(err)
		}
		if lan < 3 {
			// 1 s after the first attempt, 4 s after the second, ±20 %.
			tu, den := 0.7, 1.2
			if lan == 2 {
				tu, den = 3.1, 4.8
			}
			if status != "queued" || ma != nil || cho < tu || cho > den {
				t.Fatalf("lần %d: status=%s code=%v chờ=%.2fs, muốn queued sau %.1f–%.1fs", lan, status, ma, cho, tu, den)
			}
			continue
		}
		if status != "failed" || ma == nil || *ma != "provider_unavailable" {
			t.Fatalf("lần cuối: status=%s code=%v", status, ma)
		}
	}
	w := n.f.request("GET", "/me/nep/ai-invocations/"+job.ID, n.f.token, nil)
	requireCode(t, w, 200)
	var done NepInvocation
	_ = json.Unmarshal(w.Body.Bytes(), &done)
	if done.Status != "failed" || done.Code == nil || *done.Code != "provider_unavailable" {
		t.Fatalf("kết quả: %+v", done)
	}
	var hang int
	var ma string
	if err := n.f.pool.QueryRow(ctx, `SELECT count(*), min(code) FROM ai_turn_metrics WHERE invocation_id=$1 AND ket_thuc='that_bai'`, job.ID).Scan(&hang, &ma); err != nil || hang != 3 || ma != "provider_unavailable" {
		t.Fatalf("hàng số đo: %d %q %v", hang, ma, err)
	}
	if n.stub.SoGoi() != 3 {
		t.Fatalf("stub trả lời %d lần, muốn 3", n.stub.SoGoi())
	}
}

// A turn stopped from outside is not a provider failure, and no metrics row
// says otherwise. What happens to the job depends on who stopped it:
//
//   - its lease went (cancelled, taken over): the job is not this worker's,
//     and nothing is touched; once the lease lapses the next claim runs it;
//   - the worker is stopping (SIGTERM): the job is released back to the queue
//     at once (design 02 §4 step 9) -- queued, the attempt given back, a new
//     enqueue_seq and its outbox row -- so another worker takes it now rather
//     than after the lease.
func TestNepQuaEngineGoHuyTuNgoai(t *testing.T) {
	type hang struct {
		status, lease      string
		attempts           int
		seq                int64
		ketQua, ma, conHoi bool
		soDo, outbox       int
	}
	doc := func(t *testing.T, n nepGo, id string) hang {
		t.Helper()
		var h hang
		if err := n.f.pool.QueryRow(context.Background(), `SELECT status, COALESCE(lease_id::text,''), attempts, enqueue_seq, result IS NOT NULL, code IS NOT NULL, prompt IS NOT NULL,
			(SELECT count(*) FROM ai_turn_metrics WHERE invocation_id=$1), (SELECT count(*) FROM job_outbox WHERE ref_id=$1)
			FROM chat_ai_invocations WHERE id=$1`, id).
			Scan(&h.status, &h.lease, &h.attempts, &h.seq, &h.ketQua, &h.ma, &h.conHoi, &h.soDo, &h.outbox); err != nil {
			t.Fatal(err)
		}
		return h
	}
	xongLanSau := func(t *testing.T, n nepGo, id string, lanThu int) {
		t.Helper()
		if ok, err := n.f.handler.ProcessOne(context.Background()); !ok || err != nil {
			t.Fatalf("ProcessOne=%v %v", ok, err)
		}
		w := n.f.request("GET", "/me/nep/ai-invocations/"+id, n.f.token, nil)
		requireCode(t, w, 200)
		var done NepInvocation
		_ = json.Unmarshal(w.Body.Bytes(), &done)
		if done.Status != "succeeded" || done.Text == nil || *done.Text != "Đi dạo hồ nhé." {
			t.Fatalf("lần sau: %+v", done)
		}
		if h := n.soDo(t, id); h.lanThu != lanThu || h.ketThuc != "xong" {
			t.Fatalf("hàng số đo lần sau: %+v", h)
		}
	}
	batDau := func(t *testing.T) (nepGo, string, work) {
		t.Helper()
		n := setupNepGo(t, llm.Buoc{Text: "không bao giờ tới", Cho: time.Minute}, llm.Buoc{Text: "Đi dạo hồ nhé."})
		n.f.handler.WithWorker(fastWorker())
		code, job, raw := n.f.nepPost(t, n.f.token, nepThan("đi đâu?"))
		if code != 202 {
			t.Fatalf("status=%d body=%s", code, raw)
		}
		j, ok, err := n.f.handler.claim(context.Background())
		if !ok || err != nil {
			t.Fatalf("claim=%v %v", ok, err)
		}
		return n, job.ID, j
	}

	t.Run("mất lease", func(t *testing.T) {
		n, id, j := batDau(t)
		ctx := context.Background()
		other := newID()
		time.AfterFunc(50*time.Millisecond, func() {
			_, _ = n.f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET lease_id=$2 WHERE id=$1`, id, other)
		})
		if err := n.f.handler.runJob(ctx, j); !errors.Is(err, aiharness.ErrHuy) {
			t.Fatalf("runJob: %v", err)
		}
		if h := doc(t, n, id); h.status != "running" || h.lease != other || h.ketQua || h.ma || !h.conHoi || h.soDo != 0 || h.seq != 1 {
			t.Fatalf("job bị đụng: %+v", h)
		}
		if _, err := n.f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
		xongLanSau(t, n, id, 2)
	})

	t.Run("worker dừng", func(t *testing.T) {
		n, id, j := batDau(t)
		runCtx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(50*time.Millisecond, cancel)
		if err := n.f.handler.runJob(runCtx, j); err != nil {
			t.Fatalf("runJob: %v", err)
		}
		if h := doc(t, n, id); h.status != "queued" || h.lease != "" || h.attempts != 0 || h.seq != 2 || h.outbox != 2 || h.ketQua || h.ma || !h.conHoi || h.soDo != 0 {
			t.Fatalf("job chưa được nhả đúng: %+v", h)
		}
		// The attempt was given back, so the next run is attempt 1 again.
		xongLanSau(t, n, id, 1)
	})
}
