/* N21 verdicts reached by LOOKING, with the capture that was looked at, the
 * issue each failing row of n21-ho-so.mjs now points at, the native rows every
 * screen carries, the cases this stack cannot run (Nếp with a real AI key, the
 * MP4 worker), and the withdrawal of the placeholder row the main ledger held
 * for this feature. Same shape as n15-phan-xu.mjs.
 *
 *   source audit-env-main.sh && node kich-ban/n21-phan-xu.mjs
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
  so.ghi({ feature: "N21", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, evidence: [], ...rec, ghiChu });
  moi++;
};
/** The measured row, kept as it is (bar the fields in `sua`), now naming its issue. */
const ganIssue = (tc, cauHinh, issue, sua = {}) => {
  for (const c of cauHinh) {
    const r = hang.get(`${tc}|web|${c}`);
    if (!r) throw new Error(`không có hàng đo ${tc} ${c}; chạy n21-ho-so.mjs trước`);
    if (r.status !== "FAIL") throw new Error(`${tc} ${c} không phải FAIL (${r.status}); xem lại trước khi gắn issue`);
    if (r.issue === issue && Object.entries(sua).every(([k, v]) => r[k] === v)) continue;
    const { luc: _luc, ...giu } = r;
    so.ghi({ ...giu, ...sua, issue });
    moi++;
  }
};

// ------------------------------------------------ the placeholder row of main
{
  const cho = so.doc().filter((r) => r.tc === "TC-N-21-HO-SO");
  if (cho.length && !cho.at(-1).rut) so.rut("TC-N-21-HO-SO", "hàng giữ chỗ của feature mới #21: đã audit, thay bằng các hàng TC-N21-* (n21-ho-so.mjs, n21-phan-xu.mjs)");
  hang = cuoiCung();
}

