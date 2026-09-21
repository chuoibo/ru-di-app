//go:build postgres

package repo

// The call sequences of the eleven W7 routes (routes/outings.py), written
// against the Go repository, for the `route.*` steps of
// outings_repo_oracle_postgres_test.go. The Python side of such a step runs
// the real ApiService method; this side runs the repository calls that method
// makes, in its order, with the service's decisions (the permission table,
// the itinerary draft's validations, the two invitation doors) spelled out in
// the fewest lines that decide the same branch. This is test code, not the
// service port: it exists so the oracle proves, statement by statement, that
// the Go repository called in this order is what the Python route does.
//
// `route.authorize_outing_itinerary` is the PUT's replay branch: when the
// idempotency layer already holds the answer, the route still proves the
// caller may edit this trip before handing the stored body back.

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"mobile/services/core/internal/domain/permissions"
)

type outingRoute struct {
	repo  Repository
	actor string
	now   time.Time
	a     map[string]any
}

func (p *outingRoute) text(key string) string { return argString(p.a, key) }

func (p *outingRoute) body() map[string]any {
	body, _ := p.a["body"].(map[string]any)
	return body
}

// require is `_require_permission(action, actor, facts)` for a member.
func (p *outingRoute) require(action string, facts map[string]bool) error {
	var proven []string
	for name, ok := range facts {
		if ok {
			proven = append(proven, name)
		}
	}
	sort.Strings(proven)
	f, err := permissions.NewAuthorizationFacts(p.actor, []string{"member"}, nil, proven, "api_service")
	if err != nil {
		return err
	}
	if _, allowed, err := permissions.DenialReason(action, f); err != nil {
		return err
	} else if !allowed {
		return refuse(403, "permission_denied")
	}
	return nil
}

func (p *outingRoute) isMember(contextID string) (bool, error) {
	return p.repo.IsMember(bg, contextID, p.actor)
}

// outingOr404 is the head of six routes: the trip, then this reader's
// membership of the group that owns it.
func (p *outingRoute) outingOr404(outingID, action, notFound string) (*Outing, error) {
	outing, err := p.repo.GetOuting(bg, outingID)
	if err != nil {
		return nil, err
	}
	if outing == nil {
		return nil, refuse(404, notFound)
	}
	member, err := p.isMember(outing.ContextID)
	if err != nil {
		return nil, err
	}
	return outing, p.require(action, map[string]bool{"is_group_member": member})
}

// --- the trips of a group -------------------------------------------------

func (p *outingRoute) createOuting() error {
	contextID := p.text("context_id")
	member, err := p.isMember(contextID)
	if err != nil {
		return err
	}
	if err := p.require("create_outing", map[string]bool{"is_group_member": member}); err != nil {
		return err
	}
	body := p.body()
	_, err = p.repo.CreateOuting(bg, OutingInput{ContextID: contextID, CreatedByID: p.actor,
		Title: strings.TrimSpace(body["title"].(string)), StartsOn: argDay(body["starts_on"].(string)),
		EndsOn: argDay(body["ends_on"].(string)), Headcount: argNumber(body["headcount"]),
		BudgetPerPersonVND: argNumber(body["budget_per_person_vnd"]), Now: p.now})
	return err
}

func (p *outingRoute) listContextOutings() error {
	contextID := p.text("context_id")
	member, err := p.isMember(contextID)
	if err != nil {
		return err
	}
	if err := p.require("view_outings", map[string]bool{"is_group_member": member}); err != nil {
		return err
	}
	_, err = p.repo.ListOutings(bg, contextID)
	return err
}

// --- the version 1 timeline -----------------------------------------------

