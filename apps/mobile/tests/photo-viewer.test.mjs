import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { boundPhotoOffset, photoButtonTransform, viewerIndex } from "../dist-test/rudi/photo-viewer.js";

test("viewer keeps a bounded current page across size and collection changes", () => {
  assert.equal(viewerIndex(1, 2), 1);
  assert.equal(viewerIndex(1, 1), 0);
  assert.equal(viewerIndex(-1, 2), 0);
  assert.equal(viewerIndex(Infinity, 2), 0);
  assert.equal(viewerIndex(0, 0), 0);
  const source = readFileSync(new URL("../src/rudi/ui/PhotoViewer.tsx", import.meta.url), "utf8");
  assert.match(source, /initialScrollIndex=\{currentIndex\}/);
  assert.match(source, /photos\[currentIndex\]\?\.caption/);
  assert.match(source, /setZoomed\(false\); setWidth\(nextWidth\)/);
});

test("web: mỗi trang có chiều cao thật, bộ đếm theo cuộn, một cú vuốt một ảnh, chụm không phóng trang, đóng mờ dần (QA UI-094, UI-098)", () => {
  const source = readFileSync(new URL("../src/rudi/ui/PhotoViewer.tsx", import.meta.url), "utf8");
  assert.match(source, /<View style=\{\{ width, height, overflow: "hidden" \}\}>/);
  assert.doesNotMatch(source, /\{ width, flex: 1, overflow: "hidden" \}/, "ô cao theo flex là ô cao 0 trên web");
  assert.match(source, /onScroll=\{\(event\) => khiCuon\(event\.nativeEvent\.contentOffset\.x\)\}/);
  assert.match(source, /setIndex\(viewerIndex\(x \/ width, photos\.length\)\)/);
  // Web turns the page in JS, one page per swipe: the browser's paging let the fling run on a page (b9-sau3: 0 → 390 → 780).
  assert.match(source, /pagingEnabled=\{!WEB\} scrollEnabled=\{WEB \|\| !zoomed\}/);
  assert.match(source, /const buoc = e\.translationX < -48 \|\| e\.velocityX < -400 \? 1 : e\.translationX > 48 \|\| e\.velocityX > 400 \? -1 : 0;/);
  assert.match(source, /den\(tuTrang\.current \+ buoc\)/);
  assert.match(source, /\.onStart\(\(\) => \{ tuTrang\.current = viTri\.current; dangKeo\.current = true; \}\)/);
  assert.match(source, /event\.key === "ArrowRight"/);
  assert.match(source, /<Modal visible=\{mo\}/);
  assert.match(source, /setTimeout\(onClose, motion\.reduced \? 0 : motion\.ms\("standard"\)\)/);
});

test("pinch shrinking clamps the old pan offset on both axes", () => {
  for (const extent of [320, 900, 1200]) {
    const far = boundPhotoOffset(5000, extent, 4);
    assert.equal(far, extent * 1.5);
    assert.ok(Math.abs(boundPhotoOffset(far, extent, 1.2) - extent * 0.1) < 0.00001);
    assert.ok(Math.abs(boundPhotoOffset(-far, extent, 1.2) + extent * 0.1) < 0.00001);
    assert.equal(boundPhotoOffset(far, extent, 1), 0);
  }
});

test("viewer buttons zoom/reset without carrying a previous pan", () => {
  const far = { scale: 4, x: 480, y: -600 };
  assert.deepEqual(photoButtonTransform("zoom", far, 320, 640), { scale: 2, x: 0, y: 0 });
  assert.deepEqual(photoButtonTransform("reset", far, 320, 640), { scale: 1, x: 0, y: 0 });
});

test("pan buttons reach every edge and stay bounded on each axis", () => {
  for (const [width, height] of [[320, 640], [768, 1024], [640, 320]]) {
    let position = { scale: 2, x: 0, y: 0 };
    for (let i = 0; i < 40; i += 1) position = photoButtonTransform("right", position, width, height);
    assert.equal(position.x, width / 2);
    assert.equal(position.y, 0);
    for (let i = 0; i < 40; i += 1) position = photoButtonTransform("up", position, width, height);
    assert.equal(position.x, width / 2);
    assert.equal(position.y, -height / 2);
    for (let i = 0; i < 40; i += 1) position = photoButtonTransform("left", position, width, height);
    assert.equal(position.x, -width / 2);
    for (let i = 0; i < 40; i += 1) position = photoButtonTransform("down", position, width, height);
    assert.equal(position.y, height / 2);
    assert.deepEqual(photoButtonTransform("right", { scale: 1, x: 0, y: 0 }, width, height), { scale: 1, x: 0, y: 0 });
  }
});
