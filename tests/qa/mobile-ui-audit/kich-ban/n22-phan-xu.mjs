/* N22 verdicts reached by LOOKING, with the capture that was looked at, the
 * issue each failing row of n22-ai-chat.mjs now points at, the native rows every
 * screen carries, what this stack cannot run (a model behind the AI), and the
 * withdrawal of the placeholder row the main ledger held for this feature. Same
 * shape as n21-phan-xu.mjs.
 *
 *   source audit-env-main.sh && node kich-ban/n22-phan-xu.mjs
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
  so.ghi({ feature: "N22", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, evidence: [], ...rec, ghiChu });
  moi++;
};
/** The measured row, kept as it is (bar the fields in `sua`), now naming its issue. */
const ganIssue = (tc, cauHinh, issue, sua = {}) => {
  for (const c of cauHinh) {
    const r = hang.get(`${tc}|web|${c}`);
    if (!r) throw new Error(`không có hàng đo ${tc} ${c}; chạy n22-ai-chat.mjs trước`);
    if (r.status !== "FAIL") throw new Error(`${tc} ${c} không phải FAIL (${r.status}); xem lại trước khi gắn issue`);
    if (r.issue === issue && Object.entries(sua).every(([k, v]) => r[k] === v)) continue;
    const { luc: _luc, ...giu } = r;
    so.ghi({ ...giu, ...sua, issue });
    moi++;
  }
};

// ------------------------------------------------ the placeholder row of main
{
  const cho = so.doc().filter((r) => r.tc === "TC-N-22-AI-CHAT");
  if (cho.length && !cho.at(-1).rut) so.rut("TC-N-22-AI-CHAT", "hàng giữ chỗ của feature mới #22: đã audit, thay bằng các hàng TC-N22-* (n22-ai-chat.mjs, n22-phan-xu.mjs)");
  hang = cuoiCung();
}

