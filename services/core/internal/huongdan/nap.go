package huongdan

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"mobile/services/core/internal/rag/xephang"
)

const (
	duongRut = "data/_rut.json"
	mauSoTay = "data/*.md"
	// thuMucTab is where expo-router keeps the tab screens; _rut.json records
	// it in each route's `tep`.
	thuMucTab = "app/(tabs)/"
	// tieuDeManTien is the one heading a money screen's single section may
	// have. A heading is text a person reads like any step, and a free one is
	// room for «how to pay» that no door rule looks at; fixed, it says nothing
	// but «how to get here and on».
	tieuDeManTien = "Tới màn này và đi tiếp"
)

// canhNgoaiRut are the manual edges that are real although no route's code
// shows them, each with the reason. A manual's di_toi must be an edge of the
// code (_rut.json di_toi), a tab-bar edge, or one of these; a test holds every
// entry to the manuals that use it and to the source that makes it real.
var canhNgoaiRut = map[[2]string]string{
	{"plan", "create"}: "nút tròn «Tạo mới» của thanh tab (src/rudi/ui/RudiTabBar.tsx, router.push(\"/create\")) " +
		"do app/(tabs)/_layout.tsx vẽ; _layout không phải route nên bộ rút không gán cạnh này cho màn nào",
}

// SoTay is a parsed and validated manual. Immutable once built, so safe for
// concurrent use.
type SoTay struct {
	trang    []*trang          // one per manual file, by file name
	trangCua map[string]*trang // route id -> its manual
	doan     []Doan            // every section, file by file, in file order
	theoID   map[string]int    // section id -> index in doan
	chiMuc   *xephang.ChiMuc   // over chuChiMuc of each section, ids = section ids
	thuat    []map[string]bool // terms of each section's indexed text, for tuDem
	tuVung   map[string]bool   // every term of every section: the manual's own words, for chuanHoi
	cacMan   []string          // every route id in _rut.json, sorted
	coMan    map[string]bool
	tab      []string          // routes whose file sits in app/(tabs)/, sorted
	banDo    *banDoRut         // _rut.json as the rules read it
	ke       map[string][]canh // from -> edges, sorted by Den
	nguoc    map[string][]string
	ban      string
}

type trang struct {
	ten      string // file stem: «chat-nhom»
	man      string
	tieuDe   string
	tongQuan string
	nhanUI   map[string]bool
	diToi    []canhSoTay
	tien     bool
	doan     []Doan
}

type canhSoTay struct {
	Nhan string `json:"nhan"`
	Man  string `json:"man"`
}

// dauTrang is the front matter. Pointers tell a missing key from a zero one.
type dauTrang struct {
	Man    *string      `json:"man"`
	TieuDe *string      `json:"tieu_de"`
	NhanUI *[]string    `json:"nhanUI"`
	DiToi  *[]canhSoTay `json:"di_toi"`
	Tien   *bool        `json:"tien"`
}

type banRut struct {
	Routes []tuyenRut `json:"routes"`
}

type tuyenRut struct {
	Man   string   `json:"man"`
	DiToi []string `json:"di_toi"`
	Nhan  []string `json:"nhan"`
	Tep   []string `json:"tep"`
	// Canh are the labelled edges: a label and a navigation on one thing a
	// person taps (see apps/mobile/tools/rut-huong-dan.mjs).
	Canh []canhRut `json:"canh"`
}

type canhRut struct {
	Den  string `json:"den"`
	Nhan string `json:"nhan"`
}

// banDoRut is _rut.json as the rules read it.
type banDoRut struct {
	diToi map[string]map[string]bool  // route -> routes its code navigates to
	nhan  map[string]map[string]bool  // route -> labels printed on it
	canh  map[string]map[canhRut]bool // route -> its labelled edges
	tab   map[string]bool             // tab routes
}

func (b *banDoRut) laCanhMa(tu, den string) bool {
	return b.diToi[tu][den] || (b.tab[tu] && b.tab[den])
}

