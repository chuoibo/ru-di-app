// Package convert reproduces the sanitizer's alpha check and
// Image.convert("RGB" or "RGBA") of Pillow 12.2.0: Image.convert's
// transparency paths, libImaging/Convert.c converters, frompalette and the
// palette as _imaging.c putpalette/putpalettealpha(s) leave it.
package convert

import (
	"fmt"
	"math"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// Error is an exception convert raised; the sanitizer turns it into
// not_an_image.
type Error struct{ Msg string }

func (e *Error) Error() string { return "convert: " + e.Msg }

func fail(format string, args ...any) error { return &Error{Msg: fmt.Sprintf(format, args...)} }

// ForEncoder is the sanitizer's
//
//	has_alpha = "A" in getbands() or (mode == "P" and "transparency" in info)
//	pixels = convert("RGBA" if has_alpha else "RGB").tobytes()
//
// returning the mode and 4 or 3 bytes per pixel.
func ForEncoder(img *pil.Image) (string, []byte, error) {
	mode := "RGB"
	if pil.HasAlphaBand(img.Mode) || img.Mode == "P" && img.Info.Transparency != nil {
		mode = "RGBA"
	}
	out, err := Convert(img, mode)
	if err != nil {
		return "", nil, err
	}
	return mode, out.Pix, nil
}

// Convert is Image.convert(mode) without matrix, dither or palette
// arguments, for target modes "L", "RGB" and "RGBA".
func Convert(img *pil.Image, mode string) (*pil.Image, error) {
	if pil.BytesPerPixel(img.Mode) == 0 {
		return nil, pil.Unsupported(img.Format, "mode %q", img.Mode)
	}
	if mode == img.Mode {
		out := *img
		return &out, nil
	}
	palette, err := corePalette(img)
	if err != nil {
		return nil, err
	}
	if t := img.Info.Transparency; t != nil {
		switch {
		case (img.Mode == "L" || img.Mode == "RGB" || img.Mode == "P") && (mode == "L" || mode == "RGB"):
			// trns_im.putpixel((0, 0), t) for an int; bytes only warn.
			if img.Mode == "L" && t.Kind != pil.TransparencyInt && t.Kind != pil.TransparencyBytes {
				return nil, fail("TypeError: color must be int or single-element tuple")
			}
		case img.Mode == "P" && (mode == "RGBA"):
			switch t.Kind {
			case pil.TransparencyBytes:
				if len(t.Bytes) > 256 {
					return nil, fail("ValueError: palette alpha outside palette")
				}
				for i, a := range t.Bytes {
					palette[i*4+3] = a
				}
			case pil.TransparencyInt:
				if t.Int < 0 || t.Int >= 256 {
					return nil, fail("ValueError: palette index outside palette")
				}
				palette[t.Int*4+3] = 0
			default:
				return nil, fail("ValueError: Transparency for P mode should be bytes or int")
			}
		}
	}
	if img.Mode == "LAB" {
		return nil, pil.Unsupported(img.Format, "LAB conversion goes through ImageCms")
	}
	out, err := core(img, palette, mode)
	if err == nil {
		return out, nil
	}
	base := modeBase(img.Mode)
	if base == img.Mode {
		return nil, err
	}
	step, err := core(img, palette, base)
	if err != nil {
		return nil, err
	}
	return core(step, palette, mode)
}

func modeBase(mode string) string {
	switch mode {
	case "1", "L", "I", "F", "LA", "La", "I;16", "I;16L", "I;16B", "I;16N":
		return "L"
	case "P":
		return "P"
	}
	return "RGB"
}

// corePalette is the 256-entry RGBA palette of the core image after load:
// ImagingPaletteNew (black, alpha 255) with the plugin's palette unpacked
// over it.
func corePalette(img *pil.Image) (*[1024]byte, error) {
	var palette [1024]byte
	for i := 0; i < 256; i++ {
		palette[i*4+3] = 255
	}
	if img.Mode != "P" && img.Mode != "PA" || img.PaletteMode == "" {
		return &palette, nil
	}
	bits := 24
	if img.PaletteMode == "RGBA" {
		bits = 32
	}
	entries := len(img.Palette) * 8 / bits
	if entries > 256 {
		return nil, fail("ValueError: invalid palette size")
	}
	for i := 0; i < entries; i++ {
		copy(palette[i*4:i*4+3], img.Palette[i*bits/8:])
		if bits == 32 {
			palette[i*4+3] = img.Palette[i*4+3]
		}
	}
	return &palette, nil
}

func clip8(v int) byte {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return byte(v)
}

func muldiv255(a, b int) int {
	tmp := a*b + 128
	return ((tmp >> 8) + tmp) >> 8
}

// core is libImaging convert(): the same mode copies, a palette image goes
// through frompalette, anything else needs an entry of the converters
// table.
func core(img *pil.Image, palette *[1024]byte, mode string) (*pil.Image, error) {
	if img.Mode == mode {
		out := *img
		return &out, nil
	}
	n := img.Width * img.Height
	outStride := pil.BytesPerPixel(mode)
	out := &pil.Image{Mode: mode, Width: img.Width, Height: img.Height, Info: img.Info, Format: img.Format}
	pix := make([]byte, n*outStride)
	in := img.Pix
	set := func(i int, r, g, b, a byte) {
		switch outStride {
		case 1:
			pix[i] = r
		case 3:
			pix[3*i], pix[3*i+1], pix[3*i+2] = r, g, b
		case 4:
			pix[4*i], pix[4*i+1], pix[4*i+2], pix[4*i+3] = r, g, b, a
		}
	}
	unsupported := fail("ValueError: conversion from %s to %s not supported", img.Mode, mode)
	gray := func(fn func(i int) byte) error {
		switch mode {
		case "L", "RGB", "RGBA":
			for i := 0; i < n; i++ {
				v := fn(i)
				set(i, v, v, v, 255)
			}
			return nil
		}
		return unsupported
	}
	var err error
	switch img.Mode {
	case "P", "PA":
		alpha := img.Mode == "PA"
		switch mode {
		case "L":
			for i := 0; i < n; i++ {
				p := int(in[i*len(img.Mode)]) * 4
				pix[i] = byte((int(palette[p])*19595 + int(palette[p+1])*38470 + int(palette[p+2])*7471 + 0x8000) >> 16)
			}
		case "RGB", "RGBA":
			for i := 0; i < n; i++ {
				p := int(in[i*len(img.Mode)]) * 4
				a := palette[p+3]
				if alpha {
					a = in[2*i+1]
				} else if mode == "RGB" {
					a = 255
				}
				set(i, palette[p], palette[p+1], palette[p+2], a)
			}
		default:
			err = fail("ValueError: conversion not supported")
		}
	case "1":
		err = gray(func(i int) byte {
			if in[i] != 0 {
				return 255
			}
			return 0
		})
	case "L":
		err = gray(func(i int) byte { return in[i] })
	case "LA":
		switch mode {
		case "L":
			for i := 0; i < n; i++ {
				pix[i] = in[2*i]
			}
		case "RGB", "RGBA":
			for i := 0; i < n; i++ {
				v := in[2*i]
				set(i, v, v, v, in[2*i+1])
			}
		default:
			err = unsupported
		}
	case "I":
		err = gray(func(i int) byte {
			v := int32(uint32(in[4*i]) | uint32(in[4*i+1])<<8 | uint32(in[4*i+2])<<16 | uint32(in[4*i+3])<<24)
			return clip8(int(max(min(v, 255), 0)))
		})
	case "F":
		if mode != "L" {
			return nil, unsupported
		}
		for i := 0; i < n; i++ {
			v := math.Float32frombits(uint32(in[4*i]) | uint32(in[4*i+1])<<8 | uint32(in[4*i+2])<<16 | uint32(in[4*i+3])<<24)
			switch {
			case v <= 0:
				pix[i] = 0
			case v >= 255:
				pix[i] = 255
			case v != v:
				pix[i] = 0
			default:
				pix[i] = byte(v)
			}
		}
	case "I;16", "I;16L", "I;16B", "I;16N":
		if img.Mode != "I;16" && mode != "L" {
			return nil, unsupported
		}
		if img.Mode == "I;16" && mode != "L" && mode != "RGB" {
			return nil, unsupported
		}
		err = gray(func(i int) byte {
			if in[2*i+1] != 0 {
				return 255
			}
			return in[2*i]
		})
	case "RGB":
		switch mode {
		case "L":
			for i := 0; i < n; i++ {
				pix[i] = byte((int(in[3*i])*19595 + int(in[3*i+1])*38470 + int(in[3*i+2])*7471 + 0x8000) >> 16)
			}
		case "RGBA":
			for i := 0; i < n; i++ {
				set(i, in[3*i], in[3*i+1], in[3*i+2], 255)
			}
		default:
			err = unsupported
		}
	case "RGBA", "RGBX":
		switch mode {
		case "L":
			for i := 0; i < n; i++ {
				pix[i] = byte((int(in[4*i])*19595 + int(in[4*i+1])*38470 + int(in[4*i+2])*7471 + 0x8000) >> 16)
			}
		case "RGB":
			for i := 0; i < n; i++ {
				set(i, in[4*i], in[4*i+1], in[4*i+2], 255)
			}
		case "RGBA":
			for i := 0; i < n; i++ {
				set(i, in[4*i], in[4*i+1], in[4*i+2], 255)
			}
		default:
			err = unsupported
		}
	case "RGBa":
		switch mode {
		case "RGB", "RGBA":
			for i := 0; i < n; i++ {
				a := int(in[4*i+3])
				r, g, b := in[4*i], in[4*i+1], in[4*i+2]
				if a != 255 && a != 0 {
					r = clip8(255 * int(r) / a)
					g = clip8(255 * int(g) / a)
					b = clip8(255 * int(b) / a)
				}
				out := byte(255)
				if mode == "RGBA" {
					out = byte(a)
				}
				set(i, r, g, b, out)
			}
		default:
			err = unsupported
		}
	case "CMYK":
		switch mode {
		case "RGB", "RGBA":
			for i := 0; i < n; i++ {
				nk := 255 - int(in[4*i+3])
				set(i,
					clip8(nk-muldiv255(int(in[4*i]), nk)),
					clip8(nk-muldiv255(int(in[4*i+1]), nk)),
					clip8(nk-muldiv255(int(in[4*i+2]), nk)),
					255)
			}
		default:
			err = unsupported
		}
	case "YCbCr":
		switch mode {
		case "L":
			for i := 0; i < n; i++ {
				pix[i] = in[3*i]
			}
		case "RGB":
			for i := 0; i < n; i++ {
				y, cb, cr := int(in[3*i]), in[3*i+1], in[3*i+2]
				r := y + int(yccRCr[cr])>>6
				g := y + (int(yccGCb[cb])+int(yccGCr[cr]))>>6
				b := y + int(yccBCb[cb])>>6
				set(i, clip8(r), clip8(g), clip8(b), 255)
			}
		default:
			err = unsupported
		}
	case "HSV":
		if mode != "RGB" {
			return nil, unsupported
		}
		for i := 0; i < n; i++ {
			r, g, b := hsvToRGB(in[3*i], in[3*i+1], in[3*i+2])
			set(i, r, g, b, 255)
		}
	default:
		err = unsupported
	}
	if err != nil {
		return nil, err
	}
	out.Pix = pix
	return out, nil
}

// hsvToRGB is Convert.c hsv2rgb with its float and double arithmetic.
func hsvToRGB(h, s, v byte) (byte, byte, byte) {
	if s == 0 {
		return v, v, v
	}
	i := int(math.Floor(float64(float32(h)) * 6.0 / 255.0))
	f := float32(float64(float32(h))*6.0/255.0 - float64(float32(i)))
	fs := float32(float64(float32(s)) / 255.0)
	p := int(math.Round(float64(float32(v)) * (1.0 - float64(fs))))
	q := int(math.Round(float64(float32(v)) * (1.0 - float64(fs)*float64(f))))
	t := int(math.Round(float64(float32(v)) * (1.0 - float64(fs)*(1.0-float64(f)))))
	up, uq, ut := clip8(p), clip8(q), clip8(t)
	switch i % 6 {
	case 0:
		return v, ut, up
	case 1:
		return uq, v, up
	case 2:
		return up, v, ut
	case 3:
		return up, uq, v
	case 4:
		return ut, up, v
	}
	return v, up, uq
}
