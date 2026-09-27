/* Build the coverage matrix and the counts of the report from the ledger.
 *
 *   AUDIT_OUT=… node tong-hop.mjs <docs-dir>
 *
 * Writes <docs-dir>/coverage-matrix.md (committed), and AUDIT_OUT/coverage-matrix.csv
 * and AUDIT_OUT/dem.json (outside git; the CSV is sent to the requester, since
 * the repository guard does not admit .csv files).
 *
 * The LAST record per (test case, platform, configuration group) wins. Counts
 * include every row, BLOCKED and NOT_TESTED too: a rate computed without them
 * would be a prettier number about a smaller audit.
 */
import { existsSync, writeFileSync } from "node:fs";
import { join } from "node:path";

import { soGhi } from "./thu-vien/ghi.mjs";

const out = process.env.AUDIT_OUT;
const docs = process.argv[2];
if (!out || !docs) throw new Error("dùng: AUDIT_OUT=… node tong-hop.mjs <thư mục docs>");

const cuoi = new Map();
// A withdrawal drops every EARLIER row of its test case; later rows count again.
const daRut = new Map();
for (const r of soGhi(out).doc()) {
  if (r.rut) {
    let so = 0;
    for (const k of [...cuoi.keys()]) if (k.startsWith(`${r.tc}|`)) (cuoi.delete(k), so++);
    daRut.set(r.tc, { lyDo: r.lyDo, so: (daRut.get(r.tc)?.so ?? 0) + so });
    continue;
  }
  cuoi.set(`${r.tc}|${r.nenTang ?? "web"}|${r.cauHinh ?? ""}`, r);
}
// A seeded NOT_TESTED row is a placeholder for its test case on its platform:
// once that case has a real verdict under any configuration group, the
// placeholder goes. Any other NOT_TESTED row stays and is counted.
const coKetQua = new Set([...cuoi.values()].filter((r) => r.status !== "NOT_TESTED").map((r) => `${r.tc}|${r.nenTang ?? "web"}`));
for (const [k, r] of cuoi) if (r.status === "NOT_TESTED" && coKetQua.has(`${r.tc}|${r.nenTang ?? "web"}`)) cuoi.delete(k);
const hang = [...cuoi.values()].sort((a, b) => (a.feature + a.tc + (a.nenTang ?? "")).localeCompare(b.feature + b.tc + (b.nenTang ?? ""), "vi"));

const TT = ["PASS", "FAIL", "BLOCKED", "NOT_TESTED", "NOT_APPLICABLE"];
const dem = (loc) => Object.fromEntries(TT.map((t) => [t, hang.filter((r) => r.status === t && loc(r)).length]));
const tong = {
  tatCa: dem(() => true),
  web: dem((r) => (r.nenTang ?? "web") === "web"),
  android: dem((r) => r.nenTang === "android"),
  ios: dem((r) => r.nenTang === "ios"),
  runtime: dem((r) => r.method === "RUNTIME-WEB"),
  static: dem((r) => r.method === "STATIC"),
  hypothesis: dem((r) => r.method === "HYPOTHESIS"),
  theoFeature: Object.fromEntries([...new Set(hang.map((r) => r.feature))].sort().map((f) => [f, dem((r) => r.feature === f)])),
  soHang: hang.length,
};
writeFileSync(join(out, "dem.json"), JSON.stringify(tong, null, 1));

// Notes carry measurements; a raw float (a pixel coordinate with seven decimals) reads as a
// run of ten digits to the repository guard long-number rule (it blocked checkpoint 2).
// Round every decimal to at most one place before it reaches markdown.
const lamTron = (s) => String(s ?? "").replace(/\d+\.\d{2,}/g, (m) => String(Math.round(Number(m) * 10) / 10));
const o = (s) => lamTron(s).replace(/\|/g, "\\|").replace(/\n/g, " ");
// Only committed images get a link; the rest are named and marked as kept
// outside git, so the matrix never carries a dead link.
const ev = (r) =>
  (r.evidence ?? []).map((e) => (existsSync(join(docs, "evidence", `${e}.jpg`)) ? `[${e}](evidence/${e}.jpg)` : `${e} (ngoài git)`)).join(" ");
