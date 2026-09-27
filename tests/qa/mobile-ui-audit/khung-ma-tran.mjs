/* Seed the ledger with the rows that must exist before anything runs.
 *
 *   AUDIT_OUT=… node khung-ma-tran.mjs
 *
 * Idempotent: rows are keyed, and tong-hop.mjs keeps the LAST record per key,
 * so a later PASS/FAIL replaces the NOT_TESTED seed and re-seeding after a
 * result would be a mistake this script refuses (it only writes keys that are
 * not in the ledger yet).
 */
import { LOP, MAN } from "./danh-muc.mjs";
import { soGhi } from "./thu-vien/ghi.mjs";

const out = process.env.AUDIT_OUT;
if (!out) throw new Error("đặt AUDIT_OUT");
const so = soGhi(out);
const daCo = new Set(so.doc().map((r) => `${r.tc}|${r.nenTang ?? "web"}|${r.cauHinh ?? ""}`));
let moi = 0;
const ghi = (rec) => {
  const k = `${rec.tc}|${rec.nenTang}|${rec.cauHinh}`;
  if (daCo.has(k)) return;
  so.ghi(rec);
  daCo.add(k);
  moi++;
};

const LY_DO_ANDROID = "không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)";
const LY_DO_IOS = "không chạy được iOS: không có macOS / iOS Simulator";
const LY_DO_CHU = "react-native-web cố định fontScale = 1.0; cỡ chữ 1.3/2.0 chỉ đo được trên máy thật";

for (const [feature, screen, route, ten] of MAN) {
  ghi({ tc: `TC-${screen}-BASE`, feature, screen, layer: "-", state: "baseline, dữ liệu seed", action: `mở ${route}`, nenTang: "web", cauHinh: "C1,C2,C3", expected: `${ten}: hiển thị đủ, không tràn, không cắt, không che, vùng bấm và tên truy cập đạt`, status: "NOT_TESTED", method: "RUNTIME-WEB", evidence: [], issue: null });
  ghi({ tc: `TC-${screen}-NATIVE`, feature, screen, layer: "-", state: "mọi trạng thái", action: `mở ${route}`, nenTang: "android", cauHinh: "-", expected: "như web, trên thiết bị Android", status: "BLOCKED", method: "STATIC", evidence: [], issue: null, ghiChu: LY_DO_ANDROID });
  ghi({ tc: `TC-${screen}-NATIVE`, feature, screen, layer: "-", state: "mọi trạng thái", action: `mở ${route}`, nenTang: "ios", cauHinh: "-", expected: "như web, trên thiết bị iOS", status: "BLOCKED", method: "STATIC", evidence: [], issue: null, ghiChu: LY_DO_IOS });
  ghi({ tc: `TC-${screen}-FONT`, feature, screen, layer: "-", state: "cỡ chữ hệ thống 1.3 và 2.0", action: `mở ${route}`, nenTang: "web", cauHinh: "-", expected: "không mất nội dung, không mất vùng bấm", status: "BLOCKED", method: "STATIC", evidence: [], issue: null, ghiChu: LY_DO_CHU });
}
for (const [feature, layer, screen, ten] of LOP) {
  ghi({ tc: `TC-${layer}-VONGDOI`, feature, screen, layer, state: "đóng → mở → dùng → đóng → mở lại", action: `vòng đời ${ten}`, nenTang: "web", cauHinh: "C1", expected: "mở đúng chỗ, đóng hết mọi cách được thiết kế, không sót lớp chặn, focus trả về", status: "NOT_TESTED", method: "RUNTIME-WEB", evidence: [], issue: null });
}
console.log(`khung ma trận: thêm ${moi} hàng (sổ: ${so.file})`);
