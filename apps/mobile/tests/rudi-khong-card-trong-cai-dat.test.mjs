/**
 * Bề mặt Cài đặt là hàng + kẻ tóc trên giấy, không thẻ (DESIGN.md «Hàng + kẻ tóc
 * là container mặc định»; tái audit 10/09, R5: Cài đặt xếp tám `Card` trắng nổi
 * cạnh Cá nhân là hàng phẳng — hai hệ bề mặt cạnh nhau). Test quét NGUỒN: không
 * file nào trong `screens/cai-dat/` import `Card` từ kit; và in danh sách các màn
 * khác còn dùng `Card` để nợ không tàng hình (không đỏ vì chúng — ngoài phạm vi).
 */
import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";

const GOC = new URL("../src/rudi/screens/", import.meta.url).pathname;
const tsx = (dir) => readdirSync(dir).flatMap((f) => {
  const p = join(dir, f);
  return statSync(p).isDirectory() ? tsx(p) : p.endsWith(".tsx") ? [p] : [];
});
const importCard = (src) => /import\s*\{[^}]*\bCard\b[^}]*\}\s*from\s*"[^"]*\/ui"/.test(src);

test("không màn nào trong screens/cai-dat import Card từ kit", () => {
  const dung = tsx(join(GOC, "cai-dat")).filter((p) => importCard(readFileSync(p, "utf8")));
  assert.deepEqual(dung, [], "màn Cài đặt còn dùng Card (thẻ nổi có bóng)");
});

test("NhomHang có trong kit và Cài đặt dùng nó", () => {
  const ui = readFileSync(new URL("../src/rudi/ui.tsx", import.meta.url), "utf8");
  assert.match(ui, /export function NhomHang\(/);
  const caiDat = readFileSync(join(GOC, "cai-dat", "CaiDatScreen.tsx"), "utf8");
  assert.ok((caiDat.match(/<NhomHang>/g) ?? []).length >= 5, "CaiDatScreen phải nhóm hàng bằng NhomHang");
});

test("nợ có tên: các màn ngoài cai-dat còn import Card", () => {
  const con = tsx(GOC).filter((p) => !p.includes("/cai-dat/") && importCard(readFileSync(p, "utf8"))).map((p) => p.slice(GOC.length));
  console.log("còn dùng Card:", con.length ? con.join(", ") : "không màn nào");
  // Không đỏ: đây là nợ ngoài bề mặt audit, được ghi tên trong trả lời.
  assert.ok(Array.isArray(con));
});