// ---------------------------------------------------------------- baselines
phanXu({ tc: "TC-N21.S01-BASE", screen: "N21.S01", state: "chat-0, tab Cá nhân; sổ hành trình 4 huy hiệu, 1 lượt MP4", action: "mở /profile, cuộn tới thẻ Hành trình", cauHinh: "C1,C2,C3", expected: "thẻ Hành trình và các mục menu: hiển thị đủ, không tràn, không cắt, không che", status: "PASS", evidence: ["EV-N21-CA-NHAN-C1", "EV-N21-CA-NHAN-C2", "EV-N21-CA-NHAN-C3", "EV-N21-VAO-ghep"], ghiChu: "thẻ «Dấu mốc mới: Chuyện mình kể» (lời, ảnh huy hiệu, «4 huy hiệu · 1 lượt dựng MP4 →») thấy trọn ở C1; C2 lời xuống sáu dòng cạnh ảnh huy hiệu, không cắt; C3 tối đọc được. Tràn 0, cắt chữ 0, vùng bấm < 48: 0 ở cả ba. Năm «chữ bị che» mỗi ảnh là hàng menu nằm dưới thanh tab, cuộn tới được: không phải lỗi. Tên mục «Thành tích» khác tên màn tới: TC-N21-CA-NHAN (UI-162)" });
phanXu({ tc: "TC-N21.S02-BASE", screen: "N21.S02", state: "chat-0, sổ hành trình: 3 huy hiệu mở đầu, chưa có kết; máy chưa từng mở sổ", action: "mở /achievements", cauHinh: "C1,C3,C6", expected: "sổ hành trình: hiển thị đủ, không tràn, không cắt, không che", status: "PASS", evidence: ["EV-N21-HT-C1", "EV-N21-HT-DUOI-C1", "EV-N21-HT-C3", "EV-N21-HT-C6", "EV-N21-HT-ghep"], ghiChu: "bìa «Cuốn sổ có nhiều ngã rẽ» và dòng «N kết đã mở · N lượt dựng MP4 còn dùng được», khối «MỚI MỞ», «Chọn lối đi» với bản đồ bốn tuyến, thẻ ngã rẽ (ảnh huy hiệu có khoá, tiến độ, «Mẫu sáng tạo sắp dùng được», nút), «Nếp nhìn đường đi», «Dấu ấn đã giữ»: thấy trọn ở C1, C3 tối đọc được. C6 không tràn, không cắt, nhưng thẻ trải 720 trong khi bản đồ giữ 560 (TC-N21-HT-TABLET, UI-093). Detector báo một hoặc hai «chữ bị che» ở ảnh dưới và ảnh C6: là dấu khoá vẽ trên ảnh huy hiệu chưa đạt, nhìn ảnh thấy rõ, không phải lỗi. Phần tử nhận Tab không tên là Nếp M8: TC-N21-NEP-DIEN-TAB (UI-008)" });
phanXu({ tc: "TC-N21.S02-BASE", screen: "N21.S02", state: "chat-0, sổ hành trình: 3 huy hiệu mở đầu, chưa có kết; máy chưa từng mở sổ", action: "mở /achievements", cauHinh: "C2", expected: "sổ hành trình: hiển thị đủ, không tràn, không cắt, không che", status: "FAIL", issue: "UI-106", evidence: ["EV-N21-HT-C2", "EV-N21-HT-ghep"], ghiChu: "số đo hình sạch (tràn 0, cắt chữ 0), nhưng khối «MỚI MỞ» chỉ còn cột chữ 44px giữa ảnh huy hiệu và Nếp M8: «MỚI / MỞ», tên huy hiệu vỡ giữa từ «Dấu / châ / n / đầu / tiên» (đo bằng Range ở TC-N21-TU-VO). Phần còn lại của sổ thấy trọn" });
phanXu({ tc: "TC-N21-HT-BASE", screen: "N21.S02", state: "chat-0, sổ hành trình: 3 huy hiệu mở đầu, chưa có kết", action: "mở /achievements; đọc màn và số đo", cauHinh: "C2", expected: "bìa, bản đồ bốn tuyến, lối đang chọn và các thẻ ngã rẽ hiển thị đủ; không tràn, không cắt chữ", status: "FAIL", issue: "UI-106", evidence: ["EV-N21-HT-C2", "EV-N21-HT-DUOI-C2", "EV-N21-HT-ghep"], ghiChu: "hàng tự động PASS vì detector hình chỉ thấy tràn và cắt; nhìn ảnh, tên huy hiệu trong khối «MỚI MỞ» vỡ giữa từ «châ|n» (TC-N21-TU-VO, UI-106)" });
for (const cfg of ["C1", "C2", "C3"])
  phanXu({ tc: "TC-N21.S03-BASE", screen: "N21.S03", state: "chat-1 (bạn) mở hồ sơ chat-0; chat-0 trưng bày ba huy hiệu", action: "mở /people/[id]", cauHinh: cfg, expected: "hồ sơ, dấu ấn trưng bày, kệ sổ và trang viết hiển thị đủ; không tràn, không cắt chữ", status: "PASS", evidence: [`EV-N21-XEM-CHAT-1-${cfg}`, "EV-N21-XEM-ghep"], ghiChu: `hồ sơ «Những ngày của Chat Test 01», hộ chiếu, «Dấu ấn chọn giữ trên bìa sổ» (một huy hiệu lớn kèm lời, hai huy hiệu nhỏ), «Nhắn tin», «Thêm hành động», «Những ngày muốn giữ», trang viết: thấy trọn${cfg === "C2" ? "; tên huy hiệu nhỏ xuống dòng giữa hai từ, không cắt" : cfg === "C3" ? "; nền tối đọc được" : ""}. Tràn 0, cắt chữ 0. Năm nút «Mở N bình luận của bài» cao 18: TC-N21-TUONG-VUNG-BAM (UI-001)` });
