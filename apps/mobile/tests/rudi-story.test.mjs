/* 24-hour stories (L4, ADR-0022 §2.3), on the wire and in pure copy.
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node --test tests/rudi-story.test.mjs
 *
 * What matters: the four routes are the four paths with the bearer, and a
 * write carries an attempt key; the body carries `caption` only when there is
 * one; the ring's label says whose and whether seen, and never falls back to
 * an id; «still live» is strict at the deadline like the server; the viewer
 * opens on the first unseen story; the clock clamps.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { BASE_URL, datTokenPhien, newAttempt } from "../dist-test/api.js";
import {
  LOI_STORY,
  THOI_LUONG_MS,
  cauTuoi,
  chiSoBatDau,
  conHan,
  dangStory,
  danhDauDaXem,
  docStories,
  laCuaToi,
  nhanVong,
  nhomCua,
  thanDangStory,
  tienDo,
  xoaStory,
} from "../dist-test/rudi/story/story.js";

const ME = "4dd00000-dddd-4ddd-8ddd-0000d0000011";
const BAN = "5ee00000-eeee-4eee-8eee-0000e0000011";
const STORY = "6ff00000-ffff-4fff-8fff-0000f0000011";
const ANH = `/people/${ME}/photos/7aa00000-aaaa-4aaa-8aaa-0000a0000011`;

function traLoi(body, status = 200) {
  if (body === null) return new Response(null, { status });
  return new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } });
}

async function batFetch(traVe, chay) {
  const daGoi = [];
  const truoc = globalThis.fetch;
  globalThis.fetch = async (url, init) => {
    daGoi.push({ url: String(url), init });
    return traVe(daGoi.length);
  };
  try {
    await chay();
  } finally {
    globalThis.fetch = truoc;
  }
  return daGoi;
}

test.afterEach(() => datTokenPhien(null));

const storyWire = (thay = {}) => ({
  id: STORY,
  author_id: BAN,
  author_display_name: "Bạn QA",
  image_url: ANH,
  caption: "Story QA",
  audience: "friends",
  created_at: "2030-08-27T12:00:00Z",
  expires_at: "2030-08-28T12:00:00Z",
  seen: false,
  ...thay,
});

test("bốn đường: GET /stories, POST /stories (có attempt), POST seen, DELETE — đều mang bearer", async () => {
  datTokenPhien("tok-story");
  const goi = await batFetch(
    (n) => (n === 2 ? traLoi(storyWire(), 201) : n === 4 ? traLoi(null, 204) : traLoi(n === 1 ? { authors: [] } : { story_id: STORY, seen_at: "2030-08-27T13:00:00Z" })),
    async () => {
      const dai = await docStories(ME);
      assert.deepEqual(dai, { authors: [] });
      const moi = await dangStory(ANH, "  Story QA ", ME, newAttempt());
      assert.equal(moi.id, STORY);
      const xem = await danhDauDaXem(STORY, ME);
      assert.equal(xem.story_id, STORY);
      await xoaStory(STORY, ME);
    },
  );
  assert.deepEqual(
    goi.map((g) => [g.init.method, g.url.slice(BASE_URL.length)]),
    [
      ["GET", "/stories"],
      ["POST", "/stories"],
      ["POST", `/stories/${STORY}/seen`],
      ["DELETE", `/stories/${STORY}`],
    ],
  );
  for (const g of goi) assert.equal(g.init.headers.Authorization, "Bearer tok-story");
  assert.deepEqual(JSON.parse(goi[1].init.body), { image_url: ANH, caption: "Story QA" }, "chú thích được cắt khoảng trắng");
  assert.ok(goi[1].init.headers["Idempotency-Key"], "đăng story là một lần thử có khoá");
});

test("thân đăng story chỉ mang caption khi có chữ", () => {
  assert.deepEqual(thanDangStory(ANH, ""), { image_url: ANH });
  assert.deepEqual(thanDangStory(ANH, "   "), { image_url: ANH });
  assert.deepEqual(thanDangStory(ANH, " đi chơi "), { image_url: ANH, caption: "đi chơi" });
});

test("nhãn vòng: của bạn / của <tên>, chưa xem / đã xem — không bao giờ là id", () => {
  const nhom = { author: { id: BAN, display_name: "Bạn QA" }, all_seen: false, stories: [] };
  assert.equal(nhanVong(nhom, ME), "Story của Bạn QA, chưa xem");
  assert.equal(nhanVong({ ...nhom, all_seen: true }, ME), "Story của Bạn QA, đã xem");
  assert.equal(nhanVong({ ...nhom, author: { id: ME, display_name: "Tôi" } }, ME), "Story của bạn");
  assert.equal(laCuaToi(nhom, ME), false);
  assert.ok(!nhanVong(nhom, ME).includes(BAN));
});

test("còn hạn là NGHIÊM NGẶT ở đúng mốc, như máy chủ", () => {
  const han = Date.parse("2030-08-28T12:00:00Z");
  assert.equal(conHan("2030-08-28T12:00:00Z", han - 1), true);
  assert.equal(conHan("2030-08-28T12:00:00Z", han), false);
  assert.equal(conHan("2030-08-28T12:00:00Z", han + 1), false);
  assert.equal(conHan("không phải ngày", han), false, "chuỗi hỏng đọc là hết hạn, không phải còn mãi");
});

test("viewer mở ở story chưa xem đầu tiên; đã xem hết thì từ đầu", () => {
  const nhom = { author: { id: BAN, display_name: "Bạn QA" }, all_seen: false, stories: [storyWire({ id: "a", seen: true }), storyWire({ id: "b" }), storyWire({ id: "c" })] };
  assert.equal(chiSoBatDau(nhom), 1);
  assert.equal(chiSoBatDau({ stories: nhom.stories.map((s) => ({ ...s, seen: true })) }), 0);
  assert.equal(chiSoBatDau({ stories: [] }), 0);
  const dai = { authors: [nhom] };
  assert.equal(nhomCua(dai, BAN), nhom);
  assert.equal(nhomCua(dai, ME), null);
});

test("đồng hồ 5 giây kẹp trong 0..1", () => {
  assert.equal(THOI_LUONG_MS, 5000);
  assert.equal(tienDo(1000, 1000), 0);
  assert.equal(tienDo(1000, 3500), 0.5);
  assert.equal(tienDo(1000, 9000), 1);
  assert.equal(tienDo(1000, 500), 0, "đồng hồ máy lùi không ra số âm");
});

test("câu tuổi của story", () => {
  const luc = Date.parse("2030-08-27T12:00:00Z");
  assert.equal(cauTuoi("2030-08-27T12:00:00Z", luc + 30_000), "vừa xong");
  assert.equal(cauTuoi("2030-08-27T12:00:00Z", luc + 5 * 60_000), "5 phút trước");
  assert.equal(cauTuoi("2030-08-27T12:00:00Z", luc + 3 * 3_600_000), "3 giờ trước");
  assert.equal(cauTuoi("hỏng", luc), "");
});

test("câu lỗi không dùng gạch dài và có câu cho mọi mã máy chủ trả", () => {
  for (const [ma, cau] of Object.entries(LOI_STORY)) {
    assert.ok(!cau.includes("—"), ma);
    assert.ok(cau.length > 0, ma);
  }
  for (const ma of ["story_not_found", "photo_not_found", "permission_denied", "caption_too_long", "photo_url_invalid"]) {
    assert.ok(ma in LOI_STORY, ma);
  }
});
