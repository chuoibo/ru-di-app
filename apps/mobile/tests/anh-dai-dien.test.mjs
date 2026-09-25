/* Khung ảnh đại diện: phiên bản nào, hỏi khi nào, và ai thắng khi hai tin tới lệch nhau.
 *
 * `AvatarNguoi` chỉ đưa `nguonAvatar(...)` cho `Avatar`; mọi quyết định nằm ở
 * module này, nên cổng đo ở đây bằng hàm thật, với nguồn phiên bản giả thay
 * cho `GET /people/avatars` và bộ hẹn giờ chạy tay.
 *
 * Mỗi ca dùng một người đọc (actorId) riêng: đổi người đọc là module tự xoá
 * sạch, nên các ca không dính trạng thái của nhau -- và đó cũng là một luật
 * được đo ở dưới.
 *
 * ## Không chứng minh
 *
 * Không render `AvatarNguoi` (expo-image không có stub trong bản build test);
 * không đo máy chủ -- quyền và thứ tự "ảnh mới nhất" được đo ở
 * services/core/internal/avatarfeed/postgres_test.go.
 */
import assert from "node:assert/strict";
import test from "node:test";

import {
  TOI_DA_MOT_LAN,
  baoAnhHong,
  baoDaDoiAnh,
  dangKyAnhDaiDien,
  datNguonPhienBan,
  lamMoiTatCa,
  nguonAvatar,
  nhanSuKien,
  nhipAnhDaiDien,
} from "../dist-test/rudi/nguoi/anh-dai-dien.js";

const A = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const B = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const C = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const chu = (n, dai) => String(n).padStart(dai, "0").replace(/[0-9]/g, (d) => "abcdefghij"[d]);
let dem = 0;
const nguoiDocMoi = () => `dddddddd-dddd-4ddd-8ddd-${chu(++dem, 12)}`;

const goi = [];
const hen = [];
datNguonPhienBan(
  (ids, actorId) => new Promise((tra, loi) => goi.push({ ids, actorId, tra, loi })),
  (f) => hen.push(f),
);
const nhuong = () => new Promise((r) => setImmediate(r));
async function chayHen() {
  while (hen.length) hen.shift()();
  await nhuong();
}
async function tra(lan, dapAn) {
  lan.tra(dapAn);
  await nhuong();
  await nhuong();
}
/* Callbacks left over from the previous case run first (dropping one would
 * leave the module's "already scheduled" flag set forever), then the ledger
 * of requests starts clean. */
function batDau() {
  while (hen.length) hen.shift()();
  goi.length = 0;
  return nguoiDocMoi();
}

/* Ca CÓ chạy trước: không có nó thì mọi ca "trả null" bên dưới cũng xanh với
 * một hàm luôn trả null. */
test("chưa biết thì hỏi gộp một lần; biết rồi thì địa chỉ mang đúng phiên bản", async () => {
  const doc = batDau();
  assert.equal(nguonAvatar(A, doc), null, "chưa biết: chữ cái, không đoán");
  assert.equal(nguonAvatar(B, doc), null);
  assert.equal(nguonAvatar(A, doc), null, "gọi lại khi đang chờ không thành request thứ hai");
  await chayHen();
  assert.equal(goi.length, 1, "ba lần vẽ, hai người -> MỘT request");
  assert.deepEqual([...goi[0].ids].sort(), [A, B].sort());
  assert.equal(goi[0].actorId, doc);
  await tra(goi[0], { [A]: "v-a-1", [B]: null });
  const nguon = nguonAvatar(A, doc);
  assert.ok(nguon !== null);
  assert.match(nguon.uri, new RegExp(`/people/${A}/avatar\\?v=v-a-1$`));
  assert.equal(nguon.headers["X-Actor-ID"], doc);
  assert.equal(nguonAvatar(B, doc), null, "null = chưa có ảnh: chữ cái");
  await chayHen();
  assert.equal(goi.length, 1, "đã biết (kể cả null) thì không hỏi lại, không có 404 nào");
});

test("người không được xem (vắng trong câu trả lời) là chữ cái, và không bị hỏi lại mãi", async () => {
  const doc = batDau();
  nguonAvatar(C, doc);
  await chayHen();
  await tra(goi[0], {});
  assert.equal(nguonAvatar(C, doc), null);
  await chayHen();
  assert.equal(goi.length, 1);
});

test("hơn 100 người thì chia lô, mỗi lô tối đa 100", async () => {
  const doc = batDau();
  const ids = Array.from({ length: TOI_DA_MOT_LAN + 5 }, (_, i) => `${chu(i, 8)}-aaaa-4aaa-8aaa-aaaaaaaaaaaa`);
  for (const id of ids) nguonAvatar(id, doc);
  await chayHen();
  assert.equal(goi[0].ids.length, TOI_DA_MOT_LAN);
  await tra(goi[0], {});
  assert.equal(goi.length, 2);
  assert.equal(goi[1].ids.length, 5);
});

