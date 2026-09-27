/* F03 verdicts reached by LOOKING, with the capture that was looked at, and
 * the issue each measured row of f03-keo.mjs now points at. Same shape as
 * f02-phan-xu.mjs: `phanXu` writes a judged row, `ganIssue` copies a measured
 * row unchanged except for its issue.
 *
 *   node kich-ban/f03-phan-xu.mjs
 */
import { soGhi } from "../thu-vien/ghi.mjs";

const out = process.env.AUDIT_OUT;
if (!out) throw new Error("thiếu AUDIT_OUT");
const so = soGhi(out);

const phanXu = (rec) =>
  so.ghi({ feature: "F03", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec, ghiChu: `phân xử bằng mắt: ${rec.ghiChu}` });

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
    if (!r) throw new Error(`không có hàng đo ${tc} ${c}; chạy f03-keo.mjs trước`);
    const { luc: _luc, ...giu } = r;
    so.ghi({ ...giu, issue });
  }
};

// ---------------------------------------------------------------- baselines
phanXu({ tc: "TC-F03.S01-BASE", screen: "F03.S01", state: "3 kèo của Team Đà Lạt", action: "mở /plan", cauHinh: "C1,C2,C3", expected: "không tràn, không cắt/che, đọc được ở sáng và tối", status: "FAIL", issue: "UI-044", evidence: ["EV-F03.S01-BASE-ghep", "EV-F03-META-C2"], ghiChu: "vé kèo sắp tới đọc trọn; hai vé «Sau đó» bị cắt dòng thông tin (đo ở TC-F03-META); axe chỉ còn aria-required-parent của thanh tab (UI-003)" });
phanXu({ tc: "TC-F03.S01-BASE", screen: "F03.S01", state: "3 kèo", action: "mở /plan", cauHinh: "C4,C5", expected: "như C1–C3", status: "FAIL", issue: "UI-044", evidence: ["EV-F03.S01-rong-BASE-ghep"], ghiChu: "dòng thông tin vé «Sau đó» vẫn bị cắt ở 375 và 430" });
phanXu({ tc: "TC-F03.S01-BASE", screen: "F03.S01", state: "3 kèo", action: "mở /plan", cauHinh: "C6,C7", expected: "như C1–C3, có rail", status: "PASS", evidence: ["EV-F03.S01-rong-BASE-ghep"], ghiChu: "rail trái, vé rộng, mọi dòng đọc trọn" });
phanXu({ tc: "TC-F03.S02-BASE", screen: "F03.S02", state: "form kèo mới", action: "mở /outings/new", cauHinh: "C1,C2,C3", expected: "form đọc được; ô trống trông là ô trống; axe sạch", status: "FAIL", issue: "UI-034", evidence: ["EV-F03.S02-BASE-ghep"], ghiChu: "bố cục ổn, CTA dính đáy; ô ngân sách hiện «250000» là placeholder nhưng trông như số đã điền (UI-034); chip ngân sách role radio không có aria-checked, axe critical ×4 (UI-003); ô nhập và nút ± cao 44 (UI-001)" });
phanXu({ tc: "TC-F03.S02-BASE", screen: "F03.S02", state: "form kèo mới", action: "mở /outings/new", cauHinh: "C4,C5,C6,C7", expected: "như C1–C3", status: "PASS", evidence: ["EV-F03.S02-rong-BASE-ghep"], ghiChu: "form canh giữa ở tablet, CTA thấy được; không tràn (axe chỉ chạy ở C1/C3)" });
phanXu({ tc: "TC-F03.S03-BASE", screen: "F03.S03", state: "kèo 3 chặng, Lịch trình", action: "mở /outings/[id]", cauHinh: "C1,C2,C3", expected: "không tràn/cắt/che; axe sạch", status: "FAIL", issue: "UI-042", evidence: ["EV-F03.S03-BASE-ghep"], ghiChu: "bố cục ổn ở cả ba; axe critical: aria-selected trên nút chặng ×3, slider «Thứ tự …» thiếu aria-valuenow ×3, tab Lịch trình/Bản đồ không có tablist ×2; không có mép Nếp (UI-037)" });
phanXu({ tc: "TC-F03.S03-BASE", screen: "F03.S03", state: "kèo 3 chặng", action: "mở /outings/[id]", cauHinh: "C4,C5", expected: "như C1–C3", status: "PASS", evidence: ["EV-F03.S03-rong-cfg-BASE-ghep"], ghiChu: "không tràn, chặng đọc được" });
phanXu({ tc: "TC-F03.S03-BASE", screen: "F03.S03", state: "kèo 3 chặng", action: "mở /outings/[id]", cauHinh: "C6,C7", expected: "đầu màn thẳng cột với nội dung", status: "FAIL", issue: "UI-047", evidence: ["EV-F03.S03-rong-cfg-BASE-ghep", "EV-F03-TABLET-C6"], ghiChu: "đầu màn (Quay lại, tiêu đề, công tắc) co vào giữa, nội dung trải từ mép trái (đo ở TC-F03-DAU-MAN)" });
phanXu({ tc: "TC-F03.S03-dai-BASE", screen: "F03.S03", state: "kèo tên 96 ký tự, 12 chặng dài", action: "mở /outings/[id]", cauHinh: "C1,C2,C3", expected: "tên dài xuống dòng, chặng dài xuống dòng, không tràn", status: "PASS", evidence: ["EV-F03.S03-dai-BASE-ghep"], ghiChu: "tên kèo 4–5 dòng, không cắt; nhãn chặng dài xuống dòng; ở 320 cột tên chặng rất hẹp (UI-045, đo ở TC-F03-COT-CHANG)" });
phanXu({ tc: "TC-F03.S03-rong-BASE", screen: "F03.S03", state: "kèo 2 ngày, 0 chặng", action: "mở /outings/[id]", cauHinh: "C1,C2,C3", expected: "trạng thái rỗng nói cách thêm chặng", status: "PASS", evidence: ["EV-F03.S03-rong-BASE-ghep"], ghiChu: "«Chưa có chặng nào» + «Thêm chặng» + câu hướng dẫn về Khám phá" });
phanXu({ tc: "TC-F03.S04-BASE", screen: "F03.S04", state: "Bản đồ", action: "mở chế độ Bản đồ", cauHinh: "C1,C2,C3", expected: "mốc của các chặng có địa điểm; ngày trống nói rõ và có lối về", status: "FAIL", issue: "UI-033", evidence: ["EV-F03-BAN-DO-rong-C1", "EV-F03-BAN-DO-goc-C1"], ghiChu: "nền bản đồ trống vì proxy chặn tile (không tính); ngày trống giấu «Về Lịch trình» dưới vùng cuộn (UI-033); kèo 3 ngày không hiện mốc nào (UI-032)" });
phanXu({ tc: "TC-F03.S05-BASE", screen: "F03.S05", state: "đường dẫn không có ?place", action: "mở /outings/chon", cauHinh: "C1,C2,C3", expected: "câu nói thiếu địa điểm, có lối ra", status: "FAIL", issue: "UI-046", evidence: ["EV-F03.S05-BASE-ghep"], ghiChu: "skeleton đứng mãi; tiêu đề «Thêm vào kèo» và nút Quay lại (mở lạnh thì không làm gì, UI-018)" });
phanXu({ tc: "TC-F03.S06-BASE", screen: "F03.S06", state: "đã đăng nhập", action: "mở /check-ins/new", cauHinh: "C1,C2,C3", expected: "chuyển về /plan (B5)", status: "PASS", evidence: ["EV-F03.S06-BASE-ghep"], ghiChu: "đường cuối /plan" });
phanXu({ tc: "TC-F03.S07-BASE", screen: "F03.S07", state: "đã đăng nhập", action: "mở /trips/[id]/itinerary", cauHinh: "C1,C2,C3", expected: "chuyển về /outings/[id] (B5)", status: "PASS", evidence: ["EV-F03.S07-BASE-ghep"], ghiChu: "đường cuối /outings/<id>" });
phanXu({ tc: "TC-F03.S08-BASE", screen: "F03.S08", state: "đã đăng nhập", action: "mở /trips/[id]/timeline", cauHinh: "C1,C2,C3", expected: "chuyển về màn sống như hai route anh em", status: "FAIL", issue: "UI-035", evidence: ["EV-F03.S08-BASE-ghep"], ghiChu: "hiện lịch trình demo (Khởi hành từ TP.HCM, Check-in homestay, Bánh căn Lệ) dưới id kèo thật, không nhãn demo, không nút Quay lại" });