// phaiNap is nap for package init: the manual this binary carries either
// loads whole or the process does not start. A test feeds it a broken
// manual and expects the panic, so init is proven to run the same checks.
func phaiNap(fsys fs.FS) *SoTay {
	s, err := nap(fsys)
	if err != nil {
		panic("huongdan: " + err.Error())
	}
	return s
}

// nap reads data/_rut.json and data/*.md from fsys, checks every rule, and
// builds the index and the screen graph.
func nap(fsys fs.FS) (*SoTay, error) {
	thoRut, err := fs.ReadFile(fsys, duongRut)
	if err != nil {
		return nil, fmt.Errorf("đọc %s: %w", duongRut, err)
	}
	rut, err := docRut(thoRut)
	if err != nil {
		return nil, err
	}
	s := &SoTay{trangCua: map[string]*trang{}, theoID: map[string]int{}, coMan: map[string]bool{}, tuVung: map[string]bool{}}
	bd := &banDoRut{diToi: map[string]map[string]bool{}, nhan: map[string]map[string]bool{}, canh: map[string]map[canhRut]bool{}, tab: map[string]bool{}}
	s.banDo = bd
	for _, r := range rut.Routes {
		s.cacMan = append(s.cacMan, r.Man)
		s.coMan[r.Man] = true
		bd.diToi[r.Man] = map[string]bool{}
		for _, d := range r.DiToi {
			bd.diToi[r.Man][d] = true
		}
		bd.nhan[r.Man] = map[string]bool{}
		for _, n := range r.Nhan {
			bd.nhan[r.Man][n] = true
		}
		bd.canh[r.Man] = map[canhRut]bool{}
		for _, c := range r.Canh {
			bd.canh[r.Man][c] = true
		}
		for _, tep := range r.Tep {
			if strings.HasPrefix(tep, thuMucTab) {
				bd.tab[r.Man] = true
				s.tab = append(s.tab, r.Man)
				break
			}
		}
	}
	sort.Strings(s.cacMan)
	sort.Strings(s.tab)

	tenTep, err := fs.Glob(fsys, mauSoTay)
	if err != nil {
		return nil, err
	}
	sort.Strings(tenTep)
	if len(tenTep) == 0 {
		return nil, errors.New("không có file sổ tay nào")
	}
	var loi []error
	for _, ten := range tenTep {
		tho, err := fs.ReadFile(fsys, ten)
		if err != nil {
			return nil, err
		}
		t, err := docTrang(strings.TrimSuffix(path.Base(ten), ".md"), string(tho))
		if err != nil {
			loi = append(loi, fmt.Errorf("%s: %w", path.Base(ten), err))
			continue
		}
		s.trang = append(s.trang, t)
	}
	if len(loi) > 0 {
		return nil, errors.Join(loi...)
	}

	// Cross-file rules: the routes exist, one manual per screen, and every
	// way a manual declares is a way the app has.
	for _, t := range s.trang {
		if !s.coMan[t.man] {
			loi = append(loi, fmt.Errorf("%s.md: màn «%s» không có trong _rut.json", t.ten, t.man))
		}
		// The money flag is not the file's to decide: it follows the route.
		if t.tien != laManTien(t.man) {
			loi = append(loi, fmt.Errorf("%s.md: tien phải là %v cho màn «%s»", t.ten, laManTien(t.man), t.man))
		}
		if khac, ok := s.trangCua[t.man]; ok {
			loi = append(loi, fmt.Errorf("%s.md: màn «%s» đã có sổ tay %s.md", t.ten, t.man, khac.ten))
		}
		s.trangCua[t.man] = t
		for _, d := range t.diToi {
			if !s.coMan[d.Man] {
				loi = append(loi, fmt.Errorf("%s.md: di_toi «%s» tới «%s» không có trong _rut.json", t.ten, d.Nhan, d.Man))
				continue
			}
			if _, ngoai := canhNgoaiRut[[2]string{t.man, d.Man}]; s.coMan[t.man] && !bd.laCanhMa(t.man, d.Man) && !ngoai {
				loi = append(loi, fmt.Errorf("%s.md: di_toi «%s» từ «%s» tới «%s» không phải cạnh nào của mã (_rut.json di_toi, thanh tab, canhNgoaiRut)", t.ten, d.Nhan, t.man, d.Man))
			}
		}
	}
	if len(loi) > 0 {
		return nil, errors.Join(loi...)
	}
	for _, t := range s.trang {
		if t.tien {
			if err := kiemManTien(t, s.trang, bd); err != nil {
				loi = append(loi, fmt.Errorf("%s.md: %w", t.ten, err))
			}
		}
	}
	if len(loi) > 0 {
		return nil, errors.Join(loi...)
	}

	var chuChiMuc []xephang.Doan
	for _, t := range s.trang {
		for _, d := range t.doan {
			if _, ok := s.theoID[d.ID]; ok {
				return nil, fmt.Errorf("id mục trùng: %s", d.ID)
			}
			s.theoID[d.ID] = len(s.doan)
			s.doan = append(s.doan, d)
			chu := chuDeXep(d)
			chuChiMuc = append(chuChiMuc, xephang.Doan{ID: d.ID, Chu: chu})
			thuat := map[string]bool{}
			for _, th := range xephang.Thuat(chu) {
				thuat[th] = true
				s.tuVung[th] = true
			}
			s.thuat = append(s.thuat, thuat)
		}
	}
	s.chiMuc = xephang.Dung(chuChiMuc)
	s.dungDoThi(rut)

	tong := sha256.Sum256(thoRut)
	s.ban = hex.EncodeToString(tong[:])[:12]
	return s, nil
}

