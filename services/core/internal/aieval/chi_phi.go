package aieval

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"regexp"
	"sort"
	"strings"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// Cost (design 06 §4, last row): tokens from each recorded call's
// UsageMetadata times the prices in gia-model.json, in exact rationals
// (math/big.Rat) from the first multiplication to the last sum, rounded
// once, for display only. No float anywhere: a price is read from its
// decimal string, never from a JSON number.
//
// The prices are configuration a person confirms, not facts this code
// knows. Until someone copies them from the provider's price page -- with the
// page and the date -- every price is null and the run prints «chưa có giá»
// where a number would be.

// GiaMot is one model's prices, per million tokens, as decimal strings.
type GiaMot struct {
	VaoMoiTrieu   *string `json:"vao_moi_trieu"`
	RaMoiTrieu    *string `json:"ra_moi_trieu,omitempty"`
	CacheMoiTrieu *string `json:"cache_moi_trieu,omitempty"`
	// Nguon (the page the prices were copied from) and NgayXacNhan (when)
	// are required once any price is set.
	Nguon       *string `json:"nguon"`
	NgayXacNhan *string `json:"ngay_xac_nhan"`
}

// BangGia is gia-model.json.
type BangGia struct {
	GhiChu string `json:"ghi_chu"`
	// DonVi is the currency the prices are in: "USD" or "VND".
	DonVi string `json:"don_vi"`
	// TyGiaVND, when DonVi is USD, turns the total into đồng as well.
	TyGiaVND *string           `json:"ty_gia_vnd"`
	Model    map[string]GiaMot `json:"model"`
}

var soThapPhan = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)

func ratCua(s *string) (*big.Rat, error) {
	if s == nil {
		return nil, nil
	}
	if !soThapPhan.MatchString(*s) {
		return nil, fmt.Errorf("giá %q không phải số thập phân không âm", *s)
	}
	r, ok := new(big.Rat).SetString(*s)
	if !ok {
		return nil, fmt.Errorf("giá %q không đọc được", *s)
	}
	return r, nil
}