// ------------------------------------------------------------ layer cycles
phanXu({ tc: "TC-L08-VONGDOI", screen: "F03.S03", layer: "L08", state: "đóng → mở → dùng → đóng → mở lại", action: "vòng đời Chặng mới", cauHinh: "C1", expected: "mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về", status: "FAIL", issue: "UI-038", evidence: ["EV-F03-THEM-mo-C1", "EV-F03-THEM-C8"], ghiChu: "X, nền, Esc, kéo dài, fling đóng sạch và trả focus; kéo ngắn bật về; câu lỗi nhãn rỗng có aria-live; gửi thêm được chặng. Hỏng: Back rời cả màn (UI-038), chạm đúp mở rồi đóng (UI-039), cao 93% ở cửa sổ thấp (UI-040)" });
phanXu({ tc: "TC-L09-VONGDOI", screen: "F03.S03", layer: "L09", state: "đóng → mở → dùng → đóng", action: "vòng đời Gắn quán", cauHinh: "C1", expected: "mở khi chạm chặng chưa có địa điểm; chọn thì gắn và đóng; X/Esc/«Để sau» đóng sạch", status: "PASS", evidence: ["EV-F03-GAN-mo-C1", "EV-F03-GAN-xong-C1"], ghiChu: "gắn «Lưng Chừng Cafe» thành công; X, Esc, «Để sau» đóng sạch, focus về chặng" });
phanXu({ tc: "TC-L10-NEN", screen: "F03.S04", layer: "L10", state: "sheet «Sửa trang ngày» mở", action: "chạm công tắc «Lịch trình» phía trên sheet", cauHinh: "C1", expected: "nền của sheet phủ và làm mờ cả đầu màn; chạm ra ngoài thì đóng sheet", status: "FAIL", issue: "UI-041", evidence: ["EV-F03-SUA-NGAY-C1"], ghiChu: "đầu màn (Quay lại, tiêu đề, công tắc) không bị làm mờ, trông như vẫn dùng được; chạm vào công tắc không đổi chế độ và cũng không đóng sheet (vẫn 1 dialog); nút tên «Sửa trang ngày» mở sheet tiêu đề «Những hẹn quan trọng»" });
phanXu({ tc: "TC-L10-VONGDOI", screen: "F03.S04", layer: "L10", state: "đóng → mở → dùng → đóng", action: "vòng đời Sửa ngày", cauHinh: "C1", expected: "mở, phủ màn, đóng được", status: "FAIL", issue: "UI-041", evidence: ["EV-F03-SUA-NGAY-C1"], ghiChu: "như TC-L10-NEN; công tắc «Quay về điểm đầu» báo aria-checked false" });
phanXu({ tc: "TC-L11-VONGDOI", screen: "F03.S04", layer: "L11", state: "đóng → mở → đóng", action: "vòng đời Điểm hẹn", cauHinh: "C1", expected: "mở từ bản đồ, «Thêm điểm hẹn» chờ tên, đóng không lưu", status: "PASS", evidence: ["EV-F03-DIEM-HEN-C1"], ghiChu: "mở bằng chuột phải; nút chờ tên có viền đứt (ADR-0038); X đóng không lưu. Giữ ngón tay: BLOCKED (TC-L11-GIU)" });
phanXu({ tc: "TC-L12-VONGDOI", screen: "F03.S04", layer: "L12", state: "bản đồ có cụm", action: "vòng đời Popup cụm bản đồ", cauHinh: "C1", expected: "mở trong bản đồ, focus vào danh sách, đóng bằng nút và Esc, tên tiếng Việt", status: "FAIL", issue: "UI-043", evidence: ["EV-F03-CUM-C1"], ghiChu: "danh sách nằm trọn trong bản đồ, mục 48px, focus vào; nút đóng tên «Close popup», Esc không đóng" });
phanXu({ tc: "TC-L13-VONGDOI", screen: "F03.S04", layer: "L13", state: "trang ngày mở", action: "vòng đời Trang ngày gập được", cauHinh: "C1", expected: "gập, mở; báo trạng thái cho công nghệ hỗ trợ", status: "FAIL", issue: "UI-003", evidence: ["EV-F03-TRANG-NGAY-gap-C1"], ghiChu: "gập và mở được; không có aria-expanded (TC-L13-GAP)" });
phanXu({ tc: "TC-L14-VONGDOI", screen: "F03.S02", layer: "L14", state: "lá «Ngày đi»", action: "vòng đời Lá lịch tháng", cauHinh: "C1", expected: "mở, đổi tháng, chọn ngày, khép lại", status: "PASS", evidence: ["EV-F03-TAO-lich-C1", "EV-F03-LICH-chon-C1"], ghiChu: "mở lịch 44×44, «Tháng sau», chọn 10 thì lá đổi và lịch khép; ghi nhận: lá «Ngày về» vẫn 27/9, trước ngày đi, không tự dời" });
phanXu({ tc: "TC-L15-VONGDOI", screen: "F03.S03", layer: "L15", state: "sheet «Chặng mới»", action: "vòng đời Mặt quay giờ", cauHinh: "C1", expected: "kéo đổi giờ; gõ giờ thay thế được", status: "PASS", evidence: ["EV-F03-BAN-XOAY-C1"], ghiChu: "kéo 20:00 → 03:00; gõ 07:30 nhận; slider không mang aria-valuenow/aria-valuetext (UI-042)" });

