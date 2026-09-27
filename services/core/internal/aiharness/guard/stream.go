package guard

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// CuaSoRune is the streaming output guard's window (design 01 §3.5, contract
// §2). The invariant, exactly: a rune of an answer leaves the window only
// when (1) the guard's scan (kiem) of everything seen so far is clean, (2) at
// least this many runes follow it in what has been seen, and (3) the release
// ends at white space -- the release point is always a white space at least
// CuaSoRune runes back from the end of what was seen -- or the answer has
// ended and the scan of all of it is clean.
//
// Since the owner's rule of 2026-09-25 the guard holds data-format
// validators only (the canary marker, an email, a phone, an account or card
// number, a quoted prompt clause); whether an answer claims an action or
// money is the verifier's judgement, and the engine streams only an answer
// the verifier has passed (aiharness.phatRa). Each format falls in one of two
// classes, and the invariant covers both:
//
//   - a pattern with no white space in it (an email, the canary marker, an
//     unbroken digit run), whatever its length, never leaves in part: by (3)
//     a release never ends inside a token;
//   - a pattern that spans white space is flagged within CuaSoRune runes of
//     its first rune: a digit run is flagged once it holds nine digits, at
//     most 33 runes with the widest separator (« - »), a grouped phone is
//     shorter, and a prompt clause is looked for by its first dauLoiNhacRune
//     runes. By (2) it is whole in what was seen before its first rune
//     could leave.
//
// A new pattern that fits neither class breaks this contract and must not be
// added to the guard without widening the window.
const CuaSoRune = 48

// dauLoiNhacRune is how much of a prompt clause the window looks for. The
// whole-answer check (DauRa.Kiem) stops an answer that quotes a whole clause
// of the prompt, and a clause can run past the window: its head would leave
// before its tail was seen. The window stops on the clause's first runes
// instead, which fit, so no quoted clause leaves even in part. It is shorter
// than the window because folding (Gap) drops marks: a clause written with
// combining marks is longer in runes than its folded form.
const dauLoiNhacRune = 40

// NoiChan joins the released text and the fixed sentence when the guard stops
// an answer part-way.
const NoiChan = "\n\n"

// NhanDelta is what the window releases text to: the engine's Sink. The
// window is the only code that calls Delta outside tests
// (aigate/stream_gate_test.go holds it to that).
type NhanDelta interface {
	Delta(p int, text string)
}

// KetQuaCuaSo is how an answer left the window.
type KetQuaCuaSo struct {
	// Chu is the final text: everything released, then -- when the guard
	// stopped the answer after something had left -- NoiChan and the fixed
	// sentence. The Deltas joined are exactly Chu.
	Chu string
	// DaNha is the model's own text that left, without the fixed sentence.
	DaNha string
	// Chan is why the guard stopped the answer; RaSach when it did not.
	Chan LoaiRa
	// KhongHopLe: the answer ran past the cap or was not UTF-8. What left
	// stays; nothing more goes, and there is no fixed sentence.
	KhongHopLe bool
}

// CuaSo is one answer part's window. Not safe for concurrent use: the engine
// feeds it from the goroutine that reads the model's stream.
type CuaSo struct {
	ra NhanDelta
	p  int
	d  DauRa
	// dau are the folded heads of the prompt's clauses (dauLoiNhacRune).
	dau []string
	// tran is the answer's cap in runes; 0 is none.
	tran int
	// cauChan is the fixed sentence that ends a stopped answer ("" for none).
	cauChan string

	chu []rune
	// du holds the bytes of a rune a chunk cut in two.
	du []byte
	// nha is how many runes of chu have been released.
	nha int
	// nghiTu is len(chu) when the scan first flagged what was seen and has
	// flagged ever since; -1 while the last scan was clean.
	nghiTu int
	dung   bool
	xong   bool
	loai   LoaiRa
	hong   bool
	kq     KetQuaCuaSo
}

// MoCuaSo opens a window for answer part p. d is the output guard's
// knowledge (the canary marker, the prompt's clauses), tran the cap in runes
// (0 for none), cauChan the fixed sentence of design 01 §3.5 that follows
// released text when the guard stops the rest ("" to add none).
func MoCuaSo(ra NhanDelta, p int, d DauRa, tran int, cauChan string) *CuaSo {
	c := &CuaSo{ra: ra, p: p, d: d, tran: tran, cauChan: cauChan, nghiTu: -1}
	for _, line := range d.LoiNhac {
		g := []rune(strings.TrimSpace(Gap(line)))
		if len(g) > dauLoiNhacRune {
			g = g[:dauLoiNhacRune]
		}
		if len(g) > 0 {
			c.dau = append(c.dau, string(g))
		}
	}
	return c
}

