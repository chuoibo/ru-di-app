// Package webpdec decodes WebP files the way Pillow 12.2.0's
// WebPImagePlugin does on top of libwebp 1.6.0: WebPAnimDecoder with its
// default options (MODE_RGBA, fancy upsampling), frame 1 composed on a
// zeroed canvas, mode "RGB" when WebPGetFeatures reports no alpha.
package webpdec

import (
	"errors"

	"mobile/services/core/internal/media/sanitize/internal/pil"
)

// Plugin is the WEBP entry of Image.OPEN.
var Plugin = pil.Plugin{Name: "WEBP", Accept: Accept, Open: Open}

// Accept is WebPImagePlugin._accept with WebP support installed.
func Accept(prefix []byte) bool {
	if len(prefix) < 16 {
		return false
	}
	if string(prefix[:4]) != "RIFF" || string(prefix[8:12]) != "WEBP" {
		return false
	}
	switch string(prefix[12:16]) {
	case "VP8 ", "VP8X", "VP8L":
		return true
	}
	return false
}

var errDecoder = errors.New("webp: could not create decoder object")

type opened struct {
	dmx  *demuxer
	rgbx bool
	exif []byte
	xmp  []byte
}

// Open is WebPImageFile._open: _webp.WebPAnimDecoder raises OSError, which
// escapes Image.open.
func Open(data []byte) (pil.Opened, error) {
	feat, st := getFeatures(data)
	if st != statusOK {
		return nil, errDecoder
	}
	dmx := webpDemux(data)
	if dmx == nil {
		return nil, errDecoder
	}
	o := &opened{dmx: dmx, rgbx: !feat.hasAlpha}
	if c := dmx.chunk("EXIF"); len(c) > 0 {
		o.exif = c
	}
	if c := dmx.chunk("XMP "); len(c) > 0 {
		o.xmp = c
	}
	return o, nil
}

func (o *opened) Size() (int, int) { return o.dmx.canvasWidth, o.dmx.canvasHeight }

// Load is WebPImageFile.load for frame 0 (WebPAnimDecoderGetNext on a key
// frame) followed by the raw "RGBX"/"RGBA" unpack.
func (o *opened) Load() (*pil.Image, error) {
	w, h := o.Size()
	if o.dmx.numFrames < 1 {
		return nil, errors.New("webp: failed to read next frame")
	}
	f := o.dmx.getFrame(1)
	if f == nil {
		return nil, errors.New("webp: failed to read next frame")
	}
	stride := w * 4
	canvas := make([]byte, stride*h)
	p := &decParams{
		out:    canvas,
		stride: stride,
		rgba:   f.yOffset*stride + f.xOffset*4,
		size:   f.height * stride,
	}
	if st := webpDecode(o.dmx.framePayload(f), p); st != statusOK {
		return nil, errors.New("webp: failed to read next frame")
	}
	img := &pil.Image{Format: "WEBP", Width: w, Height: h}
	if o.rgbx {
		img.Mode = "RGB"
		img.Pix = make([]byte, w*h*3)
		for i, j := 0, 0; i < len(canvas); i, j = i+4, j+3 {
			img.Pix[j], img.Pix[j+1], img.Pix[j+2] = canvas[i], canvas[i+1], canvas[i+2]
		}
	} else {
		img.Mode = "RGBA"
		img.Pix = canvas
	}
	if o.exif != nil {
		img.Info.HasExif = true
		img.Info.Exif = o.exif
	}
	if o.xmp != nil {
		img.Info.HasXMP = true
		img.Info.XMP = o.xmp
	}
	return img, nil
}
