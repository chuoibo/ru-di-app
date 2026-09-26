// The seven rules of spec §3.3 and the two ADR-0027 adds, checked on the pure
// client model in src/rudi/to-giay/to-giay.ts. No screen, no server: hand-built
// papers shaped exactly like `GET /papers/{pid}` answers.
import assert from "node:assert/strict";
import test from "node:test";

import {
  TRANG_THAI_CUOI,
  TRANG_THAI_MO,
  TRANG_THAI_TO,
  cauTrangThai,
  coTheChot,
  coTheDeNghiSua,
  coTheDongY,
  coTheNghiTuan,
  coTheRut,
  daDongY,
  goiYChoLam,
  nenXinTo,
  demHauQuaDongSo,
  khacGi,
  laKeHoach,
  nutChoTo,
  tenNgan,
  toUuTien,
} from "../dist-test/rudi/to-giay/to-giay.js";

const TOI = "nguoi-a", KIA = "nguoi-b";
const noiDung = (gio = "18:30", viec = "Ăn tối, một quán chưa đi", ngay = "Thứ Bảy 20/09") => ({
  ngay,
  chang: [{ gio, viec, place_id: null, can_kiem: true }],
});
const phienBan = (version, sent_by, over = {}) => ({
  version,
  content: noiDung(),
  ly_do: null,
  sent_at: sent_by ? `2026-09-1${version}T10:00:00Z` : null,
  sent_by,
  author_type: "human",
  my_response: null,
  their_agreed: false,
  viewed_by_recipient_at: null,
  ...over,
});
const to = (state, versions, over = {}) => ({
  id: "to-1",
  state,
  version: versions[versions.length - 1].version,
  author_type: versions[versions.length - 1].author_type,
  sent_by: versions[versions.length - 1].sent_by,
  versions,
  outing_id: state === "chot" || state === "da_di" || state === "da_giu" ? "outing-1" : null,
  keeps: [],
  tuan: "2026-09-14",
  expires_at: "2026-09-20T17:00:00Z",
  // The server's answer, not a guess. Default true so the cases written before
  // this field existed keep asking what they were written to ask; the two cases
  // about the day itself pass it explicitly.
  co_the_ghi_da_di: true,
  ...over,
});

test("từ vựng đóng: mười ba trạng thái, sáu trạng thái cuối, năm trạng thái mở, không trùng", () => {
  assert.equal(TRANG_THAI_TO.length, 13);
  assert.equal(new Set(TRANG_THAI_TO).size, 13);
  assert.equal(TRANG_THAI_CUOI.length, 6);
  assert.equal(TRANG_THAI_MO.length, 5);
  for (const s of [...TRANG_THAI_CUOI, ...TRANG_THAI_MO]) assert.ok(TRANG_THAI_TO.includes(s), s);
  assert.equal(TRANG_THAI_CUOI.filter((s) => TRANG_THAI_MO.includes(s)).length, 0, "mở và cuối rời nhau");
});

test("luật §3.4: người bấm gửi đã đồng ý phiên bản đó; tờ Nếp gửi thì không ai đồng ý sẵn", () => {
  const nguoi = to("da_gui", [phienBan(1, TOI)]);
  assert.equal(daDongY(nguoi, "toi", TOI), true, "tôi gửi = tôi đã ừ");
  assert.equal(daDongY(nguoi, "nguoi_kia", TOI), false, "người kia chưa nói gì");
  const nep = to("da_gui", [phienBan(1, null, { author_type: "nep", sent_at: "2026-09-11T10:00:00Z" })], { author_type: "nep" });
  assert.equal(daDongY(nep, "toi", TOI), false, "Nếp gửi: tôi chưa ừ");
  assert.equal(daDongY(nep, "nguoi_kia", TOI), false, "Nếp gửi: người kia chưa ừ");
  assert.equal(coTheChot(nep, TOI), false);
  const nepCaHai = to("dong_y", [phienBan(1, null, { author_type: "nep", sent_at: "x", my_response: "dong_y", their_agreed: true })], { author_type: "nep" });
  assert.equal(coTheChot(nepCaHai, TOI), true, "tờ Nếp cần CẢ HAI ừ mới chốt (ADR-0027)");
});

