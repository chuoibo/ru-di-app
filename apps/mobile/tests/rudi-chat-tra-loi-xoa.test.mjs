/* Reply, sticker and delete on the chat wire (L1, ADR-0021 §2.1–2.3).
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node --test tests/rudi-chat-tra-loi-xoa.test.mjs
 *
 * What matters: an ordinary message carries NO `reply_to_id` key (a server
 * older than this slice refuses unknown keys), a reply carries exactly the id,
 * a sticker travels as its id and nothing else, a deletion is a DELETE on the
 * message with the bearer, and the held list after a 204 reads as the server
 * will read it on the next poll -- payload gone, reactions gone, quotes of it
 * turned into «Tin nhắn đã bị xoá».
 */
import assert from "node:assert/strict";
import test from "node:test";

import { datTokenPhien } from "../dist-test/api.js";
import { coGiDeDoi, doiNhom, thanDoiNhom } from "../dist-test/rudi/chat/nhom-cai-dat.js";
import {
  guiSticker,
  guiTin,
  thayTinDaXoa,
  trichTu,
  xemTruocTinCuoi,
  xoaTin,
} from "../dist-test/rudi/chat/tin-song.js";

const CTX = "3cc00000-cccc-4ccc-8ccc-0000c0000009";
const ME = "4dd00000-dddd-4ddd-8ddd-0000d0000009";
const BAN = "5ee00000-eeee-4eee-8eee-0000e0000009";

function tin(id, extra = {}) {
  return {
    id,
    context_id: CTX,
    author_id: ME,
    kind: "text",
    body: id,
    image_url: null,
    card: null,
    created_at: "2030-08-27T12:00:00Z",
    cursor: `c-${id}`,
    ...extra,
  };
}

function traLoi(body, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } });
}

test.afterEach(() => datTokenPhien(null));

test("guiTin không mang reply_to_id khi không trả lời; mang đúng id khi có", async () => {
  datTokenPhien("tok");
  const daGoi = [];
  const truoc = globalThis.fetch;
  globalThis.fetch = async (url, init) => {
    daGoi.push({ url, init });
    return traLoi({ ...tin("m"), intent: null }, 201);
  };
  try {
    await guiTin(CTX, ME, "xin chào", { key: "k1", at: 1 });
    const than1 = JSON.parse(daGoi[0].init.body);
    assert.deepEqual(than1, { kind: "text", body: "xin chào", image_url: null, card: null });
    assert.ok(!("reply_to_id" in than1), "không có khoá reply_to_id khi không trả lời");
    await guiTin(CTX, ME, "ừ", { key: "k2", at: 2 }, { replyToId: "goc" });
    assert.equal(JSON.parse(daGoi[1].init.body).reply_to_id, "goc");
    await guiTin(CTX, ME, "ừ", { key: "k3", at: 3 }, { replyToId: null });
    assert.ok(!("reply_to_id" in JSON.parse(daGoi[2].init.body)));
  } finally {
    globalThis.fetch = truoc;
  }
});

test("thử lại là GỬI LẠI CÙNG MỘT CHÌA: hai lời gọi cùng Attempt ra cùng header, thân giống từng byte", async () => {
  // Điều kiện đóng của F32: máy chủ nhận diện chìa và phát lại câu trả lời cũ
  // (`app/api/idempotency.py`, `Replay`), nên lần thử lại sau khi mất phản hồi
  // không thể thành tin thứ hai. Chứng minh ở phía client: chìa và thân đi ra
  // dây giống hệt nhau ở cả hai lần.
  datTokenPhien("tok");
  const daGoi = [];
  const truoc = globalThis.fetch;
  globalThis.fetch = async (url, init) => {
    daGoi.push({ url, init });
    return traLoi({ ...tin("s", { kind: "sticker", body: "cho-ti" }), intent: null }, 201);
  };
  const attempt = { key: "k-thu-lai", at: 1 };
  try {
    await guiSticker(CTX, ME, "cho-ti", attempt, { replyToId: "m-1" });
    await guiSticker(CTX, ME, "cho-ti", attempt, { replyToId: "m-1" });
    assert.equal(daGoi.length, 2);
    assert.equal(daGoi[0].init.headers["Idempotency-Key"], "k-thu-lai");
    assert.equal(daGoi[1].init.headers["Idempotency-Key"], daGoi[0].init.headers["Idempotency-Key"]);
    assert.equal(daGoi[1].init.body, daGoi[0].init.body, "khác một byte là 422 idempotency_key_reuse");
    assert.equal(daGoi[1].url, daGoi[0].url);
  } finally {
    globalThis.fetch = truoc;
  }
});

