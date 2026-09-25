// Package obs is what the AI engine records about a turn: ids, closed enums,
// counts and durations -- never a word the person or the model wrote, never a
// tool argument, never a sensitive label tied to a person (ADR-0037 §2.8).
//
// The shape enforces it. Every string-kinded field of TurnRecord is a named
// type with a Valid method over a closed set, and obs_test.go fails on a field
// that is not; so a record cannot carry free text without that test going red
// first. The same record is the one `slog` line per turn and the one
// ai_turn_metrics row (aiharness/metrics).
package obs

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"regexp"

	"mobile/services/core/internal/aiharness/cau"
)

// ID is an invocation id: a UUID, nothing else.
type ID string

var uuidShape = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func (v ID) Valid() bool { return uuidShape.MatchString(string(v)) }

// Bot is which assistant ran the turn.
type Bot string

const (
	BotNep  Bot = "nep"
	BotNhom Bot = "nhom"
)

func (v Bot) Valid() bool { return v == BotNep || v == BotNhom }

// Lenh is the command the job carried.
type Lenh string

const (
	LenhHoi      Lenh = "hoi"
	LenhPlan     Lenh = "plan"
	LenhChiaBill Lenh = "chia_bill"
)

func (v Lenh) Valid() bool { return v == LenhHoi || v == LenhPlan || v == LenhChiaBill }

// Guard is the input policy's outcome, and nothing finer: which kind of
// attack, or which kind of harm, is never stored against a person.
type Guard string

const (
	GuardProceed    Guard = "proceed"
	GuardRestricted Guard = "restricted"
	GuardRefused    Guard = "refused"
)

func (v Guard) Valid() bool { return v == GuardProceed || v == GuardRestricted || v == GuardRefused }

// OutGuard is whether the output guard stopped the answer; the kind of
// violation lives only in an eval run.
type OutGuard string

const (
	OutNone OutGuard = "none"
	OutChan OutGuard = "chan"
)

func (v OutGuard) Valid() bool { return v == OutNone || v == OutChan }

// KetThuc is how the turn ended.
type KetThuc string

const (
	KetThucXong    KetThuc = "xong"
	KetThucThatBai KetThuc = "that_bai"
)

func (v KetThuc) Valid() bool { return v == KetThucXong || v == KetThucThatBai }

// Code is the refusal or failure code, "" when the turn answered.
type Code string

func (v Code) Valid() bool { return v == "" || cau.Ma(v).Valid() }

// LoiMoHinh is the class of a provider failure, never its message: a
// provider's error text can echo the key or the question.
type LoiMoHinh string

const (
	LoiKhong   LoiMoHinh = "none"
	LoiTimeout LoiMoHinh = "timeout"
	Loi429     LoiMoHinh = "429"
	Loi5xx     LoiMoHinh = "5xx"
	LoiSafety  LoiMoHinh = "safety"
	LoiBadResp LoiMoHinh = "bad_response"
	LoiKhac    LoiMoHinh = "khac"
)

func (v LoiMoHinh) Valid() bool {
	switch v {
	case LoiKhong, LoiTimeout, Loi429, Loi5xx, LoiSafety, LoiBadResp, LoiKhac:
		return true
	}
	return false
}

// PromptVersion is the first twelve hex digits of the prompt's sha256.
type PromptVersion string

var hex12 = regexp.MustCompile(`^[0-9a-f]{12}$`)

func (v PromptVersion) Valid() bool { return hex12.MatchString(string(v)) }

// TurnRecord is one turn. The `col` tag is the column in ai_turn_metrics and
// the key in the log line; both are derived from it.
type TurnRecord struct {
	InvocationID  ID            `col:"invocation_id"`
	LanThu        int           `col:"lan_thu"`
	Bot           Bot           `col:"bot"`
	Lenh          Lenh          `col:"lenh"`
	Guard         Guard         `col:"guard"`
	OutGuard      OutGuard      `col:"out_guard"`
	KetThuc       KetThuc       `col:"ket_thuc"`
	Code          Code          `col:"code"`
	LoiMoHinh     LoiMoHinh     `col:"loi_mo_hinh"`
	PromptVersion PromptVersion `col:"prompt_version"`
	// Buoc is how many agent steps ran; SoGoiMoHinh counts every model call,
	// retries included (llm.MaxModelCallsPerTurn).
	Buoc        int `col:"buoc"`
	SoGoiMoHinh int `col:"so_goi_model"`
	SoCongCu    int `col:"so_cong_cu"`
	TokensIn    int `col:"tokens_in"`
	TokensOut   int `col:"tokens_out"`
	TokensCache int `col:"tokens_cached"`
	TokensNghi  int `col:"tokens_thoughts"`
	// What the deterministic stages dropped or flagged, as counts.
	LuotBo   int  `col:"luot_bo"`
	PhieuBo  int  `col:"phieu_bo"`
	NgayMoHo int  `col:"ngay_mo_ho"`
	KyTuAn   int  `col:"ky_tu_an"`
	KhongDau bool `col:"khong_dau"`
	// Durations in milliseconds.
	MsTrangThaiDau int `col:"ms_trang_thai_dau"`
	MsTienXuLy     int `col:"ms_tien_xu_ly"`
	MsMoHinh       int `col:"ms_mo_hinh"`
	MsTong         int `col:"ms_tong"`
}

type validator interface{ Valid() bool }

// Valid reports the first field outside its closed set.
func (r TurnRecord) Valid() error {
	v := reflect.ValueOf(r)
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() != reflect.String {
			if f.Kind() == reflect.Int && f.Int() < 0 {
				return fmt.Errorf("obs: %s is negative", v.Type().Field(i).Tag.Get("col"))
			}
			continue
		}
		val, ok := f.Interface().(validator)
		if !ok || !val.Valid() {
			return fmt.Errorf("obs: %s is outside its closed set", v.Type().Field(i).Tag.Get("col"))
		}
	}
	return nil
}

// Columns lists the `col` tags in field order.
func Columns() []string {
	t := reflect.TypeOf(TurnRecord{})
	out := make([]string, t.NumField())
	for i := range out {
		out[i] = t.Field(i).Tag.Get("col")
	}
	return out
}

// Values lists the field values in Columns order, strings as plain strings.
func (r TurnRecord) Values() []any {
	v := reflect.ValueOf(r)
	out := make([]any, v.NumField())
	for i := range out {
		f := v.Field(i)
		switch f.Kind() {
		case reflect.String:
			out[i] = f.String()
		case reflect.Int:
			out[i] = int(f.Int())
		case reflect.Bool:
			out[i] = f.Bool()
		}
	}
	return out
}

// Log writes the record as one `ai_turn` line. A record that fails Valid is
// logged as invalid with no fields at all: a bug must not become a channel.
func Log(ctx context.Context, logger *slog.Logger, r TurnRecord) {
	if logger == nil {
		return
	}
	if err := r.Valid(); err != nil {
		logger.LogAttrs(ctx, slog.LevelError, "ai_turn_invalid")
		return
	}
	cols, vals := Columns(), r.Values()
	attrs := make([]slog.Attr, len(cols))
	for i, c := range cols {
		attrs[i] = slog.Any(c, vals[i])
	}
	logger.LogAttrs(ctx, slog.LevelInfo, "ai_turn", attrs...)
}
