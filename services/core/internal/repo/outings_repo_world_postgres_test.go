//go:build postgres

package repo

// The world of the W7 oracle (outings_repo_oracle_postgres_test.go): one
// group with four trips, a second group the actor does not belong to, and a
// pair conversation, written as literal SQL through seed_postgres_test.go's
// world so the same statements seed both sides.
//
//	oMulti  three days, version 1, four stops of which two say exactly the
//	        same thing (so `unclaimed` has to pop them one at a time), one
//	        carries a catalogue place, and three arrivals hang off two of
//	        them -- two of the arrivals tied on created_at
//	oDay    one single day, version 1, no stops: what a new stop's `day` is
//	        read from
//	oPlan   version 2, days already stored, three stops covering a locked
//	        time, a catalogue place and a meeting point
//	oEmpty  version 1, revision 0, nothing on it
//	oOther  a trip of the group the actor is not in
//	oPair   a trip of a pair conversation, where the invitation door is shut
//
// The invitations cover every state accept, revoke and rotate branch on:
// live link, live named, accepted, revoked, expired, and named-then-revoked.
// No token is written down: the fixture stores sha256 of a short sample word
// computed by PostgreSQL, and the cases hand both sides that same word.

import "fmt"

const (
	kindStopCheckin  = 0xea
	kindOutingInvite = 0xec
)

// outingNow is noon in Vietnam on the first day of oMulti.
const outingNow = "2030-10-01T05:00:00.123456Z"

// tokenOf is the sample word a case presents at the door; the fixture stores
// only what sha256 makes of it.
func storedDigest(token string) raw {
	return raw("sha256(convert_to('" + token + "', 'UTF8'))")
}

type outingWorld struct {
	world
	an, binh, chi, dung, khach, ngoai, missingPerson string
	group, other, pairCtx, missingContext            string
	oMulti, oDay, oPlan, oEmpty, oOther, oPair       string
	missingOuting                                    string
	s1, s2, s3, s4, t1, t2, t3, missingStop          string
	invLink, invNamed, invAccepted                   string
	invRevoked, invExpired, invNamedRevoked          string
	missingInvite                                    string

	tokLink, tokNamed, tokAccepted    string
	tokRevoked, tokExpired, tokUnused string
}

func (w *outingWorld) member(n int, contextID, personID, state string, extra ...any) {
	pairs := []any{"id", fid(kindMembership, n), "context_id", contextID, "person_id", personID,
		"state", state, "created_at", stdCreated}
	switch state {
	case "active":
		pairs = append(pairs, "joined_at", "2030-01-02T00:00:00Z")
	case "left":
		pairs = append(pairs, "joined_at", "2030-01-02T00:00:00Z", "left_at", "2030-01-03T00:00:00Z")
	}
	w.insert("memberships", append(pairs, extra...)...)
}

func (w *outingWorld) outing(n int, contextID, creator, title, starts, ends string, revision int,
	extra ...any) string {
	id := fid(kindOuting, n)
	w.insert("outings", append([]any{"id", id, "context_id", contextID, "created_by_id", creator,
		"title", title, "starts_on", starts, "ends_on", ends, "headcount", 4,
		"budget_per_person_vnd", int64(500000), "created_at", stdCreated,
		"timeline_revision", revision}, extra...)...)
	return id
}

func (w *outingWorld) stop(n int, outingID string, position, minute int, label string, extra ...any) string {
	id := fid(kindStop, n)
	w.insert("outing_stops", append([]any{"id", id, "outing_id", outingID, "position", position,
		"minute_of_day", minute, "label", label}, extra...)...)
	return id
}

func (w *outingWorld) checkin(n int, stopID, personID, createdAt string) {
	w.insert("outing_stop_checkins", "id", fid(kindStopCheckin, n), "stop_id", stopID,
		"person_id", personID, "created_at", createdAt)
}

func (w *outingWorld) invite(n int, outingID, source string, person any, token string, expires string,
	extra ...any) string {
	id := fid(kindOutingInvite, n)
	pairs := []any{"id", id, "outing_id", outingID, "source", source, "invited_person_id", person,
		"invited_by_id", w.an, "token_digest", storedDigest(token), "created_at", stdCreated,
		"expires_at", expires}
	w.insert("outing_invites", append(pairs, extra...)...)
	return id
}

const outingDays = `[{"day": "2030-10-01", "transport_mode": "motorbike", "start_at": "08:00",` +
	` "start_stop_id": null, "end_stop_id": null, "return_to_start": false}]`

