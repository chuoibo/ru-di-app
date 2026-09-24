package ingest

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProvinceBox is a province as a destination: a centre and a bounding box.
//
// Taken from the feed's local OpenStreetMap extract, which carries the
// post-July-2025 two-level structure. Every box was accepted only after eight
// wards of the NEW structure were looked up and found inside it -- a boundary
// still drawn to the old sixty-three provinces fails exactly on the wards that
// were merged in, which is where the check has to bite.
//
// Coordinates are rounded to four decimal places, about eleven metres. A
// province centre does not know itself to seven decimals, and the repository
// guard reads a longer run of digits as something that should not be checked
// in. Box edges round OUTWARD -- floor for south and west, ceiling for north
// and east -- so a box never ends up smaller than the boundary it describes.
//
// These boxes are true geography and poor filters. Ho Chi Minh City reaches
// latitude 8.35 because Con Dao belongs to it now, and Khanh Hoa spans 27
// square degrees because it includes Truong Sa. Use `province_code` to ask
// which province a place is in; use the box only to frame a map.
type ProvinceBox struct {
	Code                     int16
	Name                     string
	Lat, Lng                 float64
	South, West, North, East float64
}

// ProvinceBoxes is every province the extract could draw.
var ProvinceBoxes = []ProvinceBox{
	{Code: 1, Name: "Thành phố Hà Nội", Lat: 21.0283, Lng: 105.854, South: 20.5645, West: 105.2889, North: 21.3855, East: 106.0201},
	{Code: 4, Name: "Tỉnh Cao Bằng", Lat: 22.7427, Lng: 106.1061, South: 22.3569, West: 105.2667, North: 23.1188, East: 106.8377},
	{Code: 8, Name: "Tỉnh Tuyên Quang", Lat: 22.3382, Lng: 105.0716, South: 21.4975, West: 104.3343, North: 23.3927, East: 105.5984},
	{Code: 11, Name: "Tỉnh Điện Biên", Lat: 21.6547, Lng: 103.2169, South: 20.8926, West: 102.1438, North: 22.548, East: 103.5988},
	{Code: 12, Name: "Tỉnh Lai Châu", Lat: 22.2922, Lng: 103.1799, South: 21.686, West: 102.3205, North: 22.814, East: 103.9859},
	{Code: 14, Name: "Tỉnh Sơn La", Lat: 21.2277, Lng: 104.1576, South: 20.573, West: 103.2122, North: 22.0308, East: 105.0254},
	{Code: 15, Name: "Tỉnh Lào Cai", Lat: 22.3069, Lng: 104.183, South: 21.3259, West: 103.5294, North: 22.845, East: 105.1004},
	{Code: 19, Name: "Tỉnh Thái Nguyên", Lat: 21.9611, Lng: 105.8441, South: 21.3264, West: 105.4311, North: 22.7414, East: 106.247},
	{Code: 20, Name: "Tỉnh Lạng Sơn", Lat: 21.8488, Lng: 106.6141, South: 21.3251, West: 106.0953, North: 22.4615, East: 107.3641},
	{Code: 22, Name: "Tỉnh Quảng Ninh", Lat: 20.9563, Lng: 107.0763, South: 20.4645, West: 106.4391, North: 21.6636, East: 108.2087},
	{Code: 25, Name: "Tỉnh Phú Thọ", Lat: 21.3008, Lng: 105.135, South: 20.3053, West: 104.8142, North: 21.7197, East: 105.8572},
	{Code: 31, Name: "Thành phố Hải Phòng", Lat: 20.8831, Lng: 106.679, South: 19.9177, West: 106.1242, North: 21.237, East: 107.9519},
	{Code: 33, Name: "Tỉnh Hưng Yên", Lat: 20.6066, Lng: 106.2843, South: 20.0771, West: 105.8953, North: 21.007, East: 107.1193},
	{Code: 37, Name: "Tỉnh Ninh Bình", Lat: 20.2867, Lng: 106.1, South: 19.411, West: 105.5416, North: 20.7044, East: 106.9524},
	{Code: 38, Name: "Tỉnh Thanh Hóa", Lat: 19.9782, Lng: 105.4816, South: 19.1874, West: 104.3759, North: 20.67, East: 106.2691},
	{Code: 40, Name: "Tỉnh Nghệ An", Lat: 19.1976, Lng: 105.0607, South: 18.5521, West: 103.8745, North: 20.0024, East: 106.2646},
	{Code: 42, Name: "Tỉnh Hà Tĩnh", Lat: 18.3505, Lng: 105.7623, South: 17.9096, West: 105.1034, North: 18.767, East: 106.7538},
	{Code: 44, Name: "Tỉnh Quảng Trị", Lat: 17.2167, Lng: 106.9548, South: 16.3021, West: 105.6076, North: 18.1316, East: 107.692},
	{Code: 46, Name: "Thành phố Huế", Lat: 16.4639, Lng: 107.5863, South: 15.9949, West: 107.0343, North: 17.111, East: 108.4806},
	{Code: 48, Name: "Thành phố Đà Nẵng", Lat: 16.0685, Lng: 108.224, South: 14.9513, West: 107.2109, North: 16.3341, East: 109.0236},
	{Code: 51, Name: "Tỉnh Quảng Ngãi", Lat: 14.9954, Lng: 108.6917, South: 13.9216, West: 107.3353, North: 15.7973, East: 109.4633},
	{Code: 52, Name: "Tỉnh Gia Lai", Lat: 14.0201, Lng: 108.6355, South: 12.9961, West: 107.4508, North: 14.7033, East: 109.585},
	{Code: 56, Name: "Tỉnh Khánh Hòa", Lat: 12.2981, Lng: 108.995, South: 9.698, West: 106.395, North: 14.8981, East: 111.5951},
	{Code: 66, Name: "Tỉnh Đắk Lắk", Lat: 12.8742, Lng: 108.7979, South: 12.1605, West: 107.4842, North: 13.6954, East: 109.6666},
	{Code: 68, Name: "Tỉnh Lâm Đồng", Lat: 11.6615, Lng: 108.1335, South: 9.378, West: 107.2061, North: 12.8126, East: 109.402},
	{Code: 75, Name: "Tỉnh Đồng Nai", Lat: 11.4388, Lng: 107.1118, South: 10.5792, West: 106.4116, North: 12.2985, East: 107.5779},
	{Code: 79, Name: "Thành phố Hồ Chí Minh", Lat: 10.7737, Lng: 106.7166, South: 8.3456, West: 105.9769, North: 11.5016, East: 108.4312},
	{Code: 80, Name: "Tỉnh Tây Ninh", Lat: 11.0801, Lng: 106.2611, South: 10.3948, West: 105.5003, North: 11.7829, East: 106.7473},
	{Code: 82, Name: "Tỉnh Đồng Tháp", Lat: 10.4252, Lng: 105.9271, South: 10.1347, West: 105.1855, North: 10.9735, East: 106.9934},
	{Code: 86, Name: "Tỉnh Vĩnh Long", Lat: 10.0468, Lng: 106.2947, South: 9.08, West: 105.6818, North: 10.3404, East: 106.9934},
	{Code: 91, Name: "Tỉnh An Giang", Lat: 10.3189, Lng: 105.0432, South: 8.9945, West: 103.0522, North: 10.9623, East: 105.5757},
	{Code: 92, Name: "Thành phố Cần Thơ", Lat: 10.0362, Lng: 105.7873, South: 8.7025, West: 105.2256, North: 10.3253, East: 106.4539},
	{Code: 96, Name: "Tỉnh Cà Mau", Lat: 9.018, Lng: 105.087, South: 8.179, West: 103.4821, North: 9.6376, East: 106.0554},
}