test("guiSticker gửi kind sticker với thân là id, không ảnh, không thẻ; có Idempotency-Key", async () => {
  datTokenPhien("tok");
  const daGoi = [];
  const truoc = globalThis.fetch;
  globalThis.fetch = async (url, init) => {
    daGoi.push({ url, init });
    return traLoi({ ...tin("s", { kind: "sticker", body: "di-thoi" }), intent: null }, 201);
  };
  try {
    await guiSticker(CTX, ME, "di-thoi", { key: "k-s", at: 1 });
    assert.match(daGoi[0].url, /\/contexts\/[^/]+\/messages$/);
    assert.equal(daGoi[0].init.method, "POST");
    assert.deepEqual(JSON.parse(daGoi[0].init.body), { kind: "sticker", body: "di-thoi", image_url: null, card: null });
    assert.equal(daGoi[0].init.headers["Idempotency-Key"], "k-s");
    assert.equal(daGoi[0].init.headers["Authorization"], "Bearer tok");
  } finally {
    globalThis.fetch = truoc;
  }
});

test("xoaTin là DELETE đúng đường, mang bearer, chấp nhận 204 không thân", async () => {
  datTokenPhien("tok");
  const daGoi = [];
  const truoc = globalThis.fetch;
  globalThis.fetch = async (url, init) => {
    daGoi.push({ url, init });
    return new Response(null, { status: 204 });
  };
  try {
    await xoaTin(CTX, "m-1", ME);
    assert.match(daGoi[0].url, /\/contexts\/[^/]+\/messages\/m-1$/);
    assert.equal(daGoi[0].init.method, "DELETE");
    assert.equal(daGoi[0].init.headers["Authorization"], "Bearer tok");
  } finally {
    globalThis.fetch = truoc;
  }
});

test("lỗi của máy chủ về xoá/trả lời thành câu tiếng Việt, không phải mã", async () => {
  datTokenPhien("tok");
  const truoc = globalThis.fetch;
  const cases = [
    ["message_already_deleted", 409, "Tin này đã bị xoá rồi."],
    ["message_kind_not_deletable", 409, "Chỉ xoá được tin nhắn, ảnh hoặc sticker của chính bạn."],
    ["sticker_unknown", 422, "Sticker này bản app chưa có."],
    ["reply_target_deleted", 409, "Tin bạn muốn trả lời đã bị xoá."],
  ];
  try {
    for (const [code, status, cau] of cases) {
      globalThis.fetch = async () => traLoi({ code, message: "raw" }, status);
      await assert.rejects(xoaTin(CTX, "m", ME), (e) => e.message === cau, code);
    }
  } finally {
    globalThis.fetch = truoc;
  }
});

test("thayTinDaXoa xoá nội dung, xoá phản ứng, và đổi mọi trích dẫn của tin đó", () => {
  const goc = tin("goc", { reactions: [{ kind: "heart", count: 2, mine: true }] });
  const traLoiGoc = tin("tl", { reply_to: { id: "goc", kind: "text", author_id: ME, preview: "goc" } });
  const khac = tin("khac");
  const sau = thayTinDaXoa([traLoiGoc, goc, khac], "goc", "2030-08-27T12:05:00Z");
  const gocSau = sau.find((t) => t.id === "goc");
  assert.equal(gocSau.kind, "deleted");
  assert.equal(gocSau.body, null);
  assert.equal(gocSau.image_url, null);
  assert.equal(gocSau.card, null);
  assert.deepEqual(gocSau.reactions, []);
  assert.equal(gocSau.deleted_at, "2030-08-27T12:05:00Z");
  const tlSau = sau.find((t) => t.id === "tl");
  assert.equal(tlSau.reply_to.kind, "deleted");
  assert.equal(tlSau.reply_to.preview, "Tin nhắn đã bị xoá");
  assert.equal(tlSau.body, "tl", "tin trả lời vẫn giữ lời của nó");
  assert.deepEqual(sau.find((t) => t.id === "khac"), khac);
});

