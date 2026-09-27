/* F05 verdicts reached by LOOKING, with the capture that was looked at, and
 * the issue each measured row of f05-chat.mjs now points at. Same shape as
 * f04-phan-xu.mjs: `phanXu` writes a judged row, `ganIssue` copies a measured
 * row unchanged except for its issue.
 *
 *   node kich-ban/f05-phan-xu.mjs
 */
import { soGhi } from "../thu-vien/ghi.mjs";

const out = process.env.AUDIT_OUT;
if (!out) throw new Error("thiếu AUDIT_OUT");
const so = soGhi(out);

const phanXu = (rec) =>
  so.ghi({ feature: "F05", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec, ghiChu: `phân xử bằng mắt: ${rec.ghiChu}` });

const cuoiCung = new Map();
for (const r of so.doc()) {
  if (r.rut) {
    for (const k of [...cuoiCung.keys()]) if (k.startsWith(`${r.tc}|`)) cuoiCung.delete(k);
    continue;
  }
  cuoiCung.set(`${r.tc}|${r.nenTang ?? "web"}|${r.cauHinh ?? ""}`, r);
}
const ganIssue = (tc, cauHinh, issue) => {
  for (const c of cauHinh) {
    const r = cuoiCung.get(`${tc}|web|${c}`);
    if (!r) throw new Error(`không có hàng đo ${tc} ${c}; chạy f05-chat.mjs trước`);
    const { luc: _luc, ...giu } = r;
    so.ghi({ ...giu, issue });
  }
};

// ---------------------------------------------------------------- baselines
phanXu({ tc: "TC-F05.S01-BASE", screen: "F05.S01", state: "Minh Anh: 1 nhóm, không lời mời", action: "mở /messages", cauHinh: "C1,C2,C3", expected: "không tràn, không cắt/che, đọc được ở sáng và tối", status: "PASS", evidence: ["EV-F05.S01-BASE-ghep"], ghiChu: "tiêu đề, «Tạo nhóm», ô «Đăng story», hàng Team Đà Lạt (vai trò + tin cuối); tin cuối cắt một dòng có chủ đích ở C2/C3" });
phanXu({ tc: "TC-F05.S01-BASE", screen: "F05.S01", state: "như trên", action: "mở /messages", cauHinh: "C4,C5,C6,C7,C9", expected: "như C1–C3", status: "PASS", evidence: ["EV-F05.S01-rong-BASE-ghep"], ghiChu: "C6/C7 có rail, hàng trải hết cột; C9 giống C1" });
phanXu({ tc: "TC-F05.S02-BASE", screen: "F05.S02", state: "Team Đà Lạt, 13 tin, bình chọn mở 0 phiếu", action: "mở chat nhóm", cauHinh: "C1,C2,C3", expected: "không tràn, không cắt/che; bong bóng, giờ, ô soạn thẳng hàng", status: "FAIL", issue: "UI-062, UI-064, UI-065", evidence: ["EV-F05.S02-BASE-ghep", "EV-F05-BO-CUC-G8-C1-ct"], ghiChu: "ở cả ba cấu hình: chữ gợi ý của ô soạn nằm trên, «+» và mũi tên gửi nằm dưới (UI-062); avatar đứng ngang dòng giờ, thấp hơn bong bóng, giờ lặp dưới mỗi cụm (UI-064); thẻ bình chọn 0 phiếu chiếm gần nửa màn, ở C2 đẩy các tin khác khỏi màn (UI-065). Nhãn «Chưa mã hoá đầu cuối» và dải ghim hiện đủ" });
phanXu({ tc: "TC-F05.S02-BASE", screen: "F05.S02", state: "như trên", action: "mở chat nhóm", cauHinh: "C4,C5,C6,C7,C9", expected: "như C1–C3", status: "FAIL", issue: "UI-062, UI-065", evidence: ["EV-F05.S02-rong-BASE-ghep"], ghiChu: "ô soạn lệch như C1 ở mọi bề rộng (UI-062); ở C6 thẻ bình chọn trải cả hàng 768dp, mỗi lựa chọn là một hộp khoảng 780px chỉ có một chữ (UI-065); C7 gom cột giữa, đọc được" });
phanXu({ tc: "TC-F05.S02-DAI-BASE", screen: "F05.S02", state: "nhóm 20 người, 40 tin tổng hợp (đoạn dài, URL liền, nhiều dòng, emoji)", action: "mở chat nhóm", cauHinh: "C1,C2,C3", expected: "tin dài xuống dòng trong bong bóng; không tràn khỏi màn", status: "FAIL", issue: "UI-063, UI-062", evidence: ["EV-F05.S02-dai-BASE-ghep"], ghiChu: "đoạn dài, 4 dòng và emoji đọc trọn; bong bóng URL của mình tràn: ở C2 mất đầu link ở mép trái, ở C3 chạm mép (UI-063); ô soạn lệch (UI-062)" });
phanXu({ tc: "TC-F05.S03-BASE", screen: "F05.S03", state: "chat hai người chưa có tin", action: "mở", cauHinh: "C1,C2,C3", expected: "đầu chat, dải tờ giấy đôi, trạng thái trống, ô soạn đọc được", status: "FAIL", issue: "UI-062", evidence: ["EV-F05.S03-BASE-ghep"], ghiChu: "dải «Tờ giấy của hai mình», nhãn mã hoá, «Một lời mở đầu.» đọc trọn; ô soạn lệch như nhóm (ở C2 chữ gợi ý hai dòng nên lấp đầy hai hàng của ô)" });
phanXu({ tc: "TC-F05.S04-BASE", screen: "F05.S04", state: "chưa đăng nhập (bản trải nghiệm)", action: "mở /votes/binh-chon-mau", cauHinh: "C1,C2,C3", expected: "nhãn Demo; lựa chọn và nút đọc được", status: "PASS", evidence: ["EV-F05.S04-BASE-ghep"], ghiChu: "3 hàng quán có icon và vòng chọn, nút «Xác nhận lựa chọn của tôi» viền đứt khi chưa chọn; câu giải thích nói bản trải nghiệm chỉ ghi phiếu của mình. Mẫu hàng gọn này đáng so với thẻ bình chọn trong chat (UI-065)" });

