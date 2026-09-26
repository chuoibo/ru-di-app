package routes

import (
	"context"
	"fmt"

	"mobile/services/core/internal/domain/pairpaper"
	"mobile/services/core/internal/domain/pairsteps"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/pyval"
)

// The eleven routes of routes/pair_papers.py. A command answers
// PaperCommandResponse and never content; content is read through
// GET /papers/{paper_id}, where the permission is checked at reading.

// listPairPapers is GET /contexts/{context_id}/papers (list_pair_papers).
func listPairPapers() Route {
	return Route{ID: "GET /contexts/{context_id}/papers", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		summaries, err := pairsteps.ListPapers(pairStore(ctx, call), pairActor(call), contextID, pairNow())
		if err != nil {
			return endpoint.Reply{}, pairError(err)
		}
		papers := pyjson.List{}
		for _, summary := range summaries {
			papers = append(papers, wirePaperSummary(summary))
		}
		body := pyjson.NewOrderedMap()
		body.Set("papers", papers)
		return endpoint.Reply{Body: body}, nil
	}}
}

// draftPairPaper is POST /contexts/{context_id}/papers/draft
// (draft_pair_paper): the body, if any, is not read.
func draftPairPaper() Route {
	return Route{ID: "POST /contexts/{context_id}/papers/draft", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		contextID, err := pathUUID(call, "context_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		command, err := pairsteps.DraftPaper(pairStore(ctx, call), pairActor(call), contextID, pairNow())
		if err != nil {
			return endpoint.Reply{}, pairError(err)
		}
		return endpoint.Reply{Body: wirePaperCommand(command)}, nil
	}}
}

// readPairPaper is GET /papers/{paper_id} (read_pair_paper, ApiService.pair_paper).
func readPairPaper() Route {
	return Route{ID: "GET /papers/{paper_id}", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		paperID, err := pathUUID(call, "paper_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		view, err := pairsteps.ReadPaper(pairStore(ctx, call), pairActor(call), paperID, pairNow())
		if err != nil {
			return endpoint.Reply{}, pairError(err)
		}
		return endpoint.Reply{Body: wirePaper(view)}, nil
	}}
}

// editPairDraft is PATCH /papers/{paper_id}/draft (edit_pair_draft).
func editPairDraft() Route {
	return Route{ID: "PATCH /papers/{paper_id}/draft", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		paperID, err := pathUUID(call, "paper_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		request, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		content, lyDo, err := paperContentAndReason(request)
		if err != nil {
			return endpoint.Reply{}, err
		}
		command, err := pairsteps.EditDraft(pairStore(ctx, call), pairActor(call), paperID, content, lyDo, pairNow())
		if err != nil {
			return endpoint.Reply{}, pairError(err)
		}
		return endpoint.Reply{Body: wirePaperCommand(command)}, nil
	}}
}

// sendPairPaper is POST /papers/{paper_id}/send (send_pair_paper).
func sendPairPaper() Route {
	return Route{ID: "POST /papers/{paper_id}/send", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		paperID, version, err := paperAndBodyVersion(call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		command, err := pairsteps.SendPaper(pairStore(ctx, call), pairActor(call), paperID, version, pairNow())
		if err != nil {
			return endpoint.Reply{}, pairError(err)
		}
		return endpoint.Reply{Body: wirePaperCommand(command)}, nil
	}}
}

// markPairPaperViewed is POST /papers/{paper_id}/versions/{version}/viewed
// (mark_pair_paper_viewed): `Response(status_code=204)`.
func markPairPaperViewed() Route {
	return Route{ID: "POST /papers/{paper_id}/versions/{version}/viewed", Status: 204, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		paperID, err := pathUUID(call, "paper_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		version, err := pathVersion(call, "version")
		if err != nil {
			return endpoint.Reply{}, err
		}
		if err := pairsteps.MarkViewed(pairStore(ctx, call), pairActor(call), paperID, version, pairNow()); err != nil {
			return endpoint.Reply{}, pairError(err)
		}
		return endpoint.Reply{Empty: true}, nil
	}}
}

// respondPairPaper is POST /papers/{paper_id}/versions/{version}/responses
// (respond_pair_paper): the body is a PaperAgreeRequest or a
// PaperReviseRequest, told apart by its `kind`.
func respondPairPaper() Route {
	return Route{ID: "POST /papers/{paper_id}/versions/{version}/responses", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		paperID, err := pathUUID(call, "paper_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		version, err := pathVersion(call, "version")
		if err != nil {
			return endpoint.Reply{}, err
		}
		request, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		kind, err := stringField(request, "kind")
		if err != nil {
			return endpoint.Reply{}, err
		}
		reply := pairsteps.Reply{Kind: kind}
		if kind != "dong_y" {
			if reply.Content, reply.LyDo, err = paperContentAndReason(request); err != nil {
				return endpoint.Reply{}, err
			}
		}
		command, err := pairsteps.RespondPaper(pairStore(ctx, call), pairActor(call), paperID, version, reply, pairNow())
		if err != nil {
			return endpoint.Reply{}, pairError(err)
		}
		return endpoint.Reply{Body: wirePaperCommand(command)}, nil
	}}
}

