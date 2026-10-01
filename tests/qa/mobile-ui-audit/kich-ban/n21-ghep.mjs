/* N21's captures, side by side: one composite per finding or per state
 * checked, from the frames n21-ho-so.mjs took. Then every ledger row whose
 * frame sits in a composite gets that composite too, so the matrix links a row
 * to a committed image (the single frames stay outside git). Same shape as
 * n15-ghep.mjs. Annotated frames (`-ct`) carry the red box of the capture.
 *
 *   source audit-env-main.sh && node kich-ban/n21-ghep.mjs
 */
import { existsSync } from "node:fs";
import { join } from "node:path";

import { ghepAnh } from "../thu-vien/chup.mjs";
import { soGhi } from "../thu-vien/ghi.mjs";
import { moTrinhDuyet } from "../thu-vien/trinh-duyet.mjs";

const out = process.env.AUDIT_OUT;
if (!out) throw new Error("thiếu AUDIT_OUT");
const so = soGhi(out);
const jpg = (id) => join(out, "jpg", `${id}.jpg`);

const GHEP = [
  ["EV-N21-HT-ghep", "N21 · sổ hành trình của chat-0 (3 huy hiệu mở đầu, chưa có kết) ở C1, C2, C3, C6; ở C6 thẻ trải 720 trong khi bản đồ giữ 560 (UI-093)", [
    ["EV-N21-HT-C1", "C1 390 · đầu sổ («MỚI MỞ» của máy mới, UI-160)"],
    ["EV-N21-HT-DUOI-C1", "C1 · thẻ ngã rẽ, «sắp dùng được»"],
    ["EV-N21-HT-C2", "C2 320"],
    ["EV-N21-HT-C3", "C3 360 tối"],
    ["EV-N21-HT-C6", "C6 768 · thẻ 720, bản đồ 560"],
  ]],
  ["EV-N21-VAO-ghep", "N21 · lối vào: thẻ trên tab Cá nhân (C1–C3); mục «Thành tích · Cấp và huy hiệu…» mở màn «Hành trình» (UI-162); không phiên: «Thành tích» bản cũ, không lối đăng nhập (UI-082)", [
    ["EV-N21-CA-NHAN-C1", "C1 · thẻ «Dấu mốc mới»"],
    ["EV-N21-CA-NHAN-C2", "C2 320"],
    ["EV-N21-CA-NHAN-C3", "C3 360 tối"],
    ["EV-N21-CA-NHAN-MENU-C1", "UI-162 · mục «Thành tích», màn tới tên «Hành trình»"],
    ["EV-N21-KHONG-PHIEN-C1", "UI-082 · không phiên: bản cũ, nhãn «Demo», không «Đăng nhập»"],
  ]],
  ["EV-N21-SO-ghep", "N21 · khoảnh khắc «MỚI MỞ», trưng bày huy hiệu, lỗi thao tác của sổ (C1)", [
    ["EV-N21-MOI-MO-C1", "UI-160 · máy mới: «MỚI MỞ» huy hiệu đạt từ 29/09"],
    ["EV-N21-KET-NHAN-C1", "nhận kết: «MỚI MỞ Chuyện mình kể» (M8)"],
    ["EV-N21-KET-M8-C1", "tải lại: khoảnh khắc không lặp"],
    ["EV-N21-TRUNG-BAY-4-C1", "UI-161 · chạm huy hiệu thứ tư: không gì đổi"],
    ["EV-N21-LOI-CHON-C1", "UI-157 · 503 khi chọn lối: câu lỗi ở y 1268, ngoài khung"],
  ]],
  ["EV-N21-NEP-ghep", "N21 · chọn lối và Nếp (C1): tuyến Ngã rẽ, chọn một ngã rẽ, bản xem trước cho Nếp, sau khi đồng ý; lỗi đọc sổ", [
    ["EV-N21-HT-NGA-RE-C1", "tuyến Ngã rẽ: cần một kết trước"],
    ["EV-N21-KET-CHON-C1", "chọn «Chuyện mình kể»: tuyến Kỷ niệm, 3/3 ngày có bài kể"],
    ["EV-N21-NEP-XEM-C1", "«Xem Nếp sẽ nhận gì»: chỉ số đếm và mã"],
    ["EV-N21-NEP-C1", "sau «Đồng ý»: thân {consent:true}, sổ tự gợi ý"],
    ["EV-N21-LOI-DOC-C1", "503 khi đọc sổ: câu lỗi và «Thử lại»"],
  ]],
  ["EV-N21-XEM-ghep", "N21 · hồ sơ chat-0 qua mắt người khác: bạn (C1–C3), cùng nhóm, người lạ; bài Cộng đồng có ảnh lên tường không ảnh (UI-156)", [
    ["EV-N21-XEM-CHAT-1-C1", "bạn · C1: ba huy hiệu trưng bày"],
    ["EV-N21-XEM-CHAT-1-C2", "C2 320"],
    ["EV-N21-XEM-CHAT-1-C3", "C3 360 tối"],
    ["EV-N21-XEM-CHAT-16-C1", "cùng nhóm, không là bạn"],
    ["EV-N21-XEM-DALAT-0-C1", "người lạ: không xem được"],
    ["EV-N21-TUONG-B2-C1", "UI-156 · thẻ bài Cộng đồng B2: không ảnh"],
  ]],
  ["EV-N21-BAI-ghep", "N21 · trang viết cũ: bình luận gốc và một tầng trả lời (C1–C3), không lối xoá (UI-158); ảnh toàn màn kèm khay (C1; C8 khay 96%, UI-040)", [
    ["EV-N21-BL-MOT-TANG-C1", "C1 · lời đáp lùi một bậc; chỉ «Thích», «Trả lời»"],
    ["EV-N21-BAI-C2", "C2 320 · đầu bài"],
    ["EV-N21-BAI-BL-C3", "C3 tối · bình luận"],
    ["EV-N21-ANH-C1", "ảnh toàn màn, khay «Lời nhắn dưới ảnh»"],
    ["EV-N21-ANH-C8", "UI-040 · C8: khay 441 trên cửa sổ 460"],
  ]],
  ["EV-N21-TUONG-ghep", "N21 · tường: đăng lại (câu xác nhận ngoài khung, UI-159), quyền xem bài gốc; trang 2 bị cắt về 20 bài khi long poll trả về (UI-155)", [
    ["EV-N21-CHIA-SE-C1", "khay «Chia sẻ bài»"],
    ["EV-N21-DANG-LAI-C1", "UI-159 · sau «Bạn bè của tôi»: câu ở y 1219"],
    ["EV-N21-DANG-LAI-ACL-C1", "người không xem được bài gốc"],
    ["EV-N21-TRANG-2-C1", "đã mở trang 2: 23 bài"],
    ["EV-N21-TRANG-SAU-SU-KIEN-C1", "UI-155 · 517 ms sau một sự kiện: 20 bài"],
  ]],
];