test("xemTruocTinCuoi: sticker và tin đã xoá thành nhãn, chữ thường giữ nguyên", () => {
  assert.equal(xemTruocTinCuoi({ kind: "sticker", preview: "[Sticker]" }), "Đã gửi một sticker");
  assert.equal(xemTruocTinCuoi({ kind: "deleted", preview: "x" }), "Tin nhắn đã bị xoá");
  assert.equal(xemTruocTinCuoi({ kind: "text", preview: "Tối nay ăn gì" }), "Tối nay ăn gì");
  assert.equal(xemTruocTinCuoi({ kind: "image", preview: "[Ảnh]" }), "[Ảnh]");
});

test("trichTu rút một dòng ≤ 80 ký tự, gọi tên loại tin không có chữ", () => {
  const ten = (id) => (id === ME ? "Bạn" : "An QA");
  const dai = "a".repeat(120);
  assert.equal(trichTu(tin("m", { body: dai }), ten).preview.length, 80);
  assert.equal(trichTu(tin("m", { body: " nhiều   khoảng\ntrắng " }), ten).preview, "nhiều khoảng trắng");
  assert.equal(trichTu(tin("a", { kind: "image", body: null, image_url: "/x" }), ten).preview, "Ảnh");
  assert.equal(trichTu(tin("a", { kind: "image", body: "bãi biển", image_url: "/x" }), ten).preview, "Ảnh: bãi biển");
  assert.equal(trichTu(tin("s", { kind: "sticker", body: "di-thoi" }), ten).preview, "Sticker");
  assert.equal(trichTu(tin("d", { kind: "deleted", body: null }), ten).preview, "Tin nhắn đã bị xoá");
  const trich = trichTu(tin("m", { author_id: BAN, body: "hi" }), ten);
  assert.deepEqual(trich, { id: "m", kind: "text", author_id: BAN, preview: "hi" });
});

test("thanDoiNhom chỉ mang khoá đã đổi, tên được trim; coGiDeDoi từ chối tên rỗng", async () => {
  assert.deepEqual(thanDoiNhom({ display_name: "  Hội Đà Lạt " }), { display_name: "Hội Đà Lạt" });
  assert.deepEqual(thanDoiNhom({ theme: "bien-dem" }), { theme: "bien-dem" });
  assert.deepEqual(thanDoiNhom({ ai_auto_suggest: false }), { ai_auto_suggest: false });
  assert.deepEqual(thanDoiNhom({}), {});
  assert.equal(coGiDeDoi({}), false);
  assert.equal(coGiDeDoi({ display_name: "   " }), false);
  assert.equal(coGiDeDoi({ theme: "ruc-ro" }), true);

  datTokenPhien("tok");
  const daGoi = [];
  const truoc = globalThis.fetch;
  globalThis.fetch = async (url, init) => {
    daGoi.push({ url, init });
    return traLoi({ id: CTX, display_name: "Hội", created_by_id: ME, created_at: "2030-01-01T00:00:00Z", theme: "ruc-ro" });
  };
  try {
    const nhom = await doiNhom(CTX, ME, { theme: "ruc-ro" }, { key: "k-t", at: 1 });
    assert.equal(nhom.theme, "ruc-ro");
    assert.match(daGoi[0].url, /\/contexts\/[^/]+$/);
    assert.equal(daGoi[0].init.method, "PATCH");
    assert.deepEqual(JSON.parse(daGoi[0].init.body), { theme: "ruc-ro" });
    assert.equal(daGoi[0].init.headers["Idempotency-Key"], "k-t");
    globalThis.fetch = async () => traLoi({ code: "theme_unknown", message: "raw" }, 422);
    await assert.rejects(doiNhom(CTX, ME, { theme: "hong" }, { key: "k-u", at: 2 }), (e) => e.message === "Bộ màu này bản app chưa có.");
  } finally {
    globalThis.fetch = truoc;
  }
});
