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
  // Checkpoint retest 2: the P3 issues.
  ["EV-R-P3-F00-F02-ghep", "Retest P3 trên main, F00 và F02: resume chậm, /create mở lạnh, chip Nếp, mép Nếp che chữ, khay ở cửa sổ thấp, Hội An, lỗi 503 và mất mạng", [
    ["EV-F00-RESUME-CHAM-1800ms-C1", "UI-009 · 1,8 s: trống, không chỉ báo"],
    ["EV-R-UI-010-C1", "UI-010 · /create lạnh: không khay"],
    ["EV-R-UI-012-C1", "UI-012 · chip gợi ý 36dp"],
    ["EV-R-UI-014-C2", "UI-014 · C2: mép Nếp che «Vui chơi»"],
    ["EV-R-UI-007-C8", "UI-007 · C8: khay 96%"],
    ["EV-F02-HOI-AN-C1", "UI-028 · Hội An: khuyên bỏ lọc"],
    ["EV-F02-LOI-503-C1", "UI-029 · 503: câu chung"],
    ["EV-F02-OFFLINE-C1", "UI-029/030 · mất mạng: màn lỗi"],
  ]],
  ["EV-R-P3-F03-ghep", "Retest P3 trên main, F03 kèo: Nếp vắng, sheet ở C8, đầu màn không mờ, vé kèo bị cắt, cột tên chặng 53px, /outings/chon kẹt", [
    ["EV-F03-NEP-lich-trinh-C1", "UI-037 · Lịch trình: không có Nếp"],
    ["EV-F03-THEM-C8", "UI-040 · C8: sheet 96%"],
    ["EV-F03-SUA-NGAY-C1", "UI-041 · đầu màn không mờ"],
    ["EV-F03-META-C2-ct", "UI-044 · C2: số chặng bị cắt"],
    ["EV-F03-DAI-lien-C2", "UI-045 · C2: cột tên 53px"],
    ["EV-F03-CHON-thieu-C1", "UI-046 · thiếu ?place: skeleton mãi"],
  ]],
  ["EV-R-P3-F04-ghep", "Retest P3 trên main, F04 tiền: Nếp M2, đọc ảnh hỏng, mép Nếp, đợt rỗng, dòng người trả, Tài chính, quyết toán C2", [
    ["EV-F04-NEP-M2-C2-ct", "UI-055 · C2: Nếp M2 ra ngoài màn"],
    ["EV-F04-ANH-DOC-C1", "UI-056 · đọc hỏng: không «Nhập tay»"],
    ["EV-F04-NEP-MEP-C1-ct", "UI-057 · chạm mép: không gì"],
    ["EV-F04-DOT-RONG-C1", "UI-058 · «Tạo đợt thu» bị từ chối"],
    ["EV-F04-DA-GHI-C1", "UI-059 · «Chat Test 0…» mất «(trả)»"],
    ["EV-R-UI-060-C1", "UI-060 · «Chi theo nhóm» không hàng nào"],
    ["EV-R-UI-061-C2", "UI-061 · C2: câu giải thích 7 dòng"],
  ]],
  ["EV-R-P3-F05-ghep", "Retest P3 trên main, F05 chat: bố cục tin, khay tờ hẹn, báo cáo, 503, thẻ thông báo, dải ghim, chat đôi sau khi chặn, lời mời nhóm", [
    ["EV-F05-BO-CUC-G8-C1-ct", "UI-064/065 · avatar thấp, thẻ bình chọn 39%"],
    ["EV-F05-TO-HEN-C1", "UI-066 · khay tờ hẹn: Esc không đóng"],
    ["EV-R-UI-067-C1", "UI-067 · lý do đang chọn: chỉ đổi nền"],
    ["EV-F05-503-C1-ct", "UI-068 · 503: câu chung, không «Thử lại»"],
    ["EV-F05-THONG-BAO-C1", "UI-069 · thẻ «Đã hiểu» mang dấu AI"],
    ["EV-F05-GHIM-TREN-NEN-C1-ct", "UI-070 · dải ghim sáng trên nền mờ"],
    ["EV-R-UI-079-NGUOI-CHAN-C1", "UI-079 · 20 s: «Đang nối lại»"],
    ["EV-R-UI-080-C1", "UI-080 · lời mời: không tên người mời"],
  ]],
  ["EV-R-P3-F06-ghep", "Retest P3 trên main, F06 nhóm và người: mời thành viên, hai «Người lập nhóm», 19 nút cùng tên, «Đồng ý» lỗi, hồ sơ sau khi chặn", [
    ["EV-R-UI-072-C1", "UI-072 · 409 → «Lần bấm trước…»"],
    ["EV-R-UI-075-C1", "UI-075 · hai «Người lập nhóm»"],
    ["EV-R-UI-076-C1", "UI-076 · «Đặt làm quản trị» ×19"],
    ["EV-R-UI-077-C1", "UI-077 · «Đồng ý» lỗi: cả danh sách thành lỗi"],
    ["EV-R-UI-078-B-C1", "UI-078 · tải lại: mất «Đã chặn»"],
  ]],
  ["EV-R-P3-F07-ghep", "Retest P3 trên main, F07 sổ hai người: đề nghị rồi «Để sau», Cài đặt sổ sau Back, bìa sổ, nút lưu tắt, dải ngày, tablet", [
    ["EV-R-UI-086-C1", "UI-086 · sau «Để sau»: không câu chờ"],
    ["EV-R-UI-087-C1", "UI-087 · Back từ chat: sheet còn mở"],
    ["EV-R-UI-090-C1", "UI-090 · bìa: «Chat Tes…» ×2"],
    ["EV-R-UI-091-C1", "UI-091 · «Lưu điều cần tránh» tắt, không lý do"],
    ["EV-R-UI-092-XA-C1", "UI-092 · ngày xa: lá đang chọn khuất"],
    ["EV-R-UI-093-C6", "UI-093 · C6: tờ và sheet trải hết bề ngang"],
  ]],
  ["EV-R-P3-F08-ghep", "Retest P3 trên main, F08 kỷ niệm: chip Check-in tràn, bài «Chỉ mình tôi», story, ảnh 9:16, kệ album, Thành tích ở 320px", [
    ["EV-R-UI-099-C1", "UI-099 · chip tên dài tràn sheet"],
    ["EV-R-UI-100-C1", "UI-100 · hai khối lỗi, hai «Thử lại»"],
    ["EV-R-UI-101-A-C1", "UI-101 · story: vùng chạm không vai trò"],
    ["EV-R-UI-101-B-C1", "UI-101 · câu hỏi xoá: focus ở nút"],
    ["EV-R-UI-102-XEM-C1", "UI-102 · xem trước: trọn ảnh"],
    ["EV-R-UI-102-TUONG-C1", "UI-102 · trên tường: cắt đầu đuôi"],
    ["EV-R-UI-103-C1", "UI-103 · kệ: chỉ năm"],
    ["EV-R-UI-106-C2", "UI-106 · C2: «châ|n» bị bẻ"],
  ]],
  ["EV-R-P3-F09-F10-E-ghep", "Retest P3 trên main, F09, bảng dev và luồng E: «Đã lưu», xoá tài khoản, tim lồng nút, kèo tạo từ chat, số thành viên", [
    ["EV-R-UI-109-C1", "UI-109 · «Đã lưu»: chỉ con số"],
    ["EV-R-UI-110-C1", "UI-110 · «XOÁ»: nút tắt, không lý do"],
    ["EV-F10-TIM-LONG-C1", "UI-114 · bản dev: nút lồng nút"],
    ["EV-R-UI-118-A-C1", "UI-118 · Back về chat: không dấu kèo"],
    ["EV-R-UI-122-C1", "UI-122 · 15 s: vẫn «2 thành viên»"],
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
