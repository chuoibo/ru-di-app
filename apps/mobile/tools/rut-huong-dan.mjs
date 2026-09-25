#!/usr/bin/env node
/**
 * Pulls the map of the app out of its own source: which screens exist, which
 * words are printed on them, and which screens each one can open.
 *
 * Why this exists. Nếp answers «how do I do X, what do I tap?» from the app
 * manual in `services/core/internal/huongdan/data/*.md`. A manual written by
 * hand drifts the day a button is renamed, and a drifted manual is worse than
 * none: it sends a person looking for a button that is not there. So the
 * manual is checked against THIS file, and this file is regenerated from the
 * code on every test run (`tests/huong-dan-khop-ma.test.mjs`). A rename then
 * turns CI red instead of turning Nếp into a liar.
 *
 * What is read, and why only that:
 *   - routes: every file under `app/`, minus `_layout`, the `[...legacy]`
 *     catch-all and `dev/`, named the way expo-router names them
 *     (`(tabs)/plan.tsx` -> `plan`, `outings/[id]/index.tsx` -> `outings/[id]`).
 *     These ids are the same strings a screen puts in `PhieuNguCanh.man`.
 *   - the route file itself plus the files it imports, ONE level deep, that
 *     live under `src/**\/screens/**`. One level on purpose: the screen a route
 *     mounts is what the route "is"; following every import would hand each
 *     route the whole kit and every list would read the same.
 *   - labels: the literal values of `label`, `accessibilityLabel`, `title` and
 *     `placeholder`, and literal JSX text. A template literal is kept with each
 *     `${...}` written as «…» (`Rủ ${ten} tới đây` -> `Rủ … tới đây`), which is
 *     how the manual quotes a label that carries a name.
 *   - navigation: `router.push/replace/navigate(...)`, `href=...` and an
 *     object's `href:` property, whose target is literal enough to name a
 *     route. `/outings/${id}?ctx=...` is cut at the query and matched against
 *     the route tree, so it records `outings/[id]`. A target no route matches
 *     keeps its path with a leading `/`, so it stays visible instead of
 *     silently dropping out.
 *
 * Parsing is TypeScript's own parser (already a dev dependency), not regex:
 * comments are trivia rather than nodes, so a label mentioned only in a
 * docstring never counts as being on screen.
 *
 * Output is deterministic: sorted keys, sorted arrays, 2-space indent,
 * trailing newline.
 *
 * The same run writes `src/rudi/nep/huong-dan-ban.ts`: the first 12 hex
 * characters of the sha256 of the exact `_rut.json` bytes. The server embeds
 * `_rut.json` and computes the same value (`huongdan.BanDung()`), so a phiếu
 * that carries the app's constant says which build of the map the app was made
 * with, and Nếp can tell when its manual describes a different app. Hashing
 * `_rut.json` rather than the manuals is deliberate: the map changes when the
 * code does, which is what makes two builds different screens.
 *
 * `--check` exits 1 when either committed file is stale.
 */
