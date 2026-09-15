package routes

import (
	"context"
	"math/big"
	"time"

	"mobile/services/core/internal/domain/budget"
	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/domain/moneysteps"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"
)

// readGroupBudget is GET /contexts/{context_id}/budget
// (routes/budget.py read_group_budget, ApiService.group_budget): members only,
// then today in Vietnam from the server clock, the wall's group_recap read
// (newest version of every expense, folded into Vietnam's calendar day in
// PostgreSQL) and the roster, then the pure budget. The candidate has no
// ceiling and is echoed with every digit. A BudgetError is not caught in
// Python and ends the request as a 500 here too.
func readGroupBudget() Route {
	return Route{ID: "GET /contexts/{context_id}/budget", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		query, err := optionalIntParam(call, "candidate_per_person_vnd")
		if err != nil {
			return endpoint.Reply{}, err
		}
		var candidate *big.Int
		if query != nil {
			candidate = query.Big()
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "view_group_budget", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		records, err := store.GroupRecap(ctx, contextID, repo.WallClockDate(time.Now()))
		if err != nil {
			return endpoint.Reply{}, err
		}
		roster, err := rosterOf(ctx, store, contextID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		outings := make([]budget.Outing, len(records))
		for i, record := range records {
			outings[i] = budget.Outing{
				OutingID:           record.Outing.ID,
				Title:              record.Outing.Title,
				Headcount:          record.Outing.Headcount,
				BudgetPerPersonVND: money.VND(record.Outing.BudgetPerPersonVND),
				SplitTotalVND:      big.NewInt(record.SplitTotalVND),
				InProgress:         record.InProgress,
			}
		}
		result, err := moneysteps.GroupBudget(outings, roster, candidate)
		if err != nil {
			return endpoint.Reply{}, err
		}
		live := make(pyjson.List, len(result.InProgress))
		for i, o := range result.InProgress {
			wire := pyjson.NewOrderedMap()
			wire.Set("outing_id", pyjson.String(o.OutingID))
			wire.Set("title", pyjson.String(o.Title))
			wire.Set("headcount", pyjson.NewInt(o.Headcount))
			wire.Set("budget_per_person_vnd", pyjson.NewInt(int64(o.BudgetPerPersonVND)))
			wire.Set("spent_per_person_vnd", pyjson.NewBigInt(o.SpentPerPersonVND))
			wire.Set("remaining_per_person_vnd", pyjson.NewBigInt(o.RemainingPerPersonVND))
			wire.Set("over_budget", pyjson.Bool(o.OverBudget))
			live[i] = wire
		}
		out := pyjson.NewOrderedMap()
		out.Set("context_id", pyjson.String(contextID))
		out.Set("outing_count", pyjson.NewInt(int64(result.OutingCount)))
		out.Set("active_member_count", pyjson.NewInt(result.ActiveMemberCount))
		if result.AvgPerPersonVND == nil {
			out.Set("avg_per_person_vnd", pyjson.Null{})
		} else {
			out.Set("avg_per_person_vnd", pyjson.NewBigInt(result.AvgPerPersonVND))
		}
		out.Set("in_progress", live)
		if result.Comparison == nil {
			out.Set("comparison", pyjson.Null{})
		} else {
			comparison := pyjson.NewOrderedMap()
			comparison.Set("candidate_per_person_vnd", pyjson.NewBigInt(result.Comparison.CandidatePerPersonVND))
			comparison.Set("delta_vnd", pyjson.NewBigInt(result.Comparison.DeltaVND))
			comparison.Set("verdict", pyjson.String(result.Comparison.Verdict))
			out.Set("comparison", comparison)
		}
		return endpoint.Reply{Body: out}, nil
	}}
}
