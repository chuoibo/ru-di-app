/* N14 verdicts reached by LOOKING, with the capture that was looked at, the
 * issue each failing row of n14-cong-dong.mjs now points at, the native rows
 * every screen carries, and the withdrawal of the placeholder row the main
 * ledger held for this feature. Same shape as n26-phan-xu.mjs.
 *
 *   source audit-env-main.sh && node kich-ban/n14-phan-xu.mjs
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
  so.ghi({ feature: "N14", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, evidence: [], ...rec, ghiChu });
  moi++;
};
/** The measured row, kept as it is, now naming its issue. */
const ganIssue = (tc, cauHinh, issue) => {
  for (const c of cauHinh) {
    const r = hang.get(`${tc}|web|${c}`);
    if (!r) throw new Error(`không có hàng đo ${tc} ${c}; chạy n14-cong-dong.mjs trước`);
    if (r.status !== "FAIL") throw new Error(`${tc} ${c} không phải FAIL (${r.status}); xem lại trước khi gắn issue`);
    if (r.issue === issue) continue;
    const { luc: _luc, ...giu } = r;
    so.ghi({ ...giu, issue });
    moi++;
  }
};

// ------------------------------------------------ the placeholder row of main
{
  const cho = so.doc().filter((r) => r.tc === "TC-N-14-CONG-DONG");
  if (cho.length && !cho.at(-1).rut) so.rut("TC-N-14-CONG-DONG", "hàng giữ chỗ của feature mới #14: đã audit, thay bằng các hàng TC-N14-* (n14-cong-dong.mjs, n14-phan-xu.mjs)");
  hang = cuoiCung();
}

// ---------------------------------------------------------------- baselines
phanXu({ tc: "TC-N14.S01-BASE", screen: "N14.S01", state: "chat-1, «Dành cho bạn» có bài đã duyệt (B2 có ảnh, B1 ngắn)", action: "mở tab Cộng đồng (chưa chạm gì)", cauHinh: "C1,C2,C3", expected: "Bảng tin: hiển thị đủ, không tràn, không cắt, không che", status: "PASS", evidence: ["EV-N14-BANG-C1", "EV-N14-BANG-C2", "EV-N14-BANG-C3"], ghiChu: "tiêu đề «Cộng đồng», ba tab, ô tìm, hộp mời cá nhân hoá, rồi thẻ bài: tên, «N phút · Cộng đồng», thân sáu dòng với «Đọc tiếp», khung ảnh, chủ đề, «Thích · Bình luận · Chia sẻ». C3 tối đọc được. Thẻ đầu là B2 ở C1, B1 ở C2 và C3: thứ hạng đổi giữa các lượt vì lượt thích. C2: dòng «18 phút · Cộng đồng» xuống hai dòng cạnh hai nút, không bị cắt. Chữ tròn trong ảnh đại diện là «0» vì tên tổng hợp «Chat Test 01» kết bằng số (dữ liệu, không phải lỗi). Khung ảnh ở C2 bị cắt mép phải (UI-143, hàng đo riêng)" });
phanXu({ tc: "TC-N14.S03-BASE", screen: "N14.S03", state: "form «Kể một khoảnh khắc» trống (chat-0; chat-1 ở C3)", action: "tab Cộng đồng → «Đăng khoảnh khắc»", cauHinh: "C1,C2,C3", expected: "Form đăng bài: hiển thị đủ, không tràn, không cắt, không che", status: "PASS", evidence: ["EV-N14-DANG-C1", "EV-N14-DANG-C2", "EV-N14-DANG-C3", "EV-N14-FORM-ghep"], ghiChu: "«Kể một khoảnh khắc», «Hôm nay có gì đáng nhớ?», ô nội dung 180px, «Thêm ảnh hoặc video», ô «Chủ đề» và dòng «Tối đa 5 chủ đề…», «Tag bạn bè», hai lựa chọn người đọc và «Lựa chọn khác»; nút gửi ở chân màn thấy trọn ở cả ba cấu hình. Nút gửi tắt không lý do (UI-091), radio không aria-checked (UI-003): hàng đo riêng" });
phanXu({ tc: "TC-N14.S04-BASE", screen: "N14.S04", state: "chat-0, chi tiết B2 của chính mình", action: "bảng tin → thân B2 (hai chạm)", cauHinh: "C1,C2,C3", expected: "Chi tiết bài: hiển thị đủ, không tràn, không cắt, không che", status: "PASS", evidence: ["EV-N14-CHI-TIET-C1", "EV-N14-CHI-TIET-C2", "EV-N14-CHI-TIET-C3"], ghiChu: "«Một câu chuyện» với «Quay lại», thẻ bài hết thân, ảnh 296×330, chủ đề, ba hành động, «@Nếp · Giúp giữ khoảnh khắc», «Chuyện trò cùng nhau» và ô bình luận" });
phanXu({ tc: "TC-N14.S09-BASE", screen: "N14.S09", state: "chat-15 (người vận hành), ba bài chờ duyệt", action: "mở /community/review", cauHinh: "C1,C2,C3", expected: "Hàng duyệt: hiển thị đủ, không tràn, không cắt, không che", status: "PASS", evidence: ["EV-N14-DUYET-C1", "EV-N14-DUYET-C2", "EV-N14-DUYET-C3"], ghiChu: "«Xem xét nội dung», mỗi mục: chú thích, thân bài, ảnh (B2 cao 260, contain), ô «Lý do quyết định», «Duyệt công khai» và «Từ chối». Mã trạng thái thô (UI-144) và nút tắt không lý do (UI-091): hàng đo riêng" });

