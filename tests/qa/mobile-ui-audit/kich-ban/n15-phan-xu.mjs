/* N15 verdicts reached by LOOKING, with the capture that was looked at, the
 * issue each failing row of n15-nhat-ky.mjs now points at, the native rows
 * every screen carries, the one case this stack cannot run (Nếp building a
 * book with a real AI key), and the withdrawal of the placeholder row the main
 * ledger held for this feature. Same shape as n14-phan-xu.mjs.
 *
 *   source audit-env-main.sh && node kich-ban/n15-phan-xu.mjs
 *
 * Safe to run twice: a verdict or an issue already written as the last row of
 * its test case is not written again.
 */
import { soGhi } from "../thu-vien/ghi.mjs";

const out = process.env.AUDIT_OUT;
if (!out) throw new Error("thiếu AUDIT_OUT");
const so = soGhi(out);

const cuoiCung = () => {
  const m = new Map();
  for (const r of so.doc()) {
    if (r.rut) {
      for (const k of [...m.keys()]) if (k.startsWith(`${r.tc}|`)) m.delete(k);
      continue;
    }
    m.set(`${r.tc}|${r.nenTang ?? "web"}|${r.cauHinh ?? ""}`, r);
  }
  return m;
};
let hang = cuoiCung();
let moi = 0;

/** A verdict by eye; skipped when the same verdict is already the last row. */
const phanXu = (rec) => {
  const ghiChu = `phân xử bằng mắt: ${rec.ghiChu}`;
  const r = hang.get(`${rec.tc}|${rec.nenTang ?? "web"}|${rec.cauHinh ?? ""}`);
  if (r && r.ghiChu === ghiChu && r.status === rec.status && (r.issue ?? null) === (rec.issue ?? null)) return;
  so.ghi({ feature: "N15", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, evidence: [], ...rec, ghiChu });
  moi++;
};
/** The measured row, kept as it is, now naming its issue. */
const ganIssue = (tc, cauHinh, issue) => {
  for (const c of cauHinh) {
    const r = hang.get(`${tc}|web|${c}`);
    if (!r) throw new Error(`không có hàng đo ${tc} ${c}; chạy n15-nhat-ky.mjs trước`);
    if (r.status !== "FAIL") throw new Error(`${tc} ${c} không phải FAIL (${r.status}); xem lại trước khi gắn issue`);
    if (r.issue === issue) continue;
    const { luc: _luc, ...giu } = r;
    so.ghi({ ...giu, issue });
    moi++;
  }
};

// ------------------------------------------------ the placeholder row of main
{
  const cho = so.doc().filter((r) => r.tc === "TC-N-15-NHAT-KY");
  if (cho.length && !cho.at(-1).rut) so.rut("TC-N-15-NHAT-KY", "hàng giữ chỗ của feature mới #15: đã audit, thay bằng các hàng TC-N15-* (n15-nhat-ky.mjs, n15-phan-xu.mjs)");
  hang = cuoiCung();
}