const dongDem = (d) => TT.map((t) => `${t} ${d[t]}`).join(" · ");
let md = `# Coverage matrix: audit UI/UX app mobile RuDi

Sinh bởi \`tests/qa/mobile-ui-audit/tong-hop.mjs\` từ sổ \`results.jsonl\`; không sửa tay. Mỗi hàng là một
(test case × nền tảng × nhóm cấu hình). Cấu hình C1–C9 ở \`report.md\` §A. Method: RUNTIME-WEB là đã chạy
trên bản web trong Chromium; STATIC là chỉ đọc mã; HYPOTHESIS là nghi vấn chưa kiểm chứng. PASS chỉ có
ở hàng RUNTIME-WEB.

## Đếm

| Phạm vi | Đếm |
|---|---|
| Tất cả (${tong.soHang} hàng) | ${dongDem(tong.tatCa)} |
| Web (Chromium) | ${dongDem(tong.web)} |
| Android native | ${dongDem(tong.android)} |
| iOS native | ${dongDem(tong.ios)} |
| Method RUNTIME-WEB | ${dongDem(tong.runtime)} |
| Method STATIC | ${dongDem(tong.static)} |
| Method HYPOTHESIS | ${dongDem(tong.hypothesis)} |

`;
for (const f of Object.keys(tong.theoFeature)) {
  md += `## ${f}\n\nĐếm: ${dongDem(tong.theoFeature[f])}\n\n`;
  md += "| ID | Feature | Screen | Layer | State | Action | Platform/config | Expected | Status | Method | Evidence | Issue |\n|---|---|---|---|---|---|---|---|---|---|---|---|\n";
  for (const r of hang.filter((x) => x.feature === f)) {
    const nen = `${r.nenTang ?? "web"}${r.cauHinh && r.cauHinh !== "-" ? ` ${r.cauHinh}` : ""}`;
    const ghiChu = r.ghiChu ? ` (${o(r.ghiChu)})` : "";
    md += `| ${o(r.tc)} | ${o(r.feature)} | ${o(r.screen)} | ${o(r.layer)} | ${o(r.state)} | ${o(r.action)} | ${o(nen)} | ${o(r.expected)} | ${r.status}${ghiChu} | ${r.method} | ${ev(r)} | ${o(r.issue ?? "")} |\n`;
  }
  md += "\n";
}
if (daRut.size) {
  md += "## Hàng đã rút\n\nHàng do lỗi của harness (sai selector, sai tên nút) được rút khỏi bảng trên; sổ vẫn giữ nguyên dòng gốc.\n\n| ID | Số hàng rút | Lý do |\n|---|---|---|\n";
  for (const [tc, v] of daRut) md += `| ${o(tc)} | ${v.so} | ${o(v.lyDo)} |\n`;
  md += "\n";
}
writeFileSync(join(docs, "coverage-matrix.md"), md);

const csvO = (s) => `"${String(s ?? "").replace(/"/g, '""')}"`;
const csv = [["ID", "Feature", "Screen", "Layer", "State", "Action", "Platform", "Config", "Expected", "Status", "Method", "Evidence", "Issue", "Note"].join(",")]
  .concat(hang.map((r) => [r.tc, r.feature, r.screen, r.layer, r.state, r.action, r.nenTang ?? "web", r.cauHinh, r.expected, r.status, r.method, (r.evidence ?? []).join(" "), r.issue ?? "", r.ghiChu ?? ""].map(csvO).join(",")))
  .join("\n");
writeFileSync(join(out, "coverage-matrix.csv"), `${csv}\n`);
console.log(JSON.stringify({ soHang: tong.soHang, tatCa: tong.tatCa }));
