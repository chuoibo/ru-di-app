//go:build postgres

package repo

// Differential test of the W2 repository methods (friends, stories, posts,
// votes) against the real SqlAlchemyApiRepository, driven through
// scripts/render_social_repo_oracle.py (render_repo_oracle.py plus these
// methods). Same design as oracle_postgres_test.go, whose case format, value
// tags, statement normalisation, probes and comparison it reuses, except that
// each side gets its own identically migrated private schema in the one
// database (TestSocialRepositoryOracle says why): each case seeded with
// identical literal SQL in a rolled-back transaction on each side, then the returned
// value (or the exception class, SQLSTATE, constraint, RepositoryConflict code
// and what the conflict was raised from), every statement and the probes
// compared.
//
// SQLAlchemy's session answers session.get of an object it already loaded
// from its identity map, without a statement, and names savepoints by a
// per-connection counter. The Go methods are per call, so a case loads a given
// object by id at most once and calls add_post_reaction at most once.
//
// Without CORE_PYTHON_IMAGE the test skips; scripts/go_postgres_tier.sh sets
// it and refuses skips.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---------------------------------------------------------------------------
// Tags
// ---------------------------------------------------------------------------

func tFriendEdge(e FriendEdge) any {
	return tRecord("FriendEdgeRecord", "id", tUUID(e.ID), "requester_id", tUUID(e.RequesterID),
		"addressee_id", tUUID(e.AddresseeID), "other_person_id", tUUID(e.OtherPersonID),
		"other_display_name", tStr(e.OtherDisplayName), "state", tStr(e.State),
		"decided_by_id", optional(e.DecidedByID, tUUID), "created_at", tInstant(e.CreatedAt),
		"decided_at", optional(e.DecidedAt, tInstant))
}

func tFriendEdges(edges []FriendEdge) any {
	items := []any{}
	for _, e := range edges {
		items = append(items, tFriendEdge(e))
	}
	return tSeq(items)
}

func tStory(s Story) any {
	return tRecord("StoryRecord", "id", tUUID(s.ID), "author_id", tUUID(s.AuthorID),
		"author_display_name", tStr(s.AuthorDisplayName), "image_url", tStr(s.ImageURL),
		"caption", optional(s.Caption, tStr), "audience", tStr(s.Audience), "created_at", tInstant(s.CreatedAt),
		"expires_at", tInstant(s.ExpiresAt), "seen", tBool(s.Seen))
}

func tPost(p Post) any {
	return tRecord("PostRecord", "id", tUUID(p.ID), "author_id", tUUID(p.AuthorID), "audience", tStr(p.Audience),
		"context_id", optional(p.ContextID, tUUID), "body", tStr(p.Body), "image_url", optional(p.ImageURL, tStr),
		"created_at", tInstant(p.CreatedAt))
}

func tPosts(posts []Post) any {
	items := []any{}
	for _, p := range posts {
		items = append(items, tPost(p))
	}
	return tSeq(items)
}

func tPostComment(c PostComment) any {
	return tRecord("PostCommentRecord", "id", tUUID(c.ID), "post_id", tUUID(c.PostID), "author_id", tUUID(c.AuthorID),
		"author_display_name", tStr(c.AuthorDisplayName), "body", tStr(c.Body), "created_at", tInstant(c.CreatedAt))
}

// tSet is render_social_repo_oracle.py's set tag: members sorted by their
// tagged JSON text. Every member here is an ASCII string, whose JSON text
// sorts as the string does.
func tSet(values []string) any {
	sorted := append([]string{}, values...)
	sort.Strings(sorted)
	items := []any{}
	for _, v := range sorted {
		items = append(items, tStr(v))
	}
	return tv("set", items)
}

func tSocialCounts(c PostSocialCounts) any {
	reactions := []any{}
	for _, r := range c.Reactions {
		kinds := []any{}
		for _, k := range r.Kinds {
			kinds = append(kinds, []any{tStr(k.Kind), tInt(k.Count)})
		}
		reactions = append(reactions, []any{tUUID(r.PostID), tv("dict", kinds)})
	}
	comments := []any{}
	for _, t := range c.Comments {
		comments = append(comments, []any{tUUID(t.PostID), tInt(t.Count)})
	}
	mine := []any{}
	for _, m := range c.Mine {
		mine = append(mine, []any{tUUID(m.PostID), tSet(m.Kinds)})
	}
	return tRecord("PostSocialCounts", "reactions", tv("dict", reactions), "comments", tv("dict", comments),
		"mine", tv("dict", mine))
}

func tOuting(o Outing) any {
	stops := []any{}
	for _, s := range o.Stops {
		stops = append(stops, tRecord("OutingStopRecord", "id", tUUID(s.ID), "position", tInt(s.Position),
			"minute_of_day", tInt(s.MinuteOfDay), "label", tStr(s.Label), "place_name", optional(s.PlaceName, tStr),
			"place_id", optional(s.PlaceID, tStr), "day", optional(s.Day, tDay),
			"duration_minutes", optional(s.DurationMinutes, tInt), "time_locked", tBool(s.TimeLocked),
			"meeting_lat", optional(s.MeetingLat, tFloat), "meeting_lng", optional(s.MeetingLng, tFloat),
			"meeting_label", optional(s.MeetingLabel, tStr)))
	}
	days := []any{}
	for _, d := range o.ItineraryDays {
		days = append(days, tJSON(d))
	}
	return tRecord("OutingRecord", "id", tUUID(o.ID), "context_id", tUUID(o.ContextID), "created_by_id", tUUID(o.CreatedByID),
		"title", tStr(o.Title), "starts_on", tDay(o.StartsOn), "ends_on", tDay(o.EndsOn), "headcount", tInt(o.Headcount),
		"budget_per_person_vnd", tInt(o.BudgetPerPersonVND), "created_at", tInstant(o.CreatedAt), "stops", tSeq(stops),
		"timeline_revision", tInt(o.TimelineRevision), "itinerary_version", tInt(o.ItineraryVersion),
		"itinerary_days", tSeq(days))
}

func tBallot(b VoteBallot) any {
	return tRecord("VoteBallotRecord", "id", tUUID(b.ID), "vote_id", tUUID(b.VoteID), "option_id", tUUID(b.OptionID),
		"voter_id", tUUID(b.VoterID), "created_at", tInstant(b.CreatedAt), "updated_at", tInstant(b.UpdatedAt))
}

