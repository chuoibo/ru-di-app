/**
 * A button that stays disabled says why (ADR-0038 §2.2, QA UI-091; B11). The
 * kit only warned in development, once per label, so 14 buttons on 10 screens
 * waited silently on a condition the person could not see: a missing name, a
 * photo not chosen, a countdown, a plan that changed on another phone.
 *
 * Swept from the source: every `RudiButton` with `disabled=` whose condition
 * holds while nothing is in flight must carry `lyDo=`. A condition made only of
 * «an action is running» terms (busy, ban, dang…, a `…dangCho`/`…dangHoi`
 * flag) is exempt: the running button shows its spinner and the wait is over
 * in a moment. Named exceptions say why. Does not prove that the sentence is
 * right, or that it shows (the screenshots do).
 */
import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const goc = fileURLToPath(new URL("..", import.meta.url));

/** «Something is running» terms; anything else in a condition can last. */
const TAM_THOI = /^!?\(?\s*(busy|ban|loading|communityBusy|dang[A-Z]\w*|trang\.dang[A-Z]\w*|buc\.dangCho|phien\.dangHoi)(\s*!==\s*null)?\s*\)?$/;

const NGOAI_LE = {
  // «Vẽ» sits beside «Gửi» and waits for the same words; «Gửi» says why once.
  "src/rudi/nep/NepBang.tsx": ["Vẽ"],
};

function tsx(thuMuc, ra = []) {
  for (const ten of readdirSync(thuMuc)) {
    const p = join(thuMuc, ten);
    if (statSync(p).isDirectory()) tsx(p, ra);
    else if (p.endsWith(".tsx")) ra.push(p);
  }
  return ra;
}

/** The text of `disabled={…}` with its braces balanced. */
function dieuKien(nut) {
  const i = nut.indexOf("disabled={");
  if (i < 0) return null;
  let sau = 0;
  for (let j = i + "disabled=".length; j < nut.length; j++) {
    if (nut[j] === "{") sau++;
    else if (nut[j] === "}" && --sau === 0) return nut.slice(i + "disabled={".length, j);
  }
  return null;
}

test("nút tắt lâu dài nói vì sao", () => {
  const thieu = [];
  let dem = 0;
  for (const p of tsx(join(goc, "src/rudi"))) {
    const rel = relative(goc, p);
    const src = readFileSync(p, "utf8");
    for (const m of src.matchAll(/<RudiButton\b(?:(?!<RudiButton\b)[\s\S])*?\/>/g)) {
      const dk = dieuKien(m[0]);
      if (dk === null) continue;
      dem += 1;
      if (dk.split("||").every((t) => TAM_THOI.test(t.trim()))) continue;
      if (/\blyDo=/.test(m[0])) continue;
      const nhan = /\blabel="([^"]+)"/.exec(m[0])?.[1] ?? /\blabel=\{([^}]+)\}/.exec(m[0])?.[1] ?? "?";
      if ((NGOAI_LE[rel] ?? []).includes(nhan)) continue;
      thieu.push(`${rel}: «${nhan}» disabled={${dk}}`);
    }
  }
  assert.ok(dem >= 60, `quét được ${dem} nút có disabled; ít hơn thế là phép quét đã hỏng`);
  assert.deepEqual(thieu, [], "nút tắt theo điều kiện lâu dài mà không có lyDo");
});
