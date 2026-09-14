// Package friendship ports app.domain.friendship (F03, F04): the friend graph
// as a state machine in which only the addressee turns a request into a
// friendship.
//
// Parity, not correctness, is the contract (ADR-0029 §2.4): oracle_test.go
// replays testdata/python_*.json, rendered by
// scripts/render_domain_w2_goldens.py from the real module in the parity API
// image. Behaviour that looks surprising is Python's and is kept; the
// function comments say where.
//
// # Values
//
// An Edge is the dict the service and the repository build. DecidedByID nil is
// Python's None and also an absent key: nothing in the module tells the two
// apart. Pair nil is an absent "pair" key. Strings are Python str values, so
// they must be valid UTF-8; state and decision words are checked the way the
// StrEnum lookups check them, and an unknown word is a *ValueError whose
// message is Python's, repr and all.
package friendship

import (
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// The four rest states of one edge (FriendState).
const (
	StatePending  = "pending"
	StateAccepted = "accepted"
	StateDeclined = "declined"
	StateBlocked  = "blocked"
)

// What an addressee may answer (Decision).
const (
	DecisionAccept  = "accept"
	DecisionDecline = "decline"
	DecisionBlock   = "block"
)

// BlockedIsSilent is BLOCKED_IS_SILENT: the one code open_request gives for a
// pending, accepted or blocked edge alike, so a block never names itself.
const BlockedIsSilent = "REQUEST_NOT_OPEN"

// Codes FriendshipError carries besides BlockedIsSilent.
const (
	CodePersonRequired         = "PERSON_REQUIRED"
	CodeSelfEdge               = "SELF_EDGE"
	CodeNotAParty              = "NOT_A_PARTY"
	CodeAlreadyBlocked         = "ALREADY_BLOCKED"
	CodeNotPending             = "NOT_PENDING"
	CodeOnlyAddresseeMayAnswer = "ONLY_ADDRESSEE_MAY_ANSWER"
	CodeNotBlocked             = "NOT_BLOCKED"
	CodeOnlyBlockerMayUnblock  = "ONLY_BLOCKER_MAY_UNBLOCK"
)

var (
	states    = [...]string{StatePending, StateAccepted, StateDeclined, StateBlocked}
	decisions = [...]string{DecisionAccept, DecisionDecline, DecisionBlock}
	// live is _LIVE: the states that still occupy the pair.
	live = [...]string{StatePending, StateAccepted, StateBlocked}
)

// States returns FriendState's values in declaration order.
func States() []string { return slices.Clone(states[:]) }

// Decisions returns Decision's values in declaration order.
func Decisions() []string { return slices.Clone(decisions[:]) }

// LiveStates returns _LIVE in declaration order.
func LiveStates() []string { return slices.Clone(live[:]) }

// FriendshipError is Python's FriendshipError: `str(exc)` is the code.
type FriendshipError struct {
	Code string
}

func (e *FriendshipError) Error() string { return e.Code }

func refuse(code string) error { return &FriendshipError{Code: code} }

// ValueError is the ValueError `FriendState(value)` or `Decision(value)`
// raises for a word that is not a member.
type ValueError struct {
	Message string
}

func (e *ValueError) Error() string { return e.Message }

// Edge is the edge dict. See the package comment for nil.
type Edge struct {
	RequesterID string
	AddresseeID string
	State       string
	DecidedByID *string
	Pair        []string
}

// clone copies the edge so a returned dict never aliases the caller's, the
// way `{**edge}` does not.
func (e Edge) clone() Edge {
	out := e
	out.Pair = slices.Clone(e.Pair)
	if e.DecidedByID != nil {
		decider := *e.DecidedByID
		out.DecidedByID = &decider
	}
	return out
}

func friendState(value string) (string, error) {
	if slices.Contains(states[:], value) {
		return value, nil
	}
	return "", &ValueError{Message: reprStr(value) + " is not a valid FriendState"}
}

func decision(value string) (string, error) {
	if slices.Contains(decisions[:], value) {
		return value, nil
	}
	return "", &ValueError{Message: reprStr(value) + " is not a valid Decision"}
}

// PairKey is pair_key: the unordered pair, sorted as Python sorts str. UTF-8
// byte order is code point order, so Go's < agrees.
func PairKey(a, b string) ([2]string, error) {
	if a == "" || b == "" {
		return [2]string{}, refuse(CodePersonRequired)
	}
	if a == b {
		return [2]string{}, refuse(CodeSelfEdge)
	}
	if a < b {
		return [2]string{a, b}, nil
	}
	return [2]string{b, a}, nil
}

// IsLiveEdge is is_live_edge.
func IsLiveEdge(state string) (bool, error) {
	member, err := friendState(state)
	if err != nil {
		return false, err
	}
	return slices.Contains(live[:], member), nil
}

// OpenRequest is open_request. Only existing.State is read.
func OpenRequest(requesterID, addresseeID string, existing *Edge) (Edge, error) {
	pair, err := PairKey(requesterID, addresseeID)
	if err != nil {
		return Edge{}, err
	}
	if existing != nil {
		occupied, err := IsLiveEdge(existing.State)
		if err != nil {
			return Edge{}, err
		}
		if occupied {
			return Edge{}, refuse(BlockedIsSilent)
		}
	}
	return Edge{
		RequesterID: requesterID,
		AddresseeID: addresseeID,
		State:       StatePending,
		Pair:        []string{pair[0], pair[1]},
	}, nil
}

// Decide is decide. The decision word is checked before the state word, and
// nothing checks that the edge is not a self-edge: a row naming one person
// twice lets that person accept.
func Decide(edge Edge, actorID, decisionWord string) (Edge, error) {
	answer, err := decision(decisionWord)
	if err != nil {
		return Edge{}, err
	}
	state, err := friendState(edge.State)
	if err != nil {
		return Edge{}, err
	}
	if actorID != edge.RequesterID && actorID != edge.AddresseeID {
		return Edge{}, refuse(CodeNotAParty)
	}
	out := edge.clone()
	if answer == DecisionBlock {
		if state == StateBlocked {
			return Edge{}, refuse(CodeAlreadyBlocked)
		}
		out.State = StateBlocked
		return out, nil
	}
	if state != StatePending {
		return Edge{}, refuse(CodeNotPending)
	}
	if actorID != edge.AddresseeID {
		return Edge{}, refuse(CodeOnlyAddresseeMayAnswer)
	}
	if answer == DecisionAccept {
		out.State = StateAccepted
	} else {
		out.State = StateDeclined
	}
	return out, nil
}

// OpenBlock is open_block. With no edge or a declined one it writes a fresh
// blocked edge owned by the blocker. Otherwise it is Decide(block), which
// keeps whatever decided_by_id the existing edge carried: despite the Python
// docstring, the blocker is not written there on that path.
func OpenBlock(blockerID, addresseeID string, existing *Edge) (Edge, error) {
	pair, err := PairKey(blockerID, addresseeID)
	if err != nil {
		return Edge{}, err
	}
	fresh := existing == nil
	if !fresh {
		state, err := friendState(existing.State)
		if err != nil {
			return Edge{}, err
		}
		fresh = state == StateDeclined
	}
	if fresh {
		decider := blockerID
		return Edge{
			RequesterID: blockerID,
			AddresseeID: addresseeID,
			State:       StateBlocked,
			DecidedByID: &decider,
			Pair:        []string{pair[0], pair[1]},
		}, nil
	}
	return Decide(*existing, blockerID, DecisionBlock)
}

// Unblock is unblock.
func Unblock(edge *Edge, actorID string) (Edge, error) {
	if edge == nil {
		return Edge{}, refuse(CodeNotBlocked)
	}
	state, err := friendState(edge.State)
	if err != nil {
		return Edge{}, err
	}
	if state != StateBlocked {
		return Edge{}, refuse(CodeNotBlocked)
	}
	if edge.DecidedByID == nil || *edge.DecidedByID != actorID {
		return Edge{}, refuse(CodeOnlyBlockerMayUnblock)
	}
	out := edge.clone()
	out.State = StateDeclined
	decider := actorID
	out.DecidedByID = &decider
	return out, nil
}

// AreFriends is are_friends.
func AreFriends(edge *Edge) (bool, error) {
	if edge == nil {
		return false, nil
	}
	state, err := friendState(edge.State)
	if err != nil {
		return false, err
	}
	return state == StateAccepted, nil
}

// reprStr is CPython 3.12's repr(str), as the enum's ValueError prints it.
func reprStr(s string) string {
	quote := byte('\'')
	if strings.IndexByte(s, '\'') >= 0 && strings.IndexByte(s, '"') < 0 {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == rune(quote) || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case isPrintable(r):
			b.WriteRune(r)
		case r <= 0xFF:
			b.WriteString(`\x` + padHex(r, 2))
		case r <= 0xFFFF:
			b.WriteString(`\u` + padHex(r, 4))
		default:
			b.WriteString(`\U` + padHex(r, 8))
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// isPrintable is Py_UNICODE_ISPRINTABLE: everything but categories C* and Z*,
// with the ASCII space kept. Go 1.23 and CPython 3.12 both read Unicode 15.0;
// oracle_test.go checks every code point.
func isPrintable(r rune) bool {
	if r < 0x7F {
		return r >= 0x20
	}
	return r != 0x7F && unicode.IsPrint(r)
}

func padHex(r rune, width int) string {
	digits := strconv.FormatInt(int64(r), 16)
	if len(digits) < width {
		digits = strings.Repeat("0", width-len(digits)) + digits
	}
	return digits
}
