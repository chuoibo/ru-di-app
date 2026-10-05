package chatv2

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ADR-0057: what a device needs around the event log -- enrollment, key
// packages, the Welcome mailbox, bootstrapping a room and committing roster
// changes under an epoch compare-and-swap. The server is the authority on who
// belongs in a room (active memberships x live devices); a commit may only add
// devices the server expects and remove devices it no longer expects.

const (
	MaxDevices           = 5
	MaxKeyPackages       = 20
	KeyPackageLifetime   = 30 * 24 * time.Hour
	maxKeyPackageBytes   = 64 << 10
	maxWelcomeBytes      = 256 << 10
	maxPublishKeyPackage = 10
	// claimHold is how long a handed-out key package waits for the commit
	// that adds its device; then it is burned (deleted, never re-issued).
	claimHold = time.Hour
	// maxCommitsPerMinute bounds one device's commits: each one moves the
	// epoch under every other member's sends.
	maxCommitsPerMinute = 10
)

var (
	ErrDeviceLimit           = errors.New("chat_v2_device_limit")
	ErrKeyPackageUnavailable = errors.New("chat_v2_key_package_unavailable")
	ErrCapacity              = errors.New("chat_v2_capacity")
	ErrRoster                = errors.New("chat_v2_roster_mismatch")
	ErrExists                = errors.New("chat_v2_conversation_exists")
)

// Card is a device as every member verifies it, the JSON shape of the Rust
// crate's IdentityCard: 32-byte keys as arrays of numbers.
type Card struct {
	ActorID               string   `json:"actor_id"`
	DeviceID              string   `json:"device_id"`
	MLSSignatureKey       [32]byte `json:"mls_signature_key"`
	TransportSignatureKey [32]byte `json:"transport_signature_key"`
}

// Enrollment is a device asking to be one of its person's (at most five).
type Enrollment struct {
	DeviceID              string `json:"device_id"`
	MLSSignatureKey       []byte `json:"mls_signature_key"`
	TransportSignatureKey []byte `json:"transport_signature_key"`
	Proof                 []byte `json:"proof"`
	Label                 string `json:"label"`
}

// EnrollmentBytes is what the device signs with its transport key: it holds
// that key, for this person, under this id, beside this MLS key.
func EnrollmentBytes(actor, device string, mlsKey []byte) []byte {
	b := []byte("RUDI-CHAT-DEVICE\x00v1\x00")
	for _, v := range []string{actor, device} {
		b = binary.BigEndian.AppendUint32(b, uint32(len(v)))
		b = append(b, v...)
	}
	return append(b, mlsKey...)
}

type DeviceView struct {
	Card      Card       `json:"card"`
	Label     string     `json:"label"`
	CreatedAt time.Time  `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at"`
}

type RosterView struct {
	Exists       bool   `json:"exists"`
	Epoch        int64  `json:"epoch"`
	LastSequence int64  `json:"last_sequence"`
	Ready        bool   `json:"ready"`
	Members      []Card `json:"members"`
	Expected     []Card `json:"expected"`
}

type Claim struct {
	Card       Card   `json:"card"`
	KeyPackage []byte `json:"key_package"`
}

type CommitRequest struct {
	Envelope Envelope `json:"envelope"`
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Welcome  []byte   `json:"welcome"`
}

// CommitBody is the stored commit event: the envelope and the roster every
// member must verify the commit against (the Rust `verified_next_roster`).
type CommitBody struct {
	Envelope Envelope `json:"envelope"`
	Roster   []Card   `json:"roster"`
}

type CommitResult struct {
	Event    Event `json:"event"`
	Ready    bool  `json:"ready"`
	Replayed bool  `json:"replayed"`
}

type WelcomeView struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	Sequence       int64  `json:"sequence"`
	Welcome        []byte `json:"welcome"`
	Roster         []Card `json:"roster"`
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:], nil
}

