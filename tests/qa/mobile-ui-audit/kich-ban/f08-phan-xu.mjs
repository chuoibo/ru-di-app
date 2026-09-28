/* F08 verdicts reached by LOOKING, with the capture that was looked at, and
 * the issue each measured row of f08-ky-niem.mjs now points at. Same shape as
 * f07-phan-xu.mjs: `phanXu` writes a judged row, `ganIssue` copies a measured
 * row unchanged except for its issue.
 *
 *   node kich-ban/f08-phan-xu.mjs
 */
import { soGhi } from "../thu-vien/ghi.mjs";

const out = process.env.AUDIT_OUT;
if (!out) throw new Error("thiếu AUDIT_OUT");
const so = soGhi(out);

const phanXu = (rec) =>
  so.ghi({ feature: "F08", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec, ghiChu: `phân xử bằng mắt: ${rec.ghiChu}` });

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
    if (!r) throw new Error(`không có hàng đo ${tc} ${c}; chạy f08-ky-niem.mjs trước`);
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
base("TC-F08.S01-BASE", "F08.S01", "mở /groups/[id]/wall", "nhóm chat-test: 3 ảnh, 1 check-in (chat-0)", [
  ["C1,C2,C3", "PASS", null, "EV-F08.S01-BASE-ghep", "«Tường nhóm · Chỉ thành viên nhóm thấy», «Thả khoảnh khắc», «Check-in», thẻ check-in, ảnh in nghiêng có băng dính; C2 không cắt, C3 tối đọc được. Avatar «0» là chữ đầu của chữ cuối tên giả «Chat Test 01». Giờ viết «10:07 28-09», khác «28/09» ở mọi chỗ khác: TC-F08-GIO-TUONG (UI-104)"],
  ["C4,C5,C6,C7", "FAIL", "UI-105", "EV-F08.S01-rong-BASE-ghep", "C4, C5 như điện thoại. C6/C7: ảnh in trải hết bề ngang, ảnh dọc cao 936 và 1192px, hơn một màn ở C6 (TC-F08-TABLET); thanh đầu, nút và thẻ check-in cũng không có trần"],
]);
base("TC-F08.S02-BASE", "F08.S02", "mở /groups/[id]/album", "kệ có một album «đang đi» (chat-0)", [
  ["C1,C2,C3", "PASS", null, "EV-F08.S02-BASE-ghep", "«Album chuyến đi · Mỗi kèo một album», hàng «Kèo album F08 · 2026 · đang đi · 20 người · 3 ảnh · 1 chỗ đã tới · 1 check-in» xuống dòng gọn ở C2, C3 tối đọc được. Chỉ ghi năm, không ghi ngày: TC-F08-ALBUM-NGAY (UI-103)"],
  ["C4,C5,C6,C7", "FAIL", "UI-105", "EV-F08.S02-rong-BASE-ghep", "C4, C5 như điện thoại. C6/C7: hàng kệ trải hết bề ngang, mũi tên nằm ở mép phải, xa chữ; phần còn lại của màn trống"],
]);
base("TC-F08.S03-BASE", "F08.S03", "mở /trips/[id]/album", "album «Kèo album F08»: 3 ảnh, 1 check-in (chat-0)", [
  ["C1,C2,C3", "PASS", null, "EV-F08.S03-BASE-ghep", "tên kèo, dòng thông tin, «3 khoảnh khắc», ảnh dẫn có chú thích, lưới 2 ảnh, «Chỗ đã tới», «Thước phim»; C2 xuống dòng gọn. Ảnh dẫn cắt ảnh dọc thành khung ngang: thiết kế ghi rõ, không tính là lỗi"],
  ["C4,C5,C6,C7", "FAIL", "UI-105", "EV-F08.S03-rong-BASE-ghep", "C4, C5 như điện thoại. C6/C7: ảnh dẫn 720×383 và 912×465 trải hết bề ngang, lưới giữ 2 ô 175 và 223px, nửa phải trống (TC-F08-TABLET)"],
]);
base("TC-F08.S04-BASE", "F08.S04", "mở /moments/new?ctx=[id]", "chưa chọn ảnh (chat-0)", [
  ["C1,C2,C3", "PASS", null, "EV-F08.S04-BASE-ghep", "khung trống có Nếp, ô câu, «300 ký tự còn lại», «Chọn ảnh»; nút ở chân trang cố định. Ở C2 chân trang che «300 ký tự còn lại» tới khi cuộn; cuộn thì dòng cuối ra khỏi chân trang (TC-F08-THA-CUON). Nút tắt không nói lý do: TC-F08.S04-NUT-TAT (UI-091)"],
  ["C4,C5", "PASS", null, "EV-F08.S04-rong-BASE-ghep", "C4 như C2 (chân trang che câu cuối tới khi cuộn); C5 thấy hết, cột trần 640px"],
]);
base("TC-F08.S05-BASE", "F08.S05", "mở /stories/new", "chưa chọn ảnh (chat-0)", [
  ["C1,C2,C3", "PASS", null, "EV-F08.S05-BASE-ghep", "câu «Một tấm ảnh, chỉ bạn bè thấy, trong 24 giờ.», khung trống, «Chú thích, nếu muốn», «24 giờ · Còn 200 ký tự», «Chọn ảnh». «Đăng story» và lý do nằm dưới nếp gấp ở C1–C3, cuộn tới được (TC-F08-VOI-TOI)"],
  ["C4,C5", "PASS", null, "EV-F08.S05-rong-BASE-ghep", "C5 thấy cả «Đăng story» và «Chọn một tấm ảnh trước đã.»; C4 như C1"],
]);
base("TC-F08.S06-BASE", "F08.S06", "mở /stories/[id] của Chat Test 01", "chat-1 xem story ngang của bạn", [
  ["C1,C2,C3", "PASS", null, "EV-F08.S06-BASE-ghep", "vạch tiến độ, tên, «3 giờ trước», nút đóng; ảnh ngang hiện trọn giữa màn ở cả ba, C3 nền tối hơn"],
]);
base("TC-F08.S07-BASE", "F08.S07", "mở /posts/new", "bài trống (chat-0)", [
  ["C1,C2,C3", "PASS", null, "EV-F08.S07-BASE-ghep", "ô «Bạn muốn kể gì?», «Chọn ảnh», bốn thẻ «Ai đọc được?» (2 cột ở C1/C3, 1 cột ở C2), «Vì sao bốn mức?», «Đăng» tắt có lý do ngay dưới. axe aria-required-attr ×4 là bốn radio thiếu aria-checked: TC-F08-DANG-BAI (UI-003)"],
  ["C4,C5", "PASS", null, "EV-F08.S07-rong-BASE-ghep", "C5 thấy «Đăng» và lý do; C4 như C1"],
]);
base("TC-F08.S08-BASE", "F08.S08", "mở /posts/[id]", "bài «Bạn bè» của Chat Test 01 có một bình luận dài; chat-1 xem", [
  ["C1,C2,C3", "PASS", null, "EV-F08.S08-BASE-ghep", "bài, ảnh in, 6 nút cảm xúc (xuống 2 hàng ở C2), bình luận dài xuống dòng, ô viết bình luận; không tràn. Nút cảm xúc 38×32 và thùng rác 18×20 là TC-F08-CAM-XUC-BAI (UI-001) và TC-F08-XOA-BINH-LUAN (UI-096)"],
  ["C4,C5,C6,C7", "FAIL", "UI-105", "EV-F08.S08-rong-BASE-ghep", "C4, C5 như điện thoại. C6/C7: thẻ bài, ảnh của bài và bình luận trải hết bề ngang; ở C7 ảnh cao hơn nửa màn"],
]);
base("TC-F08.S09-BASE", "F08.S09", "mở /achievements", "chat-0: cấp 1, huy hiệu «Mở hàng» vừa mở", [
  ["C1,C2,C3", "FAIL", "UI-106", "EV-F08.S09-BASE-ghep", "C1, C3 đọc được. C2: lần đầu thấy huy hiệu mới, cột chữ của thẻ «Mới mở» còn 36px, «Mở hà|ng», «Chia kho|ản chi đầu tiên» 6 dòng (TC-F08-TEM-HEP). axe aria-prohibited-attr ×2 (thanh tiến độ và con dấu mang aria-label mà không có role: UI-089); khối Nếp M8 112×112 không tên nhận Tab (UI-008)"],
]);

