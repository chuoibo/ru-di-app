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
// the one it always showed.
func TestNepQuaEngineGoLoiNhaCungCap(t *testing.T) {
	n := setupNepGo(t, llm.Buoc{Loi: genai.APIError{Code: 500}})
	id, done := n.ask(t, nepThan("đi đâu?"))
	if done.Status != "failed" || done.Code == nil || *done.Code != "provider_unavailable" {
		t.Fatalf("kết quả: %+v", done)
	}
	if h := n.soDo(t, id); h.ketThuc != "that_bai" || h.code == nil || *h.code != "provider_unavailable" {
		t.Fatalf("hàng số đo: %+v", h)
	}
}

// A turn stopped from outside (the heartbeat cancelled the job, or the worker
// is stopping) is not a provider failure: the job is left to its lease, not
// failed with provider_unavailable, and no metrics row says otherwise. Once
// the lease lapses the next claim runs it again and answers.
func TestNepQuaEngineGoHuyTuNgoai(t *testing.T) {
	n := setupNepGo(t, llm.Buoc{Text: "không bao giờ tới", Cho: time.Minute}, llm.Buoc{Text: "Đi dạo hồ nhé."})
	ctx := context.Background()
	code, job, raw := n.f.nepPost(t, n.f.token, nepThan("đi đâu?"))
	if code != 202 {
		t.Fatalf("status=%d body=%s", code, raw)
	}
	j, ok, err := n.f.handler.claim(ctx)
	if !ok || err != nil {
		t.Fatalf("claim=%v %v", ok, err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	time.AfterFunc(50*time.Millisecond, cancel)
	if err := n.f.handler.runJob(runCtx, j); !errors.Is(err, aiharness.ErrHuy) {
		t.Fatalf("runJob: %v", err)
	}
	var status, lease string
	var coKetQua, coMa, conHoi bool
	if err := n.f.pool.QueryRow(ctx, `SELECT status, COALESCE(lease_id::text,''), result IS NOT NULL, code IS NOT NULL, prompt IS NOT NULL FROM chat_ai_invocations WHERE id=$1`, job.ID).
		Scan(&status, &lease, &coKetQua, &coMa, &conHoi); err != nil {
		t.Fatal(err)
	}
	if status != "running" || lease != j.lease || coKetQua || coMa || !conHoi {
		t.Fatalf("job bị đụng: status=%s lease khớp=%v kết quả=%v mã=%v còn câu hỏi=%v", status, lease == j.lease, coKetQua, coMa, conHoi)
	}
	var hang int
	_ = n.f.pool.QueryRow(ctx, `SELECT count(*) FROM ai_turn_metrics WHERE invocation_id=$1`, job.ID).Scan(&hang)
	if hang != 0 {
		t.Fatalf("%d hàng số đo cho lượt bị huỷ", hang)
	}
	// The lease lapses; the job is claimed again and answered.
	if _, err := n.f.pool.Exec(ctx, `UPDATE chat_ai_invocations SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, job.ID); err != nil {
		t.Fatal(err)
	}
	if ok, err := n.f.handler.ProcessOne(ctx); !ok || err != nil {
		t.Fatalf("ProcessOne=%v %v", ok, err)
	}
	w := n.f.request("GET", "/me/nep/ai-invocations/"+job.ID, n.f.token, nil)
	requireCode(t, w, 200)
	var done NepInvocation
	_ = json.Unmarshal(w.Body.Bytes(), &done)
	if done.Status != "succeeded" || done.Text == nil || *done.Text != "Đi dạo hồ nhé." {
		t.Fatalf("lần hai: %+v", done)
	}
	if h := n.soDo(t, job.ID); h.lanThu != 2 || h.ketThuc != "xong" {
		t.Fatalf("hàng số đo lần hai: %+v", h)
	}
}
