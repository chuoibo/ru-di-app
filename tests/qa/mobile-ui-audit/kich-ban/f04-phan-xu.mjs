/* F04 verdicts reached by LOOKING, with the capture that was looked at, and
 * the issue each measured row of f04-tien.mjs now points at. Same shape as
 * f03-phan-xu.mjs: `phanXu` writes a judged row, `ganIssue` copies a measured
 * row unchanged except for its issue.
 *
 *   node kich-ban/f04-phan-xu.mjs
 */
import { soGhi } from "../thu-vien/ghi.mjs";

const out = process.env.AUDIT_OUT;
if (!out) throw new Error("thiếu AUDIT_OUT");
const so = soGhi(out);

const phanXu = (rec) =>
  so.ghi({ feature: "F04", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec, ghiChu: `phân xử bằng mắt: ${rec.ghiChu}` });

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
    if (!r) throw new Error(`không có hàng đo ${tc} ${c}; chạy f04-tien.mjs trước`);
    const { luc: _luc, ...giu } = r;
    so.ghi({ ...giu, issue });
  }
};

// ---------------------------------------------------------------- baselines
phanXu({ tc: "TC-F04.S01-BASE", screen: "F04.S01", state: "bước 1, bàn trống", action: "mở /smart-split/[id]/review", cauHinh: "C1,C2,C3", expected: "không tràn, không cắt/che, đọc được ở sáng và tối", status: "PASS", evidence: ["EV-F04.S01-BASE-ghep"], ghiChu: "tờ bill trống là nút «Chọn ảnh bill», «Nhập tay», «Cách chia»; ở C2 nhãn nút xuống hai dòng, không cắt; axe sạch ở C1/C3. Bước 2–5 đo ở các hàng riêng (TC-F04-MON-DAI, TC-F04-CHAN-*)" });
phanXu({ tc: "TC-F04.S01-BASE", screen: "F04.S01", state: "bước 1", action: "mở /smart-split/[id]/review", cauHinh: "C4,C5,C6,C7", expected: "như C1–C3", status: "PASS", evidence: ["EV-F04.S01-rong-BASE-ghep"], ghiChu: "ở tablet tờ bill trải hết bề ngang cột, vẫn đọc được" });
phanXu({ tc: "TC-F04.S02-BASE", screen: "F04.S02", state: "chưa đăng nhập (bản trải nghiệm)", action: "mở /smart-split/[id]/assignment", cauHinh: "C1,C2,C3", expected: "gán món trên bill mẫu, có nhãn nháp, đọc được", status: "PASS", evidence: ["EV-F04.S02-BASE-ghep"], ghiChu: "nhãn «Nháp»; dòng tên người của từng món cắt một dòng có chủ đích, số người là nút riêng; ô người 48dp" });
phanXu({ tc: "TC-F04.S03-BASE", screen: "F04.S03", state: "Team Đà Lạt, 7 khoản chuyển", action: "mở /settlements/[id]", cauHinh: "C1,C2,C3", expected: "không tràn, không cắt/che, đọc được", status: "FAIL", issue: "UI-061", evidence: ["EV-F04.S03-BASE-ghep"], ghiChu: "sơ đồ 8 người đọc được, số tiền đọc trọn; ở C2 dòng đầu sổ ép lời giải thích thành 9 dòng hẹp cạnh «Chưa có chuyến» (UI-061)" });
phanXu({ tc: "TC-F04.S03-BASE", screen: "F04.S03", state: "Team Đà Lạt", action: "mở /settlements/[id]", cauHinh: "C4,C5,C6,C7", expected: "như C1–C3", status: "PASS", evidence: ["EV-F04.S03-rong-BASE-ghep"], ghiChu: "ở C7 thấy trọn danh sách, đợt thu và «Tạo đợt thu từ sổ» trong một màn" });
phanXu({ tc: "TC-F04.S04-BASE", screen: "F04.S04", state: "đợt đã phát, 0/7 về", action: "mở /batches/[id]", cauHinh: "C1,C2,C3", expected: "không tràn, không cắt/che, đọc được", status: "PASS", evidence: ["EV-F04.S04-BASE-ghep"], ghiChu: "đếm, băng tiến độ, bảng ai chuyển cho ai, nút «Tiền đã về từ …» đọc trọn" });
phanXu({ tc: "TC-F04.S04-BASE", screen: "F04.S04", state: "đợt đã phát", action: "mở /batches/[id]", cauHinh: "C4,C5,C6,C7", expected: "như C1–C3", status: "PASS", evidence: ["EV-F04.S04-rong-BASE-ghep"], ghiChu: "ở C7 thấy cả mục «Gửi link riêng» (máy này không giữ link vì đợt phát ở máy khác)" });
phanXu({ tc: "TC-F04.S05-BASE", screen: "F04.S05", state: "1 khoản chi, sẽ nhận 1.120.000đ", action: "mở /finance", cauHinh: "C1,C2,C3", expected: "mỗi mục có nội dung của nó; số tiền đọc trọn", status: "FAIL", issue: "UI-060", evidence: ["EV-F04.S05-BASE-ghep"], ghiChu: "ba dòng sổ đọc trọn; tiêu đề «Chi theo nhóm» có nút «Xem quyết toán» nhưng không có hàng nào (UI-060)" });
phanXu({ tc: "TC-F04.S05-BASE", screen: "F04.S05", state: "như trên", action: "mở /finance", cauHinh: "C4,C5,C6,C7", expected: "như C1–C3", status: "FAIL", issue: "UI-060", evidence: ["EV-F04.S05-rong-BASE-ghep"], ghiChu: "như C1–C3" });