func newOutingWorld() *outingWorld {
	w := &outingWorld{}
	w.an = w.person(0x70, "An (dữ liệu mẫu)")
	w.binh = w.person(0x71, "Bình (dữ liệu mẫu)")
	w.chi = w.person(0x72, "Chi (dữ liệu mẫu)")
	w.dung = w.person(0x73, "Dũng (dữ liệu mẫu)")
	w.khach = w.person(0x74, "Khách (dữ liệu mẫu)")
	w.ngoai = w.person(0x75, "Ngoài nhóm (dữ liệu mẫu)")
	w.missingPerson = fid(kindPerson, 0x7f)

	w.group = fid(kindContext, 0x70)
	w.insert("contexts", "id", w.group, "display_name", "Nhóm đi chơi (dữ liệu mẫu)",
		"created_by_id", w.an, "created_at", stdCreated)
	w.other = fid(kindContext, 0x71)
	w.insert("contexts", "id", w.other, "display_name", "Nhóm khác (dữ liệu mẫu)",
		"created_by_id", w.binh, "created_at", stdCreated)
	w.pairCtx = fid(kindContext, 0x72)
	w.insert("contexts", "id", w.pairCtx, "display_name", "", "created_by_id", w.an,
		"created_at", stdCreated, "kind", "pair", "pair_key", w.an+":"+w.binh)
	w.missingContext = fid(kindContext, 0x7f)

	w.member(0x70, w.group, w.an, "active", "role", "admin")
	w.member(0x71, w.group, w.binh, "active")
	w.member(0x72, w.group, w.chi, "active")
	w.member(0x73, w.group, w.dung, "left")
	w.member(0x74, w.group, w.khach, "invited", "invited_by_id", w.an, "origin", "link")
	w.member(0x75, w.other, w.binh, "active")
	w.member(0x76, w.pairCtx, w.an, "active")
	w.member(0x77, w.pairCtx, w.binh, "active")

	w.catalogue()

	w.oMulti = w.outing(0x70, w.group, w.an, "Ba ngày (dữ liệu mẫu)", "2030-10-01", "2030-10-03", 4)
	w.oDay = w.outing(0x71, w.group, w.an, "Một ngày (dữ liệu mẫu)", "2030-10-05", "2030-10-05", 1)
	w.oPlan = w.outing(0x72, w.group, w.binh, "Có lịch (dữ liệu mẫu)", "2030-10-01", "2030-10-02", 7,
		"itinerary_version", 2, "itinerary_days", raw("'"+outingDays+"'::jsonb"))
	w.oEmpty = w.outing(0x73, w.group, w.chi, "Chưa có chặng (dữ liệu mẫu)", "2030-11-01", "2030-11-01", 0)
	w.oOther = w.outing(0x74, w.other, w.binh, "Của nhóm khác (dữ liệu mẫu)", "2030-10-01", "2030-10-02", 0)
	w.oPair = w.outing(0x75, w.pairCtx, w.an, "Của hai người (dữ liệu mẫu)", "2030-10-01", "2030-10-01", 0)
	w.missingOuting = fid(kindOuting, 0x7f)

	// Two stops that say exactly the same thing, so a save that keeps one of
	// them has to pop the pair one at a time rather than match both to one.
	w.s1 = w.stop(0x70, w.oMulti, 0, 480, "Cà phê sáng (dữ liệu mẫu)", "place_name", "Quán A (dữ liệu mẫu)")
	w.s2 = w.stop(0x71, w.oMulti, 1, 480, "Cà phê sáng (dữ liệu mẫu)", "place_name", "Quán A (dữ liệu mẫu)")
	w.s3 = w.stop(0x72, w.oMulti, 2, 600, "Chợ (dữ liệu mẫu)", "place_id", "p-c")
	w.s4 = w.stop(0x73, w.oMulti, 3, 700, "Biển (dữ liệu mẫu)")
	w.t1 = w.stop(0x78, w.oPlan, 0, 480, "Sáng (dữ liệu mẫu)", "day", "2030-10-01",
		"duration_minutes", 30, "time_locked", false)
	w.t2 = w.stop(0x79, w.oPlan, 1, 540, "Trưa (dữ liệu mẫu)", "day", "2030-10-01", "place_id", "p-a")
	w.t3 = w.stop(0x7a, w.oPlan, 2, 600, "Chiều (dữ liệu mẫu)", "day", "2030-10-02",
		"meeting_lat", 10.7702, "meeting_lng", 106.7, "meeting_label", "Cổng chính (dữ liệu mẫu)")
	w.missingStop = fid(kindStop, 0x7f)

	// A tie on created_at, so the order falls to the id.
	const tie = "2030-09-20T02:00:00.5Z"
	w.checkin(0x70, w.s1, w.binh, tie)
	w.checkin(0x71, w.s1, w.chi, tie)
	w.checkin(0x72, w.s3, w.binh, "2030-09-21T02:00:00Z")

	w.tokLink, w.tokNamed = "moi-lien-ket-mau", "moi-binh-mau"
	w.tokAccepted, w.tokRevoked = "moi-da-dung-mau", "moi-da-thu-hoi-mau"
	w.tokExpired, w.tokUnused = "moi-het-han-mau", "khong-ai-phat-mau"
	w.invLink = w.invite(0x70, w.oMulti, "link", nil, w.tokLink, "2030-12-01T00:00:00Z")
	w.invNamed = w.invite(0x71, w.oMulti, "group", w.binh, w.tokNamed, "2030-12-01T00:00:00Z")
	w.invAccepted = w.invite(0x72, w.oMulti, "link", nil, w.tokAccepted, "2030-12-01T00:00:00Z",
		"accepted_at", "2030-09-25T00:00:00Z", "accepted_by_id", w.chi)
	w.invRevoked = w.invite(0x73, w.oMulti, "link", nil, w.tokRevoked, "2030-12-01T00:00:00Z",
		"revoked_at", "2030-09-26T00:00:00Z")
	w.invExpired = w.invite(0x74, w.oMulti, "link", nil, w.tokExpired, "2030-02-01T00:00:00Z")
	w.invNamedRevoked = w.invite(0x75, w.oMulti, "friend", w.chi, "moi-chi-mau", "2030-12-01T00:00:00Z",
		"revoked_at", "2030-09-26T00:00:00Z")
	w.missingInvite = fid(kindOutingInvite, 0x7f)
	return w
}

// outingDayJSON is one element of an itinerary_days argument, written the way
// ItineraryDay.model_dump(mode="json") orders its fields.
func outingDayJSON(day, mode, startAt string, startStop, endStop any, returnToStart bool) string {
	anchor := func(value any) string {
		if text, ok := value.(string); ok {
			return `"` + text + `"`
		}
		return "null"
	}
	return fmt.Sprintf(`{"day": "%s", "transport_mode": "%s", "start_at": "%s", "start_stop_id": %s, `+
		`"end_stop_id": %s, "return_to_start": %t}`, day, mode, startAt, anchor(startStop), anchor(endStop),
		returnToStart)
}