func tVote(v Vote) any {
	options := []any{}
	for _, o := range v.Options {
		options = append(options, tRecord("VoteOptionRecord", "id", tUUID(o.ID), "vote_id", tUUID(o.VoteID),
			"position", tInt(o.Position), "label", tStr(o.Label), "place_name", optional(o.PlaceName, tStr)))
	}
	ballots := []any{}
	for _, b := range v.Ballots {
		ballots = append(ballots, tBallot(b))
	}
	return tRecord("VoteRecord", "id", tUUID(v.ID), "context_id", tUUID(v.ContextID), "outing_id", optional(v.OutingID, tUUID),
		"created_by_id", tUUID(v.CreatedByID), "question", tStr(v.Question), "created_at", tInstant(v.CreatedAt),
		"closed_at", optional(v.ClosedAt, tInstant), "closed_by_id", optional(v.ClosedByID, tUUID),
		"options", tSeq(options), "ballots", tSeq(ballots))
}

func nilOr[T any](v *T, tag func(T) any) any { return optional(v, tag) }

// ---------------------------------------------------------------------------
// The Go side
// ---------------------------------------------------------------------------

func argInt(a map[string]any, key string) int { return int(a[key].(float64)) }

func socialGoCall(repo Repository, method string, a map[string]any) (any, error) {
	s := func(key string) string { return argString(a, key) }
	at := func(key string) time.Time { return argInstant(s(key)) }
	switch method {
	case "get_friend_edge":
		e, err := repo.GetFriendEdge(bg, s("person_a"), s("person_b"))
		return nilOr(e, tFriendEdge), err
	case "get_friend_request":
		e, err := repo.GetFriendRequest(bg, s("request_id"), s("reader_id"))
		return nilOr(e, tFriendEdge), err
	case "open_friend_request":
		e, err := repo.OpenFriendRequest(bg, s("requester_id"), s("addressee_id"), at("now"))
		return tFriendEdge(e), err
	case "decide_friend_request":
		e, err := repo.DecideFriendRequest(bg, s("request_id"), s("state"), s("decided_by_id"), at("now"))
		return nilOr(e, tFriendEdge), err
	case "list_friend_requests":
		edges, err := repo.ListFriendRequests(bg, s("person_id"), s("direction"))
		return tFriendEdges(edges), err
	case "list_friends":
		edges, err := repo.ListFriends(bg, s("person_id"))
		return tFriendEdges(edges), err
	case "get_account_identity":
		i, err := repo.GetAccountIdentity(bg, s("provider"), s("subject"))
		return nilOr(i, func(i AccountIdentity) any {
			return tRecord("AccountIdentityRecord", "id", tUUID(i.ID), "person_id", tUUID(i.PersonID),
				"provider", tStr(i.Provider), "subject", tStr(i.Subject), "created_at", tInstant(i.CreatedAt),
				"last_login_at", tInstant(i.LastLoginAt))
		}), err
	case "get_person_image":
		m, err := repo.GetPersonImage(bg, s("person_id"), s("image_id"))
		return nilOr(m, func(m UploadedImage) any {
			return tRecord("UploadedImageRecord", "id", tUUID(m.ID), "storage_key", tStr(m.StorageKey),
				"context_id", optional(m.ContextID, tUUID), "owner_person_id", optional(m.OwnerPersonID, tUUID),
				"uploaded_by_id", tUUID(m.UploadedByID), "content_type", tStr(m.ContentType), "byte_size", tInt(m.ByteSize),
				"width", tInt(m.Width), "height", tInt(m.Height), "created_at", tInstant(m.CreatedAt), "purpose", tStr(m.Purpose))
		}), err
	case "create_story":
		story, err := repo.CreateStory(bg, StoryInput{AuthorID: s("author_id"), ImageURL: s("image_url"),
			Caption: argOptional(a, "caption"), Audience: s("audience"), Now: at("now"), ExpiresAt: at("expires_at")})
		return tStory(story), err
	case "get_story":
		story, err := repo.GetStory(bg, s("story_id"))
		return nilOr(story, tStory), err
	case "list_live_stories_for":
		stories, err := repo.ListLiveStoriesFor(bg, s("reader_id"), at("now"))
		items := []any{}
		for _, story := range stories {
			items = append(items, tStory(story))
		}
		return tSeq(items), err
	case "mark_story_seen":
		seen, err := repo.MarkStorySeen(bg, s("story_id"), s("viewer_id"), at("now"))
		return tInstant(seen), err
	case "delete_story":
		return nil, repo.DeleteStory(bg, s("story_id"))
	case "create_post":
		post, err := repo.CreatePost(bg, PostInput{AuthorID: s("author_id"), Audience: s("audience"),
			ContextID: argOptional(a, "context_id"), Body: s("body"), ImageURL: argOptional(a, "image_url"), Now: at("now")})
		return tPost(post), err
	case "get_post":
		post, err := repo.GetPost(bg, s("post_id"))
		return nilOr(post, tPost), err
	case "list_posts_visible_to":
		posts, err := repo.ListPostsVisibleTo(bg, s("reader_id"), argInt(a, "limit"))
		return tPosts(posts), err
	case "list_person_posts_visible_to":
		posts, err := repo.ListPersonPostsVisibleTo(bg, s("person_id"), s("reader_id"), argInt(a, "limit"))
		return tPosts(posts), err
	case "post_social_counts":
		counts, err := repo.PostSocialCounts(bg, argStrings(a["post_ids"]), s("viewer_id"))
		return tSocialCounts(counts), err
	case "add_post_reaction":
		r, err := repo.AddPostReaction(bg, s("post_id"), s("person_id"), s("kind"), at("now"))
		return tRecord("PostReactionRecord", "id", tUUID(r.ID), "post_id", tUUID(r.PostID), "person_id", tUUID(r.PersonID),
			"kind", tStr(r.Kind), "created_at", tInstant(r.CreatedAt)), err
	case "remove_post_reaction":
		removed, err := repo.RemovePostReaction(bg, s("post_id"), s("person_id"), s("kind"))
		return tBool(removed), err
	case "create_post_comment":
		c, err := repo.CreatePostComment(bg, s("post_id"), s("author_id"), s("body"), at("now"))
		return tPostComment(c), err
	case "get_post_comment":
		c, err := repo.GetPostComment(bg, s("comment_id"))
		return nilOr(c, tPostComment), err
	case "delete_post_comment":
		deleted, err := repo.DeletePostComment(bg, s("comment_id"))
		return tBool(deleted), err
	case "list_post_comments":
		var after *PostCommentCursor
		if pair, ok := a["after"].([]any); ok {
			after = &PostCommentCursor{CreatedAt: argInstant(pair[0].(string)), ID: pair[1].(string)}
		}
		comments, err := repo.ListPostComments(bg, s("post_id"), argInt(a, "limit"), after)
		items := []any{}
		for _, c := range comments {
			items = append(items, tPostComment(c))
		}
		return tSeq(items), err
	case "get_outing":
		o, err := repo.GetOuting(bg, s("outing_id"))
		return nilOr(o, tOuting), err
	case "create_vote":
		options := []VoteOptionInput{}
		for _, item := range a["options"].([]any) {
			option := item.(map[string]any)
			options = append(options, VoteOptionInput{Label: argString(option, "label"), PlaceName: argOptional(option, "place_name")})
		}
		v, err := repo.CreateVote(bg, VoteInput{ContextID: s("context_id"), OutingID: argOptional(a, "outing_id"),
			CreatedByID: s("created_by_id"), Question: s("question"), Options: options, Now: at("now")})
		return tVote(v), err
	case "get_vote":
		v, err := repo.GetVote(bg, s("vote_id"))
		return nilOr(v, tVote), err
	case "list_votes":
		votes, err := repo.ListVotes(bg, s("context_id"))
		items := []any{}
		for _, v := range votes {
			items = append(items, tVote(v))
		}
		return tSeq(items), err
	case "upsert_ballot":
		b, replaced, err := repo.UpsertBallot(bg, s("vote_id"), s("option_id"), s("voter_id"), at("now"))
		return tSeq([]any{tBallot(b), tBool(replaced)}), err
	case "close_vote":
		v, err := repo.CloseVote(bg, s("vote_id"), s("closed_by_id"), at("now"))
		return tVote(v), err
	case "get_person", "is_member":
		return goCall(repo, method, a)
	}
	panic("unknown method " + method)
}

