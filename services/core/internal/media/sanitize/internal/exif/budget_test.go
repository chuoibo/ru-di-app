package exif

import (
	"encoding/binary"
	"testing"
)

// Many tags pointing at one shared payload stop being loaded once the
// directory's value budget is spent, and one oversized tag is skipped
// (audit 2026-10-05, CODEC-04). Text and byte tags are one value each.
func TestAnIFDCannotExpandTagsTimesPayload(t *testing.T) {
	le := binary.LittleEndian
	const shared = 60000
	tags := 10
	data := le.AppendUint16(nil, uint16(tags+2))
	payload := 2 + (tags+2)*12
	for i := range tags {
		data = le.AppendUint16(data, uint16(0x9000+i))
		data = le.AppendUint16(data, 3)
		data = le.AppendUint32(data, shared)
		data = le.AppendUint32(data, uint32(payload))
	}
	// One SHORT tag past the per-tag cap, and one huge UNDEFINED maker note.
	data = le.AppendUint16(data, 0x9100)
	data = le.AppendUint16(data, 3)
	data = le.AppendUint32(data, maxTagValues+1)
	data = le.AppendUint32(data, uint32(payload))
	data = le.AppendUint16(data, 0x927C)
	data = le.AppendUint16(data, 7)
	data = le.AppendUint32(data, 2*shared)
	data = le.AppendUint32(data, uint32(payload))
	data = append(data, make([]byte, 2*(maxTagValues+1))...)
	got := loadIFD(&reader{data: data}, le, false)
	expanded := 0
	for tag := range got {
		if tag >= 0x9000 && tag < 0x9000+uint16(tags) {
			expanded++
		}
	}
	if expanded*shared > maxIFDValues || expanded == 0 {
		t.Fatalf("%d shared tags loaded (%d values)", expanded, expanded*shared)
	}
	if _, ok := got[0x9100]; ok {
		t.Fatal("a tag past the per-tag cap was loaded")
	}
}
