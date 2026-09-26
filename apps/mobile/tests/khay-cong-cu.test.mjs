// The chat tray's words for a pair versus a group (QA couple 23/09, §11).
import assert from "node:assert/strict";
import test from "node:test";

import { chuKhay } from "../dist-test/rudi/chat/khay-cong-cu.js";

test("nhắn riêng hai người: khay mở tờ giấy, không nói «hội»", () => {
  const c = chuKhay(true);
  assert.deepEqual(c.congCuHen, { label: "Tờ giấy", dich: "to-giay" });
  for (const cau of [c.tieuDePoll, c.goiYPoll, c.nhanPlan]) assert.doesNotMatch(cau, /hội/i, cau);
});

test("nhóm: giữ tờ hẹn AI và chữ của hội", () => {
  const c = chuKhay(false);
  assert.deepEqual(c.congCuHen, { label: "Tờ hẹn", dich: "plan" });
  assert.match(c.nhanPlan, /hội/);
});
