import assert from "node:assert/strict";
import test from "node:test";

import { createRandomRound, decodeRandomRound, encodeRandomRound, randomWinner } from "../dist-test/rudi/chat/e2ee/random-v2.js";
import { SO_TRONG, apDung } from "../dist-test/rudi/chat/e2ee/so-tin.js";
import { dungPhong } from "../dist-test/rudi/chat/e2ee/so-phong.js";
import { sangTin } from "../dist-test/rudi/chat/e2ee/ve-v2.js";

const AN = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const BINH = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const ROUND = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const ROOM = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";
const PLACE = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee";
const MESSAGE = ROUND;
const ALIAS = "ffffffff-ffff-4fff-8fff-ffffffffffff";
const AT = "2026-10-13T07:00:00Z";
const MARKER = "\n/rudi-random-v1 ";
const words = (...values) => values.map((value) => value.toString(16).padStart(8, "0")).join("");
const SEED = words(5, 2, 3, 4);
const base = (patch = {}) => ({
  id: ROUND, request: "Tối nay đi đâu?", mode: "wheel",
  options: [{ label: "Cà phê", placeId: PLACE }, { label: "Cơm" }], seed: SEED, ...patch,
});
const round = (patch = {}) => createRandomRound(base(patch));
const wire = (patch = {}) => encodeRandomRound(round(patch));
const nhan = (actor, operation, logical = MESSAGE) => ({
  kind: "application", actor_id: actor, device_id: actor, logical_send_id: logical, operation,
});
const bodyFor = (snapshot) => `${snapshot.mode === "wheel" ? "Vòng quay" : "Đua vịt"} chọn: ${snapshot.options[snapshot.winner]?.label ?? "Cơm"}${MARKER}${JSON.stringify(snapshot)}`;

for (const mode of ["wheel", "ducks"]) {
  test(`${mode}: mã hoá hữu hạn giữ cùng lựa chọn, seed, tác giả và kết quả qua text MLS`, () => {
    const expected = round({ mode });
    const body = encodeRandomRound(expected);
    assert.equal(body.split("\n")[0], `${mode === "wheel" ? "Vòng quay" : "Đua vịt"} chọn: Cơm`);
    assert.deepEqual(decodeRandomRound(body), expected);
    assert.equal(encodeRandomRound(decodeRandomRound(body)), body);
    assert.equal(body.slice(body.indexOf(MARKER) + MARKER.length),
      `{"v":1,"id":"${ROUND}","request":"Tối nay đi đâu?","mode":"${mode}","options":[{"label":"Cà phê","placeId":"${PLACE}"},{"label":"Cơm"}],"seed":"${SEED}","winner":1}`);
    const so = apDung(SO_TRONG, nhan(AN, { type: "text", body }), 1, AT);
    assert.deepEqual(so.byId[MESSAGE].random, expected);
    const projected = sangTin(so.byId[MESSAGE], ROOM, BINH, {});
    assert.equal(projected.kind, "text");
    assert.equal(projected.author_id, AN);
    assert.equal(projected.card, null);
    assert.equal(so.byId[MESSAGE].ai, null);
    assert.deepEqual(projected.random_v2, expected);
  });
}

test("chuẩn hoá trim/NFC trước phát; snapshot sao chép lựa chọn và giữ ID địa điểm là ghi nguồn", () => {
  const input = base({ request: "  Cafe\u0301 tối nay  ", options: [{ label: "  Cafe\u0301 ", placeId: PLACE }, { label: " Cơm " }] });
  const result = createRandomRound(input);
  assert.equal(result.request, "Café tối nay");
  assert.deepEqual(result.options, [{ label: "Café", placeId: PLACE }, { label: "Cơm" }]);
  input.options[0].label = "Đổi sau khi tạo";
  assert.equal(result.options[0].label, "Café");
  assert.deepEqual(decodeRandomRound(encodeRandomRound(result)), result);
  assert.equal(round({ request: "" }).request, "");
});

