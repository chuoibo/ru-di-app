package chatv2

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"testing"
)

// ADR-0057 lifecycle on a real PostgreSQL: enrollment, key packages, the
// bootstrap, commits under the epoch compare-and-swap, the server's roster
// authority, the Welcome mailbox, and account deletion. MLS bytes are opaque
// synthetic placeholders here: the server never parses them.

type lifeDevice struct {
	id        string
	transport ed25519.PrivateKey
	mls       [32]byte
}

type life struct {
	fixture
	room                     string
	tokenA, tokenB           []byte
	deviceA, deviceB         lifeDevice
	membershipA, membershipB string
}

func newDevice() lifeDevice {
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	d := lifeDevice{id: id(), transport: key}
	_, _ = rand.Read(d.mls[:])
	return d
}

func (d lifeDevice) enrollment(actor string) Enrollment {
	pub := d.transport.Public().(ed25519.PublicKey)
	return Enrollment{DeviceID: d.id, MLSSignatureKey: d.mls[:], TransportSignatureKey: pub,
		Proof: ed25519.Sign(d.transport, EnrollmentBytes(actor, d.id, d.mls[:])), Label: "Máy thử"}
}

func session(t *testing.T, f fixture, person string) []byte {
	t.Helper()
	var token [32]byte
	_, _ = rand.Read(token[:])
	digest := sha256.Sum256(token[:])
	mustExec(t, f.pool, `INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) VALUES($1,$2,$3,'genesis',now()+interval '1 hour')`, id(), person, digest[:])
	return digest[:]
}