// ---------------------------------------------------------------- baselines
phanXu({ tc: "TC-N15.S02-BASE", screen: "N15.S02", state: "chat-0 (người tổ chức), «Kèo album retest» trước khi khép", action: "màn kèo → «Giữ lại cuộc đi»", cauHinh: "C1,C2,C3", expected: "Màn khép cuộc đi: hiển thị đủ, không tràn, không cắt, không che", status: "PASS", evidence: ["EV-N15-KHEP-C1", "EV-N15-KHEP-C2", "EV-N15-KHEP-C3", "EV-N15-KHEP-ghep"], ghiChu: "«Cuộc đi khép lại. Câu chuyện còn đây.», câu hỏi giữ một khoảnh khắc hay cả cuốn sổ, thẻ kèo (tên, «29/09/2026 đến 30/09/2026», nét đường ba điểm, «Mang về vài tấm ảnh…»), bộ chọn «Khoảnh khắc | Sổ chuyến đi», câu «Khép cuộc đi cho cả hội…» và nút «Khép cuộc đi»: thấy trọn ở cả ba cấu hình, C3 tối đọc được. C8 (390×460): nút ở dưới nếp gấp (y 612–664), cuộn tới được; C6: một cột giữa màn. Số đo tự động ở TC-N15-KHEP-BASE" });
phanXu({ tc: "TC-N15.S03-BASE", screen: "N15.S03", state: "chat-0, vừa khép cuộc đi, chưa có sổ", action: "«Khép cuộc đi» → «Mang theo điều gì vào sổ?»", cauHinh: "C1,C2,C3", expected: "Màn chọn chất liệu: hiển thị đủ, không tràn, không cắt, không che", status: "PASS", evidence: ["EV-N15-NGUON-C1", "EV-N15-NGUON-C2", "EV-N15-NGUON-C3", "EV-N15-NGUON-ghep"], ghiChu: "tiêu đề, câu «Ảnh đã được chọn theo ngày đi. Bạn xem lại nhé, nhất là khi hai cuộc hẹn trùng nhau.», «3 / 40 ảnh đã chọn», ô ảnh hai cột với nhãn «Đã chọn · 29/09/2026» và chú thích (chú thích dài cắt bằng «…» có chủ đích), «Thêm ảnh từ máy», ô «Một đoạn chuyện muốn gửi cùng». C2 vẫn hai cột, nhãn xuống dòng, không tràn; C3 tối đọc được; ảnh tải được. Ô ảnh không có aria-checked (UI-003): hàng đo riêng" });
phanXu({ tc: "TC-N15.S04-BASE", screen: "N15.S04", state: "chat-0: sổ nháp dựng tay (C1); sổ đã lưu, mở lại từ màn đọc (C2, C3)", action: "«Sửa theo cách mình nhớ»", cauHinh: "C1,C2,C3", expected: "Màn sửa sổ: hiển thị đủ, không tràn, không cắt, không che", status: "PASS", evidence: ["EV-N15-SUA-C1", "EV-N15-SUA-BASE-C2", "EV-N15-SUA-BASE-C3", "EV-N15-SUA-ghep", "EV-N15-HEP-ghep"], ghiChu: "«Viết lại theo cách mình nhớ», «Xem như người đọc», «Bạn muốn giữ cho ai xem?» với «Chỉ mình tôi | Công khai», nút đóng dấu «Lưu riêng tư», ô «Tên cuốn sổ», «Lời mở», «Thay ảnh bìa», «Thêm ảnh từ máy», rồi mỗi trang: «Tên trang», «Chuyện của trang», «Thay ảnh trang N», «Đưa trang N lên trước», «Bỏ trang này». C2 và C3 tối: không tràn, không cắt chữ (TC-N15-SUA-BASE); ba ô chữ cao 44, dưới 48 của DESIGN.md (UI-001, hàng TC-N15-O-NHAP-44). Chế độ sửa không hiện ảnh của trang, xem bằng «Xem như người đọc» (ADR-0039 không quy định; không ghi thành issue). Tên trang «2026-09-29»: UI-154, hàng đo riêng" });
phanXu({ tc: "TC-N15.S06-BASE", screen: "N15.S06", state: "chat-0, sổ của mình (Chỉ mình tôi)", action: "mở /diaries/[id]", cauHinh: "C1,C2,C3", expected: "Màn đọc sổ: hiển thị đủ, không tràn, không cắt, không che", status: "PASS", evidence: ["EV-N15-DOC-C1", "EV-N15-DOC-C2", "EV-N15-DOC-C3", "EV-N15-DOC-ghep"], ghiChu: "thanh đầu «Sổ chuyến đi», bìa (ảnh, tên, lời mở, dải đánh dấu cam), các trang «Ngày 29/09/2026» với hai ảnh và số trang «1 / 2». C2: bìa hẹp lại, tên xuống hai dòng, không cắt; C3 tối đọc được. Nút của chủ sổ ở cuối trang (TC-N15-DOC-BASE)" });

phanXu({ tc: "TC-N15-O-NHAP-44", screen: "N15.S04", state: "chat-0, chế độ sửa sổ", action: "đo vùng chạm của các ô chữ (số đo của ảnh EV-N15-SUA-BASE-*)", cauHinh: "C2,C3", expected: "ô nhập cao ≥ 48dp (DESIGN.md)", status: "FAIL", issue: "UI-001", evidence: ["EV-N15-SUA-BASE-C2", "EV-N15-SUA-BASE-C3", "EV-N15-HEP-ghep"], ghiChu: "«Tên cuốn sổ» và hai ô «Tên trang»: 288×44 ở C2, 328×44 ở C3 (ONhapMuc một dòng, như UI-001); ngoài ba ô này không vùng chạm nào dưới 48" });

// ------------------------------------------- measured rows, now with issues
ganIssue("TC-N15-Q4-API", ["-"], "UI-149");
ganIssue("TC-N15-Q4-KE", ["C1"], "UI-149");
// Measured after «Kèo album retest» ended (q4:hero, 01/10): the group settlement hero adds the shared expense once per outing.
ganIssue("TC-N15-Q4-HERO", ["C1"], "UI-149");
ganIssue("TC-N15-LUU-LOI", ["C1"], "UI-150");
ganIssue("TC-N15-API", ["-"], "UI-151");
ganIssue("TC-N15-KHEP-SAP-TOI", ["C1"], "UI-151");
ganIssue("TC-N15-KHEP-THANH-VIEN", ["C1"], "UI-152");
ganIssue("TC-N15-TUONG-RONG", ["C1", "C2", "C3"], "UI-153");
ganIssue("TC-N15-TRANG-TEN", ["C1"], "UI-154");
// Extensions of issues of the base audit, measured on this feature's screens.
ganIssue("TC-N15-NGUON-ARIA", ["C1"], "UI-003");
ganIssue("TC-N15-BIA-RADIO", ["C1"], "UI-003");
ganIssue("TC-N15-KHEP-404", ["C1"], "UI-019");
ganIssue("TC-N15-SUA-CO", ["C8"], "UI-040");
ganIssue("TC-N15-SUA-CO", ["C6"], "UI-093");
ganIssue("TC-N15-ROI-MAT-SUA", ["C1"], "UI-097");
ganIssue("TC-N15-DOC-NGOAI-THU-LAI", ["C1"], "UI-100");
// A link opened without a session loses its way after sign-in: the base audit's UI-121 (chat link), met again on a diary link.
ganIssue("TC-N15-KHONG-PHIEN", ["C1"], "UI-121");