// liveSession holds the person row (FOR mode) and the bearer's session FOR
// SHARE until the transaction ends.
func liveSession(ctx context.Context, tx pgx.Tx, actor string, digest []byte, personLock string) error {
	if !ValidID(actor) || len(digest) != 32 {
		return ErrForbidden
	}
	var id string
	err := tx.QueryRow(ctx, `SELECT id::text FROM people WHERE id=$1 AND deleted_at IS NULL `+personLock, actor).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrForbidden
	}
	if err != nil {
		return err
	}
	err = tx.QueryRow(ctx, `SELECT person_id::text FROM account_sessions WHERE token_digest=$1 AND person_id=$2 AND revoked_at IS NULL AND expires_at>clock_timestamp() FOR SHARE`, digest, actor).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrForbidden
	}
	return err
}

func scanCard(row interface{ Scan(...any) error }) (Card, error) {
	var c Card
	var mls, transport []byte
	if err := row.Scan(&c.DeviceID, &c.ActorID, &mls, &transport); err != nil {
		return c, err
	}
	if len(mls) != 32 || len(transport) != 32 {
		return c, ErrInvalid
	}
	copy(c.MLSSignatureKey[:], mls)
	copy(c.TransportSignatureKey[:], transport)
	return c, nil
}

func cards(ctx context.Context, tx pgx.Tx, sql string, args ...any) ([]Card, error) {
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Card{}
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// expectedRoster is the server's word on a room: every live, MLS-enrolled
// device of every active member whose account stands.
func expectedRoster(ctx context.Context, tx pgx.Tx, conversation string) ([]Card, error) {
	return cards(ctx, tx, `SELECT d.id::text, d.person_id::text, d.mls_signature_key, d.signing_key
		  FROM memberships m
		  JOIN people p ON p.id = m.person_id AND p.deleted_at IS NULL
		  JOIN chat_v2_devices d ON d.person_id = m.person_id AND d.revoked_at IS NULL AND d.mls_signature_key IS NOT NULL
		 WHERE m.context_id=$1 AND m.state='active' AND m.left_at IS NULL
		 ORDER BY d.id`, conversation)
}

func memberCards(ctx context.Context, tx pgx.Tx, conversation string) ([]Card, error) {
	return cards(ctx, tx, `SELECT d.id::text, d.person_id::text, d.mls_signature_key, d.signing_key
		  FROM chat_v2_members cm JOIN chat_v2_devices d ON d.id = cm.device_id
		 WHERE cm.context_id=$1 ORDER BY d.id`, conversation)
}

func ids(cs []Card) map[string]Card {
	out := make(map[string]Card, len(cs))
	for _, c := range cs {
		out[c.DeviceID] = c
	}
	return out
}

func sameSet(a, b []Card) bool {
	if len(a) != len(b) {
		return false
	}
	m := ids(a)
	for _, c := range b {
		if x, ok := m[c.DeviceID]; !ok || x != c {
			return false
		}
	}
	return true
}

func (s *Store) EnrollDevice(ctx context.Context, actor string, digest []byte, e Enrollment) (Card, error) {
	var card Card
	if !ValidID(e.DeviceID) || len(e.MLSSignatureKey) != 32 || len(e.TransportSignatureKey) != 32 ||
		len(e.Proof) != ed25519.SignatureSize || len([]rune(e.Label)) > 60 {
		return card, ErrInvalid
	}
	if !ed25519.Verify(e.TransportSignatureKey, EnrollmentBytes(actor, e.DeviceID, e.MLSSignatureKey), e.Proof) {
		return card, ErrForbidden
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return card, err
	}
	defer tx.Rollback(ctx)
	// FOR UPDATE on the person serializes this person's enrollments: the
	// device count cannot be raced past five.
	if err := liveSession(ctx, tx, actor, digest, "FOR UPDATE"); err != nil {
		return card, err
	}
	var owner string
	var mls, transport []byte
	var revoked *time.Time
	err = tx.QueryRow(ctx, `SELECT person_id::text, mls_signature_key, signing_key, revoked_at FROM chat_v2_devices WHERE id=$1`, e.DeviceID).Scan(&owner, &mls, &transport, &revoked)
	switch {
	case err == nil:
		if owner != actor || revoked != nil || !bytes.Equal(mls, e.MLSSignatureKey) || !bytes.Equal(transport, e.TransportSignatureKey) {
			return card, ErrConflict
		}
	case errors.Is(err, pgx.ErrNoRows):
		var live int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM chat_v2_devices WHERE person_id=$1 AND revoked_at IS NULL AND mls_signature_key IS NOT NULL`, actor).Scan(&live); err != nil {
			return card, err
		}
		if live >= MaxDevices {
			return card, ErrDeviceLimit
		}
		if _, err := tx.Exec(ctx, `INSERT INTO chat_v2_devices(id,person_id,signing_key,mls_signature_key,enrollment_proof,label) VALUES($1,$2,$3,$4,$5,$6)`,
			e.DeviceID, actor, e.TransportSignatureKey, e.MLSSignatureKey, e.Proof, e.Label); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return card, ErrConflict
			}
			return card, err
		}
	default:
		return card, err
	}
	card = Card{ActorID: actor, DeviceID: e.DeviceID}
	copy(card.MLSSignatureKey[:], e.MLSSignatureKey)
	copy(card.TransportSignatureKey[:], e.TransportSignatureKey)
	return card, tx.Commit(ctx)
}

func (s *Store) RevokeDevice(ctx context.Context, actor string, digest []byte, device string) error {
	if !ValidID(device) {
		return ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := liveSession(ctx, tx, actor, digest, "FOR SHARE"); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE chat_v2_devices SET revoked_at=clock_timestamp() WHERE id=$1 AND person_id=$2 AND revoked_at IS NULL`, device, actor)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrForbidden
	}
	return tx.Commit(ctx)
}

