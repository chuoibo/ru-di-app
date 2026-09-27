/* Thành viên khác trong nhóm thấy câu trả lời Rủ Đi AI hiện dần (lát 12,
 * thiết kế 02 §5.3–5.4, hợp đồng §4.5).
 *
 * Ca này chạy đúng mã màn hình dùng (`dist-test/rudi/ai/phong-ai.js`,
 * `ai/tra-loi-song.js`, `chat/ai-invocations.js`):
 *   - đọc frame `ai`: chỉ enum đóng của phòng, id phải giống id, không tin gì thêm;
 *   - gộp frame vào kho phòng qua ĐÚNG máy trạng thái của người hỏi
 *     (`buocTraLoi`): đang nghĩ → đang viết → xong, trùng id bị bỏ, chữ sau
 *     xong không mở lại;
 *   - nối lại: kho làm lại từ rỗng, phát lại của máy chủ không nhân đôi chữ;
 *   - dọn: lỗi/huỷ bỏ ngay, xong giữ chờ thẻ rồi bỏ, im 30 s thì bỏ;
 *   - hàng của người xem khác: câu riêng, không lẫn câu của người hỏi; không
 *     vẽ câu trả lời của chính mình hai lần;
 *   - Reduce Motion: chữ tới theo từng câu, như màn người hỏi.
 *
 * Nó KHÔNG chứng minh: màn hình vẽ đúng (ảnh chụp ở bảng lab), WebSocket trên
 * máy thật, hay độ trễ thật.
 */
import assert from "node:assert/strict";
import { test } from "node:test";

import {
  GIU_SAU_XONG_MS,
  IM_LANG_TOI_DA_MS,
  KHO_PHONG_TRONG,
  SO_LUOT_TOI_DA,
  buocPhong,
  docKhungAi,
  donPhong,
  laKhungAi,
  luotChoNguoiXem,
} from "../dist-test/rudi/ai/phong-ai.js";
import { chuDaViet, chuHienThi } from "../dist-test/rudi/ai/tra-loi-song.js";
import { CAU_CHO_TRA_LOI, hangTraLoiSong, loiGoiCuaPhong } from "../dist-test/rudi/chat/ai-invocations.js";

const INV = "0b8f1c9e-aaaa-4bbb-8ccc-00000000aa01";
const INV2 = "0b8f1c9e-aaaa-4bbb-8ccc-00000000aa02";
const TIN = "0b8f1c9e-aaaa-4bbb-8ccc-00000000bb01";
const THE = "0b8f1c9e-aaaa-4bbb-8ccc-00000000cc01";

const khung = (id, e, d, inv = INV, extra = {}) => ({ type: "ai", inv, tin: TIN, so_tin: 6, id, e, d, ...extra });
const doc = (raw) => {
  const k = docKhungAi(raw);
  assert.ok(k, `frame refused: ${JSON.stringify(raw)}`);
  return k;
};
const gop = (frames, kho = KHO_PHONG_TRONG, luc = 1000) => frames.reduce((k, f) => buocPhong(k, doc(f), luc), kho);

/* ------------------------------------------------ đọc frame -------------- */

test("chỉ frame ai của enum đóng mới được đọc; trang thay đổi không phải frame ai", () => {
  assert.equal(laKhungAi({ context_id: "x", changes: [], next_sequence: 1 }), false, "trang thay đổi không có type");
  assert.equal(laKhungAi(khung("1-0", "delta", {})), true);
  for (const e of ["hello", "thu_hoi", "ket_noi_lai", "retract", "snapshot"]) {
    assert.equal(docKhungAi(khung("1-0", e, {})), null, `${e} không thuộc phòng`);
  }
  for (const e of ["trang_thai", "phan", "delta", "lam_lai", "xong", "that_bai", "huy"]) {
    assert.ok(docKhungAi(khung("1-0", e, {})), `${e} thuộc phòng`);
  }
  assert.equal(docKhungAi(khung("x", "delta", {})), null, "id luồng sai dạng");
  assert.equal(docKhungAi(khung("1-0", "delta", {}, "không phải id")), null, "inv sai dạng");
  const k = docKhungAi(khung("1-0", "delta", { p: 0, text: "x" }, INV, { tin: "<script>", so_tin: -3 }));
  assert.equal(k.tin, null, "tin sai dạng bị bỏ, không tin theo");
  assert.equal(k.soTin, 0);
});

