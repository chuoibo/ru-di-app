package aistream

import (
	"errors"
	"regexp"
	"strings"
)

// Keys names the Redis keys of one deployment namespace.
type Keys struct{ prefix string }

var (
	namespacePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,48}$`)
	idPattern        = regexp.MustCompile(`^[0-9a-fA-F-]{8,64}$`)
	// ErrName refuses an id that could escape its key.
	ErrName = errors.New("aistream: invalid key id")
)

// NewKeys validates the namespace the way internal/chatbus does.
func NewKeys(namespace string) (Keys, error) {
	if !namespacePattern.MatchString(namespace) {
		return Keys{}, errors.New("aistream: invalid namespace")
	}
	return Keys{prefix: "rudi:" + namespace + ":ai:"}, nil
}

// Invocation is the stream one invocation's requester reads.
func (k Keys) Invocation(id string) (string, error) { return k.name("inv:", id) }

// Room is the stream the onlookers of a legacy-lane room read. A v2 (E2EE)
// job never writes here; the writer's caller enforces that by lane.
func (k Keys) Room(contextID string) (string, error) { return k.name("room:", contextID) }

// Wake is the pub/sub channel whose messages name a key that just grew.
func (k Keys) Wake() string { return k.prefix + "wake" }

// Owns reports whether key belongs to this namespace's streams.
func (k Keys) Owns(key string) bool {
	return strings.HasPrefix(key, k.prefix+"inv:") || strings.HasPrefix(key, k.prefix+"room:")
}

func (k Keys) isRoom(key string) bool { return strings.HasPrefix(key, k.prefix+"room:") }

func (k Keys) name(kind, id string) (string, error) {
	if !idPattern.MatchString(id) {
		return "", ErrName
	}
	return k.prefix + kind + id, nil
}
