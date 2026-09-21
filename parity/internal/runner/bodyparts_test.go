package runner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"mobile/parity/internal/scenario"
)

func text(s string) *string { return &s }

func TestMultipartBodyLayout(t *testing.T) {
	parts := []scenario.Part{
		{Name: "file", Filename: text("{{who}}.txt"), ContentType: text("text/plain"), Text: text("xin chào {{who}}")},
		{Name: "note", Text: text("")},
		{Name: "empty", Filename: text(""), Text: text("")},
	}
	got, err := multipartBody("b0undary", parts, map[string]string{"who": "p1"})
	if err != nil {
		t.Fatal(err)
	}
	want := "--b0undary\r\n" +
		"Content-Disposition: form-data; name=\"file\"; filename=\"p1.txt\"\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"xin chào p1\r\n" +
		"--b0undary\r\n" +
		"Content-Disposition: form-data; name=\"note\"\r\n" +
		"\r\n" +
		"\r\n" +
		"--b0undary\r\n" +
		"Content-Disposition: form-data; name=\"empty\"; filename=\"\"\r\n" +
		"\r\n" +
		"\r\n" +
		"--b0undary--\r\n"
	if string(got) != want {
		t.Fatalf("body:\n%q\nwant:\n%q", got, want)
	}
	if none, _ := multipartBody("b", nil, nil); string(none) != "--b--\r\n" {
		t.Fatalf("no parts: %q", none)
	}
}

const uploadScript = `
id: fake/upload
routes: ["POST /people/me/photos"]
auth_mode: dev
personas: {owner: {}}
steps:
  - id: upload
    as: owner
    request:
      method: POST
      path: /people/me/photos
      headers: {content-type: 'multipart/form-data; boundary="q b"'}
      body_parts:
        - name: file
          filename: mau.png
          content_type: image/png
          image: {format: png, width: 12, height: 9, seed: 4, alpha: true, orientation: 6}
        - name: caption
          text: 'của {{persona.owner}}'
`

// A generated body reaches both stacks as the same bytes, parses as the
// multipart the header announces, and carries GenerateImage's file.
func TestBodyPartsReachBothStacksAsTheSameBytes(t *testing.T) {
	sc, err := scenario.Parse([]byte(uploadScript))
	if err != nil {
		t.Fatal(err)
	}
	wantFile := mustGenerate(t, *(*sc.Steps[0].Request.BodyParts)[0].Image)
	var mu sync.Mutex
	digests := map[string][32]byte{}
	server := func(name string) *httptest.Server {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			digests[name] = sha256.Sum256(body)
			mu.Unlock()
			mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || mediaType != "multipart/form-data" {
				w.WriteHeader(400)
				return
			}
			reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
			file, err := reader.NextPart()
			if err != nil || file.FormName() != "file" || file.FileName() != "mau.png" || file.Header.Get("Content-Type") != "image/png" {
				w.WriteHeader(422)
				return
			}
			data, _ := io.ReadAll(file)
			caption, err := reader.NextPart()
			if err != nil || caption.FileName() != "" {
				w.WriteHeader(422)
				return
			}
			captionText, _ := io.ReadAll(caption)
			if _, err := reader.NextPart(); err != io.EOF {
				w.WriteHeader(422)
				return
			}
			if !bytes.Equal(data, wantFile) || string(captionText) != "của "+r.Header.Get("X-Actor-ID") {
				w.WriteHeader(409)
				return
			}
			w.WriteHeader(201)
		}))
		t.Cleanup(s.Close)
		return s
	}
	nonce := NewNonce()
	ref, err := Execute(context.Background(), sc, stack(t, "reference", server("reference")), nonce)
	if err != nil {
		t.Fatal(err)
	}
	cand, err := Execute(context.Background(), sc, stack(t, "candidate", server("candidate")), nonce)
	if err != nil {
		t.Fatal(err)
	}
	if ref.Steps[0].Raw.Status != 201 {
		t.Fatalf("the fake API refused the generated body: %d", ref.Steps[0].Raw.Status)
	}
	if diffs := Diff(ref, cand); len(diffs) != 0 {
		t.Fatalf("diffs: %+v", diffs)
	}
	if digests["reference"] != digests["candidate"] {
		t.Fatal("the two stacks were sent different bytes")
	}
}
