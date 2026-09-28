/* F07 verdicts reached by LOOKING, with the capture that was looked at, and
 * the issue each measured row of f07-so-doi.mjs now points at. Same shape as
 * f06-phan-xu.mjs: `phanXu` writes a judged row, `ganIssue` copies a measured
 * row unchanged except for its issue.
 *
 *   node kich-ban/f07-phan-xu.mjs
 */
import { soGhi } from "../thu-vien/ghi.mjs";

const out = process.env.AUDIT_OUT;
if (!out) throw new Error("thiếu AUDIT_OUT");
const so = soGhi(out);

const phanXu = (rec) =>
  so.ghi({ feature: "F07", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec, ghiChu: `phân xử bằng mắt: ${rec.ghiChu}` });

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
    if (!r) throw new Error(`không có hàng đo ${tc} ${c}; chạy f07-so-doi.mjs trước`);
    const { luc: _luc, ...giu } = r;
    so.ghi({ ...giu, issue });
  }
};

// ---------------------------------------------------------------- baselines
const base = (tc, screen, action, state, rows) => {
  for (const [cauHinh, status, issue, evidence, ghiChu] of rows) {
    phanXu({ tc, screen, state, action, cauHinh, expected: "không tràn, không cắt/che, đọc được ở sáng và tối; ở tablet bố cục có trần bề ngang", status, issue, evidence: [evidence], ghiChu });
  }
};
base("TC-F07.S01-BASE", "F07.S01", "mở /hai-nguoi/chon-nguoi", "chat-0, một người bạn", [
  ["C1,C2,C3", "PASS", null, "EV-F07.S01-BASE-ghep", "«Rủ ai?», thẻ «Chat Test 02 · Mở tờ giấy của hai bạn», «Thêm bạn khác»; C2 không cắt, C3 tối đọc được. Avatar «0» là chữ đầu của chữ cuối tên giả «Chat Test 02» (mã lấy chữ cuối làm tên gọi), không phải lỗi"],
]);
base("TC-F07.S02-BASE", "F07.S02", "mở /groups/[id]/to-giay", "chưa có sổ hai người", [
  ["C1,C2,C3", "FAIL", "UI-090", "EV-F07.S02-BASE-ghep", "bìa sổ, Nếp, «Chưa có sổ hai người», «Đề nghị lập sổ» đọc được ở cả ba; hai nhãn tên trên bìa đều thành «Chat Tes…» (TC-F07-BIA-TEN)"],
  ["C4,C5,C6,C7", "FAIL", "UI-090", "EV-F07.S02-rong-BASE-ghep", "ở C6/C7 trạng thái rỗng gom cột ở giữa như thanh đầu; tên trên bìa vẫn «Chat Tes…» ở mọi cỡ. Tờ đã có nội dung thì trải hết bề ngang: TC-F07-TABLET (UI-093)"],
]);

// ------------------------------------------------------- judged from captures
so.rut("TC-F02-CHI-TIET-RU", "tiêu chí tự động chỉ đòi có câu trên màn; ảnh cho thấy ngay dưới câu «Tuần này hai bạn đã có tờ rồi…» là một bản phác MỚI của chính tuần đó, không có chỗ vừa chọn. Phân xử bằng mắt ở f07-phan-xu.mjs");
phanXu({ tc: "TC-F02-CHI-TIET-RU", feature: "F02", screen: "F02.S03", state: "người xem có sổ hai người; tuần này có một tờ đã chốt, không có tờ mở", action: "chạm «Rủ Chat Test 02 tới đây» ở Tiệm Nướng Xóm Lào", cauHinh: "C1", expected: "mở tờ giấy; chỗ vừa chọn nằm trên một bản phác, hoặc màn nói rõ vì sao không và không tự tạo tờ nào", status: "FAIL", issue: "UI-085", evidence: ["EV-F07-RU-TOI-DAY-C1", "EV-F07-RU-TOI-DAY-SAU-C1"], ghiChu: "nút 245×52 mở /to-giay?ru=1&cho=…; đầu trang: «Tuần này hai bạn đã có tờ rồi. Chỗ bạn chọn chưa được thêm, để dành cho tuần sau nhé.»; ngay dưới: bản phác mới «18:30 Ăn tối · Thứ Bảy 03/10», không có Tiệm Nướng Xóm Lào; «Tờ đã khép: Đã chốt · Thứ Bảy 03/10». SQL chỉ đọc: tờ của cặp 1 → 2, cùng tuần bắt đầu 28/09; tờ chốt có từ lượt «ru», tờ nháp sinh đúng lúc chạm nút" });

