package scenario

// Generated multipart bodies (body_parts), added for the W6 photo routes.
//
// Why generate at all. The photo routes are defined by what they do to a file:
// a JPEG with an EXIF orientation, a PNG with an alpha channel, a GIF, a WebP,
// a file cut short, a body past the 10 MiB cap, a header that claims more
// pixels than the sanitizer allows. None of that can be written as text in
// body_raw, and the repository guard refuses image bytes and base64 blobs in
// git, rightly: a committed photograph is how a real one gets in. So a step
// describes the file and the runner builds it.
//
// Why this is still a request "sent verbatim". The bytes are a pure function
// of the step's parameters: no clock, no randomness beyond Image.Seed, nothing
// read from the stack. The reference and the candidate are therefore sent the
// same bytes, and a difference in their answers is a difference in them. The
// encoders are Go's image/jpeg, image/png and image/gif plus two small writers
// in the runner (BMP, lossless WebP). Their output may change with the Go
// toolchain, but both stacks of one run are sent the output of one binary, and
// the runner's tests pin what matters (dimensions, pixels, EXIF, header
// fields), not a hash.
//
// Why a list of parts rather than one image field. Half of what a photo route
// refuses lives in the envelope, not in the file: no file field, a field of
// another name, a text value where a file belongs, an empty file, a second
// file. Spelling the envelope out part by part makes each of those one line.
// Anything the envelope cannot say (a part with no name, a broken boundary)
// is still written by hand in body_raw.

import (
	"errors"
	"fmt"
	"mime"
	"strings"
)

// Part is one part of a multipart/form-data body. For each part in order the
// runner writes
//
//	--<boundary>\r\n
//	Content-Disposition: form-data; name="<name>"[; filename="<filename>"]\r\n
//	[Content-Type: <content_type>\r\n]
//	\r\n
//	<text, or the generated image>\r\n
//
// and then --<boundary>--\r\n. The boundary is the one the step's own
// content-type header names; the runner never adds or changes that header.
// An empty list is a body holding only the closing delimiter.
type Part struct {
	Name string `yaml:"name"`
	// Filename, when present (even ""), makes the part a file to Starlette;
	// absent, the part is a plain form field.
	Filename    *string `yaml:"filename"`
	ContentType *string `yaml:"content_type"`
	// Exactly one of Text and Image. Text is rendered like body_raw.
	Text  *string `yaml:"text"`
	Image *Image  `yaml:"image"`
}

// Image describes a generated picture. Every field but Format, Width and
// Height is optional, and the zero value of each means "not applied".
type Image struct {
	Format string `yaml:"format"` // jpeg | png | gif | webp | bmp
	Width  int    `yaml:"width"`
	Height int    `yaml:"height"`
	// Seed picks the noise channel and the palette: one seed, one picture.
	Seed uint32 `yaml:"seed"`
	// Color is rgb (the default; palette is the default for gif), gray or
	// palette.
	Color string `yaml:"color"`
	// Alpha makes part of the picture transparent: an RGBA PNG, a tRNS chunk
	// or a GIF transparent index for a palette, the alpha bit of a WebP.
	Alpha bool `yaml:"alpha"`
	// Orientation, 1..255, writes an EXIF Orientation tag: an APP1 segment in
	// a JPEG, an eXIf chunk in a PNG, an EXIF chunk (extended format) in a
	// WebP. Values past 8 are a tag the decoder has no transpose for.
	Orientation int `yaml:"orientation"`
	// HeaderWidth and HeaderHeight overwrite the dimensions stored in the
	// file's header after encoding, so a small file can claim a huge picture.
	HeaderWidth  int `yaml:"header_width"`
	HeaderHeight int `yaml:"header_height"`
	// TruncateAt keeps only the first N bytes of the encoded file.
	TruncateAt int `yaml:"truncate_at"`
	// PadToBytes appends zero bytes until the file is N bytes long.
	PadToBytes int `yaml:"pad_to_bytes"`
}

const (
	// MaxImageSide bounds Image.Width and Image.Height: generation stays fast,
	// and a picture that must look huge uses HeaderWidth/HeaderHeight.
	MaxImageSide = 4096
	// MaxHeaderSide bounds the header override: the JPEG and GIF fields are
	// 16 bits.
	MaxHeaderSide = 65535
	// MaxPartBytes bounds TruncateAt and PadToBytes: past the API's 10 MiB
	// cap with room to spare.
	MaxPartBytes = 16 << 20
)

// Boundary returns the boundary a multipart/form-data content-type names.
func Boundary(contentType string) (string, error) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", fmt.Errorf("content-type %q: %v", contentType, err)
	}
	if mediaType != "multipart/form-data" || params["boundary"] == "" {
		return "", fmt.Errorf("content-type %q is not multipart/form-data with a boundary", contentType)
	}
	return params["boundary"], nil
}

