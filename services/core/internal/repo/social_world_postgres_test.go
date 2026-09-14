//go:build postgres

package repo

// Fixtures for the W2 methods (friends, stories, posts, votes), written as
// literal SQL through seed_postgres_test.go's world so the same statements
// seed the Go side and the Python side of the oracle. Every order a read
// imposes has a tie or an inversion in it (created_at ties broken by ids that
// sort against insertion order), every visibility rule has a reader on each
// side of it (friends, pending, declined, blocked both ways, group members who
// left or were only invited, an erased account, an unnamed person), and every
// deadline has a story on it and a microsecond before it.

import "fmt"

const (
	kindFriendRequest = 0x61
	kindImage         = 0x62
	kindStory         = 0x63
	kindPost          = 0x64
	kindPostReaction  = 0x65
	kindPostComment   = 0x66
	kindVote          = 0x67
	kindVoteOption    = 0x68
	kindBallot        = 0x69
	kindIdentity      = 0x6a
)

// socialNow is the instant the story cases stand on.
const socialNow = "2030-04-10T12:00:00Z"

type socialWorld struct {
	world
	an, binh, chi, dung, em, phuong, giang, hoa, khoa, missingPerson string
	g1, g2, g3, missingContext                                       string

	frAnBinh, frChiAn, frHoaAn, frAnDungDeclined, frDungAn, frPhuongBinh, frPhuongAn string
	frEmAn, frAnKhoa, frGiangBinh, frKhoaBinhDeclined, frBinhKhoa, frDungChiDeclined string
	frGiangChi, missingRequest, decidedAnBinh                                        string

	imgBinhPersonal, imgBinhAvatar, imgG1, imgAnPersonal, missingImage string
	subjectAn, subjectBinh, subjectEm                                  string

	stBinhOld, stBinhTieB, stBinhTieA, stBinhExpired, stAnLive, stAnExpired string
	stPhuong, stKhoa, stEm, stChi, missingStory                             string

	pBinhPublic, pBinhFriends, pBinhOnlyMe, pBinhGroup, pAnOnlyMe, pAnPublic string
	pPhuongPublic, pPhuongGroup, pHoaGroup, pDungGroup, pKhoaFriends         string
	pEmFriends, pChiPublic, missingPost                                      string
	c1, c2, c3, c4, c5, missingComment, commentTie                           string

	oG1, oBare, missingOuting                    string
	vTie, vClosed, vOpen, vG2, missingVote       string
	optOpenFar, optOpenHome, optOpenCafe, optTie string
	optClosedA, optClosedB, optG2, missingOption string
	ballotOpenBinhAt                             string
}

func (w *socialWorld) friendRequest(n int, requester, addressee, state, createdAt string, decidedBy, decidedAt any) string {
	id := fid(kindFriendRequest, n)
	w.insert("friend_requests", "id", id, "requester_id", requester, "addressee_id", addressee, "state", state,
		"created_at", createdAt, "decided_by_id", decidedBy, "decided_at", decidedAt)
	return id
}

func (w *socialWorld) image(n int, owner, context any, uploader, purpose string) string {
	id := fid(kindImage, n)
	w.insert("uploaded_images", "id", id, "storage_key", fmt.Sprintf("mau/anh-%d.jpg", n), "context_id", context,
		"owner_person_id", owner, "uploaded_by_id", uploader, "purpose", purpose, "content_type", "image/jpeg",
		"byte_size", 2048, "width", 640, "height", 480, "created_at", "2030-03-01T00:00:00.5Z")
	return id
}

func (w *socialWorld) story(n int, author, createdAt, expiresAt string, caption any) string {
	id := fid(kindStory, n)
	w.insert("stories", "id", id, "author_id", author, "image_url", fmt.Sprintf("/people/%s/photos/%s", author, fid(kindImage, n)),
		"caption", caption, "audience", "friends", "created_at", createdAt, "expires_at", expiresAt)
	return id
}

func (w *socialWorld) view(story, viewer, seenAt string) {
	w.insert("story_views", "story_id", story, "viewer_id", viewer, "seen_at", seenAt)
}

func (w *socialWorld) post(n int, author, audience string, context any, body, createdAt string, imageURL any) string {
	id := fid(kindPost, n)
	w.insert("posts", "id", id, "author_id", author, "audience", audience, "context_id", context, "body", body,
		"image_url", imageURL, "created_at", createdAt)
	return id
}

