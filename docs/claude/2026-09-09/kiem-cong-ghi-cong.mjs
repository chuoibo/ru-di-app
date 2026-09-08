/* Chạy lại điều kiện đóng của F31 trên máy dò HIỆN TẠI, không sửa product.
 *
 *     node docs/claude/2026-09-09/kiem-cong-ghi-cong.mjs
 *
 * Cùng phương pháp với probe của Codex (`docs/codex/2026-09-08/
 * review-delta-584-585-evidence/kiem-gioi-han-gate.mjs`): đọc file test như
 * văn bản, trích đúng các khai báo của máy dò rồi chạy chúng trong một context
 * mới. Bản này thêm `biDanh` và hai hằng mà máy dò mới cần; probe cũ dừng ở
 * «biDanh is not defined» vì nó liệt kê tên theo bản cũ.
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { relative } from "node:path";
import vm from "node:vm";

const app = new URL("../../../apps/mobile/", import.meta.url);
const ts = createRequire(new URL("package.json", app))("typescript");
const nguon = readFileSync(new URL("tests/rudi-anh-ghi-cong.test.mjs", app), "utf8");
const cay = ts.createSourceFile("gate.mjs", nguon, ts.ScriptTarget.Latest, true);

const HAM = ["tenImage", "biDanh", "timAnhTran"];
const HANG = ["ANH_DANH_MUC", "ANH_DANH_MUC_TRONG", "NHA_GIU_CHIA"];
const khaiBao = cay.statements
  .filter(
    (node) =>
      (ts.isFunctionDeclaration(node) && HAM.includes(node.name?.text)) ||
      (ts.isVariableStatement(node) && node.declarationList.declarations.some((d) => HANG.includes(d.name.getText(cay)))),
  )
  .map((node) => node.getText(cay))
  .join("\n");
assert.ok(khaiBao.includes("function timAnhTran"), "trích hụt máy dò");
assert.ok(khaiBao.includes("function biDanh"), "trích hụt bảng bí danh");
const timAnhTran = vm.runInNewContext(`${khaiBao}\ntimAnhTran`, { ts, relative, APP: "" });

const CA = [
  ["direct", `const A = () => <Image source={p.anh.source} />;`, true],
  ["optional-import-alias", `import { Image as X } from "expo-image";\nconst A = () => <X source={p.anh?.source ?? null} />;`, true],
  ["source-variable", `const src = noi.anh.source; const A = () => <Image source={src} />;`, true],
  ["object-alias", `const picture = noi.anh;\nconst src = picture.source;\nconst A = () => <Image source={src} />;`, true],
  ["destructure", `const { source } = noi.anh;\nconst A = () => <Image source={source} />;`, true],
  ["mo-khoa-ngoai", `const A = () => <Image source={noi.anh.ve().source} />;`, true],
  ["ve-khung-bo-ghi-cong", `const ve = veKhung(n, { hong });\nconst A = () => <Image source={ve.source} />;`, true],
  ["ve-khung-tach-bo-ghi-cong", `const { source } = veKhung(n, { hong });\nconst A = () => <Image source={source} />;`, true],
  ["mo-khoa-bien", `const mo = noi.anh.ve;\nconst A = () => <Image source={mo().source} />;`, true],
  ["mo-khoa-ngoac", `const A = () => <Image source={noi.anh["ve"]().source} />;`, true],
  ["ve-khung-du-doi", `const ve = veKhung(n, { hong });\nconst A = () => <><Image source={ve.source} /><Text>{ve.ghiCong}</Text></>;`, false],
  ["asset-thuong", `const A = () => <Image source={demoAssets.wood} />;`, false],
];

let hong = 0;
for (const [ten, mau, mongDoi] of CA) {
  const thay = timAnhTran(mau, "synthetic.tsx");
  const batDuoc = thay.length > 0;
  if (batDuoc !== mongDoi) hong += 1;
  console.log(JSON.stringify({ ca: ten, batDuoc, mongDoi, dat: batDuoc === mongDoi, so: thay.length }));
}
console.log(JSON.stringify({ bo_qua_nguyen_file: /KHUNG_IN_GHI_CONG\.has\(rel\)\)\s*continue/.test(nguon), kiem_chuoi_cauGhiCong: nguon.includes('text.includes("cauGhiCong(")') }));
process.exit(hong === 0 ? 0 : 1);