import { createHash } from "node:crypto";
import { existsSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { dirname, join, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";

export const GOC_MOBILE = fileURLToPath(new URL("..", import.meta.url));
const GOC_REPO = resolve(GOC_MOBILE, "../..");
export const DUONG_RUT = join(GOC_REPO, "services/core/internal/huongdan/data/_rut.json");
export const DUONG_BAN = join(GOC_MOBILE, "src/rudi/nep/huong-dan-ban.ts");

const THU_MUC_APP = join(GOC_MOBILE, "app");
const THU_MUC_SRC = join(GOC_MOBILE, "src");

/** JSX attributes whose value is words a person reads (or hears). */
const THUOC_TINH_NHAN = new Set(["label", "accessibilityLabel", "title", "placeholder"]);
/** How a `${...}` inside a template is written in labels and manuals. */
export const CHO_TRONG = "…";
/** Placeholder for a dynamic piece of a navigation target, before matching. */
const PH = "\u0000";

function posix(p) {
  return p.split(sep).join("/");
}

function moiTep(thuMuc) {
  const ra = [];
  for (const muc of readdirSync(thuMuc, { withFileTypes: true })) {
    const duong = join(thuMuc, muc.name);
    if (muc.isDirectory()) ra.push(...moiTep(duong));
    else if (/\.tsx?$/.test(muc.name)) ra.push(duong);
  }
  return ra.sort();
}

function docNguon(duong) {
  const chu = readFileSync(duong, "utf8");
  return ts.createSourceFile(duong, chu, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
}

/** Collapse runs of whitespace the way JSX renders them, and trim. */
function gon(s) {
  return s.replace(/\s+/g, " ").trim();
}

function coChu(s) {
  return /\p{L}/u.test(s);
}

/** `app/(tabs)/plan.tsx` -> `plan`; null when the file is not a screen route. */
export function maMan(duongTuongDoi) {
  const doan = duongTuongDoi.replace(/\.tsx?$/, "").split("/");
  if (doan[0] === "dev") return null;
  const cuoi = doan[doan.length - 1];
  if (cuoi === "_layout" || cuoi === "[...legacy]") return null;
  const con = doan.filter((d) => !/^\(.*\)$/.test(d));
  if (con[con.length - 1] === "index") con.pop();
  return con.length === 0 ? "index" : con.join("/");
}

/** Relative imports of a route file that land on a screen file under `src/`. */
function tepManDuocNhap(duongTuyetDoi, sf) {
  const ra = [];
  for (const cau of sf.statements) {
    const laNhap = ts.isImportDeclaration(cau) && !cau.importClause?.isTypeOnly;
    const laXuat = ts.isExportDeclaration(cau) && !cau.isTypeOnly;
    if (!laNhap && !laXuat) continue;
    const nguon = cau.moduleSpecifier;
    if (!nguon || !ts.isStringLiteral(nguon) || !nguon.text.startsWith(".")) continue;
    const goc = resolve(dirname(duongTuyetDoi), nguon.text);
    const thu = [goc, `${goc}.tsx`, `${goc}.ts`, join(goc, "index.tsx"), join(goc, "index.ts")];
    const thay = thu.find((p) => /\.tsx?$/.test(p) && existsSync(p));
    if (!thay) continue;
    const tuSrc = posix(relative(THU_MUC_SRC, thay));
    if (tuSrc.startsWith("..") || !tuSrc.split("/").includes("screens")) continue;
    ra.push(thay);
  }
  return ra;
}

/** Every string a label-carrying expression can evaluate to, templates as patterns. */
function chuCuaBieuThuc(e) {
  if (!e) return [];
  if (ts.isStringLiteral(e) || ts.isNoSubstitutionTemplateLiteral(e)) return [e.text];
  if (ts.isTemplateExpression(e)) {
    return [e.head.text + e.templateSpans.map((s) => CHO_TRONG + s.literal.text).join("")];
  }
  if (ts.isParenthesizedExpression(e) || ts.isAsExpression(e) || ts.isNonNullExpression(e)) {
    return chuCuaBieuThuc(e.expression);
  }
  if (ts.isConditionalExpression(e)) return [...chuCuaBieuThuc(e.whenTrue), ...chuCuaBieuThuc(e.whenFalse)];
  if (ts.isBinaryExpression(e)) {
    const k = e.operatorToken.kind;
    if (k === ts.SyntaxKind.QuestionQuestionToken || k === ts.SyntaxKind.BarBarToken) {
      return [...chuCuaBieuThuc(e.left), ...chuCuaBieuThuc(e.right)];
    }
    if (k === ts.SyntaxKind.AmpersandAmpersandToken) return chuCuaBieuThuc(e.right);
  }
  return [];
}

/**
 * The text of a JSX element whose children mix words and `{...}`, as one
 * pattern: `<Text>Ừ, hẹn {ngay}</Text>` -> `Ừ, hẹn …`. A `{" "}` child is the
 * literal it spells. Null when the children are not only text and expressions.
 */
function mauConJsx(node) {
  let co = false;
  let coBieuThuc = false;
  let chu = "";
  for (const con of node.children) {
    if (ts.isJsxText(con)) {
      chu += con.text;
      if (con.text.trim()) co = true;
    } else if (ts.isJsxExpression(con)) {
      if (!con.expression) continue;
      const e = con.expression;
      if (ts.isStringLiteral(e) || ts.isNoSubstitutionTemplateLiteral(e)) chu += e.text;
      else {
        chu += CHO_TRONG;
        coBieuThuc = true;
      }
    } else return null;
  }
  return co && coBieuThuc ? gon(chu) : null;
}

function nhanTrongTep(sf) {
  const ra = new Set();
  const them = (s) => {
    const g = gon(s);
    if (g && coChu(g)) ra.add(g);
  };
  const di = (node) => {
    if (ts.isJsxAttribute(node) && ts.isIdentifier(node.name) && THUOC_TINH_NHAN.has(node.name.text)) {
      const v = node.initializer;
      if (v && ts.isStringLiteral(v)) them(v.text);
      else if (v && ts.isJsxExpression(v)) for (const s of chuCuaBieuThuc(v.expression)) them(s);
    } else if (ts.isJsxText(node)) {
      them(node.text);
    } else if (ts.isJsxElement(node) || ts.isJsxFragment(node)) {
      const mau = mauConJsx(node);
      if (mau) them(mau);
    }
    ts.forEachChild(node, di);
  };
  di(sf);
  return ra;
}

/** Candidate targets of a navigation argument; PH marks a dynamic piece. Null = fully dynamic. */
function dichCuaBieuThuc(e) {
  if (!e) return null;
  if (ts.isStringLiteral(e) || ts.isNoSubstitutionTemplateLiteral(e)) return [e.text];
  if (ts.isTemplateExpression(e)) return [e.head.text + e.templateSpans.map((s) => PH + s.literal.text).join("")];
  if (
    ts.isParenthesizedExpression(e) ||
    ts.isAsExpression(e) ||
    ts.isNonNullExpression(e) ||
    ts.isTypeAssertionExpression(e) ||
    (ts.isSatisfiesExpression && ts.isSatisfiesExpression(e))
  ) {
    return dichCuaBieuThuc(e.expression);
  }
  if (ts.isConditionalExpression(e)) {
    const a = dichCuaBieuThuc(e.whenTrue) ?? [];
    const b = dichCuaBieuThuc(e.whenFalse) ?? [];
    return a.length + b.length ? [...a, ...b] : null;
  }
  if (ts.isBinaryExpression(e)) {
    const k = e.operatorToken.kind;
    if (k === ts.SyntaxKind.PlusToken) {
      const trai = dichCuaBieuThuc(e.left);
      const phai = dichCuaBieuThuc(e.right);
      if (!trai && !phai) return null;
      const ra = [];
      for (const a of trai ?? [PH]) for (const b of phai ?? [PH]) ra.push(a + b);
      return ra;
    }
    if (k === ts.SyntaxKind.QuestionQuestionToken || k === ts.SyntaxKind.BarBarToken) {
      const a = dichCuaBieuThuc(e.left) ?? [];
      const b = dichCuaBieuThuc(e.right) ?? [];
      return a.length + b.length ? [...a, ...b] : null;
    }
  }
  if (ts.isObjectLiteralExpression(e)) {
    for (const p of e.properties) {
      if (ts.isPropertyAssignment(p) && ts.isIdentifier(p.name) && p.name.text === "pathname") {
        return dichCuaBieuThuc(p.initializer);
      }
    }
  }
  return null;
}

function dichTrongTep(sf) {
  const ra = [];
  const di = (node) => {
    if (
      ts.isCallExpression(node) &&
      ts.isPropertyAccessExpression(node.expression) &&
      ts.isIdentifier(node.expression.expression) &&
      node.expression.expression.text === "router" &&
      ["push", "replace", "navigate"].includes(node.expression.name.text)
    ) {
      ra.push(...(dichCuaBieuThuc(node.arguments[0]) ?? []));
    } else if (ts.isJsxAttribute(node) && ts.isIdentifier(node.name) && node.name.text === "href") {
      const v = node.initializer;
      if (v && ts.isStringLiteral(v)) ra.push(v.text);
      else if (v && ts.isJsxExpression(v)) ra.push(...(dichCuaBieuThuc(v.expression) ?? []));
    } else if (
      ts.isPropertyAssignment(node) &&
      (ts.isIdentifier(node.name) || ts.isStringLiteral(node.name)) &&
      node.name.text === "href"
    ) {
      // A menu written as data (`{ title, href: "/outings/new" }`) and pushed
      // through a variable: the literal lives here, not at the call.
      ra.push(...(dichCuaBieuThuc(node.initializer) ?? []));
    }
    ts.forEachChild(node, di);
  };
  di(sf);
  return ra;
}

/**
 * A raw target (`/groups/<PH>/to-giay?ru=1`) to a route id (`groups/[id]/to-giay`).
 * Static segments beat dynamic ones, as in expo-router: `/outings/new` is
 * `outings/new`, not `outings/[id]`. Null for anything that is not an in-app path.
 */
export function manDich(tho, cacMan) {
  const cat = tho.split(/[?#]/)[0];
  if (!cat.startsWith("/")) return null;
  const doan = cat
    .split("/")
    .filter((d) => d !== "" && !/^\(.*\)$/.test(d))
    .map((d) => (d.includes(PH) ? "*" : d));
  if (doan.length === 0) return cacMan.includes("index") ? "index" : "/";
  let tot = null;
  let diemTot = -1;
  for (const man of cacMan) {
    const md = man === "index" ? [] : man.split("/");
    if (md.length !== doan.length) continue;
    let diem = 0;
    let khop = true;
    for (let i = 0; i < md.length; i++) {
      if (/^\[.*\]$/.test(md[i])) continue;
      if (md[i] !== doan[i]) {
        khop = false;
        break;
      }
      diem++;
    }
    if (khop && (diem > diemTot || (diem === diemTot && man < tot))) {
      tot = man;
      diemTot = diem;
    }
  }
  return tot ?? `/${doan.join("/")}`;
}

/** JSON with keys sorted at every depth, 2-space indent, trailing newline. */
function jsonOnDinh(v) {
  const sap = (x) => {
    if (Array.isArray(x)) return x.map(sap);
    if (x && typeof x === "object") {
      return Object.fromEntries(
        Object.keys(x)
          .sort()
          .map((k) => [k, sap(x[k])]),
      );
    }
    return x;
  };
  return `${JSON.stringify(sap(v), null, 2)}\n`;
}

/** The whole map, as the object that `_rut.json` holds. */
export function rutBanDo() {
  const tepMan = moiTep(THU_MUC_APP)
    .map((p) => ({ p, man: maMan(posix(relative(THU_MUC_APP, p))) }))
    .filter((x) => x.man !== null);
  const cacMan = [...new Set(tepMan.map((x) => x.man))].sort();
  const theoMan = new Map(cacMan.map((m) => [m, { man: m, tep: new Set(), nhan: new Set(), di_toi: new Set() }]));
  for (const { p, man } of tepMan) {
    const muc = theoMan.get(man);
    const sf = docNguon(p);
    for (const tep of [p, ...tepManDuocNhap(p, sf)]) {
      const sfTep = tep === p ? sf : docNguon(tep);
      muc.tep.add(posix(relative(GOC_MOBILE, tep)));
      for (const n of nhanTrongTep(sfTep)) muc.nhan.add(n);
      for (const d of dichTrongTep(sfTep)) {
        const den = manDich(d, cacMan);
        if (den !== null) muc.di_toi.add(den);
      }
    }
  }
  const routes = cacMan.map((m) => {
    const muc = theoMan.get(m);
    return { di_toi: [...muc.di_toi].sort(), man: m, nhan: [...muc.nhan].sort(), tep: [...muc.tep].sort() };
  });
  return { routes };
}

/** The exact bytes `_rut.json` must hold. */
export function chuoiRut() {
  return jsonOnDinh(rutBanDo());
}

/** First 12 hex characters of the sha256 of `_rut.json` as UTF-8: what `huongdan.BanDung()` returns. */
export function banDung(rut) {
  return createHash("sha256").update(rut, "utf8").digest("hex").slice(0, 12);
}

/** The exact text `huong-dan-ban.ts` must hold for a given `_rut.json`. */
export function chuoiBan(rut) {
  return [
    "/**",
    " * Which build of the app map this app was made with: the first 12 hex",
    " * characters of the sha256 of services/core/internal/huongdan/data/_rut.json.",
    " *",
    " * Written by `tools/rut-huong-dan.mjs` together with `_rut.json`; do not edit.",
    " * The server computes the same value from the copy it embeds",
    " * (`huongdan.BanDung()`), so a phiếu carrying it tells Nếp whether its manual",
    " * describes the app this person holds. `tests/huong-dan-khop-ma.test.mjs` and",
    " * a Go test in `services/core/internal/huongdan` turn a stale value red.",
    " */",
    `export const HUONG_DAN_BAN = "${banDung(rut)}";`,
    "",
  ].join("\n");
}

/**
 * Every piece of text one source file can put in front of a person: string
 * literals, templates as `…` patterns, JSX text, and mixed JSX children as one
 * pattern. Comments are not nodes, so they contribute nothing -- which is the
 * point: a label that survives only in a docstring is not on any screen.
 */
export function literalTuNguon(chu, ten = "nguon.tsx") {
  const ra = new Set();
  const them = (s) => {
    const g = gon(s);
    if (g) ra.add(g);
  };
  const di = (node) => {
    if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) them(node.text);
    else if (ts.isTemplateExpression(node)) {
      them(node.head.text + node.templateSpans.map((s) => CHO_TRONG + s.literal.text).join(""));
    } else if (ts.isJsxText(node)) them(node.text);
    else if (ts.isJsxElement(node) || ts.isJsxFragment(node)) {
      const mau = mauConJsx(node);
      if (mau) them(mau);
    }
    ts.forEachChild(node, di);
  };
  di(ts.createSourceFile(ten, chu, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX));
  return ra;
}

/**
 * `literalTuNguon` over every `.ts`/`.tsx` under `src/` and `app/`, minus
 * `app/dev/`: the UI lab is not a screen anyone reaches, so a label that lives
 * only there is not a button the manual may send a person to.
 */
export function literalTrongMa() {
  const ra = new Set();
  const dev = join(THU_MUC_APP, "dev") + sep;
  for (const thuMuc of [THU_MUC_SRC, THU_MUC_APP]) {
    for (const p of moiTep(thuMuc)) {
      if (p.startsWith(dev)) continue;
      for (const s of literalTuNguon(readFileSync(p, "utf8"), p)) ra.add(s);
    }
  }
  return ra;
}

/** Labels and route targets of one source text, the way `rutBanDo` reads a file. */
export function rutTuNguon(chu, cacMan, ten = "nguon.tsx") {
  const sf = ts.createSourceFile(ten, chu, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const diToi = new Set();
  for (const d of dichTrongTep(sf)) {
    const den = manDich(d, cacMan);
    if (den !== null) diToi.add(den);
  }
  return { nhan: [...nhanTrongTep(sf)].sort(), di_toi: [...diToi].sort() };
}

const laChinh = process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url);
if (laChinh) {
  const moi = chuoiRut();
  const banMoi = chuoiBan(moi);
  if (process.argv.includes("--check")) {
    let cu = true;
    for (const [duong, noiDung] of [
      [DUONG_RUT, moi],
      [DUONG_BAN, banMoi],
    ]) {
      if ((existsSync(duong) ? readFileSync(duong, "utf8") : null) !== noiDung) {
        console.error(`${posix(relative(GOC_REPO, duong))} is stale: run \`node tools/rut-huong-dan.mjs\` in apps/mobile.`);
        cu = false;
      }
    }
    if (!cu) process.exit(1);
    console.log(`_rut.json and huong-dan-ban.ts are fresh (${banDung(moi)})`);
  } else {
    writeFileSync(DUONG_RUT, moi);
    writeFileSync(DUONG_BAN, banMoi);
    const { routes } = JSON.parse(moi);
    console.log(`wrote ${posix(relative(GOC_REPO, DUONG_RUT))}: ${routes.length} routes`);
    console.log(`wrote ${posix(relative(GOC_REPO, DUONG_BAN))}: ${banDung(moi)}`);
  }
}
