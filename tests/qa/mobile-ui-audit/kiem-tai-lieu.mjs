/* Cross-checks the audit documents before a checkpoint commit.
 *
 *   node kiem-tai-lieu.mjs <docs-dir>            # identity: must print XANH
 *   node kiem-tai-lieu.mjs <docs-dir> --canary   # every canary must go red
 *
 * 1. Every file in <docs-dir>/evidence is pinned in .repo-guard-allowlist.json
 *    with the same sha256, the controlled-artifact rule and a reason; no pin
 *    names a missing file.
 * 2. Every `](evidence/…)` link in the .md files resolves, and every image is
 *    linked from somewhere other than evidence-manifest.md: an issue, the
 *    report, or a verdict row of coverage-matrix.md. The manifest lists every
 *    file in the folder by construction, so it cannot witness that an image
 *    backs anything (counting it made this check vacuous until checkpoint 10).
 * 3. The per-severity table in issues.md and the per-category table in
 *    report.md list exactly the issues whose entries carry that severity and
 *    category, and the issue count stated in report.md matches.
 *
 * Canaries mutate the loaded inputs in memory only; nothing on disk changes.
 */
import { readFileSync, readdirSync, existsSync } from "node:fs";
import { createHash } from "node:crypto";
import { join, resolve, relative, dirname, sep } from "node:path";

const LOAI = ["BUG", "UX ISSUE", "VISUAL POLISH"];

function timGocRepo(tu) {
  for (let d = resolve(tu); ; d = dirname(d)) {
    if (existsSync(join(d, ".repo-guard-allowlist.json"))) return d;
    if (dirname(d) === d) throw new Error(`không thấy .repo-guard-allowlist.json phía trên ${tu}`);
  }
}

function napDauVao(docs) {
  const goc = timGocRepo(docs);
  const rel = relative(goc, resolve(docs)).split(sep).join("/");
  const ev = join(docs, "evidence");
  const anh = readdirSync(ev).sort().map((ten) => {
    const buf = readFileSync(join(ev, ten));
    return { ten, co: buf.length, sha: createHash("sha256").update(buf).digest("hex") };
  });
  const ghim = JSON.parse(readFileSync(join(goc, ".repo-guard-allowlist.json"), "utf8")).artifacts
    .filter((a) => a.path.startsWith(`${rel}/evidence/`))
    .map((a) => ({ ...a }));
  const md = Object.fromEntries(readdirSync(docs).filter((f) => f.endsWith(".md")).map((f) => [f, readFileSync(join(docs, f), "utf8")]));
  return { goc, rel, anh, ghim, md };
}

