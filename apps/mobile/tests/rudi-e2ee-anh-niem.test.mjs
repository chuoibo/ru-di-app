import assert from "node:assert/strict";
import test from "node:test";
import { nguonAnhNiemDaMo } from "../dist-test/rudi/chat/e2ee/anh-niem.js";
import { nguonAnh } from "../dist-test/rudi/ky-niem/ky-niem.js";

test("ảnh mở từ native dùng bytes tại chỗ, không gửi header bearer cho nguồn ảnh", () => {
  const bytes = Buffer.from([1, 2, 3, 4]).toString("base64");
  for (const mime of ["jpeg", "png", "webp"]) {
    const uri = `data:image/${mime};base64,${bytes}`;
    assert.deepEqual(nguonAnhNiemDaMo(uri), { uri });
    assert.equal(nguonAnh(uri, "actor", "room"), null);
  }
});

test("nguồn ảnh niêm từ chối URL mạng, SVG, base64 sai và payload quá trần", () => {
  // repo-guard: allow=data-uri-base64 reason=synthetic-test-prefix-no-file-bytes
  const prefix = "data:image/jpeg;base64,";
  const bytes = Buffer.from([1, 2, 3, 4]).toString("base64");
  for (const bad of [
    null,
    "https://evil.example/x",
    "/media/x",
    // repo-guard: allow=data-uri-base64 reason=synthetic-four-byte-rejected-mime
    `data:image/svg+xml;base64,${bytes}`,
    // repo-guard: allow=data-uri-base64 reason=synthetic-four-byte-rejected-mime
    `data:text/html;base64,${bytes}`,
    prefix,
    prefix + "A===",
    prefix + bytes.slice(0, 3) + " " + bytes.slice(3),
    prefix + bytes.slice(0, -1),
    prefix + "A".repeat(35_000_000),
  ]) {
    assert.equal(nguonAnhNiemDaMo(bad), null);
  }
});
