package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"mobile/services/core/ownership"
)

// The manifest's `features` block and the feature handlers' muxes must name the
// same routes in the same order. A route registered without a row, or a row
// left behind after its route was deleted, is a front-door route nobody can
// see in the one file that answers "which process serves this".
func TestManifestFeaturesMatchRegisteredRoutes(t *testing.T) {
	manifest, err := ownership.Load()
	if err != nil {
		t.Fatal(err)
	}
	var fromManifest []featureView
	for _, f := range manifest.Features {
		fromManifest = append(fromManifest, featureView{ID: f.ID, Package: f.Package})
	}
	if got := featureRoutes(); !reflect.DeepEqual(fromManifest, got) {
		t.Fatalf("manifest features drifted from the registered routes\nmanifest: %v\nbinary:   %v", fromManifest, got)
	}
	if len(fromManifest) == 0 {
		t.Fatal("no feature rows: the check would pass while comparing nothing")
	}
}

func TestFeaturesCommandPrintsTheRegisteredRoutes(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"features", "--json"}, func(string) string { return "" }, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	var views []featureView
	if err := json.Unmarshal(out.Bytes(), &views); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(views, featureRoutes()) {
		t.Fatalf("printed %v", views)
	}
	if code := run([]string{"features"}, func(string) string { return "" }, &out, &errOut); code != 2 {
		t.Fatalf("missing --json: exit %d, want 2", code)
	}
}