test("luật 2: im lặng không phải đồng ý; hết khung là het_han, không bao giờ là kế hoạch", () => {
  const motPhia = to("dong_y", [phienBan(1, TOI)]);
  assert.equal(coTheChot(motPhia, TOI), false, "một phía ừ chưa chốt được");
  const hetHan = to("het_han", [phienBan(1, TOI)]);
  assert.equal(laKeHoach(hetHan), false);
  assert.equal(coTheChot(hetHan, TOI), false);
  assert.deepEqual(nutChoTo(hetHan, TOI), [], "hết hạn thì không còn nút nào");
  for (const s of TRANG_THAI_TO) assert.equal(laKeHoach(to(s, [phienBan(1, TOI)])), ["chot", "da_di", "da_giu"].includes(s), s);
});

test("luật 3: đồng ý gắn với phiên bản, không sang v+1; khacGi nói cái gì đã đổi", () => {
  const v1 = phienBan(1, TOI, { their_agreed: true });
  const v2 = phienBan(2, KIA, { content: noiDung("19:00", "Ăn tối, một quán chưa đi") });
  const tờ = to("da_gui", [v1, v2]);
  assert.equal(daDongY(tờ, "nguoi_kia", TOI, 1), true, "người kia đã ừ v1");
  assert.equal(daDongY(tờ, "nguoi_kia", TOI, 2), true, "người kia GỬI v2 nên đã ừ v2");
  assert.equal(daDongY(tờ, "toi", TOI, 2), false, "tôi ừ v1 không kéo sang v2");
  assert.equal(coTheChot(tờ, TOI), false);
  const doi = khacGi(v2, v1);
  assert.deepEqual(doi, ["Giờ chỗ chính: 18:30 → 19:00"]);
  const v3 = phienBan(3, TOI, { content: { ngay: "Chủ nhật 21/09", chang: [...noiDung("19:00").chang, { gio: "21:00", viec: "Đi bộ, rồi chè", place_id: null, can_kiem: true }] }, ly_do: "Tối thứ Bảy mưa." });
  const doi3 = khacGi(v3, v2);
  assert.ok(doi3.some((d) => d.startsWith("Ngày:")), "đổi ngày phải hiện");
  assert.ok(doi3.some((d) => d.startsWith("Thêm chặng đi tiếp")), "thêm chặng phải hiện");
  assert.ok(!doi3.some((d) => d.startsWith("Lý do")), "lý do không phải dòng đổi: tờ đã in «Vì: …» dưới nó");
  assert.deepEqual(khacGi(v1, undefined), [], "phiên bản đầu không có gì để so");
  assert.deepEqual(khacGi(v2, v2), []);
});

test("ADR-0027: rút chỉ khi chưa ai xem và chưa phản hồi, và chỉ người gửi", () => {
  const moiGui = to("da_gui", [phienBan(1, TOI)]);
  assert.equal(coTheRut(moiGui, TOI), true);
  assert.equal(coTheRut(moiGui, KIA), false, "người nhận không rút được");
  assert.equal(coTheRut(to("da_gui", [phienBan(1, TOI, { viewed_by_recipient_at: "2026-09-11T11:00:00Z" })]), TOI), false, "đã xem thì không rút");
  assert.equal(coTheRut(to("da_gui", [phienBan(1, TOI, { their_agreed: true })]), TOI), false, "đã có phản hồi thì không rút");
  assert.equal(coTheRut(to("da_xem", [phienBan(1, TOI)]), TOI), false, "da_xem: đường đi là phiên bản mới");
  assert.equal(coTheRut(to("da_gui", [phienBan(1, null, { author_type: "nep", sent_at: "x" })], { author_type: "nep" }), TOI), false, "tờ Nếp không ai rút");
});