// ProvincesWithoutBox are the provinces the extract had no boundary for, with
// the reason. Written down rather than omitted: a place in one of these cannot
// be projected, and the next person needs to know it is a missing boundary
// rather than a missing place.
var ProvincesWithoutBox = map[int16]string{
	24: "Nominatim không có ranh giới hành chính cấp tỉnh",
}

// ProvinceDestinationID is the catalogue destination for a province.
//
// Prefixed rather than slugged from the name, because the curated destinations
// already hold `d-tphcm`, and two rows claiming to be Ho Chi Minh City would
// have to be told apart by whoever reads them next.
func ProvinceDestinationID(code int16) string {
	return fmt.Sprintf("d-tinh-%d", code)
}

// provinceSortBase puts province destinations after the curated ones, which
// occupy the low numbers.
const provinceSortBase = 1000

// SeedProvinceDestinations writes one destination per province.
//
// Idempotent, and it does not overwrite a curated row: a destination somebody
// wrote a description for is not replaced by a province outline that happens
// to share its id.
func SeedProvinceDestinations(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	batch := &pgx.Batch{}
	for i, p := range ProvinceBoxes {
		batch.Queue(`
			INSERT INTO destinations (id, name, province, lat, lng,
			  bbox_south, bbox_west, bbox_north, bbox_east, sort_order)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT (id) DO UPDATE SET
			  name = EXCLUDED.name,
			  province = EXCLUDED.province,
			  lat = EXCLUDED.lat, lng = EXCLUDED.lng,
			  bbox_south = EXCLUDED.bbox_south, bbox_west = EXCLUDED.bbox_west,
			  bbox_north = EXCLUDED.bbox_north, bbox_east = EXCLUDED.bbox_east`,
			ProvinceDestinationID(p.Code), p.Name, p.Name, p.Lat, p.Lng,
			p.South, p.West, p.North, p.East, provinceSortBase+i)
	}
	results := pool.SendBatch(ctx, batch)
	var written int64
	for range ProvinceBoxes {
		tag, err := results.Exec()
		if err != nil {
			results.Close()
			return written, err
		}
		written += tag.RowsAffected()
	}
	return written, results.Close()
}