// ------------------------------------------------------- judged from captures
phanXu({ tc: "TC-N14-BANG-THICH", screen: "N14.S01", state: "chat-1, thẻ B1 chưa thích", action: "chạm «Thích»", cauHinh: "C1", expected: "đếm lên 1; trạng thái đã thích đọc được bằng lời hoặc aria, không chỉ bằng màu", status: "PASS", evidence: ["EV-N14-BANG-THICH-C1"], ghiChu: "tim đỏ, số «1»; tên trợ năng đổi «Thích bài, 0 lượt thích» → «Bỏ thích bài, 1 lượt thích»: trạng thái nói bằng lời trong chính tên nút. Không có aria-pressed (accessibilityState không tới DOM, họ UI-003) nhưng tên đã đủ" });
phanXu({ tc: "TC-N14-BL-MO-RONG", screen: "N14.S05", layer: "L-cong-dong-binh-luan", state: "chat-1, sheet «Bình luận» mở trên bảng tin, cửa sổ 390×460", action: "«Mở rộng» → chi tiết bài → «Quay lại»", cauHinh: "C8", expected: "«Mở rộng» tới chi tiết bài; quay lại bảng tin không còn sheet che (hoặc sheet giữ đúng chỗ, dùng được)", status: "PASS", evidence: ["EV-N14-BL-MO-RONG-C8"], ghiChu: "về bảng tin thì sheet «Bình luận» vẫn mở, đã có trả lời mới của Chat Test 01 thụt vào dưới bình luận của Chat Test 02, ô gõ cuộn tới được: dùng được. Phần đầu sheet (tay cầm, «Đóng bảng») nằm ngoài cửa sổ ở C8: UI-040" });

// ------------------------------------------- measured rows, now with issues
ganIssue("TC-N14-API-RONG", ["-"], "UI-132");
ganIssue("TC-N14-RONG-BANG-TIN", ["C1", "C2", "C3"], "UI-132");
ganIssue("TC-N14-RONG-THU-LAI", ["C1"], "UI-132");
ganIssue("TC-N14-RONG-THINH-HANH", ["C1"], "UI-132");
ganIssue("TC-N14-RONG-DE-SAU", ["C1"], "UI-132");
ganIssue("TC-N14-DANG-6-CHU-DE", ["C1"], "UI-133");
ganIssue("TC-N14-DANG-CHU-DE-NGAN", ["C1"], "UI-133");
ganIssue("TC-N14-WS-MAT-CHU", ["C1"], "UI-134");
ganIssue("TC-N14-WS-AN-HIEN", ["C1"], "UI-134");
ganIssue("TC-N14-BANG-CUON", ["C1"], "UI-135");
ganIssue("TC-N14-BANG-CHIA-SE", ["C1"], "UI-136");
ganIssue("TC-N14-BANG-CHIA-SE-LINK", ["C1"], "UI-136");
ganIssue("TC-N14-KHONG-PHIEN-SAU", ["C1"], "UI-137");
ganIssue("TC-N14-NEP", ["C1"], "UI-138");
ganIssue("TC-N14-BANG-CHAM-THAN", ["C1"], "UI-139");
ganIssue("TC-N14-BANG-THEO-DOI", ["C1"], "UI-140");
ganIssue("TC-N14-BANG-AN", ["C1"], "UI-141");
ganIssue("TC-N14-XOA-LICH-SU", ["-"], "UI-141");
ganIssue("TC-N14-SUA-BANG-TIN", ["C1"], "UI-142");
ganIssue("TC-N14-BANG-THE", ["C2"], "UI-143");
ganIssue("TC-N14-WS-DAI", ["C2"], "UI-143");
ganIssue("TC-N14-DUYET-HANG", ["C1"], "UI-144");
ganIssue("TC-N14-DUYET-BL", ["C1"], "UI-144");
ganIssue("TC-N14-TIM-RONG", ["C1"], "UI-145");
ganIssue("TC-N14-GIU", ["C1"], "UI-145");
ganIssue("TC-N14-CHU-DE", ["C1"], "UI-146");
ganIssue("TC-N14-THONG-BAO", ["C1"], "UI-147");
ganIssue("TC-N14-LOI-BL", ["C1"], "UI-148");
// Extensions of issues of the base audit, measured on this feature's screens.
ganIssue("TC-N14-TAB-ARIA", ["C1"], "UI-003");
ganIssue("TC-N14-RONG-AXE", ["C1"], "UI-003");
ganIssue("TC-N14-BANG-AXE", ["C1"], "UI-003");
ganIssue("TC-N14-DANG-RADIO", ["C1"], "UI-003");
ganIssue("TC-N14-BL-TAG", ["C1"], "UI-003");
ganIssue("TC-N14-DANG-NUT-TAT", ["C1"], "UI-091");
ganIssue("TC-N14-DANG-NHOM", ["C1"], "UI-091");
ganIssue("TC-N14-DUYET-NUT-TAT", ["C1"], "UI-091");
ganIssue("TC-N14-BL-NUT-TAT", ["C1"], "UI-091");
ganIssue("TC-N14-NEP-NUT-TAT", ["C1"], "UI-091");
ganIssue("TC-N14-SUA-KHOA", ["C1"], "UI-091");
ganIssue("TC-N14-BANG-XEM-ANH", ["C1"], "UI-094");
ganIssue("TC-N14-BANG-CAI-DAT", ["C8"], "UI-040");
ganIssue("TC-N14-BL-SHEET", ["C8"], "UI-040");
ganIssue("TC-N14-BL-XOA", ["C1"], "UI-096");
ganIssue("TC-N14-LOI-THICH", ["C1"], "UI-095");
ganIssue("TC-N14-DANG-TABLET", ["C6", "C7"], "UI-093");

