//go:build postgres

package repo

// The world of the W10 oracle (people_repo_oracle_postgres_test.go): the
// money world (groups, a ledger, collection rounds with obligations, guest
// links, payment reports and receipts) with everything else a person leaves
// behind added to its people, written as literal SQL so both sides seed the
// same statements.
//
//	an       the only admin of g, invited to g2; pays, is owed and owes; a bio,
//	         a city and a budget band; friends with binh and ban; asked by chi;
//	         declined dung; blocking hang; blocked by stranger. A pair with ban
//	         that holds a notebook (an active cycle, both consents, a couple,
//	         two constraints, a sheet both looked at, a kept sheet); posts,
//	         comments, reactions, stories and views both ways; an avatar and
//	         two personal photographs (files on disk) and a group photograph;
//	         tastes, bookmarks, a read mark, two identities, sessions live,
//	         revoked and past their expiry; messages and message reactions; a
//	         memory with a heart and a comment; a vote and a ballot; a
//	         check-in; a report.
//	binh     a member of g and g2 who owes an; friends with an and with the
//	         ended account; blocked by chi in the pair binh–chi; a pair with
//	         the ended account, whose membership ended.
//	chi      admin of g2; blocking binh; a declined edge with ban.
//	dung     an unnamed member of g; pending with ban.
//	ban      a friend of an; the pair an–ban; a pair with hang, no messages.
//	hang     blocked by an, in no group; a report about an.
//	moi      a person with nothing at all.
//
// Newest messages never tie on created_at within one person's list, and no
// two conversations of one person share a display name, so the list's order
// never falls back to the order of an unordered statement.

import (
	"fmt"
	"strings"
)

const (
	kindSession         = 0x7d
	kindSavedPlace      = 0x7e
	kindMessage         = 0x7f
	kindMessageReaction = 0x80
	kindCheckin         = 0x81
)

// peopleNow is the clock the cases hand in, after every fixture.
const peopleNow = "2030-10-01T05:00:00.654321Z"

type peopleWorld struct {
	*moneyWorld
	ban, hang, moi, newcomer string

	pAnBan, pBinhChi, pBinhErased, pBanHang string
	mAnInAnBan                              string
	pairKeyAnBan, pairKeyBinhDung           string

	hangBlockedAt, binhChiBlockedAt string

	imgAvatar, imgPersonal, imgUnreadable, imgGroup, imgBinh string
	keyAvatar, keyPersonal, keyUnreadable, keyGroup, keyBinh string

	cycleAnBan, paperOpen string
	msgG2                 string
}

func pairKeyOf(x, y string) string {
	if y < x {
		x, y = y, x
	}
	return x + ":" + y
}

