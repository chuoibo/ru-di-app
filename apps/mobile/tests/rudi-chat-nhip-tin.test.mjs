// The rhythm of a thread (QA UI-062, UI-063, UI-064, UI-068, UI-069, UI-164):
// time bands, runs, bubble corners, the sentence for a failed action, and the
// source rules of the screen that draws them.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

import { ApiError } from "../dist-test/api.js";
import { NGHI_MS, cauLoiThaoTac, cungGiong, gocBong, ngayDiaPhuong, nhanNgayDiaPhuong, nhomTheoQuang, viTriTrongCum } from "../dist-test/rudi/chat/nhip-tin.js";

const HERE = dirname(fileURLToPath(import.meta.url));
const LIVE = readFileSync(join(HERE, "..", "src", "rudi", "screens", "chat", "GroupChatLive.tsx"), "utf8");
const A = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const B = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";

/** A message at a LOCAL wall-clock time, so the test reads the same in every time zone. */
function tin(id, tac, nam, thang, ngay, gio, phut, kind = "text") {
  const luc = new Date(nam, thang - 1, ngay, gio, phut);
  return { id, context_id: "c", author_id: tac, kind, body: id, image_url: null, card: null, created_at: luc.toISOString(), cursor: id };
}
const nhan = (hang) => hang.map((h) => (h.loai === "tin" ? h.tin.id : `[${h.nhan}]`));

test("một quãng nói liền chỉ có MỘT nhãn giờ, đặt trên tin cũ nhất của quãng", () => {
  const homNay = new Date(2030, 8, 27, 20, 0);
  // Newest first, as the inverted list wants: eleven messages in one minute.
  const ds = Array.from({ length: 11 }, (_, i) => tin(`t${10 - i}`, i % 3 ? A : B, 2030, 9, 27, 13, 27));
  const hang = nhomTheoQuang(ds, homNay);
  const bang = hang.filter((h) => h.loai === "ngay");
  assert.equal(bang.length, 1, "11 tin trong một phút: một dải");
  assert.equal(bang[0].nhan, "Hôm nay · 13:27");
  assert.equal(hang[hang.length - 1].loai, "ngay", "dải nằm sau tin cũ nhất trong thứ tự list, tức là ở trên nó");
  assert.equal(bang[0].key, "quang-t0", "khoá theo tin mở quãng, đứng yên khi tin mới tới");
});

test("nghỉ quá 15 phút mở quãng mới chỉ có giờ; sang ngày khác có cả ngày", () => {
  const homNay = new Date(2030, 8, 27, 20, 0);
  const ds = [
    tin("d", A, 2030, 9, 27, 15, 40),
    tin("c", A, 2030, 9, 27, 13, 30),
    tin("b", B, 2030, 9, 27, 13, 27),
    tin("a", B, 2030, 9, 26, 21, 4),
  ];
  assert.deepEqual(nhan(nhomTheoQuang(ds, homNay)), ["d", "[15:40]", "c", "b", "[Hôm nay · 13:27]", "a", "[Hôm qua · 21:04]"]);
  // Exactly the threshold is still one stretch.
  const sat = [tin("y", A, 2030, 9, 27, 13, 42), tin("x", A, 2030, 9, 27, 13, 27)];
  assert.equal(NGHI_MS, 15 * 60_000);
  assert.equal(nhomTheoQuang(sat, homNay).filter((h) => h.loai === "ngay").length, 1);
});

test("ngày theo giờ máy, không theo UTC", () => {
  // 06:30 on the phone is that day's morning wherever the phone is.
  const sang = new Date(2030, 8, 27, 6, 30);
  assert.equal(ngayDiaPhuong(sang), "2030-09-27");
  assert.equal(nhanNgayDiaPhuong("2030-09-27", new Date(2030, 8, 27, 23, 0)), "Hôm nay");
  assert.equal(nhanNgayDiaPhuong("2030-09-26", new Date(2030, 8, 27, 0, 5)), "Hôm qua");
  assert.equal(nhanNgayDiaPhuong("2030-09-20", new Date(2030, 8, 27)), "20/09");
});

test("cụm: cùng người, không dải giờ xen giữa, không phải thẻ", () => {
  const homNay = new Date(2030, 8, 27, 20, 0);
  const ds = [
    tin("c3", A, 2030, 9, 27, 13, 29),
    tin("c2", A, 2030, 9, 27, 13, 28),
    tin("c1", A, 2030, 9, 27, 13, 27),
    tin("p", A, 2030, 9, 27, 13, 26, "ai_card"),
    tin("b", B, 2030, 9, 27, 13, 25),
  ];
  const hang = nhomTheoQuang(ds, homNay);
  const viTri = (id) => viTriTrongCum(hang, hang.findIndex((h) => h.loai === "tin" && h.tin.id === id));
  // Inverted: the newest (c3) is the run's last, the oldest (c1) its first.
  assert.equal(viTri("c3"), "cuoi");
  assert.equal(viTri("c2"), "giua");
  assert.equal(viTri("c1"), "dau");
  assert.equal(viTri("p"), "mot", "thẻ bình chọn không nhập vào cụm bong bóng");
  assert.equal(viTri("b"), "mot");
  assert.equal(cungGiong(hang[0], undefined), false);
});

