package jpegenc

// writeFileHeader is jcmarker.c write_file_header: SOI, then the JFIF APP0
// marker for grayscale and YCbCr or the Adobe APP14 marker for CMYK.
func (e *encoder) writeFileHeader(out []byte) []byte {
	out = append(out, 0xFF, 0xD8)
	switch e.opts.Color {
	case Gray, YCbCr:
		// JFIF 1.01, density_unit 0, X_density 1, Y_density 1, no thumbnail.
		out = append(out, 0xFF, 0xE0, 0, 16, 'J', 'F', 'I', 'F', 0, 1, 1, 0, 0, 1, 0, 1, 0, 0)
	case CMYK:
		// Adobe version 100, flags 0, 0, transform 0.
		out = append(out, 0xFF, 0xEE, 0, 14, 'A', 'd', 'o', 'b', 'e', 0, 100, 0, 0, 0, 0, 0)
	}
	return out
}

// writeFrameHeader is write_frame_header: a DQT per table in component
// order (each table once), then SOF0. Quality scaling forces baseline
// tables, so no 16-bit DQT and no SOF1 can occur.
func (e *encoder) writeFrameHeader(out []byte) []byte {
	var sent [2]bool
	for _, c := range e.comps {
		if sent[c.tbl] {
			continue
		}
		sent[c.tbl] = true
		out = append(out, 0xFF, 0xDB, 0, 67, byte(c.tbl))
		for i := 0; i < 64; i++ {
			out = append(out, byte(e.quant[c.tbl][naturalOrder[i]]))
		}
	}
	n := len(e.comps)
	length := 3*n + 8
	out = append(out, 0xFF, 0xC0, byte(length>>8), byte(length), 8,
		byte(e.opts.Height>>8), byte(e.opts.Height), byte(e.opts.Width>>8), byte(e.opts.Width), byte(n))
	for _, c := range e.comps {
		out = append(out, byte(c.id), byte(c.h<<4|c.v), byte(c.tbl))
	}
	return out
}

// writeScanHeader is write_scan_header for the single sequential scan:
// DHT for each component's DC then AC table not yet sent, DRI when a
// restart interval is set, SOS.
func (e *encoder) writeScanHeader(out []byte, dc, ac *[2]*huffTable) []byte {
	var sentDC, sentAC [2]bool
	emit := func(t *huffTable, index byte) {
		length := 0
		for l := 1; l <= 16; l++ {
			length += int(t.bits[l])
		}
		out = append(out, 0xFF, 0xC4, byte((length+19)>>8), byte(length+19), index)
		out = append(out, t.bits[1:]...)
		out = append(out, t.huffval[:length]...)
	}
	for _, c := range e.comps {
		if !sentDC[c.tbl] {
			emit(dc[c.tbl], byte(c.tbl))
			sentDC[c.tbl] = true
		}
		if !sentAC[c.tbl] {
			emit(ac[c.tbl], byte(0x10+c.tbl))
			sentAC[c.tbl] = true
		}
	}
	if interval := e.opts.RestartInterval; interval != 0 {
		out = append(out, 0xFF, 0xDD, 0, 4, byte(interval>>8), byte(interval))
	}
	n := len(e.comps)
	length := 2*n + 6
	out = append(out, 0xFF, 0xDA, byte(length>>8), byte(length), byte(n))
	for _, c := range e.comps {
		out = append(out, byte(c.id), byte(c.tbl<<4|c.tbl))
	}
	return append(out, 0, 63, 0)
}
