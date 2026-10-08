import assert from "node:assert/strict";
import test from "node:test";

import { SO_TRONG, apDung } from "../dist-test/rudi/chat/e2ee/so-tin.js";
import { chuDongBinhChon, docBinhChonV2, xemBinhChonV2 } from "../dist-test/rudi/chat/e2ee/binh-chon-v2.js";
import { sangTin } from "../dist-test/rudi/chat/e2ee/ve-v2.js";
import { dungPhong } from "../dist-test/rudi/chat/e2ee/so-phong.js";

const AN = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const BINH = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const POLL = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const ROOM = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";
const body = "/vote Tối nay đi đâu? Cafe | Cơm";
let next = 0;
const nhan = (actor, operation, id = `eeeeeeee-eeee-4eee-8eee-${String(++next).padStart(12, "0")}`) => ({
  kind: "application", actor_id: actor, device_id: actor, logical_send_id: id, operation,
});
const start = () => apDung(SO_TRONG, nhan(AN, { type: "text", body }, POLL), 1, "2026-10-08T07:00:00Z");
const view = (so, actor = AN) => xemBinhChonV2(so.byId[POLL], actor);

test("bình chọn từ text MLS giữ tác giả, không tự nhận ai_card hoặc tạo phiếu trên máy chủ", () => {
  const so = start();
  const t = sangTin(so.byId[POLL], ROOM, AN, {});
  assert.equal(t.kind, "text");
  assert.equal(t.author_id, AN);
  assert.equal(t.card, null);
  assert.equal(so.byId[POLL].ai, null);
  assert.equal(t.poll_v2.question, "Tối nay đi đâu");
  assert.equal(view(so).total_ballots, 0);
  assert.deepEqual(view(so).leading_option_ids, []);
  assert.equal(view(so).decided_option_id, null);
  assert.deepEqual(docBinhChonV2(body, POLL, 1).options, t.poll_v2.options);
});

test("mỗi actor một phiếu: đổi phiếu và replay không tăng tổng; target ngoài lựa chọn bị từ chối", () => {
  let so = start();
  const [cafe, com] = so.byId[POLL].poll.options;
  const ballot = nhan(BINH, { type: "vote", poll_id: POLL, option_id: cafe.id });
  so = apDung(so, ballot, 2);
  so = apDung(so, ballot, 3);
  assert.equal(view(so, BINH).total_ballots, 1);
  so = apDung(so, nhan(BINH, { type: "vote", poll_id: POLL, option_id: com.id }), 4);
  assert.deepEqual(view(so, BINH).options.map((o) => o.ballot_count), [0, 1]);
  assert.equal(view(so, BINH).my_option_id, com.id);
  assert.equal(view(so, AN).my_option_id, null);
  const bad = apDung(so, nhan(BINH, { type: "vote", poll_id: POLL, option_id: ROOM }), 5);
  assert.equal(bad, so);
  assert.equal(apDung(so, nhan(BINH, { type: "vote", poll_id: ROOM, option_id: com.id }), 6), so);
});

test("phiếu ngang nhau không chọn phía; chỉ tác giả đóng, sau đóng giữ số và từ chối phiếu mới", () => {
  let so = start();
  const [cafe, com] = so.byId[POLL].poll.options;
  so = apDung(so, nhan(AN, { type: "vote", poll_id: POLL, option_id: cafe.id }), 2);
  so = apDung(so, nhan(BINH, { type: "vote", poll_id: POLL, option_id: com.id }), 3);
  assert.equal(view(so).is_tie, true);
  assert.equal(view(so).decided_option_id, null);
  const close = { type: "edit", message_id: POLL, body: chuDongBinhChon(body) };
  assert.equal(apDung(so, nhan(BINH, close), 4), so);
  so = apDung(so, nhan(AN, close), 5, "2026-10-08T07:01:00Z");
  assert.equal(view(so).is_closed, true);
  assert.equal(view(so).closed_at, "2026-10-08T07:01:00Z");
  assert.equal(view(so).decided_option_id, null);
  assert.equal(apDung(so, nhan(BINH, { type: "vote", poll_id: POLL, option_id: cafe.id }), 6), so);
  assert.equal(apDung(so, nhan(AN, { type: "edit", message_id: POLL, body }), 7), so);
});

