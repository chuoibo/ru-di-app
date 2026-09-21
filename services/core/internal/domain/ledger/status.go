package ledger

import (
	"math/big"

	"mobile/services/core/internal/domain/money"
)

// Codes carried by *LedgerError from the obligation status functions.
const (
	CodeNonPositiveConfirmation = "NON_POSITIVE_CONFIRMATION"
	CodeNonPositiveObligation   = "NON_POSITIVE_OBLIGATION"
)

// The statuses obligation_status derives. A dispute is not one of them.
const (
	StatusOutstanding        = "outstanding"
	StatusPartiallyConfirmed = "partially_confirmed"
	StatusConfirmed          = "confirmed"
	StatusOverConfirmed      = "over_confirmed"
)

// ConfirmedTotal is confirmed_total: the exact sum of the amounts a recipient
// confirmed receiving. Each amount is a stored receipt row, so money.VND; the
// sum is derived and a *big.Int, since two receipts at the int64 bound already
// leave int64. A negative amount is NEGATIVE_AMOUNT and zero is
// NON_POSITIVE_CONFIRMATION, found in the order the receipts are given.
func ConfirmedTotal(receipts []money.VND) (*big.Int, error) {
	total := new(big.Int)
	for _, amount := range receipts {
		if _, err := RequireVND(amount, false); err != nil {
			return nil, err
		}
		if amount <= 0 {
			return nil, refuse(CodeNonPositiveConfirmation)
		}
		total.Add(total, big.NewInt(int64(amount)))
	}
	return total, nil
}

// ObligationStatus is obligation_status: derived from the confirmed receipts,
// never stored. The declared amount is checked before any receipt:
// NEGATIVE_AMOUNT, then NON_POSITIVE_OBLIGATION for zero. A payment report is
// deliberately not an input.
func ObligationStatus(declared money.VND, receipts []money.VND) (string, error) {
	if _, err := RequireVND(declared, false); err != nil {
		return "", err
	}
	if declared <= 0 {
		return "", refuse(CodeNonPositiveObligation)
	}
	confirmed, err := ConfirmedTotal(receipts)
	if err != nil {
		return "", err
	}
	if confirmed.Sign() == 0 {
		return StatusOutstanding, nil
	}
	switch confirmed.Cmp(big.NewInt(int64(declared))) {
	case -1:
		return StatusPartiallyConfirmed, nil
	case 0:
		return StatusConfirmed, nil
	}
	return StatusOverConfirmed, nil
}

// SettlementSuggestions is settlement_suggestions: the transfers of
// SettlementPlan with DefaultExactLimit. Suggestions only; nothing applies
// them.
func SettlementSuggestions(balances map[string]*big.Int) ([]Transfer, error) {
	plan, err := SettlementPlan(balances, DefaultExactLimit)
	if err != nil {
		return nil, err
	}
	return plan.Transfers, nil
}