func (w *socialWorld) reaction(n int, post, person, kind string) {
	w.insert("post_reactions", "id", fid(kindPostReaction, n), "post_id", post, "person_id", person, "kind", kind,
		"created_at", fmt.Sprintf("2030-05-01T12:00:%02dZ", n))
}

func (w *socialWorld) comment(n int, post, author, body, createdAt string) string {
	id := fid(kindPostComment, n)
	w.insert("post_comments", "id", id, "post_id", post, "author_id", author, "body", body, "created_at", createdAt)
	return id
}

func (w *socialWorld) vote(n int, context, creator, question, createdAt string, outing any, closedAt, closedBy any) string {
	id := fid(kindVote, n)
	w.insert("votes", "id", id, "context_id", context, "outing_id", outing, "created_by_id", creator,
		"question", question, "created_at", createdAt, "closed_at", closedAt, "closed_by_id", closedBy)
	return id
}

func (w *socialWorld) option(n int, vote string, position int, label string, place any) string {
	id := fid(kindVoteOption, n)
	w.insert("vote_options", "id", id, "vote_id", vote, "position", position, "label", label, "place_name", place)
	return id
}

func (w *socialWorld) ballot(n int, vote, option, voter, createdAt, updatedAt string) {
	w.insert("vote_ballots", "id", fid(kindBallot, n), "vote_id", vote, "option_id", option, "voter_id", voter,
		"created_at", createdAt, "updated_at", updatedAt)
}