// KiemCuaSo is the scan the window runs before it releases anything, for
// checks outside the engine: the eval's invariant 8 holds every Delta to it.
func KiemCuaSo(d DauRa, text string) LoaiRa { return MoCuaSo(nil, 0, d, 0, "").kiem(text) }

// kiem is the scan the window runs: the whole-answer output guard, then the
// heads of the prompt's clauses.
func (c *CuaSo) kiem(text string) LoaiRa {
	if l := c.d.Kiem(text); l != RaSach {
		return l
	}
	if len(c.dau) > 0 {
		g := Gap(text)
		for _, h := range c.dau {
			if strings.Contains(g, h) {
				return RaLoiNhac
			}
		}
	}
	return RaSach
}

// nap appends a chunk to what has been seen: leading white space of the
// answer is dropped, and a rune cut between two chunks waits for its tail.
func (c *CuaSo) nap(chunk string) {
	b := append(c.du, chunk...)
	c.du = nil
	for len(b) > 0 {
		if !utf8.FullRune(b) {
			c.du = append([]byte(nil), b...)
			return
		}
		r, n := utf8.DecodeRune(b)
		b = b[n:]
		if r == utf8.RuneError && n == 1 {
			c.hong = true
			continue
		}
		if len(c.chu) == 0 && unicode.IsSpace(r) {
			continue
		}
		c.chu = append(c.chu, r)
	}
}

// Viet feeds one chunk of the answer. It releases, as one Delta, everything
// up to the last white space that now has CuaSoRune runes after it, once the
// guard has read all that was seen and found nothing. While the guard flags what was seen, nothing
// is released: a pattern cut off at the end of what was seen may be cleared
// by the runes after it (a sum, then its currency). A flag that holds while
// CuaSoRune more runes arrive is a violation: the window stops, and nothing
// it held ever leaves. It reports whether the window still takes text; the
// engine stops the model when it does not.
func (c *CuaSo) Viet(chunk string) bool {
	if c.dung || c.xong {
		return false
	}
	c.nap(chunk)
	if c.hong || (c.tran > 0 && len(c.chu) > c.tran) {
		c.dung = true
		return false
	}
	// The release point: the last white space at least CuaSoRune runes back,
	// released with it, so a token never leaves unfinished.
	moi := len(c.chu) - CuaSoRune
	for moi > c.nha && !unicode.IsSpace(c.chu[moi-1]) {
		moi--
	}
	if moi <= c.nha {
		return true
	}
	if l := c.kiem(string(c.chu)); l != RaSach {
		if c.nghiTu < 0 {
			c.nghiTu = len(c.chu)
		}
		if len(c.chu)-c.nghiTu >= CuaSoRune {
			c.dung, c.loai = true, l
			return false
		}
		return true
	}
	c.nghiTu = -1
	c.ra.Delta(c.p, string(c.chu[c.nha:moi]))
	c.nha = moi
	return true
}

// DaDung reports whether the window stopped taking text: a violation, the
// cap, or text that is not UTF-8.
func (c *CuaSo) DaDung() bool { return c.dung }

// Xong ends the answer: the guard reads everything seen, and either the rest
// is released as one Delta (trailing white space dropped) or nothing more is,
// and -- when something had already left -- the fixed sentence follows as the
// last Delta. It can be called more than once; later calls return the same.
func (c *CuaSo) Xong() KetQuaCuaSo {
	if c.xong {
		return c.kq
	}
	c.xong = true
	if len(c.du) > 0 {
		c.hong, c.dung = true, true
	}
	if !c.dung {
		end := len(c.chu)
		for end > c.nha && unicode.IsSpace(c.chu[end-1]) {
			end--
		}
		c.chu = c.chu[:end]
		if l := c.kiem(string(c.chu)); l != RaSach {
			c.dung, c.loai = true, l
		} else if end > c.nha {
			c.ra.Delta(c.p, string(c.chu[c.nha:end]))
			c.nha = end
		}
	}
	daNha := string(c.chu[:c.nha])
	c.kq = KetQuaCuaSo{Chu: daNha, DaNha: daNha, Chan: c.loai, KhongHopLe: c.hong || (c.dung && c.loai == RaSach)}
	if c.loai != RaSach && c.nha > 0 && c.cauChan != "" {
		c.ra.Delta(c.p, NoiChan+c.cauChan)
		c.kq.Chu = daNha + NoiChan + c.cauChan
	}
	return c.kq
}

