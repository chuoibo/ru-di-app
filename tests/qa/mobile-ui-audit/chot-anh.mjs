/* Commit-side of the evidence: copy the chosen JPEGs next to the report, pin
 * each one in the repository guard's allowlist, and write the manifest.
 *
 *   AUDIT_OUT=… node chot-anh.mjs <docs-dir> <list.json>
 *
 * <list.json> is [{ "id": "EV-…", "tu": "EV-…-ct" (optional source name),
 * "moTa": "…" }]. The requester allowed committing screenshots for THIS audit
 * (27/09); the guard still fails closed on binaries, so each file is pinned by
 * path and sha256 of the exact bytes, and changing one byte means pinning
 * again. Idempotent: an entry for the same path is replaced, never duplicated.
 *
 * Every pin written from checkpoint N14 on opens with a narrow annotation for
 * the aggregate-base64-fragments rule (docs/security/repo-guard.md §6): the
 * rule adds up every mixed-case token of the allowlist, and the image paths
 * alone passed its 16 KiB ceiling at the N14 pins (15746 → 16436 bytes). The
 * annotation sits in `reason`, written before `path`, so it covers exactly
 * the reason line and the path line of that entry and nothing further. Pins
 * made before N14 keep their shape word for word (the requester's choice,
 * 30/09): the ones already in the file stay under the ceiling.
 */
import { createHash } from "node:crypto";
import { copyFileSync, existsSync, mkdirSync, readFileSync, readdirSync, statSync, writeFileSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const out = process.env.AUDIT_OUT;
const [docs, listFile] = process.argv.slice(2);
if (!out || !docs || !listFile) throw new Error("dùng: AUDIT_OUT=… node chot-anh.mjs <docs> <list.json>");
const repo = resolve(fileURLToPath(new URL("../../..", import.meta.url)));
const evDir = join(docs, "evidence");
mkdirSync(evDir, { recursive: true });
const ds = JSON.parse(readFileSync(listFile, "utf8"));
const sha = (b) => createHash("sha256").update(b).digest("hex");

for (const d of ds) {
  const src = join(out, "jpg", `${d.tu ?? d.id}.jpg`);
  if (!existsSync(src)) throw new Error(`thiếu ảnh nguồn ${src}`);
  copyFileSync(src, join(evDir, `${d.id}.jpg`));
}

const allowFile = join(repo, ".repo-guard-allowlist.json");
const allow = JSON.parse(readFileSync(allowFile, "utf8"));
// The base audit keeps its reason word for word (its pins must not churn); a
// follow-up of the same engagement (the retest of main) names its own folder.
const thuMuc = relative(repo, resolve(docs)).split("\\").join("/");
const LY_DO =
  thuMuc === "docs/claude/2026-09-27/mobile-ui-audit"
    ? "Bằng chứng ảnh của audit UI/UX mobile 27/09/2026 (docs/claude/2026-09-27/mobile-ui-audit): ảnh chụp bản web trên stack cục bộ với dữ liệu seed tổng hợp, số điện thoại đã che, không có dữ liệu người thật. Người giao việc cho phép commit ảnh trong đợt audit này. Đổi một byte phải ghim lại."
    : `Bằng chứng ảnh của phần tiếp theo audit UI/UX mobile (${thuMuc}): ảnh chụp bản web của main trên stack cục bộ thứ hai với dữ liệu seed tổng hợp, không có số điện thoại, không có dữ liệu người thật. Người giao việc cho phép commit ảnh của đợt audit này và các phần sau pipeline của nó. Đổi một byte phải ghim lại.`;
const CHU_THICH = "repo-guard: allow=aggregate-base64-fragments reason=audit-evidence-path";
const coChuThich = (a) => typeof a.reason === "string" && a.reason.startsWith(CHU_THICH);
const moTa = new Map(ds.map((d) => [d.id, d.moTa]));
const cu = existsSync(join(docs, "evidence-manifest.json")) ? JSON.parse(readFileSync(join(docs, "evidence-manifest.json"), "utf8")) : {};
const manifest = {};
let tong = 0;
for (const f of readdirSync(evDir).filter((x) => x.endsWith(".jpg")).sort()) {
  const full = join(evDir, f);
  const buf = readFileSync(full);
  const path = relative(repo, full).split("\\").join("/");
  const id = f.replace(/\.jpg$/, "");
  const i = allow.artifacts.findIndex((a) => a.path === path);
  // An old pin keeps its shape; a new pin (or one already annotated) carries the
  // annotation in a reason written ahead of the path it covers.
  const entry =
    i >= 0 && !coChuThich(allow.artifacts[i])
      ? { path, sha256: sha(buf), rules: ["controlled-artifact"], reason: LY_DO }
      : { reason: `${CHU_THICH} · ${LY_DO}`, path, sha256: sha(buf), rules: ["controlled-artifact"] };
  if (i >= 0) allow.artifacts[i] = entry;
  else allow.artifacts.push(entry);
  // Eight hex characters: enough to tell files apart, never nine digits in a
  // row (the guard reads a free-standing run of nine as a long number).
  manifest[id] = { bytes: statSync(full).size, sha8: entry.sha256.slice(0, 8), moTa: moTa.get(id) ?? cu[id]?.moTa ?? "" };
  tong += statSync(full).size;
}
writeFileSync(allowFile, `${JSON.stringify(allow, null, 2)}\n`);
writeFileSync(join(docs, "evidence-manifest.json"), `${JSON.stringify(manifest, null, 1)}\n`);
let md = `# Evidence manifest\n\nẢnh bằng chứng đã commit của audit (${Object.keys(manifest).length} ảnh, tổng ${(tong / 1048576).toFixed(2)} MiB). Mỗi ảnh được ghim path + sha256 trong \`.repo-guard-allowlist.json\`. Ảnh gốc PNG, số đo JSON và video nằm ngoài git.\n\n| Ảnh | Mô tả | Byte | sha256 (8 ký tự đầu) |\n|---|---|---|---|\n`;
for (const [id, m] of Object.entries(manifest)) md += `| [${id}](evidence/${id}.jpg) | ${m.moTa.replace(/\|/g, "\\|")} | ${m.bytes} | \`${m.sha8}\` |\n`;
writeFileSync(join(docs, "evidence-manifest.md"), md);
console.log(JSON.stringify({ anh: Object.keys(manifest).length, MiB: +(tong / 1048576).toFixed(2), ghim: allow.artifacts.length }));
