/* F11 verdicts reached by LOOKING, with the capture that was looked at, and
 * the issue each measured row of f11-demo.mjs now points at. Same shape as
 * f09-phan-xu.mjs.
 *
 *   node kich-ban/f11-phan-xu.mjs
 */
import { soGhi } from "../thu-vien/ghi.mjs";

const out = process.env.AUDIT_OUT;
if (!out) throw new Error("thiếu AUDIT_OUT");
const so = soGhi(out);

const phanXu = (rec) =>
  so.ghi({ feature: "F11", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec, ghiChu: `phân xử bằng mắt: ${rec.ghiChu}` });

const cuoiCung = new Map();
for (const r of so.doc()) {
  if (r.rut) {
    for (const k of [...cuoiCung.keys()]) if (k.startsWith(`${r.tc}|`)) cuoiCung.delete(k);
    continue;
  }
  cuoiCung.set(`${r.tc}|${r.nenTang ?? "web"}|${r.cauHinh ?? ""}`, r);
}
const hangDo = (tc, c) => {
  const r = cuoiCung.get(`${tc}|web|${c}`);
  if (!r) throw new Error(`không có hàng đo ${tc} ${c}; chạy f11-demo.mjs trước`);
  const { luc: _luc, ...giu } = r;
  return giu;
};
const ganIssue = (tc, cauHinh, issue) => {
  for (const c of cauHinh) so.ghi({ ...hangDo(tc, c), issue });
};
// A measured row whose automatic verdict the capture overturns: same row,
// new status, the reason in front of the measured note.
const lat = (tc, c, { status, issue, evidence, lyDo }) => {
  const r = hangDo(tc, c);
  // Run twice, the second run would wrap its own verdict: keep the first.
  if (r.ghiChu?.startsWith("phân xử bằng mắt:")) return;
  so.ghi({ ...r, status, issue, evidence: evidence ?? r.evidence, ghiChu: `phân xử bằng mắt: ${lyDo} (số đo tự động: ${r.ghiChu})` });
};

// ---------------------------------------------------------------- baselines
phanXu({ tc: "TC-F11.S01-BASE", screen: "F11.S01", state: "chưa đăng nhập, mở thẳng đường dẫn của tab", action: "mở /explore, /plan, /messages, /profile", cauHinh: "C1,C2,C3", expected: "4 tab demo: hiển thị đủ, không tràn, không cắt, không che, có nhãn demo, vùng bấm và tên truy cập đạt", status: "FAIL", issue: "UI-082, UI-116", evidence: ["EV-F11.S01-BASE-C1-ghep", "EV-F11.S01-BASE-C2-ghep", "EV-F11.S01-BASE-C3-ghep"], ghiChu: "Khám phá, Lên plan, Cá nhân có nhãn «Dữ liệu demo», đọc được ở cả ba cấu hình. Tin nhắn là một chat nhóm «Team Đà Lạt» 8 thành viên, trông như thật, không có nhãn demo, ô soạn vẫn mời gõ (UI-082); bong bóng «Plan xịn đó, mình bình chọn chỗ BBQ trước đi.» không xuống dòng và tràn qua mép phải ở cả ba cấu hình: thiếu 22px ở C1, 92px ở C2 («…chỗ B»), 52px ở C3; ở C3 bong bóng «Đi chứ! Tớ vote săn mây với BBQ nha» cũng thiếu 9px (UI-116). Kèm theo, không tính là lỗi riêng: placeholder «Tìm quán, món…» ở C2 còn «Tìm quán, mó…» (cùng kiểu phần placeholder của UI-024); mô tả một dòng của các thẻ quán cắt bằng dấu … có chủ đích" });
phanXu({ tc: "TC-F11.S02-BASE", screen: "F11.S02", state: "chưa đăng nhập, mở thẳng route demo", action: "mở tám route demo", cauHinh: "C1,C2,C3", expected: "route demo khi chưa đăng nhập: hiển thị đủ, không tràn, không cắt, có nhãn demo", status: "FAIL", issue: "UI-082, UI-048, UI-023", evidence: ["EV-F11.S02-BASE-C1-0-ghep", "EV-F11.S02-BASE-C1-1-ghep", "EV-F11.S02-BASE-C2-0-ghep", "EV-F11.S02-BASE-C2-1-ghep"], ghiChu: "sáu route mang nhãn «Demo» (hay «Dữ liệu demo») trên thanh đầu; «Lịch trình AI» (/trips/…/itinerary) và «Ai dùng món nào?» (/smart-split/…/assignment) chỉ mang nhãn «Nháp», nói về bản nháp của AI chứ không nói dữ liệu là mẫu (UI-082). C1 đọc được, không tràn. Ở C2: số tiền «1.106.250đ» của người thu hiện «1.106.25…» ở quyết toán (UI-048), tiêu đề «Quyết toán chuyế…»; hai nút «Chỉnh lịc…», «Dùng pla…» ở lịch trình AI và «Nhắc thà…» ở check-in (UI-023). Ở C3 còn «Chỉnh lịch trì…» và «Nhắc thành …». Mô tả một dòng ở «Match gu cả nhóm» và danh sách tên ở «Ai dùng món nào?» cắt bằng dấu … có chủ đích" });