// ------------------------------------------------------------ native rows
const LY_DO = {
  android: "không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)",
  ios: "không chạy được iOS: không có macOS / iOS Simulator",
};
for (const [screen, route] of [
  ["N14.S01", "/(tabs)/community (bảng tin ba tab, bài đã lưu, bài của tôi)"],
  ["N14.S02", "màn trong của Cộng đồng mở từ link khi chưa đăng nhập"],
  ["N14.S03", "/community/new (viết, sửa bài)"],
  ["N14.S04", "/community/posts/[id] và các sheet quản lý, báo cáo, Nếp"],
  ["N14.S05", "bình luận (trong chi tiết và sheet «Bình luận»)"],
  ["N14.S06", "/community/search"],
  ["N14.S07", "/community/topic"],
  ["N14.S08", "/community/notifications, /community/keeps"],
  ["N14.S09", "/community/review (người vận hành)"],
  ["N14.S10", "/people/[id] (tường cá nhân v2, phần gặp Cộng đồng)"],
]) {
  for (const nenTang of ["android", "ios"]) {
    const tc = `TC-${screen}-NATIVE`;
    if (hang.get(`${tc}|${nenTang}|-`)) continue;
    so.ghi({ feature: "N14", tc, screen, layer: "-", state: "mọi trạng thái", action: `mở ${route}`, nenTang, cauHinh: "-", expected: `như web, trên thiết bị ${nenTang === "android" ? "Android" : "iOS"}`, status: "BLOCKED", method: "STATIC", evidence: [], issue: null, ghiChu: LY_DO[nenTang] });
    moi++;
  }
}
// Video: the stack has no media worker (ffmpeg), so a video never leaves «processing».
if (!hang.get("TC-N14-DANG-VIDEO|web|C1")) {
  so.ghi({ feature: "N14", tc: "TC-N14-DANG-VIDEO", screen: "N14.S03", layer: "-", state: "chat-0, form đăng bài", action: "«Thêm ảnh hoặc video» → chọn một tệp MP4", nenTang: "web", cauHinh: "C1", expected: "video tải lên, xử lý xong thì «Video đã sẵn sàng»; bài có video phát được", status: "BLOCKED", method: "RUNTIME-WEB", evidence: [], issue: null, ghiChu: "stack cục bộ không chạy worker xử lý video (ffmpeg): video ở trạng thái processing mãi, MediaPicker chờ 60 × 3 s rồi báo «Video vẫn đang xử lý». Không tải video lên stack" });
  moi++;
}

// ------------------------------------------ stream timestamps in the notes
// The relay recorded each `feed.changed` frame with its epoch-millisecond
// `occurred_at_ms`: a 13-digit run that the repo guard's long-number rule
// refuses in the generated matrix. It is the server's clock, not a
// measurement (the fade samples keep their own), so the last row of each
// such test case is copied with the value masked. A second run finds nothing.
hang = cuoiCung();
for (const r of hang.values()) {
  if (r.feature !== "N14" || !/"occurred_at_ms":\d{9,}/.test(r.ghiChu ?? "")) continue;
  const { luc: _luc, ...giu } = r;
  so.ghi({ ...giu, ghiChu: r.ghiChu.replace(/"occurred_at_ms":\d{9,}/g, '"occurred_at_ms":"[ms]"') });
  moi++;
}

// ------------------------------------------------------------------- check
hang = cuoiCung();
const n14 = [...hang.values()].filter((r) => r.feature === "N14");
const thieu = n14.filter((r) => r.status === "FAIL" && !r.issue).map((r) => `${r.tc} ${r.cauHinh}`);
const dem = {};
for (const r of n14) dem[r.status] = (dem[r.status] ?? 0) + 1;
console.log(`n14-phan-xu: ${moi} dòng mới; N14 có ${n14.length} hàng ${JSON.stringify(dem)}; FAIL thiếu issue: ${thieu.join(", ") || "không"}`);
if (thieu.length) process.exitCode = 1;