test("không sửa câu/lựa chọn đã phát; đóng một người dẫn phiếu giữ kết quả duy nhất", () => {
  let so = start();
  const cafe = so.byId[POLL].poll.options[0];
  so = apDung(so, nhan(BINH, { type: "vote", poll_id: POLL, option_id: cafe.id }), 2);
  assert.equal(apDung(so, nhan(AN, { type: "edit", message_id: POLL, body: "/vote Câu khác? Khác | Nữa" }), 3), so);
  so = apDung(so, nhan(AN, { type: "edit", message_id: POLL, body: chuDongBinhChon(body) }), 4);
  assert.equal(view(so).decided_option_id, cafe.id);
  assert.equal(view(so).total_ballots, 1);
});

test("xoá cần chính tác giả; bản xoá không giữ thẻ hay cho phiếu tới sau phục hồi nội dung", () => {
  const so = start();
  assert.equal(apDung(so, nhan(BINH, { type: "delete", message_id: POLL }), 2), so);
  const deleted = apDung(so, nhan(AN, { type: "delete", message_id: POLL }), 3);
  assert.equal(view(deleted), null);
  assert.equal(sangTin(deleted.byId[POLL], ROOM, AN, {}).poll_v2, undefined);
  assert.equal(apDung(deleted, nhan(BINH, { type: "vote", poll_id: POLL, option_id: so.byId[POLL].poll.options[0].id }), 4), deleted);
});

test("lệnh sai hoặc được trích dẫn vẫn là text, không suy ra bình chọn", () => {
  for (const invalid of ["/vote", "/vote Q? A", "/vote Q? A | a", "/vote Q? A |", "/vote Q? A\n| B", "Bình đã nhận: " + body, "/vote Q? " + "x".repeat(101) + " | B", "/vote Q? A | B | C | D | E | F | G"]) {
    assert.equal(docBinhChonV2(invalid, POLL, 1), null, invalid);
    const so = apDung(SO_TRONG, nhan(AN, { type: "text", body: invalid }, POLL), 1);
    assert.equal(so.byId[POLL].poll, undefined);
    assert.equal(so.byId[POLL].body, invalid);
  }
  assert.equal(docBinhChonV2(body, "not-an-id", 1), null);
});

test("dựng lại sổ phòng cho cùng phiếu/tác giả/đóng; không phụ thuộc random state hoặc HTTP", () => {
  const first = nhan(AN, { type: "text", body }, POLL);
  const option = docBinhChonV2(body, POLL, 1).options[1].id;
  const ballot = nhan(BINH, { type: "vote", poll_id: POLL, option_id: option });
  const close = nhan(AN, { type: "edit", message_id: POLL, body: chuDongBinhChon(body) });
  const ban = [first, ballot, close].map((r, i) => ({ t: "tin", seq: i + 1, r, at: "2026-10-08T07:00:00Z" }));
  const reconstructed = dungPhong(ban).so;
  let direct = SO_TRONG;
  for (let i = 0; i < ban.length; i++) direct = apDung(direct, ban[i].r, i + 1, ban[i].at);
  assert.deepEqual(reconstructed, direct);
  assert.equal(view(reconstructed, BINH).my_option_id, option);
});

test("phiếu đang gửi hoặc đã lỗi chưa tính vào kết quả; chỉ ghi nhận sau sequence thật", () => {
  const first = nhan(AN, { type: "text", body }, POLL);
  const option = docBinhChonV2(body, POLL, 1).options[0].id;
  const ballot = nhan(BINH, { type: "vote", poll_id: POLL, option_id: option });
  const ban = [{ t: "tin", seq: 1, r: first }, { t: "cho", r: ballot, luc: 1 }];
  assert.equal(view(dungPhong(ban).so).total_ballots, 0);
  ban.push({ t: "hong", id: ballot.logical_send_id, loi: "offline", thuLai: true });
  assert.equal(view(dungPhong(ban).so).total_ballots, 0);
  ban.push({ t: "da-gui", id: ballot.logical_send_id, seq: 2 });
  assert.equal(view(dungPhong(ban).so, BINH).total_ballots, 1);
});
