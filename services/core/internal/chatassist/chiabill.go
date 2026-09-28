package chatassist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"mobile/services/core/internal/brain"
	"mobile/services/core/internal/domain/allocator"
	"mobile/services/core/internal/domain/chatintent"
	"mobile/services/core/internal/domain/companion"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/treejson"
)

// chia_bill on the invocation engine (ADR-0036 §2.9).
//
// It reads the turns the caller handed over through the EXISTING brain skill
// `chat-expense`, one message at a time, exactly the contract v1's
// draftExpensesFromChat used: `{text}` in, `{is_expense,title,amount_vnd,
// needs_review}` out. The skill has no identity channel on purpose, so who
// paid is never the model's answer: it is the author of the shared message,
// read from `messages.author_id` (tacGia), or the caller for the caller's own
// words. Nothing here reads `messages.body`.
//
// AI does not touch money. The outcome is a text card a person reads plus the
// skill's drafts, as-is, on the invocation's `result` column. No ledger row,
// no expense, no obligation, and no per-person share is computed: a split the
// server invented would be an allocation, and allocations belong to the
// confirm flow and the allocator, where Σ = total is enforced.

// maxLuotDocChi bounds model calls per invocation. It is v1's
// chiaBillModelCalls, kept so the cost of one /chia-bill does not change with
// the move onto the queue.
const maxLuotDocChi = 8

// Brain refusals that describe one message rather than the provider: that
// message is skipped and the rest are still read. Everything else means the
// reader is not there, and the whole job fails honestly.
var tuChoiMotTin = map[string]bool{
	"chat_expense_unreadable":           true,
	"chat_expense_model_named_a_person": true,
	"brain_request_invalid":             true,
}

// nguonKhoan is one text the skill will read and the person it would bill as
// payer if the text turns out to be an expense.
type nguonKhoan struct {
	text  string
	payer string
	// The shared message it came from; "" for the caller's own prompt.
	source string
}

// nguonChiaBill picks what the skill reads: text turns of real people whose
// author the server could confirm, then the caller's own words, newest last,
// cut to the newest maxLuotDocChi. A turn that is itself a command is not an
// expense and is never paid for.
func nguonChiaBill(goi []byte, prompt, caller string, authors map[string]string) ([]nguonKhoan, error) {
	out := []nguonKhoan{}
	if len(goi) > 0 {
		var bc bundle
		if err := json.Unmarshal(goi, &bc); err != nil {
			return nil, err
		}
		for _, l := range bc.Luot {
			if l.Loai != "chu" || l.Vai == "ai" {
				continue
			}
			// The payer is whoever the database says wrote it. A turn with no
			// confirmed author has nobody to bill, so it is not read at all.
			payer, ok := authors[l.ID]
			if !ok {
				continue
			}
			text := strings.TrimSpace(l.Chu)
			if text == "" || chatintent.Parse(text) != nil {
				continue
			}
			out = append(out, nguonKhoan{text: text, payer: payer, source: l.ID})
		}
	}
	// "/chia-bill mình trả 300k tiền nước" carries an expense of its own; the
	// bare command, or a request for something else, does not.
	rest := strings.TrimSpace(prompt)
	if p := chatintent.Parse(rest); p != nil {
		rest = ""
		if p.Intent == chatintent.ChiaBill || p.Intent == chatintent.Mention {
			rest = strings.TrimSpace(p.Args)
		}
	}
	if rest != "" {
		out = append(out, nguonKhoan{text: rest, payer: caller})
	}
	if len(out) > maxLuotDocChi {
		out = out[len(out)-maxLuotDocChi:]
	}
	return out, nil
}

// docKhoan is the brain call, a function so the pure part is testable
// without a network.
type docKhoan func(ctx context.Context, text string) (pyjson.Value, error)

func (h *Handler) docKhoan(ctx context.Context, text string) (pyjson.Value, error) {
	body := pyjson.NewOrderedMap()
	body.Set("text", pyjson.String(text))
	return h.brain.PostJSONContext(ctx, "chat-expense", body)
}

