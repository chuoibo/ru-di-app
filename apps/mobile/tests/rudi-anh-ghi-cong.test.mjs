/* A photograph never travels without its credit (ADR-0017 §2.5; review 08/09
 * vòng 2 F21, review delta 08/09 F31).
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/rudi-anh-ghi-cong.test.mjs
 *
 * Two layers, in this order, because the first one is the one that actually
 * holds:
 *
 * 1. THE TYPE. `AnhCoGhiCong` keeps its address in a closure and hands it out
 *    only beside the sentence, and `veKhung` is the single decision every frame
 *    renders. So «draw the picture, drop the words» is not a shape a screen can
 *    write: it is a compile error, checked on every build of every file. What
 *    this file adds is the part tsc cannot state -- that the decision itself is
 *    right for every input, and that a version of it which forgot the credit
 *    would be caught.
 * 2. THE SCAN. A backstop for what types cannot see: an `any` cast, a future
 *    author reintroducing a `{ source, nguon }` pair, or a frame that computes
 *    the decision and then renders half of it. The first version of this scan
 *    exempted three files whole and then only checked that the string
 *    `cauGhiCong(` appeared somewhere in them, comments included; Codex's probe
 *    of 08/09 showed two ordinary refactors that walked straight past it. No
 *    file is exempt here, and both of those probes are in the fixture below.
 */
import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

import {
  CAU_ANH_HONG,
  TIEN_TO_MINH_HOA,
  anhDanhMuc,
  cauGhiCong,
  khoaNguon,
  veKhung,
} from "../dist-test/rudi/ui/ghi-cong.js";

const APP = fileURLToPath(new URL("..", import.meta.url));
const GOC = [join(APP, "src/rudi"), join(APP, "app")];

/** The one module allowed to open a catalogue photograph; everyone else goes through `veKhung`. */
const NHA_GIU_CHIA = "src/rudi/ui/ghi-cong.ts";

/**
 * An expression that names a catalogue photograph the old way: `x.anh`,
 * `x.anh?`, `dd.photo`. The current type has no such property, so nothing in
 * the tree matches; the rule stands so that reintroducing the split pair is
 * caught the day it is written rather than the day a screen loses its credit.
 */
const ANH_DANH_MUC = /(\banh\??$|\.photo$)/;
const ANH_DANH_MUC_TRONG = /(\banh\??\.source\b|\.photo\b)/;

function sourceFiles(dir) {
  const out = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...sourceFiles(p));
    else if (/\.tsx?$/.test(entry.name)) out.push(p);
  }
  return out.sort();
}

/** Local names bound to an image component: `Image` from expo-image or react-native, under any alias. */
function tenImage(sf) {
  const ten = new Set(["Image"]);
  for (const st of sf.statements) {
    if (!ts.isImportDeclaration(st) || !ts.isStringLiteral(st.moduleSpecifier)) continue;
    if (!["expo-image", "react-native"].includes(st.moduleSpecifier.text)) continue;
    const bindings = st.importClause?.namedBindings;
    if (bindings && ts.isNamedImports(bindings)) {
      for (const el of bindings.elements) {
        const goc = (el.propertyName ?? el.name).text;
        if (goc === "Image") ten.add(el.name.text);
      }
    }
  }
  return ten;
}

/**
 * Names bound to a catalogue photograph (`const p = noi.anh`) and names bound
 * to a bare address taken out of one (`const src = p.source`, `const { source }
 * = noi.anh`), plus the names bound to a `veKhung(...)` decision.
 *
 * Collected in a first pass over the whole file, because `const p = noi.anh`
 * and the `<Image>` that uses it are not in the same subtree and the second one
 * is often written first.
 */