// ------------------------------------------------------- judged from captures
so.rut("TC-F05-GHIM-TREN-NEN", "tiêu chí tự động chỉ đo điểm chạm (trúng nền «Đóng»), không đo dải có bị làm mờ không; phân xử bằng mắt ở f05-phan-xu.mjs");
phanXu({ tc: "TC-F05-GHIM-TREN-NEN", screen: "F05.S02", layer: "L17", state: "nhóm có dải ghim; sheet Cài đặt nhóm (L17) hoặc menu tin (L18) đang mở", action: "nhìn dải ghim; chạm vào giữa dải", cauHinh: "C1", expected: "nền mờ phủ cả dải; chạm ngoài sheet thì đóng sheet", status: "FAIL", issue: "UI-070", evidence: ["EV-F05-GHIM-TREN-NEN-C1"], ghiChu: "đầu chat và danh sách tin bị làm mờ, riêng dải «Cùng chọn · Xem phiếu» vẫn sáng như dùng được. Chạm vào giữa dải trúng nền «Đóng»: sheet đóng, không mở phiếu (L17 và L18 như nhau, hàng đo trước khi rút). Cùng hình trong ảnh L16 và L17 lúc dải là «Tờ hẹn chung · Sửa cùng hội»" });

// ------------------------------------------- measured rows, now with issues
ganIssue("TC-F05-SOAN-CAN", ["C1"], "UI-062");
ganIssue("TC-F05-SOAN-NHIEU-DONG", ["C1"], "UI-062");
ganIssue("TC-F05-BO-CUC-TIN", ["C1 G8", "C1 G20"], "UI-062, UI-064, UI-001");
ganIssue("TC-F05-BINH-CHON-THE", ["C1 G8", "C1 G20"], "UI-065");
ganIssue("TC-F05-URL-DAI", ["C1", "C2", "C3", "C5"], "UI-063");
ganIssue("TC-F05-MENU-PHAN-UNG", ["C1"], "UI-001");
ganIssue("TC-L17-VONGDOI", ["C1"], "UI-003");
ganIssue("TC-L16-VONGDOI", ["C1"], "UI-067");
ganIssue("TC-L18-VONGDOI", ["C1"], "UI-038");
ganIssue("TC-L19-VONGDOI", ["C1"], "UI-038");
ganIssue("TC-L20-VONGDOI", ["C1"], "UI-038");
ganIssue("TC-L21-VONGDOI", ["C1"], "UI-066, UI-038");
ganIssue("TC-F05.S02-503", ["C1"], "UI-068");
ganIssue("TC-L22-KHI-DOC-CU", ["C1"], "UI-069");
console.log("đã ghi phân xử F05");
