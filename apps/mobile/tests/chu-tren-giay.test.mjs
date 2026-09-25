/* Dark `paper` is not a text surface (ADR-0037 D14).
 *
 * Measured on the dark `paper` (#2e335c): accent 4.12:1, warn 4.00:1,
 * inkFaint 4.37:1 -- all under the 4.5:1 floor for text. Light `paper` is white
 * and passes everything, which is exactly why nobody saw it: every review ran
 * in light mode first. Two surfaces already ship the mistake -- the «cần kiểm»
 * line in `warn` on the bill's paper receipt, and the coral stamp on the
 * notebook's `ToGiay` sheet.
 *
 * The rule the redesign follows: objects that carry text fill with `card`;
 * where a sheet must be `paper` (the letter), the text on it is limited to
 * ink / inkSoft / split / ai, and a stamp on it is the filled `variant="ink"`.
 *
 * The check is an AST walk, per file: a JSX element whose style paints
 * `backgroundColor: colors.paper`, or a `ToGiay` sheet (under any local name it
 * was imported as), is a paper container; inside its JSX subtree it flags
 *   - a `Text` coloured `colors.accent`, `colors.warn` or `colors.inkFaint`,
 *   - a `Stamp` in the accent tone (explicit or default) that is not `variant="ink"`,
 *   - a `RudiButton` in the accent tone drawn as outline, ghost or soft.
 * It cannot see through a component boundary; that is what the debt list and
 * the evidence captures are for. The debt list only shrinks: an entry that no
 * longer occurs fails the test until it is deleted, so a fix is recorded here.
 */
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";

const SRC = fileURLToPath(new URL("../src/rudi", import.meta.url));

/** Known offenders at the start of ADR-0037, each fixed in its slice. `file#tag` keys. */
const NO_DA_BIET = new Set([
  "screens/hai-nguoi/ToLoiRu.tsx#Stamp",
  "hanh-trinh/ManHinhHanhTrinh.tsx#Text",
  "hanh-trinh/ManHinhHanhTrinh.tsx#RudiButton",
]);

function sourceFiles(dir) {
  const out = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...sourceFiles(p));
    else if (entry.name.endsWith(".tsx")) out.push(p);
  }
  return out.sort();
}

const TO_GIAY_MODULE = /\/ToGiay$/;
const MAU_CAM = /\bcolor:\s*[^,}]*\bcolors\.(accent|warn|inkFaint)\b/;
const NEN_GIAY = /\bbackgroundColor:\s*colors\.paper\b/;

function tenThe(node) {
  const tag = ts.isJsxElement(node) ? node.openingElement.tagName : node.tagName;
  return tag.getText();
}

function thuocTinh(node, ten) {
  const attrs = (ts.isJsxElement(node) ? node.openingElement : node).attributes.properties;
  for (const a of attrs) {
    if (ts.isJsxAttribute(a) && a.name.getText() === ten) return a;
  }
  return undefined;
}

function giaTriChuoi(attr) {
  if (!attr || !attr.initializer) return undefined;
  if (ts.isStringLiteral(attr.initializer)) return attr.initializer.text;
  if (ts.isJsxExpression(attr.initializer) && attr.initializer.expression && ts.isStringLiteral(attr.initializer.expression)) {
    return attr.initializer.expression.text;
  }
  return null; // an expression: unknown at scan time
}

/** Local names under which `ToGiay` (the sheet) was imported in this file. */
function tenToGiay(sf) {
  const ten = new Set();
  for (const st of sf.statements) {
    if (!ts.isImportDeclaration(st) || !ts.isStringLiteral(st.moduleSpecifier)) continue;
    if (!TO_GIAY_MODULE.test(st.moduleSpecifier.text)) continue;
    const named = st.importClause?.namedBindings;
    if (named && ts.isNamedImports(named)) {
      for (const el of named.elements) {
        if ((el.propertyName ?? el.name).getText() === "ToGiay") ten.add(el.name.getText());
      }
    }
  }
  return ten;
}

