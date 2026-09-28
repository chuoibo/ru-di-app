// Package aictx is the short-term memory of the AI engine on Redis
// (research stm-personalization §5, option A, compatible with ADR-0036 §4):
// the only reader and writer of the aictx keys, and the Redis adapter of
// trinho.NganHan.
//
// What it holds, and for how long:
//   - Nếp: a per-turn buffer of the turns the app sent with one invocation
//     (the client stays the source of the session), key
//     …:aictx:nep:<person>:<turn>, 5 minutes, unlinked when the job ends.
//     A per-person index set lists the live buffers, so a forget-all or an
//     account deletion unlinks them exactly, with no SCAN.
//   - The group, legacy lane only: the caller's context package of one
//     invocation, key …:aictx:grp:<room>:<invocation>, 15 minutes (EX 900,
//     the sharing window), unlinked when the job ends.
//   - An end-to-end encrypted (v2) room: nothing. No key can be named for
//     it; the package's constructor refuses the lane.
//
// Every TTL is set in the same MULTI as the write, and only when the key has
// none (EXPIRE NX), so no key ever exists without one and no write extends a
// window. Every value is sealed with AES-256-GCM under a key from the
// environment, the key name as associated data: a Redis dump, replica or
// core file shows no words. The instance must not persist (save "",
// appendonly no); KiemCauHinh checks at startup.
package aictx

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"mobile/services/core/internal/aiharness/trinho"
)

// EnvKhoa names the sealing key: base64 of exactly 32 random bytes.
const EnvKhoa = "MOBILE_AICTX_KEY"

// EnvRedisURL names the dedicated redis-ai instance.
const EnvRedisURL = "MOBILE_AICTX_REDIS_URL"

const (
	// TTLNep bounds a Nếp turn buffer: the 8 s turn plus the queue's
	// retries (research stm §5.1).
	TTLNep = 5 * time.Minute
	// TTLNhom bounds a legacy-lane group buffer: the 15-minute sharing
	// window (ADR-0045 §6).
	TTLNhom = 15 * time.Minute
	// GiuToiDa is how many turns a Nếp buffer keeps; Doc returns the newest
	// trinho.MaxLuotNganHan of them.
	GiuToiDa = 12
	// GiuToiDaNhom is how many turns a group buffer keeps and Doc returns:
	// every shared turn of the bundle (trinho.MaxLuotNhom), since the card
	// says how many the assistant read.
	GiuToiDaNhom = trinho.MaxLuotNhom
	// MaxChu bounds one turn's words.
	MaxChu = 4000
)

// Lane is a room's transport as the server knows it.
type Lane string

const (
	LaneLegacy Lane = "legacy"
	LaneV2     Lane = "v2"
)

var (
	// ErrE2EE: a buffer asked for an end-to-end encrypted room.
	ErrE2EE = errors.New("aictx: no server-side context for an end-to-end encrypted room")
	// ErrPhien: a session id this package did not build.
	ErrPhien = errors.New("aictx: invalid session id")
	// ErrMoKhoa: a value that does not open under the key.
	ErrMoKhoa = errors.New("aictx: value does not open under the current key")
)

