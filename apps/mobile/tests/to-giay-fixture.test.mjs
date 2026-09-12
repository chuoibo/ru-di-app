// The fixture notebook's transitions move the paper the way the server will:
// one full week round trip, the counter-proposal path, and every refusal.
import assert from "node:assert/strict";
import test from "node:test";

import {
  boNhap,
  deNghiSua,
  dongSo,
  ghiDaDi,
  giuMotDieu,
  guiTo,
  huyBuoi,
  nghiTuan,
  nguoiKiaDongY,
  nguoiNhanXem,
  phacToGiay,
  rutTo,
  suaNhap,
  toiDongY,
} from "../dist-test/rudi/to-giay/so-fixture.js";
import { coTheChot, daDongY, phienBan } from "../dist-test/rudi/to-giay/to-giay.js";

const TOI = "nguoi-a", KIA = "nguoi-b", T = (n) => `2026-09-1${n}T10:00:00Z`;
const DU = { ngay: "Thứ Bảy 20/09", choMoi: { viec: "Ăn tối, một quán chưa đi", gio: "18:30" }, diTiep: { viec: "Đi bộ, rồi chè", gio: "20:00" }, lyDo: "Ba tuần liền hai bạn ăn ở cùng một khu." };

test("Nếp phác: nhap, một chỗ chính + đi tiếp, mọi chặng can_kiem, chưa ai gửi, chưa ai ừ", () => {
  const to = phacToGiay("to-1", DU);
  assert.equal(to.state, "nhap");
  assert.equal(to.versions.length, 1);
  assert.equal(to.versions[0].content.chang.length, 2);
  assert.ok(to.versions[0].content.chang.every((c) => c.can_kiem && c.place_id === null), "phác nói «chưa biết»");
  assert.equal(to.versions[0].sent_by, null);
  assert.equal(daDongY(to, "toi", TOI), false);
  const trong = phacToGiay("to-2", { ...DU, choMoi: null, diTiep: null });
  assert.equal(trong.versions[0].content.chang.length, 0, "routine không có gì thì tờ trống, không bịa chỗ");
});

test("vòng trọn vẹn một tuần: phác → gửi → xem → người kia ừ → chốt (một outing) → đã đi → đã giữ", () => {
  let ds = [phacToGiay("to-1", DU)];
  ds = suaNhap(ds, "to-1", { ngay: DU.ngay, chang: [{ gio: "19:00", viec: "Ăn tối, một quán chưa đi", place_id: null, can_kiem: true }] }, "Đổi giờ cho kịp.");
  assert.equal(phienBan(ds[0]).content.chang[0].gio, "19:00", "sửa nháp ghi tại chỗ");
  assert.equal(ds[0].versions.length, 1, "nháp không có lịch sử");
  ds = guiTo(ds, "to-1", TOI, T(1));
  assert.equal(ds[0].state, "da_gui");
  assert.equal(ds[0].sent_by, TOI);
  assert.equal(daDongY(ds[0], "toi", TOI), true, "gửi = đã ừ");
  assert.equal(coTheChot(ds[0], TOI), false);
  ds = nguoiNhanXem(ds, "to-1", T(2));
  assert.equal(ds[0].state, "da_xem");
  assert.equal(phienBan(ds[0]).viewed_by_recipient_at, T(2));
  assert.equal(daDongY(ds[0], "nguoi_kia", TOI), false, "xem không phải ừ");
  ds = nguoiKiaDongY(ds, "to-1", TOI, "outing-1");
  assert.equal(ds[0].state, "chot", "cả hai ừ cùng v1");
  assert.equal(ds[0].outing_id, "outing-1");
  const lai = nguoiKiaDongY(ds, "to-1", TOI, "outing-2");
  assert.equal(lai[0].outing_id, "outing-1", "gọi lại không nhân đôi outing (K3)");
  assert.equal(lai[0].state, "chot");
  ds = ghiDaDi(ds, "to-1", KIA);
  assert.equal(ds[0].state, "da_di");
  assert.deepEqual(ghiDaDi(ds, "to-1", ""), ds, "đã đi cần người ghi");
  ds = giuMotDieu(ds, "to-1", "  Quán chè đầu hẻm, lần sau lại.  ", T(3));
  assert.equal(ds[0].state, "da_giu");
  assert.deepEqual(ds[0].keeps.map((k) => k.line), ["Quán chè đầu hẻm, lần sau lại."]);
  assert.deepEqual(giuMotDieu(ds, "to-1", "   ", T(4))[0].keeps.length, 1, "dòng trống không thêm");
});

