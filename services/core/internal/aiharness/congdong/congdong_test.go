package congdong

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/motluot"
)

const okDoc = `{"relevant":true,"safe":true,"confidence_milli":950,"reason":"ok"}`

func duyet(t *testing.T, media []Media, answers ...string) (Doc, *llm.Stub, error) {
	t.Helper()
	steps := []llm.Buoc{}
	for _, a := range answers {
		steps = append(steps, llm.Buoc{Text: a})
	}
	stub := llm.NewStub(steps...)
	d, err := Duyet(context.Background(), motluot.Moi(stub, 1).Luot, "Hồ buổi sáng", false, media)
	return d, stub, err
}

func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, w, h))
	im.Set(1, 1, color.NRGBA{R: 100, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// noiseJPEG is a photograph's worst case for size: every pixel different.
func noiseJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewPCG(1, 2))
	for i := range im.Pix {
		im.Pix[i] = uint8(r.IntN(256))
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, im, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// sent is every inline part of a recorded request, as MIME type and the
// decoded image's longer side (zero for a part that is not an image), and
// the request's text.
func sent(t *testing.T, req []byte) (mimes []string, sides []int, text string) {
	t.Helper()
	var r struct {
		Contents []struct {
			Parts []struct {
				Text       string `json:"text"`
				InlineData *struct {
					MIMEType string `json:"mimeType"`
					Data     string `json:"data"`
				} `json:"inlineData"`
			} `json:"parts"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(req, &r); err != nil {
		t.Fatal(err)
	}
	for _, p := range r.Contents[0].Parts {
		if p.InlineData == nil {
			text += p.Text
			continue
		}
		mimes = append(mimes, p.InlineData.MIMEType)
		raw, err := base64.StdEncoding.DecodeString(p.InlineData.Data)
		if err != nil {
			t.Fatal(err)
		}
		side := 0
		if c, _, err := image.DecodeConfig(bytes.NewReader(raw)); err == nil {
			side = max(c.Width, c.Height)
		}
		sides = append(sides, side)
	}
	return mimes, sides, text
}

func TestDuyetDocDungHinh(t *testing.T) {
	d, _, err := duyet(t, nil, "```json\n{\"relevant\":true,\"safe\":false,\"confidence_milli\":950,\"reason\":\" "+strings.Repeat("x", 300)+"\"}\n```")
	if err != nil || !d.Relevant || d.Safe || d.Confidence != 950 || len(d.Reason) != 200 || d.MediaChecked {
		t.Fatalf("%+v %v", d, err)
	}
	for _, bad := range []string{
		`{"relevant":true,"safe":true,"confidence_milli":950.5,"reason":"ok"}`,
		`{"relevant":"yes","safe":true,"confidence_milli":950,"reason":"ok"}`,
		`{"safe":true,"confidence_milli":950,"reason":"ok"}`,
		`[true]`,
	} {
		if _, _, err := duyet(t, nil, bad); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}

func TestDuyetGuiMoiAnhDaThuNho(t *testing.T) {
	d, stub, err := duyet(t, []Media{{MIME: "image/png", Data: pngOf(t, 20, 20)}}, okDoc)
	if err != nil || !d.MediaChecked {
		t.Fatalf("a sent image was not counted as checked: %+v %v", d, err)
	}
	if mimes, sides, _ := sent(t, stub.YeuCau()[0]); len(mimes) != 1 || mimes[0] != "image/jpeg" || sides[0] != 20 {
		t.Fatalf("a small image is not sent at its own size: %v %v", mimes, sides)
	}
	// A post's ten photographs go at the first side even at their worst
	// (noise, ~1.4 MB each); a shared diary's dozen does not fit there and
	// still all go, at the largest side that fits.
	photo := noiseJPEG(t, 2000, 2000)
	for _, c := range []struct{ n, side int }{{10, canhAnh[0]}, {12, canhAnh[1]}} {
		many := []Media{}
		for range c.n {
			many = append(many, Media{MIME: "image/jpeg", Data: photo})
		}
		d, stub, err = duyet(t, many, okDoc)
		if err != nil || !d.MediaChecked || stub.SoGoi() != 1 {
			t.Fatalf("%d photographs: %+v %v calls %d", c.n, d, err, stub.SoGoi())
		}
		mimes, sides, _ := sent(t, stub.YeuCau()[0])
		if len(mimes) != c.n || sides[0] != c.side || sides[c.n-1] != c.side {
			t.Fatalf("%d photographs sent as %d parts of side %v", c.n, len(mimes), sides)
		}
	}
	// An image that does not decode cannot be read: the text goes alone.
	d, stub, _ = duyet(t, []Media{{MIME: "image/png", Data: pngOf(t, 4, 4)}, {MIME: "image/png", Data: []byte("x")}}, okDoc)
	if mimes, _, _ := sent(t, stub.YeuCau()[0]); d.MediaChecked || len(mimes) != 0 {
		t.Fatalf("an unreadable image was vouched for: %+v %v", d, mimes)
	}
}

func TestThuNhoGiuHinhVaNenTrang(t *testing.T) {
	// A 64x32 picture, left half red, right half fully transparent: halved,
	// it is red on the left and white on the right.
	im := image.NewNRGBA(image.Rect(0, 0, 64, 32))
	for y := range 32 {
		for x := range 32 {
			im.Set(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	b, err := thuNho(im, 32)
	if err != nil {
		t.Fatal(err)
	}
	out, err := jpeg.Decode(bytes.NewReader(b))
	if err != nil || out.Bounds().Dx() != 32 || out.Bounds().Dy() != 16 {
		t.Fatalf("%v %v", out.Bounds(), err)
	}
	near := func(c color.Color, r, g, bl int) bool {
		cr, cg, cb, _ := c.RGBA()
		d := func(a uint32, b int) bool { return abs(int(a>>8)-b) < 40 }
		return d(cr, r) && d(cg, g) && d(cb, bl)
	}
	if !near(out.At(4, 8), 255, 0, 0) || !near(out.At(27, 8), 255, 255, 255) {
		t.Fatalf("left %v right %v", out.At(4, 8), out.At(27, 8))
	}
}

func TestDuyetDocVideoTungDoan(t *testing.T) {
	cut := Media{MIME: "video/mp4", Doan: [][]byte{[]byte("p1"), []byte("p2"), []byte("p3")}}
	d, stub, err := duyet(t, []Media{cut},
		`{"relevant":false,"safe":true,"confidence_milli":990,"reason":"ok"}`,
		`{"relevant":true,"safe":false,"confidence_milli":950,"reason":"nudity"}`,
		`{"relevant":false,"safe":true,"confidence_milli":930,"reason":"unclear"}`)
	if err != nil || stub.SoGoi() != 3 {
		t.Fatalf("%v, %d calls", err, stub.SoGoi())
	}
	if !d.MediaChecked || !d.Relevant || d.Safe || d.Confidence != 930 || d.Reason != "nudity" {
		t.Fatalf("joined reading: %+v", d)
	}
	for i, req := range stub.YeuCau() {
		mimes, _, text := sent(t, req)
		if len(mimes) != 1 || mimes[0] != "video/mp4" || !strings.Contains(text, "Hồ buổi sáng") ||
			!strings.Contains(text, fmt.Sprintf(`"piece":%d`, i+1)) || !strings.Contains(text, fmt.Sprintf(`"starts_at_second":%d`, i*GiayDoan)) {
			t.Fatalf("piece %d: %v %s", i+1, mimes, text)
		}
	}
	// A video without its cut, or with more pieces than a video has, is
	// never sent: one call with the text alone, never vouched for.
	for _, v := range []Media{{MIME: "video/mp4"}, {MIME: "video/mp4", Doan: make([][]byte, MaxDoan+1)}} {
		d, stub, err = duyet(t, []Media{v}, okDoc)
		if mimes, _, _ := sent(t, stub.YeuCau()[0]); err != nil || d.MediaChecked || stub.SoGoi() != 1 || len(mimes) != 0 {
			t.Fatalf("unsendable video: %+v %v %d %v", d, err, stub.SoGoi(), mimes)
		}
	}
	// A piece that fails fails the reading: the job retries, nothing is
	// decided on the pieces that answered.
	stub = llm.NewStub(llm.Buoc{Text: okDoc}, llm.Buoc{Loi: errors.New("down")}, llm.Buoc{Loi: errors.New("down")})
	may := motluot.Moi(stub, 1).WithWait(func(int) time.Duration { return 0 })
	if _, err = Duyet(context.Background(), may.Luot, "x", false, []Media{cut}); err == nil {
		t.Fatal("a failed piece was ignored")
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
