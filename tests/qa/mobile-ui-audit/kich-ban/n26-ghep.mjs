/* N26's captures, side by side: one composite per finding or per state
 * checked, from the frames n26-hai-lop-chat.mjs took. Then every ledger row
 * whose frame sits in a composite gets that composite too, so the matrix links
 * a row to a committed image (the single frames stay outside git). Same shape
 * as retest-ghep.mjs.
 *
 *   source audit-env-main.sh && node kich-ban/n26-ghep.mjs
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
  ["EV-N26-KHAY-ghep", "N26 · khay công cụ của chat hai người: đám bạn bốn công cụ, cặp đôi thêm «Tờ giấy», một hàng từ 320 tới 768", [
    ["EV-N26-BAN-KHAY-C2", "đám bạn · C2 320: bốn ô 66; ô soạn bị đẩy (UI-124)"],
    ["EV-N26-BAN-KHAY-C3", "đám bạn · C3 360 tối"],
    ["EV-N26-DOI-KHAY-C1", "cặp đôi · C1 390: năm ô 65,2"],
    ["EV-N26-DOI-KHAY-C3", "cặp đôi · C3 360 tối: năm ô 59,2"],
    ["EV-N26-DOI-KHAY-C6", "cặp đôi · C6 768: năm ô 140,8"],
  ]],
  ["EV-N26-STICKER-ghep", "N26 · sticker, lệnh và chip của chat hai người, C1", [
    ["EV-N26-BAN-STICKER-C1", "đám bạn: tám sticker"],
    ["EV-N26-DOI-STICKER-C1", "cặp đôi: thêm «Cho hai người»"],
    ["EV-N26-BAN-LENH-C1", "đám bạn · «/»: lệnh viết cho hai bạn"],
    ["EV-N26-BAN-CHIP-C1", "đám bạn · «@Rủ Đi»: AI chưa sẵn sàng"],
  ]],
  ["EV-N26-SOAN-ghep", "UI-124 · chat hai người chưa có tin ở cửa sổ thấp: ô soạn và khay bị đẩy ra ngoài đáy", [
    ["EV-N26-SOAN-doi-C8", "cặp đôi · C8 390×460, khay đóng: không thấy ô soạn"],
    ["EV-N26-SOAN-doi-KHAY-C4", "cặp đôi · C4 375×667, mở khay: ô soạn ngoài màn"],
    ["EV-N26-SOAN-doi-KHAY-C2", "cặp đôi · C2 320×640, mở khay: nhãn công cụ và ô soạn ngoài màn"],
    ["EV-N26-SOAN-ban-KHAY-C8", "đám bạn · C8, mở khay: công cụ ngoài màn"],
  ]],
  ["EV-N26-CHUYEN-ghep", "UI-126, UI-127, UI-125 · từ đám bạn thành cặp đôi (chat-10 đề nghị «Một đôi», chat-11 đồng ý), C1", [
    ["EV-N26-CHUYEN-MOI-B-C1", "chat-11: hàng mời «Một đôi»"],
    ["EV-N26-CHUYEN-DEN-B-C1", "chạm hàng mời: màn không nhắc lời đề nghị"],
    ["EV-N26-CHUYEN-LOAI-SO-B-C1", "sau «Mở sổ cặp đôi»: «Đồng ý là một đôi»"],
    ["EV-N26-CHUYEN-SAU-B-C1", "đã đồng ý: sổ cặp đôi, không bìa M6"],
    ["EV-N26-CHUYEN-CHAT-A-C1", "chat-10 để chat mở: 8 s sau vẫn bốn công cụ"],
  ]],
  ["EV-N26-CHU-NHOM-ghep", "UI-128 · chữ của nhóm trong chat hai người, C1", [
    ["EV-N26-BAN-CAI-DAT-C1", "đám bạn · cài đặt: «Cả nhóm thấy cùng một màu.»"],
    ["EV-N26-DOI-C1", "cặp đôi · trạng thái rỗng: «Rủ hội một buổi»"],
  ]],
  ["EV-N26-GU-ghep", "«Gu của hai bạn» của chat-9 (cặp đôi): lời hứa nói cả chat, «Bật lại cho chat», và UI-129 khi lệnh hỏng, C1", [
    ["EV-N26-GU-C1", "lời hứa: Nếp và Rủ Đi AI trong chat"],
    ["EV-N26-GU-BAT-LAI-C1", "đồng ý cũ: «Bật lại cho chat»"],
    ["EV-N26-GU-BAT-LAI-LOI-C1", "UI-129 · bật lại hỏng nửa chừng: công tắc tắt, câu lỗi dưới lớp phủ"],
    ["EV-N26-GU-BAT-LOI-C1", "UI-129 · bật hỏng: câu lỗi dưới lớp phủ"],
  ]],
  ["EV-N26-RU-ghep", "UI-130 · «Rủ … tới đây» với cặp đám bạn, C1", [
    ["EV-N26-RU-chua-so-C1", "chat-0 · chi tiết quán: «Rủ Chat Test 02 tới đây»"],
    ["EV-N26-RU-chua-so-DEN-C1", "chưa sổ: mời lập sổ, không nhắc quán"],
    ["EV-N26-RU-co-so-DEN-C1", "chat-4 · có sổ: bảo tạo kèo rồi thêm chỗ"],
    ["EV-N26-RU-co-so-FORM-C1", "«Rủ hội mình đi chơi»: form không có quán"],
  ]],
  ["EV-N26-Q2-ghep", "UI-131 · bản phác máy chủ nhận ở cặp đám bạn chat-6/chat-7 (phác qua API), C1", [
    ["EV-N26-Q2-SO-C1", "không gian giấy «Hội bạn»: tờ phác, «Gửi cho người ấy»"],
    ["EV-N26-Q2-CHAT-C1", "chat đôi: không hàng nào"],
  ]],
  ["EV-N26-MOI-ghep", "N26 · hàng mời lập sổ ở chat của đám bạn (chat-13), và mở rộng UI-001, UI-083, UI-093", [
    ["EV-N26-MOI-C1", "C1: hàng mời hai dòng, 49dp"],
    ["EV-N26-MOI-DEN-C1", "chạm hàng mời: «Xem lời đề nghị»"],
    ["EV-N26-MOI-C6", "UI-001 · C6: hàng mời một dòng, 31dp"],
    ["EV-N26-LOI-so-503-C1", "UI-083 · đọc sổ 503: «Chưa có tờ nào tuần này»"],
    ["EV-N26-GU-C6", "UI-093 · C6: sheet gu rộng 768"],
  ]],
  ["EV-N26-LAB-ghep", "N26 · trang lab /dev/hai-lop-chat trên server dev (fixture): ba lỗi sửa ở 762d5c5 vẫn đạt", [
    ["EV-N26-LAB-KHAY-DOI-C2", "C2 320: năm ô một hàng, 51,2"],
    ["EV-N26-LAB-XEM-C1", "tấm «Xem» phủ cả cửa sổ"],
    ["EV-N26-LAB-TRA-LOI-GIAM-C1", "«Reduce Motion»: chỉ câu trọn"],
    ["EV-N26-LAB-STICKER-doi-C1", "sticker cặp đôi"],
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

// Every row naming a frame of a composite gets the composite as well; a row
// that already names it is left alone, so a second run adds nothing.
const khungCua = new Map();
for (const [id, , khung] of GHEP) for (const [f] of khung) khungCua.set(f, [...(khungCua.get(f) ?? []), id]);
// The frame sheet of the delayed return is committed as it is.
khungCua.set("EV-N26-NHAY-tre-800-khung-C1", ["EV-N26-NHAY-tre-800-khung-C1"]);
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
  if (r.feature !== "N26") continue;
  const them = [...new Set((r.evidence ?? []).flatMap((e) => khungCua.get(e) ?? []))].filter((g) => !(r.evidence ?? []).includes(g));
  if (!them.length) continue;
  const { luc: _luc, ...giu } = r;
  so.ghi({ ...giu, evidence: [...(r.evidence ?? []), ...them] });
  gan++;
}
console.log(`n26-ghep: ${GHEP.length} ảnh ghép; gắn vào ${gan} hàng`);
