// «Người lo» of the week, worded from the reader's side (ADR-0034 §2.4).
import assert from "node:assert/strict";
import test from "node:test";

import { cauVaiTuan } from "../dist-test/rudi/to-giay/vai-tuan.js";

const TOI = "toi", KIA = "kia";
const vai = (nguoi_lo, cach = "suy", diem = [[TOI, 0], [KIA, 0]]) => ({ tuan: "2026-09-21", nguoi_lo, cach, diem: diem.map(([person_id, score]) => ({ person_id, score })) });

test("ngoài «Một đôi» không có câu", () => {
  assert.equal(cauVaiTuan(null, TOI, "Minh"), null);
});

test("chưa ai gửi gì: người lập sổ lo, nói đúng vì sao", () => {
  assert.deepEqual(cauVaiTuan(vai([KIA]), TOI, "Minh"), { nhan: "Tuần này Minh lo", vi: "Chưa ai gửi tờ nào, nên người lập sổ lo trước.", laToi: false });
});

test("người hay gửi tờ trước lo, câu theo phía người đọc", () => {
  const v = vai([TOI], "suy", [[TOI, 4], [KIA, 1]]);
  assert.deepEqual(cauVaiTuan(v, TOI, "Minh"), { nhan: "Tuần này bạn lo", vi: "Bạn hay gửi tờ trước và đề nghị sửa.", laToi: true });
  assert.equal(cauVaiTuan(v, KIA, "Linh").vi, "Linh hay gửi tờ trước và đề nghị sửa.");
});

test("hoà thì nói hoà", () => {
  assert.match(cauVaiTuan(vai([TOI], "suy", [[TOI, 2], [KIA, 2]]), TOI, "Minh").vi, /như nhau/);
});

test("đã chọn, kể cả share", () => {
  assert.equal(cauVaiTuan(vai([KIA], "chon"), TOI, "Minh").vi, "Đã chọn cho tuần này.");
  const share = cauVaiTuan(vai([TOI, KIA], "chon"), TOI, "Minh");
  assert.equal(share.nhan, "Tuần này hai bạn cùng lo");
  assert.equal(share.laToi, true);
});