function biDanh(sf) {
  const doiTuong = new Set();
  const diaChi = new Set();
  // Names destructured out of a `veKhung(...)` result, so the rule below can
  // ask whether the picture was drawn while the sentence was left behind.
  const raSource = new Set();
  const raGhiCong = new Set();
  const laAnh = (node) => {
    const t = node.getText(sf);
    return ANH_DANH_MUC.test(t) || doiTuong.has(t);
  };
  // Two sweeps: `const a = noi.anh; const b = a; const src = b.source` needs
  // the alias set to be complete before the address set is read off it.
  for (let vong = 0; vong < 2; vong += 1) {
    const walk = (node) => {
      if (ts.isVariableDeclaration(node) && node.initializer) {
        const init = node.initializer;
        if (ts.isIdentifier(node.name)) {
          if (laAnh(init)) doiTuong.add(node.name.text);
          if (ts.isPropertyAccessExpression(init) && init.name.text === "source" && laAnh(init.expression)) {
            diaChi.add(node.name.text);
          }
        } else if (ts.isObjectBindingPattern(node.name)) {
          const laVeKhung = ts.isCallExpression(init) && init.expression.getText(sf).endsWith("veKhung");
          for (const el of node.name.elements) {
            const goc = (el.propertyName ?? el.name).getText(sf);
            const ten = el.name.getText(sf);
            if (goc === "source" && laAnh(init)) diaChi.add(ten);
            if (laVeKhung && goc === "source") raSource.add(ten);
            if (laVeKhung && goc === "ghiCong") raGhiCong.add(ten);
          }
        }
      }
      ts.forEachChild(node, walk);
    };
    walk(sf);
  }
  return { doiTuong, diaChi, raSource, raGhiCong };
}

/**
 * Everything that separates a catalogue picture from its words, in one file.
 *
 * `<Image source={…}>` fed a catalogue address under any import alias; a bare
 * read of `.source` off a catalogue object, whether spelled out or reached
 * through a local name; a destructured `source`; a call of `ve()` outside the
 * module that owns the key; a `veKhung` decision whose picture is rendered
 * while its sentence is not; and any surviving mention of the retired
 * `AnhChang` component.
 */
