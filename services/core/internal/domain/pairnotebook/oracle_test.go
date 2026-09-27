package pairnotebook

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/domain/pairpaper"
)

// testdata/python_pair_notebook*.json is rendered by
// scripts/render_pair_notebook_goldens.py from the real
// app.domain.pair_notebook (and pair_paper.hieu_luc) in the pinned API image.

type goldenConstants struct {
	CycleStates       []string `json:"cycle_states"`
	ConsentPurposes   []string `json:"consent_purposes"`
	ConstraintKinds   []string `json:"constraint_kinds"`
	HanDeNghiSeconds  int64    `json:"han_de_nghi_seconds"`
	NotebookErrorCode string   `json:"notebook_error_code"`
	All               []string `json:"all"`
}

type goldenFile struct {
	Mode      string           `json:"mode"`
	Constants *goldenConstants `json:"constants"`
	Fuzz      *struct {
		Shard  int `json:"shard"`
		Shards int `json:"shards"`
		Total  int `json:"total"`
	} `json:"fuzz"`
	Cases []map[string]any `json:"cases"`
}

// goNames maps every name Python's __all__ exports to its Go spelling. A new
// Python export fails the test until it is ported or listed.
var goNames = map[string]string{
	"CONSENT_PURPOSES":    "ConsentPurposes",
	"CONSTRAINT_KINDS":    "ConstraintKinds",
	"CYCLE_STATES":        "CycleStates",
	"NotebookError":       "NotebookError",
	"PER_PERSON_PURPOSES": "PerPersonPurposes",
	"gu_hai_nguoi":        "GuHaiNguoi",
	"nguoi_lo_suy":        "NguoiLoSuy",
	"vai_tuan":            "VaiTuan",
	"nguoi_mo_loi":        "NguoiMoLoi",
	"can_bat_doi":         "CanBatDoi",
	"chat_consent_active": "ChatConsentActive",
	"dang_cho":            "DangCho",
	"granted_by":          "GrantedBy",
	"granted_purposes":    "GrantedPurposes",
	"han_de_nghi":         "HanDeNghi",
	"xem_truoc_dong_so":   "XemTruocDongSo",
}

func loadGoldens(t *testing.T) []goldenFile {
	t.Helper()
	paths, err := filepath.Glob("testdata/python_pair_notebook*.json")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no golden files: %v", err)
	}
	sort.Strings(paths)
	var files []goldenFile
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var file goldenFile
		if err := json.Unmarshal(raw, &file); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		files = append(files, file)
	}
	return files
}

func instant(value any) *time.Time {
	if value == nil {
		return nil
	}
	text := value.(string)
	if !strings.HasPrefix(text, "$dt:") {
		panic("not a datetime: " + text)
	}
	parsed, err := time.Parse(pythonLayout, strings.TrimPrefix(text, "$dt:"))
	if err != nil {
		panic(err)
	}
	return &parsed
}

// pythonLayout is isoformat(timespec="microseconds") with the renderer's `|`
// before the offset.
const pythonLayout = "2006-01-02T15:04:05.000000|-07:00"

func pythonISO(x time.Time) string { return "$dt:" + x.Format(pythonLayout) }

func stringsOf(value any) []string {
	out := []string{}
	for _, item := range value.([]any) {
		out = append(out, item.(string))
	}
	return out
}

func sortedCopy(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	return out
}

func hexOf(text string) string { return strings.ReplaceAll(strings.TrimPrefix(text, "$hex:"), "_", "") }

type tally struct {
	cases, checks, mismatches int
	kinds                     map[string]int
	chat                      map[bool]int
	gu                        map[string]int
}

func (tl *tally) check(t *testing.T, c map[string]any, what string, got, want any) {
	t.Helper()
	tl.checks++
	if !reflect.DeepEqual(got, want) {
		tl.mismatches++
		t.Errorf("%s: %s: go %#v, python %#v", c["name"], what, got, want)
	}
}

