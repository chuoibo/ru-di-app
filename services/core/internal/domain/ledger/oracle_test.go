package ledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/oracletest"
)

// testdata/python_ledger*.json is rendered by scripts/render_domain_w3_goldens.py
// from the real app.domain.ledger, and testdata/python_context_balances*.json
// from the real ApiService.get_context_balances over a stub repository, in the
// parity API image. Every case is replayed here, and every int is compared
// exactly, inside int64 or not.

func text(value any) (string, error) {
	s, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%v is not a str", value)
	}
	return s, nil
}

// vnd reads a stored amount, which the service only ever holds in int64.
func vnd(value any) (money.VND, error) {
	n, ok := value.(int64)
	if !ok {
		return 0, fmt.Errorf("%v (%T) is not an int inside int64", value, value)
	}
	return money.VND(n), nil
}

// sum reads a derived sum: any int, or None as nil.
func sum(value any) (*big.Int, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case int64:
		return big.NewInt(v), nil
	case oracletest.BigInt:
		n, ok := new(big.Int).SetString(string(v), 10)
		if !ok {
			return nil, fmt.Errorf("bad int %q", v)
		}
		return n, nil
	}
	return nil, fmt.Errorf("%v (%T) is not an int or None", value, value)
}

// exact renders a *big.Int as oracletest.Plain decodes Python's int.
func exact(n *big.Int) any {
	if n.IsInt64() {
		return n.Int64()
	}
	return oracletest.BigInt(n.String())
}

func list(value any) ([]any, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%v is not a list", value)
	}
	return items, nil
}

func fields(value any, keys ...string) (map[string]any, error) {
	row, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%v is not a dict", value)
	}
	for _, key := range keys {
		if _, found := row[key]; !found {
			return nil, fmt.Errorf("%v has no %s", row, key)
		}
	}
	return row, nil
}

func pairsOf(value any) ([][2]any, error) {
	items, err := list(value)
	if err != nil {
		return nil, err
	}
	out := make([][2]any, len(items))
	for i, item := range items {
		pair, err := list(item)
		if err != nil || len(pair) != 2 {
			return nil, fmt.Errorf("%v is not a pair", item)
		}
		out[i] = [2]any{pair[0], pair[1]}
	}
	return out, nil
}

