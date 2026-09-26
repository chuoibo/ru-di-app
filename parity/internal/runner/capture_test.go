package runner

import (
	"net/http"
	"strings"
	"testing"

	"mobile/parity/internal/httpclient"
	"mobile/parity/internal/normalize"
	"mobile/parity/internal/scenario"
)

// A bind decides what a later step SENDS. It must not also decide what the
// comparator SEES -- unless the value is an id, where masking is the point.
//
// This is the property MOUNT /static depends on: each side is sent the ETag it
// issued itself, while both ETags stay visible to the comparator. A masked
// validator could never be seen to differ, which is the same shape of blindness
// the <digest#n> placeholder once gave fingerprint drift.
func TestAnEchoBindSendsTheValueWithoutHidingItFromTheComparator(t *testing.T) {
	const etag = `"643656382d0b4df1c31428b69735d3ec"`

	capturedWith := func(class string) (map[string]string, *normalize.Binder) {
		step := scenario.Step{ID: "s", Bind: map[string]scenario.Bind{
			"css_etag": {From: "header", Name: "etag", Class: class},
		}}
		resp := httpclient.Response{Status: 200, Header: http.Header{"Etag": []string{etag}}}
		vars, binder := map[string]string{}, normalize.NewBinder()
		if err := capture(step, resp, vars, binder); err != nil {
			t.Fatalf("class %q: %v", class, err)
		}
		return vars, binder
	}

	// echo: available to later steps, and still itself when compared.
	vars, binder := capturedWith("echo")
	if vars["css_etag"] != etag {
		t.Fatalf("echo did not bind the value: %q", vars["css_etag"])
	}
	body := `{"validator":` + etag + `}`
	if got := binder.Apply(body); got != body {
		t.Errorf("echo masked the value:\n got %s\nwant %s", got, body)
	}

	// token: bound AND masked, which is right for a credential and wrong here.
	vars, binder = capturedWith("token")
	if vars["css_etag"] != etag {
		t.Fatalf("token did not bind the value: %q", vars["css_etag"])
	}
	if got := binder.Apply(body); got == body || !strings.Contains(got, "token:css_etag") {
		t.Errorf("token should have masked the value, got %s", got)
	}
}

// The mask that hid the ETag, kept red on purpose.
//
// The binder numbers any 32-hex run <hex32#n> per stack, so Python's
// mtime-derived etag and Go's content-derived etag BOTH became <hex32#1> in
// their own runs and compared equal. Measured on real stacks before the fix:
// 10 /static steps differed on last-modified and NONE on etag, while the two
// etags on the wire were 643656382d0b4df1c31428b69735d3ec (Python) and
// 397aab04ee043894e776a8a6a2605f7e (Go).
func TestTheETagSurvivesNormalisationSoItIsActuallyCompared(t *testing.T) {
	const pythonETag = `"643656382d0b4df1c31428b69735d3ec"`
	const goETag = `"397aab04ee043894e776a8a6a2605f7e"`

	normalised := func(etag string) string {
		binder := normalize.NewBinder()
		// The runner observes every header value before normalising, which is
		// what registered the digest as a key in the first place.
		if err := binder.Observe(etag); err != nil {
			t.Fatal(err)
		}
		resp := httpclient.Response{Status: 200, Header: http.Header{
			"Etag":         []string{etag},
			"Content-Type": []string{"text/css; charset=utf-8"},
		}}
		return normaliseExchange(binder, "/static/guest.css", resp).Header.Get("Etag")
	}

	python, golang := normalised(pythonETag), normalised(goETag)
	if python != pythonETag || golang != goETag {
		t.Fatalf("an etag was masked: python %s, go %s", python, golang)
	}
	if python == golang {
		t.Fatal("two different etags normalised to the same value; the comparison is blind")
	}
}