function timAnhTran(text, fileName) {
  const kind = /\.tsx$/.test(fileName) ? ts.ScriptKind.TSX : ts.ScriptKind.TS;
  const sf = ts.createSourceFile(fileName, text, ts.ScriptTarget.Latest, true, kind);
  const image = tenImage(sf);
  const { doiTuong, diaChi, raSource, raGhiCong } = biDanh(sf);
  // `AnhChang` is private to the file that declares it; the rule is about any
  // OTHER file reaching for it, which is what its export once allowed.
  const tuKhaiAnhChang = /function AnhChang\b/.test(text);
  const viPham = [];
  let daXet = 0;
  const dong = (node) => sf.getLineAndCharacterOfPosition(node.getStart(sf)).line + 1;
  const laAnh = (node) => ANH_DANH_MUC.test(node.getText(sf)) || doiTuong.has(node.getText(sf));
  const dungQuyetDinh = new Map(); // veKhung binding -> fields read
  const them = (node, loai, bieuThuc) => viPham.push({ line: dong(node), loai, bieuThuc });
  const walk = (node) => {
    daXet += 1;
    if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
      const ten = node.tagName.getText(sf);
      if (image.has(ten)) {
        for (const attr of node.attributes.properties) {
          if (ts.isJsxAttribute(attr) && attr.name.getText(sf) === "source" && attr.initializer) {
            const bieuThuc = attr.initializer.getText(sf);
            const tran = bieuThuc.replace(/[{}\s]/g, "");
            if (ANH_DANH_MUC_TRONG.test(bieuThuc) || diaChi.has(tran)) them(node, "Image", bieuThuc);
          }
        }
      }
      if (ten === "AnhChang" && !tuKhaiAnhChang) them(node, "AnhChang", ten);
    }
    if (ts.isPropertyAccessExpression(node)) {
      // The address read off a catalogue object, spelled out or via a local name.
      if (node.name.text === "source" && laAnh(node.expression)) {
        them(node, "anh.source", node.getText(sf));
      }
      // Which halves of a `veKhung` decision this file actually renders.
      const da = dungQuyetDinh.get(node.expression.getText(sf));
      if (da !== undefined) da.add(node.name.text);
      // The key itself, however it is spelled: `x.anh.ve()`, `const f = x.anh.ve`
      // and `x.anh["ve"]()` are the same act, and only the first was caught.
      // Unconditional on the receiver, like the bracket rule below: a prop
      // typed `AnhCoGhiCong` is not spelled `…anh`, and narrowing this to the
      // spelling let `buc.ve()` through (finish review 09/09, R1).
      if (node.name.text === "ve" && relative(APP, fileName) !== NHA_GIU_CHIA) {
        them(node, "mở ảnh ngoài ghi-cong", node.getText(sf).slice(0, 60));
      }
    }
    if (ts.isVariableDeclaration(node) && node.initializer) {
      const init = node.initializer;
      if (ts.isCallExpression(init) && init.expression.getText(sf).endsWith("veKhung") && ts.isIdentifier(node.name)) {
        dungQuyetDinh.set(node.name.text, new Set());
      }
      if (ts.isObjectBindingPattern(node.name)) {
        for (const el of node.name.elements) {
          const goc = (el.propertyName ?? el.name).getText(sf);
          if (goc === "source" && laAnh(init)) them(node, "tách source", node.getText(sf));
        }
      }
    }
    // ... including the bracket spelling, which no property-access rule sees.
    if (
      ts.isElementAccessExpression(node) &&
      ts.isStringLiteral(node.argumentExpression) &&
      node.argumentExpression.text === "ve" &&
      relative(APP, fileName) !== NHA_GIU_CHIA
    ) {
      them(node, "mở ảnh ngoài ghi-cong", node.getText(sf).slice(0, 60));
    }
    if (ts.isIdentifier(node) && node.text === "AnhChang" && ts.isImportSpecifier(node.parent)) {
      them(node, "import AnhChang", node.text);
    }
    ts.forEachChild(node, walk);
  };
  walk(sf);
  // A frame that computed the decision and then drew only the picture. This is
  // the check the file-level exemption used to hide.
  for (const [ten, dung] of dungQuyetDinh) {
    if (dung.has("source") && !dung.has("ghiCong")) {
      viPham.push({ line: 0, loai: "vẽ ảnh bỏ ghi công", bieuThuc: `${ten}.source không đi cùng ${ten}.ghiCong` });
    }
  }
  // The same fault written the other way: `const { source } = veKhung(...)`
  // with no `ghiCong` beside it. This is the obvious refactor of a frame, and
  // the first version of this rule only understood the named-binding spelling.
  if (raSource.size > 0 && raGhiCong.size === 0) {
    viPham.push({ line: 0, loai: "vẽ ảnh bỏ ghi công", bieuThuc: `tách {${[...raSource].join(", ")}} khỏi veKhung mà không lấy ghiCong` });
  }
  return Object.assign(viPham, { daXet });
}

