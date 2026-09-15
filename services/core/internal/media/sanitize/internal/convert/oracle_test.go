//go:build oracle

package convert_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"mobile/services/core/internal/media/sanitize/internal/pil"
	pt "mobile/services/core/internal/media/sanitize/internal/pngdec/pngtest"
	"mobile/services/core/internal/media/sanitize/internal/pyoracle"
)

// convertScript builds each image with Image.frombytes, putpalette and
// info["transparency"], then runs the sanitizer's alpha check and convert.
// The shared oracle script has no op for in-memory modes that no ported
// decoder produces, so this test carries its own few lines.
const convertScript = `
import base64, hashlib, json, sys
from PIL import Image
for line in sys.stdin:
    req = json.loads(line)
    out = {"id": req["id"]}
    try:
        im = Image.frombytes(req["mode"], (req["w"], req["h"]), base64.b64decode(req["b64"]))
        if req.get("palette_mode"):
            im.putpalette(base64.b64decode(req["palette"]), req["palette_mode"])
        t = req.get("transparency")
        if t is not None:
            if "int" in t:
                im.info["transparency"] = t["int"]
            elif "bytes" in t:
                im.info["transparency"] = base64.b64decode(t["bytes"])
            elif "tuple" in t:
                im.info["transparency"] = tuple(t["tuple"])
            else:
                im.info["transparency"] = t["str"]
        has_alpha = "A" in im.getbands() or (im.mode == "P" and "transparency" in im.info)
        mode = "RGBA" if has_alpha else "RGB"
        pixels = im.convert(mode).tobytes()
        out.update(result="ok", mode=mode, sha=hashlib.sha256(pixels).hexdigest())
    except Exception as exc:
        out.update(result="error", type=type(exc).__name__, message=str(exc))
    print(json.dumps(out), flush=True)
`

type convertRequest struct {
	ID           string         `json:"id"`
	Mode         string         `json:"mode"`
	W            int            `json:"w"`
	H            int            `json:"h"`
	Data         []byte         `json:"b64"`
	PaletteMode  string         `json:"palette_mode,omitempty"`
	Palette      []byte         `json:"palette,omitempty"`
	Transparency map[string]any `json:"transparency,omitempty"`
}

type convertAnswer struct {
	ID      string `json:"id"`
	Result  string `json:"result"`
	Mode    string `json:"mode"`
	SHA     string `json:"sha"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

func runConvertOracle(t *testing.T, cases []convertCase) []convertAnswer {
	t.Helper()
	var stdin bytes.Buffer
	enc := json.NewEncoder(&stdin)
	for _, c := range cases {
		req := convertRequest{ID: c.name, Mode: c.img.Mode, W: c.img.Width, H: c.img.Height,
			Data: pt.ToBytes(c.img), PaletteMode: c.img.PaletteMode, Palette: c.img.Palette}
		if tr := c.img.Info.Transparency; tr != nil {
			switch tr.Kind {
			case pil.TransparencyInt:
				req.Transparency = map[string]any{"int": tr.Int}
			case pil.TransparencyBytes:
				req.Transparency = map[string]any{"bytes": tr.Bytes}
			case pil.TransparencyRGB:
				req.Transparency = map[string]any{"tuple": tr.RGB[:]}
			}
		}
		if c.strTransparency != "" {
			req.Transparency = map[string]any{"str": c.strTransparency}
		}
		if err := enc.Encode(req); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("docker", "run", "--rm", "-i", "--network", "none", "--entrypoint", "python", pyoracle.Image(), "-c", convertScript)
	cmd.Stdin = &stdin
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("convert oracle: %v\n%s", err, stderr.String())
	}
	var answers []convertAnswer
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 1<<20), 1<<26)
	for scanner.Scan() {
		var a convertAnswer
		if err := json.Unmarshal(scanner.Bytes(), &a); err != nil {
			t.Fatal(err)
		}
		answers = append(answers, a)
	}
	if len(answers) != len(cases) {
		t.Fatalf("convert oracle answered %d of %d\n%s", len(answers), len(cases), stderr.String())
	}
	return answers
}

// TestOracle compares the alpha check and conversion for every in-memory
// mode with Pillow.
func TestOracle(t *testing.T) {
	cases := corpus()
	answers := runConvertOracle(t, cases)
	same, unported := 0, 0
	golden := map[string]string{}
	for i, c := range cases {
		a := answers[i]
		mode, digest, err := goConvert(c)
		var unsupported *pil.UnsupportedError
		switch {
		case errors.As(err, &unsupported):
			unported++
			t.Logf("%s: unported (%v); Pillow %s %s %s", c.name, err, a.Result, a.Mode, a.Type)
		case a.Result == "error" && err != nil:
			same++
			golden[c.name] = "error"
		case a.Result == "ok" && err == nil && a.Mode == mode && a.SHA == digest:
			same++
			golden[c.name] = mode + ":" + digest[:16]
		default:
			t.Errorf("%s: go %s %.16s %v; Pillow %s %s %.16s %s: %s", c.name, mode, digest, err, a.Result, a.Mode, a.SHA, a.Type, a.Message)
		}
	}
	t.Log(fmt.Sprintf("cases %d identical %d unported %d", len(cases), same, unported))
	if !t.Failed() && os.Getenv("MEDIA_GOLDEN_UPDATE") == "1" {
		writeGolden(t, golden)
	}
}