// ---------------------------------------------------------------- baselines
phanXu({ tc: "TC-N22.S01-BASE", screen: "N22.S01", state: "chat-0 trong nhóm chat-test («Tờ hẹn chung» ghim trên đầu luồng); máy chủ không có khoá AI", action: "gõ «@Rủ Đi tối nay nhóm mình đi đâu ngắm đèn?» vào ô soạn", cauHinh: "C1,C3", expected: "thanh đầu, dải ghim, luồng, chip, ô soạn và nút gửi: hiển thị đủ, không tràn, không cắt, không che", status: "PASS", evidence: ["EV-N22-CHIP-C1", "EV-N22-CHIP-C3", "EV-N22-CHIP-ghep"], ghiChu: "thanh đầu «Phòng kiểm thử đồng thời · 20 thành viên · sổ hẹn của hội», «Chưa mã hoá đầu cuối», dải «Tờ hẹn chung · bản 1 · Sửa cùng hội», tin của nhóm, chip «Rủ Đi AI chưa sẵn sàng · Gửi như tin thường» một dòng ngay trên ô soạn, nút gửi: thấy trọn ở C1, C3 tối đọc được. Tràn 0, cắt chữ 0, không ellipsis" });
phanXu({ tc: "TC-N22.S01-BASE", screen: "N22.S01", state: "chat-0 trong nhóm chat-test («Tờ hẹn chung» ghim trên đầu luồng); máy chủ không có khoá AI", action: "gõ «@Rủ Đi tối nay nhóm mình đi đâu ngắm đèn?» vào ô soạn", cauHinh: "C2", expected: "thanh đầu, dải ghim, luồng, chip, ô soạn và nút gửi: hiển thị đủ, không tràn, không cắt, không che", status: "FAIL", issue: "UI-167", evidence: ["EV-N22-CHIP-C2", "EV-N22-CHIP-ghep"], ghiChu: "tràn 0, cắt chữ 0, nhưng chip gãy hai dòng: ký hiệu ✦ đứng riêng dòng đầu, câu xuống dòng hai (chip 288×52 thay vì một dòng 28 như ở C1, C3): TC-N22-CHIP-HEP (UI-167). Hai ellipsis: tên nhóm ở thanh đầu (thiếu 19px, một dòng có chủ đích như mọi tên nhóm) và nhãn dải ghim «Tờ hẹn chung · b…» (thiếu 12px, mất «bản 1»): TC-N22-GHIM-C2 (UI-023)" });
phanXu({ tc: "TC-N22.S02-BASE", screen: "N22.S02", state: "chat-0 trong chat đôi với chat-1 (đám bạn, luồng trống); máy chủ không có khoá AI", action: "gõ «@Rủ Đi cuối tuần hai đứa đi đâu?» vào ô soạn", cauHinh: "C1,C3", expected: "thanh đầu, trạng thái rỗng, chip, ô soạn và nút gửi: hiển thị đủ, không tràn, không cắt, không che", status: "PASS", evidence: ["EV-N22-DOI-CHIP-C1", "EV-N22-DOI-CHIP-C3", "EV-N22-CHIP-ghep"], ghiChu: "thanh đầu «Chat Test 02 · Cuộc trò chuyện của hai mình», «Chưa mã hoá đầu cuối», trạng thái rỗng «Một lời mở đầu.» với «Rủ hội một buổi», chip «Rủ Đi AI chưa sẵn sàng · Gửi như tin thường» một dòng, ô soạn, nút gửi: thấy trọn ở C1, C3 tối đọc được. Tràn 0, cắt chữ 0, không ellipsis" });
phanXu({ tc: "TC-N22.S02-BASE", screen: "N22.S02", state: "chat-0 trong chat đôi với chat-1 (đám bạn, luồng trống); máy chủ không có khoá AI", action: "gõ «@Rủ Đi cuối tuần hai đứa đi đâu?» vào ô soạn", cauHinh: "C2", expected: "thanh đầu, trạng thái rỗng, chip, ô soạn và nút gửi: hiển thị đủ, không tràn, không cắt, không che", status: "FAIL", issue: "UI-167", evidence: ["EV-N22-DOI-CHIP-C2"], ghiChu: "tràn 0, cắt chữ 0, không ellipsis; chip gãy hai dòng như ở nhóm: ✦ đứng riêng dòng đầu, câu ở dòng hai (288×52): TC-N22-CHIP-HEP (UI-167)" });
for (const cfg of ["C1", "C2", "C3"])
  phanXu({ tc: "TC-N22.S04-BASE", screen: "N22.S04", state: "trang lab /dev/tra-loi-song (server dev, fixture bật), dữ liệu bịa", action: "mở trang; cuộn tới luồng nhóm", cauHinh: cfg, expected: "bảng Nếp đang nghĩ / đang viết / xong và hàng Rủ Đi AI trong luồng nhóm (người hỏi, thành viên khác): hiển thị đủ, không tràn, không cắt, không che", status: "PASS", evidence: [`EV-N22-LAB-${cfg}`, `EV-N22-LAB-NHOM-${cfg}`, "EV-N22-LAB-ghep"], ghiChu: `đầu trang: «Dữ liệu tổng hợp», «Reduce Motion», «Chạy thử», ba thẻ bảng Nếp (câu hỏi phải, «Nếp đang nghĩ…»; chữ đang viết dừng giữa câu; câu trả lời trọn với chip «Mở bản đồ», «Gợi ý quán gần hồ»); nửa dưới: trích tin nhờ có viền trái màu AI, khung «Rủ Đi AI đang đọc 6 tin… · Câu trả lời sẽ hiện ngay dưới tin của bạn / tin nhờ này», chữ đang viết, chữ ký «✦ Rủ Đi AI» dưới khung (chuKy, có chủ đích)${cfg === "C2" ? "; ở 320 câu phụ xuống hai dòng, tiêu đề «Luồng nhóm · thành viên khác» xuống hai dòng, không cắt" : cfg === "C3" ? "; nền tối đọc được" : ""}. Tràn 0, cắt chữ 0, không ellipsis. Không vùng aria-live nào: TC-N22-LAB-LIVE (UI-165)` });

