import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { boundPhotoOffset, viewerIndex } from "../dist-test/rudi/photo-viewer.js";

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

test("pinch shrinking clamps the old pan offset on both axes", () => {
  for (const extent of [320, 900, 1200]) {
    const far = boundPhotoOffset(5000, extent, 4);
    assert.equal(far, extent * 1.5);
    assert.ok(Math.abs(boundPhotoOffset(far, extent, 1.2) - extent * 0.1) < 0.00001);
    assert.ok(Math.abs(boundPhotoOffset(-far, extent, 1.2) + extent * 0.1) < 0.00001);
    assert.equal(boundPhotoOffset(far, extent, 1), 0);
  }
});
