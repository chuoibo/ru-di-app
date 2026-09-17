// Package faces is app.domain.faces: anonymous rectangles on one photograph.
package faces

import (
	"fmt"
	"sort"
)

// MaxFaces is MAX_FACES.
const MaxFaces = 24

// Error is FaceError.
type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }

// Box is one pixel rectangle from the detector.
type Box struct {
	X, Y, Width, Height int
}

// Published is one anonymous fraction rectangle on the wire.
type Published struct {
	BoxKey string
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// AnonymousBoxes is anonymous_boxes.
func AnonymousBoxes(boxes []Box, imageWidth, imageHeight int) ([]Published, error) {
	if imageWidth <= 0 || imageHeight <= 0 {
		return nil, &Error{"IMAGE_DIMENSIONS_INVALID"}
	}
	type clamped struct{ top, left, width, height int }
	seen := map[clamped]bool{}
	var ordered []clamped
	for _, box := range boxes {
		if box.Width <= 0 || box.Height <= 0 {
			return nil, &Error{"FACE_BOX_DEGENERATE"}
		}
		left := max(0, min(box.X, imageWidth))
		top := max(0, min(box.Y, imageHeight))
		right := max(0, min(box.X+box.Width, imageWidth))
		bottom := max(0, min(box.Y+box.Height, imageHeight))
		if right <= left || bottom <= top {
			return nil, &Error{"FACE_BOX_OUTSIDE_IMAGE"}
		}
		entry := clamped{top: top, left: left, width: right - left, height: bottom - top}
		if seen[entry] {
			continue
		}
		seen[entry] = true
		ordered = append(ordered, entry)
	}
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if a.top != b.top {
			return a.top < b.top
		}
		if a.left != b.left {
			return a.left < b.left
		}
		if a.width != b.width {
			return a.width < b.width
		}
		return a.height < b.height
	})
	if len(ordered) > MaxFaces {
		return nil, &Error{"TOO_MANY_FACES"}
	}
	out := make([]Published, 0, len(ordered))
	for i, box := range ordered {
		out = append(out, Published{
			BoxKey: fmt.Sprintf("face-%d", i+1),
			X:      float64(box.left) / float64(imageWidth),
			Y:      float64(box.top) / float64(imageHeight),
			Width:  float64(box.width) / float64(imageWidth),
			Height: float64(box.height) / float64(imageHeight),
		})
	}
	return out, nil
}
