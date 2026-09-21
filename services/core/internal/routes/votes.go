package routes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"mobile/services/core/internal/domain/vote"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/pyval"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/service"
)

// createVote is POST /contexts/{context_id}/votes (routes/votes.py
// create_vote, ApiService.create_vote): membership, then the outing when one
// is named, then the insert.
func createVote() Route {
	return Route{ID: "POST /contexts/{context_id}/votes", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		request, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		question, err := stringField(request, "question")
		if err != nil {
			return endpoint.Reply{}, err
		}
		optionModels, err := modelListField(request, "options")
		if err != nil {
			return endpoint.Reply{}, err
		}
		options := make([]repo.VoteOptionInput, 0, len(optionModels))
		for _, option := range optionModels {
			label, err := stringField(option, "label")
			if err != nil {
				return endpoint.Reply{}, err
			}
			placeName, err := optionalStringField(option, "place_name")
			if err != nil {
				return endpoint.Reply{}, err
			}
			options = append(options, repo.VoteOptionInput{Label: label, PlaceName: placeName})
		}
		outingID, err := optionalUUIDField(request, "outing_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		tx, err := call.Unit.Tx(ctx)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store := repo.Repository{Q: tx}
		member, err := store.IsMember(ctx, contextID, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireFacts(call, "create_vote", map[string]bool{"is_group_member": member}); err != nil {
			return endpoint.Reply{}, err
		}
		if outingID != nil {
			outing, err := store.GetOuting(ctx, *outingID)
			if err != nil {
				return endpoint.Reply{}, err
			}
			if outing == nil {
				return endpoint.Reply{}, endpoint.Refuse(404, "outing_not_found", "Outing does not exist")
			}
			if outing.ContextID != contextID {
				return endpoint.Reply{}, endpoint.Refuse(422, "outing_not_in_context", "Outing does not belong to this context")
			}
		}
		record, err := store.CreateVote(ctx, repo.VoteInput{
			ContextID: contextID, OutingID: outingID, CreatedByID: call.Actor.ID,
			Question: question, Options: options, Now: time.Now().UTC(),
		})
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := wireVote(record, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: body}, nil
	}}
}

// listContextVotes is GET /contexts/{context_id}/votes (list_context_votes).
func listContextVotes() Route {
	return Route{ID: "GET /contexts/{context_id}/votes", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		tx, err := call.Unit.Tx(ctx)
		if err != nil {
			return endpoint.Reply{}, err
		}
		store := repo.Repository{Q: tx}
		member, err := store.IsMember(ctx, contextID, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireFacts(call, "view_votes", map[string]bool{"is_group_member": member}); err != nil {
			return endpoint.Reply{}, err
		}
		records, err := store.ListVotes(ctx, contextID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		votes := pyjson.List{}
		for _, record := range records {
			wire, err := wireVote(record, call.Actor.ID)
			if err != nil {
				return endpoint.Reply{}, err
			}
			votes = append(votes, wire)
		}
		body := pyjson.NewOrderedMap()
		body.Set("context_id", pyjson.String(contextID))
		body.Set("votes", votes)
		return endpoint.Reply{Body: body}, nil
	}}
}

// getVoteResults is GET /votes/{vote_id} (get_vote_results): the vote first,
// 404 before any permission question, then membership of its group.
func getVoteResults() Route {
	return Route{ID: "GET /votes/{vote_id}", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		store, record, err := loadVote(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		member, err := store.IsMember(ctx, record.ContextID, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireFacts(call, "view_votes", map[string]bool{"is_group_member": member}); err != nil {
			return endpoint.Reply{}, err
		}
		body, err := wireVote(*record, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: body}, nil
	}}
}

// castVoteBallot is POST /votes/{vote_id}/ballots (cast_vote_ballot): the
// service's own closed and option checks, then the repository's, which answer
// the same way when a concurrent request moved the row.
func castVoteBallot() Route {
	return Route{ID: "POST /votes/{vote_id}/ballots", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		request, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		optionID, err := uuidField(request, "option_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		store, record, err := loadVote(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		member, err := store.IsMember(ctx, record.ContextID, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireFacts(call, "cast_vote_ballot", map[string]bool{"is_group_member": member}); err != nil {
			return endpoint.Reply{}, err
		}
		if record.ClosedAt != nil {
			return endpoint.Reply{}, endpoint.Refuse(409, "vote_closed", "Vote is closed")
		}
		known := false
		for _, option := range record.Options {
			if option.ID == optionID {
				known = true
			}
		}
		if !known {
			return endpoint.Reply{}, endpoint.Refuse(422, "unknown_option", "Option does not belong to this vote")
		}
		voteID := record.ID
		ballot, replaced, err := store.UpsertBallot(ctx, voteID, optionID, call.Actor.ID, time.Now().UTC())
		var conflict *repo.Conflict
		if errors.As(err, &conflict) {
			switch conflict.Code {
			case "VOTE_NOT_FOUND":
				return endpoint.Reply{}, endpoint.Refuse(404, "vote_not_found", "Vote does not exist")
			case "VOTE_CLOSED":
				return endpoint.Reply{}, endpoint.Refuse(409, "vote_closed", "Vote is closed")
			case "UNKNOWN_OPTION":
				return endpoint.Reply{}, endpoint.Refuse(422, "unknown_option", "Option does not belong to this vote")
			}
		}
		if err != nil {
			return endpoint.Reply{}, err
		}
		body := pyjson.NewOrderedMap()
		body.Set("vote_id", pyjson.String(ballot.VoteID))
		body.Set("option_id", pyjson.String(ballot.OptionID))
		body.Set("voter_id", pyjson.String(ballot.VoterID))
		body.Set("created_at", pyjson.String(pyjson.DateTime(ballot.CreatedAt.UTC())))
		body.Set("updated_at", pyjson.String(pyjson.DateTime(ballot.UpdatedAt.UTC())))
		body.Set("replaced_previous_ballot", pyjson.Bool(replaced))
		return endpoint.Reply{Body: body}, nil
	}}
}

// closeVote is POST /votes/{vote_id}/close (close_vote): only a member who
// created the vote, once.
func closeVote() Route {
	return Route{ID: "POST /votes/{vote_id}/close", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		store, record, err := loadVote(ctx, call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		member, err := store.IsMember(ctx, record.ContextID, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := requireFacts(call, "close_vote", map[string]bool{
			"is_group_member": member,
			"is_vote_creator": record.CreatedByID == call.Actor.ID,
		}); err != nil {
			return endpoint.Reply{}, err
		}
		if record.ClosedAt != nil {
			return endpoint.Reply{}, endpoint.Refuse(409, "vote_already_closed", "Vote is already closed")
		}
		closed, err := store.CloseVote(ctx, record.ID, call.Actor.ID, time.Now().UTC())
		var conflict *repo.Conflict
		if errors.As(err, &conflict) {
			switch conflict.Code {
			case "VOTE_NOT_FOUND":
				return endpoint.Reply{}, endpoint.Refuse(404, "vote_not_found", "Vote does not exist")
			case "VOTE_ALREADY_CLOSED":
				return endpoint.Reply{}, endpoint.Refuse(409, "vote_already_closed", "Vote is already closed")
			}
		}
		if err != nil {
			return endpoint.Reply{}, err
		}
		body, err := wireVote(closed, call.Actor.ID)
		if err != nil {
			return endpoint.Reply{}, err
		}
		return endpoint.Reply{Body: body}, nil
	}}
}

// loadVote is `repository.get_vote(vote_id)` with its 404.
func loadVote(ctx context.Context, call *endpoint.Call) (repo.Repository, *repo.Vote, error) {
	voteID, err := pathUUID(call, "vote_id")
	if err != nil {
		return repo.Repository{}, nil, err
	}
	tx, err := call.Unit.Tx(ctx)
	if err != nil {
		return repo.Repository{}, nil, err
	}
	store := repo.Repository{Q: tx}
	record, err := store.GetVote(ctx, voteID)
	if err != nil {
		return repo.Repository{}, nil, err
	}
	if record == nil {
		return repo.Repository{}, nil, endpoint.Refuse(404, "vote_not_found", "Vote does not exist")
	}
	return store, record, nil
}

// wireVote is _wire_vote: VoteResponse, counted by vote.Tally. A tally
// refusal or a count missing for an option can only come from inconsistent
// rows; Python lets both escape, so they are errors here too.
func wireVote(record repo.Vote, actorID string) (*pyjson.OrderedMap, error) {
	options := make([]vote.Option, 0, len(record.Options))
	for _, option := range record.Options {
		options = append(options, vote.Option{ID: option.ID, Position: int(option.Position)})
	}
	ballots := make([]vote.Ballot, 0, len(record.Ballots))
	var myOptionID *string
	for _, ballot := range record.Ballots {
		ballots = append(ballots, vote.Ballot{VoterID: ballot.VoterID, OptionID: ballot.OptionID})
		if myOptionID == nil && ballot.VoterID == actorID {
			id := ballot.OptionID
			myOptionID = &id
		}
	}
	result, err := vote.Tally(options, ballots)
	if err != nil {
		return nil, err
	}
	optionList := pyjson.List{}
	for _, option := range record.Options {
		count, ok := result.CountOf(option.ID)
		if !ok {
			return nil, fmt.Errorf("routes: vote %s has no count for option %s", record.ID, option.ID)
		}
		item := pyjson.NewOrderedMap()
		item.Set("id", pyjson.String(option.ID))
		item.Set("position", pyjson.NewInt(option.Position))
		item.Set("label", pyjson.String(option.Label))
		item.Set("place_name", textOrNull(option.PlaceName))
		item.Set("ballot_count", pyjson.NewInt(int64(count)))
		optionList = append(optionList, item)
	}
	leading := pyjson.List{}
	for _, id := range result.LeadingOptionIDs {
		leading = append(leading, pyjson.String(id))
	}
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(record.ID))
	out.Set("context_id", pyjson.String(record.ContextID))
	out.Set("outing_id", textOrNull(record.OutingID))
	out.Set("created_by_id", pyjson.String(record.CreatedByID))
	out.Set("question", pyjson.String(record.Question))
	out.Set("created_at", pyjson.String(pyjson.DateTime(record.CreatedAt.UTC())))
	if record.ClosedAt == nil {
		out.Set("closed_at", pyjson.Null{})
	} else {
		out.Set("closed_at", pyjson.String(pyjson.DateTime(record.ClosedAt.UTC())))
	}
	out.Set("is_closed", pyjson.Bool(record.ClosedAt != nil))
	out.Set("options", optionList)
	out.Set("total_ballots", pyjson.NewInt(int64(result.TotalBallots)))
	out.Set("leading_option_ids", leading)
	out.Set("is_tie", pyjson.Bool(result.IsTie))
	out.Set("decided_option_id", textOrNull(result.DecidedOptionID))
	out.Set("my_option_id", textOrNull(myOptionID))
	return out, nil
}

// requireFacts is `_require_permission(action, actor, facts)`.
func requireFacts(call *endpoint.Call, action string, facts map[string]bool) error {
	refused, err := service.RequirePermission(action, *call.Actor, service.Resource{Proven: facts})
	if err != nil {
		return err
	}
	if refused != nil {
		return &endpoint.Refusal{Problem: *refused}
	}
	return nil
}

// optionalUUIDField reads a `UUID | None` field.
func optionalUUIDField(model *pyval.Model, name string) (*string, error) {
	value, err := field(model, name)
	if err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case pyjson.Null:
		return nil, nil
	case pyval.UUID:
		id := v.String()
		return &id, nil
	}
	return nil, fmt.Errorf("routes: %s.%s is %T, not a UUID or None", model.Class, name, value)
}

// modelListField reads a `list[Model]` field.
func modelListField(model *pyval.Model, name string) ([]*pyval.Model, error) {
	value, err := field(model, name)
	if err != nil {
		return nil, err
	}
	list, ok := value.(pyval.List)
	if !ok {
		return nil, fmt.Errorf("routes: %s.%s is %T, not a list", model.Class, name, value)
	}
	out := make([]*pyval.Model, len(list))
	for i, item := range list {
		m, ok := item.(*pyval.Model)
		if !ok {
			return nil, fmt.Errorf("routes: %s.%s[%d] is %T, not a model", model.Class, name, i, item)
		}
		out[i] = m
	}
	return out, nil
}
