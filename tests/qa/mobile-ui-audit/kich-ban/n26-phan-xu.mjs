/* N26 verdicts reached by LOOKING, with the capture that was looked at, the
 * issue each failing row of n26-hai-lop-chat.mjs now points at, the native
 * rows every screen carries, and the withdrawal of the placeholder row the
 * main ledger held for this feature. Same shape as f10-phan-xu.mjs.
 *
 *   source audit-env-main.sh && node kich-ban/n26-phan-xu.mjs
 *
 * Safe to run twice: a verdict or an issue already written as the last row of
 * its test case is not written again.
 */
import { soGhi } from "../thu-vien/ghi.mjs";

const out = process.env.AUDIT_OUT;
if (!out) throw new Error("thiếu AUDIT_OUT");
const so = soGhi(out);

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
let hang = cuoiCung();
let moi = 0;

/** A verdict by eye; skipped when the same verdict is already the last row. */
const phanXu = (rec) => {
  const ghiChu = `phân xử bằng mắt: ${rec.ghiChu}`;
  const r = hang.get(`${rec.tc}|${rec.nenTang ?? "web"}|${rec.cauHinh ?? ""}`);
  if (r && r.ghiChu === ghiChu && r.status === rec.status && (r.issue ?? null) === (rec.issue ?? null)) return;
  so.ghi({ feature: "N26", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, evidence: [], ...rec, ghiChu });
  moi++;
};
/** The measured row, kept as it is, now naming its issue. */
const ganIssue = (tc, cauHinh, issue) => {
  for (const c of cauHinh) {
    const r = hang.get(`${tc}|web|${c}`);
    if (!r) throw new Error(`không có hàng đo ${tc} ${c}; chạy n26-hai-lop-chat.mjs trước`);
    if (r.status !== "FAIL") throw new Error(`${tc} ${c} không phải FAIL (${r.status}); xem lại trước khi gắn issue`);
    if (r.issue === issue) continue;
    const { luc: _luc, ...giu } = r;
    so.ghi({ ...giu, issue });
    moi++;
  }
};

// ------------------------------------------------ the placeholder row of main
{
  const cho = so.doc().filter((r) => r.tc === "TC-N-26-HAI-LOP-CHAT");
  if (cho.length && !cho.at(-1).rut) so.rut("TC-N-26-HAI-LOP-CHAT", "hàng giữ chỗ của feature mới #26: đã audit, thay bằng các hàng TC-N26-* (n26-hai-lop-chat.mjs, n26-phan-xu.mjs)");
  hang = cuoiCung();
}

// ---------------------------------------------------------------- baselines
phanXu({ tc: "TC-N26.S01-BASE", screen: "N26.S01", state: "cặp đám bạn chat-0/chat-1, chưa có tin", action: "mở chat đôi (chưa chạm gì)", cauHinh: "C1,C2,C3", expected: "Chat hai người, đám bạn: hiển thị đủ, không tràn, không cắt, không che; chữ viết cho hai người", status: "PASS", evidence: ["EV-N26-BAN-C1", "EV-N26-BAN-C2", "EV-N26-BAN-C3"], ghiChu: "đầu «Chat Test 02 · Cuộc trò chuyện của hai mình», không hàng ghim, «Chưa mã hoá đầu cuối», trạng thái rỗng «Một lời mở đầu.» và nút «Rủ hội một buổi», ô soạn «Nhắn cho Chat Test 02». C2: placeholder xuống hai dòng, đọc trọn; C3 tối đọc được. Chữ ô soạn nằm trên, lệch nút (UI-062, đã có)" });
phanXu({ tc: "TC-N26.S02-BASE", screen: "N26.S02", state: "cặp đôi chat-8/chat-9, chưa có tin, chat-8 có bản phác tuần này", action: "mở chat đôi (chưa chạm gì)", cauHinh: "C1,C2,C3", expected: "Chat hai người, cặp đôi: hàng ghim Tờ giấy dưới tiêu đề, hiển thị đủ, không tràn, không cắt, không che", status: "PASS", evidence: ["EV-N26-DOI-C1", "EV-N26-DOI-C2", "EV-N26-DOI-C3"], ghiChu: "hàng ghim «Tờ giấy của hai mình · Bản phác, chỉ bạn thấy. Gửi đi thì người ấy mới nhận.» 78dp ở C1–C3, đọc trọn, mũi tên bên phải; trạng thái rỗng và ô soạn như đám bạn. C3 tối: ô vuông của hàng ghim nổi trên nền tối" });

