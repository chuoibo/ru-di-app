/* The retest's captures, side by side: one composite per feature, from the
 * frames retest-main.mjs (and the base scripts re-run on main) already took.
 * Then every ledger row whose frame sits in a composite gets that composite
 * too, so the matrix links a row to a committed image (the single frames
 * stay outside git). Same shape as the «e5-ghep» part of e-luong.mjs and the
 * GHEP map of e-phan-xu.mjs.
 *
 *   source audit-env-main.sh && node kich-ban/retest-ghep.mjs
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
  ["EV-R-F00-ghep", "Retest trên main, F00 vỏ: vạch rail, khay «Tạo mới» mở từ Lên plan, bảng Nếp, C1 và C6", [
    ["EV-R-UI-004-C6", "UI-004 · C6 Cá nhân: vạch lệch khỏi tab"],
    ["EV-R-UI-005-C1", "UI-005 · Back khi khay mở: trông bình thường, còn vùng inert"],
    ["EV-R-UI-006-C1", "UI-006 · chạm «Tạo mới» hai lần: khay đóng"],
    ["EV-R-UI-011-C1", "UI-011 · «Vẽ» không phản hồi"],
  ]],
  ["EV-R-F01-ghep", "Retest trên main, F01 vào cửa: pager Welcome, «Quay lại» khi mở lạnh, mã lời mời sai, C1", [
    ["EV-R-UI-016-C1", "UI-016 · sau ba lần vuốt, chấm vẫn ở trang 1"],
    ["EV-R-UI-018-C1", "UI-018 · /login mở lạnh, «Quay lại» đứng yên"],
    ["EV-R-UI-019-C1", "UI-019 · mã sai → «Cập nhật app»"],
  ]],
  ["EV-R-F02-ghep", "Retest trên main, F02 Khám phá: dòng giá, «Chỉ đường», nhãn «Lưu địa điểm», nút ✦", [
    ["EV-R-UI-021-C1", "UI-021 · C1: dòng giá bị cắt"],
    ["EV-R-UI-022-C1", "UI-022 · «Chỉ đường» không mở gì"],
    ["EV-R-UI-023-C2", "UI-023 · C2: «Lưu địa điểm» bị cắt"],
    ["EV-R-UI-024-C1", "UI-024 · ✦ → «0 kết quả»"],
  ]],
  ["EV-R-F03-ghep", "Retest trên main, F03 kèo: Bản đồ nhiều ngày, ngày trống, ngân sách trống, timeline demo, thứ tự chặng, C1", [
    ["EV-R-UI-032-C1", "UI-032 · Bản đồ: 0 mốc"],
    ["EV-R-UI-033-C1", "UI-033 · «Về Lịch trình» đã thấy; lời giải thích bị cắt"],
    ["EV-R-UI-034-C1", "UI-034 · ô trống vẫn hiện «250000»"],
    ["EV-R-UI-035-C1", "UI-035 · có phiên vẫn ra timeline demo"],
    ["EV-R-UI-036-C1", "UI-036 · tay nắm không nhận bàn phím"],
  ]],
  ["EV-R-F04-ghep", "Retest trên main, F04 tiền (kịch bản f04-tien.mjs chạy lại): tiền bị cắt, gửi link, bàn 20 người, lý do chặn, quay về bước 1", [
    ["EV-F04-TIEN-CAT-B2-C2-ct", "UI-048 · C2: số tiền bị cắt"],
    ["EV-F04-CHIA-SE-KHONG-CO-C1", "UI-049 · «Gửi cho …»: câu lỗi ở đầu trang, ngoài màn"],
    ["EV-F04-BAN-20-C2-ct", "UI-050 · C2: chạm ghế trúng ghế khác"],
    ["EV-F04-CHAN-TEN-C1-ct", "UI-051 · lý do chặn ngoài khung nhìn"],
    ["EV-F04-LUI-MAT-C1", "UI-052 · về bước 1 → «Nhập tay» ra bill trống"],
  ]],
  ["EV-R-F05-F06-ghep", "Retest trên main, F05 chat và F06 nhóm: ô soạn, link dài, sau khi lập nhóm, người được mời, tự bỏ quyền", [
    ["EV-F05-SOAN-rong-C1", "UI-062 · ô soạn: chữ lệch 20px"],
    ["EV-F05-URL-C2", "UI-063 · C2: bong bóng tràn mép trái"],
    ["EV-F06-TAO-SAU-C1", "UI-071 · lập nhóm xong về Khám phá"],
    ["EV-R-UI-073-B-C1", "UI-073 · người được mời: không qua Sở thích"],
    ["EV-R-UI-073-C-C1", "UI-073 · người lạ tra số thấy tên người mời đặt"],
    ["EV-R-UI-074-C1", "UI-074 · một chạm là mất quyền quản trị"],
  ]],
  ["EV-R-F07-ghep", "Retest trên main, F07 sổ hai người: link không phiên, đọc sổ 503, sheet «Lập sổ» sau khi đồng ý, «Rủ … tới đây» khi tuần đã chốt, C1", [
    ["EV-R-UI-082-A-C1", "UI-082 · không phiên: sổ demo dưới id cặp thật"],
    ["EV-R-UI-083-C1", "UI-083 · 503: vẽ như sổ chưa lập"],
    ["EV-R-UI-084-A-C1", "UI-084 · sổ đã mở, sheet quay về «Đề nghị lập sổ»"],
    ["EV-R-UI-085-C1", "UI-085 · tờ phác mới cùng tuần, câu nói ngược"],
  ]],
  ["EV-R-E5-ghep", "Retest trên main, E5: chat-8 chặn chat-9, chat-9 vẫn gửi được tờ hẹn, chat-8 nhận, C1", [
    ["EV-R-UI-120-B-C1", "UI-120 · người bị chặn: «Đã gửi, chờ trả lời»"],
    ["EV-R-UI-120-A-C1", "UI-120 · người chặn: nhận tờ, «Ừ, hẹn»"],
  ]],
  ["EV-R-F08-F09-ghep", "Retest trên main, F08 kỷ niệm và F09 cài đặt: trình xem ảnh, tim lỗi, bình luận bài, nháp mất, công tắc lỗi, C1", [
    ["EV-R-UI-094-MO-C1", "UI-094 · trình xem mở: «1 / 3», không thấy ảnh"],
    ["EV-R-UI-095-C1", "UI-095 · tim lỗi: câu ở đầu tường, ngoài màn"],
    ["EV-R-UI-096-C1", "UI-096 · đổi: bình luận không còn nút xoá"],
    ["EV-R-UI-097-C1", "UI-097 · mở lại «Thả khoảnh khắc»: khung trống"],
    ["EV-R-UI-107-C1", "UI-107 · công tắc lỗi: không thấy câu"],
  ]],
  ["EV-R-F10-F11-E-ghep", "Retest trên main, bảng dev, demo và luồng E: tim cặp so sánh ở 320dp, bong bóng demo, tab bị khoá sau Back, Tin nhắn demo, album sau «Tôi đã tới»", [
    ["EV-R-UI-113-C2", "UI-113 · C2: tim «Still Cafe» ở mép"],
    ["EV-R-UI-116-C2", "UI-116 · C2: bong bóng tràn mép phải"],
    ["EV-R-UI-117-B-C1", "UI-117 · Back về Cộng đồng: thanh tab bị khoá"],
    ["EV-R-UI-082-F11-C1", "UI-082 · Tin nhắn demo: không nhãn"],
    ["EV-R-UI-119-C1", "UI-119 · album: «0 chỗ đã tới»"],
  ]],
];

const browser = await moTrinhDuyet();
try {
  for (const [id, tieuDe, khung] of GHEP) {
    const thieu = khung.filter(([f]) => !existsSync(jpg(f))).map(([f]) => f);
    if (thieu.length) throw new Error(`${id}: thiếu ảnh ${thieu.join(", ")}`);
    const kq = await ghepAnh(browser, khung.map(([f, nhan]) => ({ file: jpg(f), nhan })), jpg(id), { cao: 760, tieuDe });
    console.log(`${id}: ${khung.length} khung, ${kq.bytes} byte`);
  }
} finally {
  await browser.close();
}

// Every row naming a frame of a composite gets the composite as well. The
// ledger is read after the writes above, and a row that already names its
// composite is left alone, so a second run adds nothing.
const khungCua = new Map();
for (const [id, , khung] of GHEP) for (const [f] of khung) khungCua.set(f.replace(/-ct$/, ""), [...(khungCua.get(f.replace(/-ct$/, "")) ?? []), id]);
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
  const them = [...new Set((r.evidence ?? []).flatMap((e) => khungCua.get(e) ?? []))].filter((g) => !(r.evidence ?? []).includes(g));
  if (!them.length) continue;
  const { luc: _luc, ...giu } = r;
  so.ghi({ ...giu, evidence: [...(r.evidence ?? []), ...them] });
  gan++;
}
console.log(`retest-ghep: ${GHEP.length} ảnh ghép; gắn vào ${gan} hàng`);
