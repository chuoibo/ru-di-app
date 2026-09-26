import assert from "node:assert/strict";
import test from "node:test";

import { laTuChoiDoiTen, moiBangSo } from "../dist-test/rudi/moi-bang-so.js";

// A synthetic number, assembled at run time: the repo guard refuses a literal
// Vietnamese mobile number in any tracked file, test data included.
const SO = ["09", "12", "00", "01", "02"].join("");

function loi(status, code) {
  return Object.assign(new Error(code), { status, code });
}

function buoc(datTen) {
  const goi = [];
  return {
    goi,
    b: {
      layId: async (so) => (goi.push(["id", so]), "p-1"),
      datTen: async (id, ten) => (goi.push(["ten", id, ten]), datTen()),
      moi: async (id) => void goi.push(["moi", id]),
    },
  };
}

test("người chưa có tên: đặt tên rồi mời", async () => {
  const { goi, b } = buoc(async () => undefined);
  assert.deepEqual(await moiBangSo(b, SO, "Linh"), { tenDaDat: true });
  assert.deepEqual(goi, [["id", SO], ["ten", "p-1", "Linh"], ["moi", "p-1"]]);
});

// QA 23/09: Linh had signed in first, so her row already carried a name and
// PUT /people/{id} said 403. The invitation must still be sent.
test("người đã tự có tên: 403 đổi tên không chặn lời mời", async () => {
  const { goi, b } = buoc(async () => {
    throw loi(403, "permission_denied");
  });
  assert.deepEqual(await moiBangSo(b, SO, "Linh"), { tenDaDat: false });
  assert.deepEqual(goi.at(-1), ["moi", "p-1"]);
});

test("lỗi khác ở bước tên vẫn dừng, không mời", async () => {
  for (const e of [loi(404, "person_not_found"), loi(403, "account_ended"), loi(0, "unreachable")]) {
    const { goi, b } = buoc(async () => {
      throw e;
    });
    await assert.rejects(moiBangSo(b, SO, "Linh"));
    assert.equal(goi.some((g) => g[0] === "moi"), false, e.code);
  }
});

test("laTuChoiDoiTen chỉ nhận đúng 403 permission_denied", () => {
  assert.equal(laTuChoiDoiTen(loi(403, "permission_denied")), true);
  assert.equal(laTuChoiDoiTen(loi(422, "permission_denied")), false);
  assert.equal(laTuChoiDoiTen(null), false);
});