// DocGia reads and checks a price file.
func DocGia(path string) (*BangGia, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var g BangGia
	if err := giaiMaChat(raw, &g); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := g.Kiem(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &g, nil
}

// Kiem checks the file: a known currency, every price a decimal, and no
// price without the page and the date it was confirmed on.
func (g *BangGia) Kiem() error {
	if g.DonVi != "USD" && g.DonVi != "VND" {
		return fmt.Errorf("don_vi %q: phải là USD hoặc VND", g.DonVi)
	}
	if strings.TrimSpace(g.GhiChu) == "" {
		return errors.New("thiếu ghi_chu")
	}
	if _, err := ratCua(g.TyGiaVND); err != nil {
		return fmt.Errorf("ty_gia_vnd: %w", err)
	}
	if g.DonVi == "VND" && g.TyGiaVND != nil {
		return errors.New("ty_gia_vnd chỉ dùng khi don_vi là USD")
	}
	if len(g.Model) == 0 {
		return errors.New("không có model nào")
	}
	for ten, m := range g.Model {
		coGia := false
		for _, p := range []*string{m.VaoMoiTrieu, m.RaMoiTrieu, m.CacheMoiTrieu} {
			if _, err := ratCua(p); err != nil {
				return fmt.Errorf("%s: %w", ten, err)
			}
			coGia = coGia || p != nil
		}
		if coGia && (m.Nguon == nil || strings.TrimSpace(*m.Nguon) == "" || m.NgayXacNhan == nil || strings.TrimSpace(*m.NgayXacNhan) == "") {
			return fmt.Errorf("%s: có giá mà thiếu nguon hoặc ngay_xac_nhan -- giá phải có người xác nhận", ten)
		}
	}
	return nil
}

// Token is a sum of token counts by how they are billed.
type Token struct {
	// Vao is input read at the full price: the prompt less its cached part,
	// plus the tool-use prompt.
	Vao int64 `json:"vao"`
	// Cache is input served from the context cache.
	Cache int64 `json:"cache"`
	// Ra is output: candidates plus thoughts.
	Ra int64 `json:"ra"`
	// Nghi is the thoughts part of Ra, shown separately.
	Nghi int64 `json:"nghi"`
}

// TokenCuaGoi is the tokens of one recorded call: the last UsageMetadata it
// yielded (a streamed call repeats the running total on later chunks).
func TokenCuaGoi(g GoiGhi) (Token, bool, error) {
	var u *genai.GenerateContentResponseUsageMetadata
	for _, ch := range g.PhanHoi {
		if ch.Resp == nil {
			continue
		}
		var r model.LLMResponse
		if err := json.Unmarshal(ch.Resp, &r); err != nil {
			return Token{}, false, err
		}
		if r.UsageMetadata != nil {
			u = r.UsageMetadata
		}
	}
	if u == nil {
		return Token{}, false, nil
	}
	return Token{
		Vao:   int64(u.PromptTokenCount) - int64(u.CachedContentTokenCount) + int64(u.ToolUsePromptTokenCount),
		Cache: int64(u.CachedContentTokenCount),
		Ra:    int64(u.CandidatesTokenCount) + int64(u.ThoughtsTokenCount),
		Nghi:  int64(u.ThoughtsTokenCount),
	}, true, nil
}

// Cong adds b to t.
func (t *Token) Cong(b Token) {
	t.Vao += b.Vao
	t.Cache += b.Cache
	t.Ra += b.Ra
	t.Nghi += b.Nghi
}

var motTrieu = big.NewRat(1_000_000, 1)

// ChiPhi is a run's cost: exact, and as displayed.
type ChiPhi struct {
	// CoGia says every price the tokens need is set.
	CoGia bool `json:"co_gia"`
	// ThieuGia names each price that is missing.
	ThieuGia []string `json:"thieu_gia,omitempty"`
	// ChinhXac is the exact total in DonVi, as a fraction ("a/b"); empty
	// without prices.
	ChinhXac string `json:"chinh_xac,omitempty"`
	DonVi    string `json:"don_vi"`
	// Tong and MoiLuot are rounded once from the exact values; in đồng too
	// when a rate is set. Without prices they read «chưa có giá».
	Tong        string `json:"tong"`
	MoiLuot     string `json:"moi_luot"`
	TongDong    string `json:"tong_dong,omitempty"`
	MoiLuotDong string `json:"moi_luot_dong,omitempty"`

	tong *big.Rat
}

// ChuaCoGia is what a cost reads without a confirmed price.
const ChuaCoGia = "chưa có giá"

// TinhChiPhi prices tok of modelTen over soLuot runs.
func TinhChiPhi(g *BangGia, modelTen string, tok Token, soLuot int) ChiPhi {
	out := ChiPhi{DonVi: g.DonVi, Tong: ChuaCoGia, MoiLuot: ChuaCoGia}
	m, ok := g.Model[modelTen]
	if !ok {
		out.ThieuGia = []string{modelTen + ": không có trong gia-model.json"}
		return out
	}
	tong := new(big.Rat)
	them := func(ten string, p *string, n int64) {
		if n == 0 {
			return
		}
		r, _ := ratCua(p) // Kiem already refused a malformed price
		if r == nil {
			out.ThieuGia = append(out.ThieuGia, modelTen+"."+ten)
			return
		}
		x := new(big.Rat).Mul(big.NewRat(n, 1), r)
		tong.Add(tong, x.Quo(x, motTrieu))
	}
	them("vao_moi_trieu", m.VaoMoiTrieu, tok.Vao)
	them("cache_moi_trieu", m.CacheMoiTrieu, tok.Cache)
	them("ra_moi_trieu", m.RaMoiTrieu, tok.Ra)
	sort.Strings(out.ThieuGia)
	if len(out.ThieuGia) > 0 {
		return out
	}
	out.CoGia, out.tong, out.ChinhXac = true, tong, tong.RatString()
	chuSo := 6
	if g.DonVi == "VND" {
		chuSo = 0
	}
	out.Tong = LamTron(tong, chuSo) + " " + g.DonVi
	moi := new(big.Rat)
	if soLuot > 0 {
		moi.Quo(tong, big.NewRat(int64(soLuot), 1))
	}
	out.MoiLuot = LamTron(moi, chuSo) + " " + g.DonVi
	if g.DonVi == "USD" && g.TyGiaVND != nil {
		ty, _ := ratCua(g.TyGiaVND)
		out.TongDong = LamTron(new(big.Rat).Mul(tong, ty), 0) + " đồng"
		out.MoiLuotDong = LamTron(new(big.Rat).Mul(moi, ty), 0) + " đồng"
	}
	return out
}

// LamTron renders a non-negative r with chuSo decimals, rounding half up
// exactly once.
func LamTron(r *big.Rat, chuSo int) string {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(chuSo)), nil)
	num := new(big.Int).Mul(r.Num(), scale)
	num.Mul(num, big.NewInt(2))
	num.Add(num, r.Denom())
	den := new(big.Int).Mul(r.Denom(), big.NewInt(2))
	q := new(big.Int).Quo(num, den) // floor(r·10^chuSo + 1/2) for r ≥ 0
	s := q.String()
	if chuSo == 0 {
		return s
	}
	if len(s) <= chuSo {
		s = strings.Repeat("0", chuSo-len(s)+1) + s
	}
	return s[:len(s)-chuSo] + "." + s[len(s)-chuSo:]
}