phanXu({ tc: "TC-N21.S04-BASE", screen: "N21.S04", state: "chat-1 (bạn) mở bài «Bạn bè» có ảnh của chat-0; bình luận gốc và lời đáp", action: "mở /posts/[id]; cuộn tới bình luận", cauHinh: "C1,C2,C3", expected: "trang viết: hiển thị đủ, không tràn, không cắt, không che", status: "PASS", evidence: ["EV-N21-BL-MOT-TANG-C1", "EV-N21-BAI-C2", "EV-N21-BAI-C3", "EV-N21-BAI-BL-C2", "EV-N21-BAI-BL-C3", "EV-N21-BAI-ghep"], ghiChu: "thanh đầu «Trang viết», thẻ bài (tác giả, giờ, người đọc, thân, ảnh nghiêng trong khung, «0 thích · N bình luận», «Thích», «Chia sẻ»), «Lời nhắn dưới trang» với bình luận gốc và lời đáp lùi 24px, ô «Viết điều bạn muốn nói…», «Báo cáo bài này»: thấy trọn ở C1–C3, C3 tối đọc được; tràn 0, cắt chữ 0. Nút gửi tắt viền đứt khi ô trống: đã thuộc UI-091 (F08). Vùng bấm 44: TC-N21-BL-VUNG-BAM (UI-001). Không nút xoá bình luận: TC-N21-Q3-XOA-BL (UI-158)" });
phanXu({ tc: "TC-N21-M8", screen: "N21.S02", state: "chat-0, cùng máy vừa nhận kết «Chuyện mình kể»", action: "«Nhận kết này»; tải lại sổ", cauHinh: "C1", expected: "huy hiệu vừa đạt có khoảnh khắc «MỚI MỞ» một lần", status: "PASS", evidence: ["EV-N21-KET-NHAN-C1", "EV-N21-KET-M8-C1", "EV-N21-SO-ghep"], ghiChu: "ngay sau «Nhận kết này», khối «MỚI MỞ Chuyện mình kể» cùng Nếp M8 hiện ở đầu sổ, bìa đổi thành «1 kết đã mở · 1 lượt dựng MP4 còn dùng được» (EV-N21-KET-NHAN-C1); tải lại: không còn khối (EV-N21-KET-M8-C1). Khoảnh khắc diễn một lần trên máy này. Hàng tự động đọc sau khi tải lại nên đã rút (lỗi harness)" });

// ------------------------------------- rows read off measurements already taken
phanXu({ tc: "TC-N21-TUONG-VUNG-BAM", screen: "N21.S03", state: "chat-1 mở hồ sơ chat-0; tường có năm bài", action: "đo vùng chạm của dòng «N thích · N bình luận» dưới mỗi thẻ (số đo của ảnh EV-N21-XEM-CHAT-1-*)", cauHinh: "C1,C2", expected: "vùng chạm ≥ 48dp (DESIGN.md), tối thiểu 24", status: "FAIL", issue: "UI-001", evidence: ["EV-N21-XEM-CHAT-1-C1", "EV-N21-XEM-CHAT-1-C2", "EV-N21-XEM-ghep"], ghiChu: "năm nút «Mở 0 bình luận của bài», «Mở 2 bình luận của bài» 324×18 ở C1, 254×18 ở C2: dưới cả 24. Cùng đích với nút «Mở bài: …» ngay trên nó (HoSoNguoiScreen.tsx:462, :485)" });
phanXu({ tc: "TC-N21-BL-VUNG-BAM", screen: "N21.S04", state: "chat-1 ở trang viết của chat-0; ba hoặc bốn bình luận", action: "đo vùng chạm của nút bình luận và ô soạn (số đo của ảnh EV-N21-BL-GUI-C1, EV-N21-BAI-BL-*)", cauHinh: "C1,C2,C3", expected: "nút và ô nhập ≥ 48dp (DESIGN.md)", status: "FAIL", issue: "UI-001", evidence: ["EV-N21-BL-GUI-C1", "EV-N21-BAI-BL-C2", "EV-N21-BAI-BL-C3", "EV-N21-BAI-ghep"], ghiChu: "«Thích bình luận», «Trả lời Chat Test 0N» 68×44 và ô «Viết bình luận» 302×44 ở C1 (7 vùng); 9 vùng ở C2 và C3: dưới 48 của DESIGN.md, bằng 44 của PRODUCT. Không vùng nào dưới 44" });
phanXu({ tc: "TC-N21-ANH-KHAY-C8", screen: "N21.S04", layer: "L-khay-anh", state: "chat-1 mở ảnh toàn màn của bài «Bạn bè» (?photo=1); cửa sổ thấp", action: "đo khay «Lời nhắn dưới ảnh» (số đo của TC-N21-ANH-MO C8)", cauHinh: "C8", expected: "khay cao ≤ 82% cửa sổ (DESIGN.md), ảnh còn thấy", status: "FAIL", issue: "UI-040", evidence: ["EV-N21-ANH-C8", "EV-N21-BAI-ghep"], ghiChu: "khay 390×441 trên cửa sổ 460 (96%), đỉnh ở y 19: ảnh chỉ còn dải 19px phía trên. Ở C1 khay 390×594 trên 844 (70%), ảnh thấy được" });
phanXu({ tc: "TC-N21-NEP-DIEN-TAB", screen: "N21.S02", state: "chat-0 mở sổ hành trình trên máy chưa từng mở (khối «MỚI MỞ» có Nếp M8)", action: "đọc phần tử nhận Tab không có tên (số đo của ảnh EV-N21-HT-*)", cauHinh: "C1,C2,C3,C6", expected: "mọi điểm dừng Tab có tên; phần trang trí không nhận Tab", status: "FAIL", issue: "UI-008", evidence: ["EV-N21-HT-C1", "EV-N21-HT-C2", "EV-N21-HT-C3", "EV-N21-HT-C6", "EV-N21-HT-ghep"], ghiChu: "một phần tử không tên mỗi ảnh: div 112×112 ở C1–C3, 128×128 ở C6, đúng chỗ Nếp M8 bên phải khối «MỚI MỞ». Cùng gốc với UI-008: NepDien bọc Pressable accessible={false}, trên web vẫn nhận tabindex (NepDien.tsx:52)" });