// ------------------------------------------------------- judged from captures
so.rut("TC-F08-BINH-LUAN-BAI", "tiêu chí tự động lấy dòng ngay sau nút làm lý do; dòng đó là nút «Báo cáo bài này», không phải lý do. Phân xử bằng mắt ở f08-phan-xu.mjs");
phanXu({ tc: "TC-F08-BINH-LUAN-BAI", screen: "F08.S08", state: "bài «Bạn bè» của Chat Test 01", action: "gõ và gửi bình luận 190 ký tự và 7 ký tự", cauHinh: "C1", expected: "nút gửi chưa dùng được thì ẩn hoặc có lý do (ADR-0038); bình luận xuống dòng, không tràn", status: "FAIL", issue: "UI-091", evidence: ["EV-F08.S08-BASE-ghep"], ghiChu: "«Gửi bình luận» lúc ô trống 48×48, tắt, viền đứt, không có dòng lý do; ngay dưới là nút «Báo cáo bài này». Phần còn lại đạt: hai bình luận gửi được (máy chủ 2), bình luận 190 ký tự xuống dòng, không tràn ngang" });

phanXu({ tc: "TC-F08-THA-XEM-TRUOC", screen: "F08.S04", state: "ảnh dọc 9:16 (360×640), chú thích 213 ký tự", action: "chọn ảnh, xem trước, chia sẻ, xem trên tường", cauHinh: "C1", expected: "khung xem trước cho thấy đúng phần ảnh sẽ hiện trên tường", status: "FAIL", issue: "UI-102", evidence: ["EV-F08-THA-9-16-ghep"], ghiChu: "cả hai khung kẹp về 3:4 (332×443 và 340×453, TC-F08-THA-DOC-9-16). Xem trước vẽ trọn ảnh (contain) với hai dải nền hai bên; trên tường ảnh phủ kín khung (cover), mất khoảng 25% chiều cao ở đầu và đuôi. Ảnh ngang và ảnh 3:4 không bị (tỉ lệ nằm trong khoảng kẹp)" });

