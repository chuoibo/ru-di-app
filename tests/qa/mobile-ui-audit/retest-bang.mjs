/* The retest table: every issue of the base audit, what main does with it.
 *
 *   AUDIT_OUT=<ledger of main> node retest-bang.mjs <docs of the base audit> <docs of main>
 *
 * Writes <docs of main>/retest.md from two sources only: the base issues.md
 * (title, category, severity of UI-001…) and the retest rows of the ledger
 * (`TC-R-UI-xxx`, one per issue or one per part, written by
 * kich-ban/retest-main.mjs and retest-phan-xu.mjs; `TC-M-…` for findings new
 * on main). The last row of each key wins and withdrawals apply, as in
 * tong-hop.mjs. A row's note starts with «còn», «hết» or «đổi» (after the
 * «phân xử bằng mắt:» prefix of a verdict reached by looking); the table keeps
 * that word next to the status, so «đổi» never hides whether the issue's own
 * criterion now holds.
 */
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

import { soGhi } from "./thu-vien/ghi.mjs";
import { lamTron } from "./thu-vien/lam-tron.mjs";

const out = process.env.AUDIT_OUT;
const [docsGoc, docs] = process.argv.slice(2);
if (!out || !docsGoc || !docs) throw new Error("dùng: AUDIT_OUT=… node retest-bang.mjs <docs gốc> <docs main>");

