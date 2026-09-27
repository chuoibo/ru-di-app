/* The Skia boundary (ADR-0037 D10).
 *
 * On the web, `@shopify/react-native-skia` builds its API from
 * `global.CanvasKit` the moment `Skia.web.js` is evaluated; on native, its
 * setup module throws when the dev client has no Skia. Either way, a single
 * static import of the library on an eager path turns «draw this stage» into
 * «the app does not start». So:
 *
 *   1. only files under `src/rudi/ui/skia/` import the library at all;
 *   2. everything else reaches those files through a dynamic `import()` (see
 *      `luoiSkia` in `ui/KhungSkia.tsx`), except the loader `nap-skia` and the
 *      pure shader sources `sksl`, which import nothing of Skia -- type-only
 *      imports are erased and allowed;
 *   3. the loader itself never imports the library statically;
 *   4. the 8 MB wasm is never committed (`.gitignore`) and the loader asks for
 *      it at `/canvaskit.wasm`, where `tools/chep-canvaskit.mjs` puts it;
 *   5. no source under `ui/skia/` carries a run of nine or more digits: SkSL
 *      noise constants are the classic place for one, and the repo guard reads
 *      it as an account number.
 *
 * The first test runs the scanner on a fixture so the gate cannot go blind.
 */
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync, readdirSync, existsSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";

const GOC = fileURLToPath(new URL("..", import.meta.url));
const SKIA_DIR = join(GOC, "src", "rudi", "ui", "skia");
const THU_VIEN = /^@shopify\/react-native-skia(\/|$)/;
const CHO_PHEP_TINH = new Set(["nap-skia", "sksl"]);

function sourceFiles(dir) {
  const out = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...sourceFiles(p));
    else if (/\.(tsx?|jsx?)$/.test(entry.name)) out.push(p);
  }
  return out.sort();
}

/** Static and dynamic module specifiers of one file. */
export function docImport(text, fileName) {
  const sf = ts.createSourceFile(fileName, text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const tinh = [];
  const dong = [];
  const walk = (node) => {
    if (ts.isImportDeclaration(node) && ts.isStringLiteral(node.moduleSpecifier)) {
      const chiKieu = node.importClause?.isTypeOnly === true;
      tinh.push({ spec: node.moduleSpecifier.text, chiKieu });
    } else if (ts.isExportDeclaration(node) && node.moduleSpecifier && ts.isStringLiteral(node.moduleSpecifier)) {
      tinh.push({ spec: node.moduleSpecifier.text, chiKieu: node.isTypeOnly === true });
    } else if (ts.isCallExpression(node) && node.expression.kind === ts.SyntaxKind.ImportKeyword) {
      const arg = node.arguments[0];
      if (arg && ts.isStringLiteral(arg)) dong.push(arg.text);
    }
    ts.forEachChild(node, walk);
  };
  walk(sf);
  return { tinh, dong };
}

function tenModuleSkia(fromFile, spec) {
  if (!spec.startsWith(".")) return null;
  const dich = resolve(dirname(fromFile), spec);
  const rel = relative(SKIA_DIR, dich);
  if (rel.startsWith("..") || rel === "") return null;
  return rel.split(/[\\/]/)[0].replace(/\.(native|web)$/, "").replace(/\.(tsx?|jsx?)$/, "");
}

/** Violations of rules 1-3 in one file. */
export function viPham(text, file) {
  const { tinh } = docImport(text, file);
  const trongSkia = !relative(SKIA_DIR, file).startsWith("..");
  const loi = [];
  for (const { spec, chiKieu } of tinh) {
    if (THU_VIEN.test(spec) && !trongSkia && !chiKieu) loi.push(`import tĩnh ${spec} ngoài ui/skia`);
    const ten = tenModuleSkia(file, spec);
    if (ten !== null && !trongSkia && !chiKieu && !CHO_PHEP_TINH.has(ten)) loi.push(`import tĩnh ui/skia/${ten} (phải import() lười)`);
  }
  if (trongSkia && /^nap-skia(\.native)?\.ts$/.test(file.split(/[\\/]/).pop())) {
    for (const { spec, chiKieu } of tinh) if (THU_VIEN.test(spec) && !chiKieu) loi.push(`bộ nạp import tĩnh ${spec}`);
  }
  return loi;
}

test("the scanner is not blind: a static import of a Skia renderer from a screen is caught", () => {
  const man = join(GOC, "src", "rudi", "screens", "Gia.tsx");
  const xau = `import { LopSkia } from "../ui/skia/VeSkia";\nimport { Canvas } from "@shopify/react-native-skia";\nexport const X = 1;`;
  assert.deepEqual(viPham(xau, man).length, 2);
  const sach = `import type { ThuRendererProps } from "../ui/skia/ThuRendererSkia";\nimport { napSkia } from "../ui/skia/nap-skia";\nconst L = () => import("../ui/skia/ThuRendererSkia");`;
  assert.deepEqual(viPham(sach, man), []);
  const boNap = join(SKIA_DIR, "nap-skia.ts");
  assert.equal(viPham(`import { Skia } from "@shopify/react-native-skia";`, boNap).length, 1);
});

test("Skia is imported only under ui/skia, and reached from outside only lazily", () => {
  const loi = [];
  for (const goc of [join(GOC, "src"), join(GOC, "app")]) {
    for (const file of sourceFiles(goc)) {
      for (const l of viPham(readFileSync(file, "utf8"), file)) loi.push(`${relative(GOC, file)}: ${l}`);
    }
  }
  assert.deepEqual(loi, []);
});

test("the wasm is never committed and the loader asks for it where the copy tool puts it", () => {
  const gitignore = readFileSync(join(GOC, ".gitignore"), "utf8");
  assert.match(gitignore, /^public\/canvaskit\.wasm$/m);
  const napWeb = readFileSync(join(SKIA_DIR, "nap-skia.ts"), "utf8");
  assert.match(napWeb, /locateFile: \(tep: string\) => `\/\$\{tep\}`/);
  const chep = readFileSync(join(GOC, "tools", "chep-canvaskit.mjs"), "utf8");
  assert.match(chep, /"public", "canvaskit\.wasm"/);
  const pkg = JSON.parse(readFileSync(join(GOC, "package.json"), "utf8"));
  assert.match(pkg.scripts["build:check"], /^node tools\/chep-canvaskit\.mjs && /, "web export phải có wasm trước khi đóng gói");
  assert.ok(existsSync(join(SKIA_DIR, "nap-skia.native.ts")), "native cần bộ dò riêng để dev client cũ không sập");
});

test("no run of nine or more digits in the Skia sources (SkSL constants included)", () => {
  for (const file of sourceFiles(SKIA_DIR)) {
    const m = readFileSync(file, "utf8").match(/\d{9,}/);
    assert.equal(m, null, `${relative(GOC, file)}: ${m?.[0]}`);
  }
});