func allocationsOf(value any) ([]Allocation, error) {
	pairs, err := pairsOf(value)
	if err != nil {
		return nil, err
	}
	out := make([]Allocation, len(pairs))
	for i, pair := range pairs {
		if out[i].ParticipantID, err = text(pair[0]); err != nil {
			return nil, err
		}
		if out[i].AmountVND, err = vnd(pair[1]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func obligationsOf(value any) ([]Obligation, error) {
	items, err := list(value)
	if err != nil {
		return nil, err
	}
	out := make([]Obligation, len(items))
	for i, item := range items {
		row, err := fields(item, "sender_id", "recipient_id", "amount_vnd", "source_expense_version_id")
		if err != nil {
			return nil, err
		}
		o := &out[i]
		if o.SenderID, err = text(row["sender_id"]); err != nil {
			return nil, err
		}
		if o.RecipientID, err = text(row["recipient_id"]); err != nil {
			return nil, err
		}
		if o.AmountVND, err = vnd(row["amount_vnd"]); err != nil {
			return nil, err
		}
		if o.SourceExpenseVersionID, err = text(row["source_expense_version_id"]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// edgesOf reads group_balances' obligations: only the three keys it reads,
// the amount being a derived sum.
func edgesOf(value any) ([]MergedObligation, error) {
	items, err := list(value)
	if err != nil {
		return nil, err
	}
	out := make([]MergedObligation, len(items))
	for i, item := range items {
		row, err := fields(item, "sender_id", "recipient_id", "amount_vnd")
		if err != nil {
			return nil, err
		}
		o := &out[i]
		if o.SenderID, err = text(row["sender_id"]); err != nil {
			return nil, err
		}
		if o.RecipientID, err = text(row["recipient_id"]); err != nil {
			return nil, err
		}
		if o.AmountVND, err = sum(row["amount_vnd"]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func receiptsOf(value any) (map[Pair]*big.Int, error) {
	if value == nil {
		return nil, nil
	}
	items, err := list(value)
	if err != nil {
		return nil, err
	}
	out := make(map[Pair]*big.Int, len(items))
	for _, item := range items {
		triple, err := list(item)
		if err != nil || len(triple) != 3 {
			return nil, fmt.Errorf("receipt %v is not [sender, recipient, amount]", item)
		}
		var pair Pair
		if pair.SenderID, err = text(triple[0]); err != nil {
			return nil, err
		}
		if pair.RecipientID, err = text(triple[1]); err != nil {
			return nil, err
		}
		if _, seen := out[pair]; seen {
			return nil, fmt.Errorf("receipt %v repeats a pair a dict cannot repeat", item)
		}
		if out[pair], err = sum(triple[2]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func renderObligations(obligations []Obligation) []any {
	out := make([]any, len(obligations))
	for i, o := range obligations {
		out[i] = map[string]any{
			"sender_id":                 o.SenderID,
			"recipient_id":              o.RecipientID,
			"amount_vnd":                int64(o.AmountVND),
			"source_expense_version_id": o.SourceExpenseVersionID,
		}
	}
	return out
}

func renderMerged(merged []MergedObligation) []any {
	out := make([]any, len(merged))
	for i, o := range merged {
		sources := make([]any, len(o.SourceExpenseVersionIDs))
		for j, source := range o.SourceExpenseVersionIDs {
			sources[j] = source
		}
		out[i] = map[string]any{
			"sender_id":                  o.SenderID,
			"recipient_id":               o.RecipientID,
			"amount_vnd":                 exact(o.AmountVND),
			"source_expense_version_ids": sources,
		}
	}
	return out
}

func renderBalances(balances []Balance) []any {
	out := make([]any, len(balances))
	for i, b := range balances {
		out[i] = []any{b.PersonID, exact(b.NetVND)}
	}
	return out
}

func renderPlan(plan Plan) map[string]any {
	transfers := make([]any, len(plan.Transfers))
	for i, transfer := range plan.Transfers {
		transfers[i] = map[string]any{
			"kind":         transfer.Kind,
			"sender_id":    transfer.SenderID,
			"recipient_id": transfer.RecipientID,
			"amount_vnd":   exact(transfer.AmountVND),
		}
	}
	return map[string]any{
		"transfers":      transfers,
		"transfer_count": int64(plan.TransferCount),
		"proven_minimal": plan.ProvenMinimal,
		"person_count":   int64(plan.PersonCount),
	}
}

var balanceCalls = []any{"is_member", "load_batch_inputs", "load_confirmed_receipts"}

func renderSheet(sheet Sheet) map[string]any {
	balances := make([]any, len(sheet.Balances))
	for i, b := range sheet.Balances {
		balances[i] = map[string]any{"person_id": b.PersonID, "net_vnd": exact(b.NetVND)}
	}
	transfers := make([]any, len(sheet.Transfers))
	for i, transfer := range sheet.Transfers {
		transfers[i] = map[string]any{
			"sender_id":    transfer.SenderID,
			"recipient_id": transfer.RecipientID,
			"amount_vnd":   exact(transfer.AmountVND),
		}
	}
	return map[string]any{
		"calls":   balanceCalls,
		"problem": nil,
		"response": map[string]any{
			"balances":       balances,
			"transfers":      transfers,
			"proven_minimal": sheet.ProvenMinimal,
			"transfer_count": int64(sheet.TransferCount),
		},
	}
}

// replay runs one case in Go. A *LedgerError comes back as err; the service
// turns it into its 409, which is the ok value of a context_balances case.
func replay(c oracletest.Case) (any, error) {
	decode := func(err error) error { return fmt.Errorf("decode: %w", err) }
	args, err := c.PlainArgs()
	if err != nil {
		return nil, decode(err)
	}
	switch c.Fn {
	case "require_vnd":
		value, err := vnd(args["value"])
		if err != nil {
			return nil, decode(err)
		}
		got, err := RequireVND(value, args["positive"].(bool))
		return int64(got), err
	case "obligations_from_allocations":
		allocations, err := allocationsOf(args["allocations"])
		if err != nil {
			return nil, decode(err)
		}
		advancer, err := oracletest.OptionalString(args["advancer_id"])
		if err != nil {
			return nil, decode(err)
		}
		version, err := text(args["expense_version_id"])
		if err != nil {
			return nil, decode(err)
		}
		got, err := ObligationsFromAllocations(allocations, advancer, version)
		return renderObligations(got), err
	case "merge_obligations":
		obligations, err := obligationsOf(args["obligations"])
		if err != nil {
			return nil, decode(err)
		}
		got, err := MergeObligations(obligations)
		return renderMerged(got), err
	case "group_balances":
		edges, err := edgesOf(args["obligations"])
		if err != nil {
			return nil, decode(err)
		}
		receipts, err := receiptsOf(args["receipts"])
		if err != nil {
			return nil, decode(err)
		}
		got, err := GroupBalances(edges, receipts)
		return renderBalances(got), err
	case "settlement_plan":
		pairs, err := pairsOf(args["balances"])
		if err != nil {
			return nil, decode(err)
		}
		balances := make(map[string]*big.Int, len(pairs))
		for _, pair := range pairs {
			person, err := text(pair[0])
			if err != nil {
				return nil, decode(err)
			}
			if balances[person], err = sum(pair[1]); err != nil {
				return nil, decode(err)
			}
		}
		limit, ok := args["exact_limit"].(int64)
		if !ok || len(balances) != len(pairs) {
			return nil, decode(fmt.Errorf("balances %v, exact_limit %v", args["balances"], args["exact_limit"]))
		}
		got, err := SettlementPlan(balances, int(limit))
		return renderPlan(got), err
	case "context_balances":
		expenses, err := list(args["expenses"])
		if err != nil {
			return nil, decode(err)
		}
		confirmed := make([]ConfirmedExpense, len(expenses))
		for i, item := range expenses {
			row, err := fields(item, "version_id", "paid_by_id", "allocations")
			if err != nil {
				return nil, decode(err)
			}
			if confirmed[i].VersionID, err = text(row["version_id"]); err != nil {
				return nil, decode(err)
			}
			if confirmed[i].PaidByID, err = text(row["paid_by_id"]); err != nil {
				return nil, decode(err)
			}
			if confirmed[i].Allocations, err = allocationsOf(row["allocations"]); err != nil {
				return nil, decode(err)
			}
		}
		receipts, err := receiptsOf(args["receipts"])
		if err != nil {
			return nil, decode(err)
		}
		sheet, err := ContextBalances(confirmed, receipts)
		var refused *LedgerError
		if errors.As(err, &refused) {
			return map[string]any{
				"calls":    balanceCalls,
				"problem":  map[string]any{"status": int64(ConflictStatus), "code": refused.Code, "detail": ConflictDetail},
				"response": nil,
			}, nil
		}
		if err != nil {
			return nil, err
		}
		return renderSheet(sheet), nil
	}
	return nil, decode(fmt.Errorf("unknown function %q", c.Fn))
}

type tally struct {
	cases, refusals, bigResults, bigArgs int
}

// agree replays every case of the files whose mode is prefix or one of its
// fuzz shards, and returns per-function tallies. A case whose Python answer or
// arguments hold an int outside int64 is compared exactly like any other and
// counted, so the log shows those comparisons happened.
func agree(t *testing.T, prefix string) map[string]*tally {
	t.Helper()
	files := oracletest.Load(t, "testdata/python_*.json")
	tallies := map[string]*tally{}
	mismatches, total := 0, 0
	for _, file := range files {
		if file.Mode != prefix && !strings.HasPrefix(file.Mode, prefix+"-fuzz-") {
			continue
		}
		for _, c := range file.Cases {
			want, raised, err := c.Outcome()
			if err != nil {
				t.Fatalf("%s %s: %v", file.Mode, c.Name, err)
			}
			got, goErr := replay(c)
			if goErr != nil && strings.HasPrefix(goErr.Error(), "decode:") {
				t.Fatalf("%s %s: %v", file.Mode, c.Name, goErr)
			}
			count := tallies[c.Fn]
			if count == nil {
				count = &tally{}
				tallies[c.Fn] = count
			}
			count.cases++
			total++
			if args, err := c.PlainArgs(); err == nil && oracletest.HasBigInt(args) {
				count.bigArgs++
			}
			var refused *LedgerError
			ok := false
			if raised != nil {
				count.refusals++
				ok = errors.As(goErr, &refused) && raised.Type == "LedgerError" &&
					raised.Message == refused.Code && raised.Code == refused.Code
			} else {
				ok = goErr == nil && reflect.DeepEqual(want, got)
				if oracletest.HasBigInt(want) {
					count.bigResults++
				}
				// The service answers a LedgerError as a 409 value, not a raise.
				if row, isRow := want.(map[string]any); isRow && row["problem"] != nil {
					count.refusals++
				}
			}
			if !ok {
				mismatches++
				if mismatches <= 20 {
					t.Errorf("%s %s %s(%v):\n  Python %#v raised %+v\n  Go     %#v err %v",
						file.Mode, c.Name, c.Fn, c.Args, want, raised, got, goErr)
				}
			}
		}
	}
	oracletest.CheckShards(t, files, prefix)
	if mismatches > 0 {
		t.Fatalf("%d mismatches over %d Python cases", mismatches, total)
	}
	names := make([]string, 0, len(tallies))
	for name := range tallies {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		count := tallies[name]
		t.Logf("%s: %d Python cases, 0 mismatches (%d refusals; %d answers and %d argument sets past int64, compared exactly)",
			name, count.cases, count.refusals, count.bigResults, count.bigArgs)
	}
	return tallies
}

func TestLedgerMatchesPython(t *testing.T) {
	tallies := agree(t, "ledger")
	want := map[string]int{
		"require_vnd": 28, "obligations_from_allocations": 700, "merge_obligations": 700,
		"group_balances": 700, "settlement_plan": 900,
	}
	for name, least := range want {
		count := tallies[name]
		if count == nil || count.cases < least || count.refusals == 0 {
			t.Errorf("%s: %+v, want at least %d cases and some refusals: the corpus lost its spread", name, count, least)
		}
	}
	for _, name := range []string{"merge_obligations", "group_balances", "settlement_plan"} {
		if tallies[name] == nil || tallies[name].bigResults == 0 {
			t.Errorf("%s: no answer past int64; exactness is untested", name)
		}
	}
	for _, name := range []string{"group_balances", "settlement_plan"} {
		if tallies[name] == nil || tallies[name].bigArgs == 0 {
			t.Errorf("%s: no derived sum past int64 among the arguments", name)
		}
	}
}

func TestContextBalancesMatchesPython(t *testing.T) {
	tallies := agree(t, "context_balances")
	count := tallies["context_balances"]
	if count == nil || count.cases < 600 || count.bigResults == 0 || count.bigArgs == 0 || count.refusals == 0 {
		t.Errorf("context_balances: %+v: the corpus lost its spread", count)
	}
}

func TestConstantsMatchPython(t *testing.T) {
	constants := oracletest.Constants(t, oracletest.Load(t, "testdata/python_*.json"), "ledger")
	if constants["default_exact_limit"] != int64(DefaultExactLimit) {
		t.Errorf("exact_limit default: Python %v, Go %d", constants["default_exact_limit"], DefaultExactLimit)
	}
	ported := map[string]string{
		"LedgerError":                  "LedgerError",
		"require_vnd":                  "RequireVND",
		"obligations_from_allocations": "ObligationsFromAllocations",
		"merge_obligations":            "MergeObligations",
		"group_balances":               "GroupBalances",
		"settlement_plan":              "SettlementPlan",
		"NEGATIVE":                     "money.Negative",
		"NON_POSITIVE":                 "money.NonPositive",
		"NOT_INTEGER":                  "money.NotInteger",
		"confirmed_total":              "ConfirmedTotal (W4, status.go)",
		"obligation_status":            "ObligationStatus (W4, status.go)",
		"settlement_suggestions":       "SettlementSuggestions (W4, status.go)",
	}
	skipped := map[string]string{}
	names, err := oracletest.Strings(constants["names"])
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if ported[name] == "" && skipped[name] == "" {
			t.Errorf("Python exports %s and the port neither maps nor skips it", name)
		}
	}
	if len(names) != len(ported)+len(skipped) {
		t.Errorf("Python has %d public names %v; the port accounts for %d", len(names), names, len(ported)+len(skipped))
	}
}

// The hand-computed settlement corpus is read where it lives (ADR-0029 §2.5),
// never copied: the netted balances, the minimum and greedy transfer counts,
// the number of people out and the size of the maximum zero-sum partition.
func TestSettlementCorpusInPlace(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "..", "..", "api", "tests", "domain", "golden_settlement", "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no settlement corpus found: %v", err)
	}
	type movement struct {
		SenderID    string `json:"sender_id"`
		RecipientID string `json:"recipient_id"`
		AmountVND   int64  `json:"amount_vnd"`
	}
	type vector struct {
		ID    string `json:"id"`
		Input struct {
			Obligations []movement `json:"obligations"`
			Receipts    []movement `json:"receipts"`
		} `json:"input"`
		Expect struct {
			Balances       map[string]int64 `json:"balances"`
			PeopleOut      int              `json:"nguoi_lech_khac_khong"`
			ZeroSumGroups  [][]string       `json:"nhom_zero_sum"`
			MaxGroups      int              `json:"so_nhom_toi_da"`
			MinTransfers   int              `json:"chuyen_toi_thieu"`
			GreedyTransfer int              `json:"chuyen_cua_greedy"`
		} `json:"expect"`
	}
	vectors := 0
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var corpus []vector
		if err := json.Unmarshal(raw, &corpus); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, v := range corpus {
			vectors++
			obligations := make([]MergedObligation, len(v.Input.Obligations))
			for i, o := range v.Input.Obligations {
				obligations[i] = MergedObligation{SenderID: o.SenderID, RecipientID: o.RecipientID, AmountVND: big.NewInt(o.AmountVND)}
			}
			receipts := map[Pair]*big.Int{}
			for _, r := range v.Input.Receipts {
				receipts[Pair{SenderID: r.SenderID, RecipientID: r.RecipientID}] = big.NewInt(r.AmountVND)
			}
			balances, err := GroupBalances(obligations, receipts)
			if err != nil {
				t.Fatalf("%s: %v", v.ID, err)
			}
			got := map[string]int64{}
			for _, b := range balances {
				got[b.PersonID] = b.NetVND.Int64()
			}
			if len(got) != len(v.Expect.Balances) || (len(got) > 0 && !reflect.DeepEqual(got, v.Expect.Balances)) {
				t.Errorf("%s balances: corpus %v, Go %v", v.ID, v.Expect.Balances, got)
			}
			expected := map[string]*big.Int{}
			var nonZero []entry
			for person, amount := range v.Expect.Balances {
				expected[person] = big.NewInt(amount)
				nonZero = append(nonZero, entry{person: person, amount: big.NewInt(amount)})
			}
			plan, err := SettlementPlan(expected, DefaultExactLimit)
			if err != nil {
				t.Fatalf("%s: %v", v.ID, err)
			}
			if plan.TransferCount != v.Expect.MinTransfers || !plan.ProvenMinimal || plan.PersonCount != v.Expect.PeopleOut {
				t.Errorf("%s plan: %d transfers proven=%v over %d people; corpus minimum %d over %d",
					v.ID, plan.TransferCount, plan.ProvenMinimal, plan.PersonCount, v.Expect.MinTransfers, v.Expect.PeopleOut)
			}
			cleared := map[string]int64{}
			for _, transfer := range plan.Transfers {
				if transfer.AmountVND.Sign() <= 0 || transfer.SenderID == transfer.RecipientID {
					t.Errorf("%s: transfer %+v", v.ID, transfer)
				}
				cleared[transfer.SenderID] -= transfer.AmountVND.Int64()
				cleared[transfer.RecipientID] += transfer.AmountVND.Int64()
			}
			for person, amount := range cleared {
				if amount == 0 {
					delete(cleared, person)
				}
			}
			if len(cleared) != len(v.Expect.Balances) || (len(cleared) > 0 && !reflect.DeepEqual(cleared, v.Expect.Balances)) {
				t.Errorf("%s: transfers clear %v, balances are %v", v.ID, cleared, v.Expect.Balances)
			}
			greedy, err := SettlementPlan(expected, 0)
			if err != nil || greedy.TransferCount != v.Expect.GreedyTransfer {
				t.Errorf("%s greedy: %d transfers (%v), corpus %d", v.ID, greedy.TransferCount, err, v.Expect.GreedyTransfer)
			}
			groups := [][]entry{}
			if len(nonZero) > 0 {
				groups = maximumZeroSumPartition(nonZero)
			}
			if len(groups) != v.Expect.MaxGroups || len(v.Expect.ZeroSumGroups) != v.Expect.MaxGroups {
				t.Errorf("%s: %d groups, corpus %d", v.ID, len(groups), v.Expect.MaxGroups)
			}
			for _, group := range groups {
				total := new(big.Int)
				for _, member := range group {
					total.Add(total, member.amount)
				}
				if total.Sign() != 0 {
					t.Errorf("%s: group %v does not sum to zero", v.ID, group)
				}
			}
		}
	}
	if vectors != 10 {
		t.Errorf("%d settlement vectors, want the corpus's 10", vectors)
	}
	t.Logf("%d hand-computed settlement vectors agree", vectors)
}

// A *big.Int is a pointer: an answer that shared memory with an argument, or
// with another answer, would let a caller's later arithmetic rewrite a
// balance it already handed out.
func TestNoAnswerSharesMemoryWithAnArgument(t *testing.T) {
	two63 := new(big.Int).Lsh(big.NewInt(1), 63)
	receipt := new(big.Int).Set(two63)
	obligations := []MergedObligation{
		{SenderID: "a", RecipientID: "b", AmountVND: new(big.Int).Mul(two63, big.NewInt(3))},
		{SenderID: "c", RecipientID: "b", AmountVND: big.NewInt(5)},
	}
	balances, err := GroupBalances(obligations, map[Pair]*big.Int{{SenderID: "a", RecipientID: "b"}: receipt})
	if err != nil {
		t.Fatal(err)
	}
	if obligations[0].AmountVND.Cmp(new(big.Int).Mul(two63, big.NewInt(3))) != 0 || receipt.Cmp(two63) != 0 {
		t.Fatalf("GroupBalances changed its arguments: %v, %v", obligations[0].AmountVND, receipt)
	}
	input := map[string]*big.Int{}
	for _, b := range balances {
		input[b.PersonID] = b.NetVND
	}
	before := map[string]string{}
	for person, amount := range input {
		before[person] = amount.String()
	}
	plan, err := SettlementPlan(input, DefaultExactLimit)
	if err != nil {
		t.Fatal(err)
	}
	for person, amount := range input {
		if amount.String() != before[person] {
			t.Fatalf("SettlementPlan changed balance %s from %s to %s", person, before[person], amount)
		}
	}
	for _, transfer := range plan.Transfers {
		for _, amount := range input {
			if transfer.AmountVND == amount {
				t.Fatalf("transfer %+v shares its amount with a balance", transfer)
			}
		}
	}
	transferred := new(big.Int)
	for _, transfer := range plan.Transfers {
		transferred.Add(transferred, transfer.AmountVND)
	}
	if want := new(big.Int).Add(new(big.Int).Mul(two63, big.NewInt(2)), big.NewInt(5)); transferred.Cmp(want) != 0 {
		t.Fatalf("transfers move %s, balances owe %s", transferred, want)
	}
}
