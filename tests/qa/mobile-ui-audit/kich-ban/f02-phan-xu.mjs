/* F02 verdicts reached by LOOKING, written to the ledger with the capture that
 * was looked at, so a verdict that is not a measurement still has a trail.
 *
 *   node kich-ban/f02-phan-xu.mjs
 *
 * Two kinds of rows:
 *  - `phanXu`: a baseline or motion case judged on its screenshot or contact
 *    sheet. The note names what was seen; the evidence names the image.
 *  - `ganIssue`: a measured row already in the ledger (f02-kham-pha.mjs), copied
 *    unchanged except for the issue it now points at. Its numbers stay exactly
 *    those of the run that produced them.
 * Rerunning this file appends the same rows again; the matrix keeps the last.
 */
import { soGhi } from "../thu-vien/ghi.mjs";

const out = process.env.AUDIT_OUT;
if (!out) throw new Error("thiếu AUDIT_OUT");
const so = soGhi(out);

const phanXu = (rec) =>
  so.ghi({ feature: "F02", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec, ghiChu: `phân xử bằng mắt: ${rec.ghiChu}` });

const cuoiCung = new Map();
for (const r of so.doc()) if (!r.rut) cuoiCung.set(`${r.tc}|${r.nenTang ?? "web"}|${r.cauHinh ?? ""}`, r);
const ganIssue = (tc, cauHinh, issue) => {
  for (const c of cauHinh) {
    const r = cuoiCung.get(`${tc}|web|${c}`);
    if (!r) throw new Error(`không có hàng đo ${tc} ${c}; chạy f02-kham-pha.mjs trước`);
    const { luc: _luc, ...giu } = r;
    so.ghi({ ...giu, issue });
  }
};

// ---------------------------------------------------------------- baselines
phanXu({ tc: "TC-F02.S01-BASE", screen: "F02.S01", state: "danh sách Đà Lạt (10 nơi)", action: "mở tab Khám phá", cauHinh: "C1,C2,C3", expected: "không tràn, không cắt/che chữ, đọc được ở sáng và tối", status: "FAIL", issue: "UI-021", evidence: ["EV-F02.S01-BASE-ghep", "EV-F02-CAT-META-ghep"], ghiChu: "không tràn ngang; dòng giá của mọi hàng bị cắt (số đo ở TC-F02-META); placeholder ô tìm mất chữ «Đi AI» (UI-024); ở C1, 5 tín hiệu che chữ đều là hàng đang cuộn dưới thanh tab và nút «+», không phải lỗi; axe chỉ còn aria-required-parent của thanh tab (UI-003)" });
phanXu({ tc: "TC-F02.S01-BASE", screen: "F02.S01", state: "danh sách Đà Lạt (10 nơi)", action: "mở tab Khám phá", cauHinh: "C4,C5,C6", expected: "như C1–C3", status: "FAIL", issue: "UI-021", evidence: ["EV-F02.S01-BASE-ghep", "EV-F02-CAT-META-ghep"], ghiChu: "bố cục giữ được; dòng giá vẫn bị cắt ở C4, C5 và C6 (TC-F02-META)" });
phanXu({ tc: "TC-F02.S01-BASE", screen: "F02.S01", state: "danh sách Đà Lạt (10 nơi)", action: "mở tab Khám phá", cauHinh: "C7", expected: "như C1–C3", status: "PASS", evidence: ["EV-F02.S01-BASE-ghep"], ghiChu: "C7 1024 tối: 0/10 dòng giá bị cắt, không tràn, không che" });

phanXu({ tc: "TC-F02.S02-BASE", screen: "F02.S02", state: "15 điểm đến", action: "mở «Đổi điểm đến»", cauHinh: "C1,C2,C3", expected: "lưới bưu thiếp không tràn, tên đọc được", status: "PASS", evidence: ["EV-F02.S02-BASE-ghep"], ghiChu: "hai cột ở điện thoại; tên và tỉnh đọc trọn; mô tả kẹp có dấu «…» ở C2 là chủ ý; tối đọc được; mọi tín hiệu tự động bằng 0" });
phanXu({ tc: "TC-F02.S02-BASE", screen: "F02.S02", state: "15 điểm đến", action: "mở «Đổi điểm đến»", cauHinh: "C6", expected: "medium: 2 cột (DESIGN.md, bảng size class)", status: "PASS", evidence: ["EV-F02.S02-rong-BASE-ghep"], ghiChu: "768: 2 cột, đúng bảng size class" });
phanXu({ tc: "TC-F02.S02-BASE", screen: "F02.S02", state: "15 điểm đến", action: "mở «Đổi điểm đến»", cauHinh: "C7", expected: "expanded: 3 cột (DESIGN.md: columns 3; gridFor maxColumns 3 cho hàng thẻ)", status: "FAIL", issue: "UI-031", evidence: ["EV-F02.S02-rong-BASE-ghep"], ghiChu: "1024: vẫn 2 cột, mỗi thẻ khoảng 450px, một màn thấy 8 thành phố; DiemDenScreen chia cứng (rongLuoi - KHE) / 2" });