test("góc bong bóng: phía cụm treo khít lại, phía ngoài giữ tròn, chân tin cuối của người khác tròn", () => {
  assert.deepEqual(gocBong("mot", false), { borderTopRightRadius: 18, borderBottomRightRadius: 18, borderTopLeftRadius: 18, borderBottomLeftRadius: 18 });
  assert.deepEqual(gocBong("dau", false), { borderTopRightRadius: 18, borderBottomRightRadius: 18, borderTopLeftRadius: 18, borderBottomLeftRadius: 6 });
  assert.deepEqual(gocBong("giua", false), { borderTopRightRadius: 18, borderBottomRightRadius: 18, borderTopLeftRadius: 6, borderBottomLeftRadius: 6 });
  assert.deepEqual(gocBong("cuoi", false), { borderTopRightRadius: 18, borderBottomRightRadius: 18, borderTopLeftRadius: 6, borderBottomLeftRadius: 18 });
  assert.deepEqual(gocBong("giua", true), { borderTopLeftRadius: 18, borderBottomLeftRadius: 18, borderTopRightRadius: 6, borderBottomRightRadius: 6 });
});

test("câu lỗi của một thao tác nêu đúng thao tác, và chỉ mời thử lại khi bấm lại có ích", () => {
  const mang = cauLoiThaoTac("thả ❤️", new TypeError("fetch failed"));
  assert.match(mang.cau, /^Chưa thả ❤️: máy chưa nối được Rủ Đi/);
  assert.equal(mang.thuLai, true);
  const hong = cauLoiThaoTac("xoá được tin này", new ApiError(503, "chat_temporarily_unavailable", "x"));
  assert.match(hong.cau, /^Chưa xoá được tin này: Rủ Đi đang trục trặc\. Chưa có gì thay đổi\.$/);
  assert.equal(hong.thuLai, true);
  const mat = cauLoiThaoTac("xoá được tin này", new ApiError(404, "message_not_found", "Tin này không còn."));
  assert.equal(mat.cau, "Chưa xoá được tin này. Tin này không còn.");
  assert.equal(mat.thuLai, false, "tin đã mất: bấm lại không đổi được gì");
  assert.doesNotMatch(hong.cau, /việc này/, "không câu chung «việc này»");
  assert.equal(cauLoiThaoTac("tải được tin nhắn", new ApiError(401, "session_expired", "Phiên đăng nhập đã hết.")).thuLai, false, "hết phiên: đăng nhập lại, không thử lại");
  assert.match(LIVE, /chat\.loiLoai === "phien"\s*\? \{ label: "Đăng nhập lại"/);
  assert.match(LIVE, /chat\.loiLoai === "vinh-vien"\s*\? \{ label: "Về Tin nhắn"/);
});

test("màn chat: lỗi không mặc áo AI, nằm tại chỗ; khối «Đang hỏi» chỉ khi lệnh sẵn sàng", () => {
  // No notice in the thread wears the model's sparkle any more: the only
  // sparkle rows left are the AI's own («Đang hỏi Rủ Đi AI», its cards).
  assert.doesNotMatch(LIVE, /setThongBao|thongBao\.tu/, "thẻ «Đã hiểu» mang dấu AI đã bỏ");
  assert.match(LIVE, /setLoiTin\(\{ id: tin\.id/, "lỗi thả cảm xúc / xoá tin nằm dưới đúng tin đó");
  assert.match(LIVE, /<CauTaiCho cau=\{loiCuoi\.cau\}/, "lỗi của ô soạn là một câu tại chỗ");
  // QA UI-164: the pending AI block follows the same readiness as the send.
  assert.match(LIVE, /nhac !== null && lenhSanSang\(ai\.capabilities, nhac\.lenh\)/);
  // QA UI-068: the first-page error only while nothing is on screen, with «Thử lại».
  assert.match(LIVE, /cau=\{chat\.loi\}\s*hanhDong=\{chat\.loiLoai/);
  assert.match(LIVE, /: \{ label: "Thử lại", onPress: \(\) => void chat\.taiLai\(\) \}/);
  assert.match(LIVE, /cau=\{chat\.loiCu\} hanhDong=\{\{ label: "Thử lại"/);
});

test("màn chat: avatar cùng hàng với bong bóng, giờ chỉ ở dải; ô soạn một dòng, tự cao", () => {
  // The avatar and the bubble are the only things on the measured line.
  const hang = LIVE.slice(LIVE.indexOf("testID={`chat-message-${tin.id}`}"), LIVE.indexOf("{chips.length > 0 ? ("));
  assert.match(hang, /AvatarNguoi/);
  assert.doesNotMatch(hang, /glyphPhanUng|gioPhut/, "cảm xúc và giờ không nằm trên hàng của avatar");
  assert.doesNotMatch(LIVE, /gioPhut\(tin\.created_at\)/, "không còn giờ dưới mỗi cụm");
  assert.match(LIVE, /nhomTheoQuang\(tinHien\)/);
  // QA UI-062: one row on the web, measured height between one line and the ceiling.
  assert.match(LIVE, /Platform\.OS === "web" \? \{ rows: 1 \}/);
  assert.match(LIVE, /o\.style\.height = "0px"/);
  assert.match(LIVE, /const DEM_O = \(48 - DONG_O\) \/ 2/);
  assert.match(LIVE, /const caoOToiDa = Math\.min\(4 \* DONG_O \+ 2 \* DEM_O, /, "trần 120: bốn dòng");
  // QA UI-063: a long word breaks inside the bubble on the web.
  assert.match(LIVE, /wordBreak: "break-word"/);
  assert.match(LIVE, /khoi: \{ maxWidth: "82%", minWidth: 0/);
});