// ------------------------------------------------------- judged from captures
phanXu({ tc: "TC-F04-DA-GHI-TRA", screen: "F04.S01", state: "bước 5, trang sổ 20 dòng, người trả «Chat Test 01»", action: "sau «Ghi vào sổ»", cauHinh: "C1", expected: "dòng của người trả đọc trọn, gồm «(trả)»", status: "FAIL", issue: "UI-059", evidence: ["EV-F04-DA-GHI-C1"], ghiChu: "dòng đầu hiện «Chat Test 0…»: mất đúng chữ «(trả)», các dòng khác đọc trọn; tiêu đề sổ «Tất niên nhóm kiểm …» cũng cắt nhưng tên đầy đủ có ở tiêu đề dưới" });
phanXu({ tc: "TC-F04-VUNG-BAM", screen: "F04.S01", state: "bước 2 và 4", action: "đo vùng bấm", cauHinh: "C1", expected: "ô nhập ≥48dp (DESIGN.md)", status: "FAIL", issue: "UI-001", evidence: ["EV-F04-BUOC2-C1"], ghiChu: "9 ô nhập của 3 món cao 44 (tên 326×44, số phần 110×44, tiền 204×44); «Ô tên khoản chi» 358×44" });

// ------------------------------------------------------------------- motion
phanXu({ tc: "TC-MO13-M3", screen: "F04.S01", state: "bước 5, giảm chuyển động", action: "«Ghi vào sổ»", cauHinh: "C9", expected: "Nếp và dấu hiện ngay ở khung cuối, đứng yên", status: "PASS", evidence: ["EV-F04-M3-C9-dau-cuoi"], ghiChu: "ảnh vùng Nếp đầu (SVG, 391 ms) và cuối (Skia, 3358 ms) cùng một tư thế cầm dấu; lần đổi pixel duy nhất là SVG nhường Skia, góc phong bì đổi từ đỏ sẫm sang đỏ tươi. Dấu «Đã ghi sổ» đậm 1 từ mẫu đầu. Khác M5 ở F03 (có nhảy tư thế, UI-027)" });
phanXu({ tc: "TC-MO13-M4", screen: "F04.S04", state: "đợt đã phát, giảm chuyển động", action: "«Tiền đã về từ …»", cauHinh: "C9", expected: "Nếp cúi chào hiện ngay ở khung cuối và đứng yên", status: "PASS", evidence: ["EV-F04-M4-C9-dau-cuoi"], ghiChu: "ảnh đầu (SVG, 201 ms) và cuối (Skia, 3002 ms) cùng tư thế cúi chào; chỉ sắc đỏ góc phong bì đổi khi SVG nhường Skia" });
phanXu({ tc: "TC-MO14-C1", screen: "F04.S01", state: "bước 5 «Đã ghi sổ»", action: "sau «Ghi vào sổ»", cauHinh: "C1", expected: "dấu rơi xuống và đậm lên một lần, cùng nhịp Nếp M3", status: "PASS", evidence: ["EV-F04-DA-GHI-C1"], ghiChu: "độ đậm dấu theo mẫu 150 ms: 0,2 · 0,2 · 1 rồi giữ 1 (TC-F04-NEP-M3); Nếp M3 có mặt từ mẫu đầu; dấu nằm trên trang sổ, không đè số" });
phanXu({ tc: "TC-MO14-C9", screen: "F04.S01", state: "bước 5, giảm chuyển động", action: "sau «Ghi vào sổ»", cauHinh: "C9", expected: "dấu hiện ở trạng thái cuối, không rơi", status: "PASS", evidence: ["EV-F04-M3-C9"], ghiChu: "độ đậm 1 ở cả 14 mẫu, từ mẫu đầu (TC-MO13-M3 C9)" });
phanXu({ tc: "TC-MO17-C1", screen: "F04.S01", state: "bước 3, 8 ghế", action: "kéo thẻ món tới một ghế", cauHinh: "C1", expected: "thả vào ghế nào thì ghế đó đổi; trang đứng yên khi kéo", status: "PASS", evidence: [], ghiChu: "TC-F04-BAN-KEO: ghế Hà Vy true → false, cuộn 0 → 0 (kéo cảm ứng CDP, 500 ms, 20 bước)" });
so.ghi({ feature: "F04", nenTang: "web", method: "STATIC", layer: "-", issue: null, tc: "TC-MO22", screen: "F04.S01", state: "mọi màn tiền", action: "số tiền đổi giá trị", cauHinh: "C1,C9", expected: "đếm 200 ms, cắt thẳng khi giảm chuyển động", status: "NOT_APPLICABLE", evidence: [], ghiChu: "grep: không chỗ nào truyền countUp cho Money (chỉ có định nghĩa trong ui/Money.tsx), nên không màn nào đếm số" });

