package pairpaper

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"mobile/services/core/internal/oracletest"
)

// testdata/python_pair_paper*.json is rendered by
// scripts/render_domain_w8_goldens.py from the real app.domain.pair_paper in
// the parity API image; every case is replayed here. `astimezone` is not a
// function of the module but MUI_GIO applied to an instant, the one thing the
// zone table has to reproduce.

// goNames maps every name Python's __all__ exports to its Go spelling. A new
// export fails the test until it is ported or listed.
var goNames = map[string]string{
	"AUTHOR_TYPES":   "AuthorTypes",
	"MUI_GIO":        "MuiGio",
	"OPEN_STATES":    "OpenStates",
	"PAPER_STATES":   "PaperStates",
	"PLAN_STATES":    "PlanStates",
	"RESPONSE_KINDS": "ResponseKinds",
	"TERMINAL":       "Terminal",
	"PaperError":     "PaperError",
	"chuyen":         "Chuyen",
	"co_the_rut":     "CoTheRut",
	"da_du_dong_y":   "DaDuDongY",
	"han_tuan":       "HanTuan",
	"hieu_luc":       "HieuLuc",
	"lam_giau_phac":  "LamGiauPhac",
	"ngay_de_xuat":   "NgayDeXuat",
	"phac_to_giay":   "PhacToGiay",
	"tuan_cua":       "TuanCua",
}

// functions is every case kind; codes is every refusal the corpus must hold.
var (
	functions = []string{"tuan_cua", "han_tuan", "ngay_de_xuat", "astimezone", "hieu_luc", "da_du_dong_y", "co_the_rut", "chuyen", "phac_to_giay", "lam_giau_phac"}
	codes     = []string{"paper_event_unknown", "paper_expired", "paper_frozen", "paper_wrong_state", "paper_not_withdrawable", "paper_needs_recorder", "paper_draft_needs_date"}
)

func decodeFailed(err error) (any, error) { return nil, oracletest.Decode(err) }

func optionalISO(t *time.Time) any {
	if t == nil {
		return nil
	}
	return ISOFormat(*t)
}

func paperOf(value any) (Paper, error) {
	m, err := oracletest.Row(value, "id", "state", "current_version", "expires_at")
	if err != nil {
		return Paper{}, err
	}
	var p Paper
	if p.ID, err = oracletest.Str(m["id"]); err != nil {
		return p, err
	}
	if p.State, err = oracletest.Str(m["state"]); err != nil {
		return p, err
	}
	version, err := oracletest.Int64(m["current_version"])
	if err != nil {
		return p, err
	}
	p.CurrentVersion = int(version)
	if m["expires_at"] != nil {
		expires, err := oracletest.Instant(m["expires_at"])
		if err != nil {
			return p, err
		}
		p.ExpiresAt = &expires
	}
	return p, nil
}

