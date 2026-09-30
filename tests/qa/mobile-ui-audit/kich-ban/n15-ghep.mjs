/* N15's captures, side by side: one composite per finding or per state
 * checked, from the frames n15-nhat-ky.mjs took. Then every ledger row whose
 * frame sits in a composite gets that composite too, so the matrix links a row
 * to a committed image (the single frames stay outside git). Same shape as
 * n14-ghep.mjs. Annotated frames (`-ct`) carry the red box of the capture.
 *
 *   source audit-env-main.sh && node kich-ban/n15-ghep.mjs
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
  ["EV-N15-VAO-ghep", "N15 · lối vào: «Giữ lại cuộc đi» ở màn kèo; kệ «Những ngày muốn giữ» trống (UI-153) và có sổ", [
    ["EV-N15-VAO-KEO-C1-ct", "chat-0 · màn kèo: «Giữ lại cuộc đi»"],
    ["EV-N15-TUONG-RONG-C1", "UI-153 · chat-2, kệ trống: một câu, không lối"],
    ["EV-N15-TUONG-RONG-C2", "C2 320"],
    ["EV-N15-TUONG-RONG-C3", "C3 360 tối"],
    ["EV-N15-TUONG-CO-SO-C1", "chat-0 · kệ có sổ"],
  ]],
  ["EV-N15-KHEP-ghep", "N15 · lời mời khép cuộc đi (chat-0, người tổ chức) ở năm cấu hình", [
    ["EV-N15-KHEP-C1", "C1 390"],
    ["EV-N15-KHEP-C2", "C2 320"],
    ["EV-N15-KHEP-C3", "C3 360 tối"],
    ["EV-N15-KHEP-C8", "C8 390×460"],
    ["EV-N15-KHEP-C6", "C6 768"],
  ]],
  ["EV-N15-KHEP-LOI-ghep", "UI-152, UI-151, UI-019 · màn khép cuộc đi: thành viên, kèo chưa tới ngày, kèo không dành cho mình", [
    ["EV-N15-KHEP-THANH-VIEN-C1", "UI-152 · chat-1: bộ chọn loại vẫn hiện"],
    ["EV-N15-KHEP-THANH-VIEN-DOI-C1", "chạm «Khoảnh khắc»: đổi tiêu đề, không tác dụng"],
    ["EV-N15-KHEP-SAP-TOI-C1", "UI-151 · kèo 24–25/10 mở bằng link: vẫn mời khép"],
    ["EV-N15-KHEP-SAP-TOI-LOI-C1", "chạm «Khép cuộc đi»: 409, «Mình thử lại nhé»"],
    ["EV-N15-KHEP-KHONG-CO-C1", "UI-019 · kèo không có: «Cập nhật app…»"],
  ]],
  ["EV-N15-NGUON-ghep", "N15 · «Mang theo điều gì vào sổ?» (chat-0, sau khi khép) ở năm cấu hình", [
    ["EV-N15-NGUON-C1", "C1 390"],
    ["EV-N15-NGUON-C2", "C2 320"],
    ["EV-N15-NGUON-C3", "C3 360 tối"],
    ["EV-N15-NGUON-C8", "C8 390×460"],
    ["EV-N15-NGUON-C6", "C6 768"],
  ]],
  ["EV-N15-DUNG-ghep", "N15 · dựng sổ: cùng Nếp khi không có khoá AI, và tự xếp trang (C1, C9)", [
    ["EV-N15-DUNG-AI-CHO-C1", "Nếp đang dựng; có «Quay lại»"],
    ["EV-N15-DUNG-AI-C1", "sau 5,7 s: câu lỗi, còn lối tự xếp"],
    ["EV-N15-DUNG-TAY-C1", "tự xếp trang: sổ nháp sau 270 ms"],
    ["EV-N15-DUNG-TAY-C9", "C9: hiện ngay, không lật"],
  ]],
  ["EV-N15-SUA-ghep", "N15 · sửa sổ; UI-154 tên trang «2026-09-29»; sheet ảnh bìa ở C8 (UI-040), C6 (UI-093)", [
    ["EV-N15-SUA-C1", "chế độ sửa"],
    ["EV-N15-TRANG-TEN-C1-ct", "UI-154 · ô «Tên trang»: «2026-09-29»"],
    ["EV-N15-BIA-SHEET-C1", "sheet «Chọn ảnh cho trang»"],
    ["EV-N15-BIA-SHEET-C8", "C8: sheet 96% cửa sổ"],
    ["EV-N15-BIA-SHEET-C6", "C6: sheet rộng 768"],
    ["EV-N15-TAI-ANH-C1", "ảnh từ máy thành ô thứ tư"],
  ]],
  ["EV-N15-LUU-ghep", "UI-150, UI-097 · lưu sổ: lỗi lưu ngoài khung nhìn; rời màn mất chỗ sửa; lưu được thì về tường", [
    ["EV-N15-LUU-LOI-C1", "UI-150 · PUT 503: màn không đổi, câu lỗi ở y −1139"],
    ["EV-N15-LUU-C1", "lưu riêng tư"],
    ["EV-N15-LUU-TUONG-C1", "tường có sổ «Chỉ mình tôi»"],
    ["EV-N15-ROI-TRUOC-C1", "UI-097 · đang sửa tên"],
    ["EV-N15-ROI-SAU-C1", "«Quay lại» rồi mở lại: tên sửa mất"],
  ]],
  ["EV-N15-DOC-ghep", "N15 · màn đọc sổ (chủ sổ) C1–C3; người ngoài nhóm gặp sổ riêng tư (UI-100)", [
    ["EV-N15-DOC-C1", "C1 390"],
    ["EV-N15-DOC-C2", "C2 320"],
    ["EV-N15-DOC-C3", "C3 360 tối"],
    ["EV-N15-DOC-NGOAI-RIENG-C1", "dalat-0: không lộ gì; «Thử lại» vô ích"],
  ]],
  ["EV-N15-CONG-KHAI-ghep", "N15 · công khai, gửi cộng đồng, cất về riêng tư (dalat-0 ở ngoài nhóm)", [
    ["EV-N15-CONG-KHAI-CANH-C1", "«Công khai»: câu cảnh báo"],
    ["EV-N15-CONG-KHAI-C1", "đã đăng công khai"],
    ["EV-N15-DOC-NGOAI-CONG-KHAI-C1", "dalat-0 đọc được"],
    ["EV-N15-GUI-CONG-DONG-C1", "bài cộng đồng chờ duyệt; UI-154 «2026-09-29»"],
    ["EV-N15-CAT-RIENG-C1", "cất về riêng tư: màn chủ sổ «Chỉ mình tôi»; dalat-0 nhận 404 (API)"],
  ]],
  ["EV-N15-HEP-ghep", "N15 · màn hẹp: sửa sổ ở C2 320 và C3 360 tối (đầu màn, trang đầu), ô chữ cao 44 (UI-001); kệ có sổ ở C2, C3", [
    ["EV-N15-SUA-BASE-C2", "C2 · chế độ sửa, đầu màn"],
    ["EV-N15-SUA-BASE-TRANG-C2", "C2 · trang đầu"],
    ["EV-N15-SUA-BASE-C3", "C3 tối · đầu màn"],
    ["EV-N15-SUA-BASE-TRANG-C3", "C3 tối · trang đầu"],
    ["EV-N15-TUONG-CO-SO-C2", "C2 · kệ có sổ"],
    ["EV-N15-TUONG-CO-SO-C3", "C3 tối · kệ có sổ"],
  ]],
  ["EV-N15-XOA-ghep", "N15 · thành viên giữ sổ rồi xoá từ link lạnh; UI-121 link sổ khi chưa đăng nhập", [
    ["EV-N15-THANH-VIEN-GIU-C1", "chat-1 giữ sổ riêng"],
    ["EV-N15-XOA-HOI-C1", "«Xóa cuốn sổ»: sheet hỏi"],
    ["EV-N15-XOA-C1", "xoá xong: về Cá nhân"],
    ["EV-N15-KHONG-PHIEN-SO-C1", "UI-121 · không phiên: về Welcome"],
    ["EV-N15-KHONG-PHIEN-SAU-DANG-NHAP-C1", "đăng nhập xong: Khám phá"],
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
  if (r.feature !== "N15") continue;
  const them = [...new Set((r.evidence ?? []).flatMap((e) => khungCua.get(e) ?? []))].filter((g) => !(r.evidence ?? []).includes(g));
  if (!them.length) continue;
  const { luc: _luc, ...giu } = r;
  so.ghi({ ...giu, evidence: [...(r.evidence ?? []), ...them] });
  gan++;
}
console.log(`n15-ghep: ${GHEP.length} ảnh ghép; gắn vào ${gan} hàng`);
