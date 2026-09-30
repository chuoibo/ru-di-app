/* N14's captures, side by side: one composite per finding or per state
 * checked, from the frames n14-cong-dong.mjs took. Then every ledger row
 * whose frame sits in a composite gets that composite too, so the matrix links
 * a row to a committed image (the single frames stay outside git). Same shape
 * as n26-ghep.mjs. Annotated frames (`-ct`) carry the red box of the capture.
 *
 *   source audit-env-main.sh && node kich-ban/n14-ghep.mjs
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
  ["EV-N14-RONG-ghep", "UI-132 · tab Cộng đồng khi chưa có bài công khai nào được duyệt; trạng thái rỗng chỉ tới sau bài duyệt đầu tiên", [
    ["EV-N14-RONG-C1", "chat-0 · C1: câu lỗi và «Thử lại» thay trạng thái rỗng"],
    ["EV-N14-RONG-C2", "C2 320"],
    ["EV-N14-RONG-THINH-HANH-C1", "«Thịnh hành»: cùng lỗi"],
    ["EV-N14-RONG-THEO-DOI-C1", "«Đang theo dõi»: trạng thái rỗng đúng"],
    ["EV-N14-RONG-DE-SAU-C1", "chat-3 chọn «Để sau»: hộp mời mất, lỗi còn"],
    ["EV-N14-SAU-DUYET-C1", "sau khi duyệt bài đầu tiên: hết lỗi"],
  ]],
  ["EV-N14-DANG-ghep", "UI-133, UI-091 · form «Kể một khoảnh khắc» (chat-0), C1", [
    ["EV-N14-DANG-C1", "form trống: nút gửi viền đứt, không lý do"],
    ["EV-N14-DANG-NHOM-C1", "«Một nhóm»: nút gửi tắt, không lý do"],
    ["EV-N14-DANG-6-CHU-DE-C1", "UI-133 · sáu chủ đề → «Gửi lên cộng đồng»: màn không đổi gì"],
    ["EV-N14-DANG-6-CHU-DE-LOI-C1-ct", "cuộn xuống mới thấy câu lỗi: đổ lỗi cho app"],
  ]],
  ["EV-N14-FORM-ghep", "N14 · form «Kể một khoảnh khắc» trống ở năm cấu hình: nút gửi luôn trong cửa sổ", [
    ["EV-N14-DANG-C1", "C1 390 (chat-0)"],
    ["EV-N14-DANG-C2", "C2 320"],
    ["EV-N14-DANG-C3", "C3 360 tối (chat-1)"],
    ["EV-N14-DANG-C8", "C8 390×460, đã gõ một dòng"],
    ["EV-N14-DANG-C6", "C6 768"],
  ]],
  ["EV-N14-BANG-ghep", "N14 · bảng tin có bài (chat-1), C1–C3; UI-143 ở 320dp", [
    ["EV-N14-BANG-C1", "C1 390"],
    ["EV-N14-BANG-C2", "C2 320"],
    ["EV-N14-BANG-C3", "C3 360 tối"],
    ["EV-N14-BANG-ANH-C2-ct", "UI-143 · C2: khung ảnh 296 trong vùng 288, mép phải bị cắt"],
    ["EV-N14-WS-DAI-C2-ct", "UI-143 · C2: dải «Bảng tin có cập nhật» đè lên tab 13px"],
  ]],
  ["EV-N14-DOC-ghep", "UI-135, UI-134 · chỗ đang đọc và chữ đang gõ không giữ được, C1", [
    ["EV-N14-BANG-CUON-TRUOC-C1", "UI-135 · đang đọc hết B2"],
    ["EV-N14-BANG-CUON-SAU-C1", "mở bài rồi «Quay lại»: về đầu, B2 gập lại"],
    ["EV-N14-WS-MAT-CHU-TRUOC-C1", "UI-134 · đang gõ bình luận ở chi tiết bài"],
    ["EV-N14-WS-MAT-CHU-SAU-C1", "stream nối lại: ô trống, màn về đầu"],
  ]],
  ["EV-N14-THE-ghep", "UI-136, UI-094, UI-140, UI-141 · thao tác trên thẻ bài (chat-1, chat-2), C1", [
    ["EV-N14-BANG-CHIA-SE-C1", "UI-136 · sau khi chạm «Chia sẻ»: không gì đổi"],
    ["EV-N14-BANG-XEM-ANH-C1", "UI-094 · trình xem ảnh: vùng ảnh trống"],
    ["EV-N14-BANG-THEO-DOI-C1-ct", "UI-140 · bỏ theo dõi ở B1, B2 vẫn «Bỏ theo dõi tác giả»"],
    ["EV-N14-BANG-AN-C1", "UI-141 · «Không quan tâm»: B2 biến mất, không câu, không hoàn tác"],
  ]],
  ["EV-N14-CHI-TIET-ghep", "N14 · chi tiết bài (chat-0) C1–C3; UI-138 Nếp; UI-142 bài sửa rời bảng tin của tác giả", [
    ["EV-N14-CHI-TIET-C1", "C1: chi tiết B2"],
    ["EV-N14-CHI-TIET-C2", "C2"],
    ["EV-N14-CHI-TIET-C3", "C3 tối"],
    ["EV-N14-NEP-LOI-C1", "UI-138 · Nếp lỗi 503: không câu nào trong sheet"],
    ["EV-N14-SUA-BANG-TIN-C1", "UI-142 · sau khi sửa B1: bảng tin của chat-0 không còn B1"],
  ]],
  ["EV-N14-BL-ghep", "UI-096, UI-040 · bình luận (chat-1) và sheet ở cửa sổ thấp", [
    ["EV-N14-BL-GUI-C1", "C1: bình luận chờ duyệt «Chỉ bạn thấy»"],
    ["EV-N14-BL-XOA-TRUOC-C1", "UI-096 · bình luận thử, trước khi chạm «Xóa»"],
    ["EV-N14-BL-XOA-C1", "một chạm: mất ngay, không hỏi"],
    ["EV-N14-BL-SHEET-C8", "UI-040 · C8: sheet «Bình luận» 96%, đầu sheet ngoài cửa sổ"],
    ["EV-N14-BANG-CAI-DAT-C8", "UI-040 · C8: sheet «Bảng tin của bạn», đỉnh y −55"],
  ]],
  ["EV-N14-DUYET-ghep", "N14 · hàng duyệt /community/review; UI-144 mã trạng thái thô, UI-091 nút tắt không lý do", [
    ["EV-N14-DUYET-KHONG-QUYEN-C1", "chat-16 không vai trò: câu từ chối"],
    ["EV-N14-DUYET-C1", "chat-15 · C1: «· pending», hai nút tắt không lý do"],
    ["EV-N14-DUYET-C2", "C2"],
    ["EV-N14-DUYET-C3", "C3 tối"],
  ]],
  ["EV-N14-PHU-ghep", "UI-137, UI-145, UI-146, UI-147, UI-148, UI-095 · màn trong và màn phụ, C1", [
    ["EV-N14-KHONG-PHIEN-BAI-C1", "UI-137 · không phiên, link bài: chờ mãi"],
    ["EV-N14-KHONG-PHIEN-VIET-C1", "UI-137 · không phiên, form viết: không lối đăng nhập"],
    ["EV-N14-TIM-RONG-C1", "UI-145 · tìm không ra: hai tiêu đề trống"],
    ["EV-N14-GIU-C1", "UI-145 · «Điều mình muốn giữ» trống"],
    ["EV-N14-CHU-DE-C1", "UI-146 · trang chủ đề: không «Quay lại»"],
    ["EV-N14-THONG-BAO-C1", "UI-147 · thông báo không nói ai nhắc"],
    ["EV-N14-LOI-BL-C1", "UI-148 · đọc bình luận lỗi: không «Thử lại»"],
    ["EV-N14-LOI-THICH-C1", "UI-095 · thích lỗi: câu ở y −742"],
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
  if (r.feature !== "N14") continue;
  const them = [...new Set((r.evidence ?? []).flatMap((e) => khungCua.get(e) ?? []))].filter((g) => !(r.evidence ?? []).includes(g));
  if (!them.length) continue;
  const { luc: _luc, ...giu } = r;
  so.ghi({ ...giu, evidence: [...(r.evidence ?? []), ...them] });
  gan++;
}
console.log(`n14-ghep: ${GHEP.length} ảnh ghép; gắn vào ${gan} hàng`);
