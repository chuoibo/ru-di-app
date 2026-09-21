package jpegdec

// Ports of jdmarker.c, the header half of jdinput.c and
// default_decompress_parms from jdapimin.c. Marker reads block on Pillow's
// next block instead of suspending: libjpeg restarts an interrupted marker
// from its first byte and every handler is idempotent, so the outcome is
// the same.

func (d *decoder) getSOI() {
	if d.sawSOI {
		errexit("JERR_SOI_DUPLICATE")
	}
	for i := 0; i < numArithTbls; i++ {
		d.arithDcL[i] = 0
		d.arithDcU[i] = 1
		d.arithAcK[i] = 5
	}
	d.restartInterval = 0
	d.jpegColorSpace = csUnknown
	d.sawJFIF = false
	d.sawAdobe = false
	d.adobeTransform = 0
	d.sawSOI = true
}

func (d *decoder) getSOF(isProg, isLossless, isArith bool) {
	s := d.src
	if d.sawSOF {
		errexit("JERR_SOF_DUPLICATE")
	}
	d.progressive = isProg
	d.lossless = isLossless
	d.arith = isArith
	length := s.input2Bytes()
	d.dataPrecision = s.inputByte()
	d.imageHeight = s.input2Bytes()
	d.imageWidth = s.input2Bytes()
	d.numComponents = s.inputByte()
	length -= 8
	if d.imageHeight <= 0 || d.imageWidth <= 0 || d.numComponents <= 0 {
		errexit("JERR_EMPTY_IMAGE")
	}
	if length != d.numComponents*3 {
		errexit("JERR_BAD_LENGTH")
	}
	if d.comps == nil {
		d.comps = make([]component, d.numComponents)
	}
	for ci := 0; ci < d.numComponents; ci++ {
		c := &d.comps[ci]
		c.index = ci
		c.id = s.inputByte()
		b := s.inputByte()
		c.h = (b >> 4) & 15
		c.v = b & 15
		c.quantTblNo = s.inputByte()
	}
	d.sawSOF = true
}

func (d *decoder) getSOS() {
	s := d.src
	if !d.sawSOF {
		errexit("JERR_SOS_NO_SOF")
	}
	length := s.input2Bytes()
	n := s.inputByte()
	if length != n*2+6 || n < 1 || n > maxCompsInScan {
		errexit("JERR_BAD_LENGTH")
	}
	d.compsInScan = n
	for i := range d.curComps {
		d.curComps[i] = nil
	}
	for i := 0; i < n; i++ {
		cc := s.inputByte()
		c := s.inputByte()
		found := -1
		// libjpeg-turbo tests cur_comp_info at the component index, not
		// at the scan position it fills.
		for ci := 0; ci < d.numComponents && ci < maxCompsInScan; ci++ {
			if cc == d.comps[ci].id && d.curComps[ci] == nil {
				found = ci
				break
			}
		}
		if found < 0 {
			errexit("JERR_BAD_COMPONENT_ID")
		}
		comp := &d.comps[found]
		d.curComps[i] = comp
		comp.dcTblNo = (c >> 4) & 15
		comp.acTblNo = c & 15
		for pi := 0; pi < i; pi++ {
			if d.curComps[pi] == comp {
				errexit("JERR_BAD_COMPONENT_ID")
			}
		}
	}
	d.ss = s.inputByte()
	d.se = s.inputByte()
	c := s.inputByte()
	d.ah = (c >> 4) & 15
	d.al = c & 15
	d.nextRestartNum = 0
	d.inputScanNumber++
}

func (d *decoder) getDAC() {
	s := d.src
	length := s.input2Bytes() - 2
	for length > 0 {
		index := s.inputByte()
		val := s.inputByte()
		length -= 2
		if index < 0 || index >= 2*numArithTbls {
			errexit("JERR_DAC_INDEX")
		}
		if index >= numArithTbls {
			d.arithAcK[index-numArithTbls] = uint8(val)
		} else {
			d.arithDcL[index] = uint8(val & 0x0F)
			d.arithDcU[index] = uint8(val >> 4)
			if d.arithDcL[index] > d.arithDcU[index] {
				errexit("JERR_DAC_VALUE")
			}
		}
	}
	if length != 0 {
		errexit("JERR_BAD_LENGTH")
	}
}

