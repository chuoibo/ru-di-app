/* Chat v2 rooms in the conversation list (src/rudi/chat/e2ee/tom-tat.ts,
 * ADR-0057 §8.4): the lane's summary replaces a v2 room's frozen legacy last
 * message and its uncleared legacy unread count, and the rows go newest first.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { CHU_MA_HOA, gopTomTatV2 } from "../dist-test/rudi/chat/e2ee/tom-tat.js";

const TOI = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const BAN = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
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
