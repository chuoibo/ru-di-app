/* E1–E6 verdicts reached by LOOKING, with the capture that was looked at, and
 * the issue each measured row of e-luong.mjs now points at. Same shape as
 * f11-phan-xu.mjs.
 *
 *   node kich-ban/e-phan-xu.mjs
 */
import { soGhi } from "../thu-vien/ghi.mjs";

const out = process.env.AUDIT_OUT;
if (!out) throw new Error("thiếu AUDIT_OUT");
const so = soGhi(out);

const phanXu = (rec) =>
  so.ghi({ nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec, ghiChu: `phân xử bằng mắt: ${rec.ghiChu}` });

const cuoiCung = new Map();
for (const r of so.doc()) {
  if (r.rut) {
    for (const k of [...cuoiCung.keys()]) if (k.startsWith(`${r.tc}|`)) cuoiCung.delete(k);
    continue;
  }
  cuoiCung.set(`${r.tc}|${r.nenTang ?? "web"}|${r.cauHinh ?? ""}`, r);
}
const hangDo = (tc, c) => {
  const r = cuoiCung.get(`${tc}|web|${c}`);
  if (!r) throw new Error(`không có hàng đo ${tc} ${c}; chạy e-luong.mjs trước`);
  const { luc: _luc, ...giu } = r;
  return giu;
};
const gan = (tc, issue, evidence = null) => {
  const r = hangDo(tc, "C1");
  so.ghi({ ...r, issue, evidence: evidence ?? r.evidence });
};
// A measured row whose automatic verdict the capture overturns.
const lat = (tc, { status, issue, evidence, lyDo }) => {
  const r = hangDo(tc, "C1");
  // Run twice, the second run would wrap its own verdict: keep the first.
  if (r.ghiChu?.startsWith("phân xử bằng mắt:")) return;
  so.ghi({ ...r, status, issue, evidence: evidence ?? r.evidence, ghiChu: `phân xử bằng mắt: ${lyDo} (số đo tự động: ${r.ghiChu})` });
};

// ------------------------------------------- rows withdrawn, judged from the capture
phanXu({ feature: "E3", tc: "TC-E3-B-TAI-CHINH", screen: "E3", state: "đợt đã phát, B nợ A 75.000đ", action: "B: Cá nhân → «Tài chính của tôi»", cauHinh: "C1", expected: "trang tài chính của B nói B còn phải trả 75.000đ", status: "PASS", evidence: ["EV-E3-B1-C1"], ghiChu: "ảnh: «Phần chi của bạn 75.000đ»; «Còn phải trả 75.000đ · Đã trả 0đ»; «Sẽ nhận 0đ»; câu «Bạn còn nợ 75.000đ. Số này đọc từ sổ cái, không phải số dư ngân hàng.»; «Tiền đã về: Chưa có khoản chuyển nào được xác nhận là đã về.»" });
phanXu({ feature: "E4", tc: "TC-E4-DE-NGHI", screen: "E4", state: "A và B vừa thành bạn, chưa có sổ hai người", action: "«Tạo mới» → «Rủ một người đi chơi» → B → «Đề nghị lập sổ» → sheet → «Đề nghị lập sổ»", cauHinh: "C1", expected: "tới tờ giấy của cặp; gửi đề nghị xong, màn nói đang chờ B", status: "PASS", evidence: ["EV-E4-A1-C1"], ghiChu: "tới /groups/[id]/to-giay?ru=1 từ «Tạo mới»; ảnh sau khi gửi: chữ ký «Hạ Kiểm Thử · Người đề nghị», «Chờ Khôi bạn đi dạo ký», câu «Đã đề nghị. Chờ Khôi bạn đi dạo đồng ý trên máy của họ; im lặng không phải đồng ý.». Bìa sổ phía sau ghi «Hạ Kiểm…» (tên cắt trên bìa, UI-090)" });

// ------------------------------------------- automatic verdicts the capture overturns
lat("TC-E4-A-THEO", { status: "FAIL", issue: "UI-084", evidence: ["EV-E4-A2-C1", "EV-E4-C1-ghep"], lyDo: "thân màn của A có sang sổ đã mở (data-testid giay-trong), nhưng sheet «Lập sổ hai người» vẫn phủ lên, đã quay về «Bạn ký khi bấm đề nghị · Chờ Khôi bạn đi dạo ký» với con dấu «Đề nghị lập sổ» và «Để sau»: đúng UI-084 (a), nay trên hai tài khoản mới và lối «Tạo mới»" });

