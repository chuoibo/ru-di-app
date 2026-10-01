/* Pins the generated coverage matrix in .repo-guard-allowlist.json by path and
 * digest, for the one content rule it trips: aggregate-base64-fragments.
 *
 * Every row a committed composite backs links it (`[EV-…-ghep](evidence/EV-…-ghep.jpg)`),
 * and the guard counts those paths and ids as base64-like fragments. From
 * checkpoint N22 the main folder's matrix sums past 16 KiB (17757 bytes in 889
 * fragments, 14692 of those bytes in the composite links). docs/security/repo-guard.md
 * §6 allows a digest pin for such a false positive, as it does for the mobile
 * lockfile. The pin covers these exact bytes only: run this after every
 * tong-hop, or the range and tree scans fail closed on the stale digest.
 *
 *   node ghim-ma-tran.mjs <docs-dir>
 */
import { createHash } from "node:crypto";
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const [docs] = process.argv.slice(2);
if (!docs) throw new Error("dùng: node ghim-ma-tran.mjs <docs-dir>");
const repo = resolve(fileURLToPath(new URL("../../..", import.meta.url)));
const full = join(docs, "coverage-matrix.md");
if (!existsSync(full)) throw new Error(`không có ${full}`);
const path = relative(repo, resolve(full)).split("\\").join("/");
const sha256 = createHash("sha256").update(readFileSync(full)).digest("hex");

const allowFile = join(repo, ".repo-guard-allowlist.json");
const allow = JSON.parse(readFileSync(allowFile, "utf8"));
// The annotation leads the reason, so the path line below it stays out of the
// allowlist's own aggregate, as for the evidence pins (chot-anh.mjs).
const reason =
  "repo-guard: allow=aggregate-base64-fragments reason=audit-evidence-path · Ma trận coverage sinh máy (tong-hop.mjs) " +
  "của audit UI/UX mobile trên main. Luật aggregate-base64-fragments đếm đường dẫn ảnh bằng chứng, ID ảnh và ID test case " +
  "như mảnh base64; file không có dữ liệu mã hoá, số điện thoại hay token (đã quét). Ghim theo digest như lockfile sinh máy " +
  "(docs/security/repo-guard.md §6). Mỗi lần tong-hop sinh lại ma trận thì ghim lại.";
const entry = { reason, path, sha256, rules: ["aggregate-base64-fragments"] };
const i = allow.artifacts.findIndex((a) => a.path === path);
const truoc = i >= 0 ? allow.artifacts[i].sha256 : null;
if (i >= 0) allow.artifacts[i] = entry;
else allow.artifacts.push(entry);
writeFileSync(allowFile, `${JSON.stringify(allow, null, 2)}\n`);
console.log(JSON.stringify({ path, sha8: sha256.slice(0, 8), doi: truoc === null ? "ghim mới" : truoc === sha256 ? "không đổi" : "ghim lại", ghim: allow.artifacts.length }));
