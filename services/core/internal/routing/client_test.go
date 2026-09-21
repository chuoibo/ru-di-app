package routing

import (
	"testing"

	"mobile/services/core/internal/domain/itinerary"
)

func TestConfiguredWithoutEnvIsNil(t *testing.T) {
	t.Setenv("MOBILE_VALHALLA_URL", "")
	t.Setenv("MOBILE_ROUTING_GRAPH_VERSION", "")
	if Configured() != nil {
		t.Fatal("missing env must be a nil router")
	}
}

func TestConfiguredRejectsBadURL(t *testing.T) {
	t.Setenv("MOBILE_VALHALLA_URL", "not-a-url")
	t.Setenv("MOBILE_ROUTING_GRAPH_VERSION", "graph-1")
	if Configured() != nil {
		t.Fatal("an invalid URL must be a nil router")
	}
}

func TestConfiguredRejectsBlankVersion(t *testing.T) {
	t.Setenv("MOBILE_VALHALLA_URL", "http://127.0.0.1:8002")
	t.Setenv("MOBILE_ROUTING_GRAPH_VERSION", "   ")
	if Configured() != nil {
		t.Fatal("a blank graph version must be a nil router")
	}
}

func TestRouteWithOnePointDoesNotCallTheNetwork(t *testing.T) {
	client, err := newClient("http://127.0.0.1:1", "graph-1")
	if err != nil {
		t.Fatal(err)
	}
	legs, err := client.Route([]itinerary.Point{{Lat: 10.0, Lng: 106.0}}, "walk")
	if err != nil || len(legs) != 0 {
		t.Fatalf("one point: %v %v", legs, err)
	}
}

func TestTrySlotCapsAtFour(t *testing.T) {
	held := 0
	t.Cleanup(func() {
		for i := 0; i < held; i++ {
			ReleaseSlot()
		}
	})
	for i := 0; i < slots; i++ {
		if !TrySlot() {
			t.Fatalf("slot %d refused", i)
		}
		held++
	}
	if TrySlot() {
		t.Fatal("a fifth slot must refuse")
	}
	for i := 0; i < slots; i++ {
		ReleaseSlot()
		held--
	}
	if !TrySlot() {
		t.Fatal("after release a slot must be free")
	}
	held++
	ReleaseSlot()
	held--
}

func TestConfiguredRejectsUserinfoAndQuery(t *testing.T) {
	t.Setenv("MOBILE_ROUTING_GRAPH_VERSION", "graph-1")
	t.Setenv("MOBILE_VALHALLA_URL", "http://user:pass@127.0.0.1:8002")
	if Configured() != nil {
		t.Fatal("userinfo must be a nil router")
	}
	t.Setenv("MOBILE_VALHALLA_URL", "http://127.0.0.1:8002?x=1")
	if Configured() != nil {
		t.Fatal("a query string must be a nil router")
	}
}