var (
	namespacePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,48}$`)
	idPattern        = regexp.MustCompile(`^[0-9a-fA-F-]{8,64}$`)
)

// Kho is the Redis short-term memory.
type Kho struct {
	client *redis.Client
	prefix string
	aead   cipher.AEAD
	now    func() time.Time
}

var _ trinho.NganHan = (*Kho)(nil)

// DocKhoa decodes the sealing key from its environment value.
func DocKhoa(raw string) ([]byte, error) {
	k, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil || len(k) != 32 {
		return nil, fmt.Errorf("aictx: %s must be base64 of 32 bytes", EnvKhoa)
	}
	return k, nil
}

// Moi builds the store on client. namespace separates deployments sharing
// an instance.
func Moi(client *redis.Client, namespace string, khoa []byte) (*Kho, error) {
	if !namespacePattern.MatchString(namespace) {
		return nil, errors.New("aictx: invalid namespace")
	}
	if len(khoa) != 32 {
		return nil, errors.New("aictx: the sealing key must be 32 bytes")
	}
	block, err := aes.NewCipher(khoa)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Kho{client: client, prefix: "rudi:" + namespace + ":aictx:", aead: aead, now: time.Now}, nil
}

// Open connects to url and builds the store.
func Open(url, namespace string, khoa []byte) (*Kho, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, errors.New("aictx: invalid Redis URL")
	}
	return Moi(redis.NewClient(opts), namespace, khoa)
}

// Close closes the client.
func (k *Kho) Close() error { return k.client.Close() }

// PhienNep names the buffer of one Nếp turn of person nguoi.
func PhienNep(nguoi, luot string) (string, error) {
	if !idPattern.MatchString(nguoi) || !idPattern.MatchString(luot) {
		return "", ErrPhien
	}
	return "nep:" + nguoi + ":" + luot, nil
}

// PhienNhom names the buffer of one group invocation. Only the legacy lane
// has one: for a v2 room it refuses, and nothing is ever written.
func PhienNhom(phong, invocation string, lane Lane) (string, error) {
	if lane != LaneLegacy {
		return "", ErrE2EE
	}
	if !idPattern.MatchString(phong) || !idPattern.MatchString(invocation) {
		return "", ErrPhien
	}
	return "grp:" + phong + ":" + invocation, nil
}

// phien is a parsed session id.
type phien struct {
	loai, chu, id string
}

func docPhien(s string) (phien, error) {
	p := strings.Split(s, ":")
	if len(p) != 3 || (p[0] != "nep" && p[0] != "grp") || !idPattern.MatchString(p[1]) || !idPattern.MatchString(p[2]) {
		return phien{}, ErrPhien
	}
	return phien{loai: p[0], chu: p[1], id: p[2]}, nil
}

func (k *Kho) khoa(p phien) string { return k.prefix + p.loai + ":" + p.chu + ":" + p.id }

func (k *Kho) chiMuc(nguoi string) string { return k.prefix + "idx:" + nguoi }

func ttl(p phien) time.Duration {
	if p.loai == "nep" {
		return TTLNep
	}
	return TTLNhom
}

// giu is how many turns a buffer keeps, and doc how many Doc returns.
func giu(p phien) int64 {
	if p.loai == "nep" {
		return GiuToiDa
	}
	return GiuToiDaNhom
}

func doc(p phien) int64 {
	if p.loai == "nep" {
		return trinho.MaxLuotNganHan
	}
	return trinho.MaxLuotNhom
}

// Owns reports whether a Redis key belongs to this package in this
// namespace; the gates use it.
func (k *Kho) Owns(key string) bool { return strings.HasPrefix(key, k.prefix) }

type luotJSON struct {
	Vai          string    `json:"v"`
	Chu          string    `json:"c"`
	Luc          time.Time `json:"l"`
	BangChungIDs []string  `json:"b,omitempty"`
}

func (k *Kho) niem(key string, l trinho.Luot) (string, error) {
	raw, err := json.Marshal(luotJSON{Vai: string(l.Vai), Chu: l.Chu, Luc: l.Luc.UTC(), BangChungIDs: l.BangChungIDs})
	if err != nil {
		return "", err
	}
	nonce := make([]byte, k.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return string(k.aead.Seal(nonce, nonce, raw, []byte(key))), nil
}

func (k *Kho) mo(key, v string) (trinho.Luot, error) {
	b := []byte(v)
	n := k.aead.NonceSize()
	if len(b) < n {
		return trinho.Luot{}, ErrMoKhoa
	}
	raw, err := k.aead.Open(nil, b[:n], b[n:], []byte(key))
	if err != nil {
		return trinho.Luot{}, ErrMoKhoa
	}
	var j luotJSON
	if err := json.Unmarshal(raw, &j); err != nil {
		return trinho.Luot{}, ErrMoKhoa
	}
	return trinho.Luot{Vai: trinho.VaiLuot(j.Vai), Chu: j.Chu, Luc: j.Luc, BangChungIDs: j.BangChungIDs}, nil
}

// Them implements trinho.NganHan: one sealed turn appended, the buffer cut
// to GiuToiDa, the TTL set if the key had none, and for Nếp the buffer
// entered in the person's index -- all in one MULTI.
func (k *Kho) Them(ctx context.Context, s string, l trinho.Luot) error {
	p, err := docPhien(s)
	if err != nil {
		return err
	}
	if !trinho.VaiLuots.Co(l.Vai) || l.Chu == "" || len([]rune(l.Chu)) > MaxChu || l.Luc.IsZero() {
		return errors.New("aictx: turn outside its shape")
	}
	key := k.khoa(p)
	v, err := k.niem(key, l)
	if err != nil {
		return err
	}
	han := ttl(p)
	_, err = k.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.RPush(ctx, key, v)
		pipe.LTrim(ctx, key, -giu(p), -1)
		pipe.ExpireNX(ctx, key, han)
		if p.loai == "nep" {
			idx := k.chiMuc(p.chu)
			pipe.SAdd(ctx, idx, key)
			pipe.ExpireNX(ctx, idx, han)
			pipe.ExpireGT(ctx, idx, han)
		}
		return nil
	})
	return err
}

// Doc implements trinho.NganHan: the newest trinho.MaxLuotNganHan turns of
// a Nếp buffer, every turn of a group buffer (trinho.MaxLuotNhom), oldest
// first.
func (k *Kho) Doc(ctx context.Context, s string) ([]trinho.Luot, error) {
	p, err := docPhien(s)
	if err != nil {
		return nil, err
	}
	key := k.khoa(p)
	vs, err := k.client.LRange(ctx, key, -doc(p), -1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]trinho.Luot, 0, len(vs))
	for _, v := range vs {
		l, err := k.mo(key, v)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

// Xoa implements trinho.NganHan: UNLINK, never waiting for the TTL.
func (k *Kho) Xoa(ctx context.Context, s string) error {
	p, err := docPhien(s)
	if err != nil {
		return err
	}
	key := k.khoa(p)
	_, err = k.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Unlink(ctx, key)
		if p.loai == "nep" {
			pipe.SRem(ctx, k.chiMuc(p.chu), key)
		}
		return nil
	})
	return err
}

// XoaNguoi unlinks every live Nếp buffer of a person and the index, and
// returns how many keys went. nepnho calls it in a forget-all and an account
// deletion (nepnho.XoaNganHan).
func (k *Kho) XoaNguoi(ctx context.Context, nguoi string) (int, error) {
	if !idPattern.MatchString(nguoi) {
		return 0, ErrPhien
	}
	idx := k.chiMuc(nguoi)
	keys, err := k.client.SMembers(ctx, idx).Result()
	if err != nil {
		return 0, err
	}
	var own []string
	for _, key := range keys {
		// The index names only this person's buffers; anything else in it
		// is not ours to delete.
		if strings.HasPrefix(key, k.prefix+"nep:"+nguoi+":") {
			own = append(own, key)
		}
	}
	n, err := k.client.Unlink(ctx, append(own, idx)...).Result()
	if err != nil {
		return 0, err
	}
	if n > 0 && len(keys) > 0 {
		n-- // the index itself
	}
	return int(n), nil
}

// PhienLuot implements aiharness.NganHanLuot: the buffer of one Nếp turn,
// named by the asking person and the invocation (PhienNep).
func (k *Kho) PhienLuot(nguoi, luot string) (string, error) { return PhienNep(nguoi, luot) }

// PhienLuotNhom implements aiharness.NganHanNhom: the buffer of one group
// invocation in room phong (PhienNhom). lane is the room's transport as the
// server derived it; anything but the legacy lane is refused (ErrE2EE), so
// nothing of an end-to-end encrypted room is ever written.
func (k *Kho) PhienLuotNhom(phong, luot, lane string) (string, error) {
	return PhienNhom(phong, luot, Lane(lane))
}