// ------------------------------------- rows read off measurements already taken
phanXu({ tc: "TC-N22-SAN-XEM", screen: "N22.S01", layer: "L-xem-boi-canh", state: "nhóm sẵn sàng (dựng), chip «Kèm N tin gần đây»", action: "chạm «Xem»", cauHinh: "C1", expected: "tấm «Những tin sẽ gửi kèm lời nhờ» liệt kê đúng N tin mà chip đếm; nói ai sẽ đọc lời nhờ và câu trả lời; không ảnh nào đi bằng ảnh", status: "PASS", evidence: ["EV-N22-SAN-XEM-C1", "EV-N22-XEM-GHIM-C1", "EV-N22-SAN-ghep"], ghiChu: "tấm liệt kê 40 tin, đúng số chip đếm; «Lời nhờ và câu trả lời hiện cho cả nhóm»; ảnh đi bằng chú thích, sticker bằng chữ «Sticker»; tên hiển thị và chữ đi nguyên văn. Ở C1 dòng đầu «Rủ Đi AI sẽ đọc đúng 40 tin…», tay cầm và «Đóng bảng» nằm dưới dải ghim: TC-N22-XEM-GHIM (UI-166). 40 thay vì 20: TC-N22-SAN-SO-TIN (UI-163)" });
phanXu({ tc: "TC-N22-XEM-CAO", screen: "N22.S01", layer: "L-xem-boi-canh", state: "tấm «Xem» của nhóm, 40 tin (dài hơn cửa sổ)", action: "đo chiều cao tấm (số đo của TC-N22-XEM-GHIM)", cauHinh: "C1,C2,C8", expected: "tấm cao ≤ 82% cửa sổ (DESIGN.md)", status: "FAIL", issue: "UI-040", evidence: ["EV-N22-XEM-GHIM-C1", "EV-N22-XEM-GHIM-C2", "EV-N22-XEM-GHIM-C8", "EV-N22-SAN-ghep"], ghiChu: "tấm y 88–844 (90%) ở C1, 51–640 (92%) ở C2, 19–460 (96%) ở C8: trần 82% chỉ áp cho ScrollView, hàng tay cầm 48 và lề đáy cộng thêm (Sheet.tsx:51, :219), đúng gốc UI-040. Hậu quả riêng ở chat có dải ghim: UI-166" });
phanXu({ tc: "TC-N22-GHIM-C2", screen: "N22.S01", state: "nhóm chat-test có «Tờ hẹn chung · bản 1» ghim; bề rộng 320", action: "đọc danh sách ellipsis của ảnh EV-N22-CHIP-C2 và EV-N22-SAN-CHIP-C2", cauHinh: "C2", expected: "nhãn của dải ghim (một nút mở tờ hẹn) đọc trọn", status: "FAIL", issue: "UI-023", evidence: ["EV-N22-CHIP-C2", "EV-N22-SAN-CHIP-C2", "EV-N22-CHIP-ghep"], ghiChu: "«Tờ hẹn chung · bản 1» thành «Tờ hẹn chung · b…» (thiếu 12px, mất số bản) ở cả hai ảnh; «Sửa cùng hội» và mũi tên giữ chỗ bên phải. C1, C3: đọc trọn" });
phanXu({ tc: "TC-N22-CHIP-HEP", screen: "N22.S01", state: "chip trên nút gửi ở bề rộng 320: nhóm và chat đôi chưa sẵn sàng; nhóm sẵn sàng (dựng)", action: "đọc khung chip (số đo của TC-N22-CHIP-CHUA-SAN-SANG, TC-N22-DOI-CHIP-CHUA-SAN-SANG, TC-N22-SAN-CHIP ở C2) và nhìn ảnh", cauHinh: "C2", expected: "chip gọn một dòng như ở 360 và 390 (ChipBoiCanh.tsx: «48dp touch targets on a one-line chip»); nếu phải xuống dòng thì ký hiệu đi cùng câu, không đứng riêng", status: "FAIL", issue: "UI-167", evidence: ["EV-N22-CHIP-C2", "EV-N22-DOI-CHIP-C2", "EV-N22-SAN-CHIP-C2", "EV-N22-CHIP-ghep"], ghiChu: "chưa sẵn sàng: chip 288×52 (C1 358×28, C3 328×28), ✦ đứng riêng dòng đầu, «Rủ Đi AI chưa sẵn sàng · Gửi như tin thường» ở dòng hai, ở cả nhóm lẫn chat đôi. Sẵn sàng: chip 288×82 (C1 358×42), «✦ Kèm 40 tin gần đây Xem» dòng đầu, «Chỉ gửi lời nhờ» rớt xuống dòng hai cách một khoảng trống. Chip vẫn đọc được, ô soạn và nút gửi vẫn trong cửa sổ" });
phanXu({ tc: "TC-N22-NEP-LIVE", screen: "N22.S03", layer: "L03", state: "chat-0 ở Khám phá, bảng Nếp; máy chủ không có khoá AI", action: "đọc vùng thông báo quanh «Nếp đang nghĩ…» và câu lỗi (số đo của TC-N22-NEP-KHONG-KHOA)", cauHinh: "C1", expected: "«Nếp đang nghĩ…» và câu «Nếp chưa trả lời được…» nằm trong vùng aria-live (polite), để trình đọc màn hình báo khi chúng tới", status: "FAIL", issue: "UI-165", evidence: ["EV-N22-NEP-C1", "EV-N22-SAN-ghep"], ghiChu: "aria-live null cho cả hai (đọc tại «Nếp đang nghĩ…» lúc 0 ms và câu lỗi lúc 256 ms): người dùng trình đọc màn hình gửi câu hỏi rồi không nghe gì, phải tự dò tới câu lỗi. Câu hỏi trả về ô nhập, gửi lại được" });

// ------------------------------------------- measured rows, now with issues
ganIssue("TC-N22-SAN-SO-TIN", ["C1"], "UI-163");
ganIssue("TC-N22-GUI-CHUA-SAN-SANG", ["C1"], "UI-164");
ganIssue("TC-N22-LAB-LIVE", ["C1"], "UI-165");
ganIssue("TC-N22-XEM-GHIM", ["C1", "C2", "C8"], "UI-166");
// Extensions of issues of the base audit, measured on this feature's screens.
ganIssue("TC-N22-SAN-CHIP", ["C1", "C2", "C8"], "UI-001");
ganIssue("TC-N22-LOI-GOI-THU-LAI-TAT", ["C1"], "UI-091");

