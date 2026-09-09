// Source-level diagnostic only: a small hook scheduler, not React/native E2E.
// Run from the repository root. All identities/messages below are synthetic.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import vm from "node:vm";

const require = createRequire(new URL("../../../../apps/mobile/package.json", import.meta.url));
const ts = require("typescript");
const base = new URL("../../../../apps/mobile/src/rudi/chat/", import.meta.url);
function compile(name, imports) {
  const source = readFileSync(new URL(name, base), "utf8");
  const code = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  const exports = {};
  vm.runInNewContext(code, {
    exports, require: (id) => {
      if (!(id in imports)) throw new Error(`Unexpected import: ${id}`);
      return imports[id];
    }, Date, setInterval, clearInterval,
  }, { filename: name });
  return exports;
}

let counter = 0;
const api = {
  newAttempt: () => ({ key: `synthetic-attempt-${++counter}` }),
  ApiError: class extends Error {},
  thongDiepNguoiDoc: () => "synthetic failure",
};
const wire = compile("tin-song.ts", {
  "../../api": api, "../../screens/chat/ke-hoach": {},
});
const queue = compile("hang-cho.ts", {});
const slots = [];
let index = 0;
let effects = [];
const same = (a, b) => a && b && a.length === b.length && a.every((v, i) => Object.is(v, b[i]));
const hooks = {
  useState(initial) {
    const key = index++;
    if (!(key in slots)) slots[key] = typeof initial === "function" ? initial() : initial;
    return [slots[key], (value) => { slots[key] = typeof value === "function" ? value(slots[key]) : value; }];
  },
  useRef(initial) {
    const key = index++;
    if (!(key in slots)) slots[key] = { current: initial };
    return slots[key];
  },
  useCallback(callback, deps) {
    const key = index++;
    if (!same(slots[key]?.deps, deps)) slots[key] = { deps, callback };
    return slots[key].callback;
  },
  useEffect(effect, deps) {
    const key = index++;
    if (!same(slots[key]?.deps, deps)) {
      effects.push(() => { slots[key]?.cleanup?.(); slots[key] = { deps, cleanup: effect() }; });
    }
  },
};
let resolveSend;
const reads = [];
const message = (context, suffix) => ({
  id: `${context}-${suffix}`, context_id: context, author_id: "synthetic-person",
  kind: "sticker", body: "cho-ti", image_url: null, card: null,
  cursor: `${context}-${suffix}`, created_at: "2026-09-09T00:00:00.000Z",
});
const { useTinNhan } = compile("useTinNhan.ts", {
  react: hooks,
  // Poll/focus behavior is deliberately excluded from this bounded send probe.
  "expo-router": { useFocusEffect() {} },
  "react-native": { AppState: {} },
  "../../api": api,
  "./hang-cho": queue,
  "./tin-song": {
    ...wire,
    docTrangTin: async (context) => {
      reads.push(context);
      return { messages: [message(context, "existing")], has_more: false };
    },
    guiSticker: () => new Promise((resolve) => { resolveSend = resolve; }),
  },
});
const flush = async () => { for (let i = 0; i < 8; i++) await Promise.resolve(); };
function render(context) {
  index = 0;
  effects = [];
  const result = useTinNhan(context, "synthetic-person");
  for (const effect of effects) effect();
  return result;
}
render("A");
await flush();
const pending = render("A").guiSticker("cho-ti");
render("B");
await flush();
const before = render("B").tin.map((m) => m.id);
assert.deepEqual(Array.from(before), ["B-existing"]);
resolveSend(message("A", "late-send"));
await pending;
await flush();
const after = render("B").tin.map((m) => m.id);
const reproduced = after.some((id) => id.startsWith("A-"));
console.log(JSON.stringify({ kind: "source-hook-scheduler-not-native", before, after, reads, reproduced }, null, 2));
assert.equal(reproduced, true, "The historical defect did not reproduce; review whether the source has been fixed.");
