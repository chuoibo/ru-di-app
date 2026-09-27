#!/usr/bin/env node
/* Put CanvasKit's wasm where the web build serves it (ADR-0037 D10).
 *
 *   node tools/chep-canvaskit.mjs          copy if missing or different
 *   node tools/chep-canvaskit.mjs --kiem   check only; exit 1 when absent or stale
 *
 * `@shopify/react-native-skia` loads `canvaskit-wasm/bin/full/canvaskit.js`,
 * which asks for `canvaskit.wasm` through the `locateFile` that
 * `src/rudi/ui/skia/nap-skia.ts` passes: `/canvaskit.wasm`. Expo serves
 * `public/` at the site root in dev and copies it into the export, so the file
 * has to be in `public/`.
 *
 * It is never committed: 8 MB of binary is exactly what the repo guard fails
 * closed on, and `public/canvaskit.wasm` is in `.gitignore`. It is copied from
 * the installed package instead -- on `postinstall`, and again before every web
 * build -- and compared by sha256, so a stale copy from an older install cannot
 * be served against a newer `canvaskit.js`.
 */
import { createHash } from "node:crypto";
import { copyFileSync, existsSync, mkdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const GOC = join(dirname(fileURLToPath(import.meta.url)), "..");
const NGUON = join(GOC, "node_modules", "canvaskit-wasm", "bin", "full", "canvaskit.wasm");
const DICH = join(GOC, "public", "canvaskit.wasm");
const chiKiem = process.argv.includes("--kiem");

function bam(tep) {
  return createHash("sha256").update(readFileSync(tep)).digest("hex");
}

if (!existsSync(NGUON)) {
  // No Skia installed (a partial install, or a tool that runs before npm ci):
  // nothing to copy, and the app falls back to SVG on the web by design.
  console.log("chep-canvaskit: chưa có canvaskit-wasm trong node_modules, bỏ qua.");
  process.exit(chiKiem ? 1 : 0);
}

const nguon = bam(NGUON);
const khop = existsSync(DICH) && bam(DICH) === nguon;

if (chiKiem) {
  if (!khop) {
    console.error(`chep-canvaskit --kiem: public/canvaskit.wasm ${existsSync(DICH) ? "lệch bản cài" : "chưa có"}.`);
    process.exit(1);
  }
  console.log(`chep-canvaskit --kiem: khớp (${nguon.slice(0, 12)}…).`);
  process.exit(0);
}

if (!khop) {
  mkdirSync(dirname(DICH), { recursive: true });
  copyFileSync(NGUON, DICH);
  console.log(`chep-canvaskit: đã chép canvaskit.wasm (${nguon.slice(0, 12)}…) vào public/.`);
} else {
  console.log("chep-canvaskit: public/canvaskit.wasm đã khớp.");
}