test("máy dò không mù: mọi cách tách địa chỉ khỏi ghi công đều bị bắt, kể cả hai mẫu probe của Codex", () => {
  const mau = [
    `import { Image as ExpoImage } from "expo-image";`,
    `import { AnhChang } from "./keo/HangChang";`,
    `const A = () => <Image accessibilityLabel={p.name} source={p.anh.source} />;`,
    `const B = () => <Image source={dd.photo} style={s} />;`,
    `const C = () => <AnhChang alt="x" source={p.anh.source} />;`,
    `const D = () => <Image source={demoAssets.wood} />;`,
    `const E = () => <ExpoImage source={place.anh?.source ?? null} />;`,
    `const src = noi.anh.source; const F = () => <Image source={src} />;`,
    // The two shapes Codex's probe walked past on 08/09.
    `const picture = noi.anh; const src2 = picture.source; const G = () => <Image source={src2} />;`,
    `const { source } = noi.anh; const H = () => <Image source={source} />;`,
    // Turning the key outside the module that owns it, in all three spellings.
    `const I = () => <Image source={noi.anh.ve().source} />;`,
    `const mo = noi.anh.ve; const J = () => <Image source={mo().source} />;`,
    `const K = () => <Image source={noi.anh["ve"]().source} />;`,
    // A frame that takes the value as a prop: not spelled «anh», same act.
    `function L({ buc }) { const { source } = buc.ve(); return <Image source={source} />; }`,
  ].join("\n");
  const thay = timAnhTran(mau, "mau.tsx");
  const loai = thay.map((v) => v.loai);
  // Every line above that separates a picture from its words is named at least once.
  assert.ok(loai.includes("import AnhChang"));
  assert.ok(loai.includes("AnhChang"));
  assert.ok(loai.filter((l) => l === "Image").length >= 5, `Image: ${loai.join(",")}`);
  assert.ok(loai.includes("anh.source"));
  assert.ok(loai.includes("tách source"), "destructure phải bị bắt (probe Codex mẫu 2)");
  assert.equal(loai.filter((l) => l === "mở ảnh ngoài ghi-cong").length, 4, "mọi cách viết đều phải bị bắt, kể cả khi người nhận không tên là «anh»");
  assert.equal(thay.filter((v) => v.bieuThuc.includes("wood")).length, 0, "một chất liệu không phải ảnh danh mục");
  // Object alias: `picture.source` where `picture = noi.anh` (probe Codex mẫu 1).
  assert.ok(
    thay.some((v) => v.loai === "anh.source" && v.bieuThuc === "picture.source"),
    "bí danh object phải bị bắt (probe Codex mẫu 1)",
  );
  // And the two `<Image>` fed from those locals.
  assert.ok(thay.some((v) => v.loai === "Image" && v.bieuThuc.includes("src2")));
  assert.ok(thay.some((v) => v.loai === "Image" && v.bieuThuc.includes("source")));
});

test("máy dò bắt được khung vẽ ảnh mà bỏ câu ghi công", () => {
  const xau = `const ve = veKhung(nguon, { hong });\nconst A = () => <Image source={ve.source} />;`;
  const tot = `const ve = veKhung(nguon, { hong });\nconst A = () => <><Image source={ve.source} /><Text>{ve.ghiCong}</Text></>;`;
  const xauTach = `const { source } = veKhung(nguon, { hong });\nconst A = () => <Image source={source} />;`;
  const totTach = `const { source, ghiCong } = veKhung(nguon, { hong });\nconst A = () => <><Image source={source} /><Text>{ghiCong}</Text></>;`;
  assert.ok(timAnhTran(xau, "xau.tsx").some((v) => v.loai === "vẽ ảnh bỏ ghi công"), "phải bắt");
  assert.ok(timAnhTran(xauTach, "xau2.tsx").some((v) => v.loai === "vẽ ảnh bỏ ghi công"), "bản tách cũng phải bắt");
  assert.deepEqual(timAnhTran(tot, "tot.tsx").filter((v) => v.loai === "vẽ ảnh bỏ ghi công"), [], "không được báo nhầm");
  assert.deepEqual(timAnhTran(totTach, "tot2.tsx").filter((v) => v.loai === "vẽ ảnh bỏ ghi công"), [], "không được báo nhầm");
});

test("không file nào trong vỏ tách ảnh danh mục khỏi ghi công, và AnhChang không còn ai gọi", () => {
  const loi = [];
  let daQuet = 0;
  let daXet = 0;
  for (const goc of GOC) {
    for (const file of sourceFiles(goc)) {
      daQuet += 1;
      const rel = relative(APP, file);
      const thay = timAnhTran(readFileSync(file, "utf8"), rel);
      daXet += thay.daXet;
      for (const v of thay) loi.push(`${rel}:${v.line} ${v.loai} ${v.bieuThuc}`);
    }
  }
  // Two floors, so «không có vi phạm» can never be produced by a walk that read
  // nothing: that is how a source gate dies without a sound.
  assert.ok(daQuet >= 130, `quét quá ít file (${daQuet}): gốc quét sai`);
  assert.ok(daXet >= 100000, `duyệt quá ít nút (${daXet}): máy dò dừng sớm`);
  assert.deepEqual(loi, [], "ảnh danh mục bị tách khỏi ghi công:\n" + loi.join("\n"));
});