test("nghỉ tuần bấm được tới chốt, không hơn; chốt rồi thì huỷ là đường ra", () => {
  for (const s of TRANG_THAI_MO) assert.equal(coTheNghiTuan(to(s, [phienBan(1, TOI)])), true, s);
  for (const s of ["chot", "da_di", "da_giu", ...TRANG_THAI_CUOI]) assert.equal(coTheNghiTuan(to(s, [phienBan(1, TOI)])), false, s);
  assert.deepEqual(nutChoTo(to("chot", [phienBan(1, TOI, { their_agreed: true })]), TOI), ["da_di", "huy"]);
  assert.deepEqual(nutChoTo(to("da_di", [phienBan(1, TOI, { their_agreed: true })]), TOI), ["giu"]);
});

test("bộ nút theo trạng thái × vai: người nhận ừ hoặc đề nghị sửa, người gửi chỉ rút hoặc nghỉ", () => {
  const guiBoiToi = to("da_gui", [phienBan(1, TOI)]);
  assert.deepEqual(nutChoTo(guiBoiToi, TOI), ["rut", "nghi_tuan"]);
  assert.deepEqual(nutChoTo(guiBoiToi, KIA), ["dong_y", "de_nghi_sua", "nghi_tuan"]);
  const daXem = to("da_xem", [phienBan(1, KIA, { viewed_by_recipient_at: "x" })]);
  assert.deepEqual(nutChoTo(daXem, TOI), ["dong_y", "de_nghi_sua", "nghi_tuan"], "tôi là người nhận đã mở");
  assert.deepEqual(nutChoTo(daXem, KIA), ["nghi_tuan"], "người gửi sau khi bị xem chỉ còn nghỉ");
  const toiDaU = to("dong_y", [phienBan(1, KIA, { my_response: "dong_y" })]);
  assert.deepEqual(nutChoTo(toiDaU, TOI), ["nghi_tuan"], "đã ừ thì không ừ lại, không đề nghị sửa nữa");
  assert.equal(coTheDongY(toiDaU, TOI), false);
  assert.equal(coTheDeNghiSua(toiDaU, TOI), false);
  assert.deepEqual(nutChoTo(to("nhap", [phienBan(1, null)]), TOI), ["gui", "sua_nhap", "bo", "nghi_tuan"]);
  for (const s of TRANG_THAI_CUOI) assert.deepEqual(nutChoTo(to(s, [phienBan(1, TOI)]), TOI), [], s);
});

test("câu trạng thái: mọi trạng thái × hai vai ra một câu 8 đến 120 ký tự, không gạch dài, nói đúng ai làm gì", () => {
  for (const s of TRANG_THAI_TO) {
    for (const [tenVai, nguoiGui] of [["tôi gửi", TOI], ["người kia gửi", KIA]]) {
      const cau = cauTrangThai(to(s, [phienBan(1, s === "nhap" ? null : nguoiGui)]), TOI);
      assert.equal(typeof cau, "string", `${s}/${tenVai}`);
      assert.ok(cau.length >= 8 && cau.length <= 120, `${s}/${tenVai}: ${cau.length} ký tự`);
      assert.ok(!cau.includes("—"), `${s}/${tenVai}: không gạch dài`);
    }
  }
  assert.match(cauTrangThai(to("da_gui", [phienBan(1, TOI)]), TOI), /chờ trả lời/i);
  assert.match(cauTrangThai(to("da_gui", [phienBan(1, KIA)]), TOI), /vừa gửi/i);
  assert.match(cauTrangThai(to("rut", [phienBan(1, KIA)]), TOI), /^Người ấy/);
  assert.match(cauTrangThai(to("het_han", [phienBan(1, TOI)]), TOI), /không thành kế hoạch/i);
  const nep = to("da_gui", [phienBan(1, null, { author_type: "nep", sent_at: "x" })], { author_type: "nep" });
  assert.match(cauTrangThai(nep, TOI), /Nếp gửi hộ/);
});

