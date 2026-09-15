package routes

import (
	"context"
	"errors"
	"strings"
	"time"

	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/domain/moneysteps"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
)

// confirmReceipt is POST /obligations/{obligation_id}/confirm-receipt
// (routes/obligations.py confirm_receipt, ApiService.confirm_receipt): the
// obligation locked FOR UPDATE (404 before any permission), then only its
// recipient may confirm (no membership is read, so a person who left the
// group still can), then the receipt, whose body idempotency_key is unique
// across the whole table. The status is derived from every receipt of the
// obligation after the write, or after a stored key is answered again.
//
// amount_vnd has no ceiling: an amount past int64 is compared with a stored
// receipt (409) and otherwise reaches the INSERT, which PostgreSQL refuses, so
// the request ends as Python's does, a plain-text 500 with nothing written.
func confirmReceipt() Route {
	return Route{ID: "POST /obligations/{obligation_id}/confirm-receipt", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		obligationID, err := pathUUID(call, "obligation_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		amount, err := intField(body, "amount_vnd")
		if err != nil {
			return endpoint.Reply{}, err
		}
		key, err := uuidField(body, "idempotency_key")
		if err != nil {
			return endpoint.Reply{}, err
		}
		reportID, err := optionalUUIDField(body, "payment_report_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		target, err := store.GetReceiptTarget(ctx, obligationID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if target == nil {
			return endpoint.Reply{}, endpoint.Refuse(404, "obligation_not_found", "Obligation does not exist")
		}
		if err := requireFacts(call, "confirm_receipt", map[string]bool{
			"is_recipient_of_this_obligation": call.Actor.ID == target.RecipientID,
		}); err != nil {
			return endpoint.Reply{}, err
		}
		record, err := store.SaveReceiptConfirmation(ctx, repo.ReceiptConfirmationInput{
			Target:          *target,
			ConfirmedByID:   call.Actor.ID,
			AmountVND:       amount.Big(),
			PaymentReportID: reportID,
			IdempotencyKey:  key,
			Now:             time.Now().UTC(),
		})
		var conflict *repo.Conflict
		if errors.As(err, &conflict) {
			return endpoint.Reply{}, endpoint.Refuse(409, strings.ToLower(conflict.Code), "Receipt confirmation conflicted")
		}
		if err != nil {
			return endpoint.Reply{}, err
		}
		receipts := make([]money.VND, len(record.ReceiptAmountsVND))
		for i, amount := range record.ReceiptAmountsVND {
			receipts[i] = money.VND(amount)
		}
		status, refused, err := moneysteps.ReceiptStatus(money.VND(target.AmountVND), receipts)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if refused != nil {
			return endpoint.Reply{}, refuseMoney(refused)
		}
		out := pyjson.NewOrderedMap()
		out.Set("receipt_confirmation_id", pyjson.String(record.ID))
		out.Set("obligation_id", pyjson.String(record.ObligationID))
		out.Set("amount_vnd", pyjson.NewInt(record.AmountVND))
		out.Set("obligation_status", pyjson.String(status))
		return endpoint.Reply{Body: out}, nil
	}}
}
