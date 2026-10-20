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

test("trang quán: mục rỗng ẩn hẳn, không còn «Chưa có giờ mở cửa» hay khối «Vì sao hợp nhóm?» rỗng", () => {
  assert.doesNotMatch(chiTiet, /Chưa có điểm theo gu nhóm/);
  assert.doesNotMatch(chiTiet, /hopNhomGoiY/);
  assert.doesNotMatch(chiTiet, /cauMoCua\(/, "dòng «Chưa có giờ mở cửa» không còn đi từ cauMoCua");
  // «Vì sao hợp nhóm?» only inside the branch where the model scored the place.
  assert.match(chiTiet, /\{coMatch && place\.match !== null \? \(\s*<View style=\{styles\.khoi\}>\s*<SectionHeader title="Vì sao hợp nhóm\?" \/>/);
  for (const tieuDe of ["Món nên thử", "Không khí", "Hợp với ai", "Giá", "Giờ", "Lưu ý", "Tại sao đáng đến"]) {
    assert.ok(chiTiet.includes(`"${tieuDe}"`), `thiếu mục ${tieuDe}`);
  }
  // Every section is pushed under a condition on its data.
  assert.match(chiTiet, /if \(hl\.monPhaiThu\.length > 0\)/);
  assert.match(chiTiet, /if \(hl\.khongKhi !== null\)/);
  assert.match(chiTiet, /hl\.gia !== null \|\| hl\.mucGia !== null/);
  assert.match(chiTiet, /Ước khoảng/);
});

test("ghi công ảnh chỉ một dòng: câu chung của thư viện không in thêm khi đã có ảnh bìa", () => {
  assert.match(chiTiet, /cauAnh !== null && bia === null && conLai\.length > 0/);
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
