/* F09 verdicts reached by LOOKING, with the capture that was looked at, and
 * the issue each measured row of f09-ho-so.mjs now points at. Same shape as
 * f08-phan-xu.mjs: `phanXu` writes a judged row, `ganIssue` copies a measured
 * row unchanged except for its issue.
 *
 *   node kich-ban/f09-phan-xu.mjs
 */
import { soGhi } from "../thu-vien/ghi.mjs";

const out = process.env.AUDIT_OUT;
if (!out) throw new Error("thiếu AUDIT_OUT");
const so = soGhi(out);

const phanXu = (rec) =>
  so.ghi({ feature: "F09", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec, ghiChu: `phân xử bằng mắt: ${rec.ghiChu}` });

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
    if (!r) throw new Error(`không có hàng đo ${tc} ${c}; chạy f09-ho-so.mjs trước`);
    const { luc: _luc, ...giu } = r;
    so.ghi({ ...giu, issue });
  }
};

// ---------------------------------------------------------------- baselines
const base = (tc, screen, action, state, rows) => {
  for (const [cauHinh, status, issue, evidence, ghiChu] of rows) {
    phanXu({ tc, screen, state, action, cauHinh, expected: "không tràn, không cắt/che, đọc được ở sáng và tối; ở tablet bố cục có trần bề ngang", status, issue, evidence: [evidence], ghiChu });
  }
};
base("TC-F09.S01-BASE", "F09.S01", "mở /profile", "chat-0", [
  ["C1,C2,C3", "PASS", null, "EV-F09.S01-BASE-ghep", "«Cá nhân», thẻ «Hộ chiếu Rủ Đi» (tên, con dấu «Tham gia 2026», số đếm), «Chỉnh hồ sơ», 8 hàng lối vào cao 58; C2 xuống dòng gọn, C3 tối đọc được. Thanh tab che hàng cuối cho tới khi cuộn; cuộn tới và mở được «Tài khoản» (TC-F09-HO-SO, TC-L36-VONGDOI). Avatar «0» là chữ đầu của chữ cuối tên giả"],
  ["C4,C5,C6,C7", "FAIL", "UI-105", "EV-F09.S01-BASE-ghep", "C4, C5 như điện thoại; C6 thẻ hộ chiếu 616px (vừa trong trần 640 nhờ rail); C7 thẻ 872px và các hàng 842px trải hết bề ngang (TC-F09-TABLET)"],
]);
base("TC-F09.S02-BASE", "F09.S02", "mở /settings", "moi-54 (tài khoản mới)", [
  ["C1,C2,C3", "PASS", null, "EV-F09.S02-BASE-ghep", "«Hồ sơ» (ảnh, «Đổi ảnh đại diện»), «Sở thích», «Đăng nhập & phiên», «Quyền riêng tư» (công tắc, ba chip), «Người đã chặn»; C2 xuống dòng gọn, C3 tối đọc được. Công tắc chỉ 40×20 (UI-001); ba mục «Giao diện» là tab không có tablist và nhóm chip là radiogroup chứa nút (UI-042, TC-F09-CAI-DAT-ARIA)"],
  ["C4,C5", "PASS", null, "EV-F09.S02-rong-BASE-ghep", "như C1; không tràn"],
]);
base("TC-F09.S03-BASE", "F09.S03", "mở /settings/phien", "moi-54, một phiên", [
  ["C1,C2,C3", "PASS", null, "EV-F09.S03-BASE-ghep", "câu giải thích, một hàng «Phiên này, đang dùng · Hết hạn 28/10/2026» với dấu «PHIÊN NÀY», không có nút thu hồi cho chính phiên này"],
]);
base("TC-F09.S04-BASE", "F09.S04", "mở /settings/da-chan", "chat-20 đang chặn Chat Test 22 (từ F06); moi-54 chưa chặn ai", [
  ["C1,C2,C3", "PASS", null, "EV-F09.S04-BASE-ghep", "hàng «Chat Test 22 · Chặn từ 28/9/2026» với «Bỏ chặn» 81×48; trạng thái rỗng «Bạn chưa chặn ai» của moi-54 đọc được (EV-F09.S04-rong-BASE-ghep). Ngày viết «28/9/2026», khác «28/10/2026» ngay màn phiên: TC-F09-NGAY (UI-104)"],
]);
base("TC-F09.S05-BASE", "F09.S05", "mở /settings/ve-rudi", "moi-54", [
  ["C1,C2,C3", "PASS", null, "EV-F09.S05-BASE-ghep", "dấu «BẢN NHÁP», «Điều khoản và dữ liệu», năm mục, câu cuối; đọc được ở cả ba, không tràn. Dấu bản nháp là chủ ý (chú thích mã: chưa người làm luật nào đọc)"],
]);
base("TC-F09.S06-BASE", "F09.S06", "mở /settings/xoa-tai-khoan", "moi-54 (không bấm gì)", [
  ["C1,C2,C3", "PASS", null, "EV-F09.S06-BASE-ghep", "bước 1: «Xoá tài khoản là vĩnh viễn.», bốn câu về cái gì mất và cái gì ở lại (kể cả sổ tiền giữ nguyên), «Tôi hiểu, tiếp tục» (viền), «Ở lại»"],
]);

