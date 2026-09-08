/* A photograph never travels without its credit (ADR-0017 §2.5; review 08/09
 * vòng 2, F21).
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/rudi-anh-ghi-cong.test.mjs
 *
 * The fixture type kept the credit and three screens still lost it, because
 * each of them read `.anh.source` and handed the bare address to an `Image`.
 * A type that carries metadata does not prove a renderer prints it, so this
 * test reads the consumers: outside the few frames that print the credit,
 * no `<Image>` in the shell may receive a catalogue photograph. The frames
 * themselves are named here, and a new one has to be added here on purpose.
 */
import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

import { TIEN_TO_MINH_HOA, cauGhiCong } from "../dist-test/rudi/ui/ghi-cong.js";

const APP = fileURLToPath(new URL("..", import.meta.url));
const GOC = [join(APP, "src/rudi"), join(APP, "app")];

/** The frames allowed to draw a catalogue photograph, because each prints its credit. */
const KHUNG_IN_GHI_CONG = new Set([
  "src/rudi/ui/MediaSlot.tsx",
  "src/rudi/screens/keo/HangChang.tsx",
  "src/rudi/screens/explore/HangDiaDiem.tsx",
]);

/** An expression that names a catalogue photograph: `x.anh.source`, `anh.source`, `dd.photo`, `.photo`. */
const ANH_DANH_MUC = /(\banh\.source\b|\.photo\b)/;

function sourceFiles(dir) {
  const out = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...sourceFiles(p));
    else if (/\.tsx$/.test(entry.name)) out.push(p);
  }
  return out.sort();
}

/**
 * Every `<Image … source={X}>` whose `X` names a catalogue photograph, and
 * every mention of the old `AnhChang` component, with their lines.
 */
function timAnhTran(text, fileName) {
  const sf = ts.createSourceFile(fileName, text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const viPham = [];
  const dong = (node) => sf.getLineAndCharacterOfPosition(node.getStart(sf)).line + 1;
  const walk = (node) => {
    if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
      const ten = node.tagName.getText(sf);
      if (ten === "Image") {
        for (const attr of node.attributes.properties) {
          if (ts.isJsxAttribute(attr) && attr.name.getText(sf) === "source" && attr.initializer) {
            const bieuThuc = attr.initializer.getText(sf);
            if (ANH_DANH_MUC.test(bieuThuc)) viPham.push({ line: dong(node), loai: "Image", bieuThuc });
          }
        }
      }
      if (ten === "AnhChang") viPham.push({ line: dong(node), loai: "AnhChang", bieuThuc: ten });
    }
    if (ts.isIdentifier(node) && node.text === "AnhChang" && ts.isImportSpecifier(node.parent)) {
      viPham.push({ line: dong(node), loai: "import AnhChang", bieuThuc: node.text });
    }
    ts.forEachChild(node, walk);
  };
  walk(sf);
  return viPham;
}

test("máy dò không mù: một Image nhận ảnh danh mục trần và một import AnhChang đều bị bắt", () => {
  const mau = [
    `import { AnhChang } from "./keo/HangChang";`,
    `const A = () => <Image accessibilityLabel={p.name} source={p.anh.source} />;`,
    `const B = () => <Image source={dd.photo} style={s} />;`,
    `const C = () => <AnhChang alt="x" source={p.anh.source} />;`,
    `const D = () => <Image source={demoAssets.wood} />;`,
  ].join("\n");
  const thay = timAnhTran(mau, "mau.tsx");
  assert.deepEqual(thay.map((v) => v.loai), ["import AnhChang", "Image", "Image", "AnhChang"]);
  assert.equal(thay.filter((v) => v.bieuThuc.includes("wood")).length, 0, "một chất liệu không phải ảnh danh mục");
});

test("ngoài các khung in ghi công, không Image nào trong vỏ nhận ảnh danh mục, và AnhChang không còn ai gọi", () => {
  const loi = [];
  let daQuet = 0;
  for (const goc of GOC) {
    for (const file of sourceFiles(goc)) {
      daQuet += 1;
      const rel = relative(APP, file);
      if (KHUNG_IN_GHI_CONG.has(rel)) continue;
      for (const v of timAnhTran(readFileSync(file, "utf8"), rel)) loi.push(`${rel}:${v.line} ${v.loai} ${v.bieuThuc}`);
    }
  }
  assert.ok(daQuet > 40, `quét quá ít file (${daQuet}): gốc quét sai`);
  assert.deepEqual(loi, [], "ảnh danh mục tới Image mà không qua khung in ghi công:\n" + loi.join("\n"));
});

test("các khung được phép vẫn tồn tại và vẫn gọi cauGhiCong", () => {
  for (const rel of KHUNG_IN_GHI_CONG) {
    const text = readFileSync(join(APP, rel), "utf8");
    assert.ok(text.includes("cauGhiCong("), `${rel} không còn in ghi công: bỏ khỏi danh sách hoặc in lại`);
  }
});

test("câu ghi công của ảnh minh hoạ mở bằng tiền tố và nêu tác giả · giấy phép", () => {
  const cau = cauGhiCong({ prefix: TIEN_TO_MINH_HOA, author: "Kien Tran", license: "Pexels License" });
  assert.equal(cau, "Ảnh minh hoạ: Kien Tran · Pexels License");
  assert.ok(cau.startsWith(TIEN_TO_MINH_HOA));
  assert.equal(cauGhiCong({ author: "A", license: "B", source: "C" }), "A · B · C");
  assert.ok(!cau.includes("—"));
});
