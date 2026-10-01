package achievementv1

import (
	"context"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/motluot"
	"mobile/services/core/internal/domain/achievement"
)

// Nếp's line on the journey choices (ADR-0052: the model step moved from
// the brain into this process): only offered ids come back, only counts go
// out, and every way the model cannot answer is the Go fallback.
func TestGoiYThanhTuuQuaModel(t *testing.T) {
	f := achievement.Facts{Checkins: 5, DistinctDestinations: 2, PhotoDays: 4, StoryDays: 1, SharedOutings: 3}
	choices := []achievement.Choice{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}, {ID: "earned", Earned: true}}
	ok := llm.Buoc{Text: `{"candidate_ids":["x","c","c","a","earned"],"line":"  Bạn đã   check-in 5 lần, thử hướng mới nhé. "}`}
	stub := llm.NewStub(ok)
	ids, source, line := suggestions(context.Background(), motluot.Moi(stub, 1), f, choices, "dau_chan", []string{"dau_chan"})
	if source != "ai" || strings.Join(ids, ",") != "c,a" || line != "Bạn đã check-in 5 lần, thử hướng mới nhé." {
		t.Fatalf("%v %s %q", ids, source, line)
	}
	req := string(stub.YeuCau()[0])
	if !strings.Contains(req, `\"checkins\": 5`) || strings.Contains(req, "earned") || strings.Contains(req, "story_count") {
		t.Fatalf("the prompt carried more than the counts and the offered ids: %s", req)
	}
	for name, c := range map[string]struct {
		may     *motluot.May
		history []string
	}{
		"không khoá":         {nil, nil},
		"lịch sử lạ":         {motluot.Moi(llm.NewStub(ok), 1), []string{"bay"}},
		"model hỏng":         {motluot.Moi(llm.NewStub(llm.Buoc{Text: "không phải JSON"}), 1), nil},
		"không mã hợp lệ":    {motluot.Moi(llm.NewStub(llm.Buoc{Text: `{"candidate_ids":["x"],"line":"ổn"}`}), 1), nil},
		"câu có đường dẫn":   {motluot.Moi(llm.NewStub(llm.Buoc{Text: `{"candidate_ids":["a"],"line":"xem http://x"}`}), 1), nil},
		"câu dài quá 180":    {motluot.Moi(llm.NewStub(llm.Buoc{Text: `{"candidate_ids":["a"],"line":"` + strings.Repeat("a ", 91) + `"}`}), 1), nil},
		"mã không phải mảng": {motluot.Moi(llm.NewStub(llm.Buoc{Text: `{"candidate_ids":"a","line":"ổn"}`}), 1), nil},
	} {
		ids, source, _ := suggestions(context.Background(), c.may, f, choices, "", c.history)
		if source != "go" || strings.Join(ids, ",") != "a,b,c" {
			t.Errorf("%s: %v %s", name, ids, source)
		}
	}
}
