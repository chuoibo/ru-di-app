package routes

import (
	"context"

	"mobile/services/core/internal/domain/moneysteps"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
)

// financeMovementLimit is ApiService.FINANCE_MOVEMENT_LIMIT.
const financeMovementLimit = 20

// readPersonFinance is GET /people/{person_id}/finance
// (routes/finance.py read_person_finance, ApiService.person_finance_summary):
// only the person it describes may read it, decided before any read and
// without the permission table, so no role is checked. A person with no
// ledger rows, or no people row, reads zeros. The route module builds the
// response field by field; every figure is recomputed from the ledger, and the
// four money figures stay exact past int64.
func readPersonFinance() Route {
	return Route{ID: "GET /people/{person_id}/finance", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		personID, err := pathUUID(call, "person_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		if refused := moneysteps.FinanceReadable(call.Actor.ID, personID); refused != nil {
			return endpoint.Reply{}, refuseMoney(refused)
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		summary, err := store.PersonFinanceSummary(ctx, personID, financeMovementLimit)
		if err != nil {
			return endpoint.Reply{}, err
		}
		movements := make(pyjson.List, len(summary.Movements))
		for i, m := range summary.Movements {
			wire := pyjson.NewOrderedMap()
			wire.Set("obligation_id", pyjson.String(m.ObligationID))
			wire.Set("direction", pyjson.String(m.Direction))
			wire.Set("amount_vnd", pyjson.NewInt(m.AmountVND))
			wire.Set("counterparty_id", pyjson.String(m.CounterpartyID))
			wire.Set("counterparty_name", textOrNull(m.CounterpartyName))
			wire.Set("context_id", pyjson.String(m.ContextID))
			wire.Set("context_name", textOrNull(m.ContextName))
			wire.Set("occasion", textOrNull(m.Occasion))
			wire.Set("occurred_at", pyjson.String(pyjson.DateTime(m.OccurredAt.UTC())))
			movements[i] = wire
		}
		out := pyjson.NewOrderedMap()
		out.Set("person_id", pyjson.String(summary.PersonID))
		out.Set("display_name", textOrNull(summary.DisplayName))
		out.Set("spend_vnd", pyjson.NewBigInt(summary.SpendVND))
		out.Set("settled_vnd", pyjson.NewBigInt(summary.SettledVND))
		out.Set("outstanding_vnd", pyjson.NewBigInt(summary.OutstandingVND))
		out.Set("receivable_vnd", pyjson.NewBigInt(summary.ReceivableVND))
		out.Set("expense_count", pyjson.NewInt(summary.ExpenseCount))
		out.Set("group_count", pyjson.NewInt(summary.GroupCount))
		out.Set("movements", movements)
		return endpoint.Reply{Body: out}, nil
	}}
}