// ------------------------------------------- measured rows, now with issues
ganIssue("TC-F03-NEP-LICH-TRINH", ["C1"], "UI-037");
ganIssue("TC-F03-VE-LICH-TRINH", ["C1", "C2", "C3", "C4", "C8"], "UI-033");
ganIssue("TC-F03-BAN-DO-CHANG-NHIEU-NGAY", ["C1"], "UI-032");
ganIssue("TC-L08-DONG-back", ["C1"], "UI-038");
ganIssue("TC-L08-CHAM-DUP", ["C1"], "UI-039");
ganIssue("TC-L08-KICH-THUOC", ["C8"], "UI-040");
ganIssue("TC-F03-SAP-XEP-PHIM", ["C1"], "UI-036");
ganIssue("TC-F03-TAO-NGAN-SACH-TRONG", ["C1"], "UI-034");
ganIssue("TC-F03-DEMO-TIMELINE", ["C1"], "UI-035");
ganIssue("TC-F03-CHON-THIEU", ["C1"], "UI-046");
ganIssue("TC-F03-META", ["C1", "C2", "C4", "C5"], "UI-044");
ganIssue("TC-F03-COT-CHANG", ["C2"], "UI-045");
ganIssue("TC-F03-DAU-MAN", ["C6", "C7"], "UI-047");
ganIssue("TC-F03-BAN-DO-NHAN", ["C1"], "UI-043");
ganIssue("TC-L12-CUM", ["C1"], "UI-043");
ganIssue("TC-MO-M5", ["C9"], "UI-027");
console.log("đã ghi phân xử F03");