// docRut decodes _rut.json strictly: an unknown field is a changed extractor
// this package has not been taught to read. A labelled edge must be an edge
// of its route (in di_toi) with a label printed on it (in nhan): the
// extractor writes it that way, and a hand edit that breaks it is refused.
func docRut(tho []byte) (*banRut, error) {
	dec := json.NewDecoder(bytes.NewReader(tho))
	dec.DisallowUnknownFields()
	var r banRut
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("_rut.json: %w", err)
	}
	if dec.More() {
		return nil, errors.New("_rut.json: có dữ liệu thừa sau object")
	}
	if len(r.Routes) == 0 {
		return nil, errors.New("_rut.json: không có route nào")
	}
	co := map[string]bool{}
	for _, t := range r.Routes {
		if t.Man == "" {
			return nil, errors.New("_rut.json: route không có man")
		}
		if co[t.Man] {
			return nil, fmt.Errorf("_rut.json: route «%s» trùng", t.Man)
		}
		co[t.Man] = true
	}
	for _, t := range r.Routes {
		for _, d := range t.DiToi {
			if !co[d] {
				return nil, fmt.Errorf("_rut.json: «%s» đi tới «%s» không có trong cây route", t.Man, d)
			}
		}
		for _, c := range t.Canh {
			if !chua(t.DiToi, c.Den) {
				return nil, fmt.Errorf("_rut.json: cạnh có nhãn «%s» của «%s» tới «%s» không có trong di_toi", c.Nhan, t.Man, c.Den)
			}
			if c.Nhan == "" || !chua(t.Nhan, c.Nhan) {
				return nil, fmt.Errorf("_rut.json: cạnh có nhãn «%s» của «%s» mang nhãn không có trong nhan", c.Nhan, t.Man)
			}
		}
	}
	return &r, nil
}

var (
	reBuoc  = regexp.MustCompile(`^(?:\d+\.|[-*])\s+(.*)$`)
	reTrich = regexp.MustCompile(`«([^«»]*)»`)
)

// tenKhoaDau are the only spellings a front-matter key may have: the five
// keys of dauTrang and the two of a di_toi entry.
var tenKhoaDau = map[string]bool{"man": true, "tieu_de": true, "nhanUI": true, "di_toi": true, "tien": true, "nhan": true}