test("một tờ đang mở: việc cần quyết trước, rồi tờ đang chờ, rồi kế hoạch, rồi ký ức", () => {
  const canQuyet = { ...to("da_gui", [phienBan(1, KIA)]), id: "quyet" };
  const dangCho = { ...to("da_xem", [phienBan(2, TOI, { viewed_by_recipient_at: "x" })]), id: "cho" };
  const keHoach = { ...to("chot", [phienBan(1, TOI, { their_agreed: true })]), id: "ke-hoach" };
  const kyUc = { ...to("da_giu", [phienBan(1, TOI, { their_agreed: true })]), id: "ky-uc" };
  assert.equal(toUuTien([kyUc, keHoach, dangCho, canQuyet], TOI)?.id, "quyet");
  assert.equal(toUuTien([kyUc, keHoach, dangCho], TOI)?.id, "cho");
  assert.equal(toUuTien([kyUc, keHoach], TOI)?.id, "ke-hoach");
  assert.equal(toUuTien([kyUc], TOI)?.id, "ky-uc");
  assert.equal(toUuTien([], TOI), undefined);
  const nhapCuaToi = { ...to("nhap", [phienBan(1, null)]), id: "nhap" };
  assert.equal(toUuTien([dangCho, nhapCuaToi], TOI)?.id, "nhap", "bản phác chưa gửi là việc của tôi bây giờ");
});

test("đóng sổ: đếm tờ đang chờ sẽ huỷ, tờ đã chốt sẽ khoá, lời đề nghị sẽ huỷ", () => {
  const ds = [
    to("nhap", [phienBan(1, null)]),
    to("da_gui", [phienBan(1, TOI)]),
    to("chot", [phienBan(1, TOI, { their_agreed: true })]),
    to("het_han", [phienBan(1, TOI)]),
    to("da_giu", [phienBan(1, TOI, { their_agreed: true })]),
  ];
  assert.deepEqual(demHauQuaDongSo(ds, 1), { so_nhap_bo: 1, so_to_huy: 1, so_to_khoa: 1, so_de_nghi_huy: 1 }, "nháp bỏ; đã gửi huỷ; chốt khoá; hết hạn và đã giữ không đếm");
  assert.deepEqual(demHauQuaDongSo([], 0), { so_nhap_bo: 0, so_to_huy: 0, so_to_khoa: 0, so_de_nghi_huy: 0 });
  assert.deepEqual(demHauQuaDongSo(ds, -2), { so_nhap_bo: 1, so_to_huy: 1, so_to_khoa: 1, so_de_nghi_huy: 0 }, "số âm không lọt");
});

test("nút «Đã đi rồi» chỉ hiện khi máy chủ nói ngày đã tới (§3.3 luật 6)", () => {
  const chot = [phienBan(1, TOI, { their_agreed: true, my_response: "dong_y" })];
  const truocNgay = to("chot", chot, { co_the_ghi_da_di: false });
  assert.deepEqual(nutChoTo(truocNgay, TOI), ["huy"], "chưa tới ngày thì không có nút ghi đã đi");
  assert.deepEqual(nutChoTo(truocNgay, KIA), ["huy"], "phía người kia cũng vậy");

  const toiNgay = to("chot", chot, { co_the_ghi_da_di: true });
  assert.deepEqual(nutChoTo(toiNgay, TOI), ["da_di", "huy"]);

  // Và cờ ấy KHÔNG đổi được gì ở các trạng thái khác: nó chỉ nói về một buổi
  // đã chốt, không phải một quyền chung.
  const nhap = to("nhap", [phienBan(1, null)], { co_the_ghi_da_di: true });
  assert.ok(!nutChoTo(nhap, TOI).includes("da_di"));
  const daDi = to("da_di", chot, { co_the_ghi_da_di: false });
  assert.deepEqual(nutChoTo(daDi, TOI), ["giu"], "đã ghi rồi thì cờ không rút nút giữ lại");
});

