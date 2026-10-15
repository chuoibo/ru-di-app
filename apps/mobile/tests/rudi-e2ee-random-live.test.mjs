import assert from "node:assert/strict";
import test from "node:test";

import { createRandomRound, decodeRandomRound, encodeRandomRound } from "../dist-test/rudi/chat/e2ee/random-v2.js";
import { createRandomLiveRound, decodeRandomLiveRound, encodeRandomLiveRound, randomClockSample, randomLivePreview,
  randomLiveView, randomServerNow } from "../dist-test/rudi/chat/e2ee/random-live.js";
import { dungPhong } from "../dist-test/rudi/chat/e2ee/so-phong.js";
import { sangCho, sangTin } from "../dist-test/rudi/chat/e2ee/ve-v2.js";
import { xemTruocTinV2 } from "../dist-test/rudi/chat/e2ee/tom-tat.js";
import { trichTu } from "../dist-test/rudi/chat/tin-song.js";
import { gomBoiCanhChat } from "../dist-test/rudi/chat/boi-canh-chat.js";

const AUTHOR = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const ID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const OTHER = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const ROOM = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";
const AT = "2026-10-14T10:00:00Z";
const BASE = Date.parse(AT);
const input = (mode = "ducks") => ({ id: ID, request: "Ăn gì?", mode, options: [{ label: "Phở" }, { label: "Bún", placeId: "q-source" }],
  seed: "00000001a0000002b0000003c0000004" });
const round = (mode) => createRandomLiveRound(createRandomRound(input(mode)));
const received = (operation, logical = ID, actor = AUTHOR) => ({ kind: "application", actor_id: actor, device_id: actor, logical_send_id: logical, operation });
const replay = (records) => dungPhong(JSON.parse(JSON.stringify(records)));
const record = (r, seq = 7) => ({ t: "tin", seq, r, at: AT });
const clock = (at = BASE, mono = 0, uncertaintyMs = 0) => ({ serverMs: at, monotonicMs: mono, uncertaintyMs });

test("v2 có grammar riêng, canonical đóng schema và fallback không lộ người thắng; v1 giữ nguyên", () => {
  for (const mode of ["wheel", "ducks"]) {
    const v1 = createRandomRound(input(mode));
    const live = createRandomLiveRound(v1);
    const body = encodeRandomLiveRound(live);
    assert.deepEqual(decodeRandomLiveRound(body), live);
    assert.equal(decodeRandomRound(body), null);
    assert.equal(decodeRandomLiveRound(encodeRandomRound(v1)), null);
    assert.deepEqual(decodeRandomRound(encodeRandomRound(v1)), v1);
    assert.equal(body.split("\n")[0], randomLivePreview(live));
    assert.equal(body.split("\n")[0].includes("Bún"), false);
    for (const bad of [body + " ", "Trích dẫn: " + body, body.replace('"v":2', '"v":1'),
      body.replace('"timing":"shared-v1"', '"timing":"future"'), body.replace('"winner":1', '"winner":0'),
      body.replace('"v":2,', '"v":2,"v":2,'), body.replace('"request":"Ăn gì?"', '"request":"  Ăn gì?"'),
      body.replace('"timing":"shared-v1"', '"timing":"shared-v1","extra":true'), body.replace("lượt đã khóa", "chọn: Bún")]) {
      assert.equal(decodeRandomLiveRound(bad), null);
    }
    assert.throws(() => encodeRandomLiveRound({ ...live, v: 1 }));
    assert.throws(() => encodeRandomLiveRound({ ...live, extra: true }));
    assert.equal(decodeRandomLiveRound(body + "x".repeat(6000)), null);
  }
});

