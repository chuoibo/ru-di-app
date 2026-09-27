package nepnho

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"mobile/services/core/internal/aiharness/prompts"
	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// Bot names who a profile is built for. Only Nếp has one.
type Bot string

const (
	BotNep  Bot = "nep"
	BotNhom Bot = "nhom"
)

const (
	// MaxSuThatHoSo is the most facts a profile lays into a prompt
	// (research stm-personalization §3.8: over-personalization).
	MaxSuThatHoSo = 5
	// UngVienHoSo is how many candidates the recall hands the reranker:
	// the sidecar's search answers at most MaxNho.
	UngVienHoSo = MaxNho
	// MaxRuneSuThatHoSo cuts one fact's words in the prompt.
	MaxRuneSuThatHoSo = 200
)

// HoSo is what personalization lays into one Nếp turn: at most five facts,
// reranked against the person's words, in a data block. Empty when the
// toggle is off, when the person has no fact, and always for the group.
type HoSo struct {
	SuThat []trinho.SuThat
	// DuLieu is the <du_lieu nguon="tri_nho"> block, "" when SuThat is empty.
	DuLieu string
	// KhongRerank: the reranker failed and the recall order was kept.
	KhongRerank bool
}

// ErrBotNhom: a profile asked for the group assistant. The group reaches no
// memory (ADR-0039 §10, design 05 §6): a caller that asks is a bug, and the
// answer is an error, not an empty profile it could mistake for "nothing
// remembered".
var ErrBotNhom = errors.New("nepnho: the group assistant has no memory profile")

// DungHoSo builds the profile of one Nếp turn. tn is the person's memory
// (it returns nothing while the toggle is off, see Kho.Nho); xl reorders the
// candidates against cau and keeps the best five. The rerank score orders
// and never cuts (bring-up finding: relevant facts and lexical traps score
// alike); a failed rerank keeps the recall order and says so.
func DungHoSo(ctx context.Context, bot Bot, tn trinho.TriNho, xl truyhoi.Reranker, nguoi, cau string) (HoSo, error) {
	if bot != BotNep {
		return HoSo{}, ErrBotNhom
	}
	ds, err := tn.Nho(ctx, nguoi, cau, UngVienHoSo)
	if err != nil || len(ds) == 0 {
		return HoSo{}, err
	}
	byID := make(map[string]trinho.SuThat, len(ds))
	bc := make([]truyhoi.BangChung, 0, len(ds))
	for _, s := range ds {
		byID[s.ID] = s
		bc = append(bc, truyhoi.BangChung{ID: s.ID, Nguon: truyhoi.Memory, Truong: map[string]string{"cau": s.NoiDung}})
	}
	var out HoSo
	xep, err := xl.XepLai(ctx, cau, bc, MaxSuThatHoSo)
	if err != nil {
		out.KhongRerank = true
		xep, _ = truyhoi.Passthrough{}.XepLai(ctx, cau, bc, MaxSuThatHoSo)
	}
	seen := map[string]bool{}
	for _, b := range xep {
		s, ok := byID[b.ID]
		// A reranker may reorder and cut, never add: an id it invents, or
		// repeats, is dropped.
		if !ok || seen[b.ID] || len(out.SuThat) == MaxSuThatHoSo {
			continue
		}
		seen[b.ID] = true
		out.SuThat = append(out.SuThat, s)
	}
	out.DuLieu = khoiDuLieu(out.SuThat)
	return out, nil
}

// khoiDuLieu lays the facts into Nếp's prompt data block, one per line with
// an alias the answer can cite ([f1]..[f5]) and the fact's closed kind.
func khoiDuLieu(ds []trinho.SuThat) string {
	if len(ds) == 0 {
		return ""
	}
	var b strings.Builder
	for i, s := range ds {
		words := []rune(s.NoiDung)
		if len(words) > MaxRuneSuThatHoSo {
			words = words[:MaxRuneSuThatHoSo]
		}
		b.WriteString("[f" + strconv.Itoa(i+1) + "] (" + string(s.Loai) + ") " + string(words))
		if i < len(ds)-1 {
			b.WriteByte('\n')
		}
	}
	return prompts.BocDuLieu(prompts.TriNho, b.String())
}

// HoSoNep implements aiharness.HoSo: the data block of one Nếp turn, from
// this person's own facts (Kho.Nho: nothing while the toggle is off). The
// recall order is kept (no reranker on this path: a reranker model call
// would come out of the turn's budget of eight).
func (k *Kho) HoSoNep(ctx context.Context, nguoi, cau string) (string, error) {
	if k.kho == nil {
		return "", nil
	}
	h, err := DungHoSo(ctx, BotNep, k, truyhoi.Passthrough{}, nguoi, cau)
	return h.DuLieu, err
}