// rowsOf reads [version, person] or [version, person, kind] lists.
func rowsOf(value any) ([]Row, error) {
	items, err := oracletest.List(value)
	if err != nil {
		return nil, err
	}
	out := make([]Row, len(items))
	for i, raw := range items {
		fields, err := oracletest.List(raw)
		if err != nil || len(fields) < 2 {
			return nil, fmt.Errorf("row %v", raw)
		}
		version, err := oracletest.Int64(fields[0])
		if err != nil {
			return nil, err
		}
		out[i].Version = int(version)
		if out[i].PersonID, err = oracletest.Str(fields[1]); err != nil {
			return nil, err
		}
		if len(fields) == 3 {
			if out[i].Kind, err = oracletest.Str(fields[2]); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// truthy is bool() of the fact values the corpus passes.
func truthy(value any) (bool, error) {
	switch v := value.(type) {
	case nil:
		return false, nil
	case bool:
		return v, nil
	case string:
		return v != "", nil
	case int64:
		return v != 0, nil
	}
	return false, fmt.Errorf("fact %v (%T)", value, value)
}

func dateOf(value any) (Date, error) {
	year, month, day, err := oracletest.CivilDate(value)
	return Date{Year: year, Month: month, Day: day}, err
}

func routineOf(value any) (Routine, error) {
	m, err := oracletest.Row(value, "ngay", "gio", "viec", "di_tiep")
	if err != nil {
		return Routine{}, err
	}
	var r Routine
	ngay, err := oracletest.List(m["ngay"])
	if err != nil || len(ngay) == 0 {
		return r, fmt.Errorf("ngay %v", m["ngay"])
	}
	if ngay[0] == "date" {
		d, err := dateOf(ngay[1])
		if err != nil {
			return r, err
		}
		r.Ngay = &d
	}
	if r.Gio, err = oracletest.Str(m["gio"]); err != nil {
		return r, err
	}
	if r.Viec, err = oracletest.Str(m["viec"]); err != nil {
		return r, err
	}
	if next, ok := m["di_tiep"].(map[string]any); ok && len(next) > 0 {
		stop, err := oracletest.Row(next, "gio", "viec")
		if err != nil {
			return r, err
		}
		gio, err := oracletest.Str(stop["gio"])
		if err != nil {
			return r, err
		}
		viec, err := oracletest.Str(stop["viec"])
		if err != nil {
			return r, err
		}
		r.DiTiep = &RoutineStop{Gio: gio, Viec: viec}
	}
	if reason, ok := m["ly_do"].(string); ok {
		r.LyDo = reason
	}
	return r, nil
}

func renderDraft(d Draft) any {
	chang := make([]any, len(d.Content.Chang))
	for i, stop := range d.Content.Chang {
		var place any
		if stop.PlaceID != nil {
			place = *stop.PlaceID
		}
		chang[i] = map[string]any{"gio": stop.Gio, "viec": stop.Viec, "place_id": place, "can_kiem": stop.CanKiem}
	}
	return map[string]any{
		"content": map[string]any{"ngay": d.Content.Ngay, "chang": chang},
		"ly_do":   d.LyDo,
		"nguon":   map[string]any{"scope": d.Nguon.Scope, "dung": oracletest.AnyStrings(d.Nguon.Dung), "luc": d.Nguon.Luc},
	}
}

func replay(c oracletest.Case, args map[string]any) (any, error) {
	switch c.Fn {
	case "tuan_cua", "han_tuan", "ngay_de_xuat", "astimezone":
		now, err := oracletest.Instant(args["now"])
		if err != nil {
			return decodeFailed(err)
		}
		switch c.Fn {
		case "tuan_cua":
			return TuanCua(now).ISOFormat(), nil
		case "han_tuan":
			return ISOFormat(HanTuan(now)), nil
		case "ngay_de_xuat":
			return NgayDeXuat(now).ISOFormat(), nil
		}
		return ISOFormat(Local(now)), nil
	case "hieu_luc":
		paper, err := paperOf(args["paper"])
		if err != nil {
			return decodeFailed(err)
		}
		now, err := oracletest.Instant(args["now"])
		if err != nil {
			return decodeFailed(err)
		}
		return HieuLuc(paper, now), nil
	case "da_du_dong_y":
		responses, err := rowsOf(args["responses"])
		if err != nil {
			return decodeFailed(err)
		}
		version, err := oracletest.Int64(args["version"])
		if err != nil {
			return decodeFailed(err)
		}
		return DaDuDongY(responses, int(version)), nil
	case "co_the_rut":
		paper, err := paperOf(args["paper"])
		if err != nil {
			return decodeFailed(err)
		}
		rows, err := oracletest.List(args["versions"])
		if err != nil {
			return decodeFailed(err)
		}
		versions := make([]Version, len(rows))
		for i, raw := range rows {
			fields, err := oracletest.List(raw)
			if err != nil || len(fields) != 3 {
				return decodeFailed(fmt.Errorf("version %v", raw))
			}
			number, err := oracletest.Int64(fields[0])
			if err != nil {
				return decodeFailed(err)
			}
			versions[i] = Version{Version: int(number)}
			if versions[i].AuthorType, err = oracletest.Str(fields[1]); err != nil {
				return decodeFailed(err)
			}
			if versions[i].SentBy, err = oracletest.OptionalString(fields[2]); err != nil {
				return decodeFailed(err)
			}
		}
		views, err := rowsOf(args["views"])
		if err != nil {
			return decodeFailed(err)
		}
		responses, err := rowsOf(args["responses"])
		if err != nil {
			return decodeFailed(err)
		}
		actor, err := oracletest.Str(args["actor_id"])
		if err != nil {
			return decodeFailed(err)
		}
		return CoTheRut(paper, versions, views, responses, actor), nil
	case "chuyen":
		paper, err := paperOf(args["paper"])
		if err != nil {
			return decodeFailed(err)
		}
		event, err := oracletest.Str(args["su_kien"])
		if err != nil {
			return decodeFailed(err)
		}
		now, err := oracletest.Instant(args["now"])
		if err != nil {
			return decodeFailed(err)
		}
		given, ok := args["facts"].(map[string]any)
		if !ok {
			return decodeFailed(fmt.Errorf("facts %v", args["facts"]))
		}
		var facts Facts
		for name, into := range map[string]*bool{"du_dong_y": &facts.DuDongY, "co_the_rut": &facts.CoTheRut, "nguoi_ghi": &facts.NguoiGhi} {
			if *into, err = truthy(given[name]); err != nil {
				return decodeFailed(err)
			}
		}
		after, err := Chuyen(paper, event, now, facts)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"id":              after.ID,
			"state":           after.State,
			"current_version": int64(after.CurrentVersion),
			"expires_at":      optionalISO(after.ExpiresAt),
		}, nil
	case "phac_to_giay":
		routine, err := routineOf(args["routine"])
		if err != nil {
			return decodeFailed(err)
		}
		constraints, err := oracletest.Int64(args["rang_buoc"])
		if err != nil {
			return decodeFailed(err)
		}
		now, err := oracletest.Instant(args["now"])
		if err != nil {
			return decodeFailed(err)
		}
		draft, err := PhacToGiay(routine, constraints > 0, now)
		if err != nil {
			return nil, err
		}
		return renderDraft(draft), nil
	case "lam_giau_phac":
		routine, err := routineOf(args["routine"])
		if err != nil {
			return decodeFailed(err)
		}
		boxes, err := oracletest.Strings(args["rang_buoc"])
		if err != nil {
			return decodeFailed(err)
		}
		now, err := oracletest.Instant(args["now"])
		if err != nil {
			return decodeFailed(err)
		}
		lichSu, err := lichSuOf(args["lich_su"])
		if err != nil {
			return decodeFailed(err)
		}
		choCu, err := placeRowOf(args["cho_cu"])
		if err != nil {
			return decodeFailed(err)
		}
		rawRows, err := oracletest.List(args["ung_vien"])
		if err != nil {
			return decodeFailed(err)
		}
		var ungVien []PlaceRow
		for _, raw := range rawRows {
			row, err := placeRowOf(raw)
			if err != nil || row == nil {
				return decodeFailed(fmt.Errorf("ung_vien %v: %v", raw, err))
			}
			ungVien = append(ungVien, *row)
		}
		draft, err := PhacToGiay(routine, len(boxes) > 0, now)
		if err != nil {
			return nil, err
		}
		return renderDraft(LamGiauPhac(draft, lichSu, choCu, ungVien, boxes)), nil
	}
	return decodeFailed(fmt.Errorf("unknown function %q", c.Fn))
}

// placeRowOf reads row_of's spec: [id, name, category, kinds, traits,
// rating*10, count]; nil for None. Only the str items of kinds and traits
// reach the function, as `isinstance(k, str)` keeps them.
func placeRowOf(value any) (*PlaceRow, error) {
	if value == nil {
		return nil, nil
	}
	spec, err := oracletest.List(value)
	if err != nil || len(spec) != 7 {
		return nil, fmt.Errorf("row %v", value)
	}
	var row PlaceRow
	if row.ID, err = oracletest.Str(spec[0]); err != nil {
		return nil, err
	}
	if row.Name, err = oracletest.Str(spec[1]); err != nil {
		return nil, err
	}
	if row.Category, err = oracletest.Str(spec[2]); err != nil {
		return nil, err
	}
	for _, into := range []struct {
		raw any
		out *[]string
	}{{spec[3], &row.Kinds}, {spec[4], &row.Traits}} {
		items, err := oracletest.List(into.raw)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if text, ok := item.(string); ok {
				*into.out = append(*into.out, text)
			}
		}
	}
	if spec[5] != nil {
		tenths, err := oracletest.Int64(spec[5])
		if err != nil {
			return nil, err
		}
		rating := float64(tenths) / 10
		row.Rating = &rating
	}
	if spec[6] != nil {
		count, err := oracletest.Int64(spec[6])
		if err != nil {
			return nil, err
		}
		row.RatingCount = &count
	}
	return &row, nil
}

// lichSuOf reads [[ngay, [[gio, viec, place_id], ...]], ...].
func lichSuOf(value any) ([]Content, error) {
	entries, err := oracletest.List(value)
	if err != nil {
		return nil, err
	}
	var out []Content
	for _, raw := range entries {
		entry, err := oracletest.List(raw)
		if err != nil || len(entry) != 2 {
			return nil, fmt.Errorf("lich_su %v", raw)
		}
		ngay, err := oracletest.Str(entry[0])
		if err != nil {
			return nil, err
		}
		stops, err := oracletest.List(entry[1])
		if err != nil {
			return nil, err
		}
		content := Content{Ngay: ngay}
		for _, rawStop := range stops {
			stop, err := oracletest.List(rawStop)
			if err != nil || len(stop) != 3 {
				return nil, fmt.Errorf("stop %v", rawStop)
			}
			var s Stop
			if s.Gio, err = oracletest.Str(stop[0]); err != nil {
				return nil, err
			}
			if s.Viec, err = oracletest.Str(stop[1]); err != nil {
				return nil, err
			}
			if s.PlaceID, err = oracletest.OptionalString(stop[2]); err != nil {
				return nil, err
			}
			content.Chang = append(content.Chang, s)
		}
		out = append(out, content)
	}
	return out, nil
}

func refusal(err error) (string, string, bool) {
	var refused *PaperError
	if errors.As(err, &refused) {
		return "PaperError", refused.Code, true
	}
	return "", "", false
}

func checkConstants(t *testing.T, k map[string]any) {
	t.Helper()
	for key, got := range map[string][]string{
		"paper_states":   PaperStates(),
		"open_states":    OpenStates(),
		"plan_states":    PlanStates(),
		"terminal":       Terminal(),
		"author_types":   AuthorTypes(),
		"response_kinds": ResponseKinds(),
	} {
		want, err := oracletest.Strings(k[key])
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: Go %v, Python %v (%v)", key, got, want, err)
		}
	}
	if k["mui_gio"] != MuiGio || k["thu_bay"] != int64(thuBay) {
		t.Errorf("MUI_GIO %v and _THU_BAY %v, Go %s and %d", k["mui_gio"], k["thu_bay"], MuiGio, thuBay)
	}
	if code := (&PaperError{Code: "paper_frozen"}).Error(); k["paper_error_code"] != code {
		t.Errorf("PaperError code %v, Go %s", k["paper_error_code"], code)
	}
	events, err := oracletest.List(k["events"])
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, raw := range events {
		entry, err := oracletest.List(raw)
		if err != nil || len(entry) != 2 {
			t.Fatalf("event %v", raw)
		}
		name, _ := entry[0].(string)
		names = append(names, name)
		want, err := oracletest.Strings(entry[1])
		got, known := EventSources(name)
		if err != nil || !known || !reflect.DeepEqual(got, want) {
			t.Errorf("_TU[%s]: Go %v %v, Python %v", name, got, known, want)
		}
	}
	if !reflect.DeepEqual(names, Events()) {
		t.Errorf("_TU keys: Go %v, Python %v", Events(), names)
	}
	exported, err := oracletest.Strings(k["all"])
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range exported {
		if goNames[name] == "" {
			t.Errorf("python exports %s and the port has no counterpart listed", name)
		}
	}
	if len(exported) != len(goNames) {
		t.Errorf("python exports %d names, goNames lists %d", len(exported), len(goNames))
	}
}

