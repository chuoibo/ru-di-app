import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import test from "node:test";
import { transformSync } from "@babel/core";
import { CHAT_VIEWABILITY } from "../dist-test/rudi/chat/viewability.js";

// Execute the installed RN implementation, not a local copy of its geometry.
// Babel only removes Flow and lowers modules. Its retained type-only import
// is never called by ViewabilityHelper; fail if that assumption changes.
const require = createRequire(import.meta.url);
const filename = require.resolve("@react-native/virtualized-lists/Lists/ViewabilityHelper");
const nativeRequire = createRequire(filename);
const { code } = transformSync(readFileSync(filename, "utf8"), {
  filename, presets: ["@react-native/babel-preset"], babelrc: false, configFile: false,
});
const module = { exports: {} };
new Function("require", "module", "exports", code)((name) => {
  if (name === "./ListMetricsAggregator") return function UnexpectedConstruction() {
    throw new Error("ViewabilityHelper now needs runtime ListMetricsAggregator; update the test loader.");
  };
  return nativeRequire(name);
}, module, module.exports);
const ViewabilityHelper = module.exports.default;

function geometry(length) {
  const props = { data: ["message"], getItemCount: (data) => data.length, getItem: (data, i) => data[i] };
  const metrics = { getCellMetrics: () => ({ offset: 0, length, index: 0, isMounted: true }) };
  return { props, metrics };
}

test("RN thật: tin dài hơn viewport vẫn được đọc khi chiếm đủ vùng nhìn", () => {
  const { props, metrics } = geometry(1200);
  const prior = new ViewabilityHelper({ itemVisiblePercentThreshold: 60 });
  const current = new ViewabilityHelper(CHAT_VIEWABILITY);
  for (const offset of [0, 200, 500, 700]) {
    assert.deepEqual(prior.computeViewableItems(props, offset, 500, metrics), [], "negative control must expose the old gap");
    assert.deepEqual(current.computeViewableItems(props, offset, 500, metrics), [0]);
  }
  assert.deepEqual(current.computeViewableItems(props, 1199, 500, metrics), [], "one remaining pixel is not reading");
});

test("RN thật: tin ngắn hiện đầy đủ được đọc; phần ló ở mép không được đọc", () => {
  const { props, metrics } = geometry(100);
  const helper = new ViewabilityHelper(CHAT_VIEWABILITY);
  assert.deepEqual(helper.computeViewableItems(props, 0, 500, metrics), [0]);
  assert.deepEqual(helper.computeViewableItems(props, 90, 500, metrics), []);
});

test("RN thật: chờ 600ms, lướt qua trước hạn không phát read callback", (t) => {
  t.mock.timers.enable({ apis: ["setTimeout"] });
  const { props, metrics } = geometry(1200);
  const helper = new ViewabilityHelper(CHAT_VIEWABILITY);
  const seen = [];
  const token = (index, isViewable) => ({ key: "message", item: "message", index, isViewable });
  const report = (info) => seen.push(info.viewableItems.map((v) => v.key));
  try {
    helper.onUpdate(props, 0, 500, metrics, token, report);
    t.mock.timers.tick(599);
    assert.deepEqual(seen, []);
    helper.onUpdate(props, 1200, 500, metrics, token, report);
    t.mock.timers.tick(1);
    assert.deepEqual(seen, []);
    t.mock.timers.tick(600);
    helper.onUpdate(props, 200, 500, metrics, token, report);
    t.mock.timers.tick(599);
    assert.deepEqual(seen, []);
    t.mock.timers.tick(1);
    assert.deepEqual(seen, [["message"]]);
  } finally { helper.dispose(); }
});
