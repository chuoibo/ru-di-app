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
// A NOT_TESTED placeholder (written below at checkpoint 1) is not a measurement:
// it never stops a measured row from being carried over.
const daDo = (tc) => daGhi(tc, (r) => !r.rut && r.status !== "NOT_TESTED");
const tuHang = (issue, { feature, screen, layer = "-", expected, tu, ghi, phan = null }) => {
  const tc = `TC-R-${issue}${phan ? `-${phan}` : ""}`;
  if (daDo(tc)) return;
  const nguon = tu.map(([t, c]) => hangDo(t, c));
  const status = nguon.every((r) => r.status === "PASS") ? "PASS" : "FAIL";
  so.ghi({ feature, issue, tc, nenTang: "web", method: "RUNTIME-WEB", layer, screen, state: "đo bằng kịch bản gốc chạy lại trên main", action: nguon.map((r) => `${r.tc} ${r.cauHinh}`).join(", "), cauHinh: [...new Set(tu.map(([, c]) => c))].join(","), expected, status, evidence: [...new Set(nguon.flatMap((r) => r.evidence ?? []))], ghiChu: `${status === "PASS" ? "hết" : "còn"}: ${ghi(nguon)}` });
};
/** A retest row decided by looking at captures (automatic rows that could not decide, or decided wrong). */
const nhinR = (issue, { phan = null, ...rec }) => {
  const tc = `TC-R-${issue}${phan ? `-${phan}` : ""}`;
  if (daDo(tc)) return;
  so.ghi({ nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue, ...rec, tc, ghiChu: `phân xử bằng mắt: ${rec.ghiChu}` });
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

// ------------------------------------------------ checkpoint retest 2: the P3 issues
// Rows of base scripts re-run on main (f02 … f05, f10) carried over as above; verdicts
// reached by looking at captures where the automatic row could not decide or decided wrong.
// Withdrawn with their reason: TC-R-UI-102 and TC-R-UI-105 (retest-main.mjs measured the
// frame of expo-image's zero-height wrapper; measured again on the first box with a height).

// F00. UI-009: the automatic row passed on 54 characters that are the tab labels.
phanXu({ feature: "F00", tc: "TC-F00-RESUME-CHAM", screen: "F00.S01", layer: "L35", state: "khôi phục phiên chậm (trễ 5 s giả lập)", action: "mở /plan lạnh", cauHinh: "C1", expected: "có chỉ báo đang tải trong lúc chờ, không trang trắng", status: "FAIL", evidence: ["EV-F00-RESUME-CHAM-1800ms-C1", "EV-F00-RESUME-CHAM-6800ms-C1"], ghiChu: "ở 1,8 s vùng nội dung trống, chỉ còn thanh tab (54 ký tự là nhãn năm tab), 0 progressbar, 0 skeleton; 6,8 s thì Lên plan hiện. Hàng tự động xanh vì đếm chữ của thanh tab" });
nhinR("UI-009", { feature: "F00", screen: "F00.S01", layer: "L35", state: "mở lạnh /plan khi có cookie phiên, resume trễ 5 s", action: "đọc màn ở 1,8 s và 6,8 s", cauHinh: "C1", expected: "trễ resume 5 s: có progressbar hoặc skeleton trong ≤ 300 ms", status: "FAIL", evidence: ["EV-F00-RESUME-CHAM-1800ms-C1"], ghiChu: "còn: ở 1,8 s chỉ có thanh tab trên nền trống, không progressbar, không skeleton (TC-F00-RESUME-CHAM)" });

// F02: the base script's rows on main, and what the frames show. The four MO12 rows
// could not decide by themselves («cần đọc ảnh ghép»); these are the composites read.
phanXu({ feature: "F02", tc: "TC-MO12-BAT", screen: "F02.S01", state: "mở tab Khám phá lần đầu", action: "chạm tab", cauHinh: "C1", expected: "các lớp dựng lên lần lượt rồi dừng ở tư thế đứng", status: "PASS", evidence: ["EV-F02-MO12-bat-C1"], ghiChu: "19 khung: từ 331 ms vùng sân khấu mở ra, đồi rồi cây rồi tách và mặt trời lên lần lượt, từ khoảng 600 ms đứng yên ở tư thế cuối. Danh sách bị đẩy xuống khi sân khấu chen vào là UI-025" });
phanXu({ feature: "F02", tc: "TC-MO12-BAT", screen: "F02.S01", state: "mở tab Khám phá lần đầu", action: "chạm tab", cauHinh: "C9", expected: "giảm chuyển động: sân khấu hiện đứng sẵn, không dựng từng lớp", status: "PASS", evidence: ["EV-F02-MO12-bat-C9"], ghiChu: "8 khung: 219 ms chưa có sân khấu, 256 ms sân khấu hiện trọn ở tư thế đứng trong một khung và đứng yên tới cuối. Cú đẩy danh sách xuống 149dp lúc sân khấu chen vào là UI-025" });
phanXu({ feature: "F02", tc: "TC-MO12-BO-LOC", screen: "F02.S01", state: "đang lọc «Cafe»", action: "bỏ lọc", cauHinh: "C1", expected: "sân khấu quay lại (ExploreLive ghi «dựng một lần mỗi thành phố»)", status: "FAIL", issue: "UI-026", evidence: ["EV-F02-MO12-bo-loc-C1"], ghiChu: "sân khấu mount lại và dựng lại từng lớp từ đầu, khoảng 1 s" });
phanXu({ feature: "F02", tc: "TC-MO12-BO-LOC", screen: "F02.S01", state: "đang lọc «Cafe», giảm chuyển động", action: "bỏ lọc", cauHinh: "C9", expected: "sân khấu quay lại, đứng sẵn, không khung trống", status: "PASS", evidence: ["EV-F02-MO12-bo-loc-C9"], ghiChu: "từ khung 118 ms sân khấu đứng trọn, không khung nào trống (audit gốc: trống khoảng 430–460 ms, UI-027)" });
tuHang("UI-025", { feature: "F02", screen: "F02.S01", expected: "lần đầu mở Khám phá: dòng «N nơi ở …» không bị đẩy xuống sau khi đã hiện, ở C1 và C9", tu: [["TC-F02-NHAY", "C1"], ["TC-F02-NHAY", "C9"]], ghi: (ns) => ns.map((r) => `${r.cauHinh}: ${g(r)}`).join("; ") });
nhinR("UI-026", { feature: "F02", screen: "F02.S01", state: "đang lọc «Cafe»", action: "bỏ lọc, đọc ảnh ghép khung", cauHinh: "C1", expected: "bỏ lọc hay xoá tìm không dựng lại sân khấu từ đầu", status: "FAIL", evidence: ["EV-F02-MO12-bo-loc-C1"], ghiChu: "còn: bỏ lọc làm sân khấu mount lại và dựng lại từng lớp trong khoảng 1 s (ảnh ghép 16 khung, TC-MO12-BO-LOC C1)" });
nhinR("UI-027", { phan: "KHAM-PHA", feature: "F02", screen: "F02.S01", state: "giảm chuyển động, đang lọc «Cafe»", action: "bỏ lọc, đọc ảnh ghép khung", cauHinh: "C9", expected: "ở C9 không khung nào có vùng sân khấu trống", status: "PASS", evidence: ["EV-F02-MO12-bo-loc-C9"], ghiChu: "hết: 6 khung trong khoảng 0,9 s sau khi bỏ lọc; từ khung 118 ms sân khấu đã đứng trọn (đồi, cây, mặt trời, tách), không khung nào trống (TC-MO12-BO-LOC C9)" });
tuHang("UI-027", { phan: "M5", feature: "F03", screen: "F03.S03", expected: "Nếp M5 ở C9 chỉ có một ảnh từ lúc hiện", tu: [["TC-MO-M5", "C9"]], ghi: ([r]) => g(r).slice(0, 300) });
nhinR("UI-027", { phan: "BANG-DEV", feature: "F10", screen: "F10.S01", state: "bảng dev /dev/ui-lab, renderer skia, giảm chuyển động", action: "chạm «Chạy lại», đọc ảnh ghép khung", cauHinh: "C9", expected: "ở C9 không khung nào có vùng sân khấu trống", status: "FAIL", evidence: ["EV-F10-CHAY-LAI-C9"], ghiChu: "còn: sân khấu «bật dựng» trống ở khung 590 và 619 ms, hiện đứng sẵn từ 1026 ms (TC-F10-CHAY-LAI C9, server dev của main)" });
phanXu({ feature: "F10", tc: "TC-F10-CHAY-LAI", screen: "F10.S01", state: "renderer trên trang: skia", action: "chạm «Chạy lại»", cauHinh: "C9", expected: "giảm chuyển động: sân khấu hiện đứng sẵn, không khung nào trống", status: "FAIL", evidence: ["EV-F10-CHAY-LAI-C9"], issue: "UI-027", ghiChu: "trên main: khung 590 và 619 ms vùng sân khấu trống, 1026 ms hiện đứng sẵn" });
// UI-027 is measured in three parts; its one-row placeholder of checkpoint 1 goes.
rut("TC-R-UI-027", "thay bằng ba hàng theo phần: TC-R-UI-027-KHAM-PHA (Khám phá ở C9), TC-R-UI-027-M5 (Nếp M5 ở C9), TC-R-UI-027-BANG-DEV (bảng dev ở C9)");
nhinR("UI-028", { feature: "F02", screen: "F02.S02", state: "đổi điểm đến sang Hội An (chưa có quán)", action: "đọc trạng thái rỗng", cauHinh: "C1", expected: "thành phố chưa có quán: câu nói đúng lý do, không khuyên bỏ lọc khi không lọc gì", status: "FAIL", evidence: ["EV-F02-HOI-AN-C1"], ghiChu: "còn: «0 nơi ở Hội An», «Chưa thấy nơi phù hợp» kèm lời khuyên thử từ khoá khác, bỏ bớt bộ lọc, dù không có bộ lọc nào (TC-F02-DIEM-DEN)" });
nhinR("UI-029", { feature: "F02", screen: "F02.S01", layer: "L35", state: "danh mục trả 503; và mất mạng", action: "đọc câu của hai lỗi", cauHinh: "C1", expected: "lỗi máy chủ và mất mạng nói đúng nguyên nhân, khác nhau", status: "FAIL", evidence: ["EV-F02-LOI-503-C1", "EV-F02-OFFLINE-C1"], ghiChu: "đổi, vẫn trượt: câu mặc định cũ về kiểm tra mạng đã thay bằng «Chưa tải được nội dung. Bạn thử lại nhé.», nhưng câu này dùng chung cho cả 503 lẫn mất mạng, không nói nguyên nhân nào (TC-F02-LOI-503, TC-F02-OFFLINE)" });
nhinR("UI-030", { feature: "F02", screen: "F02.S01", layer: "L35", state: "đã tải danh sách, rồi mất mạng", action: "đổi tab rồi quay lại Khám phá", cauHinh: "C1", expected: "mất mạng: danh sách đã tải vẫn còn, kèm câu báo mất mạng", status: "FAIL", evidence: ["EV-F02-OFFLINE-C1"], ghiChu: "còn: quay lại tab thì danh sách đã tải bị thay bằng màn lỗi (TC-F02-OFFLINE; hàng tự động chỉ kiểm chữ tiếng Anh)" });

// F03.
tuHang("UI-037", { feature: "F03", screen: "F03.S03", expected: "kèo ở Lịch trình có mép Nếp; Bản đồ thì không", tu: [["TC-F03-NEP-LICH-TRINH", "C1"]], ghi: ([r]) => g(r).replace(/[0-9a-f]{8}-[0-9a-f-]{27}/g, "[id]") });
tuHang("UI-038", { feature: "F03", screen: "F03.S03", layer: "L08", expected: "Back khi sheet mở: sheet đóng, URL giữ nguyên", tu: [["TC-L08-DONG-back", "C1"]], ghi: ([r]) => g(r) });
tuHang("UI-040", { feature: "F03", screen: "F03.S03", layer: "L08", expected: "C8: sheet ≤ 82%", tu: [["TC-L08-KICH-THUOC", "C8"]], ghi: ([r]) => `${g(r)} (audit gốc 93%; main thêm ô tìm địa điểm vào sheet)` });
{
  const r = hangDo("TC-L10-NEN", "C1");
  if (!String(r.ghiChu).startsWith("phân xử bằng mắt:")) so.ghi({ ...r, status: "FAIL", issue: "UI-041", ghiChu: `phân xử bằng mắt: đầu màn (Quay lại, tiêu đề, công tắc) không bị làm mờ trong khi phần bản đồ dưới đã mờ; chạm công tắc không đổi chế độ mà cũng không đóng sheet (số đo tự động: ${r.ghiChu})` });
}
nhinR("UI-041", { feature: "F03", screen: "F03.S04", layer: "L10", state: "Bản đồ, sheet «Sửa trang ngày» (tiêu đề nay «Những hẹn quan trọng») mở", action: "chạm công tắc «Lịch trình» phía trên sheet", cauHinh: "C1", expected: "khi sheet mở, đầu màn bị làm mờ và chạm vào thì đóng sheet", status: "FAIL", evidence: ["EV-F03-SUA-NGAY-C1"], ghiChu: "còn: đầu màn sáng như thường trên nền đã mờ; chạm vào không đổi chế độ, sheet vẫn mở (TC-L10-NEN)" });
tuHang("UI-043", { feature: "F03", screen: "F03.S04", layer: "L12", expected: "không còn nhãn tiếng Anh; Esc đóng popup", tu: [["TC-F03-BAN-DO-NHAN", "C1"], ["TC-L12-CUM", "C1"]], ghi: (ns) => ns.map((r) => `${r.tc.replace("TC-", "")}: ${g(r)}`).join("; ") });
tuHang("UI-044", { feature: "F03", screen: "F03.S01", expected: "C2: không dòng nào mất số chặng", tu: [["TC-F03-META", "C2"]], ghi: ([r]) => `${g(r)}; trên ảnh: «24/10/2026 · 8 người · 12 chặ…», dòng kèo rỗng mất cả «Chưa có chặng nào»` });
tuHang("UI-045", { feature: "F03", screen: "F03.S03", expected: "C2: cột tên ≥ 120px", tu: [["TC-F03-COT-CHANG", "C2"]], ghi: ([r]) => g(r) });
tuHang("UI-046", { feature: "F03", screen: "F03.S05", expected: "mở /outings/chon: một câu và một lối ra trong ≤ 1 s", tu: [["TC-F03-CHON-THIEU", "C1"]], ghi: ([r]) => g(r) });
tuHang("UI-047", { feature: "F03", screen: "F03.S03", expected: "C6/C7: nút Quay lại thẳng mép trái cột nội dung", tu: [["TC-F03-DAU-MAN", "C6"], ["TC-F03-DAU-MAN", "C7"]], ghi: (ns) => ns.map((r) => `${r.cauHinh}: ${g(r)}`).join("; ") });

// F04.
tuHang("UI-053", { feature: "F04", screen: "F04.S01", expected: "Space đổi aria-checked ở ghế và ô danh sách", tu: [["TC-F04-BAN-PHIM", "C1"]], ghi: ([r]) => g(r) });
tuHang("UI-054", { feature: "F04", screen: "F04.S03", expected: "nhóm 10 người ở 288–398: 0 cặp nhãn đè, 0 nhãn ra ngoài khung", tu: [["TC-F04-QT-20", "C1"], ["TC-F04-QT-20", "C2"]], ghi: (ns) => `${ns.map((r) => `${r.cauHinh}: ${g(r)}`).join("; ")}; ca 10 người của audit gốc là phép tính tĩnh trên hinh-tien.ts, SoDoChuyen.tsx: hai file giống hệt giữa 7ea1a7c và 461eabf, nên số 10 người giữ nguyên (4 cặp đè, 2 nhãn ra ngoài)` });
tuHang("UI-055", { feature: "F04", screen: "F04.S01", expected: "hộp Nếp M2 nằm trọn trong màn ở C1–C3", tu: [["TC-F04-NEP-M2-KHUNG", "C1"], ["TC-F04-NEP-M2-KHUNG", "C2"], ["TC-F04-NEP-M2-KHUNG", "C3"]], ghi: (ns) => ns.map((r) => `${r.cauHinh}: ${g(r)}`).join("; ") });
tuHang("UI-056", { feature: "F04", screen: "F04.S01", expected: "sau khi đọc hỏng, lối nhập tay có ngay trên màn", tu: [["TC-F04-ANH-DOC", "C1"]], ghi: ([r]) => g(r) });
tuHang("UI-057", { feature: "F04", screen: "F04.S03", expected: "ở màn tiền, không phần tử nào có nhãn hứa hành động mà không làm", tu: [["TC-F04-NEP-MEP", "C1"]], ghi: ([r]) => g(r) });
tuHang("UI-058", { feature: "F04", screen: "F04.S03", expected: "sổ không còn khoản ngoài đợt thì không có nút mời một việc chắc chắn bị từ chối", tu: [["TC-F04-DOT-RONG", "C1"]], ghi: ([r]) => g(r).replace(/[0-9a-f]{8}-[0-9a-f-]{27}/g, "[id]") });
nhinR("UI-059", { feature: "F04", screen: "F04.S01", state: "bước 5, trang sổ 20 dòng, người trả «Chat Test 01»", action: "sau «Ghi vào sổ» (lượt F04 trên main ở checkpoint retest 1)", cauHinh: "C1", expected: "ở C1–C3, dòng người trả luôn thấy chữ «trả»", status: "FAIL", evidence: ["EV-F04-DA-GHI-C1"], ghiChu: "còn: dòng đầu ghi «Chat Test 0…», phần bị cắt đúng là «(trả)»; chỉ đo ở C1, C2 và C3 chưa đo lại" });

// F05.
tuHang("UI-064", { feature: "F05", screen: "F05.S02", expected: "đáy avatar = đáy bong bóng cuối của cụm ± 4px; trong một quãng không nghỉ chỉ có một nhãn giờ", tu: [["TC-F05-BO-CUC-TIN", "C1 G8"], ["TC-F05-BO-CUC-TIN", "C1 G20"]], ghi: (ns) => ns.map((r) => `${r.cauHinh}: ${g(r).slice(0, 170)}`).join("; ") });
tuHang("UI-065", { feature: "F05", screen: "F05.S02", expected: "bình chọn 3 lựa chọn, 0 phiếu: thẻ ≤ 25% chiều cao C1; «phiếu» ≤ 1 lần khi chưa ai bầu", tu: [["TC-F05-BINH-CHON-THE", "C1 G8"]], ghi: ([r]) => `${g(r)}; vế C6 ở TC-R-UI-065-C6` });
tuHang("UI-066", { feature: "F05", screen: "F05.S02", layer: "L21", expected: "Esc đóng khay; focus về nút đã mở nó", tu: [["TC-L21-VONGDOI", "C1"]], ghi: ([r]) => g(r) });
tuHang("UI-068", { feature: "F05", screen: "F05.S02", expected: "không có câu lỗi khi tin đã hiện, hoặc câu có «Thử lại» và nói đúng phần hỏng", tu: [["TC-F05.S02-503", "C1"]], ghi: ([r]) => g(r) });
tuHang("UI-069", { feature: "F05", screen: "F05.S02", layer: "L22", expected: "khi đang đọc tin cũ: câu lỗi nằm trong vùng đang xem; thẻ không có biểu tượng hay màu của AI", tu: [["TC-L22-KHI-DOC-CU", "C1"]], ghi: ([r]) => `${g(r)}; trên ảnh EV-F05-THONG-BAO-C1 thẻ vẫn mang dấu lấp lánh và chữ «Rủ Đi» màu tím của AI` });
{
  const r = hangDo("TC-F05-GHIM-TREN-NEN", "C1");
  if (!String(r.ghiChu).startsWith("phân xử bằng mắt:")) so.ghi({ ...r, status: "FAIL", issue: "UI-070", ghiChu: `phân xử bằng mắt: dải «Tờ hẹn chung · bản 1 · Sửa cùng hội» sáng nguyên trên nền đã mờ khi «Cài đặt nhóm» mở; chạm vào chỉ đóng sheet, không mở tờ hẹn (số đo tự động: ${r.ghiChu})` });
}
nhinR("UI-070", { feature: "F05", screen: "F05.S02", state: "nhóm có tờ hẹn chung (dải ghim); sheet «Cài đặt nhóm» mở", action: "nhìn dải ghim, chạm vào nó", cauHinh: "C1", expected: "khi sheet mở, dải bị làm mờ như phần còn lại của màn", status: "FAIL", evidence: ["EV-F05-GHIM-TREN-NEN-C1"], ghiChu: "còn: dải sáng trên nền mờ, trông như còn dùng được; chạm vào chỉ đóng sheet (TC-F05-GHIM-TREN-NEN)" });

// F07. UI-092's first row passes on the day of the run only: the strip starts today, so the
// draft's 03/10 is its fourth leaf. The part XA moves the draft to the ninth day and fails.
{
  const r = hangDo("TC-R-UI-092", "C1,C8");
  if (!String(r.ghiChu).startsWith("phân xử bằng mắt:")) so.ghi({ ...r, ghiChu: `phân xử bằng mắt: đổi, không phải đã sửa: lá 03/10 thấy trọn chỉ vì dải ngày bắt đầu từ hôm nay (30/09) nên nó là lá thứ tư; DeNghiSua.tsx giống hệt bản gốc, không cuộn tới lá đang chọn; ngày xa hơn thì vẫn khuất (TC-R-UI-092-XA) (số đo: ${r.ghiChu})` });
}

// F10.
tuHang("UI-114", { feature: "F10", screen: "F10.S01", expected: "axe không còn nested-interactive ở cặp so sánh có ảnh; tim vẫn nằm trên góc ảnh và vẫn lưu được", tu: [["TC-F10-TIM-LONG", "C1"]], ghi: ([r]) => g(r).slice(0, 330) });
tuHang("UI-115", { feature: "F10", screen: "F10.S02", expected: "ở C1, kéo dọc bắt đầu trên tranh cuộn danh sách như bắt đầu ở chỗ khác", tu: [["TC-F10-KEO-DOC-TREN-TRANH", "C1"]], ghi: ([r]) => g(r) });

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
