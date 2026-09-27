package ingest

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Province is one of the 34 provinces Vietnam reorganised into on 1 July 2025.
//
// The list is carried in source rather than read from the feed's repository at
// runtime, because a lookup table the catalogue depends on must not disappear
// when a sibling checkout moves. It is generated from the same seed file the
// feed normalises against: two independently sourced lists of these provinces
// that disagree on one code produce an empty province nobody notices, and the
// most popular community dataset still returns the pre-2025 sixty-three.
type Province struct {
	Code         int16
	Name         string
	NameNorm     string
	DivisionType string
	Codename     string
}

// Provinces is the whole list, ordered by official code.
var Provinces = []Province{
	{Code: 1, Name: "Thành phố Hà Nội", NameNorm: "thanh pho ha noi", DivisionType: "thành phố trung ương", Codename: "ha_noi"},
	{Code: 4, Name: "Tỉnh Cao Bằng", NameNorm: "tinh cao bang", DivisionType: "tỉnh", Codename: "cao_bang"},
	{Code: 8, Name: "Tỉnh Tuyên Quang", NameNorm: "tinh tuyen quang", DivisionType: "tỉnh", Codename: "tuyen_quang"},
	{Code: 11, Name: "Tỉnh Điện Biên", NameNorm: "tinh dien bien", DivisionType: "tỉnh", Codename: "dien_bien"},
	{Code: 12, Name: "Tỉnh Lai Châu", NameNorm: "tinh lai chau", DivisionType: "tỉnh", Codename: "lai_chau"},
	{Code: 14, Name: "Tỉnh Sơn La", NameNorm: "tinh son la", DivisionType: "tỉnh", Codename: "son_la"},
	{Code: 15, Name: "Tỉnh Lào Cai", NameNorm: "tinh lao cai", DivisionType: "tỉnh", Codename: "lao_cai"},
	{Code: 19, Name: "Tỉnh Thái Nguyên", NameNorm: "tinh thai nguyen", DivisionType: "tỉnh", Codename: "thai_nguyen"},
	{Code: 20, Name: "Tỉnh Lạng Sơn", NameNorm: "tinh lang son", DivisionType: "tỉnh", Codename: "lang_son"},
	{Code: 22, Name: "Tỉnh Quảng Ninh", NameNorm: "tinh quang ninh", DivisionType: "tỉnh", Codename: "quang_ninh"},
	{Code: 24, Name: "Tỉnh Bắc Ninh", NameNorm: "tinh bac ninh", DivisionType: "tỉnh", Codename: "bac_ninh"},
	{Code: 25, Name: "Tỉnh Phú Thọ", NameNorm: "tinh phu tho", DivisionType: "tỉnh", Codename: "phu_tho"},
	{Code: 31, Name: "Thành phố Hải Phòng", NameNorm: "thanh pho hai phong", DivisionType: "thành phố trung ương", Codename: "hai_phong"},
	{Code: 33, Name: "Tỉnh Hưng Yên", NameNorm: "tinh hung yen", DivisionType: "tỉnh", Codename: "hung_yen"},
	{Code: 37, Name: "Tỉnh Ninh Bình", NameNorm: "tinh ninh binh", DivisionType: "tỉnh", Codename: "ninh_binh"},
	{Code: 38, Name: "Tỉnh Thanh Hóa", NameNorm: "tinh thanh hoa", DivisionType: "tỉnh", Codename: "thanh_hoa"},
	{Code: 40, Name: "Tỉnh Nghệ An", NameNorm: "tinh nghe an", DivisionType: "tỉnh", Codename: "nghe_an"},
	{Code: 42, Name: "Tỉnh Hà Tĩnh", NameNorm: "tinh ha tinh", DivisionType: "tỉnh", Codename: "ha_tinh"},
	{Code: 44, Name: "Tỉnh Quảng Trị", NameNorm: "tinh quang tri", DivisionType: "tỉnh", Codename: "quang_tri"},
	{Code: 46, Name: "Thành phố Huế", NameNorm: "thanh pho hue", DivisionType: "thành phố trung ương", Codename: "hue"},
	{Code: 48, Name: "Thành phố Đà Nẵng", NameNorm: "thanh pho da nang", DivisionType: "thành phố trung ương", Codename: "da_nang"},
	{Code: 51, Name: "Tỉnh Quảng Ngãi", NameNorm: "tinh quang ngai", DivisionType: "tỉnh", Codename: "quang_ngai"},
	{Code: 52, Name: "Tỉnh Gia Lai", NameNorm: "tinh gia lai", DivisionType: "tỉnh", Codename: "gia_lai"},
	{Code: 56, Name: "Tỉnh Khánh Hòa", NameNorm: "tinh khanh hoa", DivisionType: "tỉnh", Codename: "khanh_hoa"},
	{Code: 66, Name: "Tỉnh Đắk Lắk", NameNorm: "tinh dak lak", DivisionType: "tỉnh", Codename: "dak_lak"},
	{Code: 68, Name: "Tỉnh Lâm Đồng", NameNorm: "tinh lam dong", DivisionType: "tỉnh", Codename: "lam_dong"},
	{Code: 75, Name: "Tỉnh Đồng Nai", NameNorm: "tinh dong nai", DivisionType: "tỉnh", Codename: "dong_nai"},
	{Code: 79, Name: "Thành phố Hồ Chí Minh", NameNorm: "thanh pho ho chi minh", DivisionType: "thành phố trung ương", Codename: "ho_chi_minh"},
	{Code: 80, Name: "Tỉnh Tây Ninh", NameNorm: "tinh tay ninh", DivisionType: "tỉnh", Codename: "tay_ninh"},
	{Code: 82, Name: "Tỉnh Đồng Tháp", NameNorm: "tinh dong thap", DivisionType: "tỉnh", Codename: "dong_thap"},
	{Code: 86, Name: "Tỉnh Vĩnh Long", NameNorm: "tinh vinh long", DivisionType: "tỉnh", Codename: "vinh_long"},
	{Code: 91, Name: "Tỉnh An Giang", NameNorm: "tinh an giang", DivisionType: "tỉnh", Codename: "an_giang"},
	{Code: 92, Name: "Thành phố Cần Thơ", NameNorm: "thanh pho can tho", DivisionType: "thành phố trung ương", Codename: "can_tho"},
	{Code: 96, Name: "Tỉnh Cà Mau", NameNorm: "tinh ca mau", DivisionType: "tỉnh", Codename: "ca_mau"},
}

// SeedProvinces writes the list and updates any row whose spelling changed.
// Idempotent: calling it twice changes nothing the second time.
func SeedProvinces(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	batch := &pgx.Batch{}
	for _, p := range Provinces {
		batch.Queue(`
			INSERT INTO admin_province (code, name, name_norm, division_type, codename)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (code) DO UPDATE SET
			  name = EXCLUDED.name,
			  name_norm = EXCLUDED.name_norm,
			  division_type = EXCLUDED.division_type,
			  codename = EXCLUDED.codename`,
			p.Code, p.Name, p.NameNorm, p.DivisionType, p.Codename)
	}
	results := pool.SendBatch(ctx, batch)
	var written int64
	for range Provinces {
		tag, err := results.Exec()
		if err != nil {
			results.Close()
			return written, err
		}
		written += tag.RowsAffected()
	}
	return written, results.Close()
}
