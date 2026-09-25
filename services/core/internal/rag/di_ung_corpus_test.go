package rag

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"mobile/services/core/internal/domain/tuvung"
	"mobile/services/core/internal/repo"
)

// An allergy corpus (di_ung_heldout_v2 schema): asker sentences and place
// texts, each labelled by someone who read neither tuvung nor rag. It is
// measured through the paths the product runs: an asker's words through
// DocCau (what the public search and the engine's deterministic half read),
// a place text as the one field it names through DungHoSo (SafeDeep, the
// allergen scan of every descriptive string, diets from the name, kinds and
// traits).
type corpusDiUng struct {
	QuyUoc struct {
		PhienBan string `json:"phien_ban"`
		Tap      string `json:"tap"`
	} `json:"quy_uoc"`
	NguoiHoi []struct {
		ID      string   `json:"id"`
		Cau     string   `json:"cau"`
		DiUng   []string `json:"di_ung"`
		AnKieng []string `json:"an_kieng"`
		Loai    string   `json:"loai"`
		// DocThua: reading more than the label from this row is an accepted
		// over-read (the corpus's an_toan_neu_doc_thua).
		DocThua bool `json:"an_toan_neu_doc_thua"`
	} `json:"nguoi_hoi"`
	Quan []struct {
		ID      string   `json:"id"`
		Chu     string   `json:"chu"`
		Truong  string   `json:"truong"`
		Chua    []string `json:"chua"`
		PhucVu  []string `json:"phuc_vu"`
		Loai    string   `json:"loai"`
		DocThua bool     `json:"an_toan_neu_doc_thua"`
	} `json:"quan"`
}

// nhanCorpus maps the corpus's canonical labels onto the closed ids
// («động vật có vỏ» is oc_so, «gluten» is lua_mi).
var nhanCorpus = map[string]string{
	"tôm": "tom", "cua": "cua", "cá": "ca", "mực": "muc", "động vật có vỏ": "oc_so", "hải sản": "hai_san",
	"đậu phộng": "dau_phong", "các loại hạt": "hat_cay", "sữa": "sua", "trứng": "trung", "mè": "me",
	"gluten": "lua_mi", "đậu nành": "dau_nanh",
	"chay": "chay", "thuần chay": "thuan_chay", "halal": "halal",
}

func maCorpus(t *testing.T, labels []string) []string {
	t.Helper()
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		id, ok := nhanCorpus[l]
		if !ok {
			t.Fatalf("label %q is not one of the corpus's canonical names", l)
		}
		out = append(out, id)
	}
	return out
}

func tap(ids []string) map[string]bool {
	out := map[string]bool{}
	for _, id := range ids {
		out[id] = true
	}
	return out
}