function kiem({ goc, rel, anh, ghim, md }) {
  const sai = [];
  const ghi = [];

  // 1. pins
  const theoDuong = new Map(ghim.map((a) => [a.path, a]));
  for (const f of anh) {
    const a = theoDuong.get(`${rel}/evidence/${f.ten}`);
    if (!a) sai.push(`chưa ghim: ${f.ten}`);
    else {
      if (a.sha256 !== f.sha) sai.push(`sha256 lệch: ${f.ten}`);
      if (!a.rules?.includes("controlled-artifact")) sai.push(`thiếu rule controlled-artifact: ${f.ten}`);
      if (!a.reason || a.reason.length < 20) sai.push(`thiếu reason: ${f.ten}`);
    }
  }
  for (const a of ghim) if (!existsSync(join(goc, a.path))) sai.push(`ghim không có file: ${a.path}`);
  ghi.push(`ảnh: ${anh.length}, ghim: ${ghim.length}, tổng ${(anh.reduce((s, f) => s + f.co, 0) / 1048576).toFixed(2)} MiB`);

  // 2. links
  const coFile = new Set(anh.map((f) => f.ten));
  const duocDan = new Set();
  let soLink = 0;
  for (const [f, s] of Object.entries(md)) {
    for (const m of s.matchAll(/\]\((evidence\/[^)\s]+)\)/g)) {
      soLink++;
      const t = m[1].slice("evidence/".length);
      if (!coFile.has(t)) sai.push(`link hỏng trong ${f}: ${m[1]}`);
      if (f !== "evidence-manifest.md") duocDan.add(t);
    }
  }
  for (const f of anh) if (!duocDan.has(f.ten)) sai.push(`ảnh chỉ có manifest dẫn tới: ${f.ten}`);
  ghi.push(`link ảnh: ${soLink}; ảnh được dẫn tới ngoài manifest: ${[...duocDan].filter((t) => coFile.has(t)).length}/${anh.length}`);

  // 3. issue tables
  const iss = md["issues.md"] ?? "";
  const muc = new Map();
  for (const m of iss.matchAll(/^### (UI-\d{3}) ·[^\n]*\n([\s\S]*?)(?=^### |^## |(?![\s\S]))/gm)) {
    const d = m[2].match(/^\| Category \/ Severity \| ([^|]*)\|/m);
    if (!d) { sai.push(`không có Category / Severity: ${m[1]}`); continue; }
    const o = d[1].trim();
    const cat = LOAI.find((l) => o.startsWith(l)) ?? null;
    const sev = o.match(/\*\*(P[0-3])\*\*/)?.[1] ?? null;
    if (!cat || !sev) sai.push(`không đọc được loại/mức: ${m[1]}`);
    if (muc.has(m[1])) sai.push(`issue trùng: ${m[1]}`);
    muc.set(m[1], { cat, sev });
  }
  // Numbers run without a gap from the lowest one: UI-001 for the audit of
  // 7ea1a7c, UI-123 for its follow-up on main, which carries only new issues.
  const so = [...muc.keys()].map((k) => Number(k.slice(3)));
  const dau = so.length ? Math.min(...so) : 1;
  for (let i = dau; i < dau + muc.size; i++) {
    const id = `UI-${String(i).padStart(3, "0")}`;
    if (!muc.has(id)) sai.push(`thiếu số: ${id}`);
  }
  const dem = {};
  for (const { sev } of muc.values()) dem[sev] = (dem[sev] ?? 0) + 1;
  ghi.push(`issue: ${muc.size} (${Object.entries(dem).sort().map(([k, v]) => `${v} ${k}`).join(", ")})`);

  const tach = (o) => new Set((o ?? "").split(",").map((x) => x.trim()).filter(Boolean));
  const bangMuc = iss.match(/## Tóm tắt theo mức\n\n\| Mức \| Issue \|\n\|---\|---\|\n((?:\|[^\n]*\n)+)/);
  if (!bangMuc) sai.push("không thấy bảng theo mức trong issues.md");
  else {
    const thay = new Set();
    for (const dong of bangMuc[1].trim().split("\n")) {
      const [, sev, ds] = dong.split("|").map((x) => x.trim());
      const bang = tach(ds);
      const that = new Set([...muc].filter(([, v]) => v.sev === sev).map(([k]) => k));
      for (const k of bang) { thay.add(k); if (!that.has(k)) sai.push(`issues.md bảng mức ${sev} có ${k}, mục ghi ${muc.get(k)?.sev}`); }
      for (const k of that) if (!bang.has(k)) sai.push(`issues.md bảng mức ${sev} thiếu ${k}`);
    }
    if (thay.size !== muc.size) sai.push(`issues.md bảng mức có ${thay.size} issue, có ${muc.size} mục`);
  }
  const rep = md["report.md"] ?? "";
  const bangLoai = rep.match(/\| Mức \| BUG \| UX ISSUE \| VISUAL POLISH \|\n\|---\|---\|---\|---\|\n((?:\|[^\n]*\n)+)/);
  if (!bangLoai) sai.push("không thấy bảng theo loại trong report.md");
  else {
    const thay = new Set();
    for (const dong of bangLoai[1].trim().split("\n")) {
      const o = dong.split("|").map((x) => x.trim());
      LOAI.forEach((cat, j) => {
        const bang = tach(o[2 + j]);
        const that = new Set([...muc].filter(([, v]) => v.sev === o[1] && v.cat === cat).map(([k]) => k));
        for (const k of bang) { thay.add(k); if (!that.has(k)) sai.push(`report bảng ${o[1]}/${cat} có ${k}, mục ghi ${muc.get(k)?.sev}/${muc.get(k)?.cat}`); }
        for (const k of that) if (!bang.has(k)) sai.push(`report bảng ${o[1]}/${cat} thiếu ${k}`);
      });
    }
    if (thay.size !== muc.size) sai.push(`report bảng loại có ${thay.size} issue, issues.md có ${muc.size}`);
  }
  const soCau = rep.match(/(\d+) issue sau checkpoint/);
  if (!soCau || Number(soCau[1]) !== muc.size) sai.push(`report nói ${soCau?.[1]} issue, issues.md có ${muc.size}`);

  return { sai, ghi };
}

// Each canary breaks one thing the way a careless edit would, and names the
// message it must produce.
const CANARY = {
  sha: { doan: "sha256 lệch", lam: (v) => { v.ghim[0].sha256 = v.ghim[0].sha256.replace(/^./, (c) => (c === "0" ? "1" : "0")); } },
  // Drop the first issue named in the table, whatever the cell holds: one
  // issue or many (the follow-up on main has a single P2 at first).
  bang: {
    doan: "report bảng",
    lam: (v) => {
      v.md["report.md"] = v.md["report.md"].replace(/(\| Mức \| BUG \| UX ISSUE \| VISUAL POLISH \|\n\|---\|---\|---\|---\|\n(?:\|[^\n]*\n)*?\|[^\n]*?)(?:, )?UI-\d{3}(, )?/, (m, a, b) => a + (b && !a.endsWith("| ") ? b : ""));
    },
  },
  muc: { doan: "issues.md bảng mức", lam: (v) => { v.md["issues.md"] = v.md["issues.md"].replace(/(\n\| P[0-3] \| )UI-\d{3}(?:, )?/, "$1"); } },
  link: { doan: "link hỏng", lam: (v) => { v.md["issues.md"] += "\n![canary](evidence/EV-KHONG-CO.jpg)\n"; } },
  thua: {
    doan: "ảnh chỉ có manifest dẫn tới",
    lam: (v) => {
      // Drop every link to the first image except the manifest's own.
      const t = v.anh[0].ten.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
      for (const f of Object.keys(v.md)) if (f !== "evidence-manifest.md") v.md[f] = v.md[f].replace(new RegExp(`\\]\\(evidence/${t}\\)`, "g"), "]()");
    },
  },
};

const [docs, co] = process.argv.slice(2);
if (!docs) {
  console.error("dùng: node kiem-tai-lieu.mjs <docs-dir> [--canary]");
  process.exit(2);
}
const vao = napDauVao(docs);
const goc = kiem(vao);
for (const d of goc.ghi) console.log(d);
for (const s of goc.sai) console.log(`SAI  ${s}`);

if (co !== "--canary") {
  console.log(goc.sai.length ? `ĐỎ: ${goc.sai.length} lỗi` : "XANH: ghim, link, bảng issue khớp");
  process.exit(goc.sai.length ? 1 : 0);
}

let hong = goc.sai.length ? 1 : 0;
console.log(goc.sai.length ? "ĐỎ   identity" : "XANH identity");
for (const [ten, { doan, lam }] of Object.entries(CANARY)) {
  const v = structuredClone(vao);
  lam(v);
  const { sai } = kiem(v);
  const dung = sai.length > 0 && sai.every((s) => s.startsWith(doan));
  if (!dung) hong++;
  console.log(`${dung ? "ĐỎ ĐÚNG DỰ ĐOÁN" : "SAI DỰ ĐOÁN   "}  canary ${ten}: ${sai.length ? sai.join(" | ") : "vẫn xanh"}`);
}
console.log(hong ? `HỎNG: ${hong} phép kiểm không như dự đoán` : "canary: 5/5 đỏ đúng dự đoán, identity xanh");
process.exit(hong ? 1 : 0);