// socialGoError is goError plus render_social_repo_oracle.py's `code` and
// `cause`, and the Python exception classes the W2 methods raise themselves.
func socialGoError(err error) map[string]any {
	var conflict *Conflict
	if errors.As(err, &conflict) {
		out := map[string]any{"type": "RepositoryConflict", "sqlstate": nil, "constraint": nil,
			"code": conflict.Code, "cause": nil}
		var pg *pgconn.PgError
		var refusal *FriendshipRefusal
		switch {
		case errors.As(conflict.Err, &pg):
			cause := map[string]any{"type": pythonErrorClass(pg.Code), "sqlstate": pg.Code, "constraint": nil}
			if pg.ConstraintName != "" {
				cause["constraint"] = pg.ConstraintName
			}
			out["cause"] = cause
		case errors.As(conflict.Err, &refusal):
			out["cause"] = map[string]any{"type": "FriendshipError", "sqlstate": nil, "constraint": nil}
		case conflict.Err != nil:
			out["cause"] = map[string]any{"type": "go error: " + conflict.Err.Error(), "sqlstate": nil, "constraint": nil}
		}
		return out
	}
	out := goError(err)
	switch {
	case errors.Is(err, ErrUnknownPostAudience), errors.Is(err, ErrUnknownFriendRequestState):
		out["type"] = "ValueError"
	case errors.Is(err, ErrStoryViewVanished):
		out["type"] = "RuntimeError"
	case errors.Is(err, ErrReactionVanished):
		out["type"] = "AssertionError"
	}
	out["code"], out["cause"] = nil, nil
	return out
}

func runSocialGoCase(t *testing.T, pool *pgxpool.Pool, c oracleCase) []any {
	t.Helper()
	tx, err := pool.Begin(bg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(bg) }()
	for _, sql := range c.Setup {
		if _, err := tx.Exec(bg, sql); err != nil {
			t.Fatalf("setup: %v\n%s", err, sql)
		}
	}
	steps := []any{}
	for _, step := range c.Steps {
		for _, sql := range step.Before {
			if _, err := tx.Exec(bg, sql); err != nil {
				t.Fatalf("before: %v\n%s", err, sql)
			}
		}
		rec := &recorder{Querier: tx}
		value, err := socialGoCall(Repository{Q: rec}, step.Call, step.Args)
		statements := []any{}
		for _, sql := range rec.log {
			statements = append(statements, normalizeSQL(sql))
		}
		out := map[string]any{"result": nil, "error": nil, "warnings": []any{}, "statements": statements, "probes": nil}
		if err != nil {
			out["error"] = socialGoError(err)
			steps = append(steps, generic(t, out))
			break
		}
		out["result"] = value
		probes := []any{}
		for _, sql := range step.Probes {
			rows, err := tx.Query(bg, sql)
			if err != nil {
				t.Fatalf("probe: %v\n%s", err, sql)
			}
			texts, err := pgx.CollectRows(rows, pgx.RowTo[string])
			if err != nil {
				t.Fatal(err)
			}
			probes = append(probes, append([]string{}, texts...))
		}
		out["probes"] = probes
		steps = append(steps, generic(t, out))
	}
	return steps
}

// ---------------------------------------------------------------------------
// Cases
// ---------------------------------------------------------------------------

// socialCase is an oracleCase with what its last step must end in on the
// Python side: "" for a normal return, otherwise the exception class, or the
// RepositoryConflict code. A case that means to reach a refusal and does not
// is a broken fixture, not a pass.
type socialCase struct {
	oracleCase
	wantEnd string
}

