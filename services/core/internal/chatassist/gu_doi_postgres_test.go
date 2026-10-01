//go:build postgres

package chatassist

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"sort"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"

	"mobile/services/core/internal/aidoc"
	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aiharness/llm"
	aimetrics "mobile/services/core/internal/aiharness/metrics"
	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/gudoi"
)

// A couple's shared taste end to end (ADR-0048) against a real database: the
// model calls gu_hai_ban, the tool reads through aidoc in a READ ONLY
// transaction, the card names whose taste it used (doc.gu), and the worker
// re-checks every such person in the transaction that publishes -- a
// revocation or a broken «Một đôi» while the model writes posts no card.
// chat-capabilities says where the caller's own switch stands (gu_chat).

// setupCapGu is setupCap with the engine's couple's-taste port on the test's
// own pool.
func setupCapGu(t *testing.T, kich ...llm.Buoc) nhomGo {
	t.Helper()
	f := setup(t, nil)
	if err := aimetrics.Migrate(context.Background(), f.pool); err != nil {
		t.Fatal(err)
	}
	stub := llm.NewStub(kich...)
	var buf bytes.Buffer
	engine, err := aiharness.New(aiharness.WithModel(stub), aiharness.WithLogger(slog.New(slog.NewJSONHandler(&buf, nil))),
		aiharness.WithMaKiem("pg9nhom7x2kq"), aiharness.WithRetryWait(func(int) time.Duration { return 0 }),
		aiharness.WithNguon(tools.NguonDuLieu{Doi: aidoc.Doc{C: aidoc.Moi(f.pool, 0)}}))
	if err != nil {
		t.Fatal(err)
	}
	f.handler.WithNhomEngine(engine)
	f.exec(t, `UPDATE contexts SET kind='pair',pair_key=$2 WHERE id=$1`, f.context, f.person+":"+f.peer)
	return nhomGo{f: f, stub: stub}
}

// kichGu is a couple's turn on the tool path: the router, the model's call
// of gu_hai_ban, the answer, the verifier citing the first evidence.
func kichGu() []llm.Buoc {
	return []llm.Buoc{
		ruNhom(map[string]any{"huong": "tac_tu", "y_dinh": []string{"find_places"}}),
		{Goi: &genai.FunctionCall{Name: "gu_hai_ban", Args: map[string]any{}}},
		{Text: "Hai bạn cùng thích cafe, ghé một quán cafe yên tĩnh nhé."},
		kiemNhom("ho_tro", "e1"),
	}
}