func ngoai(a, b map[string]bool) []string {
	var out []string
	for id := range a {
		if !b[id] {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// hangCorpus is a place row holding only the corpus text, in the field it
// names. Kinds are cut at «,» and «|», traits at «;», as a catalogue stores
// them as lists. The name of a row whose text is elsewhere is «Q»: no
// marks, no allergen, so every mark the row carries is the corpus text's own.
func hangCorpus(id, truong, chu string) repo.Place {
	p := repo.Place{ID: id, DestinationID: "d-x", Name: "Q", Category: "quan-an-local", Source: "seed", Kinds: []string{}, Traits: []string{}}
	cut := func(s, seps string) []string {
		var out []string
		for _, part := range strings.FieldsFunc(s, func(r rune) bool { return strings.ContainsRune(seps, r) }) {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
		return out
	}
	switch truong {
	case "ten":
		p.Name = chu
	case "kinds":
		p.Kinds = cut(chu, ",|")
	case "traits":
		p.Traits = cut(chu, ";")
	case "mo_ta":
		p.Description = &chu
	case "review":
		p.Reviews, _ = json.Marshal([]map[string]any{{"author": "Khách", "rating": 4, "body": chu}})
	}
	return p
}

// soCorpus is one group's counts.
type soCorpus struct {
	// Askers. coNhan rows name an allergen; du of them are read in full
	// (the label's family closure inside the reading's). thua rows read an
	// allergen beyond the label where the corpus does not accept it;
	// thuaChoPhep where it does. kiengSai: a labelled diet missed, or one
	// read beyond the label where the corpus does not accept it.
	n, coNhan, du, thua, thuaChoPhep, kiengSai int
	// Places. chua: labelled allergens; chuaLot of them not hidden (the
	// unsafe miss); chuaThua rows tagged beyond the label (safe, counted
	// where the corpus does not accept it). phucVuSai: a diet read that the
	// label does not give (unsafe); phucVuLot: a labelled diet missed.
	chua, chuaLot, chuaThua, phucVu, phucVuSai, phucVuLot, bo int
}

type doCorpus struct {
	nhom map[string]*soCorpus
	// lot are the unsafe misses by id: asker allergens, place allergens,
	// place diets read wrongly.
	lotHoi, lotQuan, kiengQuanSai []string
	// thua and kieng are the rest, by id, for the log.
	thuaHoi, kiengHoi, lotKiengQuan, thuaQuan []string
}

func (d *doCorpus) g(nhom string) *soCorpus {
	if d.nhom[nhom] == nil {
		d.nhom[nhom] = &soCorpus{}
	}
	return d.nhom[nhom]
}

func doDiUng(t *testing.T, c corpusDiUng) doCorpus {
	t.Helper()
	d := doCorpus{nhom: map[string]*soCorpus{}}
	for _, r := range c.NguoiHoi {
		s, all := d.g("hoi/"+r.Loai), d.g("hoi")
		label := maCorpus(t, r.DiUng)
		y, _ := DocCau(r.Cau, nil)
		got := tap(tuvung.MoRongDiUng(y.DiUng))
		want := tap(tuvung.MoRongDiUng(label))
		for _, x := range []*soCorpus{s, all} {
			x.n++
		}
		if len(label) > 0 {
			miss := ngoai(want, got)
			for _, x := range []*soCorpus{s, all} {
				x.coNhan++
				if len(miss) == 0 {
					x.du++
				}
			}
			if len(miss) > 0 {
				d.lotHoi = append(d.lotHoi, fmt.Sprintf("%s:%s", r.ID, strings.Join(miss, "+")))
			}
		}
		if extra := ngoai(got, want); len(extra) > 0 {
			for _, x := range []*soCorpus{s, all} {
				if r.DocThua {
					x.thuaChoPhep++
				} else {
					x.thua++
				}
			}
			mark := ""
			if r.DocThua {
				mark = "~"
			}
			d.thuaHoi = append(d.thuaHoi, fmt.Sprintf("%s%s:%s", mark, r.ID, strings.Join(extra, "+")))
		}
		diets := tap(tuvung.DoiKieng(maCorpus(t, r.AnKieng)))
		gotDiets := tap(tuvung.DoiKieng(y.AnKieng))
		missD, extraD := ngoai(diets, gotDiets), ngoai(gotDiets, diets)
		if len(missD) > 0 || (len(extraD) > 0 && !r.DocThua) {
			for _, x := range []*soCorpus{s, all} {
				x.kiengSai++
			}
			d.kiengHoi = append(d.kiengHoi, fmt.Sprintf("%s:-%v+%v", r.ID, missD, extraD))
		}
	}
	for _, q := range c.Quan {
		s, all := d.g("quan/"+q.Loai), d.g("quan")
		h, rep := DungHoSo(hangCorpus(q.ID, q.Truong, q.Chu))
		for _, x := range []*soCorpus{s, all} {
			x.n++
		}
		if rep.Bo {
			// Never indexed, never shown: nothing it says can reach anyone.
			for _, x := range []*soCorpus{s, all} {
				x.bo++
			}
			continue
		}
		tags := tap(h.DiUng)
		chua := maCorpus(t, q.Chua)
		var lot []string
		for _, a := range chua {
			hidden := false
			for _, c := range tuvung.MoRongDiUng([]string{a}) {
				hidden = hidden || tags[c]
			}
			if !hidden {
				lot = append(lot, a)
			}
		}
		for _, x := range []*soCorpus{s, all} {
			x.chua += len(chua)
			x.chuaLot += len(lot)
		}
		if len(lot) > 0 {
			d.lotQuan = append(d.lotQuan, fmt.Sprintf("%s:%s", q.ID, strings.Join(lot, "+")))
		}
		if extra := ngoai(tags, tap(tuvung.MoRongDiUng(chua))); len(extra) > 0 {
			if !q.DocThua {
				for _, x := range []*soCorpus{s, all} {
					x.chuaThua++
				}
			}
			d.thuaQuan = append(d.thuaQuan, fmt.Sprintf("%s:%s", q.ID, strings.Join(extra, "+")))
		}
		label := tap(tuvung.DoiKieng(maCorpus(t, q.PhucVu)))
		got := tap(h.AnKieng)
		wrong, miss := ngoai(got, label), ngoai(label, got)
		for _, x := range []*soCorpus{s, all} {
			x.phucVu += len(label)
			x.phucVuSai += len(wrong)
			x.phucVuLot += len(miss)
		}
		if len(wrong) > 0 {
			d.kiengQuanSai = append(d.kiengQuanSai, fmt.Sprintf("%s:%s", q.ID, strings.Join(wrong, "+")))
		}
		if len(miss) > 0 {
			d.lotKiengQuan = append(d.lotKiengQuan, fmt.Sprintf("%s:%s", q.ID, strings.Join(miss, "+")))
		}
	}
	return d
}

func (s soCorpus) hoi() string {
	return fmt.Sprintf("n=%d recall=%d/%d doc_thua=%d doc_thua_cho_phep=%d an_kieng_sai=%d", s.n, s.du, s.coNhan, s.thua, s.thuaChoPhep, s.kiengSai)
}

func (s soCorpus) quan() string {
	return fmt.Sprintf("n=%d bo=%d chua_lot=%d/%d chua_thua=%d phuc_vu_sai=%d phuc_vu_lot=%d/%d", s.n, s.bo, s.chuaLot, s.chua, s.chuaThua, s.phucVuSai, s.phucVuLot, s.phucVu)
}

func (d doCorpus) baoCao(t *testing.T) {
	t.Helper()
	var keys []string
	for k := range d.nhom {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if strings.HasPrefix(k, "hoi") {
			t.Logf("%-26s %s", k, d.nhom[k].hoi())
		} else {
			t.Logf("%-26s %s", k, d.nhom[k].quan())
		}
	}
	t.Logf("lot hoi: %v", d.lotHoi)
	t.Logf("lot quan: %v", d.lotQuan)
	t.Logf("an_kieng quan sai: %v", d.kiengQuanSai)
	t.Logf("thua hoi: %v", d.thuaHoi)
	t.Logf("an_kieng hoi lech: %v", d.kiengHoi)
	t.Logf("an_kieng quan lot: %v", d.lotKiengQuan)
	t.Logf("thua quan: %v", d.thuaQuan)
}

func docCorpusDiUng(t *testing.T, path string) corpusDiUng {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var c corpusDiUng
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

// The development half of di_ung_heldout_v2 (testdata/di_ung_dev_v2.json):
// 205 asker sentences and 114 place texts, written and labelled by someone
// who read neither tuvung nor rag, under the round-2 rule that an allergen
// the sentence names in an allergy context is a label even when the
// sentence also asks about others, says «sống», «tái» or ends in «thì ok».
// Its sealed twin is opened only by the reviewer, after the commit.
//
// Measured on a609187 before any rule of this round changed -- a true
// held-out number for that code: askers 97/176 recalled, places 3/72
// allergens missed (Muc, Hau, cua as bare kinds) and 4 read as serving a
// diet they do not (Halal: N/A, vegetarian: false, halal_certified: false,
// vegetarian: null), 11/25 diets missed. The rules were then written with
// this half open, so the numbers below are fitted, not blind.
//
// Gated only in the unsafe direction, miss for miss: an asker allergen
// missed, a place allergen left untagged, a place read as serving a diet.
// Over-reads and missed diets are reported.
//
// RAG_DI_UNG_CORPUS may name another corpus of the same schema -- the
// reviewer's sealed half -- which is then measured through exactly this
// scoring and reported, never gated.
const diUngDevSHA256 = "a75d5bbf46d564854f953be19968a203f53b893178df52557eb55ebb85d4bc9b"

var ghimDiUngDev = struct{ lotHoi, lotQuan, kiengQuanSai []string }{}

func TestCorpusDiUngDev(t *testing.T) {
	raw, err := os.ReadFile("testdata/di_ung_dev_v2.json")
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != diUngDevSHA256 {
		t.Fatalf("the development corpus changed: sha256 %s, pinned %s", got, diUngDevSHA256)
	}
	c := docCorpusDiUng(t, "testdata/di_ung_dev_v2.json")
	if c.QuyUoc.Tap != "dev" || len(c.NguoiHoi) != 205 || len(c.Quan) != 114 {
		t.Fatalf("set %q with %d asker rows and %d place rows; want dev, 205 and 114", c.QuyUoc.Tap, len(c.NguoiHoi), len(c.Quan))
	}
	d := doDiUng(t, c)
	d.baoCao(t)
	w := ghimDiUngDev
	for _, x := range []struct {
		ten       string
		got, want []string
	}{
		{"asker allergens missed", d.lotHoi, w.lotHoi},
		{"place allergens left untagged", d.lotQuan, w.lotQuan},
		{"places read as serving a diet they do not", d.kiengQuanSai, w.kiengQuanSai},
	} {
		if strings.Join(x.got, " ") != strings.Join(x.want, " ") {
			t.Errorf("%s:\n got  %q\n want %q", x.ten, x.got, x.want)
		}
	}
	if path := os.Getenv("RAG_DI_UNG_CORPUS"); path != "" {
		t.Logf("---- %s, reported only", path)
		doDiUng(t, docCorpusDiUng(t, path)).baoCao(t)
	}
}