test("cùng khung, cùng phiên bản: ĐÚNG object cũ; phiên bản mới: object mới", async () => {
  const doc = batDau();
  nguonAvatar(A, doc);
  await chayHen();
  await tra(goi[0], { [A]: "v1" });
  const lan1 = nguonAvatar(A, doc);
  assert.equal(nguonAvatar(A, doc), lan1, "expo-image web tải lại theo danh tính object");
  nhanSuKien({ type: "avatar", person_id: A, avatar_id: "v2" }, doc);
  const lan2 = nguonAvatar(A, doc);
  assert.notEqual(lan2, lan1);
  assert.match(lan2.uri, /\?v=v2$/);
});

test("tin mới thắng câu trả lời cũ: đẩy tới giữa lúc đang hỏi thì câu trả lời muộn không ghi đè", async () => {
  const doc = batDau();
  nguonAvatar(A, doc);
  await chayHen();
  const dangBay = goi[0];
  nhanSuKien({ type: "avatar", person_id: A, avatar_id: "v-moi" }, doc);
  await tra(dangBay, { [A]: "v-cu" });
  assert.match(nguonAvatar(A, doc).uri, /\?v=v-moi$/);
});

test("tự đổi ảnh: có id thì đổi ngay không cần hỏi; không có id thì hỏi lại", async () => {
  const doc = batDau();
  baoDaDoiAnh(A, doc, "v-vua-tai");
  assert.match(nguonAvatar(A, doc).uri, /\?v=v-vua-tai$/);
  await chayHen();
  assert.equal(goi.length, 0);
  baoDaDoiAnh(A, doc);
  assert.equal(nguonAvatar(A, doc), null);
  await chayHen();
  assert.deepEqual(goi[0].ids, [A]);
});

test("ảnh không tải được thì chữ cái cho ĐÚNG phiên bản đó; phiên bản mới thì thử lại", async () => {
  const doc = batDau();
  baoDaDoiAnh(A, doc, "v-hong");
  baoAnhHong(A, doc);
  assert.equal(nguonAvatar(A, doc), null);
  nhanSuKien({ type: "avatar", person_id: A, avatar_id: "v-lanh" }, doc);
  assert.ok(nguonAvatar(A, doc) !== null);
});

test("đổi người đọc: quên sạch, và câu trả lời của người đọc cũ bị bỏ", async () => {
  const cu = batDau();
  nguonAvatar(A, cu);
  await chayHen();
  const dangBay = goi[0];
  const moi = nguoiDocMoi();
  assert.equal(nguonAvatar(A, moi), null);
  await tra(dangBay, { [A]: "v-cua-nguoi-cu" });
  assert.equal(nguonAvatar(A, moi), null, "quyền của người đọc cũ không nói gì về người mới");
});

test("ready từ luồng: hỏi lại tất cả đã biết; khung lạ bị bỏ qua", async () => {
  const doc = batDau();
  baoDaDoiAnh(A, doc, "v1");
  baoDaDoiAnh(B, doc, null);
  for (const rac of [null, "x", { type: "avatar" }, { type: "avatar", person_id: A, avatar_id: 7 }, { type: "khac" }]) nhanSuKien(rac, doc);
  assert.match(nguonAvatar(A, doc).uri, /\?v=v1$/, "khung hỏng không đổi gì");
  nhanSuKien({ type: "ready" }, doc);
  await chayHen();
  assert.deepEqual([...goi[0].ids].sort(), [A, B].sort());
  await tra(goi[0], { [A]: "v2", [B]: "v-b" });
  assert.match(nguonAvatar(A, doc).uri, /\?v=v2$/);
  assert.match(nguonAvatar(B, doc).uri, /\?v=v-b$/);
});

test("request hỏng thì giữ 'chưa biết' và lần vẽ sau hỏi lại", async () => {
  const doc = batDau();
  nguonAvatar(A, doc);
  await chayHen();
  goi[0].loi(new Error("mạng"));
  await nhuong();
  await nhuong();
  assert.equal(nguonAvatar(A, doc), null);
  await chayHen();
  assert.equal(goi.length, 2, "vẽ lại một người chưa biết là hỏi lại");
});

test("người nghe được báo mỗi lần có tin; huỷ đăng ký thì thôi", async () => {
  const doc = batDau();
  let n = 0;
  const bo = dangKyAnhDaiDien(() => (n += 1));
  const truoc = nhipAnhDaiDien();
  nhanSuKien({ type: "avatar", person_id: A, avatar_id: "v1" }, doc);
  assert.equal(n, 1);
  assert.notEqual(nhipAnhDaiDien(), truoc);
  bo();
  nhanSuKien({ type: "avatar", person_id: A, avatar_id: "v2" }, doc);
  assert.equal(n, 1);
  lamMoiTatCa(doc);
});
