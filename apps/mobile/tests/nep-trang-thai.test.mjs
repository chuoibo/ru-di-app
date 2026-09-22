import assert from "node:assert/strict";
import { test } from "node:test";

import { DOCK_DAU, chuyen } from "../dist-test/rudi/nep/trang-thai.js";

test("Nếp bắt đầu ở trạng thái nghỉ, không có việc, không lui", () => {
  assert.deepEqual(DOCK_DAU, { trangThai: "nghi", nen: "nghi", coViec: false, luiLai: false });
});

test("chạm từ nghỉ thì mở bảng; chạm từ ẩn chỉ lôi Nếp ra, chưa mở", () => {
  assert.equal(chuyen(DOCK_DAU, { kieu: "cham" }).trangThai, "mo");
  const dangAn = chuyen(DOCK_DAU, { kieu: "vuot-ra" });
  assert.equal(dangAn.trangThai, "an");
  assert.equal(chuyen(dangAn, { kieu: "cham" }).trangThai, "nghi");
});

test("vuốt ra thì Nếp nằm gọn trong mép, và mép là nền để quay về", () => {
  const an = chuyen(DOCK_DAU, { kieu: "vuot-ra" });
  assert.deepEqual(an, { trangThai: "an", nen: "an", coViec: false, luiLai: false });
  assert.equal(chuyen(an, { kieu: "keo-vao" }).trangThai, "nghi");
});

test("đóng bảng thì Nếp về đúng nền trước đó, không tự hiện ra", () => {
  const anRoiMo = chuyen(chuyen(chuyen(DOCK_DAU, { kieu: "vuot-ra" }), { kieu: "cham" }), { kieu: "cham" });
  assert.equal(anRoiMo.trangThai, "mo");
  assert.equal(anRoiMo.nen, "nghi");
  assert.equal(chuyen(anRoiMo, { kieu: "dong" }).trangThai, "nghi");

  const moTuAn = { trangThai: "mo", nen: "an", coViec: false, luiLai: false };
  assert.equal(chuyen(moTuAn, { kieu: "dong" }).trangThai, "an");
});

test("báo việc làm mép dày lên, nhưng chỉ việc cần trả lời mới hé ra một dòng", () => {
  const chiBao = chuyen(DOCK_DAU, { kieu: "bao-viec", canTraLoi: false });
  assert.equal(chiBao.coViec, true);
  assert.equal(chiBao.trangThai, "nghi");

  const heRa = chuyen(DOCK_DAU, { kieu: "bao-viec", canTraLoi: true });
  assert.equal(heRa.trangThai, "he");
  assert.equal(heRa.coViec, true);
});

test("bong bóng hé tự thu về nền, và xong việc thì mép mỏng lại", () => {
  const he = chuyen(DOCK_DAU, { kieu: "bao-viec", canTraLoi: true });
  assert.equal(chuyen(he, { kieu: "het-gio-he" }).trangThai, "nghi");
  assert.equal(chuyen(he, { kieu: "vuot-ra" }).trangThai, "nghi");

  const xong = chuyen(he, { kieu: "xong-viec" });
  assert.equal(xong.coViec, false);
  assert.equal(xong.trangThai, "nghi");
});

test("Luật Nếp Đứng Xa Tiền: màn tiền ép Nếp lui vào mép và cấm hé", () => {
  const oManTien = chuyen(DOCK_DAU, { kieu: "doi-man", nepLui: true });
  assert.equal(oManTien.trangThai, "an");
  assert.equal(oManTien.luiLai, true);

  const coViecOManTien = chuyen(oManTien, { kieu: "bao-viec", canTraLoi: true });
  assert.equal(coViecOManTien.trangThai, "an", "màn tiền thì tuyệt đối không hé");
  assert.equal(coViecOManTien.coViec, true, "vẫn ghi nhận có việc, chỉ là không nói ra");
});

test("rời màn tiền thì Nếp trả về đúng ý người dùng trước đó, không tự bật lên", () => {
  const nguoiDungDaGiau = chuyen(DOCK_DAU, { kieu: "vuot-ra" });
  const quaManTien = chuyen(nguoiDungDaGiau, { kieu: "doi-man", nepLui: true });
  const roiManTien = chuyen(quaManTien, { kieu: "doi-man", nepLui: false });
  assert.equal(roiManTien.trangThai, "an", "người dùng đã giấu thì vẫn giấu");
  assert.equal(roiManTien.luiLai, false);

  const roiManTienKhiDangNghi = chuyen(chuyen(DOCK_DAU, { kieu: "doi-man", nepLui: true }), { kieu: "doi-man", nepLui: false });
  assert.equal(roiManTienKhiDangNghi.trangThai, "nghi");
});

test("đang mở bảng mà vào màn tiền thì bảng đóng lại, không đứng cạnh số tiền", () => {
  const dangMo = chuyen(DOCK_DAU, { kieu: "cham" });
  assert.equal(dangMo.trangThai, "mo");
  assert.equal(chuyen(dangMo, { kieu: "doi-man", nepLui: true }).trangThai, "an");
});
