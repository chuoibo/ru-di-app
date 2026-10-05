package plugins

import (
	"encoding/binary"
	"runtime"
	"testing"
)

// A TIFF whose strip offsets carry more values than any image within the
// pixel limit needs is refused without expanding them (audit 2026-10-05,
// CODEC-02): before, every offset became a value, and a strip, first.
func TestTIFFStripOffsetsPastTheBudgetAreNotExpanded(t *testing.T) {
	le := binary.LittleEndian
	const many = maxTagValues + 1
	type tag struct {
		id, typ uint16
		n, v    uint32
	}
	payload := 8 + 2 + 8*12 + 4
	tags := []tag{
		{256, 3, 1, 1}, {257, 3, 1, 1}, {258, 3, 1, 8}, {259, 3, 1, 1}, {262, 3, 1, 1},
		{273, 3, many, uint32(payload)}, {278, 3, 1, 0}, {279, 3, many, uint32(payload)},
	}
	data := []byte{'I', 'I', 42, 0}
	data = le.AppendUint32(data, 8)
	data = le.AppendUint16(data, uint16(len(tags)))
	for _, tg := range tags {
		data = le.AppendUint16(data, tg.id)
		data = le.AppendUint16(data, tg.typ)
		data = le.AppendUint32(data, tg.n)
		data = le.AppendUint32(data, tg.v)
	}
	data = le.AppendUint32(data, 0)
	data = append(data, make([]byte, 2*many)...)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	opened, err := openTIFF(&file{data: data})
	if err == nil {
		_, err = opened.Load()
	}
	runtime.ReadMemStats(&after)
	if err == nil {
		t.Fatal("a TIFF without usable strip offsets opened and loaded")
	}
	if grown := after.TotalAlloc - before.TotalAlloc; grown > 4<<20 {
		t.Fatalf("refusing a %d-byte TIFF allocated %d bytes", len(data), grown)
	}
}
