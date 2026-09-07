/**
 * Lớp kích cỡ (size class) của vỏ RuDi.
 *
 * Trước module này, vỏ có một breakpoint `700` gõ tay ở bốn chỗ, mỗi chỗ hiểu
 * nó một kiểu, và không chỗ nào biết tới chiều cao. Test này đo đúng biên
 * 600/840 theo Android window size classes, đo chiều cao ngắn, và đo tính đơn
 * điệu: rộng hơn không bao giờ cho ít cột hơn hay lề hẹp hơn.
 */
import assert from "node:assert/strict";
import test from "node:test";

import {
  SHORT_HEIGHT,
  SIZE_CLASS_BREAKPOINTS,
  heightClassFor,
  gridFor,
  layoutFor,
  sizeClassFor,
} from "../dist-test/rudi/adaptive.js";

test("grid đo hộp nội dung: 30 địa điểm không ép thành một hàng", () => {
  for (const width of [288, 360, 692, 900, 1200, 2000]) {
    const grid = gridFor(width, 320, 12);
    assert.ok(grid.columns >= 1 && grid.columns <= 3);
    assert.ok(grid.itemWidth * grid.columns + 12 * (grid.columns - 1) <= width + 0.001);
    if (grid.columns > 1) assert.ok(grid.itemWidth >= 320);
    assert.ok(Math.ceil(30 / grid.columns) >= 10);
  }
  assert.equal(gridFor(900 - 104 - 48, 320).columns, 2);
  assert.equal(gridFor(600 - 104 - 48, 320).columns, 1);
  for (const width of [0, -1, NaN, Infinity]) {
    assert.deepEqual(gridFor(width), { columns: 1, itemWidth: 0 });
  }
});

test("maxColumns: ô album lên sáu cột trên tablet, thẻ vẫn dừng ở ba", () => {
  // Phone content box (360 - 32): three tiles either way.
  assert.equal(gridFor(328, 104, 6).columns, 3);
  assert.equal(gridFor(328, 104, 6, 6).columns, 3);
  // Tablet content box (1200 - 36 * 2 after the rail): cards cap at three, tiles at six.
  assert.equal(gridFor(1128, 104, 6).columns, 3);
  const tiles = gridFor(1128, 104, 6, 6);
  assert.equal(tiles.columns, 6);
  assert.ok(tiles.itemWidth * 6 + 6 * 5 <= 1128);
  assert.ok(Number.isInteger(tiles.itemWidth));
  // A nonsense cap falls back to the default of three.
  assert.equal(gridFor(1128, 104, 6, NaN).columns, 3);
  assert.equal(gridFor(1128, 104, 6, 0).columns, 1);
});

test("biên 600 và 840 theo Android window size classes, không phải 700", () => {
  assert.equal(SIZE_CLASS_BREAKPOINTS.medium, 600);
  assert.equal(SIZE_CLASS_BREAKPOINTS.expanded, 840);
  assert.equal(sizeClassFor(599), "compact");
  assert.equal(sizeClassFor(600), "medium");
  assert.equal(sizeClassFor(839), "medium");
  assert.equal(sizeClassFor(840), "expanded");
  // The old hand-typed 700 lands inside medium, so both halves of it agree now.
  assert.equal(sizeClassFor(699), "medium");
  assert.equal(sizeClassFor(700), "medium");
});

test("chiều rộng vô nghĩa (NaN, âm, vô cực) rơi về compact, không ném", () => {
  assert.equal(sizeClassFor(Number.NaN), "compact");
  assert.equal(sizeClassFor(-1), "compact");
  assert.equal(sizeClassFor(Number.POSITIVE_INFINITY), "compact");
});

test("chiều cao ngắn là điện thoại nằm ngang, đo bằng cửa sổ chứ không bằng máy", () => {
  assert.equal(SHORT_HEIGHT, 480);
  assert.equal(heightClassFor(479), "short");
  assert.equal(heightClassFor(480), "regular");
  // 800x360: a phone on its side is compact by width and short by height.
  assert.deepEqual(
    [layoutFor(800, 360).sizeClass, layoutFor(800, 360).heightClass],
    ["medium", "short"],
  );
});

test("hợp đồng bố cục của từng lớp: cột, lề, rail, hai khung", () => {
  const phone = layoutFor(360, 800);
  assert.deepEqual(
    [phone.columns, phone.gutter, phone.rail, phone.twoPane, phone.maxContent],
    [1, 16, false, false, 360],
  );
  const tablet = layoutFor(720, 1024);
  assert.deepEqual(
    [tablet.columns, tablet.gutter, tablet.rail, tablet.twoPane, tablet.maxContent],
    [2, 24, true, true, 960],
  );
  const wide = layoutFor(1024, 768);
  assert.deepEqual(
    [wide.columns, wide.gutter, wide.rail, wide.twoPane, wide.maxContent],
    [3, 36, true, true, 1200],
  );
});

test("medium mà ngắn thì không có khung chi tiết bên cạnh danh sách", () => {
  const halfOpen = layoutFor(720, 420);
  assert.equal(halfOpen.sizeClass, "medium");
  assert.equal(halfOpen.twoPane, false, "không đủ cao để đặt hai khung");
  assert.equal(halfOpen.rail, true, "rail vẫn còn: bề rộng có");
});

test("đơn điệu theo bề rộng: rộng hơn không bao giờ ít cột hơn hay lề hẹp hơn", () => {
  let prev = layoutFor(320, 800);
  for (let w = 321; w <= 1400; w += 7) {
    const cur = layoutFor(w, 800);
    assert.ok(cur.columns >= prev.columns, `cột giảm ở ${w}`);
    assert.ok(cur.gutter >= prev.gutter, `lề giảm ở ${w}`);
    assert.ok(cur.maxContent >= Math.min(prev.maxContent, w), `measure giảm ở ${w}`);
    prev = cur;
  }
});
