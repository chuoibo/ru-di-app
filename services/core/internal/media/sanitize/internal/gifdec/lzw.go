package gifdec

// Port of libImaging/GifDecode.c (ImagingGifDecode).

const (
	gifBits   = 12
	gifTable  = 1 << gifBits
	gifBuffer = 1 << gifBits

	codecOverrun = -1
	codecBroken  = -2
	codecConfig  = -8
)

type decoder struct {
	// GIFDECODERSTATE
	bits         int
	interlace    bool
	interlacePas int
	transparency int
	step, repeat int
	bitbuffer    int32
	bitcount     int
	blocksize    int
	codesize     int
	codemask     int
	clear, end   int
	lastcode     int
	lastdata     byte
	bufferindex  int
	buffer       [gifTable]byte
	link         [gifTable]uint16
	data         [gifTable]byte
	next         int

	// ImagingCodecState
	state                    int
	x, y                     int
	xsize, ysize, xoff, yoff int
	errcode                  int

	width int // image stride
}

// newline is the NEWLINE macro; it reports false where the macro returns -1.
func (d *decoder) newline() (out int, ok bool) {
	d.x = 0
	d.y += d.step
	for d.y >= d.ysize {
		switch d.interlacePas {
		case 1:
			d.repeat, d.y = 4, 4
			d.interlacePas = 2
		case 2:
			d.step = 4
			d.repeat, d.y = 2, 2
			d.interlacePas = 3
		case 3:
			d.step = 2
			d.repeat, d.y = 1, 1
			d.interlacePas = 0
		default:
			return 0, false
		}
	}
	return (d.y+d.yoff)*d.width + d.xoff, true
}

// decode is ImagingGifDecode: it returns the bytes consumed, or -1 when
// decoding stopped (errcode tells whether it was an error).
func (d *decoder) decode(im []byte, buf []byte) int {
	ptr := 0
	bytes := len(buf)
	if d.state == 0 {
		if d.bits < 0 || d.bits > 12 {
			d.errcode = codecConfig
			return -1
		}
		d.clear = 1 << uint(d.bits)
		d.end = d.clear + 1
		if d.interlace {
			d.interlacePas = 1
			d.step, d.repeat = 8, 8
		} else {
			d.interlacePas = 0
			d.step = 1
		}
		d.state = 1
	}
	out := (d.y+d.yoff)*d.width + d.xoff + d.x
	for {
		if d.state == 1 {
			d.next = d.clear + 2
			d.codesize = d.bits + 1
			d.codemask = (1 << uint(d.codesize)) - 1
			d.bufferindex = gifBuffer
			d.state = 2
		}
		var p []byte
		var i int
		if d.bufferindex < gifBuffer {
			i = gifBuffer - d.bufferindex
			p = d.buffer[d.bufferindex:]
			d.bufferindex = gifBuffer
		} else {
			for d.bitcount < d.codesize {
				if d.blocksize > 0 {
					c := int(buf[ptr])
					ptr++
					bytes--
					d.blocksize--
					d.bitbuffer |= int32(c) << uint(d.bitcount)
					d.bitcount += 8
				} else {
					if bytes < 1 {
						return ptr
					}
					c := int(buf[ptr])
					if bytes < c+1 {
						return ptr
					}
					d.blocksize = c
					ptr++
					bytes--
				}
			}
			c := int(d.bitbuffer) & d.codemask
			d.bitbuffer >>= uint(d.codesize)
			d.bitcount -= d.codesize
			if c == d.clear {
				if d.state != 2 {
					d.state = 1
				}
				continue
			}
			if c == d.end {
				break
			}
			i = 1
			if d.state == 2 {
				if c > d.clear {
					d.errcode = codecBroken
					return -1
				}
				d.lastdata = byte(c)
				d.lastcode = c
				d.state = 3
			} else {
				thiscode := c
				if c > d.next {
					d.errcode = codecBroken
					return -1
				}
				if c == d.next {
					if d.bufferindex <= 0 {
						d.errcode = codecBroken
						return -1
					}
					d.bufferindex--
					d.buffer[d.bufferindex] = d.lastdata
					c = d.lastcode
				}
				for c >= d.clear {
					if d.bufferindex <= 0 || c >= gifTable {
						d.errcode = codecBroken
						return -1
					}
					d.bufferindex--
					d.buffer[d.bufferindex] = d.data[c]
					c = int(d.link[c])
				}
				d.lastdata = byte(c)
				if d.next < gifTable {
					d.data[d.next] = byte(c)
					d.link[d.next] = uint16(d.lastcode)
					if d.next == d.codemask && d.codesize < gifBits {
						d.codesize++
						d.codemask = (1 << uint(d.codesize)) - 1
					}
					d.next++
				}
				d.lastcode = thiscode
			}
			p = []byte{d.lastdata}
		}
		if d.y >= d.ysize {
			d.errcode = codecOverrun
			return -1
		}
		if d.transparency == -1 {
			if i == 1 {
				if d.x < d.xsize-1 {
					im[out] = p[0]
					out++
					d.x++
					continue
				}
			} else if d.x+i <= d.xsize {
				copy(im[out:out+i], p[:i])
				out += i
				d.x += i
				if d.x == d.xsize {
					var ok bool
					if out, ok = d.newline(); !ok {
						return -1
					}
				}
				continue
			}
		}
		for c := 0; c < i; c++ {
			if int(p[c]) != d.transparency {
				im[out] = p[c]
			}
			out++
			d.x++
			if d.x >= d.xsize {
				var ok bool
				if out, ok = d.newline(); !ok {
					return -1
				}
			}
		}
	}
	return ptr
}
