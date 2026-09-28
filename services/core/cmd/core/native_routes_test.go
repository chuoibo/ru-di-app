package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"mobile/services/core/internal/achievementv1"
	"mobile/services/core/internal/profilemedia"
	"mobile/services/core/internal/socialv2"
)

func TestRoutesCommandListsEveryGoNativeExtension(t *testing.T) {
	var out, errorOut bytes.Buffer
	if code := listRoutes([]string{"--json"}, &out, &errorOut); code != 0 {
		t.Fatalf("route listing failed: %s", errorOut.String())
	}
	var rows []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, row := range rows {
		listed[row.ID] = true
	}
	for _, id := range append(append(achievementv1.RouteIDs(), socialv2.RouteIDs()...), profilemedia.RouteIDs()...) {
		if !listed[id] {
			t.Fatalf("Go-only handler %q absent from routes --json", id)
		}
	}
}
