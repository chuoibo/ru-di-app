/* A story expires (L4, ADR-0022 §2.3) -- measured against the disposable
 * stack of the e2e slice, with the deadline moved by hand in its database.
 *
 * There is no dev-only route that rewinds time, on purpose. The slice hands
 * this file the stack's own database URL (`MOBILE_E2E_DATABASE_URL`, set by
 * `scripts/e2e_slice.sh` beside the sessions file), and the one write outside
 * HTTP moves the story's two timestamps back a day (`created_at` to 25 hours
 * ago, `expires_at` to an hour ago -- both, because the CHECK
 * `expires_at > created_at` refuses a deadline before the writing) on the
 * story this test just created -- the same move the operator would make with
 * psql. The update runs through python3 + SQLAlchemy because node has no
 * Postgres driver in this tree and adding one for a single test is not worth
 * a dependency.
 *
 * What it proves, end to end through the client's own wire functions: a friend
 * sees the story on `GET /stories` and reads its photo; once the deadline is
 * past, both `GET /stories` (theirs and the author's own rail) drop it and the
 * photo answers 404 -- without anything having been deleted.
 *
 * Needs a live server, same convention as the slice: `MOBILE_REQUIRE_E2E=1`
 * turns a missing server or a missing database URL into a failure.
 */
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { deflateSync } from "node:zlib";
import test from "node:test";

import { BASE_URL, newAttempt } from "../../dist-test/api.js";
import { personById } from "../../dist-test/rudi/nhom-demo.js";
import { docStories, dangStory, nhomCua } from "../../dist-test/rudi/story/story.js";
import { guiLoiMoi, traLoiLoiMoi } from "../../dist-test/screens/ca-nhan/ban-be.js";
import { banDoPhien, batPhienE2E, fetchTho } from "./phien-e2e.mjs";

const MINH = personById("minh")?.personId;
const TRANG = personById("trang")?.personId;
const NGUOI_DANG_NHAP = batPhienE2E(MINH);
const REQUIRED = Boolean(process.env.MOBILE_REQUIRE_E2E);
const DB_URL = process.env.MOBILE_E2E_DATABASE_URL ?? "";

async function serverIsUp() {
  try {
    return (await fetch(`${BASE_URL}/healthz`)).ok;
  } catch {
    return false;
  }
}

async function skipUnlessMeasurable(t) {
  if (!(await serverIsUp())) {
    if (REQUIRED) assert.fail(`MOBILE_REQUIRE_E2E đặt rồi nhưng không có server tại ${BASE_URL}.`);
    t.skip(`không có server tại ${BASE_URL}`);
    return true;
  }
  if (NGUOI_DANG_NHAP === null || DB_URL === "") {
    if (REQUIRED) assert.fail("MOBILE_REQUIRE_E2E đặt rồi nhưng thiếu MOBILE_E2E_SESSIONS hoặc MOBILE_E2E_DATABASE_URL.");
    t.skip("thiếu phiên e2e hoặc URL cơ sở dữ liệu của stack");
    return true;
  }
  return false;
}

/** A 32×32 solid PNG built here, so no image bytes live in the repo. */
function pngNho() {
  const w = 32;
  const h = 32;
  const raw = Buffer.alloc((1 + w * 3) * h);
  for (let y = 0; y < h; y += 1) {
    raw[y * (1 + w * 3)] = 0;
    for (let x = 0; x < w; x += 1) raw.set([0x1d, 0x21, 0x40], y * (1 + w * 3) + 1 + x * 3);
  }
  const crcTable = [];
  for (let n = 0; n < 256; n += 1) {
    let c = n;
    for (let k = 0; k < 8; k += 1) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    crcTable.push(c >>> 0);
  }
  const crc = (buf) => {
    let c = 0xffffffff;
    for (const b of buf) c = crcTable[(c ^ b) & 0xff] ^ (c >>> 8);
    return (c ^ 0xffffffff) >>> 0;
  };
  const khoi = (ten, than) => {
    const len = Buffer.alloc(4);
    len.writeUInt32BE(than.length);
    const body = Buffer.concat([Buffer.from(ten, "ascii"), than]);
    const sum = Buffer.alloc(4);
    sum.writeUInt32BE(crc(body));
    return Buffer.concat([len, body, sum]);
  };
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(w, 0);
  ihdr.writeUInt32BE(h, 4);
  ihdr.set([8, 2, 0, 0, 0], 8);
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    khoi("IHDR", ihdr),
    khoi("IDAT", deflateSync(raw)),
    khoi("IEND", Buffer.alloc(0)),
  ]);
}

