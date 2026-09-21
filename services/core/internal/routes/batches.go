package routes

// Collection rounds (routes/batches.py over ApiService): freeze a batch,
// publish it with one guest link per sender, read its board, list a group's
// rounds. The rules between the reads and the writes are moneysteps'; the
// obligation status on every board is derived in the repository from the
// receipt rows (ledger.ObligationStatus), never read from a column.

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/domain/money"
	"mobile/services/core/internal/domain/moneysteps"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/pyval"
	"mobile/services/core/internal/repo"
)

// createBatch is POST /batches (create_batch): membership of the named group,
// the owner role the service derives for the creator (never refused), then
// moneysteps.FreezeBatch over load_batch_inputs, read twice when no list is
// named, and the frozen write. Each obligation carries the request's due_at,
// so the response writes it with the offset the caller sent.
func createBatch() Route {
	return Route{ID: "POST /batches", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		contextID, err := uuidField(body, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		versionIDs, named, err := batchVersionIDsField(body, "expense_version_ids")
		if err != nil {
			return endpoint.Reply{}, err
		}
		dueAt, err := batchInstantField(body, "due_at")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "create_batch", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireFactsWithRoles(call, "freeze_batch", map[string]bool{"owns_batch": true}, []string{"batch_owner"}); err != nil {
			return endpoint.Reply{}, err
		}
		now := time.Now().UTC()
		due := dueAt.Time()
		drafts, refused, err := moneysteps.FreezeBatch(due, now, versionIDs, named, func(all bool, ids []string) (moneysteps.BatchInputs, error) {
			// None reads every latest confirmed version; a tuple, empty
			// included, reads exactly those.
			if all {
				ids = nil
			} else if ids == nil {
				ids = []string{}
			}
			inputs, err := store.LoadBatchInputs(ctx, contextID, ids)
			if err != nil {
				return moneysteps.BatchInputs{}, err
			}
			return stepBatchInputs(inputs), nil
		})
		if err != nil {
			return endpoint.Reply{}, err
		}
		if refused != nil {
			return endpoint.Reply{}, endpoint.Refuse(refused.Status, refused.Code, refused.Detail)
		}
		obligations := make([]repo.ObligationDraft, len(drafts))
		for i, draft := range drafts {
			// Python sends such a sum to the BIGINT column and PostgreSQL
			// refuses it inside the same transaction: a 500 with nothing
			// written. Allocations are capped far below this.
			if !draft.AmountVND.IsInt64() {
				return endpoint.Reply{}, fmt.Errorf("routes: an obligation of %s đồng does not fit BIGINT", draft.AmountVND)
			}
			sources := make([]repo.AllocationRow, len(draft.Sources))
			for j, source := range draft.Sources {
				sources[j] = repo.AllocationRow{ID: source.ID, ParticipantID: source.ParticipantID, AmountVND: int64(source.AmountVND)}
			}
			obligations[i] = repo.ObligationDraft{SenderID: draft.SenderID, RecipientID: draft.RecipientID,
				AmountVND: draft.AmountVND.Int64(), SourceExpenseVersionIDs: draft.SourceExpenseVersionIDs, Sources: sources}
		}
		stored, err := store.SaveFrozenBatch(ctx, repo.FrozenBatchInput{ContextID: contextID, OwnerID: call.Actor.ID,
			DueAt: due, Obligations: obligations, Now: now})
		if err != nil {
			return endpoint.Reply{}, err
		}
		dueText := pyjson.String(pyjson.DateTime(due))
		list := make(pyjson.List, len(stored.Obligations))
		for i, obligation := range stored.Obligations {
			versions := make(pyjson.List, len(obligation.SourceExpenseVersionIDs))
			for j, id := range obligation.SourceExpenseVersionIDs {
				versions[j] = pyjson.String(id)
			}
			entry := pyjson.NewOrderedMap()
			entry.Set("obligation_id", pyjson.String(obligation.ID))
			entry.Set("sender_id", pyjson.String(obligation.SenderID))
			entry.Set("recipient_id", pyjson.String(obligation.RecipientID))
			entry.Set("amount_vnd", pyjson.NewInt(obligation.AmountVND))
			entry.Set("due_at", dueText)
			entry.Set("source_expense_version_ids", versions)
			list[i] = entry
		}
		out := pyjson.NewOrderedMap()
		out.Set("batch_id", pyjson.String(stored.ID))
		out.Set("batch_version_id", pyjson.String(stored.VersionID))
		out.Set("status", pyjson.String("frozen"))
		out.Set("obligations", list)
		return endpoint.Reply{Body: out}, nil
	}}
}

