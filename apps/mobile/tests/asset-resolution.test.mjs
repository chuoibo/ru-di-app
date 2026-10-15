import assert from "node:assert/strict";
import test from "node:test";
import { illustrationResolution } from "../dist-test/rudi/asset-pack/resolution.js";

test("density is applied to the occupied image, including portrait cover art", () => {
  assert.equal(illustrationResolution(220, 220 / .75, .75, 3, "contain"), "medium");
  assert.equal(illustrationResolution(350, 350 / 1.3, 1.3, 3, "contain"), "master");
  assert.equal(illustrationResolution(200, 100, 2, 1, "contain"), "small");
  assert.equal(illustrationResolution(200, 100, 2, 3, "contain"), "medium");
});

test("contain uses the letterboxed area; cover budgets the cropped source pixels", () => {
  // A 2:1 image in a tall slot occupies 200x100 when contained, but must be
  // decoded at 800x400 before its sides can be cropped to cover the same slot.
  assert.equal(illustrationResolution(200, 400, 2, 1, "contain"), "small");
  assert.equal(illustrationResolution(200, 400, 2, 1, "cover"), "master");
  assert.equal(illustrationResolution(400, 100, .5, 2, "contain"), "small");
  assert.equal(illustrationResolution(400, 100, .5, 2, "cover"), "master");
});

test("384 and 768 physical pixels are included in their raster budget", () => {
  for (const fit of ["contain", "cover"]) {
    assert.equal(illustrationResolution(128, 128, 1, 3, fit), "small");
    assert.equal(illustrationResolution(128.1, 128.1, 1, 3, fit), "medium");
    assert.equal(illustrationResolution(256, 256, 1, 3, fit), "medium");
    assert.equal(illustrationResolution(256.1, 256.1, 1, 3, fit), "master");
  }
});

test("a corrupt layout, image ratio or density cannot request a master raster", () => {
  for (const bad of [0, -1, NaN, Infinity, -Infinity]) {
    for (let index = 0; index < 4; index += 1) {
      const dimensions = [320, 240, 4 / 3, 3];
      dimensions[index] = bad;
      for (const fit of ["contain", "cover"]) {
        assert.equal(illustrationResolution(...dimensions, fit), "small");
      }
    }
  }
});