test("tên dài không phá con dấu: lấy chữ cuối, chặn độ dài", () => {
  // Con dấu là khối chữ hoa nén trong một hàng `space-between` cạnh ngày, không
  // `maxWidth`, không `numberOfLines`. Tên đầy đủ sẽ xuống dòng trong dấu hoặc
  // bóp nát cột ngày — cùng lớp lỗi với TopBar ở 360dp/1.3.
  assert.equal(tenNgan("De QA"), "QA");
  assert.equal(tenNgan("Nguyễn Thị Minh Hà"), "Hà");
  assert.equal(tenNgan("  Bình  "), "Bình");
  assert.ok(tenNgan("Bartholomewwwwwwww").length <= 10, "quá dài thì cắt");
  assert.ok(tenNgan("Bartholomewwwwwwww").endsWith("…"), "và nói rằng đã cắt");
  assert.equal(tenNgan(""), "", "chuỗi rỗng vẫn là chuỗi rỗng");
});

test("«Rủ … tới đây»: mở trình sửa khi được sửa, chờ khi chưa có tờ, nói rõ khi không thêm được", () => {
  assert.deepEqual(goiYChoLam(undefined, TOI, "Minh", "p-1"), { lam: "cho" });
  assert.deepEqual(goiYChoLam(to("nhap", [phienBan(1, null)]), TOI, "Minh", "p-1"), { lam: "mo" });
  assert.deepEqual(goiYChoLam(to("da_gui", [phienBan(1, KIA)]), TOI, "Minh", "p-1"), { lam: "mo" }, "tờ người kia gửi: đề nghị sửa với chỗ này");
  const toiGui = goiYChoLam(to("da_gui", [phienBan(1, TOI)]), TOI, "Minh", "p-1");
  assert.equal(toiGui.lam, "bao", "tờ mình đã gửi thì không lặng lẽ bỏ chỗ");
  assert.match(toiGui.cau, /chờ Minh trả lời/);
  const daChot = goiYChoLam(to("chot", [phienBan(1, TOI)]), TOI, "Minh", "p-1");
  assert.equal(daChot.lam, "bao");
  assert.match(daChot.cau, /tuần sau/);
});

test("?ru=1 chỉ xin tờ khi đã đọc xong và chưa có tờ mở", () => {
  assert.equal(nenXinTo(false, undefined), "cho", "chưa đọc xong thì chưa biết có tờ không");
  assert.equal(nenXinTo(true, undefined), "xin");
  assert.equal(nenXinTo(true, to("da_gui", [phienBan(1, TOI)])), "thoi", "tờ đang mở: không xin tờ thứ hai");
  assert.equal(nenXinTo(true, to("nhap", [phienBan(1, null)])), "thoi");
  assert.equal(nenXinTo(true, to("nghi_tuan", [phienBan(1, TOI)])), "xin");
});

test("«Rủ … tới đây» cho một chỗ đã ở trên tờ thì nói vậy, không mở trình sửa", () => {
  const coCho = phienBan(1, null, { content: { ngay: "Thứ Bảy 20/09", chang: [{ gio: "19:00", viec: "Cà phê", place_id: "p-1", can_kiem: false }] } });
  const r = goiYChoLam(to("nhap", [coCho]), TOI, "Minh", "p-1");
  assert.equal(r.lam, "bao");
  assert.match(r.cau, /đã ở trên tờ/);
  assert.deepEqual(goiYChoLam(to("nhap", [coCho]), TOI, "Minh", "p-2"), { lam: "mo" });
});
