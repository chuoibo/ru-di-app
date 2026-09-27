// Exercise the production HTTP front door with disposable synthetic accounts.
// Fixture order: author, accepted friend, stranger, independent moderator.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { randomUUID } from "node:crypto";
import { setTimeout as delay } from "node:timers/promises";

const [base, fixturePath, replica = base] = process.argv.slice(2);
for (const value of [base, replica]) {
  const u = new URL(value);
  assert(["localhost", "127.0.0.1"].includes(u.hostname), "Synthetic loopback deployment only");
}
const { actors } = JSON.parse(await readFile(fixturePath, "utf8"));
assert(actors.length >= 4 && actors.every((a) => a.token.startsWith("synthetic-")), "Synthetic tokens required");
let requests = 0;
async function call(who, method, path, body, expected = 200, host = base, mime = "application/json") {
  const response = await fetch(host + path, {
    method, headers: { Authorization: `Bearer ${actors[who].token}`, "Content-Type": mime },
    body: body === undefined ? undefined : Buffer.isBuffer(body) ? body : JSON.stringify(body),
    signal: AbortSignal.timeout(10000),
  });
  requests++;
  assert.equal(response.status, expected, `${method} ${path}: unexpected HTTP status`);
  if (response.status === 204) return null;
  if (response.headers.get("Content-Type")?.includes("application/json")) return response.json();
  return Buffer.from(await response.arrayBuffer());
}
function stream(who, post) {
  const events = [];
  const socket = new WebSocket(replica.replace(/^http/, "ws") + "/v2/community/stream");
  socket.addEventListener("open", () => socket.send(JSON.stringify({ token: actors[who].token, posts: [post] })));
  socket.addEventListener("message", (e) => events.push(JSON.parse(e.data)));
  return { events, socket };
}
async function until(predicate, message) {
  const deadline = Date.now() + 10000;
  while (Date.now() < deadline) {
    if (predicate()) return;
    await delay(50);
  }
  assert.fail(message);
}
const sockets = [], created = [];
try {
  const input = { logical_id: randomUUID(), body: "Synthetic E2E: đi bộ cùng bạn bè", audience: "friends", topics: ["đi bộ"], mentions: [actors[1].id] };
  const p = await call(0, "POST", "/v2/community/posts", input, 201);
  created.push(p.id);
  const path = `/v2/community/posts/${p.id}`;
  assert.equal((await call(0, "POST", "/v2/community/posts", input)).id, p.id);
  await call(1, "GET", path);
  await call(2, "GET", path, undefined, 404);
  await call(2, "PUT", "/v2/community/follows", { kind: "person", target: actors[0].id }, 204);
  await call(2, "GET", path, undefined, 404);
  const peer = stream(1, p.id), stranger = stream(2, p.id);
  sockets.push(peer.socket, stranger.socket);
  await until(() => peer.events.some((e) => e.kind === "sync") && stranger.events.some((e) => e.kind === "sync"), "Initial stream sync missing");
  const comment = { logical_id: randomUUID(), body: "Synthetic E2E: hẹn chuyến tiếp theo", mentions: [actors[0].id] };
  const c = await call(1, "POST", path + "/comments", comment, 201);
  assert.equal((await call(1, "POST", path + "/comments", comment)).id, c.id);
  await until(() => peer.events.some((e) => e.kind === "post.changed" && e.post_id === p.id), "Peer did not receive cross-replica event");
  assert(!stranger.events.some((e) => e.post_id === p.id), "Private post ID leaked to stranger stream");
  const one = await call(1, "PUT", path + "/like");
  const retry = await call(1, "PUT", path + "/like");
  assert.equal(one.likes, 1); assert.equal(retry.likes, 1);
  assert.equal((await call(0, "GET", path + "/comments")).comments.filter((x) => x.id === c.id).length, 1);
  await call(0, "PATCH", path + "/audience", { audience: "only_me", revision: p.revision });
  await call(1, "GET", path, undefined, 404);
  await call(1, "GET", path + "/comments", undefined, 404);
  const png = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGNomLAAAANEAbF61zYUAAAAAElFTkSuQmCC", "base64");
  const media = await call(0, "POST", "/v2/community/media", png, 201, base, "image/png");
  await call(2, "GET", `/v2/community/media/${media.id}`, undefined, 404);
  const pub = await call(0, "POST", "/v2/community/posts", { ...input, logical_id: randomUUID(), audience: "public", media_ids: [media.id] }, 201);
  created.push(pub.id);
  const publicPath = `/v2/community/posts/${pub.id}`;
  await call(2, "GET", publicPath, undefined, 404);
  const beforePublication = stranger.events.length;
  await call(3, "POST", publicPath + "/review", { revision: pub.revision, approve: true, reason: "Synthetic E2E: nội dung chuyến đi đã được xem xét" });
  await until(() => stranger.events.slice(beforePublication).some((e) => e.kind === "feed.changed" && !e.post_id), "Newly published post did not invalidate another reader's feed");
  assert.equal((await call(2, "GET", publicPath)).media[0].id, media.id);
  assert((await call(2, "GET", `/v2/community/media/${media.id}`)).length > 0);
  await delay(1100);
  assert((await call(2, "GET", "/v2/community/feed", undefined, 200, replica)).posts.some((x) => x.id === pub.id));
  const publicReader = stream(2, pub.id);
  sockets.push(publicReader.socket);
  await until(() => publicReader.events.some((e) => e.kind === "sync"), "Public reader did not connect");
  const publicCursor = publicReader.events.find((e) => e.kind === "sync").cursor;
  const pending = await call(1, "POST", publicPath + "/comments", { ...comment, logical_id: randomUUID() }, 201);
  assert(!(await call(2, "GET", publicPath + "/comments")).comments.some((x) => x.id === pending.id));
  await call(3, "POST", `/v2/community/comments/${pending.id}/review`, { approve: true, reason: "Synthetic E2E: bình luận phù hợp" });
  await until(() => publicReader.events.some((e) => e.kind === "post.changed" && e.post_id === pub.id && e.cursor > publicCursor), "Approved public comment did not reach the other replica's reader");
  assert((await call(2, "GET", publicPath + "/comments")).comments.some((x) => x.id === pending.id));
  await call(2, "PUT", "/v2/community/preferences", { personalized: true });
  await call(2, "POST", "/v2/community/interactions", { id: randomUUID(), post_id: pub.id, kind: "view", dwell_ms: 5000 }, 204);
  await call(2, "DELETE", "/v2/community/history", undefined, 204);
  await call(2, "PUT", "/v2/community/preferences", { personalized: false });
  await call(0, "PATCH", publicPath + "/audience", { audience: "only_me", revision: pub.revision });
  await call(2, "GET", `/v2/community/media/${media.id}`, undefined, 404);
  assert(!(await call(2, "GET", "/v2/community/feed")).posts.some((x) => x.id === pub.id));
  console.log(JSON.stringify({ result: "PASS", requests, real_http: true, real_websocket: true, distinct_replica: base !== replica, publication_realtime: true, comment_realtime: true, ai_model_quality: "not measured" }));
} finally {
  sockets.forEach((socket) => socket.close());
  for (const id of created) await call(0, "DELETE", `/v2/community/posts/${id}`, undefined, 204);
}