test("text/reply replay chỉ nhận cùng logical ID, giữ timestamp ACK, không sửa/nhân đôi lượt", () => {
  const live = round();
  const body = encodeRandomLiveRound(live);
  for (const operation of [{ type: "text", body }, { type: "reply", reply_to: OTHER, body }]) {
    const records = [record(received(operation))];
    const accepted = replay(records).so.byId[ID];
    assert.deepEqual(accepted.randomLive, live);
    assert.equal(accepted.at, AT);
    const projected = sangTin(accepted, ROOM, AUTHOR, {});
    assert.deepEqual(projected.random_live_v2, live);
    assert.equal(projected.created_at, AT);
    assert.equal(projected.author_id, AUTHOR);
    assert.equal(projected.card, null);
    assert.equal(projected.random_v2, undefined);
    records.push(record(received({ type: "edit", message_id: ID, body: "Thay kết quả" }, OTHER), 8));
    records.push(record(received(operation), 9));
    assert.deepEqual(replay(records).so.byId[ID], accepted);
    const alias = replay([record(received(operation, OTHER))]).so.byId[OTHER];
    assert.equal(alias.randomLive, undefined);
    assert.equal(alias.body, body);
  }
  const pending = [{ t: "cho", r: received({ type: "text", body }), luc: 1 }];
  assert.equal(replay(pending).so.byId[ID], undefined);
  pending.push({ t: "da-gui", id: ID, seq: 7, at: AT });
  const acked = replay(pending).so.byId[ID];
  pending.push({ t: "da-gui", id: ID, seq: 8, at: "2026-10-14T10:01:00Z" });
  assert.deepEqual(replay(pending).so.byId[ID], acked, "a replayed ACK never resets the scheduled start");
  const edited = replay([record(received({ type: "text", body: "Tin thường" })),
    record(received({ type: "edit", message_id: ID, body }, OTHER), 8)]).so.byId[ID];
  assert.equal(edited.randomLive, undefined);
  const deleted = replay([record(received({ type: "text", body })),
    record(received({ type: "delete", message_id: ID }, OTHER), 8)]).so.byId[ID];
  assert.equal(deleted.randomLive, undefined);
  assert.equal(deleted.deleted, true);
});

test("timestamp ACK RFC3339 giữ nguyên qua sổ/reopen và offset cùng instant có cùng lịch", () => {
  const live = round();
  const body = encodeRandomLiveRound(live);
  const fraction = ".458684";
  const spellings = [`2026-10-14T10:00:00${fraction}Z`, `2026-10-14T17:00:00${fraction}+07:00`,
    `2026-10-14T05:30:00${fraction}-04:30`, `2026-10-13T23:00:00${fraction}-11:00`,
    `2026-10-14T10:00:00${fraction}+00:00`];
  const anchor = clock(BASE + 458, 100, 20);
  const moments = [100, 5100, 12_100, 18_100, 18_120];
  const expected = moments.map((mono) => randomLiveView(live, spellings[0], anchor, mono));
  assert.deepEqual(expected.map((view) => view.phase), ["countdown", "countdown", "racing", "racing", "finished"]);
  assert.equal(expected[2].progress, 0.5);
  assert.equal(expected[3].reveal, false);
  assert.equal(expected[4].reveal, true);
  for (const at of spellings) {
    const records = [{ t: "cho", r: received({ type: "text", body }), luc: 1 }];
    assert.equal(replay(records).so.byId[ID], undefined);
    records.push({ t: "da-gui", id: ID, seq: 7, at });
    const accepted = replay(records).so.byId[ID];
    const projected = sangTin(accepted, ROOM, AUTHOR, {});
    assert.equal(accepted.at, at);
    assert.equal(accepted.body, body);
    assert.equal(projected.created_at, at, "projection does not rewrite the sealed timestamp");
    assert.deepEqual(projected.random_live_v2, live);
    assert.deepEqual(moments.map((mono) => randomLiveView(projected.random_live_v2, projected.created_at, anchor, mono)), expected);
    records.push({ t: "da-gui", id: ID, seq: 8, at: "2026-10-14T11:00:00Z" });
    const reopened = replay(records).so.byId[ID];
    assert.deepEqual(reopened, accepted, "reopen and duplicate ACK retain the first immutable event/time");
    const reopenedProjection = sangTin(reopened, ROOM, AUTHOR, {});
    assert.deepEqual(moments.map((mono) => randomLiveView(reopenedProjection.random_live_v2, reopenedProjection.created_at, anchor, mono)), expected);
    if (!at.endsWith("Z")) assert.equal(randomClockSample(at, 100, 120), null, "the clock endpoint contract remains UTC Z only");
  }
  for (const at of ["2026-02-30T17:00:00+07:00", "2026-02-29T05:30:00-04:30", "2026-10-14T24:00:00Z",
    "2026-10-14T17:00:60+07:00", "2026-10-14T17:00:00+24:00", "2026-10-14T17:00:00+07:60",
    "2026-10-14T17:00:00+7:00", "2026-10-14T17:00:00+0700", "2026-10-14T10:00:00-00:00",
    "2026-10-14T10:00:00", "2026-10-14T10:00:00." + "1".repeat(10) + "Z", "2026-10-15T17:00:00+07:00"]) {
    const view = randomLiveView(live, at, anchor, 100);
    assert.equal(view.phase, "unavailable", at);
    assert.equal(view.reveal, false, at);
  }
  for (const at of spellings) {
    assert.equal(randomLiveView(live, at, anchor, 30_101).phase, "unavailable", "an offset cannot bypass clock expiry");
  }
});

