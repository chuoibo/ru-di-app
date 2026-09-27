package nepnho

import (
	"context"
	"crypto/rand"
	"fmt"
	"sort"
	"sync"
)

// khoGia is the in-process fake of the memory sidecar. It keeps the words
// per owner, the way the sidecar keeps rows in its collection. It ranks
// nothing: Tim returns the owner's memories newest first, cut at k
// (relevance is the real sidecar's), and it extracts nothing: Them keeps the
// sentence as it came. A fake is never evidence about the sidecar or about
// Milvus.
type khoGia struct {
	mu  sync.Mutex
	muc map[string][]MucNho // owner -> memories, oldest first
	goi map[string]int      // calls by method
	// knobs
	vang    bool   // every call fails as unreachable
	ro      string // Tim also returns this other owner's memories (a leak)
	conSot  int    // the next deletes answer "remaining" (500), once per call
	noiDoi  bool   // deletes answer remaining 0 but keep the rows (a lying sidecar)
	idSai   bool   // Them answers a non-UUID id
	tuChoi  bool   // Them keeps nothing (the extraction refused)
	tach    bool   // Them keeps the sentence as two memories
	loaiRut string // Them records this kind
	timN    int    // the k of the last Tim
	// trongXoa runs inside every delete call, while the caller waits on
	// the sidecar.
	trongXoa func()
}

func moiKhoGia() *khoGia {
	return &khoGia{muc: map[string][]MucNho{}, goi: map[string]int{}}
}

func uuidMoi() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (k *khoGia) dem(m string) error {
	k.goi[m]++
	if k.vang {
		return fmt.Errorf("%w: fake", ErrDichVuVang)
	}
	return nil
}

func (k *khoGia) Them(_ context.Context, owner, cau string) ([]MucNho, int, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if err := k.dem("them"); err != nil {
		return nil, 0, err
	}
	if k.tuChoi {
		return nil, 1, nil
	}
	texts := []string{cau}
	if k.tach {
		texts = []string{cau, cau + " (2)"}
	}
	var out []MucNho
	for _, t := range texts {
		id := uuidMoi()
		if k.idSai {
			id = "mem-khong-phai-uuid"
		}
		m := MucNho{ID: id, Text: t, Loai: k.loaiRut}
		k.muc[owner] = append(k.muc[owner], m)
		out = append(out, m)
	}
	return out, 0, nil
}

func (k *khoGia) Tim(_ context.Context, owner, _ string, n int) ([]MucNho, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if err := k.dem("tim"); err != nil {
		return nil, err
	}
	if n < 1 || n > 10 {
		// The sidecar's SearchReq: top_k 1..10, a 422 otherwise.
		return nil, fmt.Errorf("%w: search 422 for top_k %d", ErrDichVuSai, n)
	}
	k.timN = n
	all := append([]MucNho(nil), k.muc[owner]...)
	if k.ro != "" {
		all = append(all, k.muc[k.ro]...)
	}
	var out []MucNho
	for i := len(all) - 1; i >= 0 && len(out) < n; i-- {
		out = append(out, all[i])
	}
	return out, nil
}

func (k *khoGia) LietKe(_ context.Context, owner string) ([]MucNho, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if err := k.dem("lietke"); err != nil {
		return nil, err
	}
	return append([]MucNho(nil), k.muc[owner]...), nil
}

// xoa removes owner's rows (all when id is ""), honouring the knobs.
func (k *khoGia) xoa(method, owner, id string) (int, error) {
	if k.trongXoa != nil {
		k.trongXoa()
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if err := k.dem(method); err != nil {
		return 0, err
	}
	n := 0
	var giu []MucNho
	for _, m := range k.muc[owner] {
		if id == "" || m.ID == id {
			n++
			if k.noiDoi {
				giu = append(giu, m)
			}
			continue
		}
		giu = append(giu, m)
	}
	if id != "" && n == 0 {
		return 0, nil // the sidecar's 404: gone, or never this owner's
	}
	if k.conSot > 0 {
		k.conSot--
		return 0, fmt.Errorf("%w: fake left 1", ErrConHang)
	}
	k.muc[owner] = giu
	return n, nil
}

func (k *khoGia) Xoa(_ context.Context, owner, id string) (int, error) {
	return k.xoa("xoa", owner, id)
}

func (k *khoGia) XoaHet(_ context.Context, owner string) (int, error) {
	return k.xoa("xoahet", owner, "")
}

func (k *khoGia) XoaSach(_ context.Context, owner string) (int, error) {
	return k.xoa("xoasach", owner, "")
}

// owners lists the owners the fake still holds rows for.
func (k *khoGia) owners() []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []string
	for o, ms := range k.muc {
		if len(ms) > 0 {
			out = append(out, o)
		}
	}
	sort.Strings(out)
	return out
}