const browser = await moTrinhDuyet();
try {
  for (const [id, tieuDe, khung] of GHEP) {
    const thieu = khung.filter(([f]) => !existsSync(jpg(f))).map(([f]) => f);
    if (thieu.length) throw new Error(`${id}: thiếu ảnh ${thieu.join(", ")}`);
    const kq = await ghepAnh(browser, khung.map(([f, nhan]) => ({ file: jpg(f), nhan })), jpg(id), { cao: 700, tieuDe });
    console.log(`${id}: ${khung.length} khung, ${kq.bytes} byte`);
  }
} finally {
  await browser.close();
}

// Every row naming a frame of a composite (or its unannotated twin) gets the composite as well;
// a row that already names it is left alone, so a second run adds nothing.
const khungCua = new Map();
for (const [id, , khung] of GHEP)
  for (const [f] of khung) for (const ten of [f, f.replace(/-ct$/, "")]) khungCua.set(ten, [...new Set([...(khungCua.get(ten) ?? []), id])]);
const moiNhat = new Map();
for (const r of so.doc()) {
  if (r.rut) {
    for (const k of [...moiNhat.keys()]) if (k.startsWith(`${r.tc}|`)) moiNhat.delete(k);
    continue;
  }
  moiNhat.set(`${r.tc}|${r.nenTang ?? "web"}|${r.cauHinh ?? ""}`, r);
}
let gan = 0;
for (const r of moiNhat.values()) {
  if (r.feature !== "N21") continue;
  const them = [...new Set((r.evidence ?? []).flatMap((e) => khungCua.get(e) ?? []))].filter((g) => !(r.evidence ?? []).includes(g));
  if (!them.length) continue;
  const { luc: _luc, ...giu } = r;
  so.ghi({ ...giu, evidence: [...(r.evidence ?? []), ...them] });
  gan++;
}
console.log(`n21-ghep: ${GHEP.length} ảnh ghép; gắn vào ${gan} hàng`);