func (s *Store) ListDevices(ctx context.Context, actor string, digest []byte) ([]DeviceView, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := liveSession(ctx, tx, actor, digest, "FOR SHARE"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id::text, person_id::text, mls_signature_key, signing_key, coalesce(label,''), created_at, revoked_at
		FROM chat_v2_devices WHERE person_id=$1 AND mls_signature_key IS NOT NULL ORDER BY created_at, id`, actor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DeviceView{}
	for rows.Next() {
		var v DeviceView
		var mls, transport []byte
		if err := rows.Scan(&v.Card.DeviceID, &v.Card.ActorID, &mls, &transport, &v.Label, &v.CreatedAt, &v.RevokedAt); err != nil {
			return nil, err
		}
		copy(v.Card.MLSSignatureKey[:], mls)
		copy(v.Card.TransportSignatureKey[:], transport)
		out = append(out, v)
	}
	return out, rows.Err()
}

// ownDevice holds the actor's live, MLS-enrolled device FOR SHARE.
func ownDevice(ctx context.Context, tx pgx.Tx, actor, device string) error {
	if !ValidID(device) {
		return ErrInvalid
	}
	var id string
	err := tx.QueryRow(ctx, `SELECT id::text FROM chat_v2_devices WHERE id=$1 AND person_id=$2 AND revoked_at IS NULL AND mls_signature_key IS NOT NULL FOR SHARE`, device, actor).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrForbidden
	}
	return err
}

// PublishKeyPackages stores one-time key packages of the actor's device and
// answers how many it now has unclaimed (the cap counts those alone).
func (s *Store) PublishKeyPackages(ctx context.Context, actor string, digest []byte, device string, packages [][]byte) (int, error) {
	if len(packages) < 1 || len(packages) > maxPublishKeyPackage {
		return 0, ErrInvalid
	}
	for _, p := range packages {
		if len(p) < 1 || len(p) > maxKeyPackageBytes {
			return 0, ErrInvalid
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if err := liveSession(ctx, tx, actor, digest, "FOR SHARE"); err != nil {
		return 0, err
	}
	// FOR UPDATE on the device row serializes its publishers on the count.
	var id string
	err = tx.QueryRow(ctx, `SELECT id::text FROM chat_v2_devices WHERE id=$1 AND person_id=$2 AND revoked_at IS NULL AND mls_signature_key IS NOT NULL FOR UPDATE`, device, actor).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrForbidden
	}
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM chat_v2_key_packages WHERE device_id=$1 AND (expires_at<=clock_timestamp() OR claimed_at<=clock_timestamp()-$2::interval)`, device, claimHold.String()); err != nil {
		return 0, err
	}
	var have int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM chat_v2_key_packages WHERE device_id=$1 AND claimed_by IS NULL`, device).Scan(&have); err != nil {
		return 0, err
	}
	if have+len(packages) > MaxKeyPackages {
		return have, ErrCapacity
	}
	for _, p := range packages {
		kid, err := newUUID()
		if err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO chat_v2_key_packages(id,device_id,key_package,expires_at) VALUES($1,$2,$3,clock_timestamp()+$4::interval)`,
			kid, device, p, KeyPackageLifetime.String()); err != nil {
			return 0, err
		}
	}
	return have + len(packages), tx.Commit(ctx)
}