func (p *outingRoute) replaceTimeline() error {
	if _, err := p.outingOr404(p.text("outing_id"), "edit_outing_timeline", "outing_not_found"); err != nil {
		return err
	}
	body := p.body()
	lines, _ := body["stops"].([]any)
	// Every catalogue key the body names is checked before anything is
	// written; a key the catalogue does not know is a client bug.
	for _, line := range lines {
		stop := line.(map[string]any)
		placeID := argText(stop["place_id"])
		if placeID == nil {
			continue
		}
		place, err := p.repo.GetPlace(bg, *placeID)
		if err != nil {
			return err
		}
		if place == nil {
			return refuse(422, "stop_place_unknown")
		}
	}
	stops := []TimelineStop{}
	for _, line := range lines {
		stop := line.(map[string]any)
		stops = append(stops, TimelineStop{MinuteOfDay: minuteOfDay(stop["at"].(string)),
			Label: stop["label"].(string), PlaceName: argText(stop["place_name"]),
			PlaceID: argText(stop["place_id"])})
	}
	var expected *int64
	if raw, ok := body["expected_revision"]; ok && raw != nil {
		revision := argNumber(raw)
		expected = &revision
	}
	_, err := p.repo.ReplaceOutingStops(bg, p.text("outing_id"), stops, expected)
	return itineraryConflict(err)
}

// --- the version 2 itinerary ----------------------------------------------

// itineraryDraft is `_itinerary_draft`: authorize, refuse a stale revision,
// check the days and the anchors without a statement, read the arrivals, and
// resolve every catalogue key the body names -- in that order, because the
// order is what the statement log compares.
func (p *outingRoute) itineraryDraft() (*Outing, []ItineraryStop, []json.RawMessage, error) {
	outing, err := p.outingOr404(p.text("outing_id"), "edit_outing_timeline", "outing_not_found")
	if err != nil {
		return nil, nil, nil, err
	}
	body := p.body()
	if argNumber(body["expected_revision"]) != outing.TimelineRevision {
		return nil, nil, nil, refuse(409, "timeline_conflict")
	}
	known := map[string]bool{}
	for _, stop := range outing.Stops {
		known[stop.ID] = true
	}
	lines, _ := body["stops"].([]any)
	days, _ := body["days"].([]any)
	dayKeys := map[string]bool{}
	for _, item := range days {
		dayKeys[item.(map[string]any)["day"].(string)] = true
	}
	for _, item := range days {
		day := item.(map[string]any)
		key := day["day"].(string)
		if key < isoDay(outing.StartsOn) || key > isoDay(outing.EndsOn) {
			return nil, nil, nil, refuse(422, "itinerary_day_invalid")
		}
		ids := map[string]bool{}
		for _, line := range lines {
			stop := line.(map[string]any)
			if text := argText(stop["day"]); text != nil && *text == key {
				ids[stop["id"].(string)] = true
			}
		}
		for _, anchor := range []any{day["start_stop_id"], day["end_stop_id"]} {
			if text := argText(anchor); text != nil && !ids[*text] {
				return nil, nil, nil, refuse(422, "itinerary_anchor_invalid")
			}
		}
	}
	if _, err := p.repo.ListOutingCheckins(bg, outing.ID); err != nil {
		return nil, nil, nil, err
	}
	stops := []ItineraryStop{}
	for _, line := range lines {
		stop := line.(map[string]any)
		id := stop["id"].(string)
		if !isDraftStopID(id) && !known[id] {
			return nil, nil, nil, refuse(422, "stop_not_found")
		}
		day := argText(stop["day"])
		if day != nil && (*day < isoDay(outing.StartsOn) || *day > isoDay(outing.EndsOn) || !dayKeys[*day]) {
			return nil, nil, nil, refuse(422, "itinerary_day_invalid")
		}
		placeID := argText(stop["place_id"])
		if placeID != nil {
			place, err := p.repo.GetPlace(bg, *placeID)
			if err != nil {
				return nil, nil, nil, err
			}
			if place == nil {
				return nil, nil, nil, refuse(422, "stop_place_unknown")
			}
		}
		next := ItineraryStop{ID: id, MinuteOfDay: minuteOfDay(stop["at"].(string)),
			Label: stop["label"].(string), PlaceName: argText(stop["place_name"]), PlaceID: placeID,
			Day: optionalDay(day), DurationMinutes: optionalNumber(stop["duration_minutes"]),
			TimeLocked: true}
		if locked, ok := stop["time_locked"].(bool); ok {
			next.TimeLocked = locked
		}
		if point, ok := stop["meeting_point"].(map[string]any); ok && point != nil {
			lat, lng := argFloat(point["lat"]), argFloat(point["lng"])
			label := point["label"].(string)
			next.MeetingLat, next.MeetingLng, next.MeetingLabel = &lat, &lng, &label
		}
		stops = append(stops, next)
	}
	rendered := []json.RawMessage{}
	for _, item := range days {
		day := item.(map[string]any)
		rendered = append(rendered, json.RawMessage(outingDayJSON(day["day"].(string),
			day["transport_mode"].(string), day["start_at"].(string), day["start_stop_id"],
			day["end_stop_id"], day["return_to_start"] == true)))
	}
	return outing, stops, rendered, nil
}