func (f fixture) chuKy(t *testing.T) string {
	t.Helper()
	var id string
	if err := f.pool.QueryRow(context.Background(), `SELECT c.id::text FROM pair_notebook_cycles c JOIN pair_notebooks n ON n.id=c.notebook_id WHERE n.context_id=$1`, f.context).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// chiaGu files person's own `chia_gu` at luc, the way the notebook does: a
// proposal completed as it is filed, and the person's yes on it.
func (f fixture) chiaGu(t *testing.T, person string, luc time.Time) {
	t.Helper()
	p := newID()
	f.exec(t, `INSERT INTO pair_consent_proposals(id,cycle_id,purpose,proposed_by_id,completed_at,expires_at) VALUES($1,$2,'chia_gu',$3,$4,$4::timestamptz+interval '7 days')`, p, f.chuKy(t), person, luc)
	f.exec(t, `INSERT INTO pair_consents(id,proposal_id,person_id,granted_at) VALUES($1,$2,$3,$4)`, newID(), p, person, luc)
}

// thuHoi is the notebook's revocation of person's purpose (RevokeConsents).
func (f fixture) thuHoi(t *testing.T, person, purpose string) {
	t.Helper()
	f.exec(t, `UPDATE pair_consents k SET revoked_at=clock_timestamp() FROM pair_consent_proposals p WHERE p.id=k.proposal_id AND p.purpose=$2 AND k.person_id=$1 AND k.revoked_at IS NULL`, person, purpose)
}

func (f fixture) gu(t *testing.T, person string, tags ...string) {
	t.Helper()
	for _, tag := range tags {
		f.exec(t, `INSERT INTO person_interests(id,person_id,tag) VALUES($1,$2,$3)`, newID(), person, tag)
	}
}

// guCuaThe is the card's doc.gu (nil when absent).
func guCuaThe(t *testing.T, card []byte) []string {
	t.Helper()
	var c struct {
		Payload struct {
			Doc struct {
				Gu []string `json:"gu"`
			} `json:"doc"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(card, &c); err != nil {
		t.Fatal(err)
	}
	return c.Payload.Doc.Gu
}

func capGu(n nhomGo, t *testing.T) {
	t.Helper()
	n.f.batDoi(t, n.f.person, n.f.peer)
	moi := gudoi.MocChat().Add(time.Hour)
	n.f.chiaGu(t, n.f.person, moi)
	n.f.chiaGu(t, n.f.peer, moi)
	n.f.gu(t, n.f.person, "cafe", "outdoor")
	n.f.gu(t, n.f.peer, "cafe", "karaoke")
}

// Both shared under the chat's wording: the answer is posted, its card says
// whose taste it used, and the metrics row names the tool and nothing it
// returned.
func TestCapDoiDungGuTheGhiTen(t *testing.T) {
	n := setupCapGu(t, kichGu()...)
	capGu(n, t)
	job, code, ma := n.goiCap(t, "hoi")
	if job == nil {
		t.Fatalf("%d %s", code, ma)
	}
	n.chayViec(t)
	k := n.ket(t, job.ID)
	if k.status != "succeeded" || k.reply == nil {
		t.Fatalf("%s %v", k.status, k.code)
	}
	// In the cycle's participant order; the two are one statement's rows
	// here, so compare as a set.
	gu := guCuaThe(t, k.card)
	sort.Strings(gu)
	if strings.Join(gu, "|") != "Synthetic caller|Synthetic peer" {
		t.Fatalf("doc.gu %v in %s", gu, k.card)
	}
	if k.bot != "doi" {
		t.Fatalf("bot %s", k.bot)
	}
	var cc []string
	if err := n.f.pool.QueryRow(context.Background(), `SELECT cong_cu FROM ai_turn_metrics WHERE invocation_id=$1`, job.ID).Scan(&cc); err != nil || strings.Join(cc, ",") != "gu_hai_ban" {
		t.Fatalf("cong_cu %v %v", cc, err)
	}
	// The taste reached the model under an alias, a person id never.
	var req string
	for _, y := range n.stub.YeuCau() {
		req += string(y)
	}
	if !strings.Contains(req, "Cafe") || strings.Contains(req, n.f.peer) || strings.Contains(req, "Synthetic constraint") {
		t.Fatal("the taste did not reach the model as evidence, or a person id did")
	}
}

// Only the caller shared (the other person's consent is from before the
// cutoff): only the caller's taste is read and named.
func TestCapDoiChiGuNguoiDongYMoi(t *testing.T) {
	n := setupCapGu(t, ruNhom(map[string]any{"huong": "tac_tu", "y_dinh": []string{"find_places"}}),
		llm.Buoc{Goi: &genai.FunctionCall{Name: "gu_hai_ban", Args: map[string]any{}}},
		llm.Buoc{Text: "Bạn thích cafe, hai bạn ghé một quán cafe nhé."}, kiemNhom("ho_tro", "e1"))
	n.f.batDoi(t, n.f.person, n.f.peer)
	n.f.chiaGu(t, n.f.person, gudoi.MocChat().Add(time.Hour))
	n.f.chiaGu(t, n.f.peer, gudoi.MocChat().Add(-time.Hour))
	n.f.gu(t, n.f.person, "cafe")
	n.f.gu(t, n.f.peer, "karaoke")
	job, code, ma := n.goiCap(t, "hoi")
	if job == nil {
		t.Fatalf("%d %s", code, ma)
	}
	n.chayViec(t)
	k := n.ket(t, job.ID)
	if k.status != "succeeded" || strings.Join(guCuaThe(t, k.card), "|") != "Synthetic caller" {
		t.Fatalf("%s %s", k.status, k.card)
	}
	var req string
	for _, y := range n.stub.YeuCau() {
		req += string(y)
	}
	if strings.Contains(req, "Karaoke") {
		t.Fatal("an old consent's taste reached the model")
	}
}

// A revocation, or «Một đôi» broken, while the model writes: the worker asks
// again in the publishing transaction and posts nothing; the job fails with
// the sharing code.
func TestCapDoiThuHoiGiuaLuotKhongCoThe(t *testing.T) {
	for name, dong := range map[string]func(n nhomGo, t *testing.T){
		"thu hồi chia_gu": func(n nhomGo, t *testing.T) { n.f.thuHoi(t, n.f.peer, "chia_gu") },
		"thu hồi bat_doi": func(n nhomGo, t *testing.T) { n.f.thuHoi(t, n.f.person, "bat_doi") },
	} {
		t.Run(name, func(t *testing.T) {
			n := setupCapGu(t, kichGu()...)
			capGu(n, t)
			job, code, ma := n.goiCap(t, "hoi")
			if job == nil {
				t.Fatalf("%d %s", code, ma)
			}
			n.f.handler.truocChot = func(context.Context) { dong(n, t) }
			n.chayViec(t)
			status, c, cards := n.trangThai(t, job.ID)
			if status != "failed" || c != "sharing_unavailable" || cards != 0 || n.stub.SoGoi() != 4 {
				t.Fatalf("%s %s thẻ %d, mô hình %d lần", status, c, cards, n.stub.SoGoi())
			}
		})
	}
	// Control: the same revocation of somebody whose taste the answer did
	// not read (a turn that never called the tool) does not stop the card.
	n := setupCapGu(t, kichCap()...)
	capGu(n, t)
	job, code, ma := n.goiCap(t, "hoi")
	if job == nil {
		t.Fatalf("%d %s", code, ma)
	}
	n.f.handler.truocChot = func(context.Context) { n.f.thuHoi(t, n.f.peer, "chia_gu") }
	n.chayViec(t)
	k := n.ket(t, job.ID)
	if status, _, cards := n.trangThai(t, job.ID); status != "succeeded" || cards != 1 || guCuaThe(t, k.card) != nil {
		t.Fatalf("a turn that read no taste: %s %d %s", status, cards, k.card)
	}
}

// chat-capabilities' gu_chat: null outside a couple; in a couple the
// caller's switch (off, on, on under the older wording) and whether the
// other's taste may be used in the chat.
func TestCapDoiKhaNangGuChat(t *testing.T) {
	n := setupCapGu(t)
	doc := func(token string) map[string]any {
		t.Helper()
		w := n.f.request("GET", "/contexts/"+n.f.context+"/chat-capabilities", token, nil)
		requireCode(t, w, 200)
		var v map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		g, ok := v["gu_chat"]
		if !ok {
			t.Fatalf("no gu_chat: %s", w.Body.String())
		}
		if g == nil {
			return nil
		}
		return g.(map[string]any)
	}
	if g := doc(n.f.token); g != nil {
		t.Fatalf("a friends' chat of two has gu_chat %v", g)
	}
	n.f.batDoi(t, n.f.person, n.f.peer)
	if g := doc(n.f.token); g["cua_toi"] != "tat" || g["nguoi_kia"] != false {
		t.Fatalf("%v", g)
	}
	n.f.chiaGu(t, n.f.person, gudoi.MocChat().Add(-time.Hour))
	n.f.chiaGu(t, n.f.peer, gudoi.MocChat().Add(time.Hour))
	if g := doc(n.f.token); g["cua_toi"] != "can_bat_lai" || g["nguoi_kia"] != true {
		t.Fatalf("%v", g)
	}
	if g := doc(n.f.peerToken); g["cua_toi"] != "bat" || g["nguoi_kia"] != false {
		t.Fatalf("peer: %v", g)
	}
	// Re-consent: off, then on again under the chat's wording.
	n.f.thuHoi(t, n.f.person, "chia_gu")
	n.f.chiaGu(t, n.f.person, gudoi.MocChat().Add(2*time.Hour))
	if g := doc(n.f.token); g["cua_toi"] != "bat" || g["nguoi_kia"] != true {
		t.Fatalf("%v", g)
	}
	// A group has none.
	n.f.exec(t, `UPDATE contexts SET kind='group',pair_key=NULL WHERE id=$1`, n.f.context)
	if g := doc(n.f.token); g != nil {
		t.Fatalf("a group has gu_chat %v", g)
	}
}