test("veKhung: ảnh danh mục không bao giờ vẽ ra mà thiếu câu ghi công", () => {
  const anh = anhDanhMuc({ uri: "https://x/y.jpg" }, { prefix: TIEN_TO_MINH_HOA, author: "Kien Tran", license: "Pexels License" });
  const nhom = { uri: "https://x/nhom.jpg" };
  const dauVao = [
    null,
    { loai: "danh-muc", anh },
    { loai: "nhom", source: nhom },
  ];
  for (const nguon of dauVao) {
    for (const hong of [false, true]) {
      const ve = veKhung(nguon, { hong });
      if (nguon !== null && nguon.loai === "danh-muc") {
        // The invariant, over every input: a licensed picture and its sentence
        // are one decision, and the sentence stays even when the bytes failed.
        assert.equal(typeof ve.ghiCong, "string");
        assert.ok(ve.ghiCong.startsWith(TIEN_TO_MINH_HOA));
      } else {
        assert.equal(ve.ghiCong, null, "ảnh của nhóm không bịa giấy phép");
      }
      assert.equal(ve.source === null, nguon === null || hong);
      assert.equal(ve.canhBao, nguon !== null && hong ? CAU_ANH_HONG : null);
    }
  }
});

test("đột biến: một veKhung quên ghi công phải làm bất biến trên đỏ", () => {
  const veKhungQuen = (nguon, o) =>
    nguon === null || nguon.loai === "nhom"
      ? { source: nguon === null || o.hong ? null : nguon.source, ghiCong: null, canhBao: null }
      : { source: o.hong ? null : nguon.anh.ve().source, ghiCong: null, canhBao: null };
  const anh = anhDanhMuc({ uri: "u" }, { author: "A", license: "B" });
  const ve = veKhungQuen({ loai: "danh-muc", anh }, { hong: false });
  assert.throws(() => assert.equal(typeof ve.ghiCong, "string"), "bất biến phải bắt được bản quên");
});

test("ảnh danh mục không có cửa nào ra địa chỉ ngoài ve()", () => {
  const anh = anhDanhMuc({ uri: "u" }, { author: "A", license: "B" });
  assert.deepEqual(Object.keys(anh), ["ve"]);
  assert.equal(anh.source, undefined);
  assert.equal(anh.nguon, undefined);
  // The same object every call, so a frame may key an effect on it.
  assert.equal(anh.ve(), anh.ve());
  assert.deepEqual(anh.ve(), { source: { uri: "u" }, ghiCong: "A · B" });
});

test("khoá nguồn ổn định và phân biệt được hai ảnh", () => {
  const a = anhDanhMuc({ uri: "u1" }, { author: "A", license: "B" });
  const b = anhDanhMuc({ uri: "u2" }, { author: "A", license: "B" });
  assert.equal(khoaNguon({ loai: "danh-muc", anh: a }), khoaNguon({ loai: "danh-muc", anh: a }));
  assert.notEqual(khoaNguon({ loai: "danh-muc", anh: a }), khoaNguon({ loai: "danh-muc", anh: b }));
  assert.equal(khoaNguon(null), "");
  assert.equal(khoaNguon({ loai: "nhom", source: { uri: "u1" } }), khoaNguon({ loai: "danh-muc", anh: a }));
});

test("câu ghi công của ảnh minh hoạ mở bằng tiền tố và nêu tác giả · giấy phép", () => {
  const cau = cauGhiCong({ prefix: TIEN_TO_MINH_HOA, author: "Kien Tran", license: "Pexels License" });
  assert.equal(cau, "Ảnh minh hoạ: Kien Tran · Pexels License");
  assert.ok(cau.startsWith(TIEN_TO_MINH_HOA));
  assert.equal(cauGhiCong({ author: "A", license: "B", source: "C" }), "A · B · C");
  assert.ok(!cau.includes("—"));
});
