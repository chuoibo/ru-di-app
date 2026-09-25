/* Bốn lỗi QC 24/09 ở chat và màn người, sửa ở lát S5 (ADR-0037).
 *
 * Chạy từ apps/mobile:
 *     node --test tests/loi-qc-nguoi-chat.test.mjs
 *
 * Phép ghim nguồn: chứng minh hình dạng mã của từng bản sửa còn đó. Không
 * chứng minh máy chủ nhận lời mời hay trình duyệt hết cảnh báo. Hai điều đó
 * đã đo trên stack thật: «Kết bạn» gửi lời mời thật; canary bản cũ của
 * HangNguoi ra 2 cảnh báo lồng nút ở /friends, bản mới ra 0.
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const doc = (tep) => readFileSync(new URL(`../src/rudi/${tep}`, import.meta.url), "utf8");

test("B8: hàng người không lồng nút; hành động đứng cạnh, ngoài vùng bấm vào hồ sơ", () => {
  const nguon = doc("screens/friends/HangNguoi.tsx");
  const moNut = nguon.indexOf("<Pressable");
  const dongNut = nguon.indexOf("</Pressable>");
  const hanhDong = nguon.indexOf("{duoi ?");
  assert.ok(moNut >= 0 && dongNut > moNut, "không thấy vùng bấm vào hồ sơ");
  assert.ok(hanhDong > dongNut, "hành động nằm trong vùng bấm: nút lồng nút");
});

test("B3: người cùng nhóm có «Kết bạn» thật, gọi guiLoiMoi, và không mời người vừa bị chặn", () => {
  const nguon = doc("screens/nguoi/HoSoNguoiScreen.tsx");
  const nhanh = nguon.slice(nguon.indexOf('relation === "groupmate" ?'));
  assert.match(nhanh, /Kết bạn để nhắn riêng\./, "câu flow 45 kiểm đã mất");
  assert.match(nhanh, /\{!daChan \?/, "vẫn mời kết bạn người vừa bị chặn");
  assert.match(nhanh, /label="Kết bạn"/, "không có nút «Kết bạn»");
  assert.match(nguon, /await guiLoiMoi\(personId, phien\.person_id, attemptFor\(/, "nút không gọi POST /friends/requests với một attempt");
});

test("B2: thanh ghim nằm trên một dải nền đặc có nét kẻ dưới", () => {
  const nguon = doc("screens/chat/GroupChatLive.tsx");
  const dai = nguon.slice(nguon.indexOf('testID="day-ghim"') - 200, nguon.indexOf('testID="day-ghim"'));
  assert.match(dai, /styles\.dayGhim, \{ backgroundColor: colors\.ground, borderBottomColor: colors\.line \}/);
  assert.match(nguon, /dayGhim: \{ borderBottomWidth: StyleSheet\.hairlineWidth/);
});

test("mực người đi qua AvatarNguoi: avatar sống mang vòng mực của đúng người", () => {
  const nguon = doc("ui/AvatarNguoi.tsx");
  assert.match(nguon, /<Avatar \{\.\.\.rest\}[^>]*personId=\{personId\}/, "AvatarNguoi nuốt personId: mọi avatar sống mất mực riêng");
});
