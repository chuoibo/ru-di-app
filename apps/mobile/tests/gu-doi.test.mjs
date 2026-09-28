// Taste in a couple's notebook, worded (ADR-0034 §2.1–2.2).
import assert from "node:assert/strict";
import test from "node:test";

import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { CAU_BAT_LAI_CHO_CHAT, batLaiChoChat, canBatLaiChoChat, cauBatGu, cauGu, noiDanhSach } from "../dist-test/rudi/to-giay/gu-doi.js";

const SHEET = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "..", "src", "rudi", "screens", "hai-nguoi", "GuHaiBan.tsx"), "utf8");

const gu = (over = {}) => ({ mine_shared: false, theirs_shared: false, theirs: [], common: [], ...over });

test("ngoài «Một đôi» không có câu nào", () => {
  assert.equal(cauGu(null, "Minh"), null);
  assert.equal(cauGu(undefined, "Minh"), null);
});

test("chưa ai bật: không nói gu của ai", () => {
  assert.deepEqual(cauGu(gu(), "Minh"), { chung: null, cuaHo: null, cuaToi: "Gu của bạn đang để riêng." });
});

test("người kia bật thì thấy gu họ bằng nhãn, bỏ id lạ", () => {
  const c = cauGu(gu({ theirs_shared: true, theirs: ["cafe", "outdoor", "tag-la"] }), "Minh");
  assert.equal(c.cuaHo, "Minh thích Cafe và Ngoài trời.");
  assert.equal(c.chung, null, "gu chung cần cả hai bật");
});

test("cả hai bật: gu chung, kể cả khi chưa trùng", () => {
  assert.equal(cauGu(gu({ mine_shared: true, theirs_shared: true, theirs: ["cafe"], common: ["cafe"] }), "Minh").chung, "Hai bạn cùng thích Cafe.");
  assert.match(cauGu(gu({ mine_shared: true, theirs_shared: true, theirs: ["game"] }), "Minh").chung, /chưa trùng gu/);
  assert.equal(cauGu(gu({ mine_shared: true }), "Minh").cuaToi, "Minh thấy gu của bạn; Nếp dùng nó khi phác tờ, và Rủ Đi AI dùng nó trong chat của hai bạn.");
});

test("nối danh sách kiểu tiếng Việt", () => {
  assert.equal(noiDanhSach([]), "");
  assert.equal(noiDanhSach(["A"]), "A");
  assert.equal(noiDanhSach(["A", "B", "C"]), "A, B và C");
});

// ADR-0048: the promise names the chat, a consent from before it was named is
// offered «Bật lại cho chat», and re-consent is off-then-on.
test("lời bật gu nói rõ Nếp và Rủ Đi AI trong chat của hai bạn", () => {
  for (const g of [gu(), gu({ theirs_shared: true })]) {
    const c = cauBatGu(g, "Linh");
    assert.match(c, /Nếp/);
    assert.match(c, /Rủ Đi AI trong chat của hai bạn/);
    assert.match(c, /^Bật thì Linh thấy gu của bạn/);
  }
  assert.match(cauBatGu(gu(), "Linh"), /Gu của Linh chỉ hiện khi chính họ bật/);
  assert.match(cauBatGu(gu(), " "), /^Bật thì Người ấy/);
  // The sheet words its promise from here, never the old «khi phác tờ» alone.
  assert.match(SHEET, /cauBatGu\(gu, tenNguoiKia\)/);
  assert.doesNotMatch(SHEET, /và Nếp dùng nó khi phác tờ\./);
});

test("«Bật lại cho chat» chỉ hiện khi gu của mình đang bật theo lời cũ", () => {
  const bat = gu({ mine_shared: true });
  assert.equal(canBatLaiChoChat(bat, { cua_toi: "can_bat_lai", nguoi_kia: false }), true);
  assert.equal(canBatLaiChoChat(bat, { cua_toi: "bat", nguoi_kia: true }), false, "đã bật theo lời mới");
  assert.equal(canBatLaiChoChat(gu(), { cua_toi: "can_bat_lai", nguoi_kia: false }), false, "gu đang tắt");
  assert.equal(canBatLaiChoChat(bat, null), false, "máy chủ không trả lời: không nút");
  assert.equal(canBatLaiChoChat(bat, undefined), false);
  assert.equal(canBatLaiChoChat(null, { cua_toi: "can_bat_lai", nguoi_kia: false }), false, "ngoài «Một đôi»");
  assert.match(CAU_BAT_LAI_CHO_CHAT, /Rủ Đi AI dùng gu của bạn trong chat/);
  // The sheet gates the button on exactly this, and names it.
  assert.match(SHEET, /canBatLaiChoChat\(gu, guChat\)/);
  assert.match(SHEET, /label="Bật lại cho chat"/);
});

test("bật lại = tắt rồi bật; bật chỉ chạy khi tắt đã xong, hỏng thì để tắt", async () => {
  const goi = [];
  const ok = await batLaiChoChat(async () => { goi.push("tat"); return true; }, async () => { goi.push("bat"); return true; });
  assert.equal(ok, true);
  assert.deepEqual(goi, ["tat", "bat"]);
  goi.length = 0;
  assert.equal(await batLaiChoChat(async () => { goi.push("tat"); return false; }, async () => { goi.push("bat"); return true; }), false);
  assert.deepEqual(goi, ["tat"], "tắt không xong thì không bật");
  assert.equal(await batLaiChoChat(async () => true, async () => false), false);
});