// ------------------------------------------- measured rows, now with issues
gan("TC-E1-LAP-NHOM", "UI-071");
gan("TC-E1-B-VAO", "UI-073");
gan("TC-E1-TEN-B", "UI-073", ["EV-E1-C1-ghep"]);
gan("TC-E2-CHAT-BIET-KEO", "UI-118", ["EV-E2-A2-C1", "EV-E2-C1-ghep"]);
gan("TC-E2-KY-NIEM", "UI-119", ["EV-E2-A5-C1", "EV-E2-C1-ghep"]);
gan("TC-E2-ALBUM", "UI-119", ["EV-E2-A6-C1", "EV-E2-C1-ghep"]);
gan("TC-E5-SAU-CHAN-SO", "UI-120", ["EV-E5-A5-C1", "EV-E5-C1-ghep"]);
gan("TC-E5-CHAN-SO-GUI", "UI-120", ["EV-E5-B4-C1", "EV-E5-A7-C1", "EV-E5-CHAN-SO-ghep"]);
gan("TC-E6-LANH-KHONG-PHIEN-QUYET-TOAN", "UI-082", ["EV-E6-L4-C1", "EV-E6-C1-ghep"]);
gan("TC-E6-VE-LAI-LINK", "UI-121", ["EV-E6-L2-C1", "EV-E6-C1-ghep"]);
gan("TC-E1-DEM-THANH-VIEN", "UI-122");

// ------------------------------------------- every frame points at its committed composite
// Single frames stay outside git; the matrix links a row only through the
// composite that carries its frame, so each row gets that composite too.
// Read the ledger again: the rows above are newer than the map built first.
const GHEP = {
  "EV-E1-C1-ghep": ["EV-E1-A1-C1", "EV-E1-A2-C1", "EV-E1-A3-C1", "EV-E1-B1-C1", "EV-E1-B2-C1", "EV-E1-A4-C1"],
  "EV-E1-DEM-ghep": ["EV-E1-DEM-C1", "EV-E1-DEM-SAU-C1"],
  "EV-E2-C1-ghep": ["EV-E2-A1-C1", "EV-E2-A2-C1", "EV-E2-A3-C1", "EV-E2-B1-C1", "EV-E2-A5-C1", "EV-E2-A6-C1"],
  "EV-E3-C1-ghep": ["EV-E3-A1-C1", "EV-E3-A3-C1", "EV-E3-A4-C1", "EV-E3-B1-C1", "EV-E3-A5-C1", "EV-E3-B2-C1"],
  "EV-E4-C1-ghep": ["EV-E4-A1-C1", "EV-E4-B1-C1", "EV-E4-B2-C1", "EV-E4-A2-C1"],
  "EV-E5-C1-ghep": ["EV-E5-A1-C1", "EV-E5-B2-C1", "EV-E5-A3-C1", "EV-E5-B3-C1", "EV-E5-A5-C1", "EV-E5-A6-C1"],
  "EV-E5-CHAN-SO-ghep": ["EV-E5-B3-C1", "EV-E5-B4-C1", "EV-E5-A7-C1"],
  "EV-E6-C1-ghep": ["EV-E6-A2-C1", "EV-E6-A3-C1", "EV-E6-L1-C1", "EV-E6-L4-C1", "EV-E6-L2-C1", "EV-E6-L3-C1"],
};
const moiNhat = new Map();
for (const r of so.doc()) {
  if (r.rut) {
    for (const k of [...moiNhat.keys()]) if (k.startsWith(`${r.tc}|`)) moiNhat.delete(k);
    continue;
  }
  moiNhat.set(`${r.tc}|${r.nenTang ?? "web"}|${r.cauHinh ?? ""}`, r);
}
for (const r of moiNhat.values()) {
  if (!/^E\d$/.test(r.feature) || (r.nenTang ?? "web") !== "web") continue;
  const ev = r.evidence ?? [];
  const them = Object.entries(GHEP).filter(([g, khung]) => !ev.includes(g) && khung.some((k) => ev.includes(k))).map(([g]) => g);
  if (!them.length) continue;
  const { luc: _luc, ...giu } = r;
  so.ghi({ ...giu, evidence: [...ev, ...them] });
}

// ------------------------------------------- native, one row per journey and platform
for (const e of ["E1", "E2", "E3", "E4", "E5", "E6"]) {
  for (const [nenTang, ly] of [["android", "không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)"], ["ios", "không chạy được iOS: không có macOS / iOS Simulator"]]) {
    so.ghi({ feature: e, tc: `TC-${e}-NATIVE`, screen: e, layer: "-", state: "cả luồng", action: "đi hết luồng như trên web", nenTang, cauHinh: "-", expected: `như web, trên thiết bị ${nenTang === "android" ? "Android" : "iOS"}`, status: "BLOCKED", method: "STATIC", evidence: [], issue: null, ghiChu: ly });
  }
}