func newSocialWorld() *socialWorld {
	w := &socialWorld{}
	w.an = w.person(0x21, "An (dữ liệu mẫu)")
	w.binh = w.person(0x22, "Bình (dữ liệu mẫu)", "wall_comment_policy", "friends")
	w.chi = w.person(0x23, "")
	w.dung = w.person(0x24, "Dũng (dữ liệu mẫu)")
	w.em = w.person(0x25, "Em (dữ liệu mẫu)", "deleted_at", "2030-03-20T00:00:00Z")
	w.phuong = w.person(0x26, "Phương (dữ liệu mẫu)")
	w.giang = w.person(0x27, "Giang (dữ liệu mẫu)")
	w.hoa = w.person(0x28, "Hoa (dữ liệu mẫu)")
	w.khoa = w.person(0x29, "Khoa (dữ liệu mẫu)")
	w.missingPerson = fid(kindPerson, 0xfe)

	// g1: an, binh and phuong active, dung left. g2: hoa and an active, dung
	// only invited. g3: nobody.
	w.g1 = w.context(0x21, w.an)
	w.g2 = w.context(0x22, w.hoa)
	w.g3 = w.context(0x23, w.binh)
	w.missingContext = fid(kindContext, 0xfe)
	w.membership(0x61, w.g1, w.an, "active")
	w.membership(0x62, w.g1, w.binh, "active")
	w.membership(0x63, w.g1, w.phuong, "active")
	w.membership(0x64, w.g1, w.dung, "left")
	w.membership(0x65, w.g2, w.hoa, "active")
	w.membership(0x66, w.g2, w.an, "active")
	w.membership(0x67, w.g2, w.dung, "invited")

	// The friend graph. an: friends with binh and the erased em (decided at
	// the same instant), asked by chi and hoa at the same instant and by dung
	// after declining dung, asking khoa, and blocking phuong. binh: friends
	// with giang and khoa (decided at the same instant, khoa after an earlier
	// decline) and blocking phuong.
	w.decidedAnBinh = "2030-03-02T09:30:00.654321Z"
	w.frAnBinh = w.friendRequest(0x01, w.an, w.binh, "accepted", "2030-03-01T08:00:00Z", w.binh, w.decidedAnBinh)
	w.frChiAn = w.friendRequest(0x07, w.chi, w.an, "pending", "2030-03-05T10:00:00.5Z", nil, nil)
	w.frHoaAn = w.friendRequest(0x03, w.hoa, w.an, "pending", "2030-03-05T10:00:00.5Z", nil, nil)
	w.frAnDungDeclined = w.friendRequest(0x04, w.an, w.dung, "declined", "2030-03-01T00:00:00Z", w.dung, "2030-03-01T12:00:00Z")
	w.frDungAn = w.friendRequest(0x05, w.dung, w.an, "pending", "2030-03-06T00:00:00Z", nil, nil)
	w.frPhuongBinh = w.friendRequest(0x06, w.phuong, w.binh, "blocked", "2030-03-01T00:00:00Z", w.binh, "2030-03-02T00:00:00Z")
	w.frPhuongAn = w.friendRequest(0x08, w.phuong, w.an, "blocked", "2030-03-01T00:00:01Z", w.an, "2030-03-02T00:00:01Z")
	w.frEmAn = w.friendRequest(0x09, w.em, w.an, "accepted", "2030-03-01T09:00:00Z", w.an, w.decidedAnBinh)
	w.frAnKhoa = w.friendRequest(0x0a, w.an, w.khoa, "pending", "2030-03-04T00:00:00Z", nil, nil)
	w.frGiangBinh = w.friendRequest(0x0b, w.giang, w.binh, "accepted", "2030-03-01T00:00:00Z", w.binh, "2030-03-03T00:00:00Z")
	w.frKhoaBinhDeclined = w.friendRequest(0x0c, w.khoa, w.binh, "declined", "2030-02-01T00:00:00Z", w.binh, "2030-02-02T00:00:00Z")
	w.frBinhKhoa = w.friendRequest(0x0d, w.binh, w.khoa, "accepted", "2030-03-01T00:00:00Z", w.khoa, "2030-03-03T00:00:00Z")
	w.frDungChiDeclined = w.friendRequest(0x0e, w.dung, w.chi, "declined", "2030-03-01T00:00:00Z", w.chi, "2030-03-01T01:00:00Z")
	w.frGiangChi = w.friendRequest(0x0f, w.giang, w.chi, "accepted", "2030-03-01T00:00:00Z", w.chi, "2030-03-04T00:00:00Z")
	w.missingRequest = fid(kindFriendRequest, 0xfe)

	w.imgBinhPersonal = w.image(0x01, w.binh, nil, w.binh, "personal")
	w.imgBinhAvatar = w.image(0x02, w.binh, nil, w.binh, "avatar")
	w.imgG1 = w.image(0x03, nil, w.g1, w.an, "group")
	w.imgAnPersonal = w.image(0x04, w.an, nil, w.an, "personal")
	w.missingImage = fid(kindImage, 0xfe)

	w.subjectAn = "cafebabedeadbeefcafebabedeadbeef"
	w.subjectBinh = "google-sub-mau-binh"
	w.subjectEm = "facefeedfacefeedfacefeedfacefeed"
	identity := func(n int, person, provider, subject string) {
		w.insert("account_identities", "id", fid(kindIdentity, n), "person_id", person, "provider", provider,
			"subject", subject, "created_at", "2030-03-01T00:00:00.25Z", "last_login_at", "2030-03-09T07:00:00.75Z")
	}
	identity(0x01, w.an, "phone", w.subjectAn)
	identity(0x02, w.binh, "google", w.subjectBinh)
	identity(0x03, w.em, "phone", w.subjectEm)

	// Stories around socialNow. stBinhTieA and stBinhTieB tie on created_at
	// with ids against insertion order; stBinhTieA's deadline IS socialNow.
	w.stBinhOld = w.story(0x05, w.binh, "2030-04-10T00:00:00Z", "2030-04-11T00:00:00Z", nil)
	w.stBinhTieB = w.story(0x02, w.binh, "2030-04-10T06:00:00Z", "2030-04-11T06:00:00Z", "Chiều nay 🌅 (dữ liệu mẫu)")
	w.stBinhTieA = w.story(0x03, w.binh, "2030-04-10T06:00:00Z", socialNow, "")
	w.stBinhExpired = w.story(0x04, w.binh, "2030-04-08T00:00:00Z", "2030-04-09T00:00:00Z", nil)
	w.stAnLive = w.story(0x06, w.an, "2030-04-10T02:00:00Z", "2030-04-11T02:00:00Z", nil)
	w.stAnExpired = w.story(0x07, w.an, "2030-04-09T02:00:00Z", "2030-04-10T02:00:00Z", nil)
	w.stPhuong = w.story(0x08, w.phuong, "2030-04-10T03:00:00Z", "2030-04-11T03:00:00Z", nil)
	w.stKhoa = w.story(0x09, w.khoa, "2030-04-10T04:00:00Z", "2030-04-11T04:00:00Z", nil)
	w.stEm = w.story(0x0a, w.em, "2030-04-10T05:00:00Z", "2030-04-10T12:00:00.000001Z", nil)
	w.stChi = w.story(0x0b, w.chi, "2030-04-10T05:30:00Z", "2030-04-11T05:30:00Z", nil)
	w.missingStory = fid(kindStory, 0xfe)
	w.view(w.stBinhOld, w.an, "2030-04-10T01:00:00.25Z")
	w.view(w.stEm, w.an, "2030-04-10T06:00:00Z")
	w.view(w.stAnLive, w.binh, "2030-04-10T03:00:00Z")
	w.view(w.stBinhTieB, w.giang, "2030-04-10T07:00:00Z")

	// Posts. Three of binh's tie on created_at with ids against insertion
	// order; phuong is blocked by an and by binh, which hides phuong's public
	// post from both but not phuong's group post.
	tie := "2030-05-01T00:00:00Z"
	w.pBinhPublic = w.post(0x03, w.binh, "public", nil, "Công khai (dữ liệu mẫu)", tie, nil)
	w.pBinhFriends = w.post(0x02, w.binh, "friends", nil, "Bạn bè (dữ liệu mẫu)", tie, nil)
	w.pBinhOnlyMe = w.post(0x01, w.binh, "only_me", nil, "Riêng tôi (dữ liệu mẫu)", tie, nil)
	w.pBinhGroup = w.post(0x04, w.binh, "group", w.g1, "Nhóm (dữ liệu mẫu)", "2030-05-02T00:00:00Z",
		fmt.Sprintf("/contexts/%s/photos/%s", w.g1, w.imgG1))
	w.pAnOnlyMe = w.post(0x05, w.an, "only_me", nil, "Nháp (dữ liệu mẫu)", "2030-05-03T00:00:00Z", nil)
	w.pAnPublic = w.post(0x06, w.an, "public", nil, "Chào (dữ liệu mẫu)", "2030-04-30T00:00:00Z", nil)
	w.pPhuongPublic = w.post(0x07, w.phuong, "public", nil, "Phương công khai (dữ liệu mẫu)", "2030-05-04T00:00:00Z", nil)
	w.pPhuongGroup = w.post(0x08, w.phuong, "group", w.g1, "Phương trong nhóm (dữ liệu mẫu)", "2030-05-04T01:00:00Z", nil)
	w.pHoaGroup = w.post(0x09, w.hoa, "group", w.g2, "Hoa trong nhóm (dữ liệu mẫu)", "2030-05-05T00:00:00Z", nil)
	w.pDungGroup = w.post(0x0a, w.dung, "group", w.g1, "Dũng đã rời (dữ liệu mẫu)", "2030-05-06T00:00:00Z", nil)
	w.pKhoaFriends = w.post(0x0b, w.khoa, "friends", nil, "Khoa bạn bè (dữ liệu mẫu)", "2030-05-07T00:00:00Z", nil)
	w.pEmFriends = w.post(0x0c, w.em, "friends", nil, "Em bạn bè (dữ liệu mẫu)", "2030-05-08T00:00:00Z", nil)
	w.pChiPublic = w.post(0x0d, w.chi, "public", nil, "Không tên 🙂 ' \" \\ (dữ liệu mẫu)", "2030-05-09T00:00:00Z", nil)
	w.missingPost = fid(kindPost, 0xfe)

	w.reaction(0x05, w.pBinhPublic, w.an, "heart")
	w.reaction(0x01, w.pBinhPublic, w.an, "fire")
	w.reaction(0x02, w.pBinhPublic, w.binh, "heart")
	w.reaction(0x03, w.pBinhPublic, w.giang, "heart")
	w.reaction(0x04, w.pBinhPublic, w.khoa, "wow")
	w.reaction(0x06, w.pBinhFriends, w.an, "wow")
	w.reaction(0x07, w.pPhuongGroup, w.binh, "sad")

	w.commentTie = "2030-05-01T10:00:00Z"
	w.c3 = w.comment(0x03, w.pBinhPublic, w.chi, "Ba (dữ liệu mẫu)", w.commentTie)
	w.c1 = w.comment(0x01, w.pBinhPublic, w.an, "Một (dữ liệu mẫu)", w.commentTie)
	w.c2 = w.comment(0x02, w.pBinhPublic, w.binh, "Hai (dữ liệu mẫu)", "2030-05-01T09:00:00Z")
	w.c4 = w.comment(0x04, w.pBinhPublic, w.em, "Bốn (dữ liệu mẫu)", "2030-05-01T11:00:00Z")
	w.c5 = w.comment(0x05, w.pBinhFriends, w.an, "Năm (dữ liệu mẫu)", "2030-05-01T10:00:00Z")
	w.missingComment = fid(kindPostComment, 0xfe)

	w.oG1 = fid(kindOuting, 0x61)
	w.insert("outings", "id", w.oG1, "context_id", w.g1, "created_by_id", w.an, "title", "Đà Lạt (dữ liệu mẫu)",
		"starts_on", "2030-06-10", "ends_on", "2030-06-12", "headcount", 3, "budget_per_person_vnd", int64(2500000),
		"created_at", "2030-05-20T00:00:00.5Z", "timeline_revision", 4, "itinerary_version", 2,
		"itinerary_days", `[{"ngay": 1, "diem": [1, 2.5, "chợ"]}, {"ngay": 2}]`)
	w.insert("outing_stops", "id", fid(kindStop, 0x62), "outing_id", w.oG1, "position", 1, "minute_of_day", 600,
		"label", "Chợ (dữ liệu mẫu)", "place_name", "Chợ Đà Lạt (dữ liệu mẫu)", "place_id", "p-cho")
	w.insert("outing_stops", "id", fid(kindStop, 0x61), "outing_id", w.oG1, "position", 0, "minute_of_day", 420,
		"label", "Gặp nhau (dữ liệu mẫu)", "duration_minutes", 30, "time_locked", false, "day", "2030-06-10",
		"meeting_lat", 11.9404, "meeting_lng", 108.4583, "meeting_label", "Bến xe (dữ liệu mẫu)")
	w.oBare = fid(kindOuting, 0x62)
	w.insert("outings", "id", w.oBare, "context_id", w.g2, "created_by_id", w.hoa, "title", "Biển (dữ liệu mẫu)",
		"starts_on", "2030-07-01", "ends_on", "2030-07-01", "headcount", 1, "budget_per_person_vnd", int64(0),
		"created_at", "2030-05-21T00:00:00Z")
	w.missingOuting = fid(kindOuting, 0xfe)

	// Votes. vTie and vOpen tie on created_at with ids against insertion
	// order; vOpen's options are inserted out of position order and its
	// ballots tie on created_at.
	voteTie := "2030-06-01T00:00:00Z"
	w.vOpen = w.vote(0x03, w.g1, w.an, "Tối nay đi đâu? (dữ liệu mẫu)", voteTie, w.oG1, nil, nil)
	w.optOpenFar = w.option(0x01, w.vOpen, 2, "Chợ đêm (dữ liệu mẫu)", "Chợ (dữ liệu mẫu)")
	w.optOpenCafe = w.option(0x03, w.vOpen, 0, "Cà phê (dữ liệu mẫu)", nil)
	w.optOpenHome = w.option(0x02, w.vOpen, 1, "Ở nhà (dữ liệu mẫu)", "")
	w.ballotOpenBinhAt = "2030-06-01T01:00:00.000002Z"
	w.ballot(0x02, w.vOpen, w.optOpenCafe, w.binh, "2030-06-01T01:00:00Z", w.ballotOpenBinhAt)
	w.ballot(0x01, w.vOpen, w.optOpenHome, w.an, "2030-06-01T01:00:00Z", "2030-06-01T01:00:00Z")
	w.ballot(0x03, w.vOpen, w.optOpenFar, w.phuong, "2030-06-01T00:30:00Z", "2030-06-01T02:00:00Z")
	w.vTie = w.vote(0x01, w.g1, w.binh, "Mấy giờ? (dữ liệu mẫu)", voteTie, nil, nil, nil)
	w.optTie = w.option(0x04, w.vTie, 0, "Tám giờ (dữ liệu mẫu)", nil)
	w.vClosed = w.vote(0x02, w.g1, w.an, "Đã chốt (dữ liệu mẫu)", "2030-05-31T00:00:00Z", nil,
		"2030-06-02T00:00:00Z", w.an)
	w.optClosedA = w.option(0x05, w.vClosed, 0, "Có (dữ liệu mẫu)", nil)
	w.optClosedB = w.option(0x06, w.vClosed, 1, "Không (dữ liệu mẫu)", nil)
	w.ballot(0x04, w.vClosed, w.optClosedA, w.binh, "2030-06-01T00:00:00Z", "2030-06-01T00:00:00Z")
	w.vG2 = w.vote(0x04, w.g2, w.hoa, "Nhóm hai (dữ liệu mẫu)", "2030-06-03T00:00:00Z", w.oBare, nil, nil)
	w.optG2 = w.option(0x07, w.vG2, 0, "Đi (dữ liệu mẫu)", nil)
	w.missingVote = fid(kindVote, 0xfe)
	w.missingOption = fid(kindVoteOption, 0xfe)
	return w
}
