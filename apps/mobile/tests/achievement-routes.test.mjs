import assert from "node:assert/strict";
import test from "node:test";

import { datTokenPhien } from "../dist-test/api.js";
import { chonKet, docHanhTrinh, goiYNep, trungBayHuyHieu } from "../dist-test/rudi/ky-niem/achievement-routes.js";

const ACTOR = "11111111-1111-4111-8111-111111111111";

test("journey client uses server progress and explicit one-call AI consent", async () => {
  datTokenPhien("synthetic-token");
  const calls = [];
  globalThis.fetch = async (url, init) => {
    calls.push({ path: String(url), method: init.method, body: init.body ? JSON.parse(init.body) : null, auth: init.headers.Authorization });
    if (String(url).endsWith("/achievement-routes")) return Response.json({ routes: [], active_run: null, earned_badges: [], candidates: [], mp4_credits: { granted: 0, used: 0, available: 0 } });
    if (String(url).endsWith("/achievement-suggestions")) return Response.json({ candidate_ids: ["open_map"], source: "go", line: "Một ngã rẽ mới đang mở." });
    if (String(url).endsWith("/achievement-runs")) return Response.json({ id: "run-1", route_id: "dau_chan", ending_id: "open_map" }, { status: 201 });
    return Response.json({ badge_ids: ["first_photo"] });
  };
  const snapshot = await docHanhTrinh(ACTOR);
  assert.equal(snapshot.mp4_credits.available, 0);
  await assert.rejects(() => goiYNep(ACTOR, false), /đồng ý/i);
  assert.equal(calls.length, 1, "no request leaves before consent");
  const offered = await goiYNep(ACTOR, true);
  assert.deepEqual(offered.candidate_ids, ["open_map"]);
  assert.equal(offered.line, "Một ngã rẽ mới đang mở.");
  await chonKet(ACTOR, "dau_chan", "open_map");
  await trungBayHuyHieu(ACTOR, ["first_photo"]);
  assert.deepEqual(calls.map((c) => [c.method, c.body]), [
    ["GET", null], ["POST", { consent: true }],
    ["POST", { route_id: "dau_chan", ending_id: "open_map" }],
    ["PATCH", { badge_ids: ["first_photo"] }],
  ]);
  assert.ok(calls.every((c) => c.auth === "Bearer synthetic-token"));
  datTokenPhien(null);
});