// checkPaper replays the files and holds the corpus to its spread.
func checkPaper(t *testing.T, files []oracletest.File, least int) {
	t.Helper()
	report := oracletest.Agree(t, files, "pair_paper", replay, refusal)
	total := 0
	for _, tally := range report.ByFn {
		total += tally.Cases
	}
	if total < least {
		t.Errorf("%d cases, want at least %d", total, least)
	}
	for _, fn := range functions {
		if report.ByFn[fn] == nil {
			t.Errorf("no Python case of %s", fn)
		}
	}
	for _, code := range codes {
		if report.Codes[code] == 0 {
			t.Errorf("no Python case refused with %s", code)
		}
	}
	t.Logf("refusals %v", report.Codes)
}

func TestPairPaperMatchesPython(t *testing.T) {
	files := oracletest.Load(t, "testdata/python_pair_paper*.json")
	checkConstants(t, oracletest.Constants(t, files, "pair_paper"))
	checkPaper(t, files, 800)
}

// The zone table is exercised through the oracle; this pins the one reading
// every service call makes, so a broken table fails even without goldens.
func TestTodayInVietnam(t *testing.T) {
	now := time.Date(2030, 9, 22, 17, 0, 0, 0, time.UTC)
	if got := TuanCua(now); got != (Date{2030, 9, 23}) {
		t.Fatalf("tuan_cua(Monday 00:00 local) = %v", got)
	}
	if got := ISOFormat(HanTuan(now.Add(-time.Microsecond))); got != "2030-09-22T17:00:00+00:00" {
		t.Fatalf("han_tuan(Sunday 23:59:59.999999 local) = %s", got)
	}
}
