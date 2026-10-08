import assert from "node:assert/strict";
import test from "node:test";

import { datTokenPhien } from "../dist-test/api.js";
import { ghiVaoSo, hoaDonTrong, themMon } from "../dist-test/rudi/chia-bill/hoa-don.js";
import { renameLine, setLineTotal } from "../dist-test/receipt.js";

const A = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const B = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const C = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const roster = [{ id: B, name: "B QA" }, { id: C, name: "C QA" }, { id: A, name: "A QA" }];
const reviewed = { participantIds: [A, B, C], allocations: { [A]: 100001, [B]: 100000, [C]: 0 }, totalAmountVnd: 200001 };

function input() {
  let reading = renameLine(themMon(hoaDonTrong()), "mon-0", "Bữa tổng hợp");
  reading = setLineTotal(reading, "mon-0", "200001").reading;
  return { reading, assignment: { "mon-0": [B, A] }, roster, contextId: "c-qa", payerId: B, occasion: "Tổng hợp", attempts: {}, reviewedSplit: reviewed };
}

function transport(t, allocation) {
  const previous = globalThis.fetch;
  const calls = [];
  datTokenPhien("synthetic-session");
  t.after(() => { globalThis.fetch = previous; datTokenPhien(null); });
  globalThis.fetch = async (url, init = {}) => {
    const body = JSON.parse(init.body);
    calls.push({ path: new URL(url).pathname, body, headers: init.headers });
    const response = String(url).endsWith("/expenses")
      ? { expense_id: "expense-qa", proposal: body, allocation: { allocations: allocation, rounding_gainers: [A], warnings: [] } }
      : { expense_version_id: "version-qa", payer_acknowledgement: body.acknowledge_as_advancer ? "acknowledged" : "pending" };
    return new Response(JSON.stringify(response), { status: 201, headers: { "Content-Type": "application/json" } });
  };
  return calls;
}

test("ghi sổ giữ thứ tự người từ preview máy chủ, kể cả phần 0đ", async (t) => {
  const calls = transport(t, reviewed.allocations);
  const result = await ghiVaoSo(input());
  assert.equal(result.expenseVersionId, "version-qa");
  assert.deepEqual(calls[0].body.participants, [A, B, C]);
  assert.deepEqual(calls[1].body.expected_allocations, reviewed.allocations);
  assert.equal(calls[0].body.total_amount_vnd, reviewed.totalAmountVnd);
});

test("đồng lẻ đổi người giữa preview và proposal thì không xác nhận", async (t) => {
  const calls = transport(t, { [A]: 100000, [B]: 100001, [C]: 0 });
  await assert.rejects(ghiVaoSo(input()), (error) => error.code === "proposal_changed");
  assert.deepEqual(calls.map((call) => call.path), ["/expenses"]);
});

test("proposal bỏ người có phần 0đ cũng không phải bản đã xem", async (t) => {
  const calls = transport(t, { [A]: 100001, [B]: 100000 });
  await assert.rejects(ghiVaoSo(input()), (error) => error.code === "proposal_changed");
  assert.equal(calls.length, 1);
});

test("tổng preview đổi so với bill thì không tạo proposal mới", async (t) => {
  const calls = transport(t, reviewed.allocations);
  await assert.rejects(ghiVaoSo({ ...input(), reviewedSplit: { ...reviewed, totalAmountVnd: 200002 } }), (error) => error.code === "proposal_changed");
  assert.equal(calls.length, 0);
});

test("người ghi khác người trả không nhận ứng tiền thay họ", async (t) => {
  const calls = transport(t, reviewed.allocations);
  const result = await ghiVaoSo({ ...input(), actorId: A });
  assert.equal(calls[0].body.recorded_by_id, A);
  assert.equal(calls[0].body.paid_by_id, B);
  assert.equal(calls[0].headers["X-Actor-ID"], A);
  assert.equal(calls[1].headers["X-Actor-ID"], A);
  assert.equal(calls[1].body.acknowledge_as_advancer, false);
  assert.equal(result.acknowledged, false);
});