func replayConsents(t *testing.T, tl *tally, c map[string]any) {
	var consents []Consent
	for _, item := range c["consents"].([]any) {
		row := item.(map[string]any)
		// A case without `proposal_id` is Python's absent key: "" groups those
		// rows together, the per-purpose reading the older corpus was built on.
		proposalID, _ := row["proposal_id"].(string)
		consents = append(consents, Consent{
			PersonID:            row["person_id"].(string),
			Purpose:             row["purpose"].(string),
			GrantedAt:           instant(row["granted_at"]),
			RevokedAt:           instant(row["revoked_at"]),
			ProposalExpiresAt:   instant(row["proposal_expires_at"]),
			ProposalID:          proposalID,
			ProposalCompletedAt: instant(row["proposal_completed_at"]),
		})
	}
	participants := stringsOf(c["participants"])
	now := instant(c["now"])
	result := c["result"].(map[string]any)
	for _, item := range result["granted_by"].([]any) {
		pair := item.([]any)
		person := pair[0].(string)
		got := GrantedBy(consents, person, now)
		tl.check(t, c, "granted_by "+person+" (ladder order)", got, ladderOrdered(got))
		tl.check(t, c, "granted_by "+person, sortedCopy(got), stringsOf(pair[1]))
	}
	purposes := GrantedPurposes(consents, participants, now)
	tl.check(t, c, "granted_purposes", sortedCopy(purposes), stringsOf(result["granted_purposes"]))
	tl.check(t, c, "can_bat_doi", CanBatDoi(consents, participants, now), result["can_bat_doi"])
	want, present := result["chat_consent_active"]
	if (now != nil) != present {
		t.Fatalf("%s: chat_consent_active present=%v with now=%v", c["name"], present, now)
	}
	if present {
		got := ChatConsentActive(consents, participants, *now)
		tl.chat[got]++
		tl.check(t, c, "chat_consent_active", got, want)
		wantGu, ok := result["gu"].([]any)
		if !ok {
			t.Fatalf("%s: no gu with now set", c["name"])
		}
		for _, item := range wantGu {
			pair := item.([]any)
			person := pair[0].(string)
			var got any
			if taste := GuHaiNguoi(consents, participants, person, oracleGu, *now); taste != nil {
				got = map[string]any{"mine_shared": taste.MineShared, "theirs_shared": taste.TheirsShared,
					"theirs": anyList(taste.Theirs), "common": anyList(taste.Common)}
				if len(taste.Theirs) > 0 {
					tl.gu["theirs"]++
				}
				if len(taste.Common) > 0 {
					tl.gu["common"]++
				}
			}
			tl.check(t, c, "gu_hai_nguoi "+person, got, pair[1])
		}
	}
}

// oracleGu is the script's GU.
var oracleGu = map[string][]string{
	"a1a1a1a1-b1b1-4c1c-8d1d-e1e1e1e1e1e1": {"cafe", "an-uong", "game"},
	"a2a2a2a2-b2b2-4c2c-8d2d-e2e2e2e2e2e2": {"game", "outdoor", "cafe", "tag-da-bo"},
	"a3a3a3a3-b3b3-4c3c-8d3d-e3e3e3e3e3e3": {"karaoke"},
}

func anyList(values []string) []any {
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}

func ladderOrdered(values []string) []string {
	out := []string{}
	for _, rung := range ConsentPurposes() {
		for _, value := range values {
			if value == rung {
				out = append(out, rung)
				break
			}
		}
	}
	return out
}

func replayPreview(t *testing.T, tl *tally, c map[string]any) {
	var papers []Paper
	for _, item := range c["papers"].([]any) {
		row := item.(map[string]any)
		papers = append(papers, Paper{ID: row["id"].(string), State: row["state"].(string), ExpiresAt: instant(row["expires_at"])})
	}
	var proposals []Proposal
	for _, item := range c["proposals"].([]any) {
		row := item.(map[string]any)
		proposals = append(proposals, Proposal{ID: row["id"].(string), CompletedAt: instant(row["completed_at"]), ExpiresAt: instant(row["expires_at"])})
	}
	now := *instant(c["now"])
	result := c["result"].(map[string]any)
	preview := result["preview"].(map[string]any)
	got := XemTruocDongSo(papers, proposals, now, sha256.Sum256)
	want := ClosePreview{
		Revision:    hexOf(preview["revision"].(string)),
		SoNhapBo:    int(preview["so_nhap_bo"].(float64)),
		SoToHuy:     int(preview["so_to_huy"].(float64)),
		SoToKhoa:    int(preview["so_to_khoa"].(float64)),
		SoDeNghiHuy: int(preview["so_de_nghi_huy"].(float64)),
	}
	if len(preview) != 5 {
		t.Fatalf("%s: python preview has keys %v", c["name"], preview)
	}
	tl.check(t, c, "xem_truoc_dong_so", got, want)
	states := []string{}
	for _, paper := range papers {
		states = append(states, pairpaper.HieuLuc(paper, now))
	}
	tl.check(t, c, "hieu_luc", states, stringsOf(result["hieu_luc"]))
	waiting := []any{}
	for _, proposal := range proposals {
		waiting = append(waiting, DangCho(proposal, now))
	}
	tl.check(t, c, "dang_cho", waiting, result["dang_cho"])
}

func replayHan(t *testing.T, tl *tally, c map[string]any) {
	now := *instant(c["now"])
	tl.check(t, c, "han_de_nghi", pythonISO(HanDeNghi(now)), c["result"])
}

