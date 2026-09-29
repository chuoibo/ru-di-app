/* The retest ledger of main, completed by hand: verdicts reached by LOOKING,
 * rows of the base scripts that were re-run on main turned into the one-row-
 * per-issue shape of retest-main.mjs, and rows withdrawn with their reason.
 * Same shape as e-phan-xu.mjs. Run once, after retest-main.mjs; a second run
 * writes nothing new (every write checks the ledger first).
 *
 *   source audit-env-main.sh && node kich-ban/retest-phan-xu.mjs
 */
import { readFileSync } from "node:fs";

import { soGhi } from "../thu-vien/ghi.mjs";

const out = process.env.AUDIT_OUT;
if (!out) throw new Error("thiếu AUDIT_OUT");
const so = soGhi(out);

const hang = so.doc();
/** The last row of a key, withdrawals applied, as the matrix will read it. */
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
const hangDo = (tc, c) => {
  const r = cuoiCung().get(`${tc}|web|${c}`);
  if (!r) throw new Error(`không có hàng đo ${tc} ${c}`);
  const { luc: _luc, ...giu } = r;
  return giu;
};
const daGhi = (tc, dieuKien = () => true) => hang.some((r) => r.tc === tc && dieuKien(r));
const rut = (tc, lyDo) => {
  if (daGhi(tc, (r) => r.rut && r.lyDo === lyDo)) return;
  so.rut(tc, lyDo);
};
const phanXu = (rec) => {
  if (daGhi(rec.tc, (r) => String(r.ghiChu ?? "").startsWith("phân xử bằng mắt:") && r.cauHinh === rec.cauHinh)) return;
  so.ghi({ nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec, ghiChu: `phân xử bằng mắt: ${rec.ghiChu}` });
};

// ------------------------------------------------ rows withdrawn, with the reason
// The base F06 script, re-run on main before retest-main.mjs existed: its
// re-invite step sent nothing for moi-53, so the three rows after it measured a
// number nobody had invited. UI-073 and UI-074 were measured again, properly,
// by retest-main.mjs (r-f06: moi-61 invited through the UI).
const LY_DO_F06 = "lượt chạy lại kịch bản F06 gốc trên main: bước mời lại không gửi lời mời nào cho moi-53, nên hàng này đo trên một số chưa được mời; UI-073 và UI-074 đo lại ở retest-main.mjs (r-f06)";
for (const tc of ["TC-F06-DUOC-MOI-VAO-CUA", "TC-F05.S01-DONG-Y", "TC-F06-TU-BO-QUAN-TRI"]) rut(tc, LY_DO_F06);
rut("TC-F06-MOI-LAI", "lượt chạy lại kịch bản F06 gốc trên main: không phân định được lỗi app hay kịch bản cũ lệch với màn Mời của main (không thấy câu sau lần mời lại); không dùng làm bằng chứng");
// The first UI-117 row measured one path only; it is replaced by two rows, one per path.
rut("TC-R-UI-117", "thay bằng TC-R-UI-117-A (đường cũ, Back rời app: UI-123) và TC-R-UI-117-B (bắt đầu ở tab đầu Cộng đồng)");

// ------------------------------------------------ verdicts reached by looking
// The tab bar of main at C1, C2, C3, 599 and 600: the selected tab is drawn
// (accent colour, filled icon, washi strip) though aria-selected is missing (UI-003).
phanXu({ feature: "F00", tc: "TC-F00-TAB-DOI", screen: "F00.S03", layer: "L05", state: "đã đăng nhập (dalat-0), năm tab", action: "chạm lần lượt từng tab", cauHinh: "C1", expected: "URL và nội dung đổi theo tab được chạm; tab đang chọn có dấu hiệu nhìn thấy (màu + icon đặc + dải washi)", status: "PASS", evidence: ["EV-F00-TAB-ghep-a"], ghiChu: "URL đổi đúng theo từng tab (số đo tự động); trên ảnh ghép C1, C2, C3, 599 và 600, tab đang chọn «Khám phá» có màu nhấn, icon đặc và dải washi. Hàng tự động đỏ vì đọc aria-selected, thiếu trên web: UI-003 (TC-R-UI-003)" });
// UI-033: the measure passes; the capture adds that the explanation's second line is clipped.
{
  const r = hangDo("TC-R-UI-033", "C1,C2,C3,C4,C8");
  if (!String(r.ghiChu).startsWith("phân xử bằng mắt:")) so.ghi({ ...r, ghiChu: `phân xử bằng mắt: đổi: «Về Lịch trình» nay ghim thấy trọn ở cả năm cấu hình, tiêu chí đạt; trên ảnh C1, dòng thứ hai của lời giải thích nằm dưới nút, bị cắt (số đo tự động: ${r.ghiChu})` });
}
// UI-034: the automatic note read the input's placeholder attribute, empty; the
// capture shows «250000» drawn as a Text over the empty input (F44 lesson).
{
  const r = hangDo("TC-R-UI-034", "C1");
  if (!String(r.ghiChu).startsWith("phân xử bằng mắt:")) so.ghi({ ...r, ghiChu: `phân xử bằng mắt: còn: ô ngân sách trống nhưng trên ảnh vẫn hiện «250000» (chữ gợi ý vẽ bằng một Text đè lên ô, không phải thuộc tính placeholder, nên số đo tự động đọc ra rỗng); chạm «Tạo kèo» không tạo kèo, câu lỗi nằm ngoài khung nhìn (số đo tự động: ${r.ghiChu})` });
}
// UI-082, the notebook link: the second capture is byte-identical to the first
// (nothing on the demo page could be pressed), so only the first is kept.
{
  const r = hangDo("TC-R-UI-082-SO", "C1");
  if (!String(r.ghiChu).startsWith("phân xử bằng mắt:")) so.ghi({ ...r, evidence: ["EV-R-UI-082-A-C1"], ghiChu: `phân xử bằng mắt: còn, và đổi: không phiên, link tờ giấy của một cặp thật ở lại /groups/[id]/to-giay và hiện sổ demo «Hội bạn · Người ấy» (tờ «Thứ Bảy 06/09 · Bún chả, quán góc phố», «Chủ nhật 14/09»), không nhãn demo, không lối đăng nhập. Bản demo nay là sổ «hội bạn», không còn «Rủ đi chơi» nên không còn đường tới «Đã gửi» giả; ảnh thứ hai trùng từng byte với ảnh đầu, bỏ (số đo tự động: ${r.ghiChu})` });
}