test("inbox/quote không dùng raw wire hoặc lộ nhãn thắng khi không có clock", () => {
  const live = round();
  const tin = replay([record(received({ type: "text", body: encodeRandomLiveRound(live) }))]).so.byId[ID];
  const projected = sangTin(tin, ROOM, AUTHOR, {});
  assert.equal(projected.body, "Đua vịt: lượt đã khóa");
  assert.equal(xemTruocTinV2(tin), "Đua vịt: lượt đã khóa");
  assert.equal(trichTu(projected, () => "Bạn").preview, "Đua vịt: lượt đã khóa");
  const pendingReply = { id: OTHER, r: received({ type: "reply", reply_to: ID, body: "Xem nhé" }, OTHER), luc: 1, hong: false, loi: null, thuLai: true };
  assert.equal(sangCho(pendingReply, { [ID]: tin }).traLoi.preview, "Đua vịt: lượt đã khóa");
  const context = gomBoiCanhChat({ tin: [projected], personId: AUTHOR });
  assert.equal(context.luot[0].chu, "Đua vịt: lượt đã khóa");
  assert.equal(JSON.stringify(context).includes(live.seed), false);
  assert.equal(JSON.stringify(context).includes("winner"), false);
  assert.ok(tin.body.includes("/rudi-random-live-v2"), "stored body remains immutable");
});

test("clock chỉ chấp nhận DB UTC có RTT/bất định hữu hạn, không dùng giờ wall trên máy", () => {
  assert.deepEqual(randomClockSample(AT, 100, 120), { serverMs: BASE, monotonicMs: 110, uncertaintyMs: 11 });
  for (const args of [[AT, 100, 99], [AT, NaN, 120], [AT, 0, 500], ["bad", 0, 1],
    ["2026-02-30T10:00:00Z", 0, 1], ["2026-10-14T10:00:00+00:00", 0, 1]]) assert.equal(randomClockSample(...args), null);
  const anchor = clock(BASE, 100, 20);
  assert.equal(randomServerNow(anchor, 2100), BASE + 2000);
  assert.equal(randomServerNow(anchor, 99), null);
  assert.equal(randomServerNow(anchor, 30_101), null);
  assert.equal(randomServerNow({ ...anchor, uncertaintyMs: 251 }, 200), null);
  const original = Date.now;
  try {
    Date.now = () => BASE + 9_000_000;
    assert.equal(randomServerNow(anchor, 2100), BASE + 2000);
  } finally { Date.now = original; }
});

test("hai đồng hồ monotonic khác nhau theo cùng lịch, latejoin/resume seek và reveal bảo thủ chung", () => {
  const live = round();
  const a = clock(BASE, 100, 20), b = clock(BASE + 10_000, 900, 20);
  const middleA = randomLiveView(live, AT, a, 12_100);
  const middleB = randomLiveView(live, AT, b, 2900);
  assert.deepEqual(middleA, middleB);
  assert.equal(middleA.phase, "racing");
  assert.equal(middleA.progress, 0.5);
  assert.equal(middleA.reveal, false);
  assert.equal(randomLiveView(live, AT, a, 5100).remainingMs, 1000);
  assert.equal(randomLiveView(live, AT, a, 18_100).reveal, false, "uncertainty cannot reveal early");
  assert.equal(randomLiveView(live, AT, a, 18_120).reveal, true);
  assert.equal(randomLiveView(live, AT, clock(BASE + 20_000, 50), 50).phase, "finished");
  for (const [at, anchor, mono] of [[null, a, 100], ["bad", a, 100], ["2026-10-15T10:00:00Z", a, 100],
    [AT, null, 100], [AT, a, 31_000]]) {
    assert.equal(randomLiveView(live, at, anchor, mono).phase, "unavailable");
    assert.equal(randomLiveView(live, at, anchor, mono).reveal, false);
  }
  assert.equal(randomLiveView(round("wheel"), AT, clock(BASE + 12_000), 0).reveal, true);
});
