// The chat tray's words for a pair versus a group (QA couple 23/09, §11).
// Owner decision 2026-09-28: a two-person chat has a group's tools, «Tờ hẹn»
// included; what still differs is wording, and a room of two is never «hội».
import assert from "node:assert/strict";
import test from "node:test";

import { MO_DAU_HOI_AI, chuKhay } from "../dist-test/rudi/chat/khay-cong-cu.js";

test("nhắn riêng hai người: khay không nói «hội» hay «nhóm»", () => {
  const c = chuKhay(true);
  // Every word the pair's tray can show.
  const cacCau = Object.values(c);
  assert.ok(cacCau.length >= 4, "khay cặp phải có đủ chữ");
  for (const cau of cacCau) assert.doesNotMatch(cau, /hội|nhóm/i, cau);
  // Its AI entry asks for a plan, like a group's.
  assert.equal(MO_DAU_HOI_AI, "/plan ");
});

test("nhóm: giữ chữ của hội", () => {
  const c = chuKhay(false);
  assert.match(c.nhanPlan, /hội/);
  assert.match(c.tieuDePoll, /Hội/);
});