// ------------------------------------------------------------ native rows
const LY_DO = {
  android: "không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)",
  ios: "không chạy được iOS: không có macOS / iOS Simulator",
};
for (const [screen, route] of [
  ["N22.S01", "/groups/[id]/chat của nhóm (chip bối cảnh, «Xem», «Chỉ gửi lời nhờ», hàng lời nhờ hỏng, câu trả lời hiện dần)"],
  ["N22.S02", "/groups/[id]/chat của chat đôi (cùng AI như nhóm, lời của hai người)"],
  ["N22.S03", "bảng Nếp (NepPhien: đang nghĩ, đang viết, lỗi)"],
]) {
  for (const nenTang of ["android", "ios"]) {
    const tc = `TC-${screen}-NATIVE`;
    if (hang.get(`${tc}|${nenTang}|-`)) continue;
    so.ghi({ feature: "N22", tc, screen, layer: "-", state: "mọi trạng thái", action: `mở ${route}`, nenTang, cauHinh: "-", expected: `như web, trên thiết bị ${nenTang === "android" ? "Android" : "iOS"}`, status: "BLOCKED", method: "STATIC", evidence: [], issue: null, ghiChu: LY_DO[nenTang] });
    moi++;
  }
}
// What this stack cannot run: a model behind the AI.
if (!hang.get("TC-N22-AI-THAT|web|C1")) {
  so.ghi({ feature: "N22", tc: "TC-N22-AI-THAT", screen: "N22.S01", layer: "-", state: "máy chủ có khoá AI (plan, chia_bill, hoi sẵn sàng thật)", action: "gửi tin @Rủ Đi, /plan, /chia-bill trong nhóm và chat đôi; hỏi Nếp", nenTang: "web", cauHinh: "C1", expected: "lời gọi được nhận; «đang đọc N tin», chữ hiện dần qua SSE ở người hỏi và qua khung `ai` của WebSocket phòng ở thành viên khác; thẻ thật thay hàng đang viết; giảm chuyển động (C9) thì chữ tới từng câu; Nếp trả lời thật (ADR-0046, ADR-0045)", status: "BLOCKED", method: "RUNTIME-WEB", evidence: [], issue: null, ghiChu: "stack cục bộ không có khoá AI: mọi phòng báo provider_unavailable (TC-N22-API-CAP), lời gọi bị từ chối 503 trước khi có gì để stream. Trạng thái chữ hiện dần chỉ đo trên trang lab bằng chính thành phần app (TC-N22-LAB-*), C9 qua công tắc «Reduce Motion» của lab (cùng prop giamChuyenDong mà màn thật lấy từ useMotion: STATIC). Chất lượng câu trả lời, độ trễ thật, thứ tự khung WebSocket, chuyển giao sang thẻ: chưa đo được" });
  moi++;
}

// --------------------------------------- two dates that read as a phone number
// Same guard as n15-phan-xu.mjs (incident 34): a bare ISO date beside another reads, to the
// repo guard's Vietnamese mobile pattern, as a ten-digit number; each one is wrapped in «».
const GIONG_SO_DT = /(?<!\d)(?:(?:\+|00)?84(?:[ ().-]*0)?|0)[ ().-]*(?:[35789](?:[ ().-]*\d){8}|2(?:[ ().-]*\d){8,9})(?!\d)/;
hang = cuoiCung();
for (const r of hang.values()) {
  if (r.feature !== "N22" || !GIONG_SO_DT.test(r.ghiChu ?? "")) continue;
  const { luc: _luc, ...giu } = r;
  so.ghi({ ...giu, ghiChu: r.ghiChu.replace(/(?<!«)\b(\d{4}-\d{2}-\d{2})\b(?!»)/g, "«$1»") });
  moi++;
}

// ------------------------------------------------------------------- check
hang = cuoiCung();
const n22 = [...hang.values()].filter((r) => r.feature === "N22");
const thieu = n22.filter((r) => r.status === "FAIL" && !r.issue).map((r) => `${r.tc} ${r.cauHinh}`);
const dem = {};
for (const r of n22) dem[r.status] = (dem[r.status] ?? 0) + 1;
console.log(`n22-phan-xu: ${moi} dòng mới; N22 có ${n22.length} hàng ${JSON.stringify(dem)}; FAIL thiếu issue: ${thieu.join(", ") || "không"}`);
if (thieu.length) process.exitCode = 1;