func (p *outingRoute) previewItinerary() error {
	outing, _, _, err := p.itineraryDraft()
	if err != nil {
		return err
	}
	day := p.body()["day"].(string)
	if day < isoDay(outing.StartsOn) || day > isoDay(outing.EndsOn) {
		return refuse(422, "itinerary_day_invalid")
	}
	// preview_itinerary reads nothing: with no routing provider configured it
	// answers «unavailable» from the draft it was handed.
	return nil
}

func (p *outingRoute) replaceItinerary() error {
	_, stops, days, err := p.itineraryDraft()
	if err != nil {
		return err
	}
	_, err = p.repo.ReplaceOutingItinerary(bg, p.text("outing_id"), stops, days,
		argNumber(p.body()["expected_revision"]))
	return itineraryConflict(err)
}

// itineraryConflict is `_itinerary_conflict`: the four codes the two saves
// raise, and the HTTP answer each becomes.
func itineraryConflict(err error) error {
	var conflict *Conflict
	if !errors.As(err, &conflict) {
		return err
	}
	switch conflict.Code {
	case "TIMELINE_REVISION_CONFLICT":
		return refuse(409, "timeline_conflict")
	case "ITINERARY_UPGRADE_REQUIRED":
		return refuse(409, "itinerary_upgrade_required")
	case "STOP_NOT_FOUND":
		return refuse(422, "stop_not_found")
	case "OUTING_NOT_FOUND":
		return refuse(404, "outing_not_found")
	}
	return err
}

// --- arrivals --------------------------------------------------------------

func (p *outingRoute) checkIn() error {
	stopID := p.text("stop_id")
	_, outing, err := p.repo.GetOutingStop(bg, stopID)
	if err != nil {
		return err
	}
	if outing == nil {
		return refuse(404, "stop_not_found")
	}
	member, err := p.isMember(outing.ContextID)
	if err != nil {
		return err
	}
	if err := p.require("check_in_to_stop", map[string]bool{"is_group_member": member}); err != nil {
		return err
	}
	record, err := p.repo.CreateStopCheckin(bg, stopID, p.actor, p.now)
	if err != nil {
		var conflict *Conflict
		if errors.As(err, &conflict) {
			switch conflict.Code {
			case "ALREADY_CHECKED_IN":
				return refuse(409, "already_checked_in")
			case "STOP_NOT_FOUND":
				return refuse(404, "stop_not_found")
			}
		}
		return err
	}
	_, err = p.repo.GetPerson(bg, record.PersonID)
	return err
}

func (p *outingRoute) listCheckins() error {
	outing, err := p.outingOr404(p.text("outing_id"), "view_stop_checkins", "outing_not_found")
	if err != nil {
		return err
	}
	records, err := p.repo.ListOutingCheckins(bg, outing.ID)
	if err != nil {
		return err
	}
	// `_wire_stop_checkin` looks the name up per arrival; two arrivals by one
	// person are two reads.
	for _, record := range records {
		if _, err := p.repo.GetPerson(bg, record.PersonID); err != nil {
			return err
		}
	}
	return nil
}

