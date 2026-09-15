// Package gifdec decodes the first frame of a GIF the way Pillow 12.2.0's
// GifImagePlugin (LOADING_STRATEGY RGB_AFTER_FIRST) and GifDecode.c do:
// mode "P" with the frame palette, or "L" when no palette is needed, the
// canvas prefilled with the transparency index, the LZW data fed through
// ImageFile.load in 65536-byte reads.
package gifdec

import (
	"errors"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// Plugin is the GIF entry of Image.OPEN.
var Plugin = pil.Plugin{Name: "GIF", Accept: Accept, Open: Open}

// Accept is GifImagePlugin._accept.
func Accept(prefix []byte) bool {
	return len(prefix) >= 6 && (string(prefix[:6]) == "GIF87a" || string(prefix[:6]) == "GIF89a")
}

// decoderMaxBlock is ImageFile.MAXBLOCK, the size of each load read.
const decoderMaxBlock = 65536

type reader struct {
	data []byte
	pos  int
}

func (r *reader) read(n int) []byte {
	end := r.pos + n
	if end > len(r.data) || end < r.pos {
		end = len(r.data)
	}
	b := r.data[r.pos:end]
	r.pos = end
	return b
}

// block is GifImageFile.data: nil stands for None.
func (r *reader) block() []byte {
	s := r.read(1)
	if len(s) > 0 && s[0] != 0 {
		return r.read(int(s[0]))
	}
	return nil
}

func i16(b []byte, o int) (int, bool) {
	if len(b) < o+2 {
		return 0, false
	}
	return int(b[o]) | int(b[o+1])<<8, true
}

// paletteNeeded is _is_palette_needed with Python's chained comparison
// short-circuit; ok is false where Python raises IndexError.
func paletteNeeded(p []byte) (needed, ok bool) {
	for i := 0; i < len(p); i += 3 {
		if i/3 != int(p[i]) {
			return true, true
		}
		if i+1 >= len(p) {
			return false, false
		}
		if p[i] != p[i+1] {
			return true, true
		}
		if i+2 >= len(p) {
			return false, false
		}
		if p[i+1] != p[i+2] {
			return true, true
		}
	}
	return false, true
}

type opened struct {
	data          []byte
	width, height int
	mode          string
	palette       []byte
	transparency  int
	x0, y0        int
	x1, y1        int
	bits          int
	interlace     bool
	offset        int
}

func indexError(what string) error  { return pil.Next("IndexError: %s", what) }
func structError(what string) error { return pil.Next("struct.error: %s", what) }

// Open is GifImageFile._open followed by _seek(0).
func Open(data []byte) (pil.Opened, error) {
	r := &reader{data: data}
	s := r.read(13)
	if !Accept(s) {
		return nil, pil.Next("not a GIF file")
	}
	width, ok := i16(s, 6)
	if !ok {
		return nil, structError("screen width")
	}
	height, ok := i16(s, 8)
	if !ok {
		return nil, structError("screen height")
	}
	if len(s) < 11 {
		return nil, indexError("screen flags")
	}
	flags := s[10]
	bits := int(flags&7) + 1
	var globalPalette []byte
	if flags&128 != 0 {
		if len(s) < 12 {
			return nil, indexError("background")
		}
		p := r.read(3 << uint(bits))
		needed, ok := paletteNeeded(p)
		if !ok {
			return nil, indexError("global palette")
		}
		if needed {
			globalPalette = p
		}
	}
	o, err := seekFirst(r, width, height, globalPalette)
	if err != nil {
		return nil, err
	}
	if w, h := o.Size(); w <= 0 || h <= 0 {
		return nil, pil.Next("not identified by this driver")
	}
	return o, nil
}

func seekFirst(r *reader, width, height int, globalPalette []byte) (*opened, error) {
	disposalMethod := 0
	s := r.read(1)
	if len(s) == 0 || s[0] == ';' {
		// EOFError: ImageFile.__init__ re-raises it as SyntaxError.
		return nil, pil.Next("EOFError: no more images in GIF file")
	}
	var palette []byte
	paletteState := 0 // 0 None, 1 False, 2 a palette
	frameTransparency := -1
	interlace := -1
	var x0, y0, x1, y1, tileBits, offset int
	sizeW, sizeH := width, height
	for {
		if len(s) == 0 {
			s = r.read(1)
		}
		if len(s) == 0 || s[0] == ';' {
			break
		}
		if s[0] == '!' {
			s = r.read(1)
			block := r.block()
			if len(s) == 0 {
				return nil, indexError("extension label")
			}
			switch {
			case s[0] == 249 && block != nil:
				if len(block) == 0 {
					return nil, indexError("extension flags")
				}
				gceFlags := block[0]
				if gceFlags&1 != 0 {
					if len(block) < 4 {
						return nil, indexError("transparency")
					}
					frameTransparency = int(block[3])
				}
				if len(block) < 3 {
					return nil, structError("duration")
				}
				if bits := (gceFlags & 0x1c) >> 2; bits != 0 {
					disposalMethod = int(bits)
				}
			case s[0] == 254:
				for len(block) > 0 {
					block = r.block()
				}
				s = nil
				continue
			case s[0] == 255 && block != nil:
				if len(block) >= 11 && string(block[:11]) == "NETSCAPE2.0" {
					r.block()
				}
			}
			for len(r.block()) > 0 {
			}
		} else if s[0] == ',' {
			s = r.read(9)
			a, ok0 := i16(s, 0)
			b, ok1 := i16(s, 2)
			c, ok2 := i16(s, 4)
			d, ok3 := i16(s, 6)
			if !ok0 || !ok1 || !ok2 || !ok3 {
				return nil, structError("image descriptor")
			}
			x0, y0 = a, b
			x1, y1 = x0+c, y0+d
			if x1 > sizeW || y1 > sizeH {
				sizeW, sizeH = max(x1, sizeW), max(y1, sizeH)
				if err := pil.CheckBomb(sizeW, sizeH); err != nil {
					return nil, err
				}
			}
			if len(s) < 9 {
				return nil, indexError("image flags")
			}
			imgFlags := s[8]
			if imgFlags&64 != 0 {
				interlace = 1
			} else {
				interlace = 0
			}
			if imgFlags&128 != 0 {
				lbits := int(imgFlags&7) + 1
				p := r.read(3 << uint(lbits))
				needed, ok := paletteNeeded(p)
				if !ok {
					return nil, indexError("local palette")
				}
				if needed {
					palette, paletteState = p, 2
				} else {
					paletteState = 1
				}
			}
			bb := r.read(1)
			if len(bb) == 0 {
				return nil, indexError("lzw minimum code size")
			}
			tileBits = int(bb[0])
			offset = r.pos
			break
		}
		s = nil
	}
	if interlace < 0 {
		return nil, pil.Next("EOFError: image not found in GIF frame")
	}
	framePalette := globalPalette
	if paletteState == 2 {
		framePalette = palette
	} else if paletteState == 1 {
		framePalette = nil
	}
	o := &opened{
		data:         r.data,
		width:        sizeW,
		height:       sizeH,
		transparency: frameTransparency,
		x0:           x0, y0: y0, x1: x1, y1: y1,
		bits:      tileBits,
		interlace: interlace == 1,
		offset:    offset,
	}
	if framePalette != nil {
		o.mode = "P"
		o.palette = framePalette[:len(framePalette)/3*3]
	} else {
		o.mode = "L"
	}
	if disposalMethod >= 2 {
		// The dispose image of frame 0 is built at open time; only its
		// bomb check can raise here.
		if disposalMethod == 2 || frameTransparency >= 0 {
			if err := pil.CheckBomb(x1-x0, y1-y0); err != nil {
				return nil, err
			}
		}
	}
	return o, nil
}

func (o *opened) Size() (int, int) { return o.width, o.height }

// Load is ImageFile.load with GifImageFile.load_prepare for frame 0.
func (o *opened) Load() (*pil.Image, error) {
	w, h := o.width, o.height
	pix := make([]byte, w*h)
	if o.transparency >= 0 {
		for i := range pix {
			pix[i] = byte(o.transparency)
		}
	}
	if o.x0 < 0 || o.y0 < 0 || o.x1 <= o.x0 || o.y1 <= o.y0 || o.x1 > w || o.y1 > h {
		return nil, errors.New("gif: tile cannot extend outside image")
	}
	dec := &decoder{
		bits:         o.bits,
		interlace:    o.interlace,
		transparency: -1,
		xoff:         o.x0,
		yoff:         o.y0,
		xsize:        o.x1 - o.x0,
		ysize:        o.y1 - o.y0,
		width:        w,
	}
	pos := o.offset
	var buf []byte
	for {
		end := pos + decoderMaxBlock
		if end > len(o.data) {
			end = len(o.data)
		}
		s := o.data[pos:end]
		pos = end
		if len(s) == 0 {
			return nil, errors.New("gif: image file is truncated")
		}
		buf = append(buf, s...)
		n := dec.decode(pix, buf)
		if n < 0 {
			break
		}
		buf = buf[n:]
	}
	if dec.errcode < 0 {
		return nil, errors.New("gif: decoder error")
	}
	img := &pil.Image{Format: "GIF", Mode: o.mode, Width: w, Height: h, Pix: pix}
	if o.mode == "P" {
		img.PaletteMode = "RGB"
		img.Palette = o.palette
	}
	if o.transparency >= 0 {
		img.Info.Transparency = &pil.Transparency{Kind: pil.TransparencyInt, Int: o.transparency}
	}
	return img, nil
}