// ------------------------------------------------ base rows re-run on main, as retest rows
// retest-main.mjs gives each issue one row; the base scripts f04-tien.mjs and
// f05-chat.mjs had already measured these on main, so their rows are carried
// over, status and evidence unchanged, with the issue's own criterion.
const tuHang = (issue, { feature, screen, layer = "-", expected, tu, ghi }) => {
  const tc = `TC-R-${issue}`;
  if (daGhi(tc)) return;
  const nguon = tu.map(([t, c]) => hangDo(t, c));
  const status = nguon.every((r) => r.status === "PASS") ? "PASS" : "FAIL";
  so.ghi({ feature, issue, tc, nenTang: "web", method: "RUNTIME-WEB", layer, screen, state: "đo bằng kịch bản gốc chạy lại trên main", action: nguon.map((r) => `${r.tc} ${r.cauHinh}`).join(", "), cauHinh: [...new Set(tu.map(([, c]) => c))].join(","), expected, status, evidence: [...new Set(nguon.flatMap((r) => r.evidence ?? []))], ghiChu: `${status === "PASS" ? "hết" : "còn"}: ${ghi(nguon)}` });
};
const g = (r) => String(r.ghiChu ?? "").replace(/\s+/g, " ").trim();
tuHang("UI-048", { feature: "F04", screen: "F04.S01", expected: "bill trên, bước 2 ở C1–C5 và bước 3 ở C2: 0 phần tử số tiền có scrollWidth > clientWidth; quyết toán demo ở C2 cũng vậy", tu: [["TC-F04-MON-DAI", "C1"], ["TC-F04-MON-DAI", "C2"], ["TC-F04-MON-DAI", "C3"], ["TC-F04-MON-DAI", "C4"], ["TC-F04-MON-DAI", "C5"]], ghi: (ns) => `${ns.map((r) => `${r.cauHinh}: ${g(r).match(/số tiền bị cắt [^;]*/)?.[0] ?? g(r).slice(0, 90)}`).join("; ")}; quyết toán demo ở C2 chưa đo lại` });
tuHang("UI-049", { feature: "F04", screen: "F04.S04", layer: "L32", expected: "ba trường hợp: (a) chép được link, (b) hàng ghi «Đã mở khay chia sẻ», (c) không có câu lỗi", tu: [["TC-L32-VONGDOI", "C1"]], ghi: ([r]) => g(r).slice(0, 320) });
tuHang("UI-050", { feature: "F04", screen: "F04.S01", expected: "chạm tâm mỗi hình nhân trúng đúng ghế, nhóm tới 20 người ở 288–700dp", tu: [["TC-F04-BAN-20", "C1"], ["TC-F04-BAN-20", "C2"], ["TC-F04-BAN-20", "C6"]], ghi: (ns) => ns.map((r) => `${r.cauHinh}: ${g(r)}`).join("; ") });
tuHang("UI-051", { feature: "F04", screen: "F04.S01", expected: "các trường hợp đã đo: câu nằm trong khung nhìn ngay sau khi chạm", tu: [["TC-F04-CHAN-TEN", "C1"], ["TC-F04-CHAN-NGUOI", "C1"], ["TC-F04-BILL-503", "C1"]], ghi: (ns) => `${ns.map((r) => `${r.tc.replace("TC-F04-", "")}: ${g(r).slice(0, 150)}`).join("; ")}; trường hợp 4 (ảnh bill máy chủ không đọc được) chưa đo lại` });
tuHang("UI-052", { feature: "F04", screen: "F04.S01", expected: "về bước 1, Back rồi Forward, tải lại: không mất món nào mà không hỏi", tu: [["TC-F04-LUI-VE-BUOC1", "C1"], ["TC-F04-BACK-TRINH-DUYET", "C1"], ["TC-F04-TAI-LAI", "C1"]], ghi: (ns) => ns.map((r) => `${r.tc.replace("TC-F04-", "")}: ${g(r).slice(0, 140)}`).join("; ") });
tuHang("UI-062", { feature: "F05", screen: "F05.S02", expected: "web C1: ô trống thì giữa dòng chữ lệch ≤ 4px so với giữa «+» và nút gửi; gõ 7 dòng thì ô cao tới 120 rồi mới cuộn", tu: [["TC-F05-SOAN-CAN", "C1"], ["TC-F05-SOAN-NHIEU-DONG", "C1"]], ghi: (ns) => ns.map((r) => `${r.tc.replace("TC-F05-", "")}: ${g(r)}`).join("; ") });
tuHang("UI-063", { feature: "F05", screen: "F05.S02", expected: "tin có link dài ở C1, C2, C3, C5: bong bóng nằm trong [0, bề rộng cửa sổ] và ≤ 82% hàng", tu: [["TC-F05-URL-DAI", "C1"], ["TC-F05-URL-DAI", "C2"], ["TC-F05-URL-DAI", "C3"], ["TC-F05-URL-DAI", "C5"]], ghi: (ns) => ns.map((r) => `${r.cauHinh}: ${g(r)}`).join("; ") });
tuHang("UI-071", { feature: "F06", screen: "F06.S01", expected: "tạo nhóm xong tới nơi thấy nhóm vừa mở (chat hoặc mời), hoặc có câu xác nhận kèm lối mời bạn", tu: [["TC-F06-TAO-SAU", "C1"]], ghi: ([r]) => g(r) });

