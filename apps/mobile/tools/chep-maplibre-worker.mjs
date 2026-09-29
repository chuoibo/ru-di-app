#!/usr/bin/env node
/* Put MapLibre GL's worker where the web build serves it.
 *
 *   node tools/chep-maplibre-worker.mjs          copy if missing or different
 *   node tools/chep-maplibre-worker.mjs --kiem   check only; exit 1 when absent or stale
 *
 * maplibre-gl 6 loads its tile worker from a URL it derives from
 * `import.meta.url` of its own module. Inside Metro's web bundle that is not an
 * http URL, so the derived URL is empty, no worker ever starts, and the map
 * loads its style but never a single tile: the journey map on the web was a
 * blank paper sheet with pins on it. `hanh-trinh/BanDo.tsx` calls
 * `setWorkerUrl("/maplibre/maplibre-gl-worker.mjs")`; Expo serves `public/`
 * at the site root, so the worker and the shared chunk it imports go there.
 *
 * Never committed (vendored code, and it would drift from the installed
 * version): copied from node_modules on `postinstall` and before every web
 * build, compared by sha256 -- the same pattern as `chep-canvaskit.mjs`.
 */
import { createHash } from "node:crypto";
import { copyFileSync, existsSync, mkdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const GOC = join(dirname(fileURLToPath(import.meta.url)), "..");
const NGUON = join(GOC, "node_modules", "maplibre-gl", "dist");
const DICH = join(GOC, "public", "maplibre");
const TEP = ["maplibre-gl-worker.mjs", "maplibre-gl-shared.mjs"];
const chiKiem = process.argv.includes("--kiem");

const bam = (tep) => createHash("sha256").update(readFileSync(tep)).digest("hex");

if (!TEP.every((t) => existsSync(join(NGUON, t)))) {
  console.log("chep-maplibre-worker: chưa có maplibre-gl 6 trong node_modules, bỏ qua.");
  process.exit(chiKiem ? 1 : 0);
}

let cu = 0;
for (const t of TEP) {
  const nguon = join(NGUON, t);
  const dich = join(DICH, t);
  if (existsSync(dich) && bam(dich) === bam(nguon)) continue;
  cu += 1;
  if (!chiKiem) {
    mkdirSync(DICH, { recursive: true });
    copyFileSync(nguon, dich);
  }
}
if (chiKiem && cu > 0) {
  console.error(`chep-maplibre-worker: ${cu} tệp worker thiếu hoặc cũ trong public/maplibre/.`);
  process.exit(1);
}
console.log(cu ? `chep-maplibre-worker: đã chép ${cu} tệp.` : "chep-maplibre-worker: đã khớp.");
