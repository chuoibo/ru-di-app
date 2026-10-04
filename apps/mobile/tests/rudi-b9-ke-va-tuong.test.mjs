/**
 * B9, the last five of QA #663 kept for this batch: the empty shelf has one
 * way in (UI-153), the album says the trip's days (UI-103), a background
 * refresh keeps the wall's opened pages (UI-155), the story viewer's halves
 * are buttons and its question takes focus (UI-101), and the profile row is
 * named after the screen it opens (UI-162).
 *
 * The pure rules run here; the screens are read as source, which proves the
 * wiring exists, not how it renders (the harness and the screenshots do).
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { loiVaoTrangDau } from "../dist-test/rudi/diary/trang-dau.js";
import { khoangNgayChuyen, thangNamVN } from "../dist-test/rudi/ngay-viet.js";
import { lamMoiDauTuong } from "../dist-test/rudi/tuong/social-v2.js";

const nguon = (p) => readFileSync(new URL(`../src/rudi/${p}`, import.meta.url), "utf8");
const keo = (id, starts_on, ends_on, title = `Kèo ${id}`) => ({ id, title, starts_on, ends_on });

test("kệ trống: lối vào là kèo đã qua gần nhất, không có thì danh sách kèo (UI-153)", () => {
  const today = "2026-10-04";
  assert.deepEqual(loiVaoTrangDau([keo("a", "2026-09-01", "2026-09-02"), keo("b", "2026-09-28", "2026-09-29", "Đà Lạt"), keo("c", "2026-10-10", "2026-10-11")], today), { kieu: "keo", id: "b", ten: "Đà Lạt" });
  assert.deepEqual(loiVaoTrangDau([keo("c", "2026-10-10", "2026-10-11")], today), { kieu: "plan" }, "chỉ có kèo sắp tới: chưa có gì để giữ");
  assert.deepEqual(loiVaoTrangDau([], today), { kieu: "plan" });
  assert.equal(loiVaoTrangDau([keo("d", "2026-09-01", "2026-09-01", "  ")], today).ten, "cuộc đi vừa qua", "tên trống không thành nút trống");

  const wall = nguon("diary/Wall.tsx");
  assert.match(wall, /<KeTrong \/>/, "kệ của chính chủ dùng trạng thái rỗng có hành động");
  assert.match(wall, /illustration=\{<Canh id="chua-co-ky-niem"/);
  assert.match(wall, /router\.push\(`\/outings\/\$\{loi\.id\}`/);
  assert.doesNotMatch(wall, /Mở cuộc đi đã qua để giữ khoảnh khắc đầu tiên\./, "câu cũ không có lối đi");
});

test("album ghi ngày của chuyến, năm chỉ khi khác năm nay (UI-103)", () => {
  assert.equal(khoangNgayChuyen("2026-09-28", "2026-09-29", 2026), "28 - 29/09");
  assert.equal(khoangNgayChuyen("2026-09-30", "2026-10-02", 2026), "30/09 - 02/10");
  assert.equal(khoangNgayChuyen("2026-09-28", "2026-09-28", 2026), "28/09");
  assert.equal(khoangNgayChuyen("2025-09-28", "2025-09-29", 2026), "28 - 29/09/2025");
  assert.equal(khoangNgayChuyen("2025-12-30", "2026-01-02", 2026), "30/12/2025 - 02/01/2026");
  assert.equal(khoangNgayChuyen("2026-09-28", "", 2026), "28/09", "thiếu ngày cuối: một ngày");
  assert.equal(khoangNgayChuyen("không phải ngày", "2026-09-29", 2026), "", "không đọc được thì để nhãn năm của máy chủ");
  assert.equal(thangNamVN("2026-09-28T10:00:00+07:00"), "Tháng 9, 2026", "nhãn tháng trên kệ viết tay, không theo Intl của máy");
  assert.doesNotMatch(nguon("diary/Wall.tsx"), /toLocaleDateString/);

  const album = nguon("screens/ky-niem/AlbumLive.tsx");
  assert.match(album, /cauKhoang\(a, ngayChuyen\[a\.outing_id\]\)/, "kệ");
  assert.match(album, /cauKhoang\(a, ngayChuyen\[outingId\]\)/, "đầu album");
});

test("làm mới nền giữ các trang đã mở và con trỏ của chúng (UI-155)", () => {
  const b = (n) => ({ id: `p${n}` });
  const ds = (from, to) => Array.from({ length: to - from + 1 }, (_, i) => b(from + i));
  const daMo = { bai: ds(1, 23), conTro: "sau-23", conNua: false };

  // Nothing new: the first page repeats, the opened third page stays.
  let ra = lamMoiDauTuong(daMo, { posts: ds(1, 20), next_cursor: "sau-20", has_more: true });
  assert.equal(ra.bai.length, 23);
  assert.equal(ra.conTro, "sau-23");
  assert.equal(ra.conNua, false);

  // One new post on top: it leads, nothing is lost, the old cursor still reaches past p23.
  ra = lamMoiDauTuong(daMo, { posts: [b(0), ...ds(1, 19)], next_cursor: "sau-19", has_more: true });
  assert.deepEqual(ra.bai.map((x) => x.id), ["p0", ...ds(1, 23).map((x) => x.id)]);
  assert.equal(ra.conTro, "sau-23");

  // p5 deleted: it leaves; what lies below the new page stays.
  ra = lamMoiDauTuong(daMo, { posts: [...ds(1, 4), ...ds(6, 21)], next_cursor: "sau-21", has_more: true });
  assert.equal(ra.bai.some((x) => x.id === "p5"), false);
  assert.equal(ra.bai.length, 22);

  // Only the first page was open: the fresh page replaces it with its own cursor.
  ra = lamMoiDauTuong({ bai: ds(1, 20), conTro: "sau-20", conNua: true }, { posts: ds(1, 20), next_cursor: "sau-20b", has_more: true });
  assert.equal(ra.conTro, "sau-20b");

  // A whole new page arrived: the reader's old posts stay below it.
  ra = lamMoiDauTuong(daMo, { posts: ds(100, 119), next_cursor: "sau-119", has_more: true });
  assert.equal(ra.bai.length, 43);

  const man = nguon("screens/nguoi/HoSoNguoiScreen.tsx");
  assert.match(man, /lamMoiDauTuong\(current, page\)/);
  assert.match(man, /quiet \? tuongLanDoc\.current : \+\+tuongLanDoc\.current/, "làm mới lặng không huỷ trang đang mở thêm");
});

test("trình xem story: hai vùng chạm là nút, câu hỏi xoá nhận focus và trả lại (UI-101)", () => {
  const xem = nguon("screens/story/XemStoryScreen.tsx");
  assert.match(xem, /accessibilityLabel="Story trước" accessibilityRole="button"/);
  assert.match(xem, /accessibilityLabel="Story tiếp theo" accessibilityRole="button"/);
  assert.match(xem, /ref=\{nutGiu\}/);
  assert.match(xem, /ref=\{nutXoaStory\}/);
  assert.match(xem, /setAccessibilityFocus/);
  assert.match(xem, /nutTron: \{ width: 48, height: 48/);
});

test("hàng menu mang tên màn nó mở (UI-162)", () => {
  const p = nguon("screens/Profile.tsx");
  assert.match(p, /subtitle="Sổ huy hiệu và các ngã rẽ của bạn"\n\s+title="Hành trình"/);
  assert.doesNotMatch(p, /Cấp và huy hiệu/);
});

test("chia bill mở từ màn kèo đọc id kèo trong đường dẫn; bản nháp tách theo kèo (ADR-0054)", () => {
  const review = readFileSync(new URL("../app/smart-split/[id]/review.tsx", import.meta.url), "utf8");
  assert.match(review, /outingId=\{outingId\}/);
  assert.match(review, /UUID\.test\(params\.id\)/, "«moi» và chuỗi lạ không thành id kèo");
  const bill = nguon("screens/chia-bill/ChiaBillLive.tsx");
  assert.match(bill, /khoaNhapBill\(contextId, outingId\)/, "một bản nháp cho mỗi nhóm và kèo");
  assert.match(bill, /outingId \}\);/, "ghiVaoSo nhận kèo");
  assert.doesNotMatch(nguon("doc-live.ts"), /theo ngày của chuyến/);
});