func socialOracleCases() ([]socialCase, oracleSpec) {
	w := newSocialWorld()
	var cases []socialCase
	add := func(name, wantEnd string, setup []string, steps ...oracleCall) {
		cases = append(cases, socialCase{oracleCase{Name: name, Setup: append([]string{}, setup...), Steps: steps}, wantEnd})
	}
	read := func(method string, args map[string]any) oracleCall {
		return oracleCall{Call: method, Args: args, Before: append([]string{}, writesBaseline...),
			Probes: []string{probeLocks, probeWrites}}
	}
	write := func(method string, args map[string]any, dumps ...string) oracleCall {
		return oracleCall{Call: method, Args: args, Before: append([]string{}, writesBaseline...),
			Probes: append([]string{probeLocks, probeWrites}, dumps...)}
	}
	args := func(pairs ...any) map[string]any {
		out := map[string]any{}
		for i := 0; i < len(pairs); i += 2 {
			out[pairs[i].(string)] = pairs[i+1]
		}
		return out
	}
	list := func(ids ...string) []any {
		out := []any{}
		for _, id := range ids {
			out = append(out, id)
		}
		return out
	}
	base := w.sql
	vietnam := append([]string{"SET LOCAL TimeZone = 'Asia/Ho_Chi_Minh'"}, base...)
	friendRequests := dumpProbe("friend_requests", "t.created_at, t.id")
	stories := dumpProbe("stories", "t.created_at, t.id")
	storyViews := dumpProbe("story_views", "t.story_id, t.viewer_id")
	posts := dumpProbe("posts", "t.created_at, t.id")
	postReactions := dumpProbe("post_reactions", "t.post_id, t.person_id, t.kind")
	postComments := dumpProbe("post_comments", "t.created_at, t.id")
	votes := dumpProbe("votes", "t.created_at, t.question")
	voteOptions := dumpProbe("vote_options", "t.label, t.position")
	voteBallots := dumpProbe("vote_ballots", "t.vote_id, t.voter_id")

	// --- friends ------------------------------------------------------------
	edge := func(a, b string) oracleCall { return read("get_friend_edge", args("person_a", a, "person_b", b)) }
	add("get_friend_edge: every kind of pair, both ways round", "", base,
		edge(w.an, w.binh), edge(w.binh, w.an), edge(w.an, w.dung), edge(w.dung, w.chi), edge(w.binh, w.phuong),
		edge(w.an, w.phuong), edge(w.an, w.an), edge(w.an, w.missingPerson), edge(w.khoa, w.binh), edge(w.an, w.em),
		edge(w.giang, w.chi), edge(w.hoa, w.khoa))
	request := func(id, reader string) oracleCall {
		return read("get_friend_request", args("request_id", id, "reader_id", reader))
	}
	add("get_friend_request: parties, a stranger and a missing id", "", base,
		request(w.frChiAn, w.an), request(w.frChiAn, w.chi), request(w.frChiAn, w.giang), request(w.missingRequest, w.an),
		request(w.frPhuongBinh, w.phuong), request(w.frAnDungDeclined, w.dung))
	requests := func(person, direction string) oracleCall {
		return read("list_friend_requests", args("person_id", person, "direction", direction))
	}
	add("list_friend_requests: directions, ties, answered rows left out", "", base,
		requests(w.an, "incoming"), requests(w.an, "outgoing"), requests(w.an, "sideways"), requests(w.an, "Incoming"),
		requests(w.binh, "incoming"), requests(w.khoa, "incoming"), requests(w.missingPerson, "incoming"))
	friends := func(person string) oracleCall { return read("list_friends", args("person_id", person)) }
	add("list_friends: ties on decided_at, erased and unnamed friends", "", base,
		friends(w.an), friends(w.binh), friends(w.giang), friends(w.khoa), friends(w.hoa), friends(w.missingPerson))
	add("list_friends and list_friend_requests: under a Vietnam session TimeZone", "", vietnam,
		friends(w.binh), requests(w.an, "incoming"))

	open := func(requester, addressee, now string) oracleCall {
		return write("open_friend_request", args("requester_id", requester, "addressee_id", addressee, "now", now), friendRequests)
	}
	add("open_friend_request: a new pair", "", base, open(w.giang, w.hoa, "2030-03-10T08:00:00.123456+07:00"))
	add("open_friend_request: asking again after a decline", "", base, open(w.dung, w.chi, "2030-03-10T00:00:00Z"))
	add("open_friend_request: an erased requester is still written", "", base, open(w.em, w.giang, "2030-03-10T00:00:01Z"))
	add("open_friend_request: the pair already holds a friendship", "FRIEND_EDGE_EXISTS", base, open(w.binh, w.an, "2030-03-10T00:00:00Z"))
	add("open_friend_request: the reverse of a pending request", "FRIEND_EDGE_EXISTS", base, open(w.an, w.chi, "2030-03-10T00:00:00Z"))
	add("open_friend_request: a blocked pair", "FRIEND_EDGE_EXISTS", base, open(w.an, w.phuong, "2030-03-10T00:00:00Z"))
	add("open_friend_request: an addressee with no people row", "FRIEND_EDGE_EXISTS", base, open(w.an, w.missingPerson, "2030-03-10T00:00:00Z"))
	add("open_friend_request: to oneself", "FRIEND_EDGE_EXISTS", base, open(w.an, w.an, "2030-03-10T00:00:00Z"))

	decide := func(id, state, by, now string) oracleCall {
		return write("decide_friend_request", args("request_id", id, "state", state, "decided_by_id", by, "now", now), friendRequests)
	}
	add("decide_friend_request: accept a pending request", "", base, decide(w.frChiAn, "accepted", w.an, "2030-03-11T00:00:00.000001Z"))
	add("decide_friend_request: decline, then the inbox and asking again", "", base,
		decide(w.frHoaAn, "declined", w.an, "2030-03-11T07:00:00+07:00"), requests(w.an, "incoming"),
		open(w.hoa, w.an, "2030-03-12T00:00:00Z"))
	add("decide_friend_request: the requester blocks a pending request", "", base, decide(w.frAnKhoa, "blocked", w.an, "2030-03-11T00:00:00Z"))
	add("decide_friend_request: a block by the one who accepted, at the stored instant", "", base,
		decide(w.frAnBinh, "blocked", w.binh, w.decidedAnBinh))
	add("decide_friend_request: a block by the one who accepted, later", "", base,
		decide(w.frAnBinh, "blocked", w.binh, "2030-03-11T00:00:00Z"))
	add("decide_friend_request: a block by the other party", "", base, decide(w.frAnBinh, "blocked", w.an, "2030-03-11T00:00:00Z"))
	add("decide_friend_request: a missing request answers nothing, whatever the state", "", base,
		decide(w.missingRequest, "maybe", w.an, "2030-03-11T00:00:00Z"), decide(w.missingRequest, "accepted", w.an, "2030-03-11T00:00:00Z"))
	add("decide_friend_request: pending is not a decision", "NOT_A_DECISION", base, decide(w.frChiAn, "pending", w.an, "2030-03-11T00:00:00Z"))
	add("decide_friend_request: a state that does not exist", "ValueError", base, decide(w.frChiAn, "Accepted", w.an, "2030-03-11T00:00:00Z"))
	add("decide_friend_request: blocking twice", "ALREADY_BLOCKED", base, decide(w.frPhuongBinh, "blocked", w.binh, "2030-03-11T00:00:00Z"))
	add("decide_friend_request: declining a friendship", "NOT_PENDING", base, decide(w.frAnBinh, "declined", w.binh, "2030-03-11T00:00:00Z"))
	add("decide_friend_request: declining a block is not unblocking", "NOT_PENDING", base, decide(w.frPhuongAn, "declined", w.an, "2030-03-11T00:00:00Z"))
	add("decide_friend_request: the requester accepting", "ONLY_ADDRESSEE_MAY_ANSWER", base, decide(w.frChiAn, "accepted", w.chi, "2030-03-11T00:00:00Z"))
	add("decide_friend_request: a stranger blocking", "NOT_A_PARTY", base, decide(w.frChiAn, "blocked", w.giang, "2030-03-11T00:00:00Z"))
	add("decide_friend_request: a block that collides with a newer live edge", "FRIEND_EDGE_EXISTS", base,
		decide(w.frAnDungDeclined, "blocked", w.an, "2030-03-11T00:00:00Z"))

	identity := func(provider, subject string) oracleCall {
		return read("get_account_identity", args("provider", provider, "subject", subject))
	}
	add("get_account_identity: bound, unbound, the wrong provider", "", base,
		identity("phone", w.subjectAn), identity("google", w.subjectBinh), identity("phone", w.subjectBinh),
		identity("phone", ""), identity("google", w.subjectAn), identity("phone", w.subjectEm))

	// --- stories ------------------------------------------------------------
	image := func(person, id string) oracleCall {
		return read("get_person_image", args("person_id", person, "image_id", id))
	}
	add("get_person_image: only one's own personal images", "", base,
		image(w.binh, w.imgBinhPersonal), image(w.binh, w.imgBinhAvatar), image(w.an, w.imgBinhPersonal),
		image(w.an, w.imgG1), image(w.an, w.imgAnPersonal), image(w.binh, w.missingImage))
	story := func(author string, caption any, audience, now, expires string) oracleCall {
		return write("create_story", args("author_id", author, "image_url", "/people/"+author+"/photos/"+w.imgAnPersonal,
			"caption", caption, "audience", audience, "now", now, "expires_at", expires), stories)
	}
	add("create_story: without a caption, by an unnamed author", "", base,
		story(w.chi, nil, "friends", "2030-04-10T12:00:00.000001Z", "2030-04-11T12:00:00.000001Z"))
	add("create_story: a caption at the length limit, a Vietnam offset", "", base,
		story(w.an, strings.Repeat("ồ", 200), "friends", "2030-04-10T19:00:00.5+07:00", "2030-04-11T19:00:00.5+07:00"))
	add("create_story: a caption over the length limit", "IntegrityError", base,
		story(w.an, strings.Repeat("ồ", 201), "friends", socialNow, "2030-04-11T12:00:00Z"))
	add("create_story: a deadline that is not after creation", "IntegrityError", base,
		story(w.an, nil, "friends", socialNow, socialNow))
	add("create_story: an audience the check refuses", "IntegrityError", base,
		story(w.an, nil, "public", socialNow, "2030-04-11T12:00:00Z"))
	add("create_story: an audience longer than varchar(8)", "DataError", base,
		story(w.an, nil, "everybody", socialNow, "2030-04-11T12:00:00Z"))
	add("create_story: an author with no people row", "IntegrityError", base,
		story(w.missingPerson, nil, "friends", socialNow, "2030-04-11T12:00:00Z"))
	getStory := func(id string) oracleCall { return read("get_story", args("story_id", id)) }
	add("get_story: live, expired, unnamed author, missing", "", base,
		getStory(w.stBinhTieB), getStory(w.stBinhExpired), getStory(w.stChi), getStory(w.missingStory))
	rail := func(reader, now string) oracleCall {
		return read("list_live_stories_for", args("reader_id", reader, "now", now))
	}
	add("list_live_stories_for: every reader, deadlines at the boundary", "", base,
		rail(w.an, socialNow), rail(w.an, "2030-04-10T11:59:59.999999Z"), rail(w.an, "2030-04-10T19:00:00+07:00"),
		rail(w.an, "2030-04-10T12:00:00.000001Z"), rail(w.binh, socialNow), rail(w.phuong, socialNow),
		rail(w.giang, socialNow), rail(w.khoa, socialNow), rail(w.chi, socialNow), rail(w.em, socialNow),
		rail(w.missingPerson, socialNow), rail(w.an, "2030-04-12T00:00:00Z"))
	add("list_live_stories_for: under a Vietnam session TimeZone", "", vietnam, rail(w.an, socialNow), rail(w.giang, socialNow))
	seen := func(id, viewer, now string) oracleCall {
		return write("mark_story_seen", args("story_id", id, "viewer_id", viewer, "now", now), storyViews)
	}
	add("mark_story_seen: first look, second look, a look from the seed", "", base,
		seen(w.stBinhTieB, w.an, "2030-04-10T12:00:00.25Z"), seen(w.stBinhTieB, w.an, "2030-04-10T13:00:00Z"),
		seen(w.stBinhOld, w.an, socialNow), seen(w.stBinhExpired, w.khoa, "2030-04-10T19:00:00+07:00"))
	add("mark_story_seen: a missing story", "IntegrityError", base, seen(w.missingStory, w.an, socialNow))
	add("mark_story_seen: a viewer with no people row", "IntegrityError", base, seen(w.stBinhOld, w.missingPerson, socialNow))
	deleteStory := func(id string) oracleCall { return write("delete_story", args("story_id", id), stories, storyViews) }
	add("delete_story: a story with views takes them along", "", base, deleteStory(w.stBinhOld))
	add("delete_story: an expired story", "", base, deleteStory(w.stAnExpired))
	add("delete_story: a missing story issues no DELETE", "", base, deleteStory(w.missingStory))

	// --- posts --------------------------------------------------------------
	post := func(author, audience string, context any, body string, imageURL any, now string) oracleCall {
		return write("create_post", args("author_id", author, "audience", audience, "context_id", context, "body", body,
			"image_url", imageURL, "now", now), posts)
	}
	add("create_post: public, then friends", "", base,
		post(w.an, "public", nil, "Xin chào (dữ liệu mẫu)", nil, "2030-05-10T00:00:00.000001Z"),
		post(w.an, "friends", nil, "Chỉ bạn bè 🙂 (dữ liệu mẫu)", nil, "2030-05-10T08:00:00+07:00"))
	add("create_post: a group post with a photograph", "", base,
		post(w.binh, "group", w.g1, "Ảnh nhóm (dữ liệu mẫu)", "/contexts/"+w.g1+"/photos/"+w.imgG1, "2030-05-10T00:00:02Z"))
	add("create_post: only_me by an erased account", "", base, post(w.em, "only_me", nil, "Nháp ' \" \\ (dữ liệu mẫu)", "", "2030-05-10T00:00:03Z"))
	add("create_post: an unknown audience", "ValueError", base, post(w.an, "everyone", nil, "x", nil, socialNow))
	add("create_post: a group audience without a group", "IntegrityError", base, post(w.an, "group", nil, "x", nil, socialNow))
	add("create_post: a public post naming a group", "IntegrityError", base, post(w.an, "public", w.g1, "x", nil, socialNow))
	add("create_post: an empty body", "IntegrityError", base, post(w.an, "public", nil, "", nil, socialNow))
	add("create_post: a group that does not exist", "IntegrityError", base, post(w.an, "group", w.missingContext, "x", nil, socialNow))
	add("create_post: an author with no people row", "IntegrityError", base, post(w.missingPerson, "public", nil, "x", nil, socialNow))
	getPost := func(id string) oracleCall { return read("get_post", args("post_id", id)) }
	add("get_post: every audience and a missing id", "", base,
		getPost(w.pBinhOnlyMe), getPost(w.pBinhGroup), getPost(w.pPhuongPublic), getPost(w.pChiPublic), getPost(w.missingPost))
	feed := func(reader string, limit int) oracleCall {
		return read("list_posts_visible_to", args("reader_id", reader, "limit", limit))
	}
	add("list_posts_visible_to: every reader", "", base,
		feed(w.an, 50), feed(w.binh, 50), feed(w.phuong, 50), feed(w.dung, 50), feed(w.hoa, 50), feed(w.giang, 50),
		feed(w.khoa, 50), feed(w.chi, 50), feed(w.em, 50), feed(w.missingPerson, 50))
	add("list_posts_visible_to: page edges across a created_at tie", "", base,
		feed(w.binh, 0), feed(w.binh, 1), feed(w.binh, 5), feed(w.binh, 6), feed(w.binh, 7), feed(w.binh, 8), feed(w.binh, 9))
	add("list_posts_visible_to: a negative limit", "DataError", base, feed(w.an, -1))
	wall := func(person, reader string, limit int) oracleCall {
		return read("list_person_posts_visible_to", args("person_id", person, "reader_id", reader, "limit", limit))
	}
	add("list_person_posts_visible_to: walls by reader", "", base,
		wall(w.binh, w.an, 50), wall(w.binh, w.binh, 50), wall(w.binh, w.phuong, 50), wall(w.phuong, w.an, 50),
		wall(w.phuong, w.binh, 50), wall(w.phuong, w.giang, 50), wall(w.an, w.an, 50), wall(w.an, w.binh, 50),
		wall(w.dung, w.an, 50), wall(w.dung, w.dung, 50), wall(w.hoa, w.dung, 50), wall(w.em, w.an, 50),
		wall(w.missingPerson, w.an, 50), wall(w.binh, w.binh, 2), wall(w.binh, w.binh, 3), wall(w.binh, w.binh, 0))
	add("list_person_posts_visible_to: a negative limit", "DataError", base, wall(w.binh, w.an, -5))
	counts := func(viewer string, ids ...string) oracleCall {
		return read("post_social_counts", args("post_ids", list(ids...), "viewer_id", viewer))
	}
	add("post_social_counts: kinds, comments, the viewer's own, duplicates, none", "", base,
		counts(w.an, w.pBinhPublic, w.pBinhFriends, w.pPhuongGroup, w.pAnPublic), counts(w.binh, w.pBinhPublic),
		counts(w.an), counts(w.giang, w.pBinhPublic, w.pBinhPublic), counts(w.missingPerson, w.pAnPublic),
		counts(w.an, w.missingPost), counts(w.khoa, w.pPhuongGroup, w.pBinhFriends, w.pBinhPublic))
	react := func(post, person, kind, now string) oracleCall {
		return write("add_post_reaction", args("post_id", post, "person_id", person, "kind", kind, "now", now), postReactions)
	}
	unreact := func(post, person, kind string) oracleCall {
		return write("remove_post_reaction", args("post_id", post, "person_id", person, "kind", kind), postReactions)
	}
	add("add_post_reaction: a new kind", "", base, react(w.pBinhPublic, w.binh, "fire", "2030-05-10T00:00:00.000001Z"))
	add("add_post_reaction: the same kind twice answers the first row, and the transaction goes on", "", base,
		react(w.pBinhPublic, w.an, "heart", "2030-05-10T00:00:00Z"), unreact(w.pBinhPublic, w.an, "fire"))
	add("add_post_reaction: a kind the check refuses", "IntegrityError", base, react(w.pBinhPublic, w.an, "love", socialNow))
	add("add_post_reaction: a kind longer than varchar(16)", "DataError", base, react(w.pBinhPublic, w.an, "heart-heart-heart", socialNow))
	add("add_post_reaction: a missing post", "IntegrityError", base, react(w.missingPost, w.an, "heart", socialNow))
	add("add_post_reaction: a person with no people row", "IntegrityError", base, react(w.pBinhPublic, w.missingPerson, "heart", socialNow))
	add("remove_post_reaction: one's own row, twice, another kind, another's row", "", base,
		unreact(w.pBinhPublic, w.an, "heart"), unreact(w.pBinhPublic, w.an, "heart"), unreact(w.pBinhPublic, w.an, "wow"),
		unreact(w.pBinhFriends, w.binh, "wow"), unreact(w.missingPost, w.an, "heart"))
	comment := func(post, author, body, now string) oracleCall {
		return write("create_post_comment", args("post_id", post, "author_id", author, "body", body, "now", now), postComments)
	}
	add("create_post_comment: by an unnamed author, then a named one", "", base,
		comment(w.pBinhPublic, w.chi, "Không tên (dữ liệu mẫu)", "2030-05-10T00:00:00.000001Z"),
		comment(w.pBinhPublic, w.an, "Có tên 🙂 (dữ liệu mẫu)", "2030-05-10T08:00:00+07:00"))
	add("create_post_comment: an empty body", "IntegrityError", base, comment(w.pBinhPublic, w.an, "", socialNow))
	add("create_post_comment: a missing post", "IntegrityError", base, comment(w.missingPost, w.an, "x", socialNow))
	add("create_post_comment: an author with no people row", "IntegrityError", base, comment(w.pBinhPublic, w.missingPerson, "x", socialNow))
	getComment := func(id string) oracleCall { return read("get_post_comment", args("comment_id", id)) }
	add("get_post_comment: named, unnamed, erased, missing", "", base,
		getComment(w.c1), getComment(w.c3), getComment(w.c4), getComment(w.missingComment))
	deleteComment := func(id string) oracleCall { return write("delete_post_comment", args("comment_id", id), postComments) }
	add("delete_post_comment: a comment", "", base, deleteComment(w.c2))
	add("delete_post_comment: a missing comment", "", base, deleteComment(w.missingComment))
	thread := func(post string, limit int, after ...any) oracleCall {
		a := args("post_id", post, "limit", limit)
		if len(after) == 1 {
			a["after"] = after[0]
		}
		return read("list_post_comments", a)
	}
	add("list_post_comments: pages and cursors across a created_at tie", "", base,
		thread(w.pBinhPublic, 50), thread(w.pBinhPublic, 2), thread(w.pBinhPublic, 3),
		thread(w.pBinhPublic, 2, []any{w.commentTie, w.c1}), thread(w.pBinhPublic, 2, []any{w.commentTie, w.c3}),
		thread(w.pBinhPublic, 2, []any{"2030-05-01T11:00:00Z", w.c4}),
		thread(w.pBinhPublic, 50, []any{"2030-05-01T17:00:00+07:00", w.c1}),
		thread(w.pBinhPublic, 50, []any{"2030-05-01T09:00:00Z", w.missingComment}),
		thread(w.pBinhPublic, 0), thread(w.pBinhPublic, 50, nil), thread(w.pBinhFriends, 50), thread(w.missingPost, 50))
	add("list_post_comments: a negative limit", "DataError", base, thread(w.pBinhPublic, -1))

	// --- votes --------------------------------------------------------------
	add("get_outing: stops and itinerary days, no stops, missing", "", base,
		read("get_outing", args("outing_id", w.oG1)), read("get_outing", args("outing_id", w.oBare)),
		read("get_outing", args("outing_id", w.missingOuting)))
	getVote := func(id string) oracleCall { return read("get_vote", args("vote_id", id)) }
	add("get_vote: open with ballots tied on created_at, closed, another group, missing", "", base,
		getVote(w.vOpen), getVote(w.vClosed), getVote(w.vG2), getVote(w.missingVote))
	votesOf := func(id string) oracleCall { return read("list_votes", args("context_id", id)) }
	add("list_votes: ties on created_at, an empty group, a missing group", "", base,
		votesOf(w.g1), votesOf(w.g2), votesOf(w.g3), votesOf(w.missingContext))
	add("list_votes: under a Vietnam session TimeZone", "", vietnam, votesOf(w.g1))
	option := func(label string, place any) map[string]any {
		return map[string]any{"label": label, "place_name": place}
	}
	newVote := func(context string, outing any, question, now string, options ...map[string]any) oracleCall {
		items := []any{}
		for _, o := range options {
			items = append(items, o)
		}
		return write("create_vote", args("context_id", context, "outing_id", outing, "created_by_id", w.an,
			"question", question, "options", items, "now", now), votes, voteOptions, voteBallots)
	}
	add("create_vote: two options, one with a place, linked to an outing", "", base,
		newVote(w.g1, w.oG1, "Ăn gì? (dữ liệu mẫu)", "2030-06-05T00:00:00.000001Z",
			option("Phở (dữ liệu mẫu)", nil), option("Bún chả (dữ liệu mẫu)", "Hàng Mành (dữ liệu mẫu)")))
	add("create_vote: no options at all", "", base, newVote(w.g2, nil, "Trống (dữ liệu mẫu)", "2030-06-05T07:00:00+07:00"))
	add("create_vote: one option with an empty place", "", base,
		newVote(w.g3, nil, "Một lựa chọn (dữ liệu mẫu)", "2030-06-05T00:00:01Z", option("Duy nhất (dữ liệu mẫu)", "")))
	many := []map[string]any{}
	for i := 0; i < 1001; i++ {
		many = append(many, option(fmt.Sprintf("Lựa chọn %04d (dữ liệu mẫu)", 1001-i), nil))
	}
	manyVote := newVote(w.g1, nil, "Nhiều lựa chọn (dữ liệu mẫu)", "2030-06-05T00:00:02Z", many...)
	manyVote.Probes = []string{probeWrites}
	add("create_vote: 1001 options", "", base, manyVote)
	add("create_vote: an empty question", "IntegrityError", base,
		newVote(w.g1, nil, "", socialNow, option("A (dữ liệu mẫu)", nil)))
	add("create_vote: an empty label in the second option", "IntegrityError", base,
		newVote(w.g1, nil, "Nhãn trống (dữ liệu mẫu)", socialNow, option("A (dữ liệu mẫu)", nil), option("", nil)))
	add("create_vote: a group that does not exist", "IntegrityError", base,
		newVote(w.missingContext, nil, "Không nhóm (dữ liệu mẫu)", socialNow, option("A (dữ liệu mẫu)", nil)))
	add("create_vote: an outing that does not exist", "IntegrityError", base,
		newVote(w.g1, w.missingOuting, "Không chuyến (dữ liệu mẫu)", socialNow, option("A (dữ liệu mẫu)", nil)))
	cast := func(vote, option, voter, now string) oracleCall {
		return write("upsert_ballot", args("vote_id", vote, "option_id", option, "voter_id", voter, "now", now), voteBallots)
	}
	add("upsert_ballot: a first ballot", "", base, cast(w.vOpen, w.optOpenFar, w.giang, "2030-06-05T00:00:00.000001Z"))
	add("upsert_ballot: changing one's mind", "", base, cast(w.vOpen, w.optOpenFar, w.binh, "2030-06-05T07:00:00+07:00"))
	add("upsert_ballot: the same option later", "", base, cast(w.vOpen, w.optOpenCafe, w.binh, "2030-06-05T00:00:00Z"))
	add("upsert_ballot: the same option at the stored instant", "", base, cast(w.vOpen, w.optOpenCafe, w.binh, w.ballotOpenBinhAt))
	add("upsert_ballot: another option at the stored instant", "", base, cast(w.vOpen, w.optOpenHome, w.binh, w.ballotOpenBinhAt))
	add("upsert_ballot: a missing vote", "VOTE_NOT_FOUND", base, cast(w.missingVote, w.optOpenFar, w.binh, socialNow))
	add("upsert_ballot: a closed vote", "VOTE_CLOSED", base, cast(w.vClosed, w.optClosedB, w.binh, socialNow))
	add("upsert_ballot: an option of another vote", "UNKNOWN_OPTION", base, cast(w.vOpen, w.optClosedA, w.binh, socialNow))
	add("upsert_ballot: a missing option", "UNKNOWN_OPTION", base, cast(w.vOpen, w.missingOption, w.binh, socialNow))
	add("upsert_ballot: a voter with no people row", "IntegrityError", base, cast(w.vOpen, w.optOpenFar, w.missingPerson, socialNow))
	closeVote := func(vote, by, now string) oracleCall {
		return write("close_vote", args("vote_id", vote, "closed_by_id", by, "now", now), votes, voteBallots)
	}
	add("close_vote: an open vote with ballots", "", base, closeVote(w.vOpen, w.an, "2030-06-05T00:00:00.000001Z"))
	add("close_vote: a ballot, the close, then a ballot too late", "VOTE_CLOSED", base,
		cast(w.vTie, w.optTie, w.an, "2030-06-05T00:00:00Z"), closeVote(w.vTie, w.giang, "2030-06-05T07:00:01+07:00"),
		cast(w.vTie, w.optTie, w.hoa, "2030-06-05T00:00:02Z"))
	add("close_vote: already closed", "VOTE_ALREADY_CLOSED", base, closeVote(w.vClosed, w.an, socialNow))
	add("close_vote: a missing vote", "VOTE_NOT_FOUND", base, closeVote(w.missingVote, w.an, socialNow))
	add("close_vote: a closer with no people row", "IntegrityError", base, closeVote(w.vG2, w.missingPerson, socialNow))

	// --- the reads a service method makes, in its order ---------------------
	add("the reads read_post makes for a friend's post", "", base,
		getPost(w.pBinhFriends), edge(w.an, w.binh), read("is_member", args("context_id", w.g1, "person_id", w.an)),
		edge(w.an, w.binh), counts(w.an, w.pBinhFriends), read("get_person", args("person_id", w.binh)))
	add("the reads list_stories makes", "", base,
		rail(w.an, socialNow), edge(w.an, w.an), edge(w.an, w.binh), edge(w.an, w.binh), edge(w.an, w.em), edge(w.an, w.em))
	add("the reads send_friend_request makes before it writes", "", base,
		read("get_person", args("person_id", w.giang)), edge(w.hoa, w.giang), open(w.hoa, w.giang, "2030-03-10T00:00:00Z"))

	spec := oracleSpec{Clock: []string{}}
	for _, c := range cases {
		spec.Cases = append(spec.Cases, c.oracleCase)
	}
	return cases, spec
}