function laViPham(node) {
  const tag = tenThe(node);
  if (tag === "Text") {
    const style = thuocTinh(node, "style");
    return Boolean(style && MAU_CAM.test(style.getText()));
  }
  if (tag === "Stamp") {
    const tone = giaTriChuoi(thuocTinh(node, "tone"));
    const variant = giaTriChuoi(thuocTinh(node, "variant"));
    const toneCam = tone === undefined || tone === "accent" || tone === null;
    return toneCam && variant !== "ink";
  }
  if (tag === "RudiButton") {
    const tone = giaTriChuoi(thuocTinh(node, "tone"));
    const variant = giaTriChuoi(thuocTinh(node, "variant"));
    const toneCam = tone === undefined || tone === "accent";
    return toneCam && (variant === "outline" || variant === "ghost" || variant === "soft" || variant === null);
  }
  return false;
}

/** Offenders inside paper containers, as `{tag, line}`. */
export function quetChuTrenGiay(text, fileName) {
  const sf = ts.createSourceFile(fileName, text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const toGiay = tenToGiay(sf);
  const out = [];
  const laHopGiay = (node) => {
    if (!ts.isJsxElement(node)) return false;
    const tag = tenThe(node);
    if (toGiay.has(tag)) return true;
    const style = thuocTinh(node, "style");
    return Boolean(style && NEN_GIAY.test(style.getText()));
  };
  const quetTrong = (node) => {
    if ((ts.isJsxElement(node) || ts.isJsxSelfClosingElement(node)) && laViPham(node)) {
      out.push({ tag: tenThe(node), line: sf.getLineAndCharacterOfPosition(node.getStart()).line + 1 });
    }
    ts.forEachChild(node, quetTrong);
  };
  const tim = (node) => {
    if (laHopGiay(node)) {
      for (const child of node.children) quetTrong(child);
      return;
    }
    ts.forEachChild(node, tim);
  };
  tim(sf);
  return out;
}

test("the scanner is not blind: it finds the offender in a fixture and passes the clean one", () => {
  const xau = `
    import { ToGiay as To } from "../../ui/ToGiay";
    export function A() {
      return (<>
        <View style={[s.a, { backgroundColor: colors.paper }]}>
          <Text style={[typography.caption, { color: canKiem ? colors.warn : colors.inkSoft }]}>cần kiểm</Text>
          <Text style={[typography.caption, { color: colors.ink }]}>ổn</Text>
        </View>
        <To><Stamp label="Bản phác" /></To>
        <To><Stamp label="Bản phác" variant="ink" /></To>
        <View style={{ backgroundColor: colors.card }}><Text style={{ color: colors.warn }}>ổn trên card</Text></View>
      </>);
    }`;
  const found = quetChuTrenGiay(xau, "fixture.tsx");
  assert.deepEqual(found.map((f) => f.tag), ["Text", "Stamp"]);
});

test("no accent, warn or faint text on a paper surface, outside the shrinking debt list", () => {
  const seen = new Set();
  const moi = [];
  for (const file of sourceFiles(SRC)) {
    const rel = relative(SRC, file).split("\\").join("/");
    for (const f of quetChuTrenGiay(readFileSync(file, "utf8"), file)) {
      const key = `${rel}#${f.tag}`;
      seen.add(key);
      if (!NO_DA_BIET.has(key)) moi.push(`${rel}:${f.line} ${f.tag}`);
    }
  }
  assert.deepEqual(moi, [], `chữ cam/cảnh báo/mờ trên nền paper (tối chỉ ~4:1):\n${moi.join("\n")}`);
  const daSua = [...NO_DA_BIET].filter((k) => !seen.has(k));
  assert.deepEqual(daSua, [], `đã sửa, hãy xoá khỏi NO_DA_BIET: ${daSua.join(", ")}`);
});