// QuaCuaSo passes a finished text through a window as one final Delta: the
// guard reads it whole and it leaves at once, or nothing of it does. It is
// how an answer that did not stream (the group's brain path) reaches the
// stream.
func QuaCuaSo(ra NhanDelta, p int, d DauRa, tran int, cauChan, text string) KetQuaCuaSo {
	c := MoCuaSo(ra, p, d, tran, cauChan)
	c.nap(text)
	if c.hong || (c.tran > 0 && len(c.chu) > c.tran) {
		c.dung = true
	}
	return c.Xong()
}

// ManhPhat is how many runes of a finished text PhatTheoNhip hands the window
// at a time. The window, not this number, decides where a Delta ends (at
// white space, CuaSoRune runes back).
const ManhPhat = 16

// TranNhip bounds the whole pacing of one text: however long the text, its
// release is spread over at most this long, so pacing never costs a long
// answer more than this before its xong. It is a bound on the waiting, laid
// out as deadlines from the start (chunk i is due at start + i·nhip, never
// later than start + TranNhip), so a timer that fires late or a slow window
// scan on one chunk is absorbed by the next wait instead of adding up.
const TranNhip = 800 * time.Millisecond

// DongHo is the clock PhatTheoNhip paces by: the real one in production, a
// virtual one in tests, so the pacing bound is checked on the schedule it
// lays out rather than on a loaded machine's wall time.
type DongHo interface {
	Now() time.Time
	// Sau fires once d has passed.
	Sau(d time.Duration) (<-chan time.Time, func())
}

type dongHoThat struct{}

func (dongHoThat) Now() time.Time { return time.Now() }

func (dongHoThat) Sau(d time.Duration) (<-chan time.Time, func()) {
	t := time.NewTimer(d)
	return t.C, func() { t.Stop() }
}

// PhatTheoNhip releases a text that is already whole -- verified by the
// engine, or finished by the brain -- through a window, a ManhPhat-rune chunk
// at a time and nhip apart (0: no pause; the spacing is shortened so the
// whole text is due within TranNhip), so a client that renders each Delta as
// it comes shows the answer progressively. The window's scan reads the whole
// text first: a text it would stop releases nothing at all (Chan is set and
// Chu is ""), so the window never has to stop a text it has already released
// part of. A cancelled ctx stops the release between chunks with ctx's error;
// what left stays.
func PhatTheoNhip(ctx context.Context, ra NhanDelta, p int, d DauRa, tran int, cauChan, text string, nhip time.Duration) (KetQuaCuaSo, error) {
	return PhatTheoNhipVoi(ctx, dongHoThat{}, ra, p, d, tran, cauChan, text, nhip)
}

// PhatTheoNhipVoi is PhatTheoNhip on clock dh. Chunk i (0-based) is due at
// start + i·nhip, and no due time is past start + TranNhip: each wait is
// measured from the clock's now to the chunk's due time, so lateness never
// accumulates.
func PhatTheoNhipVoi(ctx context.Context, dh DongHo, ra NhanDelta, p int, d DauRa, tran int, cauChan, text string, nhip time.Duration) (KetQuaCuaSo, error) {
	c := MoCuaSo(ra, p, d, tran, cauChan)
	if l := c.kiem(text); l != RaSach {
		return KetQuaCuaSo{Chan: l}, nil
	}
	r := []rune(text)
	n := (len(r) + ManhPhat - 1) / ManhPhat
	if n > 1 && nhip > 0 && nhip*time.Duration(n-1) > TranNhip {
		nhip = TranNhip / time.Duration(n-1)
	}
	batDau := dh.Now()
	for k, i := 0, 0; i < len(r); k, i = k+1, i+ManhPhat {
		if k > 0 && nhip > 0 {
			den := min(time.Duration(k)*nhip, TranNhip)
			if cho := den - dh.Now().Sub(batDau); cho > 0 {
				ch, dung := dh.Sau(cho)
				select {
				case <-ctx.Done():
					dung()
					return KetQuaCuaSo{}, ctx.Err()
				case <-ch:
				}
			} else if err := ctx.Err(); err != nil {
				return KetQuaCuaSo{}, err
			}
		}
		if !c.Viet(string(r[i:min(i+ManhPhat, len(r))])) {
			break
		}
	}
	return c.Xong(), nil
}