func (d *decoder) getDHT() {
	s := d.src
	length := s.input2Bytes() - 2
	for length > 16 {
		index := s.inputByte()
		var bits [17]uint8
		count := 0
		for i := 1; i <= 16; i++ {
			bits[i] = uint8(s.inputByte())
			count += int(bits[i])
		}
		length -= 1 + 16
		if count > 256 || count > length {
			errexit("JERR_BAD_HUFF_TABLE")
		}
		var huffval [256]uint8
		for i := 0; i < count; i++ {
			huffval[i] = uint8(s.inputByte())
		}
		length -= count
		var slot **huffTable
		if index&0x10 != 0 {
			index -= 0x10
			if index < 0 || index >= numHuffTbls {
				errexit("JERR_DHT_INDEX")
			}
			slot = &d.acHuffTbls[index]
		} else {
			if index < 0 || index >= numHuffTbls {
				errexit("JERR_DHT_INDEX")
			}
			slot = &d.dcHuffTbls[index]
		}
		if *slot == nil {
			*slot = &huffTable{}
		}
		(*slot).bits = bits
		(*slot).huffval = huffval
	}
	if length != 0 {
		errexit("JERR_BAD_LENGTH")
	}
}

func (d *decoder) getDQT() {
	s := d.src
	length := s.input2Bytes() - 2
	for length > 0 {
		n := s.inputByte()
		prec := n >> 4
		n &= 0x0F
		if n >= numQuantTbls {
			errexit("JERR_DQT_INDEX")
		}
		if d.quantTbls[n] == nil {
			d.quantTbls[n] = new([64]uint16)
		}
		q := d.quantTbls[n]
		for i := 0; i < 64; i++ {
			var tmp int
			if prec != 0 {
				tmp = s.input2Bytes()
			} else {
				tmp = s.inputByte()
			}
			q[naturalOrder[i]] = uint16(tmp)
		}
		length -= 64 + 1
		if prec != 0 {
			length -= 64
		}
	}
	if length != 0 {
		errexit("JERR_BAD_LENGTH")
	}
}

func (d *decoder) getDRI() {
	s := d.src
	if s.input2Bytes() != 4 {
		errexit("JERR_BAD_LENGTH")
	}
	d.restartInterval = s.input2Bytes()
}

// getInterestingAPPn reads APP0 and APP14, the only application markers
// libjpeg looks into when Pillow saves none.
func (d *decoder) getInterestingAPPn() {
	s := d.src
	length := s.input2Bytes() - 2
	numtoread := 0
	if length >= 14 {
		numtoread = 14
	} else if length > 0 {
		numtoread = length
	}
	var b [14]byte
	for i := 0; i < numtoread; i++ {
		b[i] = byte(s.inputByte())
	}
	length -= numtoread
	data := b[:numtoread]
	switch d.unreadMarker {
	case 0xE0:
		if len(data) >= 14 && data[0] == 'J' && data[1] == 'F' && data[2] == 'I' && data[3] == 'F' && data[4] == 0 {
			d.sawJFIF = true
		}
	case 0xEE:
		if len(data) >= 12 && data[0] == 'A' && data[1] == 'd' && data[2] == 'o' && data[3] == 'b' && data[4] == 'e' {
			d.sawAdobe = true
			d.adobeTransform = int(data[11])
		}
	}
	if length > 0 {
		s.skip(length)
	}
}

func (d *decoder) skipVariable() {
	length := d.src.input2Bytes() - 2
	if length > 0 {
		d.src.skip(length)
	}
}

func (d *decoder) nextMarker() {
	s := d.src
	for {
		c := s.inputByte()
		for c != 0xFF {
			c = s.inputByte()
		}
		for {
			c = s.inputByte()
			if c != 0xFF {
				break
			}
		}
		if c != 0 {
			d.unreadMarker = c
			return
		}
	}
}

func (d *decoder) firstMarker() {
	c := d.src.inputByte()
	c2 := d.src.inputByte()
	if c != 0xFF || c2 != 0xD8 {
		errexit("JERR_NO_SOI")
	}
	d.unreadMarker = c2
}

func (d *decoder) readMarkers() int {
	for {
		if d.unreadMarker == 0 {
			if !d.sawSOI {
				d.firstMarker()
			} else {
				d.nextMarker()
			}
		}
		m := d.unreadMarker
		switch {
		case m == 0xD8:
			d.getSOI()
		case m == 0xC0 || m == 0xC1:
			d.getSOF(false, false, false)
		case m == 0xC2:
			d.getSOF(true, false, false)
		case m == 0xC3:
			d.getSOF(false, true, false)
		case m == 0xC9:
			d.getSOF(false, false, true)
		case m == 0xCA:
			d.getSOF(true, false, true)
		case m == 0xCB:
			d.getSOF(false, true, true)
		case m == 0xC5 || m == 0xC6 || m == 0xC7 || m == 0xC8 || m == 0xCD || m == 0xCE || m == 0xCF:
			errexit("JERR_SOF_UNSUPPORTED 0x%02x", m)
		case m == 0xDA:
			d.getSOS()
			d.unreadMarker = 0
			return reachedSOS
		case m == 0xD9:
			d.unreadMarker = 0
			return reachedEOI
		case m == 0xCC:
			d.getDAC()
		case m == 0xC4:
			d.getDHT()
		case m == 0xDB:
			d.getDQT()
		case m == 0xDD:
			d.getDRI()
		case m == 0xE0 || m == 0xEE:
			d.getInterestingAPPn()
		case m >= 0xE1 && m <= 0xEF, m == 0xFE, m == 0xDC:
			d.skipVariable()
		case m >= 0xD0 && m <= 0xD7, m == 0x01:
		default:
			errexit("JERR_UNKNOWN_MARKER 0x%02x", m)
		}
		d.unreadMarker = 0
	}
}

