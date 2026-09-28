// The chat tray's words for a pair versus a group (QA couple 23/09, §11;
// design 2026-09-28: a pair asks Rủ Đi AI with `hoi` only).
import assert from "node:assert/strict";
import test from "node:test";

import { chuKhay } from "../dist-test/rudi/chat/khay-cong-cu.js";

test("nhắn riêng hai người: khay mở tờ giấy, không nói «hội»", () => {
  const c = chuKhay(true);
  assert.deepEqual(c.congCuHen, { label: "Tờ giấy", dich: "to-giay" });
  // Every word the pair's tray can show, the AI entry included.
  const cacCau = [c.tieuDePoll, c.goiYPoll, c.nhanPlan, c.congCuHen.label, ...Object.values(c.hoiAiTrenKhay ?? {})];
  assert.ok(cacCau.length >= 6, "khay cặp phải có lối vào Rủ Đi AI");
  for (const cau of cacCau) assert.doesNotMatch(cau, /hội|nhóm/i, cau);
  // Its AI entry asks the one command a pair has, never a plan.
  assert.equal(c.lenhAi, "hoi");
  assert.doesNotMatch(c.moDauHoiAi, /\/plan|\/chia-bill/);
});

test("nhóm: giữ tờ hẹn AI và chữ của hội", () => {
  const c = chuKhay(false);
  assert.deepEqual(c.congCuHen, { label: "Tờ hẹn", dich: "plan" });
  assert.match(c.nhanPlan, /hội/);
  assert.equal(c.lenhAi, "plan");
  assert.equal(c.moDauHoiAi, "/plan ");
  assert.equal(c.hoiAiTrenKhay, null);
});
