/* F10 verdicts reached by LOOKING, with the capture that was looked at, and
 * the issue each measured row of f10-bang-dev.mjs now points at. Same shape
 * as f09-phan-xu.mjs.
 *
 *   node kich-ban/f10-phan-xu.mjs
 *
 * It first repairs a test-id collision: the first F10 run wrote its stage
 * rows as TC-MO12-KEO and TC-MO12-GAP, and TC-MO12-KEO is also F02's row (the
 * tilt is not wired on Khám phá, NOT_APPLICABLE). The ledger keys rows by
 * tc|platform|configuration, so F10's rows displaced F02's. The F10 rows now
 * live under TC-F10-MO12-*; here the old ids are withdrawn and F02's last
 * rows are written back unchanged. Safe to run twice: the repair checks first.
 */
import { soGhi } from "../thu-vien/ghi.mjs";

const out = process.env.AUDIT_OUT;
if (!out) throw new Error("thiếu AUDIT_OUT");
const so = soGhi(out);

// ------------------------------------------------------ id collision repair
{
  const tatCa = so.doc();
  const cuoi = new Map();
  for (const r of tatCa) {
    if (r.tc !== "TC-MO12-KEO") continue;
    if (r.rut) { cuoi.clear(); continue; }
    cuoi.set(r.cauHinh, r);
  }
  const conF10 = [...cuoi.values()].some((r) => r.feature !== "F02");
  if (conF10) {
    const f02 = new Map();
    for (const r of tatCa) if (r.tc === "TC-MO12-KEO" && r.feature === "F02" && !r.rut) f02.set(r.cauHinh, r);
    so.rut("TC-MO12-KEO", "trùng ID: lượt F10 đầu ghi hàng kéo nghiêng dưới TC-MO12-KEO, trùng hàng N/A của F02 (Khám phá không nối nghiêng) cùng khoá tc|web|cấu hình, nên đè hàng F02; tiêu chí F10 còn gộp hai ý. Hàng F02 được ghi lại nguyên văn ngay sau đây; F10 đo lại dưới TC-F10-MO12-KEO và TC-F10-KEO-DOC-TREN-TRANH");
    for (const r of f02.values()) {
      const { luc: _luc, ...giu } = r;
      so.ghi(giu);
    }
  }
  const gap = tatCa.filter((r) => r.tc === "TC-MO12-GAP");
  if (gap.length && !gap.at(-1).rut) so.rut("TC-MO12-GAP", "đổi ID thành TC-F10-MO12-GAP để không dùng chung dải MO12 với feature khác (xem TC-MO12-KEO); số đo không đổi, đã đo lại dưới ID mới");
}

const phanXu = (rec) =>
  so.ghi({ feature: "F10", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec, ghiChu: `phân xử bằng mắt: ${rec.ghiChu}` });

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
    if (!r) throw new Error(`không có hàng đo ${tc} ${c}; chạy f10-bang-dev.mjs trước`);
    const { luc: _luc, ...giu } = r;
    so.ghi({ ...giu, issue });
  }
};

// ---------------------------------------------------------------- baselines
phanXu({ tc: "TC-F10.S01-BASE", screen: "F10.S01", state: "bảng dev, dữ liệu tổng hợp, chưa đăng nhập", action: "mở /dev/ui-lab trên server dev (fixture) và chụp từng cửa sổ khi cuộn", cauHinh: "C1,C2,C3", expected: "Bảng component (dev): hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt", status: "FAIL", issue: "UI-113", evidence: ["EV-F10.S01-BASE-C2-21", "EV-F10-SO-SANH-C2"], ghiChu: "đã xem 23 cửa sổ ở C1, 30 ở C2, 24 ở C3: đọc được ở sáng và tối, không tràn, không che. Một chỗ cắt thật ở C2: tim «Lưu Still Cafe» của cặp so sánh không ảnh bị đẩy qua mép phải (cửa sổ 21, TC-F10-SO-SANH-TIM). «−/+» 33×48 thuộc CauRu, chỉ bảng dev dùng. axe: aria-prohibited-attr ×29 (div «Sticker: …» không role, UI-089), aria-required-attr ×4 (mặt quay và tay nắm thứ tự, UI-036/UI-042), nested-interactive ×1 (mặt quay, UI-042)" });
phanXu({ tc: "TC-F10.S02-BASE", screen: "F10.S02", state: "bảng dev, dữ liệu tổng hợp", action: "mở /dev/san-khau (và ?canh=1)", cauHinh: "C1,C2,C3", expected: "Sân khấu gập (dev): hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt", status: "PASS", evidence: ["EV-F10.S02-BASE-ghep"], ghiChu: "sân khấu «Phố đêm» đứng, nhãn «Màn thử · dữ liệu tổng hợp», tiêu đề, câu dẫn, danh sách 18 dòng; C2 gọn, C3 tối đọc được; ?canh=1 hiện cảnh «Chưa có hội». axe: scrollable-region-focusable ×1 (danh sách toàn chữ, không phần tử nhận focus; chỉ ở bảng dev)" });