// ------------------------------------------- computed from the app's own layout
so.ghi({ feature: "F04", nenTang: "web", method: "STATIC", layer: "-", issue: "UI-050", tc: "TC-F04-BAN-HINH", screen: "F04.S01", state: "bàn gán món 2–20 người, khung rộng 288–700", action: "tính viTriGhe (dist-test) với chính các vị từ của tests/hinh-tien.test.mjs", cauHinh: "C1,C2,C5,C6", expected: "không ghế nào đè nhau, tên không đè hình nhân hay tên khác, trong khoảng nhóm 4–10 người của PRODUCT.md", status: "FAIL", evidence: [], ghiChu: "n ≤ 8: 0 chỗ đè ở mọi bề rộng (đúng phạm vi test repo, chỉ kiểm n ≤ 8). n = 9: tên đè hình nhân 2 chỗ ở 288–358; n = 10: 4 chỗ. n ≥ 12: tên đè tên. n ≥ 16: hình nhân đè nhau (4 cặp ở n = 16, 12–20 cặp ở n = 20). Từ 398 trở lên chỉ bắt đầu ở n = 11" });
so.ghi({ feature: "F04", nenTang: "web", method: "STATIC", layer: "-", issue: "UI-054", tc: "TC-F04-SO-DO-HINH", screen: "F04.S03", state: "sơ đồ quyết toán, n − 1 người trả cho 1 người", action: "tính soDoChuyen (dist-test), dải tên 72 như test repo", cauHinh: "C1,C2,C5", expected: "tên không đè nhau, không tràn khung", status: "FAIL", evidence: [], ghiChu: "tới 9 người: 0; 10 người: 4 cặp đè + 2 tên ra ngoài khung ở mọi bề rộng 288–358; 20 người: 25–32 cặp. Ở 398 bắt đầu từ 12 người" });

// ------------------------------------------- measured rows, now with issues
ganIssue("TC-F04-MON-DAI", ["C1", "C2", "C3", "C4", "C5"], "UI-048");
ganIssue("TC-L32-VONGDOI", ["C1"], "UI-049");
ganIssue("TC-F04-BAN-20", ["C1", "C2", "C6"], "UI-050");
ganIssue("TC-F04-CHAN-TEN", ["C1"], "UI-051");
ganIssue("TC-F04-CHAN-NGUOI", ["C1"], "UI-051");
ganIssue("TC-F04-BILL-503", ["C1"], "UI-051");
ganIssue("TC-F04-LUI-VE-BUOC1", ["C1"], "UI-052");
ganIssue("TC-F04-BACK-TRINH-DUYET", ["C1"], "UI-052");
ganIssue("TC-F04-TAI-LAI", ["C1"], "UI-052");
ganIssue("TC-F04-BAN-PHIM", ["C1"], "UI-053");
ganIssue("TC-F04-QT-20", ["C1", "C2"], "UI-054");
ganIssue("TC-F04-NEP-M2-KHUNG", ["C1", "C2", "C3"], "UI-055");
ganIssue("TC-F04-ANH-DOC", ["C1"], "UI-056");
ganIssue("TC-F04-NEP-MEP", ["C1"], "UI-057");
ganIssue("TC-F04-DOT-RONG", ["C1"], "UI-058");
console.log("đã ghi phân xử F04");
