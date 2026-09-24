import assert from "node:assert/strict";
import { test } from "node:test";

import { GIOI_HAN, KHOA_SO_LIEU, donPhieu, nepPhaiLui } from "../dist-test/rudi/nep/phieu.js";

test("phiếu tối thiểu chỉ cần tên màn", () => {
  assert.deepEqual(donPhieu({ man: "outings/[id]" }), { man: "outings/[id]" });
});

test("không có tên màn thì không có phiếu, chứ không phải phiếu rỗng", () => {
  assert.equal(donPhieu({}), null);
  assert.equal(donPhieu({ man: "" }), null);
  assert.equal(donPhieu({ man: "   " }), null);
  assert.equal(donPhieu(null), null);
  assert.equal(donPhieu("outings"), null);
});

test("khoá số liệu lạ bị loại, không đi kèm sang máy chủ", () => {
  const p = donPhieu({
    man: "groups/[id]/chat",
    soLieu: { soNguoi: 4, tenThanhVien: "Lan", soTien: 250000, noiDungChat: "tối nay ăn gì" },
  });
  assert.deepEqual(p.soLieu, { soNguoi: 4 });
  for (const k of Object.keys(p.soLieu)) assert.ok(KHOA_SO_LIEU.includes(k));
});

test("tất cả khoá bị loại thì bỏ hẳn soLieu chứ không để lại object rỗng", () => {
  const p = donPhieu({ man: "finance", soLieu: { soTien: 250000 } });
  assert.equal(p.soLieu, undefined);
});

test("khoá hợp lệ nhưng giá trị không phải số hay chuỗi ngắn thì vẫn bị loại", () => {
  const p = donPhieu({
    man: "trips/[id]/album",
    soLieu: { soAnh: 12, soChang: { a: 1 }, soNgay: null, soMuc: "x".repeat(GIOI_HAN.soLieuChu + 1) },
  });
  assert.deepEqual(p.soLieu, { soAnh: 12 });
});

test("khoá lạ ở tầng trên cùng cũng bị loại, phiếu là danh sách đóng", () => {
  const p = donPhieu({ man: "plan", tinNhan: "bí mật", token: "abc", nguoiDung: { id: 1 } });
  assert.deepEqual(Object.keys(p).sort(), ["man"]);
});

test("tiêu đề và gợi ý bị cắt theo hạn, chuỗi dài không thành đường tuồn dữ liệu", () => {
  const p = donPhieu({
    man: "plan",
    tieuDe: "T".repeat(GIOI_HAN.tieuDe + 50),
    goiY: ["a", "b", "c", "d", "e"],
  });
  assert.equal(p.tieuDe.length, GIOI_HAN.tieuDe);
  assert.equal(p.goiY.length, GIOI_HAN.goiY);
});

test("nhịp kèo và loại sổ đi qua nguyên vẹn vì chúng là từ vựng đóng sẵn có", () => {
  const p = donPhieu({ man: "outings/[id]", nhip: { kieu: "sap-toi", conNgay: 3 }, loaiSo: "doi" });
  assert.deepEqual(p.nhip, { kieu: "sap-toi", conNgay: 3 });
  assert.equal(p.loaiSo, "doi");
  assert.equal(donPhieu({ man: "outings/[id]", loaiSo: "bia-ra" }).loaiSo, undefined);
  assert.equal(donPhieu({ man: "outings/[id]", nhip: { kieu: "bia-ra" } }).nhip, undefined);
});

test("Luật Nếp Đứng Xa Tiền nhận ra màn tiền ở cả hai cách viết đường dẫn", () => {
  for (const man of ["finance", "/finance", "settlements/[id]", "/settlements/abc", "batches/[id]", "smart-split/[id]/review"]) {
    assert.equal(nepPhaiLui(man), true, man);
  }
});

test("màn thường không bị nhầm thành màn tiền, kể cả khi tên bắt đầu giống", () => {
  for (const man of ["explore", "plan", "/groups/abc/chat", "financial-report", "settlements-guide", "", "/"]) {
    assert.equal(nepPhaiLui(man), false, man);
  }
});

// The slip belongs to the focused screen (ghiPhieu in phieu.ts). The provider
// used to clear it on pathname change, and lost the race: expo-router runs the
// incoming screen's effects before the provider sees the new pathname
// (measured 24-09: Khám phá declared while the provider was still on /otp).
import { ghiPhieu } from "../dist-test/rudi/nep/phieu.js";

test("màn được focus khai phiếu thì Nếp đọc được phiếu đó", () => {
  const p = donPhieu({ man: "explore" });
  assert.equal(ghiPhieu(null, { kieu: "khai", phieu: p }), p);
});

test("màn rời focus rút phiếu của mình: màn sau không khai gì thì không thừa hưởng", () => {
  const p = donPhieu({ man: "outings/[id]", soLieu: { soChang: 3 } });
  const ghi = ghiPhieu(null, { kieu: "khai", phieu: p });
  assert.equal(ghiPhieu(ghi, { kieu: "bo", phieu: p }), null);
});

test("màn cũ rời focus muộn không xoá phiếu màn mới vừa khai", () => {
  const cu = donPhieu({ man: "explore" });
  const moi = donPhieu({ man: "plan" });
  let ghi = ghiPhieu(null, { kieu: "khai", phieu: cu });
  ghi = ghiPhieu(ghi, { kieu: "khai", phieu: moi });
  ghi = ghiPhieu(ghi, { kieu: "bo", phieu: cu });
  assert.equal(ghi, moi);
});
