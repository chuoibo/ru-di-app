import assert from "node:assert/strict";
import test from "node:test";
import { newNativeRandomRound, RandomSourceUnavailable } from "../dist-test/rudi/chat/e2ee/random-native.js";
import { duongDua, khungDuongDua, MOC_DUONG_DUA, viTriVatDua } from "../dist-test/rudi/chat/e2ee/duong-dua.js";

const options = [{ label: "Món A" }, { label: "Món B" }, { label: "Món C" }];
const id = (word) => `${word}-cafe-4abc-8abc-abcdefabcdef`;
function native(t, generator) {
  const old = Object.getOwnPropertyDescriptor(globalThis, "expo");
  Object.defineProperty(globalThis, "expo", { configurable: true, value: generator ? { uuidv4: generator } : undefined });
  t.after(() => { if (old) Object.defineProperty(globalThis, "expo", old); else delete globalThis.expo; });
}

test("native entropy unavailable or malformed fails closed without Math.random", (t) => {
  native(t, undefined);
  assert.throws(() => newNativeRandomRound("", "wheel", options), RandomSourceUnavailable);
  Object.defineProperty(globalThis, "expo", { configurable: true, value: { uuidv4: () => "not-a-native-uuid" } });
  assert.throws(() => newNativeRandomRound("", "ducks", options), RandomSourceUnavailable);
});
test("four independent native prefixes create a frozen 128-bit seed and one round id", (t) => {
  const words = ["abcdefab", "00000002", "abcdefcd", "feabcdef", "fedcbaab"];
  let calls = 0;
  native(t, () => id(words[calls++]));
  const round = newNativeRandomRound("Ăn tối", "ducks", options);
  assert.equal(calls, 5);
  assert.equal(round.id, id(words[0]));
  assert.equal(round.seed, words.slice(1).join(""));
  assert.equal(round.winner, 2);
});
test("a wholly rejected seed is replaced before freezing; it never falls back to biased modulo", (t) => {
  const words = ["abcdefab", ...Array(4).fill("ffffffff"), "00000001", "abcdefcd", "feabcdef", "fedcbaab"];
  let calls = 0;
  native(t, () => id(words[calls++]));
  const round = newNativeRandomRound("", "wheel", options);
  assert.equal(calls, 9);
  assert.equal(round.id, id(words[0]));
  assert.equal(round.seed, words.slice(5).join(""));
  assert.equal(round.winner, 1);
});
test("invalid choices are refused before reading any draw entropy", (t) => {
  let calls = 0;
  native(t, () => { calls++; return id("abcdefab"); });
  assert.throws(() => newNativeRandomRound("", "wheel", [{ label: "A" }, { label: "A" }]));
  assert.equal(calls, 1, "only the logical id was minted; no draw was sampled");
});

test("race paths replay exactly, move only forward and finish with the frozen winner", () => {
  const seed = "abcdefab00000002abcdefcdfeabcdef";
  for (let count = 2; count <= 8; count++) for (let winner = 0; winner < count; winner++) {
    const paths = Array.from({ length: count }, (_, runner) => duongDua(seed, runner, winner));
    paths.forEach((path, runner) => {
      assert.deepEqual(path, duongDua(seed, runner, winner));
      assert.equal(path.length, MOC_DUONG_DUA.length);
      assert.equal(path[0], 0);
      path.forEach((x, i) => { assert.ok(Number.isFinite(x) && x >= 0 && x <= 1); if (i) assert.ok(x > path[i - 1]); });
      assert.equal(path.at(-1) === 1, runner === winner);
    });
    assert.ok(paths.every((path, runner) => runner === winner || path.at(-1) < paths[winner].at(-1)));
  }
});

test("race pace permits overtakes before the final surge; it is not a fixed rank progress bar", () => {
  const paths = Array.from({ length: 4 }, (_, i) => duongDua("abcdefab00000002abcdefcdfeabcdef", i, 2));
  let overtakes = 0;
  for (let a = 0; a < 4; a++) for (let b = a + 1; b < 4; b++) {
    for (let step = 2; step < 19; step++) if ((paths[a][step] - paths[b][step]) * (paths[a][step - 1] - paths[b][step - 1]) < 0) overtakes++;
  }
  assert.ok(overtakes >= 2, `expected repeated overtakes, saw ${overtakes}`);
});

test("eight runners keep a compact continuous arena and the winner beak reaches the finish", () => {
  for (const width of [200, 288, 358, 560]) for (let count = 2; count <= 8; count++) for (const compact of [false, true]) {
    const arena = khungDuongDua(width, count, compact);
    const tops = viTriVatDua(arena, count, compact);
    assert.equal(arena.width, width);
    assert.ok(arena.height <= 306);
    assert.ok(12 + arena.travel + arena.duck <= width);
    assert.ok(Math.abs(12 + arena.travel + arena.duck * 0.95 - arena.finish) < 0.001);
    assert.equal(tops.length, count);
    assert.ok(tops[0] >= 12);
    assert.ok(tops.at(-1) + arena.duck <= arena.height - 12);
    for (let i = 1; i < tops.length; i++) {
      assert.ok(tops[i] - tops[i - 1] >= arena.duck, `lanes ${i - 1} and ${i} must not overlap for count=${count} compact=${compact}`);
    }
  }
});
