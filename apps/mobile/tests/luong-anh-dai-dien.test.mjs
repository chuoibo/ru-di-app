/* Luồng ảnh đại diện: bắt tay, nối lại, tạm dừng khi app xuống nền.
 *
 * Socket và bộ hẹn giờ là đồ giả chạy tay, nên đo được đúng thứ tự: token gửi
 * trong khung đầu (trình duyệt không đặt được header cho WebSocket), "ready"
 * sau mỗi lần nối là lệnh đồng bộ lại, và nối lại có lùi dần chứ không dồn dập.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { baoDaDoiAnh, datNguonPhienBan, nguonAvatar } from "../dist-test/rudi/nguoi/anh-dai-dien.js";
import { choTruocKhiNoiLai, moLuongAnhDaiDien } from "../dist-test/rudi/nguoi/luong-anh-dai-dien.js";

const A = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const goi = [];
datNguonPhienBan((ids) => {
  goi.push({ ids });
  return Promise.resolve({});
}, (f) => f());

function dungCu(token = "tok-1") {
  const sockets = [];
  const hen = [];
  const luong = moLuongAnhDaiDien({
    url: "ws://may-chu/people/avatars/stream",
    actorId: "ffffffff-ffff-4fff-8fff-ffffffffffff",
    token: () => token,
    taoSocket: (url) => {
      const s = { url, readyState: 0, sent: [], closed: false, onopen: null, onmessage: null, onclose: null, onerror: null,
        send(d) { this.sent.push(JSON.parse(d)); }, close() { if (!this.closed) { this.closed = true; this.onclose?.(); } } };
      sockets.push(s);
      return s;
    },
    hen: (f, ms) => { const h = { f, ms }; hen.push(h); return h; },
    huyHen: (h) => { const i = hen.indexOf(h); if (i >= 0) hen.splice(i, 1); },
    ngauNhien: () => 0,
  });
  return { luong, sockets, hen };
}

test("mở là nối, bắt tay bằng token trong khung đầu; khung 'avatar' tới kho ngay", () => {
  const { sockets } = dungCu();
  assert.equal(sockets.length, 1);
  assert.equal(sockets[0].url, "ws://may-chu/people/avatars/stream");
  sockets[0].onopen();
  assert.deepEqual(sockets[0].sent, [{ type: "authenticate", token: "tok-1" }]);
  sockets[0].onmessage({ data: JSON.stringify({ type: "avatar", person_id: A, avatar_id: "v-day" }) });
  assert.match(nguonAvatar(A, "ffffffff-ffff-4fff-8fff-ffffffffffff").uri, /\?v=v-day$/);
});

test("mất kết nối thì nối lại có lùi dần; 'ready' đặt lại bộ đếm", () => {
  const { sockets, hen } = dungCu();
  sockets[0].close();
  assert.equal(hen.length, 1);
  assert.equal(hen[0].ms, choTruocKhiNoiLai(1, () => 0));
  hen.shift().f();
  assert.equal(sockets.length, 2);
  sockets[1].close();
  assert.equal(hen[0].ms, choTruocKhiNoiLai(2, () => 0));
  assert.ok(hen[0].ms > choTruocKhiNoiLai(1, () => 0), "lần hỏng thứ hai chờ lâu hơn");
  hen.shift().f();
  sockets[2].onmessage({ data: JSON.stringify({ type: "ready" }) });
  sockets[2].close();
  assert.equal(hen[0].ms, choTruocKhiNoiLai(1, () => 0), "đã nối được thì lần sau lại bắt đầu từ ngắn");
});

test("xuống nền: đóng và KHÔNG tự nối lại; lên lại: đồng bộ rồi nối", async () => {
  const { luong, sockets, hen } = dungCu();
  luong.tamDung();
  assert.equal(sockets[0].closed, true);
  assert.equal(hen.length, 0, "nền thì im, không hẹn nối lại");
  baoDaDoiAnh(A, "ffffffff-ffff-4fff-8fff-ffffffffffff", "v-truoc");
  await new Promise((r) => setImmediate(r));
  goi.length = 0;
  luong.tiepTuc();
  assert.equal(sockets.length, 2);
  assert.deepEqual(goi.map((g) => g.ids).flat(), [A], "lên lại là hỏi lại những người đã vẽ");
});

test("dừng hẳn (đăng xuất): huỷ cả hẹn nối lại, tiepTuc không mở lại", () => {
  const { luong, sockets, hen } = dungCu();
  sockets[0].close();
  assert.equal(hen.length, 1);
  luong.dung();
  assert.equal(hen.length, 0);
  luong.tiepTuc();
  assert.equal(sockets.length, 1);
});

test("không có token thì không mở socket; khung không phải JSON thì đóng để nối lại sạch", () => {
  const khong = dungCu(null);
  assert.equal(khong.sockets.length, 0);
  const { sockets } = dungCu();
  sockets[0].onmessage({ data: "không phải json" });
  assert.equal(sockets[0].closed, true);
});

test("thời gian chờ có trần 30 giây", () => {
  assert.equal(choTruocKhiNoiLai(50, () => 0), 30_000);
  assert.ok(choTruocKhiNoiLai(1, () => 0.999) < 500 + 250);
});