// Roster is the room as the server sees it, for an active member's live
// device: whether it exists, its epoch, its MLS members and who it expects.
func (s *Store) Roster(ctx context.Context, actor string, digest []byte, device, conversation string) (RosterView, error) {
	var v RosterView
	if !ValidID(conversation) {
		return v, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return v, err
	}
	defer tx.Rollback(ctx)
	if err := liveSession(ctx, tx, actor, digest, "FOR SHARE"); err != nil {
		return v, err
	}
	if err := ownDevice(ctx, tx, actor, device); err != nil {
		return v, err
	}
	if err := activeMember(ctx, tx, actor, conversation); err != nil {
		return v, err
	}
	err = tx.QueryRow(ctx, `SELECT epoch,last_sequence,ready FROM chat_v2_conversations WHERE context_id=$1 FOR SHARE`, conversation).Scan(&v.Epoch, &v.LastSequence, &v.Ready)
	switch {
	case err == nil:
		v.Exists = true
		if v.Members, err = memberCards(ctx, tx, conversation); err != nil {
			return v, err
		}
	case errors.Is(err, pgx.ErrNoRows):
		v.Members = []Card{}
	default:
		return v, err
	}
	if v.Expected, err = expectedRoster(ctx, tx, conversation); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// activeMember holds the actor's active membership of a group or pair the
// actor may still message (a block on a pair forbids it), FOR SHARE.
func activeMember(ctx context.Context, tx pgx.Tx, actor, conversation string) error {
	var kind string
	err := tx.QueryRow(ctx, `SELECT c.kind FROM memberships m JOIN contexts c ON c.id = m.context_id
		WHERE m.context_id=$1 AND m.person_id=$2 AND m.state='active' AND m.left_at IS NULL FOR SHARE OF m`, conversation, actor).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrForbidden
	}
	if err != nil {
		return err
	}
	if kind == "pair" {
		var blocked bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM friend_requests f WHERE f.state='blocked' AND
			((f.requester_id=$1 AND f.addressee_id IN (SELECT person_id FROM memberships WHERE context_id=$2))
			 OR (f.addressee_id=$1 AND f.requester_id IN (SELECT person_id FROM memberships WHERE context_id=$2))))`, actor, conversation).Scan(&blocked); err != nil {
			return err
		}
		if blocked {
			return ErrForbidden
		}
	}
	return nil
}

func membershipOf(ctx context.Context, tx pgx.Tx, conversation, person string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `SELECT id::text FROM memberships WHERE context_id=$1 AND person_id=$2 AND state='active' AND left_at IS NULL ORDER BY id LIMIT 1`, conversation, person).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrRoster
	}
	return id, err
}

// Bootstrap opens the room's v2 lane with the actor's device as its only MLS
// member at epoch 1. Ready only when that device is all the server expects.
func (s *Store) Bootstrap(ctx context.Context, actor string, digest []byte, device, conversation string) (RosterView, error) {
	var v RosterView
	if !ValidID(conversation) {
		return v, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return v, err
	}
	defer tx.Rollback(ctx)
	if err := liveSession(ctx, tx, actor, digest, "FOR SHARE"); err != nil {
		return v, err
	}
	if err := ownDevice(ctx, tx, actor, device); err != nil {
		return v, err
	}
	if err := activeMember(ctx, tx, actor, conversation); err != nil {
		return v, err
	}
	membership, err := membershipOf(ctx, tx, conversation, actor)
	if err != nil {
		return v, err
	}
	expected, err := expectedRoster(ctx, tx, conversation)
	if err != nil {
		return v, err
	}
	only := []Card{}
	for _, c := range expected {
		if c.DeviceID == device {
			only = append(only, c)
		}
	}
	tag, err := tx.Exec(ctx, `INSERT INTO chat_v2_conversations(context_id,epoch,ready,last_sequence) VALUES($1,1,$2,0) ON CONFLICT DO NOTHING`,
		conversation, sameSet(only, expected))
	if err != nil {
		return v, err
	}
	if tag.RowsAffected() == 0 {
		// Already open: idempotent only for the device that opened it and is
		// still its sole member at epoch 1.
		members, err := memberCards(ctx, tx, conversation)
		if err != nil {
			return v, err
		}
		var epoch int64
		if err := tx.QueryRow(ctx, `SELECT epoch FROM chat_v2_conversations WHERE context_id=$1 FOR SHARE`, conversation).Scan(&epoch); err != nil {
			return v, err
		}
		if epoch != 1 || len(members) != 1 || members[0].DeviceID != device {
			return v, ErrExists
		}
	} else if _, err := tx.Exec(ctx, `INSERT INTO chat_v2_members(context_id,device_id,membership_id,first_sequence) VALUES($1,$2,$3,1)`,
		conversation, device, membership); err != nil {
		return v, err
	}
	if err := tx.QueryRow(ctx, `SELECT epoch,last_sequence,ready FROM chat_v2_conversations WHERE context_id=$1`, conversation).Scan(&v.Epoch, &v.LastSequence, &v.Ready); err != nil {
		return v, err
	}
	v.Exists = true
	v.Expected = expected
	if v.Members, err = memberCards(ctx, tx, conversation); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

// ClaimKeyPackages takes one key package for each target device, for a
// member device about to add them. Every target must be expected in the room
// and not yet a member. All or nothing.
func (s *Store) ClaimKeyPackages(ctx context.Context, actor string, digest []byte, device, conversation string, targets []string) ([]Claim, error) {
	if len(targets) < 1 || len(targets) > 500 {
		return nil, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := authorizeSessionWithLock(ctx, tx, actor, device, conversation, lockShare, digest); err != nil {
		return nil, err
	}
	expected, err := expectedRoster(ctx, tx, conversation)
	if err != nil {
		return nil, err
	}
	members, err := memberCards(ctx, tx, conversation)
	if err != nil {
		return nil, err
	}
	want, have := ids(expected), ids(members)
	seen := map[string]bool{}
	out := make([]Claim, 0, len(targets))
	for _, t := range targets {
		card, expectedNow := want[t]
		if !ValidID(t) || seen[t] || !expectedNow {
			return nil, ErrRoster
		}
		if _, member := have[t]; member {
			return nil, ErrRoster
		}
		seen[t] = true
		// Packages held past claimHold are burned: never re-issued (RFC 9420:
		// used once), and no claimer can keep a target's packages tied up
		// (security review 05/10: resource drain).
		if _, err := tx.Exec(ctx, `DELETE FROM chat_v2_key_packages WHERE device_id=$1 AND claimed_at<=clock_timestamp()-$2::interval`, t, claimHold.String()); err != nil {
			return nil, err
		}
		// The package this device already holds for the target, else a fresh
		// one. A handed-out package never goes to another claimer (security
		// review 05/10: two Welcomes to one init key).
		var kp []byte
		err := tx.QueryRow(ctx, `SELECT key_package FROM chat_v2_key_packages
			WHERE device_id=$1 AND claimed_by=$2 AND expires_at>clock_timestamp()
			ORDER BY claimed_at DESC, id LIMIT 1`, t, device).Scan(&kp)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `UPDATE chat_v2_key_packages SET claimed_by=$2, claimed_at=clock_timestamp() WHERE id = (
				SELECT id FROM chat_v2_key_packages WHERE device_id=$1 AND expires_at>clock_timestamp() AND claimed_by IS NULL
				ORDER BY created_at, id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING key_package`, t, device).Scan(&kp)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrKeyPackageUnavailable
		}
		if err != nil {
			return nil, err
		}
		out = append(out, Claim{Card: card, KeyPackage: kp})
	}
	return out, tx.Commit(ctx)
}

// Commit appends a member device's MLS commit when its epoch is still the
// room's (compare-and-swap; the loser gets ErrEpoch), moves the room to the
// next epoch, applies the roster change the server allows, stores the
// Welcome for each added device, and sets ready when the MLS roster is now
// exactly what the server expects.
func (s *Store) Commit(ctx context.Context, actor string, digest []byte, req CommitRequest) (CommitResult, error) {
	var result CommitResult
	e := req.Envelope
	preimage, err := SigningBytes(e)
	if err != nil || len(e.Signature) != ed25519.SignatureSize {
		return result, ErrInvalid
	}
	if (len(req.Added) == 0) != (len(req.Welcome) == 0) || len(req.Welcome) > maxWelcomeBytes || len(req.Added)+len(req.Removed) > 500 {
		return result, ErrInvalid
	}
	release, err := s.writes.acquire(ctx, e.ConversationID)
	if err != nil {
		return result, err
	}
	defer release()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	a, err := authorizeSessionWithLock(ctx, tx, actor, e.DeviceID, e.ConversationID, lockUpdate, digest)
	if err != nil {
		return result, err
	}
	if !ed25519.Verify(a.key, preimage, e.Signature) {
		return result, ErrForbidden
	}
	sum := sha256.Sum256(preimage)
	var existing []byte
	var seq int64
	err = tx.QueryRow(ctx, `SELECT digest,sequence FROM chat_v2_sends WHERE context_id=$1 AND device_id=$2 AND logical_send_id=$3`, e.ConversationID, e.DeviceID, e.LogicalSendID).Scan(&existing, &seq)
	if err == nil {
		if !bytes.Equal(existing, sum[:]) {
			return result, ErrConflict
		}
		if result.Event, err = readEvent(ctx, tx, e.ConversationID, seq); err != nil {
			return result, err
		}
		result.Replayed, result.Ready = true, a.ready
		return result, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	if e.Epoch != a.epoch {
		return result, ErrEpoch
	}
	var recent int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM chat_v2_sends s JOIN chat_v2_events ev ON ev.context_id=s.context_id AND ev.sequence=s.sequence
		WHERE s.device_id=$1 AND ev.kind='commit' AND ev.created_at>clock_timestamp()-interval '1 minute'`, e.DeviceID).Scan(&recent); err != nil {
		return result, err
	}
	if recent >= maxCommitsPerMinute {
		return result, ErrCapacity
	}
	expected, err := expectedRoster(ctx, tx, e.ConversationID)
	if err != nil {
		return result, err
	}
	members, err := memberCards(ctx, tx, e.ConversationID)
	if err != nil {
		return result, err
	}
	want, next := ids(expected), ids(members)
	for _, r := range req.Removed {
		_, member := next[r]
		_, stillExpected := want[r]
		// Only a device the server no longer expects may be removed; no one
		// can push a live member's device out.
		if !member || stillExpected || r == e.DeviceID {
			return result, ErrRoster
		}
		delete(next, r)
	}
	added := map[string]bool{}
	for _, d := range req.Added {
		card, expectedNow := want[d]
		if _, member := next[d]; member || !expectedNow || added[d] {
			return result, ErrRoster
		}
		added[d] = true
		next[d] = card
	}
	roster := make([]Card, 0, len(next))
	for _, c := range next {
		roster = append(roster, c)
	}
	sort.Slice(roster, func(i, j int) bool { return roster[i].DeviceID < roster[j].DeviceID })
	ready := sameSet(roster, expected)
	body, err := json.Marshal(CommitBody{Envelope: e, Roster: roster})
	if err != nil {
		return result, err
	}
	var bodyJSON []byte
	err = tx.QueryRow(ctx, `WITH bumped AS (
 UPDATE chat_v2_conversations SET last_sequence=last_sequence+1, epoch=epoch+1, ready=$6 WHERE context_id=$1 AND epoch=$5 RETURNING last_sequence
), inserted AS (
 INSERT INTO chat_v2_events(context_id,sequence,kind,actor_id,body)
 SELECT $1,last_sequence,'commit',$2,$3 FROM bumped
 RETURNING sequence,kind,actor_id::text,body,created_at
), queued AS (
 INSERT INTO chat_v2_outbox(context_id,sequence) SELECT $1,sequence FROM inserted
), deduplicated AS (
 INSERT INTO chat_v2_sends(context_id,device_id,logical_send_id,digest,sequence)
 SELECT $1,$4::uuid,$7::uuid,$8::bytea,sequence FROM inserted
)
SELECT sequence,kind,actor_id,body,created_at FROM inserted`,
		e.ConversationID, actor, body, e.DeviceID, e.Epoch, ready, e.LogicalSendID, sum[:]).
		Scan(&result.Event.Sequence, &result.Event.Kind, &result.Event.ActorID, &bodyJSON, &result.Event.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, ErrEpoch
	}
	if err != nil {
		return result, err
	}
	if err := decodeBody(&result.Event, bodyJSON); err != nil {
		return result, err
	}
	for _, r := range req.Removed {
		if _, err := tx.Exec(ctx, `DELETE FROM chat_v2_marks WHERE context_id=$1 AND device_id=$2`, e.ConversationID, r); err != nil {
			return result, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM chat_v2_members WHERE context_id=$1 AND device_id=$2`, e.ConversationID, r); err != nil {
			return result, err
		}
	}
	for _, d := range req.Added {
		// The commit consumes the packages this device reserved for d.
		if _, err := tx.Exec(ctx, `DELETE FROM chat_v2_key_packages WHERE device_id=$1 AND claimed_by=$2`, d, e.DeviceID); err != nil {
			return result, err
		}
		membership, err := membershipOf(ctx, tx, e.ConversationID, want[d].ActorID)
		if err != nil {
			return result, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO chat_v2_members(context_id,device_id,membership_id,first_sequence) VALUES($1,$2,$3,$4)`,
			e.ConversationID, d, membership, result.Event.Sequence+1); err != nil {
			return result, err
		}
		wid, err := newUUID()
		if err != nil {
			return result, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO chat_v2_welcomes(id,device_id,context_id,sequence,welcome) VALUES($1,$2,$3,$4,$5)`,
			wid, d, e.ConversationID, result.Event.Sequence, req.Welcome); err != nil {
			return result, err
		}
	}
	result.Ready = ready
	return result, tx.Commit(ctx)
}

// Welcomes are the pending Welcomes of the actor's device, each with the
// roster to verify it against.
func (s *Store) Welcomes(ctx context.Context, actor string, digest []byte, device string) ([]WelcomeView, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := liveSession(ctx, tx, actor, digest, "FOR SHARE"); err != nil {
		return nil, err
	}
	if err := ownDevice(ctx, tx, actor, device); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT w.id::text, w.context_id::text, w.sequence, w.welcome, e.body
		  FROM chat_v2_welcomes w JOIN chat_v2_events e ON e.context_id = w.context_id AND e.sequence = w.sequence
		 WHERE w.device_id=$1 ORDER BY w.created_at, w.id LIMIT 100`, device)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WelcomeView{}
	for rows.Next() {
		var v WelcomeView
		var body []byte
		if err := rows.Scan(&v.ID, &v.ConversationID, &v.Sequence, &v.Welcome, &body); err != nil {
			return nil, err
		}
		var commit CommitBody
		if err := json.Unmarshal(body, &commit); err != nil {
			return nil, err
		}
		v.Roster = commit.Roster
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

func (s *Store) AckWelcome(ctx context.Context, actor string, digest []byte, device, welcome string) error {
	if !ValidID(welcome) {
		return ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := liveSession(ctx, tx, actor, digest, "FOR SHARE"); err != nil {
		return err
	}
	if err := ownDevice(ctx, tx, actor, device); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM chat_v2_welcomes WHERE id=$1 AND device_id=$2`, welcome, device)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrForbidden
	}
	return tx.Commit(ctx)
}

// OnV2Lane is whether the room has its v2 lane open: the legacy writers'
// cutover check (ADR-0057 §8.2).
func (s *Store) OnV2Lane(ctx context.Context, conversation string) (bool, error) {
	if !ValidID(conversation) {
		return false, nil
	}
	var onLane bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat_v2_conversations WHERE context_id=$1)`, conversation).Scan(&onLane)
	return onLane, err
}

// Summary is one end-to-end room as the conversation list shows it: what the
// lane records in the clear, never what anybody said. The device draws the
// preview from its own sealed record (ADR-0057 §8.4).
type Summary struct {
	ConversationID string     `json:"conversation_id"`
	LastSequence   int64      `json:"last_sequence"`
	LastAt         *time.Time `json:"last_at"`
	LastActorID    *string    `json:"last_actor_id"`
	// Messages from others past the furthest point any of the person's
	// devices in the room has read, from where those devices could read.
	Unread       int   `json:"unread"`
	ReadSequence int64 `json:"read_sequence"`
}

// maxUnread bounds the count the list draws («99+» is the screen's word).
const maxUnread = 100

// Summaries lists every v2 room the person is an active member of, with its
// last message's sequence, time and sender (kind 'envelope' only: marks and
// commits are not messages) and the person's unread count. A person with no
// device in a room yet has nothing readable there, so nothing unread.
func (s *Store) Summaries(ctx context.Context, actor string, digest []byte) ([]Summary, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := liveSession(ctx, tx, actor, digest, "FOR SHARE"); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `
WITH mine AS (
  SELECT DISTINCT c.context_id FROM chat_v2_conversations c
  JOIN memberships m ON m.context_id=c.context_id AND m.person_id=$1 AND m.state='active' AND m.left_at IS NULL
), devs AS (
  SELECT cm.context_id, max(coalesce(mk.read_sequence,0)) AS read_seq, min(cm.first_sequence) AS first_seq
  FROM chat_v2_members cm
  JOIN chat_v2_devices d ON d.id=cm.device_id AND d.person_id=$1 AND d.revoked_at IS NULL
  LEFT JOIN chat_v2_marks mk ON mk.context_id=cm.context_id AND mk.device_id=cm.device_id
  WHERE cm.context_id IN (SELECT context_id FROM mine)
  GROUP BY cm.context_id
)
SELECT m.context_id::text, coalesce(last.sequence,0), last.created_at, last.actor_id::text,
  CASE WHEN d.context_id IS NULL THEN 0 ELSE (SELECT count(*) FROM (SELECT 1 FROM chat_v2_events e
    WHERE e.context_id=m.context_id AND e.kind='envelope' AND e.actor_id<>$1
      AND e.sequence > greatest(d.read_seq, d.first_seq-1) LIMIT $2) n) END,
  coalesce(d.read_seq,0)
FROM mine m
LEFT JOIN devs d ON d.context_id=m.context_id
LEFT JOIN LATERAL (SELECT sequence, created_at, actor_id FROM chat_v2_events e
  WHERE e.context_id=m.context_id AND e.kind='envelope' ORDER BY sequence DESC LIMIT 1) last ON true
ORDER BY m.context_id`, actor, maxUnread)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Summary{}
	for rows.Next() {
		var v Summary
		if err := rows.Scan(&v.ConversationID, &v.LastSequence, &v.LastAt, &v.LastActorID, &v.Unread, &v.ReadSequence); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}