func TestPairNotebookMatchesPython(t *testing.T) {
	tl := &tally{kinds: map[string]int{}, chat: map[bool]int{}, gu: map[string]int{}}
	fuzzShards, fuzzCases, fuzzTotal := 0, 0, 0
	sawConstants := false
	for _, file := range loadGoldens(t) {
		if file.Constants != nil {
			sawConstants = true
			k := file.Constants
			tl.check(t, map[string]any{"name": "constants"}, "CYCLE_STATES", CycleStates(), k.CycleStates)
			tl.check(t, map[string]any{"name": "constants"}, "CONSENT_PURPOSES", ConsentPurposes(), k.ConsentPurposes)
			tl.check(t, map[string]any{"name": "constants"}, "CONSTRAINT_KINDS", ConstraintKinds(), k.ConstraintKinds)
			tl.check(t, map[string]any{"name": "constants"}, "HAN_DE_NGHI", int64(OfferWindow/time.Second), k.HanDeNghiSeconds)
			tl.check(t, map[string]any{"name": "constants"}, "NotebookError.code",
				(&NotebookError{Code: "notebook_revision_stale"}).Error(), k.NotebookErrorCode)
			for _, name := range k.All {
				if goNames[name] == "" {
					t.Errorf("python exports %s and the port has no counterpart listed", name)
				}
			}
			if len(k.All) != len(goNames) {
				t.Errorf("python exports %d names, goNames lists %d", len(k.All), len(goNames))
			}
		}
		if file.Fuzz != nil {
			fuzzShards++
			fuzzCases += len(file.Cases)
			fuzzTotal = file.Fuzz.Total
		}
		for _, c := range file.Cases {
			tl.cases++
			fn := c["fn"].(string)
			tl.kinds[fmt.Sprintf("%s/%v", fn, file.Fuzz != nil)]++
			switch fn {
			case "consents":
				replayConsents(t, tl, c)
			case "preview":
				replayPreview(t, tl, c)
			case "han_de_nghi":
				replayHan(t, tl, c)
			case "nguoi_lo":
				replayNguoiLo(t, tl, c)
			default:
				t.Fatalf("unknown case kind %q", fn)
			}
		}
	}
	if !sawConstants {
		t.Fatal("no constants block: the edge file is missing")
	}
	if fuzzTotal < 2000 || fuzzCases != fuzzTotal {
		t.Fatalf("fuzz: %d shards carry %d of %d cases", fuzzShards, fuzzCases, fuzzTotal)
	}
	for _, fn := range []string{"consents", "preview", "han_de_nghi", "nguoi_lo"} {
		for _, fuzz := range []bool{false, true} {
			if tl.kinds[fmt.Sprintf("%s/%v", fn, fuzz)] == 0 {
				t.Fatalf("no %s case with fuzz=%v", fn, fuzz)
			}
		}
	}
	if tl.gu["theirs"] < 10 || tl.gu["common"] < 5 {
		t.Fatalf("gu_hai_nguoi outcomes too few to tell apart: %v", tl.gu)
	}
	if tl.chat[true] < 20 || tl.chat[false] < 20 {
		t.Fatalf("chat_consent_active outcomes too lopsided to tell apart: %v", tl.chat)
	}
	t.Logf("pair_notebook oracle: %d cases (%v), %d checks, %d mismatches", tl.cases, tl.kinds, tl.checks, tl.mismatches)
}

func optText(value any) *string {
	if value == nil {
		return nil
	}
	text := value.(string)
	return &text
}

func nguoiLoView(v NguoiLo, withCach bool) map[string]any {
	diem := []any{}
	for _, d := range v.Diem {
		diem = append(diem, []any{d.PersonID, float64(d.Score)})
	}
	out := map[string]any{"nguoi_lo": anyList(v.NguoiLo), "diem": diem}
	if withCach {
		out["cach"] = v.Cach
	}
	return out
}

func replayNguoiLo(t *testing.T, tl *tally, c map[string]any) {
	var toGiay []ToTinHieu
	for _, item := range c["to_giay"].([]any) {
		row := item.(map[string]any)
		to := ToTinHieu{CycleID: optText(row["cycle_id"]), Tuan: row["tuan"].(string)}
		for _, v := range row["versions"].([]any) {
			vm := v.(map[string]any)
			to.Versions = append(to.Versions, PhienBanTinHieu{Version: int(vm["version"].(float64)), AuthorType: vm["author_type"].(string),
				SentBy: optText(vm["sent_by"]), SentAt: instant(vm["sent_at"])})
		}
		for _, r := range row["responses"].([]any) {
			rm := r.(map[string]any)
			to.Responses = append(to.Responses, TraLoiTinHieu{PersonID: rm["person_id"].(string), Kind: rm["kind"].(string)})
		}
		toGiay = append(toGiay, to)
	}
	participants := stringsOf(c["participants"])
	suy := NguoiLoSuy(participants, toGiay, c["cycle"].(string), optText(c["lap_so"]))
	result := c["result"].(map[string]any)
	tl.check(t, c, "nguoi_lo_suy", nguoiLoView(suy, false), result["suy"])
	for i, tuan := range []string{"2026-08-31", "2026-09-07", "2026-09-14"} {
		var got any
		if who := NguoiMoLoi(toGiay, c["cycle"].(string), tuan); who != nil {
			got = *who
		}
		tl.check(t, c, "nguoi_mo_loi "+tuan, got, result["mo_loi"].([]any)[i])
	}
	var moLoi []*string
	for _, raw := range c["mo_loi"].([]any) {
		moLoi = append(moLoi, optText(raw))
	}
	for i, raw := range c["chon"].([]any) {
		var chon **string
		if raw != nil {
			id := optText(raw.(map[string]any)["nguoi_lo_id"])
			chon = &id
		}
		tl.check(t, c, fmt.Sprintf("vai_tuan %d", i), nguoiLoView(VaiTuan(suy, chon, participants, moLoi), true), result["vai"].([]any)[i])
	}
}
