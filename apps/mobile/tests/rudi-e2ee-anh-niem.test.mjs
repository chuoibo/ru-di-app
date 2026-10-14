import assert from "node:assert/strict";
import test from "node:test";
import { khungAnhChat, nguonAnhNiemDaMo } from "../dist-test/rudi/chat/e2ee/anh-niem.js";
import { nguonAnh } from "../dist-test/rudi/ky-niem/ky-niem.js";
import { SO_TRONG, apDung } from "../dist-test/rudi/chat/e2ee/so-tin.js";
import { sangTin } from "../dist-test/rudi/chat/e2ee/ve-v2.js";

test("ảnh mở từ native dùng bytes tại chỗ, không gửi header bearer cho nguồn ảnh", () => {
  const bytes = Buffer.from([1, 2, 3, 4]).toString("base64");
  for (const mime of ["jpeg", "png", "webp"]) {
    const uri = `data:image/${mime};base64,${bytes}`;
    assert.deepEqual(nguonAnhNiemDaMo(uri), { uri });
    assert.equal(nguonAnh(uri, "actor", "room"), null);
  }
});

test("ảnh ngang/dọc giữ tỷ lệ và không vượt khung chat; không tin kích thước không hợp lệ", () => {
  assert.deepEqual(khungAnhChat(960, 640), { width: 208, height: 139 });
  assert.deepEqual(khungAnhChat(960, 640, 191), { width: 191, height: 127 });
  assert.deepEqual(khungAnhChat(640, 800), { width: 208, height: 260 });
  assert.deepEqual(khungAnhChat(640, 1600), { width: 120, height: 300 });
  for (const [w, h] of [[undefined, undefined], [0, 640], [640, -1], [NaN, 640], [1.5, 640], [Infinity, 640]]) {
    assert.deepEqual(khungAnhChat(w, h), { width: 208, height: 208 });
    assert.deepEqual(khungAnhChat(w, h, 180), { width: 180, height: 180 });
  }
});

test("projection giữ kích thước trong image operation đã xác thực, xoá không giữ ảnh", () => {
  const id = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
  const actor = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
  const received = { kind: "application", actor_id: actor, device_id: actor, logical_send_id: id,
    operation: { type: "image", media: { media_id: id, mime: "image/jpeg" }, caption: null, width: 960, height: 640 } };
  const so = apDung(SO_TRONG, received, 1);
  const tin = sangTin(so.byId[id], "room", actor, {});
  assert.equal(tin.image_width, 960);
  assert.equal(tin.image_height, 640);
  const deleted = apDung(so, { ...received, logical_send_id: actor, operation: { type: "delete", message_id: id } }, 2);
  const row = sangTin(deleted.byId[id], "room", actor, {});
  assert.equal(row.image_width, undefined);
  assert.equal(row.image_height, undefined);
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
