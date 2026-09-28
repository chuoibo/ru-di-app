package testkit

import (
	"context"
	"errors"
	"sync"
	"time"

	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// Nhom is an in-memory tools.DocNhom: the group's outings, soonest first as
// given, and its member count. It ranks nothing; it records which groups
// were read.
type Nhom struct {
	mu        sync.Mutex
	ChuyenDis []truyhoi.BangChung
	SoNguoi   int
	Hoi       []string
}

// ChuyenDi returns at most k of the outings, whatever ngay and sapToi say:
// the fake has no clock.
func (n *Nhom) ChuyenDi(_ context.Context, nhomID string, _ time.Time, _ bool, k int) ([]truyhoi.BangChung, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Hoi = append(n.Hoi, nhomID)
	out := append([]truyhoi.BangChung(nil), n.ChuyenDis...)
	if k > 0 && len(out) > k {
		out = out[:k]
	}
	return out, nil
}

// SoThanhVien is the member count.
func (n *Nhom) SoThanhVien(_ context.Context, nhomID string) (int, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Hoi = append(n.Hoi, nhomID)
	return n.SoNguoi, nil
}

// ErrE2EE is NganHanNhom refusing a lane that is not the legacy one.
var ErrE2EE = errors.New("testkit: no server-side context for an end-to-end encrypted room")

// NganHanNhom is an in-memory aiharness.NganHanNhom: one session per group
// invocation, every turn kept and returned (trinho.MaxLuotNhom), and no
// session named for any lane but the legacy one (the aictx rule).
type NganHanNhom struct {
	mu    sync.Mutex
	phien map[string][]trinho.Luot
	// DaViet counts every turn written, for the canaries of a v2 room.
	DaViet int
	DaXoa  []string
}

// MoiNganHanNhom makes an empty store.
func MoiNganHanNhom() *NganHanNhom { return &NganHanNhom{phien: map[string][]trinho.Luot{}} }

// PhienLuotNhom names the session of one invocation of room phong.
func (n *NganHanNhom) PhienLuotNhom(phong, luot, lane string) (string, error) {
	if lane != "legacy" {
		return "", ErrE2EE
	}
	return "grp:" + phong + ":" + luot, nil
}

// Doc returns every turn of the session, oldest first, at most
// trinho.MaxLuotNhom.
func (n *NganHanNhom) Doc(_ context.Context, phien string) ([]trinho.Luot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	l := n.phien[phien]
	if len(l) > trinho.MaxLuotNhom {
		l = l[len(l)-trinho.MaxLuotNhom:]
	}
	return append([]trinho.Luot(nil), l...), nil
}

// Them appends a turn with a known speaker.
func (n *NganHanNhom) Them(_ context.Context, phien string, l trinho.Luot) error {
	if !trinho.VaiLuots.Co(l.Vai) {
		return errors.New("testkit: unknown speaker")
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.phien[phien] = append(n.phien[phien], l)
	n.DaViet++
	return nil
}

// Xoa drops the session.
func (n *NganHanNhom) Xoa(_ context.Context, phien string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.phien, phien)
	n.DaXoa = append(n.DaXoa, phien)
	return nil
}

// ConPhien is how many sessions are still held.
func (n *NganHanNhom) ConPhien() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.phien)
}
