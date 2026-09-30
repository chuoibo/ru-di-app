package tuvung

import "mobile/services/core/internal/domain/catalog"

// KhongRo is the value a model-filled slot answers when the text does not
// say («không rõ»). It is never an id of a vocabulary: rag/nap.KhongRo and
// vectordb.KhongRo are the same string, and a test holds it.
const KhongRo = "khong_ro"

// DanhMuc is the closed list of what a place is for (owner, 2026-09-29):
// the ingest's classification pass (rag/nap) assigns a place every one of
// these it fits, from the place's own text, by the definitions below. No phrase list: nothing scans text for a category, a
// model reads the definition and Go checks the answer is one of the ids.
// Finer than the app's four UI categories (catalog.Categories); NhomUI maps
// each id onto exactly one of them.
var DanhMuc = dung("danh_muc", []Muc{
	{ID: "quan_an", Nhan: "Quán ăn",
		DinhNghia: "nơi ăn bữa chính (cơm, bún, phở, hủ tiếu, lẩu, nướng, hải sản, ốc, quán chay, nhà hàng). Không gồm quán chủ yếu bán đồ ăn vặt/tráng miệng."},
	{ID: "an_vat", Nhan: "Ăn vặt & tráng miệng",
		DinhNghia: "món ăn giữa bữa (bánh bèo/tráng/hẹ…, chè, kem, tiệm bánh, xe đẩy ăn vặt). Không gồm nơi chủ yếu bán đồ uống (→ cafe)."},
	{ID: "cafe", Nhan: "Cà phê & đồ uống",
		DinhNghia: "nơi đến chủ yếu để uống (cà phê, trà, trà sữa, trà trái cây, nước ép, tiệm trà). Không gồm quán có cồn hoặc nhạc về đêm (→ bar_nhau)."},
	{ID: "bar_nhau", Nhan: "Bar, nhậu & về đêm",
		DinhNghia: "đồ uống có cồn hoặc sinh hoạt ban đêm (bar, pub, cocktail, hidden bar, lounge, phòng trà, quán nhậu). Không gồm quán ăn có bán bia nhưng khách đến chủ yếu để ăn."},
	{ID: "cho_am_thuc", Nhan: "Chợ & khu ẩm thực",
		DinhNghia: "một khu có nhiều hàng, không phải một quán (chợ, chợ đêm, chợ nổi, phố ẩm thực, food court). Không gồm một sạp riêng lẻ trong chợ (xếp theo món nó bán)."},
	{ID: "thien_nhien", Nhan: "Thiên nhiên & cảnh quan",
		DinhNghia: "đến để ngắm cảnh tự nhiên (biển, núi, thác, hồ, đảo, rừng, khu sinh thái, điểm ngắm hoàng hôn). Không gồm công viên nhân tạo (→ vui_choi)."},
	{ID: "van_hoa", Nhan: "Văn hoá, lịch sử, tâm linh",
		DinhNghia: "chùa, đền, nhà thờ, di tích, bảo tàng, triển lãm, làng nghề, công trình kiến trúc. Không gồm workshop làm nghề có tổ chức (→ trai_nghiem)."},
	{ID: "vui_choi", Nhan: "Vui chơi giải trí",
		DinhNghia: "nơi chơi tự do, có sẵn trò (khu vui chơi, công viên, game, bowling, trượt băng, karaoke, rạp, phố đi bộ). Không gồm hoạt động phải đặt hoặc có người hướng dẫn."},
	{ID: "trai_nghiem", Nhan: "Hoạt động & trải nghiệm",
		DinhNghia: "hoạt động có tổ chức, thường đặt trước, có thời lượng (workshop, lớp học, tour, du thuyền, chèo SUP, câu cá, hái trái cây). Không gồm nơi chỉ đến ngắm hoặc chơi tự do."},
	{ID: "luu_tru", Nhan: "Lưu trú & nghỉ dưỡng",
		DinhNghia: "có chỗ ngủ qua đêm (homestay, resort, glamping, khu cắm trại có chỗ ở, khách sạn). Không gồm quán/điểm chỉ ghé ban ngày."},
})

// DanhMucTheoNhomUI groups the DanhMuc ids under the app's four UI
// categories (catalog.Categories ids): every DanhMuc id is in exactly one
// group.
var DanhMucTheoNhomUI = map[string][]string{
	"quan-an-local": {"quan_an", "an_vat", "cho_am_thuc"},
	"cafe":          {"cafe"},
	"di-choi-dem":   {"bar_nhau"},
	"vui-choi":      {"thien_nhien", "van_hoa", "vui_choi", "trai_nghiem", "luu_tru"},
}

// NhomUI is the app UI category (a catalog.Categories id) DanhMuc id
// belongs to, or "" for khong_ro or any value outside the list.
func NhomUI(danhMuc string) string {
	for _, c := range catalog.Categories {
		for _, id := range DanhMucTheoNhomUI[c.ID] {
			if id == danhMuc {
				return c.ID
			}
		}
	}
	return ""
}

// DanhMucCuaNhomUI are the DanhMuc ids a UI category covers, in DanhMuc's
// declaration order (nil for an id that is not a UI category): the
// category filter a UI tab asks the index for.
func DanhMucCuaNhomUI(nhom string) []string {
	return DanhMuc.LocHopLe(DanhMucTheoNhomUI[nhom])
}