// withdrawPairPaper is POST /papers/{paper_id}/withdraw (withdraw_pair_paper).
func withdrawPairPaper() Route {
	return Route{ID: "POST /papers/{paper_id}/withdraw", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		paperID, version, err := paperAndBodyVersion(call)
		if err != nil {
			return endpoint.Reply{}, err
		}
		command, err := pairsteps.WithdrawPaper(pairStore(ctx, call), pairActor(call), paperID, version, pairNow())
		if err != nil {
			return endpoint.Reply{}, pairError(err)
		}
		return endpoint.Reply{Body: wirePaperCommand(command)}, nil
	}}
}

// skipPairWeek is POST /papers/{paper_id}/skip (skip_pair_week).
func skipPairWeek() Route {
	return Route{ID: "POST /papers/{paper_id}/skip", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		paperID, err := pathUUID(call, "paper_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		command, err := pairsteps.SkipWeek(pairStore(ctx, call), pairActor(call), paperID, pairNow())
		if err != nil {
			return endpoint.Reply{}, pairError(err)
		}
		return endpoint.Reply{Body: wirePaperCommand(command)}, nil
	}}
}

// recordPairOutingDone is POST /papers/{paper_id}/done
// (record_pair_outing_done).
func recordPairOutingDone() Route {
	return Route{ID: "POST /papers/{paper_id}/done", Status: 200, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		paperID, err := pathUUID(call, "paper_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		command, err := pairsteps.RecordDone(pairStore(ctx, call), pairActor(call), paperID, pairNow())
		if err != nil {
			return endpoint.Reply{}, pairError(err)
		}
		return endpoint.Reply{Body: wirePaperCommand(command)}, nil
	}}
}

// keepPairPaperLine is POST /papers/{paper_id}/keeps (keep_pair_paper_line).
func keepPairPaperLine() Route {
	return Route{ID: "POST /papers/{paper_id}/keeps", Status: 201, Serve: func(ctx context.Context, call *endpoint.Call) (endpoint.Reply, error) {
		paperID, err := pathUUID(call, "paper_id")
		if err != nil {
			return endpoint.Reply{}, err
		}
		request, err := bodyModel(call, "request")
		if err != nil {
			return endpoint.Reply{}, err
		}
		line, err := stringField(request, "line")
		if err != nil {
			return endpoint.Reply{}, err
		}
		keep, err := pairsteps.KeepLine(pairStore(ctx, call), pairActor(call), paperID, line, pairNow())
		if err != nil {
			return endpoint.Reply{}, pairError(err)
		}
		return endpoint.Reply{Body: wireKeep(keep)}, nil
	}}
}

// paperAndBodyVersion reads the path paper_id and the body's PaperVersion.
func paperAndBodyVersion(call *endpoint.Call) (string, int64, error) {
	paperID, err := pathUUID(call, "paper_id")
	if err != nil {
		return "", 0, err
	}
	request, err := bodyModel(call, "request")
	if err != nil {
		return "", 0, err
	}
	version, err := intField(request, "version")
	if err != nil {
		return "", 0, err
	}
	return paperID, saturated(version), nil
}

// paperContentAndReason reads `content: PaperContentInput` and
// `ly_do: str | None` of a draft edit or a counter-proposal.
func paperContentAndReason(request *pyval.Model) (pairsteps.ContentInput, *string, error) {
	contentModel, err := modelField(request, "content")
	if err != nil {
		return pairsteps.ContentInput{}, nil, err
	}
	value, err := field(contentModel, "ngay")
	if err != nil {
		return pairsteps.ContentInput{}, nil, err
	}
	ngay, ok := value.(pyval.Date)
	if !ok {
		return pairsteps.ContentInput{}, nil, fmt.Errorf("routes: %s.ngay is %T, not a date", contentModel.Class, value)
	}
	stops, err := modelListField(contentModel, "chang")
	if err != nil {
		return pairsteps.ContentInput{}, nil, err
	}
	content := pairsteps.ContentInput{
		Ngay:  pairpaper.Date{Year: ngay.Year, Month: ngay.Month, Day: ngay.Day},
		Chang: make([]pairsteps.StopInput, len(stops)),
	}
	for i, stop := range stops {
		gio, err := stringField(stop, "gio")
		if err != nil {
			return pairsteps.ContentInput{}, nil, err
		}
		viec, err := stringField(stop, "viec")
		if err != nil {
			return pairsteps.ContentInput{}, nil, err
		}
		// A catalogue id (slug), the spelling OutingStopInput.place_id takes.
		placeID, err := optionalStringField(stop, "place_id")
		if err != nil {
			return pairsteps.ContentInput{}, nil, err
		}
		canKiem, err := boolField(stop, "can_kiem")
		if err != nil {
			return pairsteps.ContentInput{}, nil, err
		}
		content.Chang[i] = pairsteps.StopInput{Gio: gio, Viec: viec, PlaceID: placeID, CanKiem: canKiem}
	}
	lyDo, err := optionalStringField(request, "ly_do")
	if err != nil {
		return pairsteps.ContentInput{}, nil, err
	}
	return content, lyDo, nil
}