test("rejection sampling tại biên 3/5/8 chọn word đầu hợp lệ theo big-endian", () => {
  for (const count of [3, 5]) {
    assert.equal(randomWinner(words(0xfffffffe, 0, 0, 0), count), count - 1);
    assert.equal(randomWinner(words(0xffffffff, 1, 2, 3), count), 1);
    assert.equal(randomWinner(words(0xffffffff, 0xffffffff, 2, 3), count), 2);
    assert.equal(randomWinner("ffffffffffffffffffffffff00000000", count), 0);
    assert.equal(randomWinner("ffffffffffffffffffffffffffffffff", count), null);
  }
  assert.equal(randomWinner("ffffffffffffffffffffffffffffffff", 8), 7);
  assert.equal(randomWinner(words(0xfffffffe, 0, 0, 0), 8), 6);
  assert.equal(randomWinner(words(0x100, 0, 0, 0), 3), 1);
  assert.equal(randomWinner(words(1, 0, 0, 0), 2), 1);
  for (const count of [0, 1, 9, 2.5, NaN, Infinity, "3"]) assert.equal(randomWinner(SEED, count), null);
  for (const seed of ["", "0".repeat(31), "0".repeat(33), "A".repeat(32), "g".repeat(32), 0, null]) {
    assert.equal(randomWinner(seed, 3), null);
  }
  assert.throws(() => round({ options: [{ label: "A" }, { label: "B" }, { label: "C" }], seed: "f".repeat(32) }));
});

test("lựa chọn phải có 2–8 nhãn khác nhau sau caseless/NFC, tối đa 80 ký tự, không điều khiển", () => {
  const invalid = [
    [], [{ label: "A" }], Array.from({ length: 9 }, (_, i) => ({ label: `Mục ${i}` })),
    [{ label: " A " }, { label: "a" }], [{ label: "Café" }, { label: "CAFE\u0301" }],
    [{ label: "Straße" }, { label: "STRASSE" }], [{ label: "Σ" }, { label: "ς" }],
    [{ label: "" }, { label: "B" }], [{ label: "   " }, { label: "B" }],
    [{ label: "a".repeat(81) }, { label: "B" }], [{ label: "A\nB" }, { label: "C" }],
    [{ label: "A\tB" }, { label: "C" }], [{ label: "A\u0000B" }, { label: "C" }],
    [{ label: "A\u202eB" }, { label: "C" }], [{ label: "A\u2028B" }, { label: "C" }],
    [{ label: 4 }, { label: "B" }], ["A", "B"],
    [{ label: "A", placeId: "not-an-id" }, { label: "B" }],
    [{ label: "A", placeId: PLACE.toUpperCase() }, { label: "B" }],
    [{ label: "A", placeId: null }, { label: "B" }],
    [{ label: "A", placeId: undefined }, { label: "B" }],
    [{ label: "A", source: "server" }, { label: "B" }],
  ];
  for (const choices of invalid) assert.throws(() => round({ options: choices }), JSON.stringify(choices));
  assert.equal(round({ options: [{ label: "a".repeat(80) }, { label: "B" }] }).options[0].label.length, 80);
  assert.equal(round({ options: Array.from({ length: 8 }, (_, i) => ({ label: `Mục ${i}` })) }).options.length, 8);
  assert.equal(round({ options: [{ label: "i" }, { label: "ı" }] }).options.length, 2);
});

test("schema, request, UUID, mode, seed và winner sai đều bị từ chối, kể cả khoá lạ", () => {
  const expected = round();
  for (const patch of [
    { v: 2 }, { id: "bad-id" }, { id: ROUND.toUpperCase() }, { mode: "coin" }, { mode: null },
    { request: 5 }, { request: "a".repeat(301) }, { request: "a\nb" }, { request: "a\u007fb" },
    { request: "  Không canonical " }, { request: "Cafe\u0301" },
    { seed: "0".repeat(31) }, { seed: "A".repeat(32) }, { seed: "g".repeat(32) },
    { winner: 0 }, { winner: "1" }, { winner: 1.5 }, { extra: true },
    { options: [{ label: " Cà phê ", placeId: PLACE }, { label: "Cơm" }] },
  ]) assert.equal(decodeRandomRound(bodyFor({ ...expected, ...patch })), null, JSON.stringify(patch));
  for (const key of Object.keys(expected)) {
    const missing = { ...expected };
    delete missing[key];
    assert.equal(decodeRandomRound(`Vòng quay chọn: Cơm${MARKER}${JSON.stringify(missing)}`), null, key);
  }
  for (const input of [null, [], { ...base(), extra: true }, { ...base(), mode: "coin" }, { ...base(), request: "a".repeat(301) }]) {
    assert.throws(() => createRandomRound(input));
  }
  assert.equal(round({ request: "a".repeat(300) }).request.length, 300);
  assert.throws(() => encodeRandomRound({ ...expected, winner: 0 }));
});