/* ------------------------------------------------ cùng máy trạng thái ---- */

test("thành viên khác đi đúng máy trạng thái của người hỏi: đang nghĩ → đang viết → xong", () => {
  let kho = gop([khung("1-0", "trang_thai", { cau: "dang_doc" })]);
  assert.equal(kho[INV].traLoi.pha, "dang_nghi");
  assert.equal(kho[INV].tin, TIN);
  assert.equal(kho[INV].soTin, 6);
  kho = gop([khung("2-0", "delta", { p: 0, text: "Hồ Xuân Hương " }), khung("2-1", "delta", { p: 0, text: "lên đèn." })], kho);
  assert.equal(kho[INV].traLoi.pha, "dang_viet");
  assert.equal(chuDaViet(kho[INV].traLoi), "Hồ Xuân Hương lên đèn.");
  const truoc = kho;
  kho = gop([khung("2-1", "delta", { p: 0, text: "lên đèn." })], kho);
  assert.equal(kho, truoc, "frame trùng id không đổi gì (cùng một đối tượng)");
  kho = gop([khung("3-0", "xong", { message_id: THE })], kho);
  assert.equal(kho[INV].traLoi.pha, "xong");
  assert.equal(kho[INV].traLoi.ketThuc.messageId, THE);
  assert.equal(chuDaViet(kho[INV].traLoi), "Hồ Xuân Hương lên đèn.", "xong của nhóm không mang chữ: chữ đã hiện giữ nguyên");
  kho = gop([khung("4-0", "delta", { p: 0, text: " chữ muộn" })], kho);
  assert.equal(chuDaViet(kho[INV].traLoi), "Hồ Xuân Hương lên đèn.", "chữ sau xong không mở lại câu trả lời");
});

test("hai câu trả lời cùng lúc trong phòng giữ riêng; có trần số lượt", () => {
  let kho = gop([khung("1-0", "delta", { p: 0, text: "A" }), khung("1-1", "delta", { p: 0, text: "B" }, INV2)]);
  assert.equal(chuDaViet(kho[INV].traLoi), "A");
  assert.equal(chuDaViet(kho[INV2].traLoi), "B");
  for (let i = 0; i < SO_LUOT_TOI_DA + 3; i++) {
    kho = gop([khung(`9-${i}`, "trang_thai", { cau: "dang_doc" }, `0b8f1c9e-aaaa-4bbb-8ccc-ddddeeee99${String(i).padStart(2, "0")}`)], kho);
  }
  assert.equal(Object.keys(kho).length, SO_LUOT_TOI_DA);
});

/* ------------------------------------------------ nối lại ---------------- */

test("nối lại: kho làm lại từ rỗng, phát lại gộp của máy chủ cho đúng chữ một lần", () => {
  // Before the drop: three deltas live.
  let kho = gop([khung("5-0", "trang_thai", { cau: "dang_doc" }), khung("5-1", "delta", { p: 0, text: "Tối " }), khung("5-2", "delta", { p: 0, text: "nay " })]);
  // The new socket (useChatChanges calls moi): start over, then the server's
  // replay -- the status and ONE delta with the text so far, under the id of
  // the last merged entry -- then live.
  kho = gop([khung("5-0", "trang_thai", { cau: "dang_doc" }), khung("5-3", "delta", { p: 0, text: "Tối nay đi " }), khung("5-4", "delta", { p: 0, text: "đâu?" })], KHO_PHONG_TRONG);
  assert.equal(chuDaViet(kho[INV].traLoi), "Tối nay đi đâu?");
});

/* ------------------------------------------------ dọn -------------------- */

