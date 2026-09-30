package congdong

import (
	"context"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/motluot"
)

func TestDuyetDocDungHinh(t *testing.T) {
	doc := func(answer string, media ...Media) (Doc, error) {
		return Duyet(context.Background(), motluot.Moi(llm.NewStub(llm.Buoc{Text: answer}), 1).Luot(1), "Hồ buổi sáng", false, media)
	}
	d, err := doc("```json\n{\"relevant\":true,\"safe\":false,\"confidence_milli\":950,\"reason\":\" " + strings.Repeat("x", 300) + "\"}\n```")
	if err != nil || !d.Relevant || d.Safe || d.Confidence != 950 || len(d.Reason) != 200 || d.MediaChecked {
		t.Fatalf("%+v %v", d, err)
	}
	for _, bad := range []string{
		`{"relevant":true,"safe":true,"confidence_milli":950.5,"reason":"ok"}`,
		`{"relevant":"yes","safe":true,"confidence_milli":950,"reason":"ok"}`,
		`{"safe":true,"confidence_milli":950,"reason":"ok"}`,
		`[true]`,
	} {
		if _, err := doc(bad); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
	ok := `{"relevant":true,"safe":true,"confidence_milli":950,"reason":"ok"}`
	if d, _ := doc(ok, Media{MIME: "image/png", Data: []byte("x")}); !d.MediaChecked {
		t.Fatal("a sent image was not counted as checked")
	}
	if d, _ := doc(ok, Media{MIME: "image/png", Data: []byte("x")}, Media{MIME: "video/mp4"}); d.MediaChecked {
		t.Fatal("a video was counted as checked")
	}
	big := make([]byte, MaxByteAnh)
	if d, _ := doc(ok, Media{MIME: "image/png", Data: big}, Media{MIME: "image/png", Data: []byte("x")}); d.MediaChecked {
		t.Fatal("an image over the cap was counted as checked")
	}
}
