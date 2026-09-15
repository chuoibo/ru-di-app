package ledger

import (
	"errors"
	"math/big"
	"testing"

	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/oracletest"
)

// testdata/python_ledger_status*.json is rendered by
// scripts/render_domain_w4_goldens.py from the real app.domain.ledger in the
// parity API image: the status functions the W4 batch and receipt routes
// reach, and settlement_suggestions.

func statusReceipts(value any) ([]money.VND, error) {
	rows, err := oracletest.List(value)
	if err != nil {
		return nil, err
	}
	out := make([]money.VND, len(rows))
	for i, raw := range rows {
		r, err := oracletest.Row(raw, "amount_vnd")
		if err != nil {
			return nil, err
		}
		n, err := oracletest.Int64(r["amount_vnd"])
		if err != nil {
			return nil, err
		}
		out[i] = money.VND(n)
	}
	return out, nil
}

func statusReplay(c oracletest.Case, args map[string]any) (any, error) {
	switch c.Fn {
	case "confirmed_total":
		receipts, err := statusReceipts(args["receipt_confirmations"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		total, err := ConfirmedTotal(receipts)
		if err != nil {
			return nil, err
		}
		return oracletest.Exact(total), nil
	case "obligation_status":
		declared, err := oracletest.Int64(args["declared_amount_vnd"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		receipts, err := statusReceipts(args["receipt_confirmations"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		return ObligationStatus(money.VND(declared), receipts)
	case "settlement_suggestions":
		entries, err := oracletest.List(args["balances"])
		if err != nil {
			return nil, oracletest.Decode(err)
		}
		balances := make(map[string]*big.Int, len(entries))
		for _, entry := range entries {
			pair, err := oracletest.List(entry)
			if err != nil || len(pair) != 2 {
				return nil, oracletest.Decode(errors.New("a balance is not [person, amount]"))
			}
			person, err := oracletest.Str(pair[0])
			if err != nil {
				return nil, oracletest.Decode(err)
			}
			if balances[person], err = oracletest.Integer(pair[1]); err != nil {
				return nil, oracletest.Decode(err)
			}
		}
		transfers, err := SettlementSuggestions(balances)
		if err != nil {
			return nil, err
		}
		out := make([]any, len(transfers))
		for i, transfer := range transfers {
			out[i] = map[string]any{
				"kind":         transfer.Kind,
				"sender_id":    transfer.SenderID,
				"recipient_id": transfer.RecipientID,
				"amount_vnd":   oracletest.Exact(transfer.AmountVND),
			}
		}
		return out, nil
	}
	return nil, oracletest.Decode(errors.New("unknown function " + c.Fn))
}

func statusRefusal(err error) (string, string, bool) {
	var refused *LedgerError
	if errors.As(err, &refused) {
		return "LedgerError", refused.Code, true
	}
	return "", "", false
}

// statusCodes is every refusal the edge cases reach; a nil balance
// (AMOUNT_NOT_INTEGER) appears only among them.
var statusCodes = []string{CodeNegativeAmount, CodeNonPositiveConfirmation, CodeNonPositiveObligation, CodeBalancesDoNotNetToZero, CodeAmountNotInteger}

func TestObligationStatusMatchesPython(t *testing.T) {
	checkStatus(t, oracletest.Load(t, "testdata/python_*.json"), "ledger_status", 300, statusCodes)
}

// checkStatus replays the cases of module in files: at least least in all,
// refusals from each function, a confirmed total past int64, every code.
func checkStatus(t *testing.T, files []oracletest.File, module string, least int, codes []string) {
	t.Helper()
	report := oracletest.Agree(t, files, module, statusReplay, statusRefusal)
	total := 0
	for _, fn := range []string{"confirmed_total", "obligation_status", "settlement_suggestions"} {
		tally := report.ByFn[fn]
		if tally == nil || tally.Refusals == 0 {
			t.Errorf("%s: %+v, want some cases and some refusals", fn, tally)
			continue
		}
		total += tally.Cases
	}
	if total < least {
		t.Errorf("%d cases, want at least %d", total, least)
	}
	if tally := report.ByFn["confirmed_total"]; tally == nil || tally.BigResults == 0 {
		t.Errorf("confirmed_total: no sum past int64")
	}
	for _, code := range codes {
		if report.Codes[code] == 0 {
			t.Errorf("no Python case refused with %s", code)
		}
	}
	t.Logf("refusals %v", report.Codes)
}