// ------------------------------------------------ the base issues
const goc = readFileSync(join(docsGoc, "issues.md"), "utf8");
const ISSUE = [];
for (const m of goc.matchAll(/^### (UI-\d{3}) · ([^\n]*)\n([\s\S]*?)(?=^### |^## |(?![\s\S]))/gm)) {
  const o = m[3].match(/^\| Category \/ Severity \| ([^|]*)\|/m)?.[1]?.trim() ?? "";
  ISSUE.push({ id: m[1], ten: m[2].trim(), loai: ["BUG", "UX ISSUE", "VISUAL POLISH"].find((l) => o.startsWith(l)) ?? "?", muc: o.match(/\*\*(P[0-3])\*\*/)?.[1] ?? "?" });
}

// ------------------------------------------------ the retest rows
const cuoi = new Map();
for (const r of soGhi(out).doc()) {
  if (r.rut) {
    for (const k of [...cuoi.keys()]) if (k.startsWith(`${r.tc}|`)) cuoi.delete(k);
    continue;
  }
  cuoi.set(`${r.tc}|${r.nenTang ?? "web"}|${r.cauHinh ?? ""}`, r);
}
const hangCua = new Map();
const hangMoi = [];
for (const r of cuoi.values()) {
  const m = /^TC-R-(UI-\d{3})(?:-(.+))?$/.exec(r.tc);
  if (m) hangCua.set(m[1], [...(hangCua.get(m[1]) ?? []), r]);
  else if (/^TC-M-/.test(r.tc)) hangMoi.push(r);
}

const bo = (s) => String(s ?? "").replace(/^phân xử bằng mắt: /, "");
const tu = (r) => bo(r.ghiChu).match(/^(còn|hết|đổi)/)?.[1] ?? "?";
/** One word for the issue, from its rows: a failing row wins over a passing one. */
const ketLuan = (tatCa) => {
  const rows = (tatCa ?? []).filter((r) => r.status !== "NOT_TESTED");
  if (!rows.length) return { chu: "chưa đo lại", nhom: "chua" };
  const fail = rows.filter((r) => r.status === "FAIL");
  const pass = rows.filter((r) => r.status === "PASS");
  if (fail.length) return fail.every((r) => tu(r) === "đổi") ? { chu: "đổi, vẫn trượt tiêu chí", nhom: "con" } : { chu: "còn", nhom: "con" };
  if (pass.length && pass.length === rows.filter((r) => r.status !== "BLOCKED").length) return pass.some((r) => tu(r) === "đổi") ? { chu: "đổi, đạt tiêu chí", nhom: "het" } : { chu: "hết", nhom: "het" };
  return { chu: "không đo được", nhom: "chan" };
};
const o = (s) => lamTron(s).replace(/\|/g, "\\|").replace(/\n/g, " ");
const cat = (s, n) => (s.length > n ? `${s.slice(0, n)}…` : s);
const anh = (r) => {
  const co = (r.evidence ?? []).filter((e) => existsSync(join(docs, "evidence", `${e}.jpg`)));
  return co.length ? co.map((e) => `[${e}](evidence/${e}.jpg)`).join(" ") : (r.evidence ?? []).length ? "ảnh ngoài git" : "-";
};
const dongHang = (r) => `\`${r.tc}\` ${r.cauHinh ?? ""} **${r.status}**`;

const dem = {};
for (const i of ISSUE) {
  const k = ketLuan(hangCua.get(i.id));
  dem[i.muc] ??= { con: 0, het: 0, chan: 0, chua: 0 };
  dem[i.muc][k.nhom]++;
}

let md = `# Retest trên main: từng issue của audit 7ea1a7c

Sinh bởi \`tests/qa/mobile-ui-audit/retest-bang.mjs\` từ \`issues.md\` của audit gốc
(\`docs/claude/2026-09-27/mobile-ui-audit/\`) và sổ retest của main; không sửa tay. Mỗi issue có một hàng
\`TC-R-UI-xxx\` (hoặc một hàng cho mỗi phần, như \`TC-R-UI-084-A\`, \`-B\`) mà tiêu chí chính là «Tiêu chí gỡ» của
issue: PASS là tiêu chí đạt trên main, FAIL là chưa. Chữ đầu ghi chú nói điều đã thấy: «còn» là lỗi y như cũ,
«hết» là lỗi không còn, «đổi» là màn đã đổi nên phải nói rõ tiêu chí đạt hay không. Hàng bắt đầu bằng
«phân xử bằng mắt» là kết luận sau khi mở ảnh ra xem.

Bản đo: main \`461eabf\`, bản web export trên stack cục bộ thứ hai (Postgres, API Python, Go core ở cổng khác stack
của audit gốc), Chromium 141, cấu hình C1–C9 như audit gốc. Mức và loại giữ nguyên như audit gốc.

## Tóm tắt

| Mức | Còn | Hết | Không đo được | Chưa đo lại |
|---|---|---|---|---|
${["P1", "P2", "P3"].map((m) => `| ${m} | ${dem[m]?.con ?? 0} | ${dem[m]?.het ?? 0} | ${dem[m]?.chan ?? 0} | ${dem[m]?.chua ?? 0} |`).join("\n")}

`;
for (const muc of ["P1", "P2", "P3"]) {
  const ds = ISSUE.filter((i) => i.muc === muc).sort((a, b) => a.id.localeCompare(b.id));
  const daDo = ds.filter((i) => ketLuan(hangCua.get(i.id)).nhom !== "chua");
  const chua = ds.filter((i) => ketLuan(hangCua.get(i.id)).nhom === "chua");
  md += `## ${muc} (${ds.length} issue)\n\n`;
  if (daDo.length) {
    md += "| Issue | Loại | Issue gốc | Trên main | Hàng | Bằng chứng | Ghi chú của hàng |\n|---|---|---|---|---|---|---|\n";
    for (const i of daDo) {
      // A NOT_TESTED placeholder goes once the issue has a measured row (as in tong-hop.mjs).
      const rows = hangCua.get(i.id).filter((r) => r.status !== "NOT_TESTED").sort((a, b) => a.tc.localeCompare(b.tc) || String(a.cauHinh).localeCompare(String(b.cauHinh)));
      const k = ketLuan(rows);
      md += `| ${i.id} | ${i.loai} | ${o(cat(i.ten, 110))} | **${k.chu}** | ${rows.map(dongHang).join("<br>")} | ${[...new Set(rows.map(anh))].join(" ")} | ${rows.map((r) => o(cat(bo(r.ghiChu), 260))).join("<br>")} |\n`;
    }
    md += "\n";
  }
  if (chua.length) md += `Chưa đo lại ở checkpoint này (${chua.length}): ${chua.map((i) => i.id).join(", ")}.\n\n`;
}
md += `## Phát hiện mới trên main

Không phải issue của audit gốc; chi tiết ở \`issues.md\` của thư mục này.

| Hàng | Trạng thái | Bằng chứng | Ghi chú |
|---|---|---|---|
${hangMoi.map((r) => `| \`${r.tc}\` ${r.cauHinh ?? ""} | **${r.status}** | ${anh(r)} | ${o(cat(String(r.ghiChu ?? ""), 400))} |`).join("\n")}
`;
writeFileSync(join(docs, "retest.md"), md);
console.log(JSON.stringify({ issue: ISSUE.length, daDo: ISSUE.filter((i) => ketLuan(hangCua.get(i.id)).nhom !== "chua").length, dem, moi: hangMoi.length }));