// ------------------------------------------------ not measured yet: NOT_TESTED, never silent
// Every issue of the base audit without a retest row gets one, NOT_TESTED, and
// so does each feature new on main that the queue has not reached. A later
// measurement replaces the row (the last row of a key wins).
const goc = readFileSync(new URL("../../../../docs/claude/2026-09-27/mobile-ui-audit/issues.md", import.meta.url), "utf8");
const coHangR = (id) => so.doc().some((r) => !r.rut && (r.tc === `TC-R-${id}` || r.tc.startsWith(`TC-R-${id}-`)));
let chua = 0;
for (const m of goc.matchAll(/^### (UI-\d{3}) · [^\n]*\n([\s\S]*?)(?=^### |^## |(?![\s\S]))/gm)) {
  if (coHangR(m[1])) continue;
  const fs = m[2].match(/^\| Feature[^|]*\| ([^|]*)\|/m)?.[1] ?? "";
  const feature = fs.match(/\b(F\d{2}|E\d)\b/)?.[1] ?? "?";
  const tieuChi = m[2].match(/^\| Tiêu chí gỡ \| ([^|]*)\|/m)?.[1]?.trim() ?? "tiêu chí gỡ của issue";
  so.ghi({ feature, issue: m[1], tc: `TC-R-${m[1]}`, nenTang: "web", method: "RUNTIME-WEB", layer: "-", screen: "-", state: "-", action: "chưa đo lại trên main", cauHinh: "-", expected: tieuChi, status: "NOT_TESTED", evidence: [], ghiChu: "chưa đo lại ở checkpoint retest 1 (P1 và P2 trước); để checkpoint sau" });
  chua++;
}
const MOI = [
  ["N14", "TC-N-14-CONG-DONG", "tab Cộng đồng: bảng tin kiểm duyệt, đăng bài, thông báo, realtime (#14)"],
  ["N15", "TC-N-15-NHAT-KY", "Sổ kỷ niệm Nếp v3 / Nhật ký chuyến, màn kết thúc kèo (#657, #15)"],
  ["N21", "TC-N-21-HO-SO", "hồ sơ kể chuyện, sổ huy hiệu nhiều ngã rẽ, tường cá nhân v2 (#658, #21)"],
  ["N22", "TC-N-22-AI-CHAT", "Rủ Đi AI trong chat, chat hai người (#654, #659, #22)"],
  ["N26", "TC-N-26-HAI-LOP-CHAT", "hai lớp chat «đám bạn» / «cặp đôi» (#660, #26)"],
];
for (const [feature, tc, ten] of MOI) {
  if (daGhi(tc)) continue;
  so.ghi({ feature, tc, nenTang: "web", method: "RUNTIME-WEB", layer: "-", screen: "-", state: "-", action: `audit feature mới: ${ten}`, cauHinh: "-", expected: "audit đủ chuỗi feature → màn → lớp → trạng thái như audit gốc", status: "NOT_TESTED", evidence: [], ghiChu: "feature mới trên main, trong hàng đợi sau retest; chưa audit" });
  chua++;
}
console.log(`retest-phan-xu: xong; ${chua} hàng NOT_TESTED mới`);