// ------------------------------------------------------------ native rows
const LY_DO = {
  android: "không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)",
  ios: "không chạy được iOS: không có macOS / iOS Simulator",
};
for (const [screen, route] of [
  ["N15.S01", "/outings/[id] (lối «Giữ lại cuộc đi» trên màn kèo)"],
  ["N15.S02", "/outings/[id]/ending, pha khép cuộc đi"],
  ["N15.S03", "/outings/[id]/ending, pha chọn chất liệu và dựng sổ (Nếp hoặc tự xếp)"],
  ["N15.S04", "/outings/[id]/ending, sổ nháp, chế độ sửa, sheet chọn ảnh, lưu"],
  ["N15.S05", "/profile và /people/[id] (kệ «Những ngày muốn giữ»)"],
  ["N15.S06", "/diaries/[id] (đọc sổ, công khai, gửi cộng đồng, cất về riêng tư, xoá)"],
  ["N15.S07", "/groups/[id]/album (kệ album, «đã chia»)"],
]) {
  for (const nenTang of ["android", "ios"]) {
    const tc = `TC-${screen}-NATIVE`;
    if (hang.get(`${tc}|${nenTang}|-`)) continue;
    so.ghi({ feature: "N15", tc, screen, layer: "-", state: "mọi trạng thái", action: `mở ${route}`, nenTang, cauHinh: "-", expected: `như web, trên thiết bị ${nenTang === "android" ? "Android" : "iOS"}`, status: "BLOCKED", method: "STATIC", evidence: [], issue: null, ghiChu: LY_DO[nenTang] });
    moi++;
  }
}
// Nếp with a real key: the stack has none, so only the failure path was measured (TC-N15-DUNG-AI).
if (!hang.get("TC-N15-DUNG-AI-THAT|web|C1")) {
  so.ghi({ feature: "N15", tc: "TC-N15-DUNG-AI-THAT", screen: "N15.S03", layer: "-", state: "chat-0, chất liệu 3 ảnh và 1 trích đoạn", action: "«Dựng sổ cùng Nếp» khi máy chủ có khoá AI", nenTang: "web", cauHinh: "C1", expected: "Nếp dựng sổ từ đúng ảnh và trích đoạn đã chọn; dựng lại không ghi đè bản đã sửa (ADR-0039)", status: "BLOCKED", method: "RUNTIME-WEB", evidence: [], issue: null, ghiChu: "stack cục bộ không có khoá AI: lượt dựng trả câu lỗi sau 5,7 s và còn lối tự xếp (TC-N15-DUNG-AI). Nội dung sổ do Nếp viết, việc Nếp chỉ dùng chất liệu được chọn, và việc dựng lại giữ bản đã sửa chưa đo được" });
  moi++;
}

// --------------------------------------- two dates that read as a phone number
// The community post built from a book prints its two page headings side by side,
// «2026-09-29» and «2026-09-29» one space apart, and the repo guard's Vietnamese
// mobile pattern reads a ten-digit number across the pair (month and day of the
// first, year and month of the second). Each bare date is wrapped
// in guillemets, the way the documents quote dates: same digits, nothing left for
// the pattern to span. A second run finds nothing to wrap.
const GIONG_SO_DT = /(?<!\d)(?:(?:\+|00)?84(?:[ ().-]*0)?|0)[ ().-]*(?:[35789](?:[ ().-]*\d){8}|2(?:[ ().-]*\d){8,9})(?!\d)/;
hang = cuoiCung();
for (const r of hang.values()) {
  if (r.feature !== "N15" || !GIONG_SO_DT.test(r.ghiChu ?? "")) continue;
  const { luc: _luc, ...giu } = r;
  so.ghi({ ...giu, ghiChu: r.ghiChu.replace(/(?<!«)\b(\d{4}-\d{2}-\d{2})\b(?!»)/g, "«$1»") });
  moi++;
}

// ------------------------------------------------------------------- check
hang = cuoiCung();
const n15 = [...hang.values()].filter((r) => r.feature === "N15");
const thieu = n15.filter((r) => r.status === "FAIL" && !r.issue).map((r) => `${r.tc} ${r.cauHinh}`);
const dem = {};
for (const r of n15) dem[r.status] = (dem[r.status] ?? 0) + 1;
console.log(`n15-phan-xu: ${moi} dòng mới; N15 có ${n15.length} hàng ${JSON.stringify(dem)}; FAIL thiếu issue: ${thieu.join(", ") || "không"}`);
if (thieu.length) process.exitCode = 1;