type khoanNhap struct {
	title  string
	amount int64
	payer  string
	source string
}

var errKhongDoc = errors.New("chat expense reading malformed")

// docMotKhoan re-checks the skill's answer instead of trusting the seam. The
// amount must already be an integer number of đồng: a float is refused, not
// rounded, because rounding is exactly where money rule 1 is broken.
func docMotKhoan(raw pyjson.Value) (title string, amount int64, isExpense bool, err error) {
	obj, ok := raw.(*pyjson.OrderedMap)
	if !ok {
		return "", 0, false, errKhongDoc
	}
	flag, _ := obj.Get("is_expense")
	on, ok := flag.(pyjson.Bool)
	if !ok {
		return "", 0, false, errKhongDoc
	}
	if !bool(on) {
		return "", 0, false, nil
	}
	tv, _ := obj.Get("title")
	t, ok := tv.(pyjson.String)
	if !ok || strings.TrimSpace(string(t)) == "" {
		return "", 0, false, errKhongDoc
	}
	av, _ := obj.Get("amount_vnd")
	a, ok := av.(pyjson.Int)
	if !ok {
		return "", 0, false, errKhongDoc
	}
	n, ok := a.Int64()
	if !ok || n <= 0 || n > int64(allocator.MaxAmountVND) {
		return "", 0, false, errKhongDoc
	}
	return strings.TrimSpace(string(t)), n, true, nil
}

// chiaBill reads each source and keeps the expenses, oldest first. code is
// the job's failure code when there is nothing to publish.
func chiaBill(ctx context.Context, doc docKhoan, nguon []nguonKhoan) ([]khoanNhap, string) {
	out := []khoanNhap{}
	for _, n := range nguon {
		raw, err := doc(ctx, n.text)
		if err != nil {
			var refused *brain.Error
			if errors.As(err, &refused) && tuChoiMotTin[refused.Code] {
				continue
			}
			return nil, "provider_unavailable"
		}
		title, amount, isExpense, err := docMotKhoan(raw)
		if err != nil || !isExpense {
			continue
		}
		out = append(out, khoanNhap{title: title, amount: amount, payer: n.payer, source: n.source})
	}
	if len(out) == 0 {
		return nil, "chia_bill_no_expenses"
	}
	return out, ""
}

// thanhVienDangO is the active members' person ids in byte order, the same
// "shared by everyone still here" v1 proposed.
func thanhVienDangO(memberships []repo.Membership) []string {
	ids := []string{}
	for _, m := range memberships {
		if m.State == "active" {
			ids = append(ids, m.PersonID)
		}
	}
	sort.Strings(ids)
	return ids
}

// ketQuaChiaBill is the structured outcome stored on the invocation: v1's
// expense_draft payload, drafts as the skill produced them, every one marked
// for review. It carries no split, only who paid, how much, and the proposed
// set of people to share it.
func ketQuaChiaBill(drafts []khoanNhap, shared []string) ([]byte, error) {
	list := pyjson.List{}
	for _, d := range drafts {
		e := pyjson.NewOrderedMap()
		e.Set("title", pyjson.String(d.title))
		e.Set("amount_vnd", pyjson.NewInt(d.amount))
		e.Set("paid_by_id", pyjson.String(d.payer))
		people := pyjson.List{}
		for _, id := range shared {
			people = append(people, pyjson.String(id))
		}
		e.Set("shared_by", people)
		if d.source == "" {
			e.Set("source_message_id", pyjson.Null{})
		} else {
			e.Set("source_message_id", pyjson.String(d.source))
		}
		e.Set("needs_review", pyjson.Bool(true))
		list = append(list, e)
	}
	out := pyjson.NewOrderedMap()
	out.Set("kind", pyjson.String("expense_draft"))
	out.Set("drafts", list)
	return pyjson.Dumps(out)
}

const nguoiKhongTen = "Một người trong nhóm"

