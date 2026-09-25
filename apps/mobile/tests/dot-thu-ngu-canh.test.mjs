/* Một đợt thu mở trong đúng ngữ cảnh của nó (S1, lỗi B9 tìm thấy khi lấy bằng
 * chứng 25/09).
 *
 * Chạy từ apps/mobile:
 *     node --test tests/dot-thu-ngu-canh.test.mjs
 *
 * Màn đợt thu đọc trạng thái đợt từ danh sách đợt của NGỮ CẢNH. Route
 * `/batches/{id}` trước đây không mang ngữ cảnh, nên đợt thu của một sổ hai
 * người (không bao giờ là nhóm hiện tại, ADR-0021 §2.5) được đọc trong nhóm
 * hiện tại: đợt đã phát hiện «Chưa phát», nút «Phát đợt thu» hiện lại, và
 * người nhận không có nút «Tiền đã về». Sửa giống route quyết toán (QA 23/09):
 * mở đợt kèm `?ctx=`, route chỉ nhận ngữ cảnh mà người dùng đang ở trong.
 *
 * Phép ghim nguồn: chứng minh hai đầu dây còn nối. Không chứng minh: máy chủ
 * trả đúng trạng thái -- đó là việc của ảnh chụp trên stack thật (đợt của sổ
 * đôi hiện «Đã phát» và nút «Tiền đã về từ …»).
 */
import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";

const GOC = new URL("..", import.meta.url).pathname;
const ROUTE = readFileSync(join(GOC, "app/batches/[id]/index.tsx"), "utf8");

function tep(dir) {
  const ra = [];
  for (const ten of readdirSync(dir)) {
    const p = join(dir, ten);
    if (statSync(p).isDirectory()) ra.push(...tep(p));
    else if (ten.endsWith(".tsx") || ten.endsWith(".ts")) ra.push(p);
  }
  return ra;
}

test("route đợt thu nhận ?ctx= qua nguCanhMo và chạy màn trong ngữ cảnh đó", () => {
  assert.match(ROUTE, /useLocalSearchParams<\{[^}]*ctx\?: string[^}]*\}>/, "route không đọc ?ctx=");
  assert.match(ROUTE, /nguCanhMo\(phien, params\.ctx\)/, "route không hỏi nguCanhMo ngữ cảnh nào được mở");
  assert.match(ROUTE, /context_id: mo\.contextId/, "route không đổi ngữ cảnh của phiên sang ngữ cảnh của đợt");
});

test("mọi chỗ mở một đợt thu đều mang ngữ cảnh của nó", () => {
  const mo = [];
  for (const f of [...tep(join(GOC, "src")), ...tep(join(GOC, "app"))]) {
    const nguon = readFileSync(f, "utf8");
    for (const m of nguon.matchAll(/router\.(?:push|replace)\(`\/batches\/[^`]*`/g)) mo.push({ f: f.slice(GOC.length), s: m[0] });
  }
  assert.ok(mo.length >= 2, `chỉ thấy ${mo.length} chỗ mở đợt thu: bộ đọc đã hỏng`);
  const thieu = mo.filter((x) => !x.s.includes("?ctx="));
  assert.deepEqual(thieu, [], "mở đợt thu mà không mang ngữ cảnh");
});