// ------------------------------------------------------- judged from captures
phanXu({ tc: "TC-N26-DOI-TRONG", screen: "N26.S02", state: "cặp đôi chat-8/chat-9, chưa có tin", action: "đọc trạng thái rỗng", cauHinh: "C1", expected: "chữ của phòng cặp đôi viết cho hai người, không «hội»/«nhóm» (luật chữ hai người của khay: 762d5c5; lời nhắc cặp đôi cấm «hội»: ADR-0046 §8.4)", status: "FAIL", issue: "UI-128", evidence: ["EV-N26-DOI-C1"], ghiChu: "nút duy nhất của trạng thái rỗng là «Rủ hội một buổi», cùng nút với nhóm (GroupChatLive.tsx:816, «hiện ở mọi phòng» từ c021702), ngay dưới «Một lời mở đầu.» và «Một tin nhắn nhỏ cho Chat Test 10.»" });
phanXu({ tc: "TC-N26-LOI-SO-503", screen: "N26.S02", state: "cặp đôi chat-8/chat-9, chat-8 có bản phác tuần này; GET notebook trả 503 (page.route)", action: "Tin nhắn → chat đôi", cauHinh: "C1", expected: "hàng ghim không nói sai về tờ tuần này khi đọc sổ lỗi", status: "FAIL", issue: "UI-083", evidence: ["EV-N26-LOI-so-503-C1"], ghiChu: "hàng ghim đọc «Tờ giấy của hai mình · Chưa có tờ nào tuần này. Đi đâu không?» trong khi chat-8 đang có bản phác (hết lỗi thì đọc «Bản phác, chỉ bạn thấy…»). Cùng cơ chế UI-083: đọc sổ lỗi thì vẽ như sổ trống. Không mã lỗi nào trên màn" });
phanXu({ tc: "TC-N26-Q2-UI", screen: "N26.S03", state: "cặp đám bạn chat-6/chat-7 có một bản phác của chat-6 (phác qua API, UI-131)", action: "chat-6 mở không gian giấy, rồi chat đôi", cauHinh: "C1", expected: "tờ còn đang mở ở cặp đám bạn giữ các nút của nó (chú thích KhongGianGiay.tsx:207: tờ gửi trước khi cặp thành hội bạn, hoặc từ client cũ); chat của đám bạn không có hàng ghim Tờ giấy", status: "PASS", evidence: ["EV-N26-Q2-SO-C1", "EV-N26-Q2-CHAT-C1"], ghiChu: "không gian giấy «Hội bạn · Chat Test 08» vẽ tờ «18:30 Ăn tối · Thứ Bảy 03/10 · BẢN PHÁC» với «Gửi cho người ấy»; chat đôi không hàng nào. Đúng chú thích của mã. Bản phác đã bỏ bằng «Bỏ bản phác này» → «Bỏ bản phác» (dây: nghỉ tuần, SoDoiSong.tsx:112–114)" });

phanXu({ tc: "TC-N26-GU-BAT-LAI-CAU", screen: "N26.S04", layer: "L23", state: "chat-9 đang chia gu, máy chủ báo gu_chat.cua_toi = can_bat_lai (page.route, như TC-N26-GU-BAT-LAI-HIEN)", action: "đọc sheet «Gu của hai bạn»", cauHinh: "C1", expected: "sheet nói đúng một điều về gu của tôi trong chat: đồng ý cũ chưa phủ chat (ADR-0048 §3.1, §3.5)", status: "FAIL", issue: "UI-129", evidence: ["EV-N26-GU-BAT-LAI-C1"], ghiChu: "hai dòng liền nhau nói ngược nhau: «Chat Test 09 thấy gu của bạn; Nếp dùng nó khi phác tờ, và Rủ Đi AI dùng nó trong chat của hai bạn.» (cauGu, gu-doi.ts:52, chỉ đọc mine_shared) rồi «Bạn bật từ trước, khi lời hứa chỉ là Nếp dùng gu khi phác tờ. Muốn Rủ Đi AI dùng gu của bạn trong chat thì bật lại.» (CAU_BAT_LAI_CHO_CHAT). Theo máy chủ, gu này chưa được dùng trong chat" });
phanXu({ tc: "TC-N26-RU-FORM-CHU", screen: "N26.S06", state: "form «Kèo mới» của cặp đám bạn chat-4/chat-5, mở từ «Rủ hội mình đi chơi»", action: "đọc form", cauHinh: "C1", expected: "chữ của form viết cho hai người khi phòng là chat hai người", status: "FAIL", issue: "UI-128", evidence: ["EV-N26-RU-co-so-FORM-C1"], ghiChu: "tiêu đề «Hội mình đi đâu?», và dưới ô số người: «Chat Test 06 hiện có 2 người; bớt đi nếu chỉ một phần đi.»: tên phòng hai người là tên người kia, nên câu đọc thành «[người kia] hiện có 2 người»" });