// kiemKhoaDau walks the tokens of front matter that already decoded and
// refuses a key that appears twice in one object, or one spelled other than
// as the field it fills. encoding/json does neither: it keeps the last of two
// keys and matches a key to a field ignoring case, so a second «nhanUI» (or
// an «NhanUI» after it) would silently replace the list every rule reads.
func kiemKhoaDau(fm string) error {
	type khung struct {
		khoa    map[string]bool // keys seen so far; nil for an array
		choKhoa bool            // the next string token is a key
	}
	var ngan []*khung
	xongGiaTri := func() {
		if n := len(ngan); n > 0 && ngan[n-1].khoa != nil {
			ngan[n-1].choKhoa = true
		}
	}
	dec := json.NewDecoder(strings.NewReader(fm))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("front matter không đọc được: %w", err)
		}
		if d, ok := tok.(json.Delim); ok {
			switch d {
			case '{':
				ngan = append(ngan, &khung{khoa: map[string]bool{}, choKhoa: true})
			case '[':
				ngan = append(ngan, &khung{})
			default:
				ngan = ngan[:len(ngan)-1]
				xongGiaTri()
			}
			continue
		}
		if n := len(ngan); n > 0 && ngan[n-1].choKhoa {
			k, _ := tok.(string)
			if ngan[n-1].khoa[k] {
				return fmt.Errorf("front matter có khoá trùng «%s»", k)
			}
			if !tenKhoaDau[k] {
				return fmt.Errorf("front matter: khoá «%s» phải viết đúng hoa thường như tên khoá", k)
			}
			ngan[n-1].khoa[k] = true
			ngan[n-1].choKhoa = false
			continue
		}
		xongGiaTri()
	}
}