phanXu({ tc: "TC-F02.S03-BASE", screen: "F02.S03", state: "chi tiết Tiệm Nướng Xóm Lào", action: "mở chi tiết", cauHinh: "C1,C2,C3", expected: "không tràn, nhãn và chữ đọc trọn", status: "FAIL", issue: "UI-023", evidence: ["EV-F02.S03-BASE-ghep", "EV-F02-CAT-LUU-ghep"], ghiChu: "thân trang ổn (địa chỉ xuống dòng, giá một dòng); nhãn nút chân trang «Lưu địa điểm» bị cắt ở cả ba cấu hình (TC-F02-NHAN-LUU); tín hiệu che chữ là nội dung cuộn dưới chân trang cố định" });
phanXu({ tc: "TC-F02.S03-BASE", screen: "F02.S03", state: "chi tiết Tiệm Nướng Xóm Lào", action: "mở chi tiết", cauHinh: "C4", expected: "như C1–C3", status: "FAIL", issue: "UI-023", evidence: ["EV-F02-CAT-LUU-ghep"], ghiChu: "nhãn «Lưu địa điểm» còn bị cắt ở 375" });
phanXu({ tc: "TC-F02.S03-BASE", screen: "F02.S03", state: "chi tiết Tiệm Nướng Xóm Lào", action: "mở chi tiết", cauHinh: "C5,C6,C7", expected: "như C1–C3", status: "PASS", evidence: ["EV-F02-CAT-LUU-ghep"], ghiChu: "430, 768, 1024: nhãn đọc trọn, không tràn; ở 768/1024 là một cột trải tới maxContent theo DESIGN.md" });
phanXu({ tc: "TC-F02.S03-TEN-DAI", screen: "F02.S03", state: "tên 76 ký tự, giá 1.250.000đ – 12.500.000đ, không có điểm", action: "mở chi tiết", cauHinh: "C1,C2,C3", expected: "tên dài xuống dòng, không cắt, không tràn; giá 8 chữ số đọc trọn", status: "PASS", evidence: ["EV-F02.S03-dai-BASE-ghep"], ghiChu: "tên xuống 4 dòng; giá một dòng; thiếu điểm thì không vẽ sao; địa chỉ dài xuống dòng cạnh nút Chỉ đường (nhãn Lưu vẫn bị cắt, thuộc UI-023)" });
phanXu({ tc: "TC-F02.S03-TEN-LIEN", screen: "F02.S03", state: "tên 57 ký tự không có dấu cách", action: "mở chi tiết", cauHinh: "C1,C2,C3", expected: "chuỗi liền được bẻ, không tràn ngang", status: "PASS", evidence: ["EV-F02.S03-lien-BASE-ghep"], ghiChu: "bẻ giữa từ thành 3–4 dòng, không tràn ngang ở 320" });
phanXu({ tc: "TC-F02.S04-BASE", screen: "F02.S04", state: "đã đăng nhập", action: "mở /ai-match", cauHinh: "C1", expected: "chuyển về Khám phá (route cũ), không trang trắng", status: "PASS", evidence: ["EV-F02.S04-BASE-C1"], ghiChu: "đường dẫn cuối /explore, trang Khám phá đầy đủ" });

