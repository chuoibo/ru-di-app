/* Đợt 2 (20/10/2026): trang quán có chỉ mục, Khám phá phân trang, điểm đến có ảnh bìa.
 * Source-level guards for what the host tests cannot see: a section that must hide
 * when empty, a sentence that must not come back, a credit said once. */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const doc = (p) => readFileSync(new URL(`../src/rudi/${p}`, import.meta.url), "utf8");
const chiTiet = doc("screens/explore/PlaceDetailLive.tsx");
const kham = doc("screens/explore/ExploreLive.tsx");
const diemDen = doc("screens/explore/DiemDenScreen.tsx");
const gon = doc("kham-pha/chi-tiet-gon.ts");

test("trang quán: mục rỗng ẩn hẳn, không còn «Chưa có giờ mở cửa» hay khối «Vì sao hợp nhóm?» rỗng", () => {
  assert.doesNotMatch(chiTiet, /Chưa có điểm theo gu nhóm/);
  assert.doesNotMatch(chiTiet, /hopNhomGoiY/);
  assert.doesNotMatch(chiTiet, /cauMoCua\(/, "dòng «Chưa có giờ mở cửa» không còn đi từ cauMoCua");
  // «Vì sao hợp nhóm?» only inside the branch where the model scored the place.
  assert.match(chiTiet, /\{coMatch && place\.match !== null \? \(\s*<View style=\{styles\.khoi\}>\s*<SectionHeader title="Vì sao hợp nhóm\?" \/>/);
  for (const tieuDe of ["Món nên thử", "Không khí", "Hợp với ai", "Giờ", "Lưu ý", "Tại sao đáng đến"]) {
    assert.ok(chiTiet.includes(`"${tieuDe}"`), `thiếu mục ${tieuDe}`);
  }
  // Every section is pushed under a condition on its data.
  assert.match(chiTiet, /if \(hl\.monPhaiThu\.length > 0\)/);
  assert.match(chiTiet, /if \(hl\.khongKhi !== null\)/);
});

test("trang quán (21/10): giá nói MỘT lần, trong khối số liệu chính; không còn mục «Giá» riêng", () => {
  assert.doesNotMatch(chiTiet, /muc\("Giá"/, "mục «Giá» riêng đã gộp vào khối số liệu chính");
  assert.doesNotMatch(chiTiet, /cauKhoangGia|cauGia\(/, "trang không tự dựng câu giá lần thứ hai");
  assert.match(chiTiet, /dongChinh\(place, hl, new Date\(\)\)/);
  assert.match(chiTiet, /muc\.icon === "star" \|\| muc\.icon === "navigate-outline"/, "hàng sao không mang ví tiền");
  assert.doesNotMatch(chiTiet, /Giờ đẹp nhất: \$\{/, "giờ đẹp nhất chỉ ở khối số liệu chính");
  assert.match(gon, /"Ước khoảng"/);
});

test("trang quán: chip chỉ là khí chất, khử trùng không phân biệt hoa thường/dấu; «Đọc thêm» chỉ khi chữ thật sự bị cắt", () => {
  assert.match(chiTiet, /const nhan = nhanKhiChat\(place, hl\)/);
  assert.doesNotMatch(chiTiet, /new Set\(nhan\)/);
  assert.doesNotMatch(chiTiet, /dau\.length > 150/, "không đoán theo độ dài");
  assert.match(chiTiet, /const cat = cao\.du > cao\.gon \+ 1;/);
  assert.match(chiTiet, /\{cat \|\| mo \? \(/);
});

test("ảnh trang quán: MỘT bìa lật trang, mỗi trang đúng một dòng ghi công; câu nguồn chung nằm xa bìa", () => {
  assert.doesNotMatch(chiTiet, /<DaiAnh anh=/, "không còn dải ảnh lớn thứ hai dưới bìa");
  assert.match(chiTiet, /<ScrollView horizontal pagingEnabled/);
  assert.match(chiTiet, /`\$\{i \+ 1\}\/\$\{anh\.length\}`/, "đếm «1/3» trên ảnh");
  assert.match(chiTiet, /const TI_LE_BIA = 16 \/ 9;/, "bìa dưới nửa màn 844");
  assert.match(chiTiet, /cauAnh === null \|\| trangAnh\.length === 0 \? null/);
  assert.match(chiTiet, /source: CAU_ANH_BAI_DANG/, "ảnh bài đăng không có tác giả vẫn mang một dòng nguồn");
});

test("không nhãn «Minh hoạ» ở ba màn Khám phá, điểm đến, trang quán", () => {
  for (const src of [chiTiet, kham, diemDen]) assert.doesNotMatch(src, /minh\s*ho[aạ]/i);
});

test("Khám phá: cuộn vô hạn theo next_offset, cache theo điểm đến, lý do tải riêng", () => {
  assert.match(kham, /onGanCuoi=\{taiThem\}/);
  assert.match(kham, /offset: tiepHienTai/);
  assert.match(kham, /boNhoDanhMuc/);
  assert.match(kham, /docLyDo\(can/);
  assert.doesNotMatch(kham, /HANG_MOI_LUOT|soHang/, "không còn cắt 20 dòng phía máy khách");
});

test("điểm đến: ảnh thật đi qua MediaSlot/AnhCoGhiCong, thiếu ảnh dùng tranh vẽ không nhãn", () => {
  assert.match(diemDen, /d\.anhBia !== null/);
  assert.match(diemDen, /nguon=\{\{ loai: "danh-muc", anh: d\.anhBia \}\}/);
  assert.doesNotMatch(diemDen, /<Image\b/, "không tự dựng Image");
});

test("điểm đến (21/10): một nét vẽ cho mọi hàng, phụ đề không bị cắt, chỗ cho hai dòng nên hàng cao bằng nhau; ảnh thật chỉ ở thẻ nổi bật", () => {
  assert.doesNotMatch(diemDen, /CANH_DIEM_DEN/, "bảng cảnh cũ (Hội An cho Hà Nội) đã bỏ");
  assert.match(diemDen, /<ThanhPhoNet id=\{d\.id\} width=\{RONG_THU\} \/>/);
  assert.match(diemDen, /typography\.caption, \{ color: colors\.inkSoft \}, !noiBat && \{ minHeight: typography\.caption\.lineHeight \* 2 \}/);
  assert.doesNotMatch(diemDen, /numberOfLines=\{noiBat \? undefined : 1\} style=\{\[typography\.caption/, "phụ đề bị cắt «…» là lỗi review 02c94551");
  assert.match(diemDen, /thuHang: \{ width: RONG_THU, height: 64 \}/);
  assert.doesNotMatch(diemDen, /numberOfLines=\{noiBat \? undefined : 1\}/, "tên tỉnh bị cắt «…» ở 320 dp (review 52b4c7d6)");
  assert.match(diemDen, /canhDiemDen\(d\.id\)/);
});

test("Khám phá (21/10): sân khấu thành phố dùng ảnh bìa thật khi có, tranh vẽ chỉ của đúng vùng", () => {
  assert.doesNotMatch(kham, /name="scene-hills"/, "không còn hồ Đà Lạt cố định cho mọi thành phố");
  assert.match(kham, /docBiaDiemDen\(idDiemDen\)/);
  assert.match(kham, /<MediaSlot alt=\{`Ảnh \$\{ten\}`\}/);
  assert.match(kham, /phanCanhDanhSach\(/);
});