// docTrang parses one manual file and checks every rule that file can check
// alone.
func docTrang(ten, noiDung string) (*trang, error) {
	dong := strings.Split(noiDung, "\n")
	if dong[0] != "---json" {
		return nil, errors.New("dòng đầu phải là ---json")
	}
	het := -1
	for i := 1; i < len(dong); i++ {
		if dong[i] == "---" {
			het = i
			break
		}
	}
	if het == -1 {
		return nil, errors.New("thiếu dòng --- đóng front matter")
	}
	dec := json.NewDecoder(strings.NewReader(strings.Join(dong[1:het], "\n")))
	dec.DisallowUnknownFields()
	var dau dauTrang
	if err := dec.Decode(&dau); err != nil {
		return nil, fmt.Errorf("front matter không đọc được: %w", err)
	}
	if dec.More() {
		return nil, errors.New("front matter có dữ liệu thừa")
	}
	if err := kiemKhoaDau(strings.Join(dong[1:het], "\n")); err != nil {
		return nil, err
	}
	switch {
	case dau.Man == nil || *dau.Man == "":
		return nil, errors.New("front matter thiếu man")
	case dau.TieuDe == nil || strings.TrimSpace(*dau.TieuDe) == "":
		return nil, errors.New("front matter thiếu tieu_de")
	case dau.NhanUI == nil:
		return nil, errors.New("front matter thiếu nhanUI")
	case dau.DiToi == nil:
		return nil, errors.New("front matter thiếu di_toi")
	case dau.Tien == nil:
		return nil, errors.New("front matter thiếu tien")
	}
	t := &trang{ten: ten, man: *dau.Man, tieuDe: *dau.TieuDe, nhanUI: map[string]bool{}, diToi: *dau.DiToi, tien: *dau.Tien}
	for _, n := range *dau.NhanUI {
		if n == "" {
			return nil, errors.New("nhanUI có nhãn rỗng")
		}
		t.nhanUI[n] = true
	}
	for _, d := range t.diToi {
		if d.Nhan == "" || d.Man == "" {
			return nil, errors.New("di_toi phải có nhan và man")
		}
		if !t.nhanUI[d.Nhan] {
			return nil, fmt.Errorf("di_toi đi bằng nhãn «%s» không khai trong nhanUI", d.Nhan)
		}
		// A way to the screen one is already on is not a way anywhere; on a
		// money screen it would turn any button into a «door».
		if d.Man == t.man {
			return nil, fmt.Errorf("di_toi «%s» về chính màn «%s»", d.Nhan, t.man)
		}
	}

	than := dong[het+1:]
	if strings.Count(strings.Join(than, "\n"), "«") != strings.Count(strings.Join(than, "\n"), "»") {
		return nil, errors.New("số « và » không khớp")
	}
	var tongQuan []string
	var muc *Doan
	var chuMuc []string
	daThay := map[string]bool{}
	dong2 := func() error {
		if muc == nil {
			return nil
		}
		muc.Chu = strings.TrimSpace(strings.Join(chuMuc, "\n"))
		if len(muc.Buoc) == 0 {
			return fmt.Errorf("mục «%s» không có bước nào", muc.TieuDe)
		}
		if len(muc.Buoc) > MaxBuoc {
			return fmt.Errorf("mục «%s» có %d bước, tối đa %d", muc.TieuDe, len(muc.Buoc), MaxBuoc)
		}
		t.doan = append(t.doan, *muc)
		return nil
	}
	for _, l := range than {
		if strings.HasPrefix(l, "#") {
			if !strings.HasPrefix(l, "## ") {
				return nil, fmt.Errorf("chỉ dùng tiêu đề «## »: %q", l)
			}
			if err := dong2(); err != nil {
				return nil, err
			}
			tieuDe := strings.TrimSpace(l[3:])
			slug := strings.Join(xephang.AmTiet(tieuDe), "-")
			if slug == "" {
				return nil, fmt.Errorf("tiêu đề mục rỗng: %q", l)
			}
			id := ten + "/" + slug
			if daThay[id] {
				return nil, fmt.Errorf("hai mục cùng id %s", id)
			}
			daThay[id] = true
			muc = &Doan{ID: id, Man: t.man, TieuDeMan: t.tieuDe, TieuDe: tieuDe, Tien: t.tien}
			chuMuc = nil
			// A heading is read like a step: what it quotes is a label too.
			for _, q := range reTrich.FindAllStringSubmatch(tieuDe, -1) {
				if !t.nhanUI[q[1]] {
					return nil, fmt.Errorf("tiêu đề mục «%s» trích «%s» mà nhãn không khai trong nhanUI", tieuDe, q[1])
				}
				if !chua(muc.Nhan, q[1]) {
					muc.Nhan = append(muc.Nhan, q[1])
				}
			}
			continue
		}
		if muc == nil {
			tongQuan = append(tongQuan, l)
			continue
		}
		chuMuc = append(chuMuc, l)
		if m := reBuoc.FindStringSubmatch(l); m != nil {
			muc.Buoc = append(muc.Buoc, strings.TrimSpace(m[1]))
		}
		for _, q := range reTrich.FindAllStringSubmatch(l, -1) {
			if !t.nhanUI[q[1]] {
				return nil, fmt.Errorf("mục «%s» trích «%s» mà nhãn không khai trong nhanUI", muc.TieuDe, q[1])
			}
			if !chua(muc.Nhan, q[1]) {
				muc.Nhan = append(muc.Nhan, q[1])
			}
		}
	}
	if err := dong2(); err != nil {
		return nil, err
	}
	t.tongQuan = strings.TrimSpace(strings.Join(tongQuan, "\n"))
	if t.tongQuan == "" {
		return nil, errors.New("thiếu đoạn tổng quan trước mục ## đầu tiên")
	}
	for _, q := range reTrich.FindAllStringSubmatch(t.tongQuan, -1) {
		if !t.nhanUI[q[1]] {
			return nil, fmt.Errorf("tổng quan trích «%s» mà nhãn không khai trong nhanUI", q[1])
		}
	}
	if len(t.doan) == 0 {
		return nil, errors.New("chưa có mục ## nào")
	}
	if t.tien && strings.IndexFunc(strings.Join(than, "\n"), unicode.IsDigit) >= 0 {
		return nil, errors.New("màn tiền không được có chữ số trong thân")
	}
	return t, nil
}

