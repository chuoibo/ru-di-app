import assert from "node:assert/strict";
import test from "node:test";

import { datTokenPhien } from "../dist-test/api.js";
import { createProfileVideo, profileVideoFileURL, profileVideoStatus, savedProfileVideos, videoAttempt, videoCredits } from "../dist-test/rudi/nep/profile-video.js";

const actor = "11111111-1111-4111-8111-111111111111";

test("earned video client keeps one credit request and discovers saved private films", async () => {
  const oldFetch = globalThis.fetch;
  datTokenPhien("synthetic-session");
  const calls = [];
  globalThis.fetch = async (url, init) => {
    const path = new URL(String(url)).pathname;
    calls.push({ path, method: init.method, key: init.headers["Idempotency-Key"], bearer: init.headers.Authorization, actor: init.headers["X-Actor-ID"], body: init.body ? JSON.parse(init.body) : null });
    if (path.endsWith("/credits")) return Response.json({ granted: 2, used: 1, available: 1 });
    if (path.endsWith("/file")) throw new Error("file must be loaded by authenticated player");
    if (init.method === "POST") return Response.json({ job_id: "movie-1", kind: "nep_video", status: "queued" }, { status: 202 });
    if (path.endsWith("/movie-1")) return Response.json({ job_id: "movie-1", kind: "nep_video", status: "ready" });
    return Response.json({ jobs: [{ job_id: "movie-1", kind: "nep_video", status: "ready", created_at: "2026-09-23T00:00:00Z" }] });
  };
  try {
    assert.equal((await videoCredits(actor)).available, 1);
    const saved = await savedProfileVideos(actor);
    assert.equal(saved[0].job_id, "movie-1");
    const attempt = videoAttempt();
    await createProfileVideo(actor, ["image-1"], attempt);
    await profileVideoStatus(actor, "movie-1");
    const post = calls.find((call) => call.method === "POST");
    assert.equal(post.key, attempt.key);
    assert.equal(post.body.idempotency_key, attempt.key);
    assert.deepEqual(post.body.image_job_ids, ["image-1"]);
    assert.ok(calls.every((call) => call.bearer === "Bearer synthetic-session" && call.actor === actor));
    assert.match(profileVideoFileURL("a/b"), /\/me\/profile-videos\/a%2Fb\/file$/);
  } finally {
    globalThis.fetch = oldFetch;
    datTokenPhien(null);
  }
});
