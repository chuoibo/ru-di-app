/* One post and what sits under it (L3, ADR-0022 §2.2), on the wire and in pure copy.
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node --test tests/rudi-bai-chi-tiet.test.mjs
 *
 * What matters: `coTheBinhLuan` reads the server's flag and nothing else (a
 * relation the client could compute is exactly what must not decide it); the
 * six routes are the six paths with the bearer and an attempt key on every
 * write; a personal photo's frame source carries one BASE_URL and this
 * reader's headers, a group photo's carries the group; an absolute url is
 * never a source; the wall policy PATCH sends only that key.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { BASE_URL, datTokenPhien, newAttempt } from "../dist-test/api.js";
import { duongDanAnhNguoi, laAnhNguoi, nguonAnhBai, nhomCuaAnh } from "../dist-test/rudi/nguoi/anh-ca-nhan.js";
import { CHINH_SACH, datChinhSachBinhLuan, laChinhSach, nhanChinhSach } from "../dist-test/rudi/nguoi/chinh-sach-tuong.js";
import {
  CAU_KHONG_BINH_LUAN,
  LOI_BAI,
  apPhanUng,
  boBinhLuan,
  boPhanUngBai,
  cauTuongTacBai,
  coTheBinhLuan,
  coTheXoaBinhLuan,
  daPhanUng,
  demLoai,
  docBinhLuanBai,
  ghepBinhLuan,
  guiBinhLuanBai,
  themPhanUngBai,
  tongPhanUng,
  xoaBinhLuanBai,
} from "../dist-test/rudi/tuong/bai-chi-tiet.js";

const ME = "4dd00000-dddd-4ddd-8ddd-0000d0000009";
const BAN = "5ee00000-eeee-4eee-8eee-0000e0000009";
const POST = "6ff00000-ffff-4fff-8fff-0000f0000009";
const CTX = "1aa00000-aaaa-4aaa-8aaa-0000a0000009";
const PHOTO = "7aa00000-aaaa-4aaa-8aaa-0000a0000010";

function traLoi(body, status = 200) {
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

test("coTheBinhLuan đọc đúng cờ của máy chủ, không suy từ quan hệ hay mức người đọc", () => {
  assert.equal(coTheBinhLuan({ can_comment: true }), true);
  assert.equal(coTheBinhLuan({ can_comment: false }), false);
  assert.equal(coTheBinhLuan({}), false, "máy chủ cũ không gửi cờ: đóng");
  assert.equal(coTheBinhLuan({ can_comment: "yes" }), false);
});

test("đếm và câu tương tác đọc từ danh sách máy chủ trả, thiếu thì là 0", () => {
  const bai = { reactions: [{ kind: "heart", count: 2 }, { kind: "fire", count: 1 }], my_reactions: ["fire"], comment_count: 3 };
  assert.equal(demLoai(bai, "heart"), 2);
  assert.equal(demLoai(bai, "sad"), 0);
  assert.equal(tongPhanUng(bai), 3);
  assert.equal(daPhanUng(bai, "fire"), true);
  assert.equal(daPhanUng(bai, "heart"), false);
  assert.equal(cauTuongTacBai(bai), "2 tim · 3 bình luận");
  assert.equal(cauTuongTacBai({}), "0 tim · 0 bình luận");
  const sau = apPhanUng(bai, { post_id: POST, reactions: [{ kind: "heart", count: 3 }], my_reactions: ["heart"] });
  assert.equal(demLoai(sau, "heart"), 3);
  assert.equal(sau.comment_count, 3, "phản ứng không chạm số bình luận");
});

test("xoá bình luận: tác giả bình luận hoặc chủ bài, không ai khác", () => {
  const bai = { author_id: ME };
  assert.equal(coTheXoaBinhLuan({ author_id: BAN }, bai, ME), true, "chủ bài quét tường mình");
  assert.equal(coTheXoaBinhLuan({ author_id: BAN }, bai, BAN), true);
  assert.equal(coTheXoaBinhLuan({ author_id: BAN }, { author_id: BAN }, ME), false);
});

test("ghép trang bình luận không lặp id; bỏ bình luận theo id", () => {
  const a = { id: "1", post_id: POST, author_id: ME, author_display_name: "Tôi", body: "a", created_at: "2030-01-01T00:00:00Z" };
  const b = { ...a, id: "2", body: "b" };
  assert.deepEqual(ghepBinhLuan([a], [a, b]).map((c) => c.id), ["1", "2"]);
  assert.deepEqual(boBinhLuan([a, b], "1").map((c) => c.id), ["2"]);
});

test("sáu đường của bài: đúng path, đúng method, bearer, và khoá attempt trên mọi lần ghi", async () => {
  datTokenPhien("tok");
  const daGoi = await batFetch(
    (n) => (n === 1 ? traLoi({ post_id: POST, comments: [], next_cursor: null, has_more: false }) : n === 4 ? new Response(null, { status: 204 }) : traLoi({ post_id: POST, reactions: [], my_reactions: [] })),
    async () => {
      await docBinhLuanBai(POST, ME, "c-1", 20);
      await guiBinhLuanBai(POST, "  Hay quá  ", ME, newAttempt());
      await themPhanUngBai(POST, "heart", ME, newAttempt());
      await xoaBinhLuanBai(POST, "bl-1", ME, newAttempt());
      await boPhanUngBai(POST, "heart", ME, newAttempt());
    },
  );
  assert.equal(daGoi.length, 5);
  assert.match(daGoi[0].url, new RegExp(`/posts/${POST}/comments\\?limit=20&after=c-1$`));
  assert.equal(daGoi[0].init.method, "GET");
  assert.match(daGoi[1].url, new RegExp(`/posts/${POST}/comments$`));
  assert.equal(daGoi[1].init.method, "POST");
  assert.deepEqual(JSON.parse(daGoi[1].init.body), { body: "Hay quá" });
  assert.match(daGoi[2].url, new RegExp(`/posts/${POST}/reactions$`));
  assert.deepEqual(JSON.parse(daGoi[2].init.body), { kind: "heart" });
  assert.match(daGoi[3].url, new RegExp(`/posts/${POST}/comments/bl-1$`));
  assert.equal(daGoi[3].init.method, "DELETE");
  assert.match(daGoi[4].url, new RegExp(`/posts/${POST}/reactions/heart$`));
  assert.equal(daGoi[4].init.method, "DELETE");
  for (const goi of daGoi) assert.equal(goi.init.headers.Authorization, "Bearer tok");
  for (const goi of daGoi.slice(1)) assert.ok(goi.init.headers["Idempotency-Key"], goi.url);
});

test("từ chối của máy chủ đọc thành câu: tường đóng là câu chung, không nói lý do khác", async () => {
  datTokenPhien("tok");
  await batFetch(
    () => traLoi({ code: "comments_closed", detail: "x" }, 403),
    async () => {
      await assert.rejects(guiBinhLuanBai(POST, "a", ME, newAttempt()), (loi) => loi.code === "comments_closed" && loi.message === CAU_KHONG_BINH_LUAN);
    },
  );
  for (const cau of Object.values(LOI_BAI)) assert.ok(!cau.includes("—"), cau);
});

test("nguồn ảnh của bài: ảnh cá nhân mang bearer, ảnh nhóm mang nhóm, đường tuyệt đối không bao giờ", () => {
  datTokenPhien("tok");
  const caNhan = nguonAnhBai(duongDanAnhNguoi(BAN, PHOTO), ME);
  assert.equal(caNhan.uri, `${BASE_URL}/people/${BAN}/photos/${PHOTO}`);
  assert.equal((caNhan.uri.match(/http:\/\//g) ?? []).length, 1, "một BASE_URL, không nối hai lần");
  assert.equal(caNhan.headers.Authorization, "Bearer tok");
  const nhom = nguonAnhBai(`/contexts/${CTX}/photos/${PHOTO}`, ME);
  assert.equal(nhom.headers["X-Actor-Contexts"], CTX);
  assert.equal(nhomCuaAnh(`/contexts/${CTX}/photos/${PHOTO}`), CTX);
  assert.equal(laAnhNguoi(`/people/${BAN}/photos/${PHOTO}`), true);
  assert.equal(laAnhNguoi(`/people/${BAN}/avatar`), false);
  assert.equal(nguonAnhBai("https://tracker.example/pixel.png", ME), null);
  assert.equal(nguonAnhBai(null, ME), null);
  assert.equal(nguonAnhBai("", ME), null);
});

test("chính sách tường: ba từ của máy chủ, PATCH chỉ mang một khoá", async () => {
  assert.deepEqual(CHINH_SACH.map((c) => c.id), ["readers", "friends", "nobody"]);
  assert.equal(laChinhSach("friends"), true);
  assert.equal(laChinhSach("everyone"), false);
  assert.equal(nhanChinhSach("nobody"), "Không ai");
  assert.equal(nhanChinhSach(undefined), CHINH_SACH[0].nhan);
  datTokenPhien("tok");
  const daGoi = await batFetch(
    () => traLoi({ id: ME, display_name: "Tôi", wall_comment_policy: "friends" }),
    async () => {
      await datChinhSachBinhLuan("friends", ME, newAttempt());
    },
  );
  assert.match(daGoi[0].url, /\/people\/me$/);
  assert.equal(daGoi[0].init.method, "PATCH");
  assert.deepEqual(JSON.parse(daGoi[0].init.body), { wall_comment_policy: "friends" });
});
