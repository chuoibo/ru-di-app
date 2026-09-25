package huongdan

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
)

// SoTay is a parsed and validated manual. Immutable once built, so safe for
// concurrent use.
type SoTay struct {
	trang    []*trang          // one per manual file, by file name
	trangCua map[string]*trang // route id -> its manual
	doan     []Doan            // every section, file by file, in file order
	theoID   map[string]int    // section id -> index in doan
	chiMuc   *xephang.ChiMuc   // over chuChiMuc of each section, ids = section ids
	thuat    []map[string]bool // terms of each section's indexed text, for tuDem
	cacMan   []string          // every route id in _rut.json, sorted
	coMan    map[string]bool
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
	s := &SoTay{trangCua: map[string]*trang{}, theoID: map[string]int{}, coMan: map[string]bool{}}
	for _, r := range rut.Routes {
		s.cacMan = append(s.cacMan, r.Man)
		s.coMan[r.Man] = true
	}
	sort.Strings(s.cacMan)

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

	// Cross-file rules: the routes exist, one manual per screen.
	for _, t := range s.trang {
		if !s.coMan[t.man] {
			loi = append(loi, fmt.Errorf("%s.md: màn «%s» không có trong _rut.json", t.ten, t.man))
		}
		if khac, ok := s.trangCua[t.man]; ok {
			loi = append(loi, fmt.Errorf("%s.md: màn «%s» đã có sổ tay %s.md", t.ten, t.man, khac.ten))
		}
		s.trangCua[t.man] = t
		for _, d := range t.diToi {
			if !s.coMan[d.Man] {
				loi = append(loi, fmt.Errorf("%s.md: di_toi «%s» tới «%s» không có trong _rut.json", t.ten, d.Nhan, d.Man))
			}
		}
	}
	if len(loi) > 0 {
		return nil, errors.Join(loi...)
	}
	for _, t := range s.trang {
		if t.tien {
			if err := kiemManTien(t, s.trang); err != nil {
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
// this package has not been taught to read.
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
	}
	return &r, nil
}

var (
	reBuoc  = regexp.MustCompile(`^(?:\d+\.|[-*])\s+(.*)$`)
	reTrich = regexp.MustCompile(`«([^«»]*)»`)
)

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
// §4b, «Màn tiền chỉ có đoạn điều hướng»). It must have exactly one section;
// every non-blank line of that section must be a step; and every step must
// quote at least one door of this screen: a label some manual's di_toi uses to
// come here, a label this manual's di_toi uses to leave, or the title of a
// screen whose manual has a di_toi here (the place a person starts from). A
// step that names no door («Bấm «Tiền đã về» khi đã nhận») is how-to-pay text
// and refuses the whole manual. Digits were already refused in docTrang.
func kiemManTien(t *trang, tatCa []*trang) error {
	if len(t.doan) != 1 {
		return fmt.Errorf("màn tiền chỉ được có một mục chỉ đường, đang có %d", len(t.doan))
	}
	cua := map[string]bool{}
	for _, d := range t.diToi {
		cua[d.Nhan] = true
	}
	for _, khac := range tatCa {
		for _, d := range khac.diToi {
			if d.Man == t.man {
				cua[d.Nhan] = true
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
		coCua := false
		for _, q := range reTrich.FindAllStringSubmatch(m[1], -1) {
			if cua[q[1]] {
				coCua = true
				break
			}
		}
		if !coCua {
			return fmt.Errorf("màn tiền: bước không chỉ lối vào hay lối ra nào: %q", m[1])
		}
	}
	return nil
}

// chuDeXep is the text a section is ranked on: its heading, then its body,
// whose «…» are the labels. Measured on testdata/truy-hoi-so-tay.json
// (recall@5 / MRR): body alone 0.9615 / 0.8148; heading + body 0.9725 /
// 0.8560; heading twice + body the same; screen title + heading + body
// 0.9615 / 0.8590, because «Chat nhóm» on every chat section makes all of them
// match «nhóm» and pinning then buries the answer on another screen.
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