// ------------------------------------------------------- judged from captures
phanXu({ tc: "TC-F11-LOI-DANG-NHAP", screen: "F11.S01", state: "chưa đăng nhập, đang ở một màn demo bất kỳ", action: "tìm lối về cửa đăng nhập", cauHinh: "C1", expected: "một lối «Đăng nhập» dễ thấy từ bản demo", status: "FAIL", issue: "UI-082", evidence: ["EV-F11.S01-BASE-C1-ghep", "EV-F11-THOAT-C1"], ghiChu: "bốn tab và tám route demo không có nút «Đăng nhập». Lối duy nhất: tab Cá nhân → hàng «Tài khoản» → «Đăng xuất bản trải nghiệm», rồi về màn chào (TC-F11-THOAT đạt). Nhãn «Đăng xuất» cho một việc thực chất là đi vào" });
// The measured rows only checked height and reach; the expected line is about width.
phanXu({ tc: "TC-L28-KICH-THUOC", screen: "F11.S01", layer: "L28", state: "sheet «Tùy chọn chuyến đi» mở", action: "đo sheet", cauHinh: "C6", expected: "sheet có trần bề ngang hợp lý ở tablet (tiêu chí UI-093: nội dung sheet ≤ 640px)", status: "FAIL", issue: "UI-093", evidence: ["EV-F11-L28-C6"], ghiChu: "sheet trải 664px, hết phần nội dung bên phải rail 104px; vượt trần 640 của UI-093 24px, nhìn vẫn gọn: tiêu đề sát trái, ba lối bấm canh giữa, X sát phải. Chiều cao 26%, mọi nút với tới" });
phanXu({ tc: "TC-L28-KICH-THUOC", screen: "F11.S01", layer: "L28", state: "sheet «Tùy chọn chuyến đi» mở", action: "đo sheet", cauHinh: "C7", expected: "sheet có trần bề ngang hợp lý ở tablet (tiêu chí UI-093: nội dung sheet ≤ 640px)", status: "FAIL", issue: "UI-093", evidence: ["EV-F11-L28-C7"], ghiChu: "sheet trải 920px trong cửa sổ 1024 (hết phần nội dung bên phải rail); tiêu đề ở mép trái, X ở mép phải cách nhau hơn 800px, ba lối bấm ngắn canh giữa một dải trống rộng. Chiều cao 19%" });

// ------------- route rows the automatic check passed with an ellipsis on them
lat("TC-F11-ROUTE-SETTLEMENTS", "C2", { status: "FAIL", issue: "UI-048", evidence: ["EV-F11-ROUTE-settlements-C2", "EV-F11-CAT-settlements-C2"], lyDo: "số tiền người thu «1.106.250đ» hiện «1.106.25…» (cần 116px, có 114px); tiêu đề «Quyết toán chuyế…» (TC-F11-CAT-SETTLEMENTS)" });
for (const c of ["C2", "C3"]) {
  lat("TC-F11-ROUTE-ITINERARY", c, { status: "FAIL", issue: "UI-082, UI-023", evidence: [`EV-F11-ROUTE-itinerary-${c}`, `EV-F11-CAT-itinerary-${c}`], lyDo: `ngoài nhãn «Nháp» (UI-082), ${c === "C2" ? "hai nút «Chỉnh lịc…» và «Dùng pla…»" : "nút «Chỉnh lịch trì…»"} bị cắt (TC-F11-CAT-ITINERARY)` });
  lat("TC-F11-ROUTE-CHECKIN", c, { status: "FAIL", issue: "UI-023", evidence: [`EV-F11-ROUTE-checkin-${c}`, `EV-F11-CAT-checkin-${c}`], lyDo: `nút «Nhắc thành viên» ở cuối trang hiện «${c === "C2" ? "Nhắc thà…" : "Nhắc thành …"}» (TC-F11-CAT-CHECKIN)` });
}

// ------------------------------------------- measured rows, now with issues
ganIssue("TC-F11-TAB-MESSAGES", ["C1", "C2", "C3"], "UI-082, UI-116");
ganIssue("TC-F11-ROUTE-ITINERARY", ["C1"], "UI-082");
ganIssue("TC-F11-ROUTE-ASSIGNMENT", ["C1", "C2", "C3"], "UI-082");
ganIssue("TC-F11-CO-PHIEN-TIMELINE", ["C1"], "UI-035");
ganIssue("TC-F11-CAT-SETTLEMENTS", ["C2"], "UI-048");
ganIssue("TC-F11-CAT-ITINERARY", ["C2", "C3"], "UI-023");
ganIssue("TC-F11-CAT-CHECKIN", ["C2", "C3"], "UI-023");
ganIssue("TC-L28-BACK", ["C1"], "UI-117");
ganIssue("TC-L28-BACK-LOI-RA", ["C1"], "UI-117");
ganIssue("TC-L28-BACK-CHAM-XUYEN", ["C1"], "UI-117");