// --- invitations -----------------------------------------------------------

func (p *outingRoute) createInvite() error {
	outing, err := p.outingOr404(p.text("outing_id"), "invite_to_outing", "outing_not_found")
	if err != nil {
		return err
	}
	// `_require_group_kind`: the roster door is shut on a pair, and only a
	// member ever learns that, because the permission check came first.
	record, err := p.repo.GetContext(bg, outing.ContextID)
	if err != nil {
		return err
	}
	if record != nil && record.Kind == "pair" {
		return refuse(409, "not_a_group")
	}
	body := p.body()
	source := body["source"].(string)
	var invited *string
	if source != "link" {
		person := argText(body["person_id"])
		found, err := p.repo.GetPerson(bg, *person)
		if err != nil {
			return err
		}
		if found == nil {
			return refuse(409, "person_not_registered")
		}
		if source == "group" {
			members, err := p.repo.ListMembers(bg, outing.ContextID)
			if err != nil {
				return err
			}
			active := false
			for _, member := range members {
				if member.PersonID == *person && member.State == "active" {
					active = true
				}
			}
			if !active {
				return refuse(422, "participant_not_in_context")
			}
		}
		existing, err := p.repo.FindOutingInviteForPerson(bg, outing.ID, *person)
		if err != nil {
			return err
		}
		if existing != nil {
			return refuse(409, "invite_already_exists")
		}
		invited = person
	}
	_, err = p.repo.CreateOutingInvite(bg, OutingInviteInput{OutingID: outing.ID, Source: source,
		InvitedPersonID: invited, InvitedByID: p.actor, TokenDigest: digestOf(p.text("token")),
		ExpiresAt: p.now.Add(outingInviteTTL), Now: p.now})
	return err
}

// outingInviteTTL is OUTING_INVITE_TTL.
const outingInviteTTL = 7 * 24 * time.Hour

// inviteOnOuting is the head revoke and rotate share: the trip and the
// invitation, then the one 404 that tells a stranger nothing.
func (p *outingRoute) inviteOnOuting(action string) (*OutingInvite, error) {
	outing, err := p.repo.GetOuting(bg, p.text("outing_id"))
	if err != nil {
		return nil, err
	}
	invite, err := p.repo.GetOutingInvite(bg, p.text("invite_id"))
	if err != nil {
		return nil, err
	}
	if outing == nil || invite == nil || invite.OutingID != outing.ID {
		return nil, refuse(404, "invite_not_found")
	}
	member, err := p.isMember(outing.ContextID)
	if err != nil {
		return nil, err
	}
	return invite, p.require(action, map[string]bool{"is_group_member": member})
}

func (p *outingRoute) revokeInvite() error {
	invite, err := p.inviteOnOuting("revoke_outing_invite")
	if err != nil {
		return err
	}
	if invite.AcceptedAt != nil {
		return refuse(409, "invite_already_accepted")
	}
	if _, err := p.repo.RevokeOutingInvite(bg, invite.ID, p.now); err != nil {
		var conflict *Conflict
		if errors.As(err, &conflict) {
			switch conflict.Code {
			case "OUTING_INVITE_ALREADY_ACCEPTED":
				return refuse(409, "invite_already_accepted")
			case "OUTING_INVITE_NOT_FOUND":
				return refuse(404, "invite_not_found")
			}
		}
		return err
	}
	return nil
}

