package nap

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

// A breach must last SLOKeoDai before the verdict turns; recovery clears
// it at once; a failed reading is a failed verdict.
func TestGiamSatKeoDai(t *testing.T) {
	g := &GiamSat{}
	t0 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	if k := g.Ghi(t0, DoTuoi{LechGiay: 5}, nil); !k.Dat || len(k.ViPham) != 0 {
		t.Fatalf("a fresh index: %+v", k)
	}
	lech := DoTuoi{LechGiay: SLOLechGiay + 1}
	if k := g.Ghi(t0.Add(time.Second), lech, nil); !k.Dat || !reflect.DeepEqual(k.ViPham, []string{"lech"}) {
		t.Fatalf("a breach just begun: %+v", k)
	}
	if k := g.Ghi(t0.Add(time.Second+SLOKeoDai-time.Millisecond), lech, nil); !k.Dat {
		t.Fatalf("a breach shorter than %v turned the verdict", SLOKeoDai)
	}
	if k := g.Ghi(t0.Add(time.Second+SLOKeoDai), lech, nil); k.Dat {
		t.Fatalf("a breach of %v kept the verdict", SLOKeoDai)
	}
	if k := g.Ghi(t0.Add(10*time.Minute), DoTuoi{}, nil); !k.Dat || k.TuGiay != 0 {
		t.Fatalf("recovery: %+v", k)
	}
	if k := g.Ghi(t0.Add(11*time.Minute), DoTuoi{}, errors.New("down")); k.Dat || k.Loi == "" {
		t.Fatalf("a failed reading: %+v", k)
	}
	if g.Doc() == nil || g.Doc().Dat {
		t.Fatal("the last reading is not the one served")
	}
	if g.Song(t0, time.Minute) {
		t.Fatal("a loop that never passed is alive")
	}
	g.Nhip(t0)
	if !g.Song(t0.Add(time.Minute), time.Minute) || g.Song(t0.Add(2*time.Minute), time.Minute) {
		t.Fatal("heartbeat window")
	}
}

func TestDoTuoiViPham(t *testing.T) {
	d := DoTuoi{LechGiay: 61, DLQ: 1, Ingest: []DoTreNguon{{Nguon: "a", VongGiay: 10}, {Nguon: "b", VongGiay: SLOIngestGiay + 1}}}
	if got := d.ViPham(); !reflect.DeepEqual(got, []string{"dlq", "ingest", "lech"}) {
		t.Fatalf("%v", got)
	}
	if got := (DoTuoi{LechGiay: 60, Ingest: []DoTreNguon{{VongGiay: SLOIngestGiay}}, ChoNhung: 500, Hoan: 9}).ViPham(); len(got) != 0 {
		t.Fatalf("waits and edges broke the SLO: %v", got)
	}
}