// nhanNguoiTra names a payer on a card the whole room reads: the display name
// when it is safe, never an account id, never «Mình» (which would name each
// reader in turn).
func nhanNguoiTra(memberships []repo.Membership) map[string]string {
	out := map[string]string{}
	for _, m := range memberships {
		if name := tenThanhVien(m); name != "" {
			out[m.PersonID] = name
		}
	}
	return out
}

func catChu(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:n-1])) + "…"
}

// dong formats whole đồng with dot grouping: 1250000 -> "1.250.000đ".
func dong(n int64) string {
	s := fmt.Sprintf("%d", n)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return b.String() + "đ"
}

// theChiaBill is the card text. It says what the AI read, and says twice in
// plain words that nothing was written: a proposal to confirm, not a record.
// It always fits companion.MaxText, dropping lines into «và N khoản nữa»
// rather than letting the card be cut mid-sentence.
func theChiaBill(drafts []khoanNhap, names map[string]string, soNguoi int) string {
	var tong int64
	for _, d := range drafts {
		tong += d.amount
	}
	dongKhoan := make([]string, len(drafts))
	for i, d := range drafts {
		name, ok := names[d.payer]
		if !ok {
			name = nguoiKhongTen
		}
		dongKhoan[i] = fmt.Sprintf("• %s trả %s: %s", catChu(name, 24), dong(d.amount), catChu(d.title, 40))
	}
	dau := "Đề xuất chia bill, chưa ghi vào sổ:"
	cuoi := fmt.Sprintf("Tổng %s. Đề xuất chia đều cho %d người đang trong nhóm.\nMọi người xem lại rồi xác nhận ở mục Chia bill. Rủ Đi AI không tự ghi khoản nào.", dong(tong), soNguoi)
	for hien := len(dongKhoan); hien >= 0; hien-- {
		phan := append([]string{dau}, dongKhoan[:hien]...)
		if con := len(dongKhoan) - hien; con > 0 {
			phan = append(phan, fmt.Sprintf("• Và %d khoản nữa", con))
		}
		phan = append(phan, cuoi)
		text := strings.Join(phan, "\n")
		if utf8.RuneCountInString(text) <= companion.MaxText {
			return text
		}
	}
	return dau + "\n" + cuoi
}

// theChu is a text card through GroundCard, so the published shape is the
// one parity pins and the client already draws; no new card kind.
func theChu(text string) ([]byte, error) {
	payload := pyjson.NewOrderedMap()
	payload.Set("text", pyjson.String(text))
	raw := pyjson.NewOrderedMap()
	raw.Set("kind", pyjson.String("text"))
	raw.Set("payload", payload)
	grounded, err := companion.GroundCard(treejson.To(raw), nil)
	if err != nil {
		return nil, err
	}
	return pyjson.Dumps(treejson.From(grounded))
}

func (h *Handler) processChiaBill(ctx context.Context, j work, dap dapThem) error {
	nguon, err := nguonChiaBill(j.goi, j.prompt, j.person, dap.authors)
	if err != nil {
		return h.finishFailure(ctx, j, "invalid_ai_result")
	}
	inference, cancel := context.WithTimeout(ctx, 60*time.Second)
	drafts, code := chiaBill(inference, h.docKhoan, nguon)
	cancel()
	if code != "" {
		return h.finishFailure(ctx, j, code)
	}
	shared := thanhVienDangO(dap.memberships)
	result, err := ketQuaChiaBill(drafts, shared)
	if err != nil {
		return h.finishFailure(ctx, j, "invalid_ai_result")
	}
	card, err := theChu(theChiaBill(drafts, nhanNguoiTra(dap.memberships), len(shared)))
	if err != nil {
		return h.finishFailure(ctx, j, "invalid_ai_result")
	}
	return h.publish(ctx, j, card, result)
}

// ketQuaHoacNull keeps `result` NULL for a job with no structured outcome,
// for the same reason goiHoacNull keeps `boi_canh` NULL.
func ketQuaHoacNull(result []byte) any {
	if len(result) == 0 {
		return nil
	}
	return json.RawMessage(result)
}