async function taiAnhCaNhan(personId) {
  const form = new FormData();
  form.append("file", new Blob([pngNho()], { type: "image/png" }), "anh.png");
  const response = await fetchTho(`${BASE_URL}/people/me/photos`, {
    method: "POST",
    headers: { Authorization: `Bearer ${banDoPhien()[personId]}` },
    body: form,
  });
  // One read of the body: the assertion message must not consume what the
  // JSON parse needs a line later.
  const than = await response.text();
  assert.equal(response.status, 201, than);
  return JSON.parse(than).url;
}

async function docAnh(url, personId) {
  const response = await fetchTho(`${BASE_URL}${url}`, {
    headers: { Authorization: `Bearer ${banDoPhien()[personId]}` },
  });
  return response.status;
}

/** Friends, idempotently: a request that already exists is not a failure here. */
async function ketBan(a, b) {
  let loiMoi;
  try {
    loiMoi = await guiLoiMoi(b, a, newAttempt());
  } catch (error) {
    if (!/đã|already|request_not_open|đang/i.test(String(error?.message ?? error))) throw error;
    return;
  }
  if (loiMoi.state === "accepted") return;
  await traLoiLoiMoi(loiMoi.id, "accept", b, newAttempt());
}

function tuaQuaHan(storyId) {
  execFileSync(
    "python3",
    [
      "-c",
      [
        "import os, sys, uuid",
        "from sqlalchemy import create_engine, text",
        "engine = create_engine(os.environ['MOBILE_DATABASE_URL'])",
        "with engine.begin() as conn:",
        "    n = conn.execute(text(\"UPDATE stories SET created_at = now() - interval '25 hours', expires_at = now() - interval '1 hour' WHERE id = :id\"), {'id': uuid.UUID(sys.argv[1])}).rowcount",
        "assert n == 1, n",
      ].join("\n"),
      storyId,
    ],
    { env: { ...process.env, MOBILE_DATABASE_URL: DB_URL }, cwd: new URL("../../../../services/api", import.meta.url).pathname, stdio: "pipe" },
  );
}

test("story hết hạn rời GET /stories của cả hai và ảnh đóng lại, không xoá gì", async (t) => {
  if (await skipUnlessMeasurable(t)) return;
  assert.ok(MINH && TRANG, "hai người demo phải có id");
  await ketBan(MINH, TRANG);

  const url = await taiAnhCaNhan(MINH);
  assert.equal(await docAnh(url, TRANG), 404, "chưa có story hay bài trỏ tới: bạn cũng chưa mở được ảnh");

  const story = await dangStory(url, "Story e2e", MINH, newAttempt());
  assert.equal(story.author_id, MINH);
  assert.equal(Date.parse(story.expires_at) - Date.parse(story.created_at), 24 * 3600 * 1000);

  const cuaTrang = nhomCua(await docStories(TRANG), MINH);
  assert.ok(cuaTrang, "bạn thấy nhóm story của tác giả");
  assert.ok(cuaTrang.stories.some((s) => s.id === story.id));
  assert.equal(await docAnh(url, TRANG), 200, "story còn hạn mở ảnh cho bạn");
  assert.ok(nhomCua(await docStories(MINH), MINH), "tác giả thấy story của mình trên dải");

  tuaQuaHan(story.id);

  assert.equal(nhomCua(await docStories(TRANG), MINH), null, "qua hạn: bạn không còn thấy");
  assert.equal(nhomCua(await docStories(MINH), MINH), null, "qua hạn: dải của chính mình cũng không còn");
  assert.equal(await docAnh(url, TRANG), 404, "qua hạn: ảnh đóng lại với bạn");
  assert.equal(await docAnh(url, MINH), 200, "chủ ảnh luôn mở được");
});