test("dọn: lỗi và huỷ bỏ ngay, xong chờ thẻ một lúc rồi bỏ, im 30 s thì bỏ", () => {
  let kho = gop([khung("1-0", "trang_thai", { cau: "dang_doc" })]);
  kho = gop([khung("1-1", "that_bai", { code: "ai_tu_choi" })], kho);
  assert.deepEqual(Object.keys(donPhong(kho, 1001)), [], "thất bại: người xem khác không đọc mã");
  kho = gop([khung("2-0", "huy", {}, INV2)]);
  assert.deepEqual(Object.keys(donPhong(kho, 1001)), []);
  kho = gop([khung("3-0", "delta", { p: 0, text: "x" }), khung("3-1", "xong", { message_id: THE })]);
  assert.equal(donPhong(kho, 1000 + GIU_SAU_XONG_MS), kho, "còn chờ thẻ tới");
  assert.deepEqual(Object.keys(donPhong(kho, 1001 + GIU_SAU_XONG_MS)), []);
  kho = gop([khung("4-0", "delta", { p: 0, text: "x" })]);
  assert.equal(donPhong(kho, 1000 + IM_LANG_TOI_DA_MS), kho);
  assert.deepEqual(Object.keys(donPhong(kho, 1001 + IM_LANG_TOI_DA_MS)), [], "im lặng quá 30 s");
});

/* ------------------------------------------------ hàng của người xem ---- */

test("hàng của người xem khác: câu riêng, nhường chỗ khi thẻ tới, không vẽ câu của chính mình", () => {
  const kho = gop([khung("1-0", "trang_thai", { cau: "dang_doc" }), khung("1-1", "trang_thai", { cau: "dang_doc" }, INV2, { tin: null })]);
  const cuaAi = luotChoNguoiXem(kho, () => false);
  assert.deepEqual(cuaAi.map((l) => l.inv), [INV], "câu trả lời ngoài luồng (không có tin) không có hàng");
  assert.deepEqual(luotChoNguoiXem(kho, (inv) => inv === INV), [], "câu trả lời của chính mình để hàng người hỏi vẽ");
  const l = cuaAi[0];
  const req = loiGoiCuaPhong(l.inv, l.tin, l.soTin);
  const nghi = hangTraLoiSong(req, l.traLoi, "", () => false, "thanh_vien");
  assert.deepEqual(nghi, { kieu: "nghi", tieuDe: "Rủ Đi AI đang đọc 6 tin…", cau: CAU_CHO_TRA_LOI.thanh_vien });
  assert.notEqual(CAU_CHO_TRA_LOI.thanh_vien, CAU_CHO_TRA_LOI.nguoi_hoi);
  assert.equal(hangTraLoiSong(req, l.traLoi, "", () => false).cau, CAU_CHO_TRA_LOI.nguoi_hoi, "mặc định vẫn là người hỏi");
  const xong = gop([khung("2-0", "delta", { p: 0, text: "Đi hồ." }), khung("2-1", "xong", { message_id: THE })], kho)[INV];
  assert.deepEqual(hangTraLoiSong(req, xong.traLoi, "Đi hồ.", () => false, "thanh_vien"), { kieu: "viet", chu: "Đi hồ." }, "chưa có thẻ: chữ đứng yên");
  assert.deepEqual(hangTraLoiSong(req, xong.traLoi, "Đi hồ.", (id) => id === THE, "thanh_vien"), { kieu: "an" }, "thẻ tới: hàng nhường chỗ");
});

test("Reduce Motion: người xem khác cũng nhận chữ theo từng câu", () => {
  const kho = gop([khung("1-0", "delta", { p: 0, text: "Hồ Xuân Hương lên đèn từ 18 giờ. Đi bộ một vòng mất" })]);
  assert.equal(chuHienThi(kho[INV].traLoi, true), "Hồ Xuân Hương lên đèn từ 18 giờ.");
  assert.equal(chuHienThi(kho[INV].traLoi, false), "Hồ Xuân Hương lên đèn từ 18 giờ. Đi bộ một vòng mất");
});