// publishBatch is POST /batches/{batch_id}/publish (publish_batch): the batch
// locked FOR UPDATE (404 before any permission), publish_batch with the
// batch_owner role derived for the owner alone, then moneysteps.PublishBatch
// (expiry 422 before the gates 409, then the transition), one token per
// sender in uuid order, and the write. Only the digest of a token is stored;
// the token itself appears once, in the response path.
func publishBatch() Route {
	return Route{ID: "POST /batches/{batch_id}/publish", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		batchID, err := pathUUID(call, "batch_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		deliveryMethod, err := stringField(body, "delivery_method")
		if err != nil {
			return endpoint.Reply{}, err
		}
		expiresAt, err := batchInstantField(body, "guest_link_expires_at")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		// Conflict BATCH_HAS_NO_VERSION is not caught in Python: a 500.
		batch, err := store.LoadBatchForPublish(ctx, batchID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if batch == nil {
			return endpoint.Reply{}, endpoint.Refuse(404, "batch_not_found", "Batch does not exist")
		}
		owns := call.Actor.ID == batch.OwnerID
		var derived []string
		if owns {
			derived = []string{"batch_owner"}
		}
		if err := requireFactsWithRoles(call, "publish_batch", map[string]bool{"owns_batch": owns}, derived); err != nil {
			return endpoint.Reply{}, err
		}
		now := time.Now().UTC()
		expires := expiresAt.Time()
		obligations := make([]moneysteps.PublishObligation, len(batch.Obligations))
		for i, o := range batch.Obligations {
			obligations[i] = moneysteps.PublishObligation{ID: o.ID, BatchVersionID: o.BatchVersionID, SenderID: o.SenderID,
				RecipientID: o.RecipientID, AmountVND: money.VND(o.AmountVND)}
		}
		plan, refused, err := moneysteps.PublishBatch(moneysteps.Batch{VersionID: batch.VersionID, Status: batch.Status,
			AdvancerAcknowledged: batch.AdvancerAcknowledged, Obligations: obligations}, deliveryMethod, expires, now)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if refused != nil {
			return endpoint.Reply{}, endpoint.Refuse(refused.Status, refused.Code, refused.Detail)
		}
		tokens := make([]string, len(plan.Links))
		drafts := make([]repo.GuestLinkDraft, len(plan.Links))
		senders := map[string]bool{}
		for i, link := range plan.Links {
			token, err := mintGuestToken()
			if err != nil {
				return endpoint.Reply{}, err
			}
			tokens[i] = token
			drafts[i] = repo.GuestLinkDraft{SenderID: link.SenderID, TokenDigest: auth.TokenDigest(token), ExpiresAt: expires}
			senders[link.SenderID] = true
		}
		stored, err := store.SavePublishedBatch(ctx, *batch, plan.State, drafts, call.Actor.ID, now)
		var conflict *repo.Conflict
		if errors.As(err, &conflict) {
			return endpoint.Reply{}, endpoint.Refuse(409, strings.ToLower(conflict.Code), "Batch publication conflicted")
		}
		if err != nil {
			return endpoint.Reply{}, err
		}
		written := map[string]bool{}
		for _, link := range stored {
			written[link.SenderID] = true
		}
		if !sameIDSet(written, senders) {
			return endpoint.Reply{}, endpoint.Refuse(500, "guest_link_write_mismatch", "Guest link write was incomplete")
		}
		expiresText := pyjson.String(pyjson.DateTime(expires))
		links := make(pyjson.List, len(plan.Links))
		for i, link := range plan.Links {
			owed := make(pyjson.List, len(link.Obligations))
			for j, o := range link.Obligations {
				entry := pyjson.NewOrderedMap()
				entry.Set("obligation_id", pyjson.String(o.ID))
				entry.Set("amount_vnd", pyjson.NewInt(int64(o.AmountVND)))
				owed[j] = entry
			}
			entry := pyjson.NewOrderedMap()
			entry.Set("sender_id", pyjson.String(link.SenderID))
			entry.Set("path", pyjson.String("/g/"+tokens[i]))
			entry.Set("expires_at", expiresText)
			entry.Set("obligations", owed)
			links[i] = entry
		}
		out := pyjson.NewOrderedMap()
		out.Set("batch_id", pyjson.String(batch.ID))
		out.Set("status", pyjson.String("published"))
		out.Set("guest_links", links)
		return endpoint.Reply{Body: out}, nil
	}}
}

// listBatchObligations is GET /batches/{batch_id}/obligations
// (list_batch_obligations): the board is read first, so an unknown batch is
// 404 before any permission question, then membership of the batch's group.
func listBatchObligations() Route {
	return Route{ID: "GET /batches/{batch_id}/obligations", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		batchID, err := pathUUID(call, "batch_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		board, err := store.ListBatchObligations(ctx, batchID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if board == nil {
			return endpoint.Reply{}, endpoint.Refuse(404, "unknown_batch", "No such batch")
		}
		if err := requireGroupMember(ctx, call, store, "view_collection_board", board.ContextID); err != nil {
			return endpoint.Reply{}, err
		}
		rows := make(pyjson.List, len(board.Obligations))
		var disputed, reported int64
		for i, row := range board.Obligations {
			entry := pyjson.NewOrderedMap()
			entry.Set("obligation_id", pyjson.String(row.ObligationID))
			entry.Set("sender_id", pyjson.String(row.SenderID))
			entry.Set("recipient_id", pyjson.String(row.RecipientID))
			entry.Set("amount_vnd", pyjson.NewInt(row.AmountVND))
			entry.Set("obligation_status", pyjson.String(row.Status))
			entry.Set("disputed", pyjson.Bool(row.Disputed))
			entry.Set("disputed_reason", textOrNull(row.DisputedReason))
			entry.Set("payment_reported_at", instantOrNull(row.PaymentReportedAt))
			rows[i] = entry
			if row.Disputed {
				disputed++
			}
			if row.PaymentReportedAt != nil {
				reported++
			}
		}
		out := pyjson.NewOrderedMap()
		out.Set("batch_id", pyjson.String(batchID))
		out.Set("obligations", rows)
		out.Set("disputed_count", pyjson.NewInt(disputed))
		out.Set("payment_reported_count", pyjson.NewInt(reported))
		return endpoint.Reply{Body: out}, nil
	}}
}

