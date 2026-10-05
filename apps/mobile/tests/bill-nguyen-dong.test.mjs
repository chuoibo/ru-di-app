/**
 * nguyenDong refuses a number past 2^53 (audit 2026-10-05, PER-FE-MONEY-01):
 * JSON.parse has already lost đồng there, and showing the rounded figure would
 * be the app inventing an amount nobody's ledger holds.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { nguyenDong } from "../dist-test/bill.js";

test("nguyenDong: số nguyên an toàn đi qua nguyên vẹn", () => {
  assert.equal(nguyenDong(82_000, "amount_vnd"), 82_000);
  assert.equal(nguyenDong(Number.MAX_SAFE_INTEGER, "net_vnd"), Number.MAX_SAFE_INTEGER);
  assert.equal(nguyenDong(-Number.MAX_SAFE_INTEGER, "net_vnd"), -Number.MAX_SAFE_INTEGER);
});

test("nguyenDong: số đã mất đồng khi qua JSON bị từ chối", () => {
  const parsed = JSON.parse(`{"net_vnd": ${(2n ** 53n + 1n).toString()}}`).net_vnd;
  assert.throws(() => nguyenDong(parsed, "net_vnd"), RangeError);
  assert.throws(() => nguyenDong(1.5, "amount_vnd"), RangeError);
});