func (p *outingRoute) rotateInvite() error {
	invite, err := p.inviteOnOuting("invite_to_outing")
	if err != nil {
		return err
	}
	if invite.InvitedPersonID == nil {
		return refuse(409, "invite_not_named")
	}
	if _, err := p.repo.RotateOutingInviteDigest(bg, invite.ID, digestOf(p.text("token")),
		p.now.Add(outingInviteTTL), p.now); err != nil {
		var conflict *Conflict
		if errors.As(err, &conflict) {
			return refuse(404, "invite_not_found")
		}
		return err
	}
	return nil
}

func (p *outingRoute) acceptInvite() error {
	invite, err := p.repo.GetOutingInviteByDigest(bg, digestOf(p.text("token")))
	if err != nil {
		return err
	}
	if invite == nil || invite.InvitedPersonID != nil {
		return refuse(404, "invite_not_found")
	}
	if invite.AcceptedAt != nil {
		return refuse(409, "invite_already_accepted")
	}
	if invite.RevokedAt != nil || !invite.ExpiresAt.After(pythonInstant(p.now)) {
		return refuse(404, "invite_not_found")
	}
	outing, err := p.repo.GetOuting(bg, invite.OutingID)
	if err != nil {
		return err
	}
	if outing == nil {
		return refuse(404, "invite_not_found")
	}
	if _, err := p.repo.AcceptOutingInvite(bg, invite.ID, p.actor, p.now); err != nil {
		var conflict *Conflict
		if errors.As(err, &conflict) {
			switch conflict.Code {
			case "OUTING_INVITE_ALREADY_ACCEPTED":
				return refuse(409, "invite_already_accepted")
			case "OUTING_INVITE_NOT_FOUND", "OUTING_INVITE_NOT_REDEEMABLE":
				return refuse(404, "invite_not_found")
			}
		}
		return err
	}
	_, err = p.repo.EnsureInvitedMembership(bg, outing.ContextID, p.actor, invite.InvitedByID, "link", p.now)
	return err
}

// ---------------------------------------------------------------------------

func outingRouteGo(repo Repository, name string, a map[string]any) (any, error) {
	p := &outingRoute{repo: repo, actor: argString(a, "actor_id"), now: argInstant(argString(a, "now")), a: a}
	var err error
	switch name {
	case "route.create_outing":
		err = p.createOuting()
	case "route.list_context_outings":
		err = p.listContextOutings()
	case "route.replace_outing_timeline":
		err = p.replaceTimeline()
	case "route.preview_outing_itinerary":
		err = p.previewItinerary()
	case "route.replace_outing_itinerary":
		err = p.replaceItinerary()
	case "route.authorize_outing_itinerary":
		_, err = p.outingOr404(p.text("outing_id"), "edit_outing_timeline", "outing_not_found")
	case "route.check_in_to_stop":
		err = p.checkIn()
	case "route.list_outing_checkins":
		err = p.listCheckins()
	case "route.create_outing_invite":
		err = p.createInvite()
	case "route.revoke_outing_invite":
		err = p.revokeInvite()
	case "route.rotate_outing_invite":
		err = p.rotateInvite()
	case "route.accept_outing_invite":
		err = p.acceptInvite()
	default:
		panic("unknown route " + name)
	}
	return nil, err
}

// --- small conversions -----------------------------------------------------

// digestOf is `token_digest(token)`: what the server keeps of a secret.
func digestOf(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func minuteOfDay(clock string) int64 {
	var hour, minute int64
	if _, err := fmt.Sscanf(clock, "%d:%d", &hour, &minute); err != nil {
		panic(err)
	}
	return hour*60 + minute
}

func isoDay(day time.Time) string { return day.Format("2006-01-02") }

func optionalDay(text *string) *time.Time {
	if text == nil {
		return nil
	}
	day := argDay(*text)
	return &day
}

func optionalNumber(v any) *int64 {
	if v == nil {
		return nil
	}
	n := argNumber(v)
	return &n
}

func argFloat(v any) float64 {
	switch x := v.(type) {
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			panic(err)
		}
		return f
	case float64:
		return x
	}
	panic(fmt.Sprintf("not a float: %T", v))
}
