/* Mỗi bước chia bill là một trang mới, mở từ đầu trang (kế hoạch UI v3, S1).
 *
 * Chạy từ apps/mobile:
 *     node --test tests/chia-bill-buoc.test.mjs
 *
 * Vì sao cần: bàn gán món làm bước 3 cao hơn một màn. `.maestro/28` cuộn hết
 * bước 2 rồi bấm «Tiếp», và trước đây trang mới giữ nguyên vị trí cuộn của
 * trang cũ: nút «Quay lại» và «Bước 2/5» nằm ngoài màn, flow bấm vào khoảng
 * không. Màn truyền bước hiện tại cho `RudiScreen`, và `RudiScreen` cuộn về 0
 * mỗi khi giá trị đó đổi.
 *
 * Đây là phép ghim nguồn: chứng minh hai đầu dây còn nối. Không chứng minh:
 * danh sách thật sự cuộn về đầu trên máy -- đó là việc của ảnh chụp (đo y của
 * «Quay lại» sau khi đổi bước) và của flow 28 trên máy thật.
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const MAN = readFileSync(new URL("../src/rudi/screens/chia-bill/ChiaBillLive.tsx", import.meta.url), "utf8");
const UI = readFileSync(new URL("../src/rudi/ui.tsx", import.meta.url), "utf8");

test("màn chia bill đưa bước hiện tại cho RudiScreen", () => {
  // The stepped screen is the one whose footer is the step's decision.
  const the = [...MAN.matchAll(/<RudiScreen\b[^>]*>/gs)].map((m) => m[0]).find((t) => t.includes("footer={nutChinh}"));
  assert.ok(the, "không thấy thẻ RudiScreen mang nút bước của màn chia bill");
  assert.match(the, /cuonVeDau=\{buoc\.ten\}/, "RudiScreen của màn chia bill thiếu cuonVeDau={buoc.ten}");
});

test("RudiScreen cuộn về 0 mỗi khi cuonVeDau đổi, không cần hoạt ảnh", () => {
  const hieuUng = /useEffect\(\(\) => \{\s*if \(cuonVeDau === undefined\) return;\s*cuon\.current\?\.scrollTo\(\{ y: 0, animated: false \}\);\s*\}, \[cuonVeDau\]\);/;
  assert.match(UI, hieuUng, "hiệu ứng cuộn-về-đầu của RudiScreen đã đổi hình");
  assert.match(UI, /<ScrollView\s+ref=\{cuon\}/, "ScrollView thường không còn gắn ref cuon: scrollTo không tới đâu");
});