// ------------------------------------------- measured rows, now with issues
ganIssue("TC-N21-TRANG-GIU", ["C1"], "UI-155");
ganIssue("TC-N21-TRANG-GIU-YEN", ["C1"], "UI-155");
ganIssue("TC-N21-TUONG-ANH-CD", ["C1"], "UI-156");
ganIssue("TC-N21-LOI-CHON", ["C1"], "UI-157");
// The measured note stands; the expected named a route that does not exist. The way to delete is
// DELETE /posts/{post_id}/comments/{comment_id} (routes.json: LIVE-GO), wrapped by xoaBinhLuanBai.
ganIssue("TC-N21-Q3-XOA-BL", ["C1"], "UI-158", { expected: "người viết xoá được bình luận của mình (có bước hỏi); máy chủ vẫn có DELETE /posts/{post_id}/comments/{comment_id} (LIVE-GO), app có hàm xoaBinhLuanBai" });
ganIssue("TC-N21-DANG-LAI", ["C1"], "UI-159");
ganIssue("TC-N21-MOI-MO", ["C1"], "UI-160");
ganIssue("TC-N21-TRUNG-BAY-4", ["C1"], "UI-161");
ganIssue("TC-N21-CA-NHAN", ["C1", "C2", "C3"], "UI-162");
// Extensions of issues of the base audit, measured on this feature's screens.
ganIssue("TC-N21-HT-TAB-ARIA", ["C1"], "UI-003");
ganIssue("TC-N21-HT-AXE", ["C1"], "UI-003");
ganIssue("TC-N21-NEP-ARIA", ["C1"], "UI-003");
ganIssue("TC-N21-HT-TABLET", ["C6", "C7"], "UI-093");
ganIssue("TC-N21-TU-VO", ["C2"], "UI-106");
ganIssue("TC-N21-XEM-LA-THU-LAI", ["C1"], "UI-100");
ganIssue("TC-N21-KHONG-PHIEN-DANG-NHAP", ["C1"], "UI-082");

