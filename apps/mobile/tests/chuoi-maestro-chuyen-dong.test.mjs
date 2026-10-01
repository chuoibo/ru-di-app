/**
 * Chuỗi của các flow quay chuyển động (`.maestro-motion/`, chạy bằng
 * scripts/do/quay-chuyen-canh.sh, không chạy trong CI) còn có mặt trong mã.
 *
 * Đợt thanh tab 5 cột (02/10) bỏ dòng tiêu đề cũ của Khám phá; ba flow r1 vẫn
 * chờ nó và sẽ hết giờ vì một lý do không phải chuyển động (review toàn nhánh).
 * Phép kiểm này đọc mọi chữ mà một flow chuyển động chờ, bấm hay khẳng định
 * DƯƠNG, bỏ chuỗi có ký tự regex, và đòi nó nằm nguyên văn trong `src/` hoặc
 * `app/`. Không chứng minh: flow chạy xanh trên máy thật.
 */
import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";

const GOC = new URL("..", import.meta.url).pathname;
const THU_MUC = join(GOC, ".maestro-motion");

function tep(dir) {
  const ra = [];
  for (const ten of readdirSync(dir)) {
    const p = join(dir, ten);
    if (statSync(p).isDirectory()) ra.push(...tep(p));
    else if (/\.(ts|tsx)$/.test(ten)) ra.push(p);
  }
  return ra;
}

const KHO = [...tep(join(GOC, "src")), ...tep(join(GOC, "app"))].map((p) => readFileSync(p, "utf8")).join("\n");

/** Strings a motion flow may wait for that are not the app's own words, each with its reason. */
const NGOAI_LE = new Map([
  ["KHONG_BAO_GIO_CO_CHUOI_NAY_TREN_MAN", "09-canary-phai-do.yaml: the canary that must time out, proving the harness can fail"],
  ["Continue", "_bo-qua-dev-menu.yaml: the Expo dev client's own menu, not the app"],
]);

test("mọi chữ thường mà flow chuyển động chờ hay bấm còn có trong mã", () => {
  const thieu = [];
  for (const f of readdirSync(THU_MUC).filter((t) => t.endsWith(".yaml"))) {
    for (const dong of readFileSync(join(THU_MUC, f), "utf8").split("\n")) {
      if (/^\s*#/.test(dong) || /assertNotVisible|notVisible/.test(dong)) continue;
      const m = dong.match(/^\s*-?\s*(?:visible|text|tapOn|assertVisible):\s*"([^"]+)"\s*$/);
      if (!m || /[.*+?|()[\]{}\\^$]/.test(m[1])) continue;
      if (!KHO.includes(m[1]) && !NGOAI_LE.has(m[1])) thieu.push(`${f}: «${m[1]}»`);
    }
  }
  assert.deepEqual(thieu, [], `flow chuyển động chờ chữ không còn trong mã:\n${thieu.join("\n")}`);
});