// wirePaperCommand is PaperCommandResponse.
func wirePaperCommand(command pairsteps.Command) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(command.ID))
	out.Set("state", pyjson.String(command.State))
	out.Set("version", pyjson.NewInt(int64(command.Version)))
	out.Set("outing_id", textOrNull(command.OutingID))
	return out
}

// wirePaperSummary is PaperSummary.
func wirePaperSummary(summary pairsteps.PaperSummary) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(summary.ID))
	out.Set("state", pyjson.String(summary.State))
	out.Set("version", pyjson.NewInt(int64(summary.Version)))
	out.Set("tuan", pyjson.String(summary.Tuan.ISOFormat()))
	if summary.Ngay == nil {
		out.Set("ngay", pyjson.Null{})
	} else {
		out.Set("ngay", pyjson.String(summary.Ngay.ISOFormat()))
	}
	out.Set("expires_at", dateTimeValue(summary.ExpiresAt))
	if summary.ChangDau == nil {
		out.Set("chang_dau", pyjson.Null{})
	} else {
		stop := pyjson.NewOrderedMap()
		stop.Set("gio", pyjson.String(summary.ChangDau.Gio))
		stop.Set("viec", pyjson.String(summary.ChangDau.Viec))
		out.Set("chang_dau", stop)
	}
	out.Set("dong_giu_dau", textOrNull(summary.DongGiuDau))
	return out
}

// wirePaper is PaperResponse.
func wirePaper(view pairsteps.PaperView) *pyjson.OrderedMap {
	versions := pyjson.List{}
	for _, version := range view.Versions {
		versions = append(versions, wirePaperVersion(version))
	}
	keeps := pyjson.List{}
	for _, keep := range view.Keeps {
		keeps = append(keeps, wireKeep(keep))
	}
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(view.ID))
	out.Set("state", pyjson.String(view.State))
	out.Set("version", pyjson.NewInt(int64(view.Version)))
	out.Set("author_type", pyjson.String(view.AuthorType))
	out.Set("sent_by", textOrNull(view.SentBy))
	out.Set("tuan", pyjson.String(view.Tuan.ISOFormat()))
	out.Set("expires_at", dateTimeValue(view.ExpiresAt))
	out.Set("outing_id", textOrNull(view.OutingID))
	out.Set("co_the_ghi_da_di", pyjson.Bool(view.CoTheGhiDaDi))
	out.Set("versions", versions)
	out.Set("keeps", keeps)
	return out
}

// wirePaperVersion is PaperVersionResponse.
//
// Note the shape of this function, because it is load-bearing: every object is
// built field by field, in the order `class PaperContent` and `class PaperStop`
// declare. That is what keeps it correct, not luck.
//
// `content` and `nguon` are jsonb columns, and jsonb does not keep the order it
// was written in -- Postgres sorts keys by length and then bytewise. Python
// never sees that order because the row passes through a nested model on its
// way out. Rewriting this to echo the column straight through would be shorter,
// would serve the same JSON, and would ship a different wire: measured on the
// two places columns that DID echo, the bodies were byte-for-byte the same
// LENGTH (5694 against 5694) and differed from byte 489. Only a byte comparison
// notices, so every test but parity would stay green.
func wirePaperVersion(version pairsteps.VersionView) *pyjson.OrderedMap {
	stops := pyjson.List{}
	for _, stop := range version.Content.Chang {
		item := pyjson.NewOrderedMap()
		item.Set("gio", pyjson.String(stop.Gio))
		item.Set("viec", pyjson.String(stop.Viec))
		item.Set("place_id", textOrNull(stop.PlaceID))
		item.Set("can_kiem", pyjson.Bool(stop.CanKiem))
		stops = append(stops, item)
	}
	content := pyjson.NewOrderedMap()
	content.Set("ngay", pyjson.String(version.Content.Ngay.ISOFormat()))
	content.Set("chang", stops)
	out := pyjson.NewOrderedMap()
	out.Set("version", pyjson.NewInt(int64(version.Version)))
	out.Set("content", content)
	out.Set("ly_do", textOrNull(version.LyDo))
	out.Set("author_type", pyjson.String(version.AuthorType))
	out.Set("sent_at", optionalDateTimeValue(version.SentAt))
	out.Set("sent_by", textOrNull(version.SentBy))
	out.Set("my_response", textOrNull(version.MyResponse))
	out.Set("their_agreed", pyjson.Bool(version.TheirAgreed))
	out.Set("viewed_by_recipient_at", optionalDateTimeValue(version.ViewedByRecipientAt))
	return out
}

// wireKeep is PaperKeepResponse.
func wireKeep(keep pairsteps.Keep) *pyjson.OrderedMap {
	out := pyjson.NewOrderedMap()
	out.Set("id", pyjson.String(keep.ID))
	out.Set("line", pyjson.String(keep.Line))
	out.Set("created_at", dateTimeValue(keep.CreatedAt))
	return out
}
