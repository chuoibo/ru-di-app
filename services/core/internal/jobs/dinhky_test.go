package jobs

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestKiemDinhKyRefusesWhatCannotRun(t *testing.T) {
	pass := func(context.Context, pgx.Tx) error { return nil }
	good := []DinhKy{{Ten: "jobs.don_outbox", Nhip: time.Minute, Chay: pass}, {Ten: "chatassist.sweep", Nhip: 5 * time.Second, Chay: pass}}
	if err := KiemDinhKy(good); err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string][]DinhKy{
		"twice":     {good[0], good[0]},
		"no name":   {{Nhip: time.Second, Chay: pass}},
		"bad name":  {{Ten: "Jobs Sweep", Nhip: time.Second, Chay: pass}},
		"no period": {{Ten: "x", Chay: pass}},
		"no pass":   {{Ten: "x", Nhip: time.Second}},
	} {
		if err := KiemDinhKy(bad); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestParseQueues(t *testing.T) {
	all := []string{"ai.group", "ai.nep"}
	if got, err := ParseQueues("", all); err != nil || strings.Join(got, ",") != "ai.group,ai.nep" {
		t.Fatalf("empty: %v %v", got, err)
	}
	if got, err := ParseQueues("ai.nep", all); err != nil || strings.Join(got, ",") != "ai.nep" {
		t.Fatalf("one: %v %v", got, err)
	}
	for _, bad := range []string{"ai.nep,ai.nep", "memory", "ai.nep, ai.group", ",", "ai.nep,"} {
		if _, err := ParseQueues(bad, all); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// The broker URL carries a password; a refusal must not repeat it.
func TestCheckURLDoesNotEchoTheURL(t *testing.T) {
	if err := CheckURL("amqp://tier:s3cret-value@127.0.0.1:5672/"); err != nil {
		t.Fatal(err)
	}
	err := CheckURL("http://tier:s3cret-value@127.0.0.1:5672/")
	if err == nil || strings.Contains(err.Error(), "s3cret") {
		t.Fatalf("%v", err)
	}
}

// A process without a broker, or one whose consumers are not all attached,
// is not "up": its poller must run at the fast pace.
func TestSongOnlyWithEveryConsumerAttached(t *testing.T) {
	var none *Ket
	if none.Song() {
		t.Fatal("a nil Ket is up")
	}
	k := &Ket{Queues: []string{"ai.group", "ai.nep"}}
	if k.Song() {
		t.Fatal("no consumer attached, yet up")
	}
	k.attached.Add(1)
	if k.Song() {
		t.Fatal("one of two consumers attached, yet up")
	}
	k.attached.Add(1)
	if !k.Song() {
		t.Fatal("both attached, yet down")
	}
}