func (d *decoder) readRestartMarker() {
	if d.unreadMarker == 0 {
		d.nextMarker()
	}
	if d.unreadMarker == 0xD0+d.nextRestartNum {
		d.unreadMarker = 0
	} else {
		d.resyncToRestart(d.nextRestartNum)
	}
	d.nextRestartNum = (d.nextRestartNum + 1) & 7
}

// resyncToRestart is jpeg_resync_to_restart, the resync Pillow installs.
func (d *decoder) resyncToRestart(desired int) {
	marker := d.unreadMarker
	for {
		action := 1
		switch {
		case marker < 0xC0:
			action = 2
		case marker < 0xD0 || marker > 0xD7:
			action = 3
		case marker == 0xD0+((desired+1)&7) || marker == 0xD0+((desired+2)&7):
			action = 3
		case marker == 0xD0+((desired-1)&7) || marker == 0xD0+((desired-2)&7):
			action = 2
		}
		switch action {
		case 1:
			d.unreadMarker = 0
			return
		case 2:
			d.nextMarker()
			marker = d.unreadMarker
		default:
			return
		}
	}
}

// resetInputController is reset_input_controller with
// reset_marker_reader, run when jpeg_consume_input leaves DSTATE_START.
func (d *decoder) resetInputController() {
	d.scanActive = false
	d.hasMultipleScans = false
	d.eoiReached = false
	d.inHeaders = true
	d.comps = nil
	d.inputScanNumber = 0
	d.unreadMarker = 0
	d.sawSOI = false
	d.sawSOF = false
	d.coefBits = nil
}

// consumeMarkers is consume_markers.
func (d *decoder) consumeMarkers() int {
	if d.eoiReached {
		return reachedEOI
	}
	val := d.readMarkers()
	switch val {
	case reachedSOS:
		if d.inHeaders {
			d.initialSetup()
			d.inHeaders = false
		} else {
			if !d.hasMultipleScans {
				errexit("JERR_EOI_EXPECTED")
			}
			d.startInputPass()
		}
	case reachedEOI:
		d.eoiReached = true
		if d.inHeaders {
			if d.sawSOF {
				errexit("JERR_SOF_NO_SOS")
			}
		} else if d.outputScanNumber > d.inputScanNumber {
			d.outputScanNumber = d.inputScanNumber
		}
	}
	return val
}

func divRoundUp(a, b int) int { return (a + b - 1) / b }

func roundUp(a, b int) int {
	a += b - 1
	return a - a%b
}

func (d *decoder) dataUnit() int {
	if d.lossless {
		return 1
	}
	return 8
}

// initialSetup is jdinput.c initial_setup.
func (d *decoder) initialSetup() {
	if d.imageHeight > maxDimension || d.imageWidth > maxDimension {
		errexit("JERR_IMAGE_TOO_BIG")
	}
	if d.lossless {
		if d.dataPrecision < 2 || d.dataPrecision > 16 {
			errexit("JERR_BAD_PRECISION")
		}
	} else if d.dataPrecision != 8 && d.dataPrecision != 12 {
		errexit("JERR_BAD_PRECISION")
	}
	if d.numComponents > maxComponents {
		errexit("JERR_COMPONENT_COUNT")
	}
	du := d.dataUnit()
	d.maxH, d.maxV = 1, 1
	for ci := range d.comps {
		c := &d.comps[ci]
		if c.h <= 0 || c.h > maxSampFactor || c.v <= 0 || c.v > maxSampFactor {
			errexit("JERR_BAD_SAMPLING")
		}
		d.maxH = max(d.maxH, c.h)
		d.maxV = max(d.maxV, c.v)
	}
	for ci := range d.comps {
		c := &d.comps[ci]
		c.widthInBlocks = divRoundUp(d.imageWidth*c.h, d.maxH*du)
		c.heightInBlocks = divRoundUp(d.imageHeight*c.v, d.maxV*du)
		c.downsampledWidth = divRoundUp(d.imageWidth*c.h, d.maxH)
		c.downsampledHeight = divRoundUp(d.imageHeight*c.v, d.maxV)
		c.quant = nil
	}
	d.totalIMCURows = divRoundUp(d.imageHeight, d.maxV*du)
	d.hasMultipleScans = d.compsInScan < d.numComponents || d.progressive
}