// kiemManTien holds a money screen's manual to navigation only (design 04
// §4b, «Màn tiền chỉ có đoạn điều hướng»). It must have exactly one section,
// headed tieuDeManTien; every non-blank line of that section must be a step;
// and every step must quote at least one door of this screen and nothing but
// doors. «At least one» alone let a step quote the payment button next to a
// door («…bấm «Đánh dấu đã trả», rồi bấm «Xem quyết toán».», review 13
// round 2); a section heading printed on the screen («Chi theo nhóm») is not
// a door either, so a step names it in plain words.
//
// A door is a way in or out that the code itself shows:
//   - a label this manual's di_toi uses to leave, which must be a labelled
//     edge of this route in _rut.json (a button with that label whose
//     handler goes there);
//   - a label another manual's di_toi uses to come here, held the same way
//     to a labelled edge of that manual's route;
//   - the title of a non-money screen whose manual has a di_toi here, when
//     that title is printed on that screen (the place a person starts from).
//
// A declared way that is not a labelled edge refuses the whole manual: the
// payment button declared as a way out («Đánh dấu đã trả», which leads
// nowhere) is exactly that. A step that names no door («Bấm «Tiền đã về» khi
// đã nhận») is how-to-pay text and refuses it too, and so does a step that
// quotes any label besides its doors. Digits were already refused in
// docTrang. What no rule here can see is how-to-pay prose with no «…» on a
// line that also names a door.
func kiemManTien(t *trang, tatCa []*trang, bd *banDoRut) error {
	if len(t.doan) != 1 {
		return fmt.Errorf("màn tiền chỉ được có một mục chỉ đường, đang có %d", len(t.doan))
	}
	if t.doan[0].TieuDe != tieuDeManTien {
		return fmt.Errorf("màn tiền: tiêu đề mục phải là «%s», đang là «%s»", tieuDeManTien, t.doan[0].TieuDe)
	}
	cua := map[string]bool{}
	for _, d := range t.diToi {
		if !bd.canh[t.man][canhRut{Den: d.Man, Nhan: d.Nhan}] {
			return fmt.Errorf("màn tiền: lối ra «%s» tới «%s» không phải nút nào của «%s» dẫn tới đó (canh trong _rut.json)", d.Nhan, d.Man, t.man)
		}
		cua[d.Nhan] = true
	}
	for _, khac := range tatCa {
		for _, d := range khac.diToi {
			if d.Man != t.man {
				continue
			}
			if !bd.canh[khac.man][canhRut{Den: t.man, Nhan: d.Nhan}] {
				return fmt.Errorf("màn tiền: lối vào «%s» từ %s.md không phải nút nào của «%s» dẫn tới đây (canh trong _rut.json)", d.Nhan, khac.ten, khac.man)
			}
			cua[d.Nhan] = true
			if !khac.tien && bd.nhan[khac.man][khac.tieuDe] {
				cua[khac.tieuDe] = true
			}
		}
	}
	d := t.doan[0]
	for _, l := range strings.Split(d.Chu, "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		m := reBuoc.FindStringSubmatch(l)
		if m == nil {
			return fmt.Errorf("màn tiền: dòng không phải bước chỉ đường: %q", l)
		}
		coCua, ngoai := false, ""
		for _, q := range reTrich.FindAllStringSubmatch(m[1], -1) {
			if cua[q[1]] {
				coCua = true
			} else if ngoai == "" {
				ngoai = q[1]
			}
		}
		if !coCua {
			return fmt.Errorf("màn tiền: bước không chỉ lối vào hay lối ra nào: %q", m[1])
		}
		if ngoai != "" {
			return fmt.Errorf("màn tiền: bước trích «%s», không phải lối vào hay lối ra nào: %q", ngoai, m[1])
		}
	}
	return nil
}

// chuDeXep is the text a section is ranked on: its heading, then its body,
// whose «…» are the labels. Measured on testdata/truy-hoi-so-tay.json
// (recall@5 / MRR, ranking of 5c3a3c1): body alone 0.9615 / 0.8148; heading +
// body 0.9725 / 0.8560; heading twice + body the same; screen title + heading
// + body 0.9615 / 0.8590, because «Chat nhóm» on every chat section makes all
// of them match «nhóm» and pinning then buries the answer on another screen.
func chuDeXep(d Doan) string {
	return d.TieuDe + "\n" + d.Chu
}

func chua(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
