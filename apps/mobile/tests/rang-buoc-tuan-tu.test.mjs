import assert from "node:assert/strict";
import test from "node:test";

import { attemptFor, quenLuot } from "../dist-test/api.js";
import { ghiRangBuocTuanTu } from "../dist-test/rudi/to-giay/so-doi-map.js";

// A fake of `useToGiay.lam`: one command at a time, and a second one started
// while the first is in flight answers false -- exactly the live hook's rule.
function soGia() {
  let dangLam = false;
  const daGhi = [];
  const lam = async (ten, lamGi) => {
    if (dangLam) return false;
    dangLam = true;
    await new Promise((r) => setTimeout(r, 5));
    daGhi.push(ten);
    lamGi?.();
    dangLam = false;
    return true;
  };
  return {
    daGhi,
    buoc: {
      dat: (kind, noiDung) => lam(`dat:${kind}:${noiDung}`),
      xoa: (kind) => lam(`xoa:${kind}`),
    },
  };
}

const TRONG = { khong_an_duoc: "", dung: "" };

// QA 23/09: «Đừng» never saved, for anyone, on any save.
test("hai ô cùng lưu: cả hai lệnh tới nơi, lần lượt", async () => {
  const { daGhi, buoc } = soGia();
  const ok = await ghiRangBuocTuanTu(TRONG, { khong_an_duoc: "Hải sản", dung: "Quán bar ồn" }, buoc);
  assert.equal(ok, true);
  assert.deepEqual(daGhi, ["dat:khong_an_duoc:Hải sản", "dat:dung:Quán bar ồn"]);
});

test("ô không đổi không gửi lại; ô xoá thành DELETE", async () => {
  const { daGhi, buoc } = soGia();
  const ok = await ghiRangBuocTuanTu({ khong_an_duoc: "Hải sản", dung: "Ồn" }, { khong_an_duoc: "Hải sản ", dung: "" }, buoc);
  assert.equal(ok, true);
  assert.deepEqual(daGhi, ["xoa:dung"]);
});

test("lệnh đầu không tới nơi thì dừng và báo false", async () => {
  const daGhi = [];
  const ok = await ghiRangBuocTuanTu(TRONG, { khong_an_duoc: "A", dung: "B" }, {
    dat: async (kind) => (daGhi.push(kind), false),
    xoa: async () => true,
  });
  assert.equal(ok, false);
  assert.deepEqual(daGhi, ["khong_an_duoc"]);
});

// A landed write must not leave its key behind: the next, different write
// under the same name would be replayed or refused with 422 key reuse.
test("quenLuot: sau khi thành công, lần ghi sau có khoá mới", () => {
  const so = {};
  const dau = attemptFor(so, "rang-buoc:dung");
  assert.equal(attemptFor(so, "rang-buoc:dung"), dau, "thử lại giữ khoá");
  quenLuot(so, "rang-buoc:dung");
  assert.notEqual(attemptFor(so, "rang-buoc:dung"), dau);
});