// ------------------------------------------------------------ native rows
const LY_DO = {
  android: "không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)",
  ios: "không chạy được iOS: không có macOS / iOS Simulator",
};
for (const [screen, route] of [
  ["N21.S01", "/profile (thẻ Hành trình, mục «Thành tích»)"],
  ["N21.S02", "/achievements (sổ hành trình: lối, ngã rẽ, kết, Nếp, trưng bày, «MỚI MỞ»)"],
  ["N21.S03", "/people/[id] (hồ sơ, huy hiệu trưng bày, tường v2, phân trang, long poll)"],
  ["N21.S04", "/posts/[id] (trang viết cũ: bình luận một tầng, thích bình luận, ảnh toàn màn kèm khay, chia sẻ, đăng lại)"],
]) {
  for (const nenTang of ["android", "ios"]) {
    const tc = `TC-${screen}-NATIVE`;
    if (hang.get(`${tc}|${nenTang}|-`)) continue;
    so.ghi({ feature: "N21", tc, screen, layer: "-", state: "mọi trạng thái", action: `mở ${route}`, nenTang, cauHinh: "-", expected: `như web, trên thiết bị ${nenTang === "android" ? "Android" : "iOS"}`, status: "BLOCKED", method: "STATIC", evidence: [], issue: null, ghiChu: LY_DO[nenTang] });
    moi++;
  }
}
// What this stack cannot run: Nếp with a key, and the worker that renders an MP4.
if (!hang.get("TC-N21-NEP-AI-THAT|web|C1")) {
  so.ghi({ feature: "N21", tc: "TC-N21-NEP-AI-THAT", screen: "N21.S02", layer: "-", state: "chat-0, đã đồng ý cho lần hỏi", action: "«Đồng ý hỏi Nếp lần này» khi máy chủ có khoá AI", nenTang: "web", cauHinh: "C1", expected: "Nếp gợi ý 2–3 ngã rẽ chỉ từ số đếm và mã ngã rẽ; không nhận ảnh, bài, bạn đồng hành, chat, vị trí (03-profile-story-social.md)", status: "BLOCKED", method: "RUNTIME-WEB", evidence: [], issue: null, ghiChu: "stack cục bộ không có khoá AI: gợi ý đến từ sổ («Nếp đang vắng. Số đã gợi ý từ tiến độ thật của bạn.», source go; TC-N21-NEP-DONG-Y). Thân yêu cầu từ app chỉ có {\"consent\":true}; phần máy chủ gửi gì cho model thật chưa đo được" });
  moi++;
}
if (!hang.get("TC-N21-MP4-DUNG|web|C1")) {
  so.ghi({ feature: "N21", tc: "TC-N21-MP4-DUNG", screen: "N21.S02", layer: "-", state: "chat-0 có 1 lượt dựng MP4 (kết «Chuyện mình kể»)", action: "dùng lượt dựng MP4", nenTang: "web", cauHinh: "C1", expected: "lượt được giữ chỗ, video vào thư viện riêng, job hỏng thì hoàn lượt (03-profile-story-social.md)", status: "BLOCKED", method: "STATIC", evidence: [], issue: null, ghiChu: "bộ dựng mẫu chưa nối vào worker: mọi thẻ ghi «Mẫu sáng tạo sắp dùng được» (TC-N21-HT-MAU-MP4) và app không có lối dùng lượt; chỉ đo được số lượt (TC-N21-NHAN-KET)" });
  moi++;
}

// --------------------------------------- two dates that read as a phone number
// Same guard as n15-phan-xu.mjs (incident 34): a bare ISO date beside another reads, to the
// repo guard's Vietnamese mobile pattern, as a ten-digit number; each one is wrapped in «».
const GIONG_SO_DT = /(?<!\d)(?:(?:\+|00)?84(?:[ ().-]*0)?|0)[ ().-]*(?:[35789](?:[ ().-]*\d){8}|2(?:[ ().-]*\d){8,9})(?!\d)/;
hang = cuoiCung();
for (const r of hang.values()) {
  if (r.feature !== "N21" || !GIONG_SO_DT.test(r.ghiChu ?? "")) continue;
  const { luc: _luc, ...giu } = r;
  so.ghi({ ...giu, ghiChu: r.ghiChu.replace(/(?<!«)\b(\d{4}-\d{2}-\d{2})\b(?!»)/g, "«$1»") });
  moi++;
}

// ------------------------------------------------------------------- check
hang = cuoiCung();
const n21 = [...hang.values()].filter((r) => r.feature === "N21");
const thieu = n21.filter((r) => r.status === "FAIL" && !r.issue).map((r) => `${r.tc} ${r.cauHinh}`);
const dem = {};
for (const r of n21) dem[r.status] = (dem[r.status] ?? 0) + 1;
console.log(`n21-phan-xu: ${moi} dòng mới; N21 có ${n21.length} hàng ${JSON.stringify(dem)}; FAIL thiếu issue: ${thieu.join(", ") || "không"}`);
if (thieu.length) process.exitCode = 1;