// listContextBatches is GET /contexts/{context_id}/batches
// (list_context_batches): membership before any batch is read, so an unknown
// group and someone else's group refuse alike; each round is folded from its
// own board, newest first.
func listContextBatches() Route {
	return Route{ID: "GET /contexts/{context_id}/batches", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, err := groupStore(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireGroupMember(ctx, call, store, "view_collection_board", contextID); err != nil {
			return endpoint.Reply{}, err
		}
		rounds, err := store.ListContextBatches(ctx, contextID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		list := make(pyjson.List, len(rounds))
		for i, round := range rounds {
			entry := pyjson.NewOrderedMap()
			entry.Set("batch_id", pyjson.String(round.BatchID))
			entry.Set("status", pyjson.String(round.Status))
			entry.Set("created_at", pyjson.String(pyjson.DateTime(round.CreatedAt.UTC())))
			entry.Set("published_at", instantOrNull(round.PublishedAt))
			entry.Set("obligation_count", pyjson.NewInt(round.ObligationCount))
			entry.Set("confirmed_count", pyjson.NewInt(round.ConfirmedCount))
			entry.Set("disputed_count", pyjson.NewInt(round.DisputedCount))
			entry.Set("total_vnd", pyjson.NewBigInt(round.TotalVND))
			list[i] = entry
		}
		out := pyjson.NewOrderedMap()
		out.Set("context_id", pyjson.String(contextID))
		out.Set("batches", list)
		return endpoint.Reply{Body: out}, nil
	}}
}

// stepBatchInputs hands load_batch_inputs' answer to moneysteps.
func stepBatchInputs(inputs repo.BatchInputs) moneysteps.BatchInputs {
	out := moneysteps.BatchInputs{Expenses: make([]moneysteps.BatchExpense, len(inputs.Expenses)),
		UnavailableVersionIDs: append([]string{}, inputs.UnavailableVersionIDs...)}
	for i, expense := range inputs.Expenses {
		rows := make([]moneysteps.AllocationRow, len(expense.Allocations))
		for j, row := range expense.Allocations {
			rows[j] = moneysteps.AllocationRow{ID: row.ID, ParticipantID: row.ParticipantID, AmountVND: money.VND(row.AmountVND)}
		}
		out.Expenses[i] = moneysteps.BatchExpense{VersionID: expense.VersionID, PaidByID: expense.PaidByID, Allocations: rows}
	}
	return out
}

// mintGuestToken is secrets.token_urlsafe(32): 32 random bytes, base64url
// without padding, 43 characters.
func mintGuestToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

// sameIDSet is set equality, as `{...} != set(...)` compares.
func sameIDSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for id := range a {
		if !b[id] {
			return false
		}
	}
	return true
}

// batchInstantField reads an aware `datetime` field (_require_timezone has
// already refused a naive one).
func batchInstantField(model *pyval.Model, name string) (pyval.DateTime, error) {
	value, err := field(model, name)
	if err != nil {
		return pyval.DateTime{}, err
	}
	instant, ok := value.(pyval.DateTime)
	if !ok || !instant.Aware {
		return pyval.DateTime{}, fmt.Errorf("routes: %s.%s is %T, not an aware datetime", model.Class, name, value)
	}
	return instant, nil
}

// batchVersionIDsField reads `list[UUID] | None`: named is false for None. A
// named list is never nil, so an empty list stays an empty tuple.
func batchVersionIDsField(model *pyval.Model, name string) (ids []string, named bool, err error) {
	value, err := field(model, name)
	if err != nil {
		return nil, false, err
	}
	switch v := value.(type) {
	case pyjson.Null:
		return nil, false, nil
	case pyval.List:
		ids = make([]string, len(v))
		for i, item := range v {
			id, ok := item.(pyval.UUID)
			if !ok {
				return nil, false, fmt.Errorf("routes: %s.%s[%d] is %T, not a UUID", model.Class, name, i, item)
			}
			ids[i] = id.String()
		}
		return ids, true, nil
	}
	return nil, false, fmt.Errorf("routes: %s.%s is %T, not a list of UUID or None", model.Class, name, value)
}