so.rut("TC-F07-SO-MO-BEN-KIA", "tiêu chí tự động chỉ đợi thân sổ đã mở; ảnh cho thấy sheet «Lập sổ hai người» của người đề nghị vẫn mở và quay về trạng thái mời «Đề nghị lập sổ». Phân xử bằng mắt ở f07-phan-xu.mjs");
phanXu({ tc: "TC-F07-SO-MO-BEN-KIA", screen: "F07.S02", layer: "L23", state: "mình vừa đề nghị, sheet chờ còn mở; người kia đồng ý trên máy họ", action: "đợi, không tải lại", cauHinh: "C1", expected: "màn tự đổi sang sổ đã mở; sheet chờ đóng hoặc nói sổ đã mở", status: "FAIL", issue: "UI-084", evidence: ["EV-F07-SO-MO-A-C1"], ghiChu: "thân sổ đổi sang «đã mở» sau 5 ms kể từ khi bắt đầu đợi (lượt đọc 4 s đã tới trước đó), nhưng sheet vẫn mở, đổi về «Bạn ký khi bấm đề nghị · Chờ Chat Test 02 ký» với con dấu «Đề nghị lập sổ» và «Để sau», che bìa sổ vừa mở (M6)" });

phanXu({ tc: "TC-F07-DONG-Y-LAP-SO", screen: "F07.S02", layer: "L23", state: "người kia đã đề nghị lập sổ", action: "«Xem lời đề nghị» → «Đồng ý»", cauHinh: "C1", expected: "sheet đóng khi máy chủ trả lời; sổ mở, bìa mở một lần (M6); màn mời «Rủ đi chơi»", status: "PASS", evidence: ["EV-F07-XEM-DE-NGHI-C1", "EV-F07-SO-MO-B-C1"], ghiChu: "sheet «Chat Test 01 đã đề nghị. Bạn đồng ý thì sổ mở.» + «Đồng ý»; sau khi chạm: sổ mở, dấu «Sổ đã mở», «Chưa có tờ nào tuần này», «Rủ đi chơi». Số đo «4 ảnh khác nhau, đổi lần cuối 7300 ms» của lượt đầu là ảnh chụp cách 150 ms, bắt đầu sau khi bìa đã mở xong: nó đếm Nếp diễn và SVG nhường Skia, không đo bìa. Bìa đo theo từng khung ở TC-MO16-C1; chỗ sheet nháy trạng thái mời ở TC-F07-DONG-Y-NHAY-C1 (UI-084)" });

phanXu({ tc: "TC-MO16-C9", screen: "F07.S02", state: "giảm chuyển động; người kia đã đề nghị lập sổ (cặp chat-12/chat-13)", action: "«Xem lời đề nghị» → «Đồng ý»", cauHinh: "C9", expected: "bìa hiện ngay ở góc cuối (−108°), không có góc trung gian; dấu hiện ngay", status: "PASS", evidence: ["EV-F07-M6-KHUNG-C9"], ghiChu: "mẫu rAF đầu tiên có bìa đã ở −108°, 0 góc trung gian (hai lượt C9: cặp chat-10/11 và chat-12/13); khung compositor từ 603 ms đứng yên ở tư thế cuối, Nếp đứng yên. Dấu «Sổ đã mở» đổi từ 0,2/scale 1,4 sang 1/scale 1 trong 73 ms ở lượt đầu; ở lượt sau có một khoảng 406 ms không có khung rAF nào (luồng chính bận khi M6 dựng), nên không đọc được thời lượng; mã `useNhipDau.ts:36–38` đặt trễ 0 khi giảm chuyển động" });

