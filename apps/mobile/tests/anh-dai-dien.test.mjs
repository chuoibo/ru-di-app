/* Khung ảnh đại diện: khi nào được hỏi máy chủ, và hỏi địa chỉ nào.
 *
 * `AvatarNguoi` là một lớp mỏng: nó chỉ đưa `nguonAvatar(...)` cho `Avatar` và
 * báo `baoAnhHong` khi khung không tải được. Mọi quyết định nằm ở module này,
 * nên cổng đo ở đây, bằng hàm thật.
 *
 * ## Không chứng minh
 *
 * Không render `AvatarNguoi`: `expo-image` không có stub trong bản build test,
 * nên chuyện `onError` của expo-image thật sự bắn trên web/native với 404/403
 * là việc của ảnh chụp màn hình, không phải của ca này. Không đo quyền máy chủ
 * (`view_person_avatar`): máy chủ quyết, client chỉ nhớ lời từ chối.
 */
import assert from "node:assert/strict";
import test from "node:test";

import {
  NHO_HONG_MS,
  baoAnhHong,
  baoDaDoiAnh,
  dangKyAnhDaiDien,
  nguonAvatar,
  nhipAnhDaiDien,
} from "../dist-test/rudi/nguoi/anh-dai-dien.js";

const A = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const B = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const C = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";

/* Ca CÓ chạy trước: không có nó thì mọi ca "trả null" bên dưới cũng xanh với
 * một hàm luôn trả null. */
test("có phiên và có id: khung hỏi đúng địa chỉ ảnh đại diện, kèm danh tính người đọc", () => {
  const nguon = nguonAvatar(B, A, 0);
  assert.ok(nguon !== null);
  assert.match(nguon.uri, new RegExp(`/people/${B}/avatar\\?v=0$`));
  assert.equal(nguon.headers["X-Actor-ID"], A);
});

test("cùng khung, cùng địa chỉ: trả lại ĐÚNG object cũ, vì expo-image web tải lại theo danh tính object", () => {
  const D = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";
  const lan1 = nguonAvatar(D, A, 0);
  assert.equal(nguonAvatar(D, A, 1), lan1, "render lại không được thành request mới");
  baoDaDoiAnh(D, A);
  const sauDoi = nguonAvatar(D, A, 2);
  assert.notEqual(sauDoi, lan1, "ảnh mới thì phải là object mới để khung tải lại");
  assert.equal(nguonAvatar(D, A, 3), sauDoi);
});

test("thiếu id người hoặc thiếu phiên: chữ cái đầu, không request", () => {
  assert.equal(nguonAvatar(null, A), null);
  assert.equal(nguonAvatar(undefined, A), null);
  assert.equal(nguonAvatar("", A), null);
  assert.equal(nguonAvatar(B, null), null);
  assert.equal(nguonAvatar(B, undefined), null);
});

test("bị từ chối thì nhớ: không hỏi lại trong NHO_HONG_MS, hết hạn thì được hỏi lại", () => {
  const t0 = 1_000_000;
  assert.ok(nguonAvatar(C, A, t0) !== null);
  baoAnhHong(C, A, t0);
  assert.equal(nguonAvatar(C, A, t0 + 1), null, "vừa hỏng thì không bắn lại");
  assert.equal(nguonAvatar(C, A, t0 + NHO_HONG_MS - 1), null, "ngay trước hạn vẫn không bắn");
  assert.ok(nguonAvatar(C, A, t0 + NHO_HONG_MS) !== null, "tới hạn thì hỏi lại, có thể người ấy vừa tải ảnh");
});

test("một người hỏng không kéo người khác thành chữ cái", () => {
  baoAnhHong(B, A, 5);
  assert.equal(nguonAvatar(B, A, 6), null);
  assert.ok(nguonAvatar(C, A, 6) !== null);
});

test("đổi ảnh: xoá lời từ chối và đổi khoá cache của mọi khung", () => {
  baoAnhHong(B, A, 10);
  assert.equal(nguonAvatar(B, A, 11), null);
  baoDaDoiAnh(B, A);
  const sau = nguonAvatar(B, A, 12);
  assert.ok(sau !== null, "ảnh mới phải hiện ngay, không chờ hết hạn");
  assert.match(sau.uri, /\?v=1$/);
  baoDaDoiAnh(B, A);
  assert.match(nguonAvatar(B, A, 13).uri, /\?v=2$/);
});

test("đổi tài khoản: quên sạch những gì tài khoản trước biết", () => {
  baoAnhHong(C, A, 20);
  baoDaDoiAnh(B, A);
  assert.equal(nguonAvatar(C, A, 21), null);
  const khac = nguonAvatar(C, B, 22);
  assert.ok(khac !== null, "lời từ chối của A không nói gì về quyền của B");
  assert.equal(khac.headers["X-Actor-ID"], B);
  assert.match(nguonAvatar(B, B, 23).uri, /\?v=0$/, "bộ đếm tải lên của A không đi theo sang B");
});

test("mỗi lần báo, người nghe được gọi và nhịp đổi; báo hỏng lặp lại không phát thêm", () => {
  let goi = 0;
  const bo = dangKyAnhDaiDien(() => {
    goi += 1;
  });
  const truoc = nhipAnhDaiDien();
  baoDaDoiAnh(A, C);
  assert.equal(goi, 1);
  assert.notEqual(nhipAnhDaiDien(), truoc);
  baoAnhHong(B, C, 30);
  baoAnhHong(B, C, 31);
  assert.equal(goi, 2, "một khung hỏng hai lần là một tin, không phải hai lần render lại cả danh sách");
  bo();
  baoDaDoiAnh(A, C);
  assert.equal(goi, 2, "đã huỷ đăng ký thì không nghe nữa");
});
