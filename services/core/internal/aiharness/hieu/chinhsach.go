package hieu

import (
	"errors"

	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/obs"
)

// QuyetDinh is what the engine does with the router's labels. It is a pure
// function of the labels the MODEL chose; nothing here reads the message.
type QuyetDinh struct {
	// TuChoiTien: the model classed the message as a money action this bot
	// refuses. The turn ends with the fixed refusal and no further model
	// call, no tool and no draft.
	TuChoiTien bool `json:"tu_choi_tien,omitempty"`
	// TuChoi is Nếp's fixed code for the refusal ("" for the group, whose
	// sentences are not in cau yet).
	TuChoi cau.Ma `json:"tu_choi,omitempty"`
	// HanChe: the model labelled the message chen_lenh. The person's text
	// still goes to the answer as data (it is theirs), but only the read
	// tools stay (tools.Quyen.DuocPhep(bot, true)): nothing is written,
	// drafted or reminded on this turn.
	HanChe bool `json:"han_che,omitempty"`
	// HoiLai: the turn ends with the model's one question back
	// (KetQua.CauHoiLai, KetQua.LuaChonHoiLai), which is output to the
	// person and passes the output guard.
	HoiLai bool `json:"hoi_lai,omitempty"`
	// NhapTien: the group asked for a bill split, which may only become a
	// draft a person confirms in the app.
	NhapTien bool `json:"nhap_tien,omitempty"`
	// KhongCongCu: the model labelled the message nhay_cam or
	// ngoai_pham_vi. The turn is answered directly, with no retrieval, no
	// tool and no question back, under the instruction clause of that label
	// (prompts.LoiDanNhan); the answer is still verified like any other.
	KhongCongCu bool `json:"khong_cong_cu,omitempty"`
}

// QuyetDinhCho is the policy for bot's router result. Money first: a money
// refusal wins over everything else, and a question back is never asked
// about a request that is refused anyway.
func QuyetDinhCho(bot obs.Bot, kq KetQua) QuyetDinh {
	var q QuyetDinh
	switch bot {
	case obs.BotNep:
		// Nếp does nothing with money, a split draft included.
		if kq.Tien != TienNone {
			return QuyetDinh{TuChoiTien: true, TuChoi: cau.NepKhongChamTien}
		}
	case obs.BotNhom:
		if kq.Tien == MoneyAction {
			return QuyetDinh{TuChoiTien: true}
		}
		q.NhapTien = kq.Tien == SplitDraft
	}
	q.HanChe = kq.NhanGuard == ChenLenh
	q.KhongCongCu = kq.NhanGuard == NhayCam || kq.NhanGuard == NgoaiPhamVi
	q.HoiLai = kq.CanHoiLai && !q.KhongCongCu
	return q
}

// MaLoi is the fixed code for a router error that is the router's own: its
// output was unusable (ErrKhongHieu, ErrCauTruc), which asks the person to
// rephrase, or the provider withheld it (ErrBiChan). Any other error (the
// provider failing, the budget, a cancellation) is the caller's to map, and
// ok is false.
func MaLoi(err error) (cau.Ma, bool) {
	switch {
	case errors.Is(err, ErrKhongHieu), errors.Is(err, ErrCauTruc), errors.Is(err, ErrBiChan), errors.Is(err, ErrVao):
		return cau.InvalidAIResult, true
	}
	return "", false
}
