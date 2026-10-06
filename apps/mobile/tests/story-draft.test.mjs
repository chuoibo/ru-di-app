import assert from "node:assert/strict";
import test from "node:test";
import { boNhapDang, docNhapDang, ghiNhapDang, khoaNhapDang } from "../dist-test/rudi/ky-niem/nhap-dang.js";

test("an uploaded photo survives a failed story POST and composer remount, even without a caption", () => {
  const key = khoaNhapDang("story", "person-synthetic-one", null);
  const uploaded = { anh: null, chu: "", anhDaTai: "/people/photos/synthetic-upload" };
  try {
    ghiNhapDang(key, uploaded);
    assert.deepEqual(docNhapDang(key), uploaded);
    assert.equal(docNhapDang(khoaNhapDang("story", "person-synthetic-two", null)), null, "another account inherited the image");
    assert.equal(docNhapDang(khoaNhapDang("khoanh-khac", "person-synthetic-one", null)), null, "another composer inherited the image");
  } finally { boNhapDang(key); }
});

test("an old picked-photo draft remains compatible; deliberately dropping all content clears it", () => {
  const key = khoaNhapDang("story", "person-synthetic-one", null);
  const picked = { anh: { uri: "file:///synthetic-photo", width: 320, height: 240 }, chu: "Câu đang viết" };
  try {
    ghiNhapDang(key, picked);
    assert.deepEqual(docNhapDang(key), picked);
    ghiNhapDang(key, { anh: null, chu: " ", anhDaTai: null });
    assert.equal(docNhapDang(key), null);
    ghiNhapDang(key, { anh: null, chu: "Một câu chưa gửi", anhDaTai: "/people/photos/synthetic-upload" });
    boNhapDang(key);
    assert.equal(docNhapDang(key), null, "discarded draft reappeared");
  } finally { boNhapDang(key); }
});
