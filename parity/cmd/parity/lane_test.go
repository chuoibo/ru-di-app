package main

import (
	"io"
	"testing"

	"mobile/parity/internal/scenario"
)

func TestFilterLaneKeepsOneLane(t *testing.T) {
	all := []*scenario.Scenario{{ID: "a"}, {ID: "b", Lane: scenario.LaneLimiter}, {ID: "c"}}
	main, err := filterLane(all, "main", io.Discard)
	if err != nil || len(main) != 2 || main[0].ID != "a" || main[1].ID != "c" {
		t.Fatalf("main = %v, %v", main, err)
	}
	limiter, err := filterLane(all, scenario.LaneLimiter, io.Discard)
	if err != nil || len(limiter) != 1 || limiter[0].ID != "b" {
		t.Fatalf("limiter = %v, %v", limiter, err)
	}
	if _, err := filterLane(all[:1], scenario.LaneLimiter, io.Discard); err == nil {
		t.Fatal("an empty lane was accepted")
	}
	if _, err := filterLane(all, "limit", io.Discard); err == nil {
		t.Fatal("an unknown lane was accepted")
	}
}