// ------------------------------------------- measured rows, now with issues
ganIssue("TC-N26-SOAN-CHAT-RONG-BAN", ["C2", "C8"], "UI-124");
ganIssue("TC-N26-SOAN-CHAT-RONG-DOI", ["C4", "C2", "C8"], "UI-124");
ganIssue("TC-N26-NHAY-VE-CHAT", ["C1"], "UI-125");
ganIssue("TC-N26-NHAY-VE-CHAT-TRE", ["C1"], "UI-125");
ganIssue("TC-N26-CHUYEN-BEN-KIA", ["C1"], "UI-125");
ganIssue("TC-N26-MOI-BAT-DOI-DEN", ["C1"], "UI-126");
ganIssue("TC-N26-M6-BAT-DOI", ["C1"], "UI-127");
ganIssue("TC-N26-BAN-CAI-DAT", ["C1"], "UI-128");
ganIssue("TC-N26-GU-BAT-LAI-LOI", ["C1"], "UI-129");
ganIssue("TC-N26-GU-BAT-LOI", ["C1"], "UI-129");
ganIssue("TC-N26-RU-BAN-CHUA-SO", ["C1"], "UI-130");
ganIssue("TC-N26-RU-BAN-CO-SO", ["C1"], "UI-130");
ganIssue("TC-N26-Q2-PHAC-API", ["-"], "UI-131");
// Extensions of issues of the base audit, measured on this feature's screens.
ganIssue("TC-N26-MOI-VUNG-BAM", ["C6"], "UI-001");
ganIssue("TC-N26-GU-LOI", ["C6"], "UI-093");

// ------------------------------------------------------------ native rows
const LY_DO = {
  android: "không chạy được Android: container không có /dev/kvm, không có Android SDK (proxy chặn dl.google.com)",
  ios: "không chạy được iOS: không có macOS / iOS Simulator",
};
for (const [screen, route] of [
  ["N26.S01", "/groups/[id]/chat (chat hai người, đám bạn)"],
  ["N26.S02", "/groups/[id]/chat (chat hai người, cặp đôi)"],
  ["N26.S03", "/groups/[id]/to-giay (sổ của đám bạn, thang đồng ý)"],
  ["N26.S04", "sheet «Gu của hai bạn»"],
  ["N26.S05", "/dev/hai-lop-chat (bản dev có cờ fixture)"],
  ["N26.S06", "/places/[id] → «Rủ … tới đây»"],
]) {
  for (const nenTang of ["android", "ios"]) {
    const tc = `TC-${screen}-NATIVE`;
    if (hang.get(`${tc}|${nenTang}|-`)) continue;
    so.ghi({ feature: "N26", tc, screen, layer: "-", state: "mọi trạng thái", action: `mở ${route}`, nenTang, cauHinh: "-", expected: `như web, trên thiết bị ${nenTang === "android" ? "Android" : "iOS"}`, status: "BLOCKED", method: "STATIC", evidence: [], issue: null, ghiChu: LY_DO[nenTang] });
    moi++;
  }
}

// ------------------------------------------------------------------- check
hang = cuoiCung();
const n26 = [...hang.values()].filter((r) => r.feature === "N26");
const thieu = n26.filter((r) => r.status === "FAIL" && !r.issue).map((r) => `${r.tc} ${r.cauHinh}`);
const dem = {};
for (const r of n26) dem[r.status] = (dem[r.status] ?? 0) + 1;
console.log(`n26-phan-xu: ${moi} dòng mới; N26 có ${n26.length} hàng ${JSON.stringify(dem)}; FAIL thiếu issue: ${thieu.join(", ") || "không"}`);
if (thieu.length) process.exitCode = 1;