phanXu({ tc: "TC-MO13-M6-C1", screen: "F07.S02", state: "người kia đã đề nghị lập sổ (cặp chat-8/chat-9)", action: "«Đồng ý»", cauHinh: "C1", expected: "Nếp diễn M6 một lần (kéo dải, nhảy) cạnh bìa, rồi đứng ở khung tĩnh; không đè chữ", status: "PASS", evidence: ["EV-F07-M6-KHUNG-C1"], ghiChu: "81 khung compositor: bìa lật (630–819 ms), dấu «Sổ đã mở» đậm dần, Nếp cầm dải kéo (1098–1253 ms) rồi vẫy, nhảy và đứng yên từ khoảng 2 s; Nếp nằm trong ô riêng bên phải bìa, không đè tiêu đề hay nút «Rủ đi chơi»" });
phanXu({ tc: "TC-MO13-M6-C9", screen: "F07.S02", state: "giảm chuyển động (cặp chat-12/chat-13)", action: "«Đồng ý»", cauHinh: "C9", expected: "Nếp hiện ngay ở khung tĩnh, không diễn", status: "PASS", evidence: ["EV-F07-M6-KHUNG-C9"], ghiChu: "Nếp có mặt từ khung 603 ms ở tư thế vẫy (khung tĩnh) và giữ nguyên tới 1303 ms; không thấy tư thế cầm dải như ở C1" });

phanXu({ tc: "TC-F07-RANG-BUOC-NUT-TAT", screen: "F07.S02", layer: "L23", state: "sheet «Hai ô ràng buộc», chưa gõ gì", action: "mở từ «Cài đặt sổ»", cauHinh: "C1", expected: "ADR-0038 §2.2: nút chưa có nghĩa thì không hiện; còn hiện thì nói lý do ngay dưới nó", status: "FAIL", issue: "UI-091", evidence: ["EV-F07-RANG-BUOC-C1"], ghiChu: "«Lưu hai ô của tôi» viền đứt (tắt), dưới nó là «Đóng», không có dòng lý do nào. Mã: `RangBuoc.tsx:40` không truyền `lyDo`; kit chỉ in lý do khi có `lyDo` (`ui.tsx:544`)" });

phanXu({ tc: "TC-F07-DAI-NGAY", screen: "F07.S02", layer: "L23", state: "sheet sửa tờ, ngày của tờ là Thứ Bảy 03/10", action: "mở «Sửa trước khi gửi» / «Đề nghị sửa»", cauHinh: "C1", expected: "ngày đang chọn nằm trọn trong dải ngày khi sheet mở", status: "FAIL", issue: "UI-092", evidence: ["EV-F07-RU-TOI-DAY-NHAP-C1", "EV-F07-SUA-NHAP-C1"], ghiChu: "dải 14 lá (28/09 → 11/10), cuộn ngang; lá đang chọn «T7 03 Th 10» là lá thứ sáu, chỉ thấy 36/56px ở mép phải (TC-F02-CHI-TIET-RU-NHAP); ngày vẫn đọc được bằng chữ «Thứ Bảy 03/10» dưới dải. Ba sheet sửa tờ đã chụp đều cắt ở cùng chỗ" });

// ------------------------------------------- measured rows, now with issues
ganIssue("TC-F07-BIA-TEN", ["C1"], "UI-090");
ganIssue("TC-F07-SUA-NHAP", ["C1"], "UI-040, UI-001");
ganIssue("TC-L23-CAI-DAT-DAY", ["C1"], "UI-087");
ganIssue("TC-L23-SHEET-CON", ["C1"], "UI-089, UI-003");
ganIssue("TC-F07-CHO-SAU-KHI-DONG", ["C1"], "UI-086");
ganIssue("TC-L23-CHAM-DUP", ["C1"], "UI-088");
ganIssue("TC-F07-DONG-Y-NHAY-C1", ["C1"], "UI-084");
ganIssue("TC-F07-DONG-Y-NHAY-C9", ["C9"], "UI-084, UI-013");
ganIssue("TC-F07-C8", ["C8"], "UI-040");
ganIssue("TC-F07.S02-LANH-KHONG-PHIEN", ["C1"], "UI-082");
ganIssue("TC-F07-KHONG-PHIEN-GUI", ["C1"], "UI-082");
ganIssue("TC-F07.S02-503", ["C1"], "UI-083");
ganIssue("TC-F07-TABLET", ["C6", "C7"], "UI-093");
ganIssue("TC-L23-TABLET", ["C6", "C7"], "UI-093");
console.log("đã ghi phân xử F07");
