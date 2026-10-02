/**
 * Khám phá › Cộng đồng after the owner's choice of 02/10: the feed's modes
 * are five text tabs — the three feeds plus «Đã lưu» and «Bài của tôi», which
 * used to hide in the settings sheet — and the sheet keeps only what is not a
 * list of posts. Each new tab has an empty state that says what goes there.
 *
 * Read from the source: the screen needs the router, the session and the
 * stream, and is not rendered here. Does not prove: the row on a device (that
 * is the probe `kiem-ux/thanh-tab-5.mjs`) or that the words read well (the
 * screenshots).
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const man = readFileSync(new URL("../src/rudi/community/CommunityScreen.tsx", import.meta.url), "utf8");
// The settings sheet, from its opening tag to its close.
const sheet = man.slice(man.indexOf('accessibilityLabel="Bảng tin của bạn"'), man.indexOf("</Sheet>", man.indexOf('accessibilityLabel="Bảng tin của bạn"')));

test("trên tab, các chế độ bảng tin là một hàng chữ-tab năm mục theo đúng thứ tự", () => {
  assert.match(man, /<HangChuTab\b/);
  const nhan = [...man.slice(man.indexOf("const CHE_DO")).matchAll(/nhan: "([^"]+)"/g)].slice(0, 5).map((m) => m[1]);
  assert.deepEqual(nhan, ["Dành cho bạn", "Đang theo dõi", "Thịnh hành", "Đã lưu", "Bài của tôi"]);
  // The chips of the first cut are gone: a chip narrows a list, a tab picks it.
  assert.doesNotMatch(man, /<Chip key=\{t\.mode\} vaiTab/);
});

test("menu cài đặt chỉ giữ cái không phải một danh sách bài", () => {
  assert.ok(sheet.length > 0, "không thấy sheet «Bảng tin của bạn»");
  for (const daRa of ['label="Bài đã lưu"', 'label="Bài của tôi · Trạng thái duyệt"', 'label="Thông báo"']) {
    assert.ok(!sheet.includes(daRa), `${daRa} vẫn còn trong menu cài đặt`);
  }
  for (const con of ['label="Bài đã ẩn"', 'label="Điều mình muốn giữ"', "Bật cá nhân hóa", 'label="Xóa lịch sử đề xuất"']) {
    assert.ok(sheet.includes(con), `${con} phải còn trong menu cài đặt`);
  }
});

test("«Đã lưu» và «Bài của tôi» rỗng thì nói ở đó sẽ có gì, «Bài của tôi» mời viết bài", () => {
  assert.match(man, /mode === "saved" \?[\s\S]{0,400}Chưa lưu bài nào[\s\S]{0,300}Chạm dấu lưu ở cuối một bài để đọc lại sau\./);
  assert.match(man, /mode === "mine" \?[\s\S]{0,600}Chưa kể chuyện nào<[\s\S]{0,400}label="Viết bài" onPress=\{\(\) => router\.push\("\/community\/new" as never\)\}/);
});

test("lời xin cá nhân hoá chỉ ở «Dành cho bạn», nơi nó đổi được điều gì", () => {
  assert.match(man, /prefs && !prefs\.asked && !topic && mode === "for_you" \? <View style=\{\[styles\.consent/);
});