// ------------------------------------------------------------------ motion
phanXu({ tc: "TC-MO12-BAT", screen: "F02.S01", state: "mở tab Khám phá lần đầu", action: "chạm tab", cauHinh: "C1", expected: "các lớp dựng lên rồi dừng ở tư thế đứng", status: "PASS", evidence: ["EV-F02-MO12-bat-C1"], ghiChu: "sân khấu hiện và đứng yên ở tư thế cuối; không kẹt, không nháy. Không kết luận thời lượng: khung SwiftShader không đại diện" });
phanXu({ tc: "TC-MO12-BAT", screen: "F02.S01", state: "mở tab Khám phá lần đầu", action: "chạm tab", cauHinh: "C9", expected: "giảm chuyển động: hiện đứng sẵn, không dựng từng lớp", status: "PASS", evidence: ["EV-F02-MO12-bat-C9"], ghiChu: "sân khấu hiện trọn trong một khung, không có khung trung gian" });
phanXu({ tc: "TC-MO12-BO-LOC", screen: "F02.S01", state: "đang lọc «Cafe»", action: "bỏ lọc", cauHinh: "C1", expected: "sân khấu quay lại mà không dựng lại (ExploreLive: «dựng một lần mỗi thành phố»)", status: "FAIL", issue: "UI-026", evidence: ["EV-F02-MO12-bo-loc-C1"], ghiChu: "sân khấu mount lại và dựng từ phẳng tới đứng, khoảng 1 giây trong môi trường này, mỗi lần bỏ lọc" });
phanXu({ tc: "TC-MO12-BO-LOC", screen: "F02.S01", state: "đang lọc «Cafe»", action: "bỏ lọc", cauHinh: "C9", expected: "sân khấu hiện đứng sẵn, không trống", status: "FAIL", issue: "UI-027", evidence: ["EV-F02-MO12-bo-loc-C9"], ghiChu: "khung 117ms có tranh (SVG), khung 176ms vùng sân khấu trống, khung 609ms có lại (Skia); dòng thời gian data-renderer: SVG gỡ ở 145ms khi Skia op 1" });

// ------------------------------------------------------- non-ideal states
phanXu({ tc: "TC-F02-DIEM-DEN-RONG", screen: "F02.S01", state: "thành phố chưa có quán (Hội An), không lọc, không tìm", action: "đổi điểm đến", cauHinh: "C1", expected: "câu nói thành phố chưa có địa điểm, lối ra là đổi điểm đến", status: "FAIL", issue: "UI-028", evidence: ["EV-F02-HOI-AN-C1"], ghiChu: "hiện «Chưa thấy nơi phù hợp / Thử từ khóa khác, hoặc bỏ bớt bộ lọc để thấy lại cả danh mục» và nút «Xóa lọc» dù không có lọc hay từ khoá nào" });
phanXu({ tc: "TC-F02-LOI-503-CAU", screen: "F02.S01", layer: "L35", state: "danh mục trả 503", action: "mở Khám phá", cauHinh: "C1", expected: "câu nói đúng nguyên nhân (máy chủ gặp sự cố), không nói về dữ liệu đã nhập", status: "FAIL", issue: "UI-029", evidence: ["EV-F02-LOI-503-C1"], ghiChu: "hiện câu mặc định của ErrorState «Kiểm tra mạng rồi thử lại. Những gì bạn đã nhập vẫn còn nguyên.» cho lỗi 503; câu riêng của máy chủ lỗi đã tính sẵn (trang.loi) nhưng không được truyền" });
phanXu({ tc: "TC-F02-OFFLINE-GIU", screen: "F02.S01", layer: "L35", state: "đã tải danh sách, rồi mất mạng", action: "sang tab khác rồi quay lại", cauHinh: "C1", expected: "danh sách đã tải còn nguyên, kèm một câu báo đang offline", status: "FAIL", issue: "UI-030", evidence: ["EV-F02-OFFLINE-C1"], ghiChu: "danh sách 10 nơi đã tải bị thay bằng «Chưa đọc được danh mục» khi tab được focus lại; thành phố và sân khấu vẫn hiện" });

phanXu({ tc: "TC-L33-VONGDOI", screen: "F02.S03", layer: "L33", state: "đóng → mở → dùng → đóng → mở lại", action: "vòng đời Link chỉ đường", cauHinh: "C1", expected: "mở bản đồ ngoài ở tab/cửa sổ mới; quay lại app thì trang còn nguyên", status: "FAIL", issue: "UI-022", evidence: ["EV-F02-CHI-TIET-CHI-DUONG-C1"], ghiChu: "trên web không có gì được mở (window.open geo: trả null, không có trang mới) nên không có lớp nào để đi hết vòng đời; số đo ở TC-F02-CHI-TIET-CHI-DUONG" });

// ------------------------------------------- measured rows, now with issues
ganIssue("TC-F02-META", ["C1", "C2", "C3", "C4", "C5", "C6"], "UI-021");
ganIssue("TC-F02-CHI-TIET-CHI-DUONG", ["C1"], "UI-022");
ganIssue("TC-F02-NHAN-LUU", ["C1", "C2", "C3", "C4"], "UI-023");
ganIssue("TC-F02-AI-MAU", ["C1"], "UI-024");
ganIssue("TC-F02-AI-HOI", ["C1"], "UI-024");
ganIssue("TC-F02-NHAY", ["C1", "C9"], "UI-025");
console.log("đã ghi phân xử F02");