func newPeopleWorld() *peopleWorld {
	w := &peopleWorld{moneyWorld: newMoneyWorld()}
	w.catalogue()
	w.sql = append(w.sql, fmt.Sprintf(`UPDATE people SET bio = 'Thích đi chợ ('' "dữ liệu mẫu")', city = 'Hội An (dữ liệu mẫu)',
		budget_band = 'tiet-kiem', wall_comment_policy = 'friends' WHERE id = '%s'`, w.an))
	w.ban = w.person(0x57, "Bạn thân (dữ liệu mẫu)", "bio", "Hay đi phượt (dữ liệu mẫu)", "budget_band", "vua-phai")
	w.hang = w.person(0x58, "Hằng (dữ liệu mẫu)", "wall_comment_policy", "nobody", "discoverable_by_phone", false)
	w.moi = w.person(0x59, "Mới (dữ liệu mẫu)")
	w.newcomer = fid(kindPerson, 0x5d)

	member := func(n int, contextID, personID, state string) string {
		id := fid(kindMembership, n)
		pairs := []any{"id", id, "context_id", contextID, "person_id", personID, "state", state, "created_at", stdCreated}
		switch state {
		case "active":
			pairs = append(pairs, "joined_at", "2030-01-03T00:00:00Z")
		case "left":
			pairs = append(pairs, "joined_at", "2030-01-03T00:00:00Z", "left_at", "2030-08-01T00:00:00Z")
		}
		w.insert("memberships", pairs...)
		return id
	}
	pair := func(n int, x, y string) string {
		id := fid(kindContext, n)
		w.insert("contexts", "id", id, "display_name", "", "created_by_id", x, "created_at", stdCreated,
			"kind", "pair", "pair_key", pairKeyOf(x, y))
		return id
	}
	w.pAnBan = pair(0x57, w.an, w.ban)
	w.mAnInAnBan = member(0x59, w.pAnBan, w.an, "active")
	member(0x5a, w.pAnBan, w.ban, "active")
	w.pBinhChi = pair(0x58, w.chi, w.binh)
	member(0x5b, w.pBinhChi, w.binh, "active")
	member(0x5c, w.pBinhChi, w.chi, "active")
	w.pBinhErased = pair(0x59, w.binh, w.erased)
	member(0x5d, w.pBinhErased, w.binh, "active")
	member(0x5e, w.pBinhErased, w.erased, "left")
	w.pBanHang = pair(0x5a, w.hang, w.ban)
	member(0x5f, w.pBanHang, w.ban, "active")
	member(0x60, w.pBanHang, w.hang, "active")
	w.pairKeyAnBan = pairKeyOf(w.an, w.ban)
	w.pairKeyBinhDung = pairKeyOf(w.binh, w.dung)

	// The friend graph.
	edge := func(n int, requester, addressee, state, createdAt string, decidedBy, decidedAt any) {
		w.insert("friend_requests", "id", fid(kindFriendRequest, n), "requester_id", requester, "addressee_id", addressee,
			"state", state, "created_at", createdAt, "decided_by_id", decidedBy, "decided_at", decidedAt)
	}
	w.hangBlockedAt = "2030-09-10T00:00:00.5Z"
	w.binhChiBlockedAt = "2030-09-12T08:00:00.25Z"
	edge(0x71, w.an, w.binh, "accepted", "2030-03-01T00:00:00Z", w.binh, "2030-03-02T00:00:00Z")
	edge(0x72, w.ban, w.an, "accepted", "2030-03-03T00:00:00Z", w.an, "2030-03-04T00:00:00Z")
	edge(0x73, w.chi, w.an, "pending", "2030-09-01T00:00:00Z", nil, nil)
	edge(0x74, w.an, w.dung, "declined", "2030-04-01T00:00:00Z", w.dung, "2030-04-02T00:00:00Z")
	edge(0x75, w.hang, w.an, "blocked", "2030-09-09T00:00:00Z", w.an, w.hangBlockedAt)
	edge(0x76, w.an, w.stranger, "blocked", "2030-09-11T00:00:00Z", w.stranger, "2030-09-11T01:00:00Z")
	edge(0x77, w.binh, w.chi, "blocked", "2030-02-01T00:00:00Z", w.chi, w.binhChiBlockedAt)
	edge(0x78, w.erased, w.binh, "accepted", "2030-02-01T00:00:00Z", w.binh, "2030-02-02T00:00:00Z")
	edge(0x79, w.dung, w.ban, "pending", "2030-09-02T00:00:00Z", nil, nil)
	edge(0x7a, w.chi, w.ban, "declined", "2030-05-01T00:00:00Z", w.ban, "2030-05-02T00:00:00Z")

	// The pair an–ban's notebook.
	pw := &pairRepoWorld{}
	nb := pw.notebook(0x31, w.pAnBan)
	w.cycleAnBan = pw.activeCycle(0x31, nb, "2030-09-01T00:00:00Z", w.an, w.ban)
	lapSo := pw.proposal(0x31, w.cycleAnBan, "lap_so", w.an, "2030-09-01T00:00:00Z", "2030-09-08T00:00:00Z", "2030-09-01T01:00:00Z")
	pw.consent(0x31, lapSo, w.an, "2030-09-01T00:00:00Z", nil, "2030-09-01T00:00:00Z")
	pw.consent(0x32, lapSo, w.ban, "2030-09-01T01:00:00Z", nil, "2030-09-01T01:00:00Z")
	batDoi := pw.proposal(0x32, w.cycleAnBan, "bat_doi", w.ban, "2030-09-02T00:00:00Z", "2030-09-09T00:00:00Z", "2030-09-02T01:00:00Z")
	pw.consent(0x33, batDoi, w.ban, "2030-09-02T00:00:00Z", nil, "2030-09-02T00:00:00Z")
	pw.consent(0x34, batDoi, w.an, "2030-09-02T01:00:00Z", nil, "2030-09-02T01:00:00Z")
	for _, p := range []string{w.an, w.ban} {
		pw.insert("active_couple_members", "person_id", p, "cycle_id", w.cycleAnBan, "since", "2030-09-02T01:00:00Z")
	}
	pw.constraint(w.cycleAnBan, w.an, "dung", "Đừng gọi sớm (dữ liệu mẫu)", 1, "2030-09-03T00:00:00Z")
	pw.constraint(w.cycleAnBan, w.ban, "khong_an_duoc", "Tôm 🦐 (dữ liệu mẫu)", 2, "2030-09-03T00:00:00Z")
	w.paperOpen = pw.sentPaper(0x31, w.pAnBan, w.cycleAnBan, w.an, "da_xem", "2030-09-30", "2030-09-30T02:00:00Z",
		"2030-10-06T17:00:00Z", "2030-10-05")
	pw.view(w.paperOpen, 1, w.ban, "2030-09-30T03:00:00Z")
	pw.view(w.paperOpen, 1, w.an, "2030-09-30T02:30:00Z")
	pw.response(0x31, w.paperOpen, 1, w.an, "dong_y", "2030-09-30T02:00:00Z")
	kept := pw.sentPaper(0x32, w.pAnBan, w.cycleAnBan, w.ban, "da_giu", "2030-09-16", "2030-09-16T02:00:00Z",
		"2030-09-22T17:00:00Z", "2030-09-21", "done_recorded_by_id", w.an, "done_recorded_at", "2030-09-21T12:00:00Z")
	pw.response(0x32, kept, 1, w.ban, "dong_y", "2030-09-16T02:00:00Z")
	pw.response(0x33, kept, 1, w.an, "dong_y", "2030-09-17T00:00:00Z")
	pw.keep(0x31, kept, w.an, "Vui (dữ liệu mẫu)", "2030-09-21T13:00:00Z")
	pw.keep(0x32, kept, w.ban, "Lần sau đi sớm (dữ liệu mẫu)", "2030-09-21T13:00:00Z")
	w.sql = append(w.sql, pw.sql...)

	// Photographs. The files are the oracle's to put on disk, per case.
	image := func(n int, owner, context any, uploader, purpose string) (string, string) {
		id := fid(kindImage, n)
		w.insert("uploaded_images", "id", id, "storage_key", photoKey(n), "context_id", context, "owner_person_id", owner,
			"uploaded_by_id", uploader, "purpose", purpose, "content_type", "image/jpeg", "byte_size", 2048,
			"width", 640, "height", 480, "created_at", fmt.Sprintf("2030-09-0%dT00:00:00Z", n-0x70))
		return id, photoKey(n)
	}
	w.imgAvatar, w.keyAvatar = image(0x71, w.an, nil, w.an, "avatar")
	w.imgPersonal, w.keyPersonal = image(0x72, w.an, nil, w.an, "personal")
	w.imgUnreadable, w.keyUnreadable = image(0x73, w.an, nil, w.an, "personal")
	w.imgGroup, w.keyGroup = image(0x74, nil, w.g, w.an, "group")
	w.imgBinh, w.keyBinh = image(0x75, w.binh, nil, w.binh, "avatar")

	// Posts, comments, reactions, stories and views.
	post := func(n int, author, audience string, context any, createdAt string, imageURL any) string {
		id := fid(kindPost, n)
		w.insert("posts", "id", id, "author_id", author, "audience", audience, "context_id", context,
			"body", fmt.Sprintf("Bài %d ' \" (dữ liệu mẫu)", n), "image_url", imageURL, "created_at", createdAt)
		return id
	}
	anPublic := post(0x71, w.an, "public", nil, "2030-08-01T00:00:00Z", "/people/"+w.an+"/photos/"+w.imgPersonal)
	anFriends := post(0x72, w.an, "friends", nil, "2030-08-02T00:00:00Z", nil)
	post(0x73, w.an, "group", w.g, "2030-08-03T00:00:00Z", nil)
	binhPublic := post(0x74, w.binh, "public", nil, "2030-08-04T00:00:00Z", nil)
	comment := func(n int, postID, author string) {
		w.insert("post_comments", "id", fid(kindPostComment, n), "post_id", postID, "author_id", author,
			"body", fmt.Sprintf("Bình luận %d (dữ liệu mẫu)", n), "created_at", fmt.Sprintf("2030-08-05T00:00:%02dZ", n-0x70))
	}
	comment(0x71, anPublic, w.binh)
	comment(0x72, binhPublic, w.an)
	comment(0x73, anFriends, w.chi)
	comment(0x74, binhPublic, w.binh)
	reaction := func(n int, postID, person, kind string) {
		w.insert("post_reactions", "id", fid(kindPostReaction, n), "post_id", postID, "person_id", person, "kind", kind,
			"created_at", fmt.Sprintf("2030-08-06T00:00:%02dZ", n-0x70))
	}
	reaction(0x71, anPublic, w.binh, "heart")
	reaction(0x72, binhPublic, w.an, "like")
	reaction(0x73, anPublic, w.an, "heart")
	reaction(0x74, binhPublic, w.chi, "wow")
	story := func(n int, author, createdAt, expiresAt string) string {
		id := fid(kindStory, n)
		w.insert("stories", "id", id, "author_id", author, "image_url", "/people/"+author+"/photos/"+fid(kindImage, n),
			"caption", nil, "audience", "friends", "created_at", createdAt, "expires_at", expiresAt)
		return id
	}
	anStory := story(0x71, w.an, "2030-09-30T12:00:00Z", "2030-10-01T12:00:00Z")
	binhStory := story(0x72, w.binh, "2030-09-30T13:00:00Z", "2030-10-01T13:00:00Z")
	view := func(storyID, viewer, seenAt string) {
		w.insert("story_views", "story_id", storyID, "viewer_id", viewer, "seen_at", seenAt)
	}
	view(anStory, w.binh, "2030-09-30T14:00:00Z")
	view(binhStory, w.an, "2030-09-30T14:00:01Z")
	view(binhStory, w.chi, "2030-09-30T14:00:02Z")

	// Tastes, bookmarks and identities.
	for n, row := range [][2]string{{w.an, "cafe"}, {w.an, "karaoke"}, {w.binh, "cafe"}} {
		w.insert("person_interests", "id", fid(kindInterest, 0x71+n), "person_id", row[0], "tag", row[1],
			"created_at", stdCreated)
	}
	bookmark := func(n int, person, place, createdAt string) {
		w.insert("saved_places", "id", fid(kindSavedPlace, n), "person_id", person, "place_id", place,
			"created_at", createdAt)
	}
	bookmark(0x72, w.an, "p-b", "2030-09-01T00:00:00Z")
	bookmark(0x71, w.an, "p-c", "2030-09-01T00:00:00Z")
	bookmark(0x73, w.an, "p-da-dong-cua", "2030-09-02T00:00:00Z")
	bookmark(0x74, w.binh, "p-b", "2030-09-03T00:00:00Z")
	identity := func(n int, person, provider, subject string) {
		w.insert("account_identities", "id", fid(kindIdentity, n), "person_id", person, "provider", provider,
			"subject", subject, "created_at", "2030-03-01T00:00:00.25Z", "last_login_at", "2030-09-09T07:00:00.75Z")
	}
	identity(0x71, w.an, "phone", "an-so-dien-thoai-mau")
	identity(0x72, w.an, "google", "an-google-mau")
	identity(0x73, w.binh, "phone", "binh-so-dien-thoai-mau")
	session := func(n int, person, via, createdAt, expiresAt string, revokedAt any) {
		w.insert("account_sessions", "id", fid(kindSession, n), "person_id", person,
			"token_digest", raw(fmt.Sprintf("sha256(convert_to('phien-mau-%d', 'UTF8'))", n)), "issued_via", via,
			"created_at", createdAt, "expires_at", expiresAt, "revoked_at", revokedAt)
	}
	session(0x72, w.an, "otp", "2030-09-01T00:00:00Z", "2030-12-01T00:00:00Z", nil)
	session(0x71, w.an, "google", "2030-08-01T00:00:00Z", "2030-11-01T00:00:00Z", "2030-08-15T00:00:00Z")
	session(0x73, w.an, "genesis", "2030-07-01T00:00:00Z", "2030-09-15T00:00:00Z", nil)
	session(0x74, w.binh, "otp", "2030-09-01T00:00:00Z", "2030-12-01T00:00:00Z", nil)

	// Messages: every preview shape is somebody's newest message.
	message := func(n int, contextID string, author any, kind string, createdAt string, extra ...any) string {
		id := fid(kindMessage, n)
		w.insert("messages", append([]any{"id", id, "context_id", contextID, "author_id", author, "kind", kind,
			"created_at", createdAt}, extra...)...)
		return id
	}
	long := "  　Dòng một\nDòng hai " + strings.Repeat("rất dài ", 12) + "(dữ liệu mẫu)\t "
	m1 := message(0x71, w.g, w.an, "text", "2030-09-01T10:00:00Z", "body", "Chào cả nhóm (dữ liệu mẫu)")
	m2 := message(0x72, w.g, w.binh, "image", "2030-09-01T10:05:00.5Z", "image_url", "/static/mau/anh-72.jpg")
	message(0x73, w.g, nil, "ai_card", "2030-09-01T10:10:00Z", "card", `{"kind": "itinerary"}`)
	message(0x74, w.g, w.an, "text", "2030-09-01T10:20:00Z", "body", long)
	m5 := message(0x75, w.pAnBan, w.ban, "sticker", "2030-09-02T09:00:00Z", "body", "tim-do")
	message(0x76, w.pAnBan, w.an, "deleted", "2030-09-02T09:30:00Z", "deleted_at", "2030-09-02T09:31:00Z")
	message(0x78, w.pBinhChi, nil, "ai_card", "2030-08-31T00:00:00Z", "card", `{"kind": 7}`)
	m7 := message(0x77, w.pBinhChi, w.chi, "text", "2030-09-01T10:20:00.000001Z", "body", "Ơ kìa (dữ liệu mẫu)")
	message(0x79, w.pBinhErased, w.erased, "text", "2030-08-01T00:00:00Z", "body", "Tạm biệt (dữ liệu mẫu)")
	w.msgG2 = fid(kindMessage, 0x7f)
	mark := func(contextID, person, messageID, at string) {
		w.insert("context_read_marks", "context_id", contextID, "person_id", person, "last_read_message_id", messageID,
			"last_read_at", at, "updated_at", at)
	}
	mark(w.g, w.an, m2, "2030-09-01T10:05:00.5Z")
	mark(w.pBinhChi, w.binh, m7, "2030-09-01T10:20:00.000001Z")
	mark(w.pAnBan, w.ban, m5, "2030-09-02T09:00:00Z")
	w.insert("message_reactions", "id", fid(kindMessageReaction, 0x71), "message_id", m1, "person_id", w.binh,
		"kind", "heart", "created_at", "2030-09-01T11:00:00Z")
	w.insert("message_reactions", "id", fid(kindMessageReaction, 0x72), "message_id", m2, "person_id", w.an,
		"kind", "like", "created_at", "2030-09-01T11:00:01Z")

	// What stays: a memory with a heart and comments, a vote, a check-in, reports.
	memory := w.photo(0x71, w.g, w.an, "2030-09-04T00:00:00Z", "caption", "Chợ đêm (dữ liệu mẫu)")
	binhMemory := w.photo(0x72, w.g, w.binh, "2030-09-04T00:00:01Z")
	w.insert("memory_reactions", "id", fid(kindReaction, 0x71), "memory_id", memory, "person_id", w.binh,
		"created_at", "2030-09-04T01:00:00Z")
	w.insert("memory_reactions", "id", fid(kindReaction, 0x72), "memory_id", binhMemory, "person_id", w.an,
		"created_at", "2030-09-04T01:00:01Z")
	w.insert("memory_comments", "id", fid(kindComment, 0x71), "memory_id", memory, "author_id", w.binh,
		"body", "Đẹp quá (dữ liệu mẫu)", "created_at", "2030-09-04T02:00:00Z")
	w.insert("memory_comments", "id", fid(kindComment, 0x72), "memory_id", binhMemory, "author_id", w.an,
		"body", "Cảm ơn (dữ liệu mẫu)", "created_at", "2030-09-04T02:00:01Z")
	vote := fid(kindVote, 0x71)
	w.insert("votes", "id", vote, "context_id", w.g, "outing_id", nil, "created_by_id", w.an,
		"question", "Đi đâu? (dữ liệu mẫu)", "created_at", "2030-09-05T00:00:00Z", "closed_at", nil, "closed_by_id", nil)
	optA, optB := fid(kindVoteOption, 0x71), fid(kindVoteOption, 0x72)
	w.insert("vote_options", "id", optA, "vote_id", vote, "position", 0, "label", "Biển (dữ liệu mẫu)", "place_name", nil)
	w.insert("vote_options", "id", optB, "vote_id", vote, "position", 1, "label", "Núi (dữ liệu mẫu)", "place_name", nil)
	w.insert("vote_ballots", "id", fid(kindBallot, 0x71), "vote_id", vote, "option_id", optA, "voter_id", w.an,
		"created_at", "2030-09-05T01:00:00Z", "updated_at", "2030-09-05T01:00:00Z")
	w.insert("vote_ballots", "id", fid(kindBallot, 0x72), "vote_id", vote, "option_id", optB, "voter_id", w.binh,
		"created_at", "2030-09-05T01:00:01Z", "updated_at", "2030-09-05T01:00:01Z")
	stop := fid(kindStop, 0x71)
	w.insert("outing_stops", "id", stop, "outing_id", fid(kindOuting, 0x51), "position", 0, "minute_of_day", 600,
		"label", "Chợ (dữ liệu mẫu)")
	w.insert("outing_stop_checkins", "id", fid(kindCheckin, 0x71), "stop_id", stop, "person_id", w.an,
		"created_at", "2030-05-01T10:00:00Z")
	w.insert("outing_stop_checkins", "id", fid(kindCheckin, 0x72), "stop_id", stop, "person_id", w.binh,
		"created_at", "2030-05-01T10:00:01Z")
	w.insert("reports", "id", fid(kindReport, 0x71), "reporter_id", w.an, "target_type", "post", "target_id", binhPublic,
		"reason", "spam", "note", nil, "created_at", "2030-08-10T00:00:00Z")
	w.insert("reports", "id", fid(kindReport, 0x72), "reporter_id", w.hang, "target_type", "person", "target_id", w.an,
		"reason", "harassment", "note", "Ghi chú (dữ liệu mẫu)", "created_at", "2030-09-09T00:00:01Z")
	return w
}

// g2Card is the SQL of an ai_card message that becomes g2's newest, for the
// cases about preview shapes.
func (w *peopleWorld) g2Card(card string) string {
	return insertSQL("messages", "id", w.msgG2, "context_id", w.g2, "author_id", nil, "kind", "ai_card",
		"card", card, "created_at", "2030-09-20T00:00:00Z")
}
