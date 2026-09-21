package imageopen

import (
	"bytes"
	"errors"
	"testing"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

type sized struct{ w, h int }

func (s sized) Size() (int, int)             { return s.w, s.h }
func (s sized) Load() (*pil.Image, error)    { return nil, errors.New("not loaded") }
func refuse(data []byte) (pil.Opened, error) { return nil, pil.Next("refused") }

// registry answers "not accepted" for every plugin, so a test only sets
// the entries it exercises.
func registry(overrides map[string]pil.Plugin) map[string]pil.Plugin {
	plugins := map[string]pil.Plugin{}
	for _, name := range Order {
		plugins[name] = pil.Plugin{Name: name, Accept: func([]byte) bool { return false }, Open: refuse}
	}
	for name, plugin := range overrides {
		plugins[name] = plugin
	}
	return plugins
}

func TestOrderHasEveryPluginOnce(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range Order {
		if seen[name] {
			t.Fatalf("%s listed twice", name)
		}
		seen[name] = true
	}
	if len(Order) != 43 {
		t.Fatalf("Order has %d plugins, Image.ID had 43", len(Order))
	}
}

func TestFirstAcceptingPluginThatOpensWins(t *testing.T) {
	var tried []string
	track := func(name string, result pil.Opened, err error) pil.Plugin {
		return pil.Plugin{Name: name, Open: func([]byte) (pil.Opened, error) {
			tried = append(tried, name)
			return result, err
		}}
	}
	plugins := registry(map[string]pil.Plugin{
		"PNG":  track("PNG", nil, pil.Next("bad header")),
		"TGA":  track("TGA", sized{3, 2}, nil),
		"WEBP": track("WEBP", sized{9, 9}, nil),
	})
	name, opened, err := Open([]byte("anything"), plugins)
	if err != nil || name != "TGA" {
		t.Fatalf("got %q, %v", name, err)
	}
	if w, h := opened.Size(); w != 3 || h != 2 {
		t.Fatalf("size %dx%d", w, h)
	}
	if want := []string{"PNG", "TGA"}; len(tried) != 2 || tried[0] != want[0] || tried[1] != want[1] {
		t.Fatalf("tried %v, want %v", tried, want)
	}
}

func TestAcceptSeesSixteenBytePrefix(t *testing.T) {
	raw := bytes.Repeat([]byte{7}, 40)
	var got int
	plugins := registry(map[string]pil.Plugin{
		"BMP": {Name: "BMP", Accept: func(prefix []byte) bool { got = len(prefix); return false }, Open: refuse},
	})
	if _, _, err := Open(raw, plugins); !errors.Is(err, ErrUnidentified) {
		t.Fatalf("err %v", err)
	}
	if got != 16 {
		t.Fatalf("accept saw %d bytes", got)
	}
	if _, _, err := Open(raw[:5], plugins); !errors.Is(err, ErrUnidentified) || got != 5 {
		t.Fatalf("short input: accept saw %d bytes, err %v", got, err)
	}
}

func TestEscapingExceptionStopsTheLoop(t *testing.T) {
	boom := errors.New("ValueError")
	plugins := registry(map[string]pil.Plugin{
		"GIF": {Name: "GIF", Open: func([]byte) (pil.Opened, error) { return nil, boom }},
		"PNG": {Name: "PNG", Open: func([]byte) (pil.Opened, error) { return sized{1, 1}, nil }},
	})
	name, _, err := Open([]byte("x"), plugins)
	if !errors.Is(err, boom) || name != "GIF" {
		t.Fatalf("got %q, %v", name, err)
	}
}

func TestBombCheckUsesPillowLimits(t *testing.T) {
	cases := []struct {
		w, h    int
		want    bool
		warning bool
	}{
		{pil.MaxImagePixels, 1, false, false},
		{pil.MaxImagePixels + 1, 1, true, true},
		{2 * pil.MaxImagePixels, 1, true, true},
		{2*pil.MaxImagePixels + 1, 1, true, false},
		{0, 2*pil.MaxImagePixels + 1, false, false}, // size <= 0 is refused before the bomb check
	}
	for _, c := range cases {
		plugins := registry(map[string]pil.Plugin{
			"PPM": {Name: "PPM", Open: func([]byte) (pil.Opened, error) { return sized{c.w, c.h}, nil }},
		})
		_, _, err := Open([]byte("P6"), plugins)
		var bomb *pil.BombError
		if errors.As(err, &bomb) != c.want || (c.want && bomb.Warning != c.warning) {
			t.Fatalf("%dx%d: err %v", c.w, c.h, err)
		}
	}
}

func TestZeroSizeTriesNextPlugin(t *testing.T) {
	plugins := registry(map[string]pil.Plugin{
		"GIF": {Name: "GIF", Open: func([]byte) (pil.Opened, error) { return sized{0, 5}, nil }},
		"PNG": {Name: "PNG", Open: func([]byte) (pil.Opened, error) { return sized{2, 2}, nil }},
	})
	name, _, err := Open([]byte("x"), plugins)
	if err != nil || name != "PNG" {
		t.Fatalf("got %q, %v", name, err)
	}
}
