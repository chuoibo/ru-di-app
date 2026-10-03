/* The pieces around the OTP door that are not the wire: what the code screen
 * shows of a number, that no demo door is left, and what `nguon.ts` says
 * about a session that has no group yet.
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node --test tests/rudi-cua-otp.test.mjs
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { readdirSync, statSync } from "node:fs";
import { nguonHienTai } from "../dist-test/rudi/nguon.js";
import { cheSo } from "../dist-test/rudi/otp-dang-cho.js";

const doc = (path) => readFileSync(new URL(`../${path}`, import.meta.url), "utf8");

// Few digits on purpose: the repo guard reads nine in a row as a phone number.
test("cheSo giữ đúng ba số cuối và giấu phần còn lại", () => {
  assert.equal(cheSo("09 345 678"), "••• ••• 678");
  assert.equal(cheSo("+84 (9) 34-678"), "••• ••• 678");
  assert.ok(!cheSo("09 345 678").includes("345"));
  assert.equal(cheSo("12"), "số của bạn");
});

/** Every source file of the app shell, for the sweep below. */
function moiTep(thuMuc) {
  const goc = new URL(`../${thuMuc}/`, import.meta.url);
  return readdirSync(goc, { recursive: true })
    .filter((ten) => /\.(ts|tsx)$/.test(ten))
    .map((ten) => `${thuMuc}/${ten}`)
    .filter((duong) => statSync(new URL(`../${duong}`, import.meta.url)).isFile());
}

test("không còn cửa demo nào: không nút «bản trải nghiệm», không cờ EXPO_PUBLIC_RUDI_FIXTURE", () => {
  const tep = [...moiTep("app"), ...moiTep("src")];
  assert.ok(tep.length > 100, "quét không thấy mã nguồn");
  for (const duong of tep) {
    const src = doc(duong);
    assert.doesNotMatch(src, /EXPO_PUBLIC_RUDI_FIXTURE|CUA_FIXTURE_DEV/, `${duong} còn cờ cửa fixture`);
    assert.doesNotMatch(src, /"[^"\n]*[Bb]ản trải nghiệm[^"\n]*"/, `${duong} còn chữ «bản trải nghiệm» hiện ra màn`);
  }
});

test("Team Đà Lạt chỉ còn trong dữ liệu harness, không màn nào import", () => {
  for (const duong of [...moiTep("app"), ...moiTep("src")]) {
    if (duong === "src/rudi/nhom-demo.ts") continue;
    assert.doesNotMatch(doc(duong), /from "[^"]*nhom-demo"/, `${duong} import dữ liệu harness`);
  }
});

test("phiên không có nhóm là chưa-có-nhóm có lý do, không phải live với contextId rỗng", () => {
  const nguon = nguonHienTai(
    { person_id: "p", context_id: null, membership_state: null },
    {},
  );
  assert.equal(nguon.kieu, "chua-co-nhom");
  assert.match(nguon.viSao, /chưa ở nhóm nào/);
});
