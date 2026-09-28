/* F06 verdicts reached by LOOKING, with the capture that was looked at, and
 * the issue each measured row of f06-nhom-nguoi.mjs now points at. Same shape
 * as f05-phan-xu.mjs: `phanXu` writes a judged row, `ganIssue` copies a
 * measured row unchanged except for its issue.
 *
 *   node kich-ban/f06-phan-xu.mjs
 */
import { soGhi } from "../thu-vien/ghi.mjs";

const out = process.env.AUDIT_OUT;
if (!out) throw new Error("thiếu AUDIT_OUT");
const so = soGhi(out);

const phanXu = (rec) =>
  so.ghi({ feature: "F06", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec, ghiChu: `phân xử bằng mắt: ${rec.ghiChu}` });

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
    if (!r) throw new Error(`không có hàng đo ${tc} ${c}; chạy f06-nhom-nguoi.mjs trước`);
    const { luc: _luc, ...giu } = r;
    so.ghi({ ...giu, issue });
  }
};

// ---------------------------------------------------------------- baselines
const base = (tc, screen, action, rows) => {
  for (const [cauHinh, status, issue, evidence, ghiChu] of rows) {
    phanXu({ tc, screen, state: "Minh Anh (Team Đà Lạt, 7 bạn)", action, cauHinh, expected: "không tràn, không cắt/che, đọc được ở sáng và tối; ở tablet bố cục có trần bề ngang như các form", status, issue, evidence: [evidence], ghiChu });
  }
};
base("TC-F06.S01-BASE", "F06.S01", "mở /groups/new", [
  ["C1,C2,C3", "PASS", null, "EV-F06.S01-BASE-ghep", "bìa sổ có ô tên, «Mở nhóm», ghi chú «Chỉ hai người?… Thêm bạn». Ô 44dp đo ở TC-F06-VUNG-BAM (UI-001)"],
  ["C4,C5,C6,C7", "PASS", null, "EV-F06.S01-rong-BASE-ghep", "tablet gom cột 560 ở giữa"],
]);
base("TC-F06.S02-BASE", "F06.S02", "mở /groups/[id]/members", [
  ["C1,C2,C3", "PASS", null, "EV-F06.S02-BASE-ghep", "hàng tên có mực riêng, vai trò, nút «Đặt làm quản trị»; ở C2 cuộn được tới nút mời. Nút trùng tên và hàng không mở hồ sơ đo ở TC-F06-THANH-VIEN-20 (UI-076)"],
  ["C4,C5,C6,C7", "FAIL", "UI-081", "EV-F06.S02-rong-BASE-ghep", "ở C6/C7 hàng trải hết bề ngang: nút «Đặt làm quản trị» dạt mép phải, xa tên người như ở Bạn bè (UI-081)"],
]);
base("TC-F06.S03-BASE", "F06.S03", "mở /groups/[id]/invite", [
  ["C1,C2,C3", "PASS", null, "EV-F06.S03-BASE-ghep", "phong bì hai ô và «Gửi lời mời» đọc trọn; ô 44dp ở UI-001"],
  ["C4,C5,C6,C7", "PASS", null, "EV-F06.S03-rong-BASE-ghep", "tablet gom cột ở giữa"],
]);
base("TC-F06.S04-BASE", "F06.S04", "mở /groups/empty", [["C1,C2,C3", "PASS", null, "EV-F06.S04-BASE-ghep", "chuyển về /messages như thiết kế (route không có lối vào UI)"]]);
base("TC-F06.S05-BASE", "F06.S05", "mở /friends", [
  ["C1,C2,C3", "PASS", null, "EV-F06.S05-BASE-ghep", "3 phân đoạn, 7 hàng bạn, «Nhắn tin», nút chân trang; ở C2 hàng cuối cuộn lên được trên nút chân trang (TC-F06-BAN-CUOI)"],
  ["C4,C5,C6,C7", "FAIL", "UI-081", "EV-F06.S05-rong-BASE-ghep", "ở C6/C7 nút «Nhắn tin» cách cuối tên 487 và 679px (TC-F06-BAN-TABLET)"],
]);
base("TC-F06.S06-BASE", "F06.S06", "mở /friends/add", [
  ["C1,C2,C3", "PASS", null, "EV-F06.S06-BASE-ghep", "danh thiếp có ô số (đã che) và «Tìm»; ô 44dp ở UI-001"],
  ["C4,C5,C6,C7", "PASS", null, "EV-F06.S06-rong-BASE-ghep", "tablet gom cột ở giữa"],
]);
base("TC-F06.S07-BASE", "F06.S07", "mở /people/[id] của một người bạn", [
  ["C1,C2,C3", "PASS", null, "EV-F06.S07-BASE-ghep", "hộ chiếu, «Nhắn tin», «Thêm hành động», tường rỗng có câu giải thích"],
  ["C4,C5,C6,C7", "PASS", null, "EV-F06.S07-rong-BASE-ghep", "tablet trải hết bề ngang nhưng chỉ có một cột nội dung, đọc được"],
]);
base("TC-F06.S08-BASE", "F06.S08", "mở /people/[id] của chính mình", [["C1,C2,C3", "PASS", null, "EV-F06.S08-BASE-ghep", "«Đăng bài mới», 3 chip «Ai được bình luận», tường rỗng; ở C2 chip xuống hai dòng, không cắt"]]);

// ------------------------------------------------------- judged from captures
so.rut("TC-F05.S01-LOI-MOI", "tiêu chí tự động chỉ đòi có nút «Đồng ý»; kỳ vọng còn gồm «ai mời». Phân xử bằng mắt ở f06-phan-xu.mjs");
phanXu({ tc: "TC-F05.S01-LOI-MOI", feature: "F06", screen: "F05.S01", state: "người được mời vừa đăng nhập bằng số đó", action: "mở Tin nhắn", cauHinh: "C1", expected: "lời mời nói nhóm nào và ai mời; có «Đồng ý» và cách từ chối", status: "FAIL", issue: "UI-080", evidence: ["EV-F06-DUOC-MOI-VAO-CUA-C1"], ghiChu: "hàng lời mời có tên nhóm, «2 thành viên · bạn được mời», «Chưa có tin nhắn nào.» và một nút «Đồng ý vào nhóm» 358×48. Không có tên người mời, không có nút từ chối (mã: chỉ một RudiButton, `groups/Conversations.tsx:216–224`)" });

// ------------------------------------------- measured rows, now with issues
ganIssue("TC-F06-TAO-SAU", ["C1"], "UI-071");
ganIssue("TC-F06-MOI-THANH-VIEN", ["C1"], "UI-072");
ganIssue("TC-F06-DUOC-MOI-VAO-CUA", ["C1"], "UI-073");
ganIssue("TC-F06-TU-BO-QUAN-TRI", ["C1"], "UI-074, UI-075");
ganIssue("TC-F06-THANH-VIEN-20", ["C1", "C6"], "UI-076");
ganIssue("TC-F06-DONG-Y-503", ["C1"], "UI-077");
ganIssue("TC-F06-CHAN-MO-LAI", ["C1"], "UI-078");
ganIssue("TC-F06-CHAN-CHAT", ["C1"], "UI-079");
ganIssue("TC-F06-BI-CHAN-CHAT", ["C1"], "UI-079");
ganIssue("TC-F06-BAN-TABLET", ["C6", "C7"], "UI-081");
ganIssue("TC-F06-VUNG-BAM", ["C1"], "UI-001");
ganIssue("TC-L06-VONGDOI", ["C1"], "UI-038");
console.log("đã ghi phân xử F06");