// setupLife is the fixture plus a fresh group room with no v2 lane yet, two
// members, each with one enrolled device.
func setupLife(t *testing.T) life {
	t.Helper()
	f := setup(t, "group")
	ctx := context.Background()
	mustExec(t, f.pool, `CREATE TABLE account_sessions (LIKE public.account_sessions INCLUDING ALL)`)
	l := life{fixture: f, room: id(), deviceA: newDevice(), deviceB: newDevice(), membershipA: id(), membershipB: id()}
	mustExec(t, f.pool, `INSERT INTO contexts(id,display_name,created_by_id,kind) VALUES($1,'Phòng thử',$2,'group')`, l.room, f.actor)
	mustExec(t, f.pool, `INSERT INTO memberships(id,context_id,person_id,state,role,origin) VALUES($1,$2,$3,'active','admin','named'),($4,$2,$5,'active','member','named')`,
		l.membershipA, l.room, f.actor, l.membershipB, f.other)
	l.tokenA, l.tokenB = session(t, f, f.actor), session(t, f, f.other)
	if _, err := f.store.EnrollDevice(ctx, f.actor, l.tokenA, l.deviceA.enrollment(f.actor)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.EnrollDevice(ctx, f.other, l.tokenB, l.deviceB.enrollment(f.other)); err != nil {
		t.Fatal(err)
	}
	return l
}

func (l life) envelope(d lifeDevice, epoch int64, logical string) Envelope {
	return sign(d.transport, Envelope{ConversationID: l.room, DeviceID: d.id, LogicalSendID: logical, Protocol: Protocol, Epoch: epoch, Ciphertext: []byte("synthetic opaque MLS bytes")})
}

func TestEnrollmentProvesTheKeyAndCapsFiveDevices(t *testing.T) {
	l := setupLife(t)
	ctx := context.Background()
	again := l.deviceA.enrollment(l.actor)
	if _, err := l.store.EnrollDevice(ctx, l.actor, l.tokenA, again); err != nil {
		t.Fatalf("re-enrolling the same keys is idempotent: %v", err)
	}
	forged := newDevice().enrollment(l.actor)
	forged.Proof = ed25519.Sign(newDevice().transport, EnrollmentBytes(l.actor, forged.DeviceID, forged.MLSSignatureKey))
	if _, err := l.store.EnrollDevice(ctx, l.actor, l.tokenA, forged); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a proof by another key: %v", err)
	}
	stolen := l.deviceB.enrollment(l.actor)
	if _, err := l.store.EnrollDevice(ctx, l.actor, l.tokenA, stolen); !errors.Is(err, ErrConflict) {
		t.Fatalf("another person's device keys under a new id: %v", err)
	}
	for i := 0; i < MaxDevices-1; i++ {
		if _, err := l.store.EnrollDevice(ctx, l.actor, l.tokenA, newDevice().enrollment(l.actor)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := l.store.EnrollDevice(ctx, l.actor, l.tokenA, newDevice().enrollment(l.actor)); !errors.Is(err, ErrDeviceLimit) {
		t.Fatalf("a sixth device: %v", err)
	}
	if _, err := l.store.EnrollDevice(ctx, l.actor, l.tokenB, newDevice().enrollment(l.actor)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("another person's session: %v", err)
	}
	if err := l.store.RevokeDevice(ctx, l.actor, l.tokenA, l.deviceA.id); err != nil {
		t.Fatal(err)
	}
	if _, err := l.store.PublishKeyPackages(ctx, l.actor, l.tokenA, l.deviceA.id, [][]byte{[]byte("kp")}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a revoked device published: %v", err)
	}
}

func TestBootstrapClaimCommitWelcomeAndSend(t *testing.T) {
	l := setupLife(t)
	ctx := context.Background()
	v, err := l.store.Bootstrap(ctx, l.actor, l.tokenA, l.deviceA.id, l.room)
	if err != nil {
		t.Fatal(err)
	}
	if v.Epoch != 1 || v.Ready || len(v.Members) != 1 || len(v.Expected) != 2 {
		t.Fatalf("bootstrap: %+v", v)
	}
	if _, err := l.store.Bootstrap(ctx, l.other, l.tokenB, l.deviceB.id, l.room); !errors.Is(err, ErrExists) {
		t.Fatalf("a second bootstrap: %v", err)
	}
	if n, err := l.store.PublishKeyPackages(ctx, l.other, l.tokenB, l.deviceB.id, [][]byte{[]byte("kp-1"), []byte("kp-2")}); err != nil || n != 2 {
		t.Fatalf("publish: %d %v", n, err)
	}
	if _, err := l.store.ClaimKeyPackages(ctx, l.other, l.tokenB, l.deviceB.id, l.room, []string{l.deviceA.id}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a non-member device claimed: %v", err)
	}
	claims, err := l.store.ClaimKeyPackages(ctx, l.actor, l.tokenA, l.deviceA.id, l.room, []string{l.deviceB.id})
	if err != nil || len(claims) != 1 || string(claims[0].KeyPackage) != "kp-1" || claims[0].Card.DeviceID != l.deviceB.id {
		t.Fatalf("claim: %+v %v", claims, err)
	}

	add := l.envelope(l.deviceA, 1, id())
	res, err := l.store.Commit(ctx, l.actor, l.tokenA, CommitRequest{Envelope: add, Added: []string{l.deviceB.id}, Welcome: []byte("welcome")})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Ready || res.Event.Kind != "commit" || res.Event.Commit == nil || len(res.Event.Commit.Roster) != 2 {
		t.Fatalf("commit: %+v", res)
	}
	again, err := l.store.Commit(ctx, l.actor, l.tokenA, CommitRequest{Envelope: add, Added: []string{l.deviceB.id}, Welcome: []byte("welcome")})
	if err != nil || !again.Replayed || again.Event.Sequence != res.Event.Sequence {
		t.Fatalf("a retried commit is a replay: %+v %v", again, err)
	}

	welcomes, err := l.store.Welcomes(ctx, l.other, l.tokenB, l.deviceB.id)
	if err != nil || len(welcomes) != 1 || string(welcomes[0].Welcome) != "welcome" || len(welcomes[0].Roster) != 2 {
		t.Fatalf("welcomes: %+v %v", welcomes, err)
	}
	if err := l.store.AckWelcome(ctx, l.actor, l.tokenA, l.deviceA.id, welcomes[0].ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("another device acknowledged it: %v", err)
	}
	if err := l.store.AckWelcome(ctx, l.other, l.tokenB, l.deviceB.id, welcomes[0].ID); err != nil {
		t.Fatal(err)
	}
	sent, err := l.store.SendSession(ctx, l.other, l.tokenB, l.envelope(l.deviceB, 2, id()))
	if err != nil {
		t.Fatalf("the added device sends at the new epoch: %v", err)
	}
	page, err := l.store.Events(ctx, l.other, l.deviceB.id, l.room, 0, 10)
	if err != nil || len(page.Events) != 1 || page.Events[0].Sequence != sent.Event.Sequence {
		t.Fatalf("the added device reads from after its commit: %+v %v", page, err)
	}
}

func TestOneCommitWinsAnEpochAndTheServerOwnsTheRoster(t *testing.T) {
	l := setupLife(t)
	ctx := context.Background()
	if _, err := l.store.Bootstrap(ctx, l.actor, l.tokenA, l.deviceA.id, l.room); err != nil {
		t.Fatal(err)
	}
	mustExec(t, l.pool, `INSERT INTO chat_v2_key_packages(id,device_id,key_package,expires_at) VALUES($1,$2,'kp',now()+interval '1 day')`, id(), l.deviceB.id)
	if _, err := l.store.ClaimKeyPackages(ctx, l.actor, l.tokenA, l.deviceA.id, l.room, []string{l.deviceB.id}); err != nil {
		t.Fatal(err)
	}
	stranger := newDevice()
	if _, err := l.store.Commit(ctx, l.actor, l.tokenA, CommitRequest{Envelope: l.envelope(l.deviceA, 1, id()), Added: []string{stranger.id}, Welcome: []byte("w")}); !errors.Is(err, ErrRoster) {
		t.Fatalf("adding a device the server does not expect: %v", err)
	}
	if _, err := l.store.Commit(ctx, l.actor, l.tokenA, CommitRequest{Envelope: l.envelope(l.deviceA, 1, id()), Added: []string{l.deviceB.id}, Welcome: []byte("w")}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.store.Commit(ctx, l.actor, l.tokenA, CommitRequest{Envelope: l.envelope(l.deviceA, 2, id()), Removed: []string{l.deviceB.id}}); !errors.Is(err, ErrRoster) {
		t.Fatalf("pushing a live member's device out: %v", err)
	}
	// Two rekeys at epoch 2: the first wins, the second is told the epoch moved.
	if _, err := l.store.Commit(ctx, l.other, l.tokenB, CommitRequest{Envelope: l.envelope(l.deviceB, 2, id())}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.store.Commit(ctx, l.actor, l.tokenA, CommitRequest{Envelope: l.envelope(l.deviceA, 2, id())}); !errors.Is(err, ErrEpoch) {
		t.Fatalf("the loser of the epoch: %v", err)
	}

	// B leaves: the room stops being ready, B can no longer read, A still can,
	// and A's commit removing B's device makes it ready again.
	mustExec(t, l.pool, `UPDATE memberships SET state='left', left_at=now() WHERE id=$1`, l.membershipB)
	if _, err := l.store.SendSession(ctx, l.actor, l.tokenA, l.envelope(l.deviceA, 3, id())); !errors.Is(err, ErrNotReady) {
		t.Fatalf("sending before the roster catches up: %v", err)
	}
	if _, err := l.store.Events(ctx, l.other, l.deviceB.id, l.room, 0, 10); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a member who left still reads: %v", err)
	}
	if _, err := l.store.Events(ctx, l.actor, l.deviceA.id, l.room, 0, 10); err != nil {
		t.Fatalf("a remaining member cannot read while not ready: %v", err)
	}
	res, err := l.store.Commit(ctx, l.actor, l.tokenA, CommitRequest{Envelope: l.envelope(l.deviceA, 3, id()), Removed: []string{l.deviceB.id}})
	if err != nil || !res.Ready || len(res.Event.Commit.Roster) != 1 {
		t.Fatalf("removing the departed device: %+v %v", res, err)
	}
	if _, err := l.store.SendSession(ctx, l.actor, l.tokenA, l.envelope(l.deviceA, 4, id())); err != nil {
		t.Fatalf("sending after the roster caught up: %v", err)
	}
}

func TestAccountDeletionRevokesDevicesAndErasesTheirMaterial(t *testing.T) {
	l := setupLife(t)
	ctx := context.Background()
	if _, err := l.store.PublishKeyPackages(ctx, l.other, l.tokenB, l.deviceB.id, [][]byte{[]byte("kp")}); err != nil {
		t.Fatal(err)
	}
	mustExec(t, l.pool, `UPDATE people SET deleted_at=now() WHERE id=$1`, l.other)
	var revoked bool
	var packages int
	_ = l.pool.QueryRow(ctx, `SELECT revoked_at IS NOT NULL FROM chat_v2_devices WHERE id=$1`, l.deviceB.id).Scan(&revoked)
	_ = l.pool.QueryRow(ctx, `SELECT count(*) FROM chat_v2_key_packages WHERE device_id=$1`, l.deviceB.id).Scan(&packages)
	if !revoked || packages != 0 {
		t.Fatalf("after deletion: revoked=%v key packages=%d", revoked, packages)
	}
}

// Security review 05/10: claiming reserves, it does not consume. A member who
// claims again and again gets the same package and never drains the target;
// only the commit that adds the device spends it.
func TestClaimingReservesAndOnlyTheAddingCommitConsumes(t *testing.T) {
	l := setupLife(t)
	ctx := context.Background()
	if _, err := l.store.Bootstrap(ctx, l.actor, l.tokenA, l.deviceA.id, l.room); err != nil {
		t.Fatal(err)
	}
	if _, err := l.store.PublishKeyPackages(ctx, l.other, l.tokenB, l.deviceB.id, [][]byte{[]byte("kp-1"), []byte("kp-2")}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		claims, err := l.store.ClaimKeyPackages(ctx, l.actor, l.tokenA, l.deviceA.id, l.room, []string{l.deviceB.id})
		if err != nil || string(claims[0].KeyPackage) != "kp-1" {
			t.Fatalf("claim %d: %+v %v", i, claims, err)
		}
	}
	var left int
	_ = l.pool.QueryRow(ctx, `SELECT count(*) FROM chat_v2_key_packages WHERE device_id=$1`, l.deviceB.id).Scan(&left)
	if left != 2 {
		t.Fatalf("claims drained packages: %d left", left)
	}
	if _, err := l.store.Commit(ctx, l.actor, l.tokenA, CommitRequest{Envelope: l.envelope(l.deviceA, 1, id()), Added: []string{l.deviceB.id}, Welcome: []byte("w")}); err != nil {
		t.Fatal(err)
	}
	_ = l.pool.QueryRow(ctx, `SELECT count(*) FROM chat_v2_key_packages WHERE device_id=$1`, l.deviceB.id).Scan(&left)
	if left != 1 {
		t.Fatalf("the adding commit did not consume its package: %d left", left)
	}
}

// Security review 05/10: each commit moves the epoch under everyone's sends,
// so one device commits at most ten times a minute.
func TestOneDeviceCannotChurnTheEpoch(t *testing.T) {
	l := setupLife(t)
	ctx := context.Background()
	if _, err := l.store.Bootstrap(ctx, l.actor, l.tokenA, l.deviceA.id, l.room); err != nil {
		t.Fatal(err)
	}
	for epoch := int64(1); epoch <= maxCommitsPerMinute; epoch++ {
		if _, err := l.store.Commit(ctx, l.actor, l.tokenA, CommitRequest{Envelope: l.envelope(l.deviceA, epoch, id())}); err != nil {
			t.Fatalf("commit %d: %v", epoch, err)
		}
	}
	if _, err := l.store.Commit(ctx, l.actor, l.tokenA, CommitRequest{Envelope: l.envelope(l.deviceA, maxCommitsPerMinute+1, id())}); !errors.Is(err, ErrCapacity) {
		t.Fatalf("an eleventh commit within the minute: %v", err)
	}
}

// Security review 05/10: a package handed to one adding device never goes to
// another, however long it waits (RFC 9420 §16.8: used once).
func TestAHandedOutKeyPackageNeverGoesToASecondClaimer(t *testing.T) {
	l := setupLife(t)
	ctx := context.Background()
	if _, err := l.store.Bootstrap(ctx, l.actor, l.tokenA, l.deviceA.id, l.room); err != nil {
		t.Fatal(err)
	}
	if _, err := l.store.PublishKeyPackages(ctx, l.other, l.tokenB, l.deviceB.id, [][]byte{[]byte("kp-only")}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.store.ClaimKeyPackages(ctx, l.actor, l.tokenA, l.deviceA.id, l.room, []string{l.deviceB.id}); err != nil {
		t.Fatal(err)
	}
	// Within the hold the claim stays the claimer's; past it the package is
	// burned, never re-issued.
	mustExec(t, l.pool, `UPDATE chat_v2_key_packages SET claimed_at=now()-interval '59 minutes' WHERE device_id=$1`, l.deviceB.id)
	second := newDevice()
	if _, err := l.store.EnrollDevice(ctx, l.actor, l.tokenA, second.enrollment(l.actor)); err != nil {
		t.Fatal(err)
	}
	mustExec(t, l.pool, `INSERT INTO chat_v2_members(context_id,device_id,membership_id,first_sequence) VALUES($1,$2,$3,1)`, l.room, second.id, l.membershipA)
	if _, err := l.store.ClaimKeyPackages(ctx, l.actor, l.tokenA, second.id, l.room, []string{l.deviceB.id}); !errors.Is(err, ErrKeyPackageUnavailable) {
		t.Fatalf("the package went to a second claimer: %v", err)
	}
	if n, err := l.store.PublishKeyPackages(ctx, l.other, l.tokenB, l.deviceB.id, [][]byte{[]byte("kp-fresh")}); err != nil || n != 1 {
		t.Fatalf("the cap counts unclaimed packages: %d %v", n, err)
	}
	if err := l.store.RevokeDevice(ctx, l.actor, l.tokenA, l.deviceA.id); err != nil {
		t.Fatal(err)
	}
	var held int
	_ = l.pool.QueryRow(ctx, `SELECT count(*) FROM chat_v2_key_packages WHERE claimed_by=$1`, l.deviceA.id).Scan(&held)
	if held != 0 {
		t.Fatalf("a revoked device still holds %d packages", held)
	}
}

// Security review 05/10: a held package that never got its commit is burned
// after the hold, so no claimer ties a target's packages up for good, and the
// burned package is not handed to anyone.
func TestAHeldKeyPackageIsBurnedAfterTheHold(t *testing.T) {
	l := setupLife(t)
	ctx := context.Background()
	if _, err := l.store.Bootstrap(ctx, l.actor, l.tokenA, l.deviceA.id, l.room); err != nil {
		t.Fatal(err)
	}
	if _, err := l.store.PublishKeyPackages(ctx, l.other, l.tokenB, l.deviceB.id, [][]byte{[]byte("kp-1"), []byte("kp-2")}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.store.ClaimKeyPackages(ctx, l.actor, l.tokenA, l.deviceA.id, l.room, []string{l.deviceB.id}); err != nil {
		t.Fatal(err)
	}
	mustExec(t, l.pool, `UPDATE chat_v2_key_packages SET claimed_at=now()-interval '2 hours' WHERE device_id=$1 AND claimed_by IS NOT NULL`, l.deviceB.id)
	claims, err := l.store.ClaimKeyPackages(ctx, l.actor, l.tokenA, l.deviceA.id, l.room, []string{l.deviceB.id})
	if err != nil || string(claims[0].KeyPackage) != "kp-2" {
		t.Fatalf("after the hold the claimer gets a fresh package: %+v %v", claims, err)
	}
	var left int
	_ = l.pool.QueryRow(ctx, `SELECT count(*) FROM chat_v2_key_packages WHERE device_id=$1`, l.deviceB.id).Scan(&left)
	if left != 1 {
		t.Fatalf("the burned package is still stored: %d rows", left)
	}
}
