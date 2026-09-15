package scenario

import (
	"strings"
	"testing"
)

const withParts = `
id: photos/upload
routes: ["POST /people/me/photos"]
auth_mode: dev
personas: {owner: {}}
steps:
  - id: upload
    as: owner
    request:
      method: POST
      path: /people/me/photos
      headers: {content-type: 'multipart/form-data; boundary=parityboundary'}
      body_parts:
        - name: file
          filename: mau.jpg
          content_type: image/jpeg
          image: {format: jpeg, width: 64, height: 48, seed: 1, orientation: 6, pad_to_bytes: 4096}
        - name: note
          text: 'ghi chú của {{persona.owner}}'
        - name: empty
          filename: ''
          text: ''
  - id: no_parts
    as: owner
    request:
      method: POST
      path: /people/me/photos
      headers: {Content-Type: multipart/form-data; boundary=parityboundary}
      body_parts: []
`

func TestBodyPartsLoad(t *testing.T) {
	sc, err := Parse([]byte(withParts))
	if err != nil {
		t.Fatal(err)
	}
	parts := *sc.Steps[0].Request.BodyParts
	if len(parts) != 3 || parts[0].Image.Orientation != 6 || *parts[2].Filename != "" || parts[1].Filename != nil {
		t.Fatalf("parts = %+v", parts)
	}
	if sc.Steps[1].Request.BodyParts == nil || len(*sc.Steps[1].Request.BodyParts) != 0 {
		t.Fatal("an empty body_parts list must load as present and empty")
	}
}

func TestBodyPartsRefusals(t *testing.T) {
	cases := map[string]struct{ from, to, want string }{
		"body_raw too":           {"      body_parts: []\n", "      body_parts: []\n      body_raw: 'x'\n", "one body"},
		"no content-type":        {"      headers: {Content-Type: multipart/form-data; boundary=parityboundary}\n", "", "content-type header"},
		"not multipart":          {"{Content-Type: multipart/form-data; boundary=parityboundary}", "{Content-Type: application/json}", "boundary"},
		"no boundary":            {"{Content-Type: multipart/form-data; boundary=parityboundary}", "{Content-Type: multipart/form-data}", "boundary"},
		"templated content-type": {"{Content-Type: multipart/form-data; boundary=parityboundary}", "{Content-Type: 'multipart/form-data; boundary={{persona.owner}}'}", "literal"},
		"text and image":         {"          text: 'ghi chú", "          image: {format: png, width: 1, height: 1}\n          text: 'ghi chú", "exactly one"},
		"neither":                {"          filename: ''\n          text: ''\n", "          filename: ''\n", "exactly one"},
		"no name":                {"        - name: note\n", "        - name: ''\n", "name"},
		"quote in name":          {"        - name: note\n", "        - name: 'no\"te'\n", "name"},
		"line break in filename": {"filename: mau.jpg", "filename: \"mau\\r\\n.jpg\"", "filename"},
		"unbound template":       {"{{persona.owner}}'", "{{nobody}}'", "not bound"},
		"unknown part field":     {"          content_type: image/jpeg\n", "          content_type: image/jpeg\n          charset: utf-8\n", "not found"},
		"unknown image field":    {"seed: 1,", "seed: 1, quality: 50,", "not found"},
		"format":                 {"format: jpeg", "format: tiff", "format"},
		"zero width":             {"width: 64", "width: 0", "width"},
		"too wide":               {"width: 64", "width: 4097", "width"},
		"alpha on jpeg":          {"seed: 1,", "seed: 1, alpha: true,", "alpha"},
		"gray alpha png":         {"format: jpeg", "format: png, color: gray, alpha: true", "alpha"},
		"palette jpeg":           {"format: jpeg", "format: jpeg, color: palette", "color"},
		"rgb gif":                {"format: jpeg", "format: gif, color: rgb", "color"},
		"orientation on gif":     {"format: jpeg", "format: gif", "orientation"},
		"orientation too big":    {"orientation: 6", "orientation: 256", "orientation"},
		"header claim on webp":   {"format: jpeg", "format: webp, header_width: 20000", "webp"},
		"header claim too big":   {"orientation: 6", "header_height: 65536", "header"},
		"pad past the cap":       {"pad_to_bytes: 4096", "pad_to_bytes: 16777217", "pad_to_bytes"},
		"pad shorter than cut":   {"pad_to_bytes: 4096", "pad_to_bytes: 10, truncate_at: 20", "pad_to_bytes"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			text := strings.Replace(withParts, tc.from, tc.to, 1)
			if text == withParts {
				t.Fatal("the mutation did not apply")
			}
			_, err := Parse([]byte(text))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestBoundary(t *testing.T) {
	for header, want := range map[string]string{
		"multipart/form-data; boundary=parityboundary":     "parityboundary",
		`Multipart/Form-Data; boundary="with space"`:       "with space",
		"multipart/form-data; charset=utf-8; boundary=AbC": "AbC",
	} {
		if got, err := Boundary(header); err != nil || got != want {
			t.Errorf("Boundary(%q) = %q, %v; want %q", header, got, err, want)
		}
	}
	for _, header := range []string{"", "application/json", "multipart/form-data", "multipart/mixed; boundary=x"} {
		if _, err := Boundary(header); err == nil {
			t.Errorf("Boundary(%q) accepted", header)
		}
	}
}