test("JSON canonical và dòng fallback phải khớp chính xác; giả kết quả hoặc seed không được promote", () => {
  const valid = wire();
  const json = valid.slice(valid.indexOf(MARKER) + MARKER.length);
  const invalid = [
    valid.replace("Vòng quay chọn: Cơm", "Vòng quay chọn: Cà phê"),
    valid.replace("Vòng quay", "Đua vịt"), "Trích dẫn: " + valid, " " + valid, valid + "\n",
    valid.replace(MARKER, "\r\n/rudi-random-v1 "), valid.replace("/rudi-random-v1 ", "/rudi-random-v2 "),
    valid.replace(MARKER, MARKER + " "), valid.replace('"v":1,', '"v":1,"v":1,'),
    valid.replace('"label":"Cơm"', '"label":"Cơm","label":"Cơm"'),
    valid.replace('"winner":1', '"winner":0'), valid.replace(SEED, words(4, 2, 3, 4)),
    `Vòng quay chọn: Cơm${MARKER}{"id":"${ROUND}",${json.slice(1).replace(`"v":1,"id":"${ROUND}",`, '"v":1,')}`,
    `Vòng quay chọn: Cơm${MARKER}${json.replace("Cơm", "C\\u01a1m")}`,
    `Vòng quay chọn: Cơm${MARKER}{`, `Vòng quay chọn: Cơm${MARKER}null`,
    `Vòng quay chọn: Cơm${MARKER}[]`, "Vòng quay chọn: Cơm", "",
  ];
  for (const body of invalid) {
    assert.equal(decodeRandomRound(body), null, body);
    const so = apDung(SO_TRONG, nhan(AN, { type: "text", body }), 1);
    assert.equal(so.byId[MESSAGE].random, undefined);
    assert.equal(so.byId[MESSAGE].body, body);
  }
});

test("body quá 6000 ký tự hoặc 12 KiB UTF-8 bị từ chối trước giải mã", () => {
  assert.equal(decodeRandomRound(wire() + " ".repeat(6001)), null);
  const multibyte = "界".repeat(4100);
  assert.ok(multibyte.length < 6000);
  assert.ok(Buffer.byteLength(multibyte, "utf8") > 12 * 1024);
  assert.equal(decodeRandomRound(multibyte), null);
  assert.equal(decodeRandomRound(null), null);
});

test("hai thành viên và replay sealed log khôi phục cùng snapshot; logical id không bị ghi đè", () => {
  const body = wire({ mode: "ducks" });
  const first = nhan(AN, { type: "text", body });
  const peerOne = apDung(SO_TRONG, first, 1, AT);
  const peerTwo = apDung(SO_TRONG, structuredClone(first), 1, AT);
  assert.deepEqual(peerOne, peerTwo);
  assert.deepEqual(sangTin(peerOne.byId[MESSAGE], ROOM, AN, {}).random_v2,
    sangTin(peerTwo.byId[MESSAGE], ROOM, BINH, {}).random_v2);
  assert.equal(apDung(peerOne, first, 2, AT), peerOne);
  const changedDuplicate = nhan(AN, { type: "text", body: wire({ seed: words(4, 2, 3, 4) }) });
  assert.equal(apDung(peerOne, changedDuplicate, 3), peerOne);
  const records = [{ t: "tin", seq: 1, r: first, at: AT }, { t: "tin", seq: 3, r: changedDuplicate, at: AT }];
  assert.deepEqual(dungPhong(records).so, peerOne);
  const persisted = JSON.parse(JSON.stringify(records));
  assert.deepEqual(dungPhong(persisted).so, peerOne);
});

test("reply MLS có thể giữ vòng random; caption/image/AI card không được đọc thành vòng", () => {
  const body = wire();
  const reply = apDung(SO_TRONG, nhan(AN, { type: "reply", reply_to: PLACE, body }), 1, AT);
  assert.deepEqual(reply.byId[MESSAGE].random, round());
  assert.equal(sangTin(reply.byId[MESSAGE], ROOM, AN, {}).reply_to.id, PLACE);
  const media = { media_id: PLACE, key: "key", nonce: "nonce", sha256: "hash", mime: "image/png", size: 12 };
  const image = apDung(SO_TRONG, nhan(AN, { type: "image", caption: body, width: 1, height: 1, media }), 1);
  assert.equal(image.byId[MESSAGE].random, undefined);
  const ai = apDung(SO_TRONG, nhan(AN, { type: "ai_card", invocation_id: ROUND, reply_to: PLACE, card: JSON.stringify({ body }) }), 1);
  assert.equal(ai.byId[MESSAGE].random, undefined);
  assert.equal(sangTin(ai.byId[MESSAGE], ROOM, AN, {}), null);
});

