/* Tin nào cũng là lời nhờ Rủ Đi AI (ADR-0039, đề xuất).
 *
 * Đo: `@Rủ Đi` được nhận ở bất kỳ vị trí nào trong tin, ở cả dạng NFC lẫn NFD
 * (bàn phím gõ dấu rời), có dấu hay không dấu; KHÔNG bao giờ bị nhận trong
 * một địa chỉ email hay một chữ dài hơn; `/plan` và `/chia-bill` chỉ ở đầu
 * tin; lời nhờ là phần còn lại sau khi bỏ lệnh và mọi mention, và một lời nhờ
 * rỗng vẫn gửi được vì máy chủ từ chối prompt rỗng.
 *
 * KHÔNG đo: máy chủ làm gì với tin (tin `@Rủ Đi` là chữ thường ở máy chủ,
 * cổng Go `actOnMessageIntent` giữ điều đó), hay màn thật vẽ chip ra sao.
 *
 * Chạy từ apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs
 *     node --test tests/nhac-ai.test.mjs
 */
import assert from "node:assert/strict";
import test from "node:test";

import { LOI_NHO_PLAN, tachLoiNho, timNhacAi } from "../dist-test/rudi/chat/nhac-ai.js";
import { LOI_NHO_CHIA_BILL } from "../dist-test/rudi/chat/ai-invocations.js";

test("mention ở bất kỳ đâu trong tin, có dấu hay không dấu, hoa hay thường", () => {
  const ca = {
    "@Rủ Đi tối nay ăn gì gần Q1?": "tối nay ăn gì gần Q1?",
    "tối nay @Rủ Đi gợi ý quán": "tối nay gợi ý quán",
    "Đi đâu đây @rủ đi": "Đi đâu đây",
    "@rudi ơi": "ơi",
    "@ru di chỗ nào mát": "chỗ nào mát",
    "@RUDI chọn giúp": "chọn giúp",
    "@Rủ  Đi, quán nào mở khuya?": "quán nào mở khuya?",
    "Đi đâu đây @Rủ Đi.": "Đi đâu đây.",
  };
  for (const [tin, loiNho] of Object.entries(ca)) {
    assert.deepEqual(timNhacAi(tin), { lenh: "plan", loiNho }, tin);
  }
  // An opening bracket ends a word as well as a space does.
  assert.equal(timNhacAi("(@rudi) gợi ý")?.lenh, "plan");
});

test("bàn phím gõ dấu rời (NFD) vẫn là mention", () => {
  const nfd = "@Rủ Đi tối nay".normalize("NFD");
  assert.notEqual(nfd, "@Rủ Đi tối nay", "chuỗi thử phải thật sự ở dạng NFD");
  assert.deepEqual(timNhacAi(nfd), { lenh: "plan", loiNho: "tối nay" });
  assert.deepEqual(timNhacAi(`hỏi ${"@rủ đi".normalize("NFD")} giúp`), { lenh: "plan", loiNho: "hỏi giúp" });
});

test("không phải mention: email, chữ dài hơn, tên miền, hay không có @", () => {
  for (const tin of [
    // repo-guard: allow=email reason=synthetic-invalid-domain
    "gửi lan@rudi.invalid nhé",
    // repo-guard: allow=email reason=synthetic-invalid-domain
    "email: a.b@rudi.invalid",
    "@rudi.invalid là trang của mình",
    "@rudivn",
    "@ru dinh dưỡng",
    "rủ đi chơi không",
    "tối nay ăn gì",
    "@Rủ Đinh",
    "x_@rudi",
  ]) {
    assert.equal(timNhacAi(tin), null, tin);
  }
});

test("/plan và /chia-bill chỉ ở đầu tin; /vote và chữ giữa câu là tin thường", () => {
  assert.deepEqual(timNhacAi("/plan tối nay đi đâu"), { lenh: "plan", loiNho: "tối nay đi đâu" });
  assert.deepEqual(timNhacAi("/PLAN"), { lenh: "plan", loiNho: LOI_NHO_PLAN });
  assert.deepEqual(timNhacAi("/chia-bill"), { lenh: "chia_bill", loiNho: LOI_NHO_CHIA_BILL });
  assert.deepEqual(timNhacAi("/chiabill  "), { lenh: "chia_bill", loiNho: LOI_NHO_CHIA_BILL });
  assert.deepEqual(timNhacAi("/Chia-Bill mình trả 300k tiền nước"), { lenh: "chia_bill", loiNho: "mình trả 300k tiền nước" });
  // A mention inside a /chia-bill message is still a /chia-bill.
  assert.deepEqual(timNhacAi("/chia-bill @Rủ Đi gom giúp"), { lenh: "chia_bill", loiNho: "gom giúp" });
  for (const tin of ["/planning", "/vote Ăn gì? Phở | Bún", "tối nay /plan đi", "/chia-billx"]) {
    assert.equal(timNhacAi(tin), null, tin);
  }
});

test("lời nhờ rỗng vẫn gửi được, vì máy chủ từ chối prompt rỗng", () => {
  assert.equal(tachLoiNho("@Rủ Đi"), LOI_NHO_PLAN);
  assert.equal(tachLoiNho("@Rủ Đi, @rudi"), LOI_NHO_PLAN);
  assert.equal(tachLoiNho("/plan"), LOI_NHO_PLAN);
  assert.ok(LOI_NHO_PLAN.trim().length > 0 && LOI_NHO_CHIA_BILL.trim().length > 0);
  // Every mention goes, wherever it sat; what is left keeps its own words.
  assert.equal(tachLoiNho("@Rủ Đi, @rudi ơi đi đâu"), "ơi đi đâu");
});
