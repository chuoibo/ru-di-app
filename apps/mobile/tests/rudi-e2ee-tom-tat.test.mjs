/* Chat v2 rooms in the conversation list (src/rudi/chat/e2ee/tom-tat.ts,
 * ADR-0057 §8.4): the lane's summary replaces a v2 room's frozen legacy last
 * message and its uncleared legacy unread count, and the rows go newest first.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { createRandomRound, encodeRandomRound } from "../dist-test/rudi/chat/e2ee/random-v2.js";
import { dungPhong } from "../dist-test/rudi/chat/e2ee/so-phong.js";
import { CHU_MA_HOA, gopTomTatV2, xemTruocTinV2 } from "../dist-test/rudi/chat/e2ee/tom-tat.js";
import { xemTruocTinCuoi } from "../dist-test/rudi/chat/tin-song.js";

const TOI = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const BAN = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const ROUND = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const ALIAS = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";
const AT = "2026-10-14T08:00:00Z";
const received = (operation, logical = ROUND) => ({
  kind: "application", actor_id: TOI, device_id: TOI, logical_send_id: logical, operation,
});
const replay = (records) => dungPhong(JSON.parse(JSON.stringify(records))).so;
const roundBody = (mode) => encodeRandomRound(createRandomRound({
  id: ROUND, request: "Ăn gì?", mode, options: [{ label: "Phở" }, { label: "Bún" }],
  seed: "00000001a0000002b0000003c0000004",
}));
const media = { media_id: ALIAS, key: "key", nonce: "nonce", sha256: "hash", mime: "image/png", size: 12 };
const hang = (id, created_at, unread = 0, extra = {}) => ({
  id, display_name: id, my_state: "active", membership_id: id, member_count: 2, unread_count: unread,
  last_message: created_at === null ? null : { id: "m" + id, kind: "text", preview: "cũ " + id, author_id: BAN, author_display_name: "Bạn A", created_at },
  ...extra,
});

test("phòng v2: tin cuối và chưa đọc từ làn, chữ từ máy này, xếp lại theo tin mới nhất", () => {
  const nhom = [
    hang("legacy-moi", "2026-10-06T08:00:00Z", 2),
    hang("v2", "2026-10-01T08:00:00Z", 5, { kind: "pair", counterpart: { id: BAN, display_name: "Bình" } }),
    hang("trong", null),
  ];
  const rooms = [{ conversation_id: "v2", last_sequence: 9, last_at: "2026-10-06T09:00:00Z", last_actor_id: BAN, unread: 1, read_sequence: 7 }];
  const ra = gopTomTatV2(nhom, rooms, { v2: "Tối nay 7 giờ nhé" });
  assert.deepEqual(ra.map((n) => n.id), ["v2", "legacy-moi", "trong"]);
  assert.equal(ra[0].unread_count, 1, "the legacy unread count of a v2 room can never clear: the lane's replaces it");
  assert.equal(ra[0].last_message.preview, "Tối nay 7 giờ nhé");
  assert.equal(ra[0].last_message.author_display_name, "Bình");
  assert.equal(ra[1].unread_count, 2, "a legacy room keeps its own count");
});

test("không có chữ trên máy này thì nói tin đã mã hoá; phòng v2 chưa có tin giữ tin cũ", () => {
  const nhom = [hang("v2", "2026-10-01T08:00:00Z", 3), hang("v2-rong", "2026-09-01T08:00:00Z", 4)];
  const rooms = [
    { conversation_id: "v2", last_sequence: 2, last_at: "2026-10-02T08:00:00Z", last_actor_id: TOI, unread: 0, read_sequence: 2 },
    { conversation_id: "v2-rong", last_sequence: 0, last_at: null, last_actor_id: null, unread: 0, read_sequence: 0 },
  ];
  const ra = gopTomTatV2(nhom, rooms, {});
  assert.equal(ra[0].last_message.preview, CHU_MA_HOA);
  assert.equal(ra[0].last_message.author_id, TOI);
  assert.equal(ra[1].last_message.preview, "cũ v2-rong");
  assert.equal(ra[1].unread_count, 0);
});

for (const mode of ["wheel", "ducks"]) {
  test(`${mode}: inbox dùng metadata từ sổ phòng, không lộ snapshot kỹ thuật qua preview`, () => {
    const body = roundBody(mode);
    const expected = `${mode === "wheel" ? "Vòng quay" : "Đua vịt"} chọn: Bún`;
    for (const operation of [{ type: "text", body }, { type: "reply", reply_to: ALIAS, body }]) {
      const records = [{ t: "tin", seq: 7, r: received(operation), at: AT }];
      const so = replay(records);
      const tin = so.byId[ROUND];
      const unchanged = replay(records).byId[ROUND];
      assert.ok(tin.random, "the local replay accepted the canonical round and matching logical ID");
      assert.ok(tin.body.includes("\n/rudi-random-v1 "));
      const preview = xemTruocTinV2(tin);
      assert.equal(preview, expected);
      assert.equal(preview.includes("/rudi-random-v1"), false);
      const [row] = gopTomTatV2([hang("v2", "2026-10-01T08:00:00Z")], [
        { conversation_id: "v2", last_sequence: 7, last_at: AT, last_actor_id: TOI, unread: 0, read_sequence: 7 },
      ], { v2: preview });
      assert.equal(xemTruocTinCuoi(row.last_message), expected, "the inbox formatter receives only the readable result");
      assert.equal(row.last_message.author_id, TOI);
      assert.deepEqual(tin, unchanged, "previewing never changes the sealed message body, identity or result");
    }
    const ownAck = replay([
      { t: "cho", r: received({ type: "text", body }), luc: 1 },
      { t: "da-gui", id: ROUND, seq: 7, at: AT },
    ]).byId[ROUND];
    assert.equal(xemTruocTinV2(ownAck), expected, "this phone's acknowledged outbox send uses the same preview");
    assert.equal(ownAck.body, body);
  });
}

test("preview không nâng raw body, alias, snapshot hỏng hoặc edit text thành lượt random", () => {
  const body = roundBody("wheel");
  const cases = [
    { operation: { type: "text", body }, logical: ALIAS },
    { operation: { type: "reply", reply_to: ROUND, body }, logical: ALIAS },
    { operation: { type: "text", body: body.replace('"winner":1', '"winner":0') }, logical: ROUND },
    { operation: { type: "text", body: `Trích dẫn: ${body}` }, logical: ROUND },
  ];
  for (const { operation, logical } of cases) {
    const tin = replay([{ t: "tin", seq: 1, r: received(operation, logical), at: AT }]).byId[logical];
    assert.equal(tin.random, undefined);
    assert.equal(xemTruocTinV2(tin), operation.body);
  }
  const edited = replay([
    { t: "tin", seq: 1, r: received({ type: "text", body: "Tin thường" }), at: AT },
    { t: "tin", seq: 2, r: received({ type: "edit", message_id: ROUND, body }, ALIAS), at: AT },
  ]).byId[ROUND];
  assert.equal(edited.random, undefined);
  assert.equal(xemTruocTinV2(edited), body);
});

test("preview giữ chính sách chữ/media cũ và ưu tiên xoá lượt đã nhận", () => {
  const operations = [
    [{ type: "text", body: "Hẹn tối nay" }, "Hẹn tối nay"],
    [{ type: "sticker", pack_id: "pack", sticker_id: "sticker" }, "Đã gửi một sticker"],
    [{ type: "image", caption: roundBody("wheel"), width: 1, height: 1, media }, "Đã gửi một ảnh"],
    [{ type: "voice", media: { ...media, mime: "audio/mp4" }, duration_ms: 1000 }, CHU_MA_HOA],
    [{ type: "ai_card", invocation_id: ALIAS, reply_to: ROUND, card: "{}" }, "Rủ Đi AI đã trả lời"],
  ];
  for (const [operation, expected] of operations) {
    const tin = replay([{ t: "tin", seq: 1, r: received(operation), at: AT }]).byId[ROUND];
    assert.equal(xemTruocTinV2(tin), expected);
  }
  const deleted = replay([
    { t: "tin", seq: 1, r: received({ type: "text", body: roundBody("ducks") }), at: AT },
    { t: "tin", seq: 2, r: received({ type: "delete", message_id: ROUND }, ALIAS), at: AT },
  ]).byId[ROUND];
  assert.equal(deleted.deleted, true);
  assert.equal(deleted.random, undefined);
  assert.equal(xemTruocTinV2(deleted), "Tin nhắn đã bị xoá");
});