// ------------------------------------------------------- judged from captures
phanXu({ tc: "TC-F10-MO12-GAP", screen: "F10.S02", state: "sân khấu đứng yên ở đầu màn", action: "cuộn danh sách xuống từng nấc 40dp", cauHinh: "C1", expected: "sân khấu gập vào trang theo cuộn (đọc ảnh), không nhảy", status: "PASS", evidence: ["EV-F10-GAP-C1-ghep"], ghiChu: "cuộn 40: sân khấu còn đứng, trượt lên dưới thanh; cuộn 80: các lớp đã nằm dẹt thành một dải mỏng; cuộn 120: hết, còn tiêu đề và câu dẫn. Không nhảy khung nào" });
phanXu({ tc: "TC-F10-CHAY-LAI", screen: "F10.S01", state: "renderer trên trang: skia", action: "chạm «Chạy lại»", cauHinh: "C1", expected: "Nếp dựng lên và đường mực tự vẽ lại (đọc ảnh ghép)", status: "PASS", evidence: ["EV-F10-CHAY-LAI-C1"], ghiChu: "499 ms: cả hai mục trống; 546–658 ms: sân khấu ghế dựng từ vạch sàn; 1155–1419 ms: Nếp hiện; 2070–2317 ms: đường mực tự vẽ tới hết" });
phanXu({ tc: "TC-F10-CHAY-LAI", screen: "F10.S01", state: "renderer trên trang: skia", action: "chạm «Chạy lại»", cauHinh: "C9", expected: "giảm chuyển động: Nếp và đường mực hiện ở tư thế cuối, không diễn", status: "FAIL", issue: "UI-027", evidence: ["EV-F10-CHAY-LAI-C9"], ghiChu: "Nếp và đường mực đứng yên ở tư thế cuối suốt 1,8 s (đạt); nhưng sân khấu ghế ngay dưới biến mất ở 603 và 704 ms rồi hiện lại đứng sẵn ở 1098 ms: khoảng trống khoảng 0,5 s khi mount lại, như UI-027 đo ở Khám phá" });
phanXu({ tc: "TC-F10-TAM-STICKER", screen: "F10.S01", state: "bảng chú thích của trang dev: tám sticker ở cỡ 120 và 64 kèm tên", action: "cuộn tới bảng", cauHinh: "C2", expected: "tên mỗi sticker hiện trọn trên một dòng", status: "NOT_APPLICABLE", evidence: ["EV-F10-TAM-STICKER-C2"], ghiChu: "«Cà phê k…» và «Về tới ch…» bị «…» ở C2, nhưng đó là chú thích riêng của bảng dev (hàng Text của ui-lab), không có trong app. Tên trong khay thật đọc trọn, xuống hai dòng khi cần (TC-F10-KHAY-STICKER-TEN)" });

// ------------------------------------------- measured rows, now with issues
ganIssue("TC-F10-SO-SANH-TIM", ["C2"], "UI-113");
ganIssue("TC-F10-TIM-LONG", ["C1"], "UI-114");
ganIssue("TC-F10-KEO-DOC-TREN-TRANH", ["C1"], "UI-115");
ganIssue("TC-F10-BAN-XOAY-LONG", ["C1"], "UI-042");
ganIssue("TC-F10-ALBUM-XEM", ["C1"], "UI-094");
ganIssue("TC-F10-XEM-ANH", ["C1", "C9"], "UI-094");