phanXu({ tc: "TC-F08-ALBUM-NGAY", screen: "F08.S02", state: "kèo «Kèo album F08» từ 28/09 tới 29/09", action: "mở kệ album và album của kèo", cauHinh: "C1", expected: "album ghi ngày của chuyến (chú thích mã của AlbumLive nói ngày đứng đầu)", status: "FAIL", issue: "UI-103", evidence: ["EV-F08.S02-BASE-ghep", "EV-F08.S03-BASE-ghep"], ghiChu: "kệ và đầu album đều ghi «2026 · đang đi · 20 người · 3 ảnh…», không có ngày nào; bản demo cùng màn ghi «17 - 19/10/2026» (TC-F08-KHONG-PHIEN)" });

phanXu({ tc: "TC-F08-GIO-TUONG", screen: "F08.S01", state: "tường có 3 ảnh và 1 check-in đăng hôm nay", action: "đọc giờ dưới tên người đăng và trên mép ảnh in", cauHinh: "C1", expected: "ngày viết một kiểu trong app («28/09»)", status: "FAIL", issue: "UI-104", evidence: ["EV-F08.S01-BASE-ghep"], ghiChu: "tường ghi «10:07 28-09» và «Chat Test 01 · 10:04 28-09» (gạch nối); chat, bài, story và album ghi «vừa xong», «3 giờ trước», «28/09»" });

// ------------------------------------------- measured rows, now with issues
ganIssue("TC-F08.S04-NUT-TAT", ["C1"], "UI-091");
ganIssue("TC-L24-NUT-TAT", ["C1"], "UI-091");
ganIssue("TC-F08-TIM-503", ["C1"], "UI-095");
ganIssue("TC-F08-XOA-BINH-LUAN", ["C1"], "UI-096");
ganIssue("TC-L34-VONGDOI", ["C1"], "UI-097");
ganIssue("TC-MO24-DONG", ["C1"], "UI-098");
ganIssue("TC-L24-CHIP-DAI", ["C1"], "UI-099");
ganIssue("TC-F08-BAI-KHONG-DANH-CHO", ["C1"], "UI-100");
ganIssue("TC-F08-STORY-VUNG-CHAM", ["C1"], "UI-101, UI-001");
ganIssue("TC-L26-XOA", ["C1"], "UI-101, UI-001");
ganIssue("TC-L25-ANH", ["C1"], "UI-094");
ganIssue("TC-L25-CU-CHI", ["C1"], "UI-094");
ganIssue("TC-F08-TABLET", ["C6", "C7"], "UI-105");
ganIssue("TC-F08-TEM-HEP", ["C2"], "UI-106");
ganIssue("TC-F08-DANG-BAI", ["C1"], "UI-003");
ganIssue("TC-F08-CAM-XUC-BAI", ["C1"], "UI-001");
ganIssue("TC-F08-KHONG-PHIEN", ["C1"], "UI-082");
ganIssue("TC-F08-C8", ["C8"], "UI-040");