// validateParts checks a step's body_parts and returns the texts in it that
// may carry {{templates}}.
func validateParts(req Request) ([]string, error) {
	if req.BodyRaw != nil {
		return nil, errors.New("body_raw and body_parts: a step has one body")
	}
	var contentType string
	found := false
	for name, value := range req.Headers {
		if strings.EqualFold(name, "content-type") {
			contentType, found = value, true
		}
	}
	if !found {
		return nil, errors.New("body_parts needs a content-type header naming the boundary")
	}
	if templatePattern.MatchString(contentType) {
		return nil, errors.New("body_parts: the content-type header must be literal, so the boundary is known before the run")
	}
	if _, err := Boundary(contentType); err != nil {
		return nil, fmt.Errorf("body_parts: %w", err)
	}
	var texts []string
	for i, part := range *req.BodyParts {
		where := fmt.Sprintf("body_parts[%d]", i)
		if part.Name == "" || strings.ContainsAny(part.Name, "\"\r\n") {
			return nil, fmt.Errorf("%s: name must be non-empty, without quotes or line breaks (write a nameless part in body_raw)", where)
		}
		texts = append(texts, part.Name)
		if part.Filename != nil {
			if strings.ContainsAny(*part.Filename, "\"\r\n") {
				return nil, fmt.Errorf("%s: filename must not hold quotes or line breaks", where)
			}
			texts = append(texts, *part.Filename)
		}
		if part.ContentType != nil {
			if strings.ContainsAny(*part.ContentType, "\r\n") {
				return nil, fmt.Errorf("%s: content_type must not hold line breaks", where)
			}
			texts = append(texts, *part.ContentType)
		}
		switch {
		case (part.Text == nil) == (part.Image == nil):
			return nil, fmt.Errorf("%s: exactly one of text and image", where)
		case part.Text != nil:
			texts = append(texts, *part.Text)
		default:
			if err := part.Image.Validate(); err != nil {
				return nil, fmt.Errorf("%s: %w", where, err)
			}
		}
	}
	return texts, nil
}

// Validate reports whether the image can be generated as described.
func (img Image) Validate() error {
	colors := map[string][]string{
		"jpeg": {"rgb", "gray"},
		"png":  {"rgb", "gray", "palette"},
		"gif":  {"palette"},
		"webp": {"rgb"},
		"bmp":  {"rgb"},
	}
	allowed, ok := colors[img.Format]
	if !ok {
		return fmt.Errorf("image format %q must be jpeg, png, gif, webp or bmp", img.Format)
	}
	if img.Width < 1 || img.Width > MaxImageSide || img.Height < 1 || img.Height > MaxImageSide {
		return fmt.Errorf("image %dx%d: width and height are 1..%d (claim more with header_width/header_height)", img.Width, img.Height, MaxImageSide)
	}
	if img.Color != "" {
		known := false
		for _, color := range allowed {
			known = known || color == img.Color
		}
		if !known {
			return fmt.Errorf("image color %q: %s takes %s", img.Color, img.Format, strings.Join(allowed, " or "))
		}
	}
	if img.Alpha && (img.Format == "jpeg" || img.Format == "bmp" || img.Color == "gray") {
		return fmt.Errorf("image alpha: %s %s has no transparency to write", img.Format, img.EffectiveColor())
	}
	if img.Orientation < 0 || img.Orientation > 255 {
		return errors.New("image orientation is 0 (no EXIF) or 1..255")
	}
	if img.Orientation > 0 && img.Format != "jpeg" && img.Format != "png" && img.Format != "webp" {
		return fmt.Errorf("image orientation: EXIF is written for jpeg, png and webp, not %s", img.Format)
	}
	if img.HeaderWidth < 0 || img.HeaderWidth > MaxHeaderSide || img.HeaderHeight < 0 || img.HeaderHeight > MaxHeaderSide {
		return fmt.Errorf("image header_width/header_height are 0 (as encoded) or 1..%d", MaxHeaderSide)
	}
	if (img.HeaderWidth > 0 || img.HeaderHeight > 0) && img.Format == "webp" {
		// A WebP decoder allocates the canvas the header claims before any
		// size check could refuse it; a huge claim would test the container's
		// memory, not the route.
		return errors.New("image header_width/header_height: not for webp")
	}
	if img.TruncateAt < 0 || img.TruncateAt > MaxPartBytes || img.PadToBytes < 0 || img.PadToBytes > MaxPartBytes {
		return fmt.Errorf("image truncate_at and pad_to_bytes are 0 (not applied) or up to %d", MaxPartBytes)
	}
	if img.TruncateAt > 0 && img.PadToBytes > 0 && img.PadToBytes < img.TruncateAt {
		return errors.New("image pad_to_bytes must not be shorter than truncate_at")
	}
	return nil
}

// EffectiveColor is Color with its default filled in.
func (img Image) EffectiveColor() string {
	switch {
	case img.Color != "":
		return img.Color
	case img.Format == "gif":
		return "palette"
	default:
		return "rgb"
	}
}
