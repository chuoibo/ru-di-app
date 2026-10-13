import assert from "node:assert/strict";
import test from "node:test";
import { newNativeRandomRound, RandomSourceUnavailable } from "../dist-test/rudi/chat/e2ee/random-native.js";

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
