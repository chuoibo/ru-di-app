package aieval

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"

	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aiharness/obs"
)

// Gieo is what one run of a case is seeded with.
type Gieo struct {
	// Turn is the engine's turn, built from the case exactly as the worker
	// builds it from a stored job (chatassist luotEngine): the slip, the
	// session and the question the device sent, and the instant the question
	// was stored -- nothing looked up.
	Turn aiharness.Turn
	// MaKiem is the canary marker the engine puts in its system instruction.
	// The engine draws a random one per process; the eval fixes one per case,
	// so a run is byte for byte repeatable and a script can play a leak.
	MaKiem string
}

// GieoCa seeds run lap (1-based) of c.
func GieoCa(c Ca, lap int) (Gieo, error) {
	luc, err := c.Luc()
	if err != nil {
		return Gieo{}, err
	}
	if lap < 1 {
		return Gieo{}, fmt.Errorf("lap %d < 1", lap)
	}
	t := aiharness.Turn{
		Bot:          obs.Bot(c.BeMat),
		InvocationID: idCua(c.CaID, lap),
		LanThu:       1,
		Lenh:         obs.Lenh(c.Lenh),
		// created_at comes back from Postgres as an instant; UTC is its plain
		// form. The engine must get Vietnam's wall clock from it on its own.
		Luc:        luc.UTC(),
		LoiNho:     c.DauVao.LoiNho,
		DaGoiTruoc: c.DauVao.DaGoiTruoc,
	}
	if p := c.DauVao.Phieu; p != nil {
		t.PhieuNep = &aiharness.PhieuNep{Man: p.Man, TieuDe: p.TieuDe, LoaiSo: p.LoaiSo, SoLieu: p.SoLieu, GoiY: p.GoiY}
		if p.Nhip != nil {
			t.PhieuNep.Nhip = &aiharness.Nhip{Kieu: p.Nhip.Kieu, ConNgay: p.Nhip.ConNgay, TruocNgay: p.Nhip.TruocNgay}
		}
	}
	for _, l := range c.DauVao.Luot {
		t.LuotNep = append(t.LuotNep, aiharness.LuotNep{Vai: l.Vai, Chu: l.Chu})
	}
	return Gieo{Turn: t, MaKiem: maKiemCua(c.CaID)}, nil
}

// idCua is a UUID-shaped id derived from the case and the run, so the log
// line and the record are repeatable too.
func idCua(caID string, lap int) string {
	sum := sha256.Sum256([]byte("invocation|" + caID + "|" + strconv.Itoa(lap)))
	h := hex.EncodeToString(sum[:16])
	return h[0:8] + "-" + h[8:12] + "-5" + h[13:16] + "-8" + h[17:20] + "-" + h[20:32]
}

// maKiemCua is the case's canary marker: twelve hex digits, the engine's own
// shape (aiharness New).
func maKiemCua(caID string) string {
	sum := sha256.Sum256([]byte("ma-kiem|" + caID))
	return hex.EncodeToString(sum[:6])
}
