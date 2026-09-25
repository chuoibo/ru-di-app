package chatassist

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The one property that has to survive everything else in this package.
//
// The whole design rests on a single claim: the server does not read the
// conversation, the caller hands over what it chose to share. That claim is
// what lets the same code keep working after the cutover, when chat v2 is end
// to end encrypted and there is no body for the server to read even if it
// wanted one.
//
// A claim like that decays quietly. Someone fixing a quality complaint reaches
// for `SELECT body FROM messages` because it is right there and it obviously
// improves the answer, every test stays green, and the property is gone with
// nobody noticing until the cutover fails. So it is checked here rather than
// promised in a comment.
//
// The package IS allowed to touch `messages`: `thuocPhong` reads `id` and
// `author_id` to confirm a shared turn is a real message of this room, and
// publication writes an `ai_card` row. What it may never do is read the text.
func TestGoiBoiCanhKhongBaoGioDocNoiDungTinNhan(t *testing.T) {
	duong, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	// A SQL literal mentioning the messages table in a reading position.
	docBang := regexp.MustCompile(`(?is)select\b[^;]*?\bfrom\s+messages\b`)
	// A scan that looked at nothing reads exactly like a scan that found
	// nothing. Count what was actually examined, and require that the one
	// legitimate read of `messages` in this package was among it.
	daDoc, daThay := 0, 0
	// The reads added for the in-thread answer (ADR-0039): the trigger check
	// (it is the only one naming deleted_at) and publish's lock on the trigger.
	// Each must be among what the gate examined, or a later edit to either is
	// a read nobody checks.
	thayTinTag, thayGiuTag := false, false
	for _, f := range duong {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		daDoc++
		for _, cau := range docBang.FindAllString(string(raw), -1) {
			daThay++
			if strings.Contains(cau, "deleted_at") && strings.Contains(cau, "author_id") {
				thayTinTag = true
			}
			if cau == "SELECT id FROM messages" {
				thayGiuTag = true
			}
			// `body` is the message text. Naming it in a read of `messages` is
			// the server reading the conversation, which is the thing this
			// package exists to not do.
			if regexp.MustCompile(`(?i)\bbody\b`).MatchString(cau) {
				t.Errorf("%s đọc nội dung tin nhắn:\n%s", f, cau)
			}
		}
	}
	if daDoc < 5 {
		t.Fatalf("chỉ quét được %d file nguồn; cổng đang nhìn vào chỗ trống", daDoc)
	}
	if daThay == 0 {
		t.Fatal("không thấy câu đọc `messages` nào, mà kiểm quyền sở hữu bối cảnh có đúng một câu như thế; mẫu đã trượt")
	}
	if !thayTinTag || !thayGiuTag {
		t.Fatalf("cổng không thấy câu đọc tin tag (kiemTrigger=%v, giuTrigger=%v); câu đọc mới nhất đang nằm ngoài cổng", thayTinTag, thayGiuTag)
	}
}

// The guard above is only worth having if it can fail. A regexp that matches
// nothing reads exactly like a regexp that found nothing wrong.
func TestCongNayThucSuDoDuoc(t *testing.T) {
	docBang := regexp.MustCompile(`(?is)select\b[^;]*?\bfrom\s+messages\b`)
	xau := "tx.Query(ctx, `SELECT id, author_id, body FROM messages WHERE context_id=$1`)"
	cau := docBang.FindString(xau)
	if cau == "" {
		t.Fatal("mẫu không bắt được một câu đọc messages ngay trước mắt")
	}
	if !regexp.MustCompile(`(?i)\bbody\b`).MatchString(cau) {
		t.Fatal("bắt được câu nhưng không thấy cột nội dung trong đó")
	}
	lanh := "tx.Query(ctx, `SELECT id, author_id FROM messages WHERE context_id=$1`)"
	if regexp.MustCompile(`(?i)\bbody\b`).MatchString(docBang.FindString(lanh)) {
		t.Fatal("câu chỉ đọc id và tác giả bị coi là đọc nội dung")
	}
}
