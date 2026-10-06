import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { duongChiaBillCu, duongKeoCu } from "../dist-test/rudi/duong-cu.js";
import { nguCanhMo } from "../dist-test/rudi/ngu-canh-mo.js";

const outing = "d9aa82b4-7d9c-4675-8b27-36a96bc18609";
const session = {
  context_id: "newest-group",
  contexts: [{ id: "outing-group", my_state: "active" }, { id: "invited-group", my_state: "invited" }],
};
const target = (href) => new URL(href, "http://app.test");

test("cold legacy outing link retains the active group instead of using the newest group", () => {
  const route = target(duongKeoCu(outing, "outing-group"));
  assert.equal(route.pathname, `/outings/${outing}`);
  assert.deepEqual(nguCanhMo(session, route.searchParams.get("ctx")), { kieu: "khac", contextId: "outing-group" });
});

test("legacy links retain refused contexts until the canonical access guard rejects them", () => {
  for (const ctx of ["invited-group", "another-person-group", "bad&ctx=newest-group"]) {
    const route = target(duongKeoCu(outing, ctx));
    assert.equal(route.searchParams.get("ctx"), ctx);
    assert.deepEqual(nguCanhMo(session, route.searchParams.get("ctx")), { kieu: "tu-choi" });
  }
});

test("old assignment link keeps the outing, group and occasion in the real bill route", () => {
  const route = target(duongChiaBillCu(outing.toUpperCase(), "outing-group", "Ăn trưa & cà phê"));
  assert.equal(route.pathname, `/smart-split/${outing}/review`);
  assert.equal(route.searchParams.get("dip"), "Ăn trưa & cà phê");
  assert.deepEqual(nguCanhMo(session, route.searchParams.get("ctx")), { kieu: "khac", contextId: "outing-group" });
});

test("resource-less and historical non-outing links keep their established destination", () => {
  assert.equal(duongKeoCu(undefined, "outing-group"), "/(tabs)/plan");
  assert.equal(duongKeoCu(outing, undefined), `/outings/${outing}`);
  assert.equal(target(duongChiaBillCu("old-bill", "outing-group", undefined)).pathname, "/smart-split/moi/review");
  assert.equal(target(duongChiaBillCu("moi", undefined, undefined)).search, "");
});

test("both shipped trip aliases read and forward ctx through the tested adapter", () => {
  for (const name of ["timeline", "itinerary"]) {
    const source = readFileSync(new URL(`../app/trips/[id]/${name}.tsx`, import.meta.url), "utf8");
    assert.match(source, /\{ id, ctx \} = useLocalSearchParams/);
    assert.match(source, /<Redirect href=\{duongKeoCu\(id, ctx\)/);
  }
  const source = readFileSync(new URL("../app/smart-split/[id]/assignment.tsx", import.meta.url), "utf8");
  assert.match(source, /duongChiaBillCu\(id, ctx, dip\)/);
  assert.match(source, /<Redirect href=\{href/);
});