test("đề nghị sửa: v+1 do người phản hồi gửi, người ấy đã ừ v+1, tờ về da_gui cho phía kia; ừ v1 không theo sang", () => {
  let ds = guiTo([phacToGiay("to-1", DU)], "to-1", TOI, T(1));
  ds = nguoiNhanXem(ds, "to-1", T(2));
  const noiDung2 = { ngay: DU.ngay, chang: [{ gio: "18:00", viec: "Ăn tối, một quán chưa đi", place_id: null, can_kiem: true }] };
  ds = deNghiSua(ds, "to-1", KIA, TOI, noiDung2, "Sớm hơn nửa tiếng.", T(3));
  assert.equal(ds[0].state, "da_gui");
  assert.equal(ds[0].version, 2);
  assert.equal(ds[0].versions.length, 2);
  assert.equal(phienBan(ds[0]).sent_by, KIA);
  assert.equal(daDongY(ds[0], "nguoi_kia", TOI, 2), true, "người sửa đã ừ bản của mình");
  assert.equal(daDongY(ds[0], "toi", TOI, 2), false, "tôi ừ v1 không kéo sang v2");
  assert.equal(phienBan(ds[0], 1).sent_by, TOI, "v1 bất biến");
  ds = toiDongY(ds, "to-1", TOI, "outing-9");
  assert.equal(ds[0].state, "chot");
  assert.equal(ds[0].outing_id, "outing-9");
  // I cannot counter my own version.
  const mine = guiTo([phacToGiay("to-2", DU)], "to-2", TOI, T(1));
  assert.deepEqual(deNghiSua(mine, "to-2", TOI, TOI, noiDung2, null, T(2)), mine);
  // The other cannot answer a version they sent.
  assert.deepEqual(nguoiKiaDongY(ds, "to-1", TOI, "x"), ds, "tờ đã chốt: không đổi");
});

test("rút, nghỉ tuần, bỏ nháp, huỷ buổi: đúng trạng thái đích, sai điều kiện thì không đổi", () => {
  const nhap = [phacToGiay("to-1", DU)];
  assert.equal(nghiTuan(nhap, "to-1")[0].state, "nghi_tuan", "nghỉ khi còn nháp = không phác lại tuần đó");
  assert.equal(boNhap(nhap, "to-1")[0].state, "bo");
  const gui = guiTo(nhap, "to-1", TOI, T(1));
  assert.equal(rutTo(gui, "to-1", TOI)[0].state, "rut");
  assert.deepEqual(rutTo(gui, "to-1", KIA), gui, "người nhận không rút");
  const xem = nguoiNhanXem(gui, "to-1", T(2));
  assert.deepEqual(rutTo(xem, "to-1", TOI), xem, "đã xem thì không rút");
  assert.equal(nghiTuan(xem, "to-1")[0].state, "huy", "nghỉ khi đã gửi = huỷ tờ");
  const chot = nguoiKiaDongY(xem, "to-1", TOI, "o");
  assert.deepEqual(nghiTuan(chot, "to-1"), chot, "chốt rồi không nghỉ tuần được");
  assert.equal(huyBuoi(chot, "to-1")[0].state, "huy");
  assert.deepEqual(huyBuoi(gui, "to-1"), gui, "chưa chốt thì không có buổi để huỷ");
  assert.deepEqual(guiTo(gui, "to-1", TOI, T(9)), gui, "gửi lại tờ đã gửi không đổi gì");
});

test("đóng sổ là đóng: nháp bỏ, tờ mở huỷ, kế hoạch và ký ức giữ nguyên", () => {
  const nhap = phacToGiay("nhap", DU);
  const gui = guiTo([phacToGiay("gui", DU)], "gui", TOI, T(1))[0];
  const chot = nguoiKiaDongY(nguoiNhanXem([gui], "gui", T(2)), "gui", TOI, "o")[0];
  const giu = giuMotDieu(ghiDaDi([{ ...chot, id: "giu" }], "giu", TOI), "giu", "Một điều.", T(3))[0];
  const sau = dongSo([nhap, gui, { ...chot, id: "chot" }, giu]);
  assert.deepEqual(sau.map((t) => t.state), ["bo", "huy", "chot", "da_giu"]);
  assert.deepEqual(dongSo(sau), sau, "đóng lần hai không làm sống lại gì");
});
