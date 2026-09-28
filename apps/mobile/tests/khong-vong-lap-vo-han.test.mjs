/**
 * Không animation nào tự chạy mãi (ADR-0037 D3, DESIGN «không animation nền»).
 *
 * Chuyển động của sân khấu giấy hoặc có thời lượng (bật dựng ≤ 420ms, một tiết
 * mục ≤ 1 400ms, lật trang 300ms), hoặc gắn với tay/cuộn/cảm biến và dừng khi
 * tay dừng. Vòng lặp vô hạn duy nhất được phép là khung xương đang tải
 * (`ui/Skeleton.tsx`): nó biến mất khi dữ liệu tới.
 */
import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import test from "node:test";

const GOC = new URL("..", import.meta.url).pathname;
const DUOC_PHEP = new Set(["src/rudi/ui/Skeleton.tsx"]);

function tep(dir) {
  const ra = [];
  for (const ten of readdirSync(dir)) {
    const p = join(dir, ten);
    if (statSync(p).isDirectory()) ra.push(...tep(p));
    else if (/\.(ts|tsx)$/.test(ten)) ra.push(p);
  }
  return ra;
}

test("withRepeat, lặp vô hạn kiểu CSS hay setInterval vẽ hình chỉ nằm ở khung xương đang tải", () => {
  const vi = [];
  for (const p of [...tep(join(GOC, "src")), ...tep(join(GOC, "app"))]) {
    const rel = relative(GOC, p);
    const ma = readFileSync(p, "utf8").replace(/\/\*[\s\S]*?\*\//g, "").replace(/(^|[^:])\/\/.*$/gm, "$1");
    if (DUOC_PHEP.has(rel)) continue;
    if (/\bwithRepeat\s*\(/.test(ma)) vi.push(`${rel}: withRepeat`);
    if (/animationIterationCount\s*:\s*["']infinite["']/.test(ma)) vi.push(`${rel}: animationIterationCount infinite`);
    if (/Animated\.loop\s*\(/.test(ma)) vi.push(`${rel}: Animated.loop`);
  }
  assert.deepEqual(vi, [], `vòng lặp chuyển động ngoài khung xương đang tải:\n${vi.join("\n")}`);
});

test("khung xương đang tải vẫn là nơi duy nhất, và vẫn còn đó", () => {
  const ma = readFileSync(join(GOC, "src/rudi/ui/Skeleton.tsx"), "utf8");
  assert.match(ma, /withRepeat\s*\(/, "nếu Skeleton thôi lặp thì gỡ ngoại lệ này");
});