// socialMethods is every method this port covers; the corpus must reach each
// with at least one normal return.
var socialMethods = []string{
	"get_friend_edge", "get_friend_request", "open_friend_request", "decide_friend_request", "list_friend_requests",
	"list_friends", "get_account_identity", "get_person_image", "create_story", "get_story", "list_live_stories_for",
	"mark_story_seen", "delete_story", "create_post", "get_post", "list_posts_visible_to", "list_person_posts_visible_to",
	"post_social_counts", "add_post_reaction", "remove_post_reaction", "create_post_comment", "get_post_comment",
	"delete_post_comment", "list_post_comments", "get_outing", "create_vote", "get_vote", "list_votes", "upsert_ballot",
	"close_vote",
}

func TestSocialRepositoryOracle(t *testing.T) {
	image := os.Getenv("CORE_PYTHON_IMAGE")
	if image == "" {
		t.Skip("CORE_PYTHON_IMAGE not set; see the comment at the top of social_oracle_postgres_test.go")
	}
	scripts, err := filepath.Abs("../../../../scripts")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(scripts, "render_social_repo_oracle.py")); err != nil {
		t.Fatal(err)
	}
	// Two private schemas in the one database, migrated alike: Python runs
	// every case in one, Go in the other. post_social_counts has GROUP BY
	// without ORDER BY, so its dicts keep the order the plan emits (sorted for
	// GroupAggregate, hash order for HashAggregate), and the plan of a table
	// that was never analyzed scales with the table's current size on disk,
	// which rolled-back fixture rows keep growing. In one shared schema Go's
	// case N would plan against a heap that Python's later cases had already
	// grown; in two identical schemas each side's case N sees exactly what its
	// own cases before it left. One database still, so collation, server
	// settings and TimeZone cannot differ; autovacuum is off in both so no
	// ANALYZE lands in the middle of a run.
	pyPool, url := migratedOracleSchema(t, image, "social_oracle_py_")
	pool, _ := migratedOracleSchema(t, image, "social_oracle_go_")
	for _, p := range []*pgxpool.Pool{pyPool, pool} {
		if _, err := p.Exec(bg, `DO $$ DECLARE t record; BEGIN
			FOR t IN SELECT tablename FROM pg_tables WHERE schemaname = current_schema() LOOP
				EXECUTE format('ALTER TABLE %I SET (autovacuum_enabled = false)', t.tablename);
			END LOOP; END $$`); err != nil {
			t.Fatal(err)
		}
	}

	cases, built := socialOracleCases()
	payload, err := json.Marshal(built)
	if err != nil {
		t.Fatal(err)
	}
	var spec oracleSpec
	if err := json.Unmarshal(payload, &spec); err != nil {
		t.Fatal(err)
	}
	driver := osexec.Command("docker", "run", "--rm", "-i", "--network", "host",
		"-e", "ORACLE_DATABASE_URL="+url, "-v", scripts+":/oracle:ro",
		"--entrypoint", "python", image, "/oracle/render_social_repo_oracle.py")
	driver.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	driver.Stdout, driver.Stderr = &stdout, &stderr
	if err := driver.Run(); err != nil {
		t.Fatalf("render_social_repo_oracle.py: %v\n%s", err, tail(stderr.Bytes()))
	}
	var python pythonRun
	if err := json.Unmarshal(stdout.Bytes(), &python); err != nil {
		t.Fatalf("python output: %v", err)
	}
	if len(python.Cases) != len(spec.Cases) {
		t.Fatalf("python answered %d cases of %d", len(python.Cases), len(spec.Cases))
	}

	tally := &repoTally{}
	returned := map[string]int{}
	for i, c := range spec.Cases {
		tally.cases++
		if python.Cases[i].Name != c.Name {
			t.Fatalf("case %d is %q in python", i, python.Cases[i].Name)
		}
		pySteps := []any{}
		for j, s := range python.Cases[i].Steps {
			statements := []any{}
			for _, entry := range s.Statements {
				for k := 0; k < int(entry[1].(float64)); k++ {
					statements = append(statements, normalizeSQL(entry[0].(string)))
				}
			}
			warnings := s.Warnings
			if warnings == nil {
				warnings = []any{}
			}
			if s.Error == nil {
				returned[c.Steps[j].Call]++
			}
			pySteps = append(pySteps, generic(t, map[string]any{"result": s.Result, "error": s.Error,
				"warnings": warnings, "statements": statements, "probes": s.Probes}))
		}

		// The fixture must reach what the case is named for.
		pyCase := python.Cases[i]
		end := ""
		if len(pyCase.Steps) > 0 {
			if e, ok := pyCase.Steps[len(pyCase.Steps)-1].Error.(map[string]any); ok {
				end, _ = e["type"].(string)
				if code, ok := e["code"].(string); ok {
					end = code
				}
			}
		}
		if len(pyCase.Steps) != len(c.Steps) && end == "" {
			t.Errorf("case %q: python ran %d of %d steps", c.Name, len(pyCase.Steps), len(c.Steps))
		}
		if end != cases[i].wantEnd {
			t.Errorf("case %q: python ended in %q, the case is written for %q", c.Name, end, cases[i].wantEnd)
		}
		if cases[i].wantEnd != "" && len(pyCase.Steps) != len(c.Steps) {
			t.Errorf("case %q: python refused at step %d of %d", c.Name, len(pyCase.Steps), len(c.Steps))
		}

		t.Run(c.Name, func(t *testing.T) {
			compareCase(t, tally, c, pySteps, runSocialGoCase(t, pool, c))
		})
	}
	for _, method := range socialMethods {
		if returned[method] == 0 {
			t.Errorf("no case reaches a normal return of %s", method)
		}
	}
	t.Logf("social repo oracle: %d cases, %d steps (%d results, %d refusals), %d statements, %d probe rows of which "+
		"%d table rows, %d generated ids bound, %d mismatches", tally.cases, tally.steps, tally.results, tally.errors,
		tally.statements, tally.probeRows, tally.tableRows, tally.generated, tally.mismatches)
}
