package socialv2

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type wallCursor struct {
	CreatedAt time.Time
	ID        string
}

func encodeWallCursor(created time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(created.UTC().Format(time.RFC3339Nano) + "|" + id))
}

func decodeWallCursor(value string) (wallCursor, error) {
	var out wallCursor
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) > 128 {
		return out, errors.New("invalid wall cursor")
	}
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return out, errors.New("invalid wall cursor")
	}
	out.CreatedAt, err = time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return wallCursor{}, errors.New("invalid wall cursor")
	}
	id := parts[1]
	if len(id) != 36 || id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		return wallCursor{}, errors.New("invalid wall cursor")
	}
	if _, err = hex.DecodeString(strings.ReplaceAll(id, "-", "")); err != nil || strings.ToLower(id) != id {
		return wallCursor{}, errors.New("invalid wall cursor")
	}
	out.ID = id
	return out, nil
}

type commentParent struct {
	PostID   string
	ParentID string
}

func replyParentAllowed(postID string, parent commentParent) bool {
	return parent.PostID == postID && parent.ParentID == ""
}