test("round ID phải trùng logical send ID xác thực; text/reply mang alias vẫn là chữ thường", () => {
  const body = wire();
  for (const operation of [{ type: "text", body }, { type: "reply", reply_to: PLACE, body }]) {
    const received = nhan(AN, operation, ALIAS);
    const direct = apDung(SO_TRONG, received, 1, AT);
    assert.equal(direct.byId[ALIAS].random, undefined);
    assert.equal(direct.byId[ALIAS].body, body);
    assert.equal(sangTin(direct.byId[ALIAS], ROOM, BINH, {}).random_v2, undefined);
    const reconstructed = dungPhong([{ t: "tin", seq: 1, r: received, at: AT }]).so;
    assert.deepEqual(reconstructed, direct);
  }
});

test("vòng đã phát không sửa được kể cả tác giả; edit text thường không tạo vòng mới", () => {
  const body = wire();
  const first = nhan(AN, { type: "text", body });
  const so = apDung(SO_TRONG, first, 1, AT);
  for (const actor of [AN, BINH]) {
    for (const replacement of ["đổi thành text", body, wire({ seed: words(4, 2, 3, 4) })]) {
      assert.equal(apDung(so, nhan(actor, { type: "edit", message_id: MESSAGE, body: replacement }, PLACE), 2), so);
    }
  }
  const ordinary = apDung(SO_TRONG, nhan(AN, { type: "text", body: "/random" }), 1, AT);
  const edited = apDung(ordinary, nhan(AN, { type: "edit", message_id: MESSAGE, body }, PLACE), 2, AT);
  assert.equal(edited.byId[MESSAGE].body, body);
  assert.equal(edited.byId[MESSAGE].edited, true);
  assert.equal(edited.byId[MESSAGE].random, undefined);
  assert.equal(sangTin(edited.byId[MESSAGE], ROOM, AN, {}).random_v2, undefined);
  const replay = dungPhong([{ t: "tin", seq: 1, r: first, at: AT }, {
    t: "tin", seq: 2, r: nhan(AN, { type: "edit", message_id: MESSAGE, body: "đổi" }, PLACE), at: AT,
  }]).so;
  assert.deepEqual(replay, so);
});

test("chỉ tác giả xoá; xoá bỏ snapshot và reaction, replay/edit không phục hồi vòng", () => {
  const first = nhan(AN, { type: "text", body: wire() });
  let so = apDung(SO_TRONG, first, 1, AT);
  so = apDung(so, nhan(BINH, { type: "reaction", message_id: MESSAGE, emoji: "❤️" }, PLACE), 2);
  assert.equal(apDung(so, nhan(BINH, { type: "delete", message_id: MESSAGE }, ROOM), 3), so);
  const deletion = nhan(AN, { type: "delete", message_id: MESSAGE }, ALIAS);
  const deleted = apDung(so, deletion, 4, AT);
  assert.equal(deleted.byId[MESSAGE].deleted, true);
  assert.equal(deleted.byId[MESSAGE].body, null);
  assert.equal(Object.hasOwn(deleted.byId[MESSAGE], "random"), false);
  assert.deepEqual({ ...deleted.byId[MESSAGE].reactions }, {});
  assert.equal(sangTin(deleted.byId[MESSAGE], ROOM, AN, {}).random_v2, undefined);
  assert.equal(apDung(deleted, first, 5), deleted);
  assert.equal(apDung(deleted, nhan(AN, { type: "edit", message_id: MESSAGE, body: wire() }, PLACE), 6), deleted);
  const reconstructed = dungPhong([{ t: "tin", seq: 1, r: first, at: AT }, { t: "tin", seq: 4, r: deletion, at: AT }]).so;
  assert.equal(reconstructed.byId[MESSAGE].random, undefined);
  assert.equal(reconstructed.byId[MESSAGE].deleted, true);
});

test("pending/lỗi không tự phát kết quả; ACK cùng bản nháp mới ghi snapshot một lần", () => {
  const first = nhan(AN, { type: "text", body: wire() });
  const records = [{ t: "cho", r: first, luc: 1 }];
  assert.equal(dungPhong(records).so.byId[MESSAGE], undefined);
  records.push({ t: "hong", id: MESSAGE, loi: "offline", thuLai: true });
  assert.equal(dungPhong(records).so.byId[MESSAGE], undefined);
  records.push({ t: "da-gui", id: MESSAGE, seq: 7, at: AT });
  const sent = dungPhong(records);
  assert.deepEqual(sent.so.byId[MESSAGE].random, round());
  assert.equal(sent.cho.length, 0);
  records.push({ t: "tin", seq: 7, r: first, at: AT });
  assert.deepEqual(dungPhong(records).so, sent.so);
});
