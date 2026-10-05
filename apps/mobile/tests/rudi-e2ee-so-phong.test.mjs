/* Chat v2: a room's durable record on the device and what it draws
 * (src/rudi/chat/e2ee/so-phong.ts, ADR-0057 §4.4).
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/rudi-e2ee-so-phong.test.mjs
 *
 * A replay that wrote a record twice draws it once; messages go in lane order
 * whatever order they were written; an own send is a pending row until the
 * lane holds it, a failed row after a failure, and gone once given up.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { canGuiLai, dungPhong } from "../dist-test/rudi/chat/e2ee/so-phong.js";
import { danhSach } from "../dist-test/rudi/chat/e2ee/so-tin.js";

const AN = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const BINH = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
let n = 0;
const id = () => `cccccccc-cccc-4ccc-8ccc-${String(++n).padStart(12, "0")}`;
const nhan = (actor, operation, logical = id()) => ({ kind: "application", actor_id: actor, device_id: actor, logical_send_id: logical, operation });
const text = (body) => ({ type: "text", body });

test("bản ghi phát lại vẽ một lần, tin theo thứ tự trên làn, tin của mình nằm đúng chỗ", () => {
  const b1 = nhan(BINH, text("một"));
  const b3 = nhan(BINH, text("ba"));
  const mine = nhan(AN, text("hai"));
  const r = dungPhong([
    { t: "cho", r: mine, luc: 1 },
    { t: "tin", seq: 3, r: b3 },
    { t: "tin", seq: 1, r: b1 },
    { t: "tin", seq: 1, r: b1 }, // the same page written again after a crash
    { t: "da-gui", id: mine.logical_send_id, seq: 2 },
    { t: "da-gui", id: mine.logical_send_id, seq: 2 },
  ]);
  assert.deepEqual(danhSach(r.so).map((t) => t.body), ["một", "hai", "ba"]);
  assert.deepEqual(r.cho, []);
});

test("một reaction phát lại không bật rồi tắt", () => {
  const goc = nhan(BINH, text("đi không"));
  const tim = nhan(BINH, { type: "reaction", message_id: goc.logical_send_id, emoji: "❤️" });
  const r = dungPhong([
    { t: "tin", seq: 1, r: goc },
    { t: "tin", seq: 2, r: tim },
    { t: "tin", seq: 2, r: tim },
  ]);
  assert.deepEqual(Object.keys(danhSach(r.so)[0].reactions), ["❤️"]);
});

test("hàng chờ: đang gửi → hỏng → thử lại → lên làn; bỏ thì biến mất; chỉ hàng tạo tin mới được vẽ", () => {
  const a = nhan(AN, text("đang đi"));
  const b = nhan(AN, text("hỏng rồi"));
  const c = nhan(AN, text("bỏ đi"));
  const sua = nhan(AN, { type: "edit", message_id: a.logical_send_id, body: "sửa" });
  let ban = [
    { t: "cho", r: a, luc: 1 },
    { t: "cho", r: b, luc: 2 },
    { t: "hong", id: b.logical_send_id, loi: "Mất mạng", thuLai: true },
    { t: "cho", r: c, luc: 3 },
    { t: "hong", id: c.logical_send_id, loi: "Không có quyền", thuLai: false },
    { t: "bo", id: c.logical_send_id },
    { t: "cho", r: sua, luc: 4 },
  ];
  let r = dungPhong(ban);
  assert.deepEqual(r.cho.map((x) => [x.r.operation.body, x.hong, x.loi]), [["đang đi", false, null], ["hỏng rồi", true, "Mất mạng"]]);
  assert.deepEqual(r.dangDi.map((x) => x.r.operation.type), ["text", "text", "edit"]);
  // Only what is on its way and not failed goes again on its own.
  assert.deepEqual(canGuiLai(ban).map((x) => x.r.operation.body ?? x.r.operation.type), ["đang đi", "sửa"]);
  ban = [...ban, { t: "thu", id: b.logical_send_id }, { t: "da-gui", id: b.logical_send_id, seq: 5 }];
  r = dungPhong(ban);
  assert.deepEqual(danhSach(r.so).map((t) => t.body), ["hỏng rồi"]);
  assert.deepEqual(r.cho.map((x) => x.r.operation.body), ["đang đi"]);
});

test("tin không mở được chỉ được đếm, không thành hàng", () => {
  const r = dungPhong([{ t: "khong-mo", seq: 1, actor: BINH }, { t: "tin", seq: 2, r: nhan(BINH, text("sau đó")) }]);
  assert.equal(r.khongMo, 1);
  assert.deepEqual(danhSach(r.so).map((t) => t.body), ["sau đó"]);
});

test("một thành viên dùng lại logical id của tin mình không chiếm chỗ tin mình", () => {
  const mine = nhan(AN, text("của An"));
  // Bình read An's id off the lane and sends something else under it.
  const copy = nhan(BINH, text("Bình giả"), mine.logical_send_id);
  const r = dungPhong([
    { t: "cho", r: mine, luc: 1 },
    { t: "da-gui", id: mine.logical_send_id, seq: 5 },
    { t: "tin", seq: 6, r: copy },
  ]);
  assert.deepEqual(danhSach(r.so).map((t) => [t.authorId, t.body]), [[AN, "của An"]]);
});

test("màn chat nhận tin mới nhất trước, mỗi tin mang giờ của làn", async () => {
  const { moiTruoc } = await import("../dist-test/rudi/chat/e2ee/so-tin.js");
  const mine = nhan(AN, text("An hỏi"));
  const reply = nhan(BINH, text("Bình đáp"));
  const r = dungPhong([
    { t: "cho", r: mine, luc: 1 },
    { t: "da-gui", id: mine.logical_send_id, seq: 3, at: "2026-10-05T16:21:00Z" },
    { t: "tin", seq: 4, r: reply, at: "2026-10-05T16:21:05Z" },
  ]);
  assert.deepEqual(moiTruoc(r.so).map((t) => [t.body, t.at]), [["Bình đáp", "2026-10-05T16:21:05Z"], ["An hỏi", "2026-10-05T16:21:00Z"]]);
});
