import assert from "node:assert/strict";
import test from "node:test";
import { apDungAnhChup, gopTin, thayPhanUng, tinChoHoiThoai } from "../dist-test/rudi/chat/tin-song.js";
import { docTrangThayDoi, gopBinhChon } from "../dist-test/rudi/chat/thay-doi.js";
import { goiAi, gopAiInvocations, taoKeoTuChat } from "../dist-test/rudi/chat/ai-invocations.js";
import { datTokenPhien } from "../dist-test/danh-tinh.js";

const context = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const actor = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";
const message = (id, revision, extra = {}) => ({ id, revision, context_id: context, author_id: actor, kind: "text", body: "Lời rủ", image_url: null, card: null, cursor: id, created_at: "2030-09-21T12:00:00Z", ...extra });

test("An older snapshot cannot resurrect a deletion or erase a newer reaction", () => {
  const held = [message("a", 7, { kind: "deleted", body: null }), message("b", 9, { reactions: [{ kind: "heart", count: 2, mine: true }] })];
  const next = gopTin(held, [message("a", 6), message("b", 8, { reactions: [] }), message("quote", 2, { reply_to: { id: "a", kind: "text", author_id: actor, preview: "Old text" } })]);
  assert.equal(next.find((m) => m.id === "a").body, null);
  assert.equal(next.find((m) => m.id === "b").reactions[0].count, 2);
  assert.equal(next.find((m) => m.id === "quote").reply_to.kind, "deleted");
  assert.equal(gopTin(held, [message("a", undefined)])[1].kind, "deleted");
});

test("Change pages reject sequence gaps, wrong rooms and ambiguous cursor advancement", () => {
  const page = { context_id: context, changes: [{ sequence: 5, type: "message", entity_id: "a", revision: 5 }], next_sequence: 5, watermark: 8, has_more: true };
  assert.ok(docTrangThayDoi(page, context, 4));
  assert.equal(docTrangThayDoi(page, context, 3), null);
  assert.equal(docTrangThayDoi(page, "other", 4), null);
  assert.equal(docTrangThayDoi({ ...page, next_sequence: 8 }, context, 4), null);
  assert.ok(docTrangThayDoi({ ...page, changes: [], next_sequence: 4, has_more: false }, context, 4));
});

test("An old reaction does not skip unread history pages, but its tombstone still sanitizes quotes", () => {
  const held = [message("new", 4, { reply_to: { id: "old", kind: "text", preview: "Private old text", author_id: actor } })];
  const ancient = message("old", 8, { created_at: "2029-09-21T12:00:00Z", reactions: [{ kind: "heart", count: 1, mine: false }] });
  assert.deepEqual(apDungAnhChup(held, [ancient]).map((item) => item.id), ["new"]);
  const next = apDungAnhChup(held, [{ ...ancient, kind: "deleted", body: null }]);
  assert.equal(next.length, 1);
  assert.equal(next[0].reply_to.kind, "deleted");
  assert.equal(apDungAnhChup(held, [message("z-fresh", 9)])[0].id, "z-fresh");
});

test("Votes and invocation errors keep the most recent server version", () => {
  const newest = { id: "poll", revision: 10, total_ballots: 3 };
  assert.equal(gopBinhChon({ poll: newest }, [{ ...newest, revision: 9, total_ballots: 2 }]).poll.total_ballots, 3);
  const failed = { id: "job", status: "failed", created_at: "2030-09-21T00:00:00Z", updated_at: "2030-09-21T00:02:00Z" };
  assert.equal(gopAiInvocations([failed], [{ ...failed, status: "running", updated_at: "2030-09-21T00:01:00Z" }])[0].status, "failed");
});

test("A delayed own reaction ACK cannot overwrite a newer peer snapshot", () => {
  const held = [message("m", 12, { reactions: [{ kind: "heart", count: 2, mine: true }] })];
  assert.equal(thayPhanUng(held, "m", [{ kind: "heart", count: 1, mine: true }])[0].reactions[0].count, 2);
  assert.equal(thayPhanUng([message("m", undefined)], "m", [{ kind: "heart", count: 1, mine: true }])[0].reactions[0].count, 1);
});

test("Only a completed poll hides its raw slash command; malformed and failed calls remain", () => {
  const command = message("command", 1, { body: "/vote Ăn gì? Phở | Bún" });
  const poll = message("poll", 2, { kind: "ai_card", card: { kind: "poll", payload: { vote_id: "p", question: "Ăn gì?", options: [{ id: "a", label: "Phở" }, { id: "b", label: "Bún" }] } } });
  assert.deepEqual(tinChoHoiThoai([command, poll]).map((m) => m.id), ["poll"]);
  assert.equal(tinChoHoiThoai([command]).length, 1);
  assert.equal(tinChoHoiThoai([message("bad", 0, { body: "/vote bad input" }), poll]).length, 2);
});

test("An AI invocation shares only its explicit prompt; plan promotion carries no chat history", async () => {
  const original = globalThis.fetch;
  const calls = [];
  datTokenPhien("synthetic-test-token");
  globalThis.fetch = async (url, options) => { calls.push({ url, ...options }); return { ok: true, status: 201, json: async () => ({ id: "job" }), text: async () => "{}" }; };
  try {
    await goiAi(context, actor, "Lời nhờ được đồng ý", actor);
    assert.deepEqual(JSON.parse(calls[0].body), { logical_id: actor, command: "plan", prompt: "Lời nhờ được đồng ý" });
    assert.equal(calls[0].headers.Authorization, "Bearer synthetic-test-token");
    await taoKeoTuChat(context, actor, "source", { title: "Hẹn cuối tuần", starts_on: "2030-09-21", ends_on: "2030-09-21", headcount: 4, budget_per_person_vnd: 200000 });
    assert.equal(JSON.parse(calls[1].body).source_message_id, "source");
    assert.equal(Object.hasOwn(JSON.parse(calls[1].body), "messages"), false);
  } finally { globalThis.fetch = original; datTokenPhien(null); }
});