// ------------------------------------------------------- judged from captures
phanXu({ tc: "TC-F09-DA-LUU", screen: "F09.S01", layer: "L36", state: "moi-54 lưu 2 địa điểm qua API", action: "tab Cá nhân → «Đã lưu»", cauHinh: "C1", expected: "panel cho thấy các nơi đã lưu (tên, lối mở), không chỉ con số", status: "FAIL", issue: "UI-109", evidence: ["EV-F09-DA-LUU-C1"], ghiChu: "panel «Đã lưu»: tiêu đề «2 địa điểm», câu «Danh sách lưu trong tài khoản của bạn. Mở Khám phá để thêm.», nút «Mở Khám phá»; không có tên chỗ nào. Đọc mã: Khám phá chỉ tô tim trên từng hàng (`ExploreLive.tsx:361`), không có bộ lọc «đã lưu»; không màn nào liệt kê chỗ đã lưu" });

phanXu({ tc: "TC-F09-NGAY", screen: "F09.S04", state: "phiên và người bị chặn tạo ngày 28/09", action: "đọc ngày ở «Người đã chặn» và «Đăng nhập & phiên»", cauHinh: "C1", expected: "ngày viết một kiểu trong app («28/09»)", status: "FAIL", issue: "UI-104", evidence: ["EV-F09.S04-BASE-ghep"], ghiChu: "«Chặn từ 28/9/2026» và «Số điện thoại · từ 28/9/2026» (tháng không đệm 0) nằm cạnh «Hết hạn 28/10/2026»; màn khác viết «28/09». Nguồn: `toLocaleDateString(\"vi-VN\")` ở `DaChanScreen.tsx:102`, `PhienScreen.tsx:103`, `phien-cai-dat.ts:71`" });

phanXu({ tc: "TC-F09-FOCUS-MAN", feature: "F00", screen: "(toàn app)", state: "đi tới màn bằng nút trong app, có phiên", action: "đọc `document.activeElement` ngay sau khi màn mới hiện", cauHinh: "C1", expected: "focus vào màn mới (tiêu đề hoặc phần tử đầu), để trình đọc màn hình biết đã sang màn khác", status: "FAIL", issue: "UI-112", evidence: [], ghiChu: "focus nằm ở `body` sau khi mở: «Cài đặt» (TC-F09-CAI-DAT-ARIA), «Đăng nhập & phiên» (TC-F09-PHIEN), «Người đã chặn» (TC-F09-BO-CHAN), «Xoá tài khoản» (TC-L27-VONGDOI), trình xem story (TC-L26-VONGDOI), «Thả khoảnh khắc» và «Đăng story» (TC-L34-VONGDOI). Đối chứng: hộp thoại và sheet thì đưa focus vào trong (TC-L25-VONGDOI, TC-L24-VONGDOI)" });

// ------------------------------------------- measured rows, now with issues
ganIssue("TC-F09-LOI-CONG-TAC", ["C1"], "UI-107");
ganIssue("TC-L36-VONGDOI", ["C1"], "UI-108");
ganIssue("TC-F09-SUA-BACK", ["C1"], "UI-108");
ganIssue("TC-L27-VONGDOI", ["C1"], "UI-110");
ganIssue("TC-F09-CHU-CHAN-TRANG", ["C1"], "UI-111");
ganIssue("TC-F09-CAI-DAT-ARIA", ["C1"], "UI-042, UI-089");
ganIssue("TC-F09-KHONG-PHIEN", ["C1"], "UI-082");
ganIssue("TC-F09-TABLET", ["C7"], "UI-105");

// Left unlinked since checkpoint 2 (F00 had no judgement script then): the
// measured row behind UI-002, found by the «no FAIL without an issue» check.
ganIssue("TC-F00-LA-DANG-NHAP", ["C1"], "UI-002");
