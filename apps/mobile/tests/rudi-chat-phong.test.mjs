// Source rules for the chat room's surroundings (QA UI-065, UI-067, UI-079,
// UI-122, UI-124, UI-125, UI-128, UI-165): the poll note, the report reasons,
// a room that stopped, the roster count, the empty page, the room's class, and
// the live regions. Read from source because each is a property of a component
// tree this suite does not render.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..", "src");
const doc = (...p) => readFileSync(join(SRC, ...p), "utf8");
const THE = doc("rudi", "screens", "chat", "TheAi.tsx");
const LIVE = doc("rudi", "screens", "chat", "GroupChatLive.tsx");

test("thẻ bình chọn: một hàng một lựa chọn, «N phiếu» một lần, thanh tỉ lệ, trần 560dp", () => {
  const poll = THE.slice(THE.indexOf("function ThePoll"));
  // Per row only the number; the word «phiếu» once, for the total.
  // «N phiếu» per row only as the spoken value on native, never printed.
  const moiHang = poll.match(/`\$\{so\} phiếu[^`]*`/g) ?? [];
  assert.equal(moiHang.length, 1, "không còn «N phiếu» in trên mỗi hàng");
  assert.match(poll, /accessibilityValue=\{Platform\.OS !== "web" && so > 0 \? \{ text: `\$\{so\} phiếu` \}/);
  assert.match(poll, /\{tong > 0 \? `\$\{tong\} phiếu` : "Chưa có phiếu"\}/);
  assert.match(poll, /tong > 0 \|\| dong \|\| \(cuaNguoiTao && !confirmClose\) \?/, "chưa ai bầu thì không có dòng chân, trừ nút chốt của người tạo");
  // One scale for the whole poll, exposed as a progress bar.
  assert.match(poll, /flex: so, backgroundColor: muc/);
  assert.match(poll, /flex: Math\.max\(tong - so, so === 0 \? 1 : 0\)/);
  assert.doesNotMatch(poll, /\{tong > 0 \? \(\s*<View\s*accessibilityRole="progressbar"/, "rãnh có cả khi chưa ai bầu");
  assert.match(poll, /accessibilityRole="progressbar"/);
  // ADR-0037 D1 kept: ballots are still thumbprints, a few, then the number.
  assert.match(poll, /lop=\{HINH_VAN_TAY\}/);
  assert.match(poll, /Math\.min\(so, 3\)/);
  assert.match(THE, /thePoll: \{[^}]*maxWidth: 560/);
  assert.match(THE, /luaChon: \{[^}]*minHeight: 48/, "mỗi hàng vẫn là đích bấm 48dp");
  // The creator's close shares the foot line with the total, in the words the handbook uses.
  assert.match(poll, />Chốt bình chọn<\/Text>/);
});

test("báo cáo: năm lý do là radio có trạng thái, không phải năm nút đổi màu", () => {
  const bc = doc("rudi", "screens", "nguoi", "NoiDungBaoCao.tsx");
  assert.match(bc, /accessibilityRole="radiogroup"/);
  assert.match(bc, /toggleState\("radio", chon, /);
  assert.doesNotMatch(bc, /variant=\{lyDo === muc\.ma \? "soft" : "ghost"\}/);
});

test("phòng đã dừng: luồng thôi nối lại khi máy chủ từ chối, người chặn được nói rõ", () => {
  const feed = doc("rudi", "chat", "useChatChanges.ts");
  assert.match(feed, /error\.status === 403\) \{\s*unsupported = true;\s*setConnection\("dung"\)/);
  assert.match(LIVE, /changes\.connection === "recovering"/, "«Đang nối lại» chỉ khi thật sự đang nối lại");
  // The blocker reads their own list; nothing is said to the blocked side.
  assert.match(LIVE, /docDaChan\(personId\)/);
  assert.match(LIVE, /Bạn đã chặn \$\{tenNhom\}\./);
  assert.match(LIVE, /khongNhanTin \|\| gonDau \? null : \(/, "không mời mở lời trong phòng đã dừng; trang rỗng ẩn khi khay mở ở cửa sổ thấp");
});

test("số thành viên đọc lại khi người vừa vào nhắn, kể cả khi tên đã biết từ lời mời", () => {
  assert.match(LIVE, /!dangOIds\.has\(t\.author_id\) && !tinLucDoc\.current\.has\(t\.id\)/);
  assert.match(LIVE, /setDangOIds\(new Set\(ds\.filter\(\(tv\) => tv\.state === "active"\)/);
});

test("trang rỗng là phần co giãn của cột, ô soạn luôn ở đáy", () => {
  assert.match(LIVE, /<ScrollView contentContainerStyle=\{\[styles\.rongNoi/);
  assert.match(LIVE, /rong: \{ flex: 1 \}/);
  assert.match(LIVE, /style=\{rongTrang \? styles\.dsRong : styles\.ds\}/);
  assert.match(LIVE, /thapCuaSo \|\| khay !== null \? null : <Nep/, "hình vẽ nhường chỗ trước");
  assert.match(LIVE, /const gonDau = khay !== null && thapCuaSo;/);
  assert.match(LIVE, /\(toHen \|\| keoTrenBang\) && !gonDau/, "dải ghim nhường chỗ cho khay ở cửa sổ thấp");
  assert.match(LIVE, /!khongNhanTin && !gonDau \? <HangToGiaySong/, "hàng tờ giấy cũng vậy");
  assert.match(LIVE, /veCuoi: \{ position: "absolute"/, "nút «Tin mới nhất» nổi trên danh sách, không lấy chỗ của nó");
  assert.match(doc("rudi", "screens", "chat", "SoHen.tsx"), /tools: \{[^}]*flexShrink: 1, minHeight: 0/, "khay co lại thay vì đẩy ô soạn ra ngoài");
  assert.match(LIVE, /label=\{nhanRieng \? "Rủ đi một buổi" : "Rủ hội một buổi"\}/);
});

test("lớp phòng giữ nguyên khi quay lại, và phòng hai người tự đọc lại", () => {
  const ai = doc("rudi", "chat", "useChatAi.ts");
  assert.match(ai, /if \(phongDaDoc\.current !== phong\) \{\s*phongDaDoc\.current = phong;\s*setCapabilities\(null\)/);
  assert.match(ai, /haiNguoi \? setInterval\(\(\) => \{ void refresh\(true\); \}, NHIP_DOC_LAI_PHONG_DOI_MS\)/);
});

test("chữ của phòng hai người: cài đặt và form kèo mới", () => {
  const cd = doc("rudi", "screens", "chat", "CaiDatNhom.tsx");
  assert.match(cd, /laPair \? "Cài đặt cuộc trò chuyện" : "Cài đặt nhóm"/);
  assert.match(cd, /laPair \? "Hai bạn thấy cùng một màu\." : "Cả nhóm thấy cùng một màu\."/);
  assert.match(LIVE, /nhanRieng \? "Cài đặt cuộc trò chuyện" : "Cài đặt nhóm"/);
  const keo = doc("rudi", "screens", "keo", "CreateOutingLive.tsx");
  assert.match(keo, /nhom\.haiNguoi \? "Hai bạn đi đâu\?" : "Hội mình đi đâu\?"/);
  assert.match(keo, /nhom\.soNguoi && !nhom\.haiNguoi \?/, "không «X hiện có 2 người» khi X là người kia");
});

test("chữ AI hiện dần và dòng trạng thái nằm trong vùng thông báo", () => {
  const dv = doc("rudi", "screens", "chat", "TraLoiAiDangViet.tsx");
  assert.match(dv, /\{\.\.\.vungSong\(traLoi\.pha !== "xong"\)\}/);
  assert.match(dv, /<View accessibilityLiveRegion="polite" style=\{\[styles\.bong/);
  const nep = doc("rudi", "nep", "NepPhien.tsx");
  assert.match(nep, /\{\.\.\.vungSong\(song\?\.pha !== "xong"\)\}/);
  assert.match(nep, /accessibilityLiveRegion="polite"[^>]*testID="nep-dang-nghi"/);
  assert.match(doc("rudi", "nep", "NepBang.tsx"), /accessibilityLiveRegion="polite"[^>]*testID="nep-loi-hoi"/);
  const a11y = doc("ui", "a11y.ts");
  assert.match(a11y, /accessibilityLiveRegion: "polite", "aria-busy": dangChay/);
});