// perScanSetup is jdinput.c per_scan_setup.
func (d *decoder) perScanSetup() {
	if d.compsInScan == 1 {
		c := d.curComps[0]
		d.mcusPerRow = c.widthInBlocks
		d.mcuRowsInScan = c.heightInBlocks
		c.mcuWidth, c.mcuHeight, c.mcuBlocks = 1, 1, 1
		c.lastColWidth = 1
		tmp := c.heightInBlocks % c.v
		if tmp == 0 {
			tmp = c.v
		}
		c.lastRowHeight = tmp
		d.blocksInMCU = 1
		d.mcuMembership[0] = 0
		return
	}
	if d.compsInScan <= 0 || d.compsInScan > maxCompsInScan {
		errexit("JERR_COMPONENT_COUNT")
	}
	du := d.dataUnit()
	d.mcusPerRow = divRoundUp(d.imageWidth, d.maxH*du)
	d.mcuRowsInScan = divRoundUp(d.imageHeight, d.maxV*du)
	d.blocksInMCU = 0
	for ci := 0; ci < d.compsInScan; ci++ {
		c := d.curComps[ci]
		c.mcuWidth = c.h
		c.mcuHeight = c.v
		c.mcuBlocks = c.h * c.v
		tmp := c.widthInBlocks % c.mcuWidth
		if tmp == 0 {
			tmp = c.mcuWidth
		}
		c.lastColWidth = tmp
		tmp = c.heightInBlocks % c.mcuHeight
		if tmp == 0 {
			tmp = c.mcuHeight
		}
		c.lastRowHeight = tmp
		if d.blocksInMCU+c.mcuBlocks > dMaxBlocksInMCU {
			errexit("JERR_BAD_MCU_SIZE")
		}
		for n := 0; n < c.mcuBlocks; n++ {
			d.mcuMembership[d.blocksInMCU] = ci
			d.blocksInMCU++
		}
	}
}

func (d *decoder) latchQuantTables() {
	for ci := 0; ci < d.compsInScan; ci++ {
		c := d.curComps[ci]
		if c.quant != nil {
			continue
		}
		n := c.quantTblNo
		if n < 0 || n >= numQuantTbls || d.quantTbls[n] == nil {
			errexit("JERR_NO_QUANT_TABLE")
		}
		table := *d.quantTbls[n]
		c.quant = &table
	}
}

// startInputPass is jdinput.c start_input_pass.
func (d *decoder) startInputPass() {
	d.perScanSetup()
	if !d.lossless {
		d.latchQuantTables()
	}
	switch {
	case d.arith:
		d.startPassArith()
	case d.progressive:
		d.startPassPhuff()
	default:
		d.startPassHuff()
	}
	d.inputIMCURow = 0
	d.startIMCURow()
	d.scanActive = true
}

func (d *decoder) finishInputPass() { d.scanActive = false }

// startIMCURow is jdcoefct.c start_iMCU_row.
func (d *decoder) startIMCURow() {
	if d.compsInScan > 1 {
		d.mcuRowsPerIMCURow = 1
	} else if d.inputIMCURow < d.totalIMCURows-1 {
		d.mcuRowsPerIMCURow = d.curComps[0].v
	} else {
		d.mcuRowsPerIMCURow = d.curComps[0].lastRowHeight
	}
}

// defaultDecompressParms is default_decompress_parms.
func (d *decoder) defaultDecompressParms() {
	switch d.numComponents {
	case 1:
		d.jpegColorSpace = csGrayscale
		d.outColorSpace = csGrayscale
	case 3:
		switch {
		case d.sawJFIF:
			d.jpegColorSpace = csYCbCr
		case d.sawAdobe:
			if d.adobeTransform == 0 {
				d.jpegColorSpace = csRGB
			} else {
				d.jpegColorSpace = csYCbCr
			}
		default:
			cid0, cid1, cid2 := d.comps[0].id, d.comps[1].id, d.comps[2].id
			switch {
			case cid0 == 1 && cid1 == 2 && cid2 == 3:
				if d.lossless {
					d.jpegColorSpace = csRGB
				} else {
					d.jpegColorSpace = csYCbCr
				}
			case cid0 == 82 && cid1 == 71 && cid2 == 66:
				d.jpegColorSpace = csRGB
			case d.lossless:
				d.jpegColorSpace = csRGB
			default:
				d.jpegColorSpace = csYCbCr
			}
		}
		d.outColorSpace = csRGB
	case 4:
		if d.sawAdobe && d.adobeTransform == 0 {
			d.jpegColorSpace = csCMYK
		} else if d.sawAdobe {
			d.jpegColorSpace = csYCCK
		} else {
			d.jpegColorSpace = csCMYK
		}
		d.outColorSpace = csCMYK
	default:
		d.jpegColorSpace = csUnknown
		d.outColorSpace = csUnknown
	}
}
