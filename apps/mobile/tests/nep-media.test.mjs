import assert from "node:assert/strict";
import { test } from "node:test";

import { MEDIA_REFUSALS, conPhaiHoi, duongFile, nguonAnhNep, nhipHoiMs } from "../dist-test/rudi/nep/media.js";

test("còn phải hỏi lại khi việc chưa ngã ngũ, và thôi khi đã ngã ngũ", () => {
  assert.equal(conPhaiHoi("dang-cho"), true);
  assert.equal(conPhaiHoi("dang-chay"), true);
  assert.equal(conPhaiHoi("xong"), false);
  assert.equal(conPhaiHoi("hong"), false, "hỏng cũng là đã xong việc hỏi");
});

test("nhịp hỏi tăng dần và có trần, vì một tấm ảnh mất chín mươi chín giây", () => {
  assert.equal(nhipHoiMs(0), 2000);
  assert.ok(nhipHoiMs(3) > nhipHoiMs(1));
  assert.equal(nhipHoiMs(1000), 15000, "phải có trần, không tăng vô hạn");
  assert.equal(nhipHoiMs(-5), 2000);
  assert.equal(nhipHoiMs(Number.NaN), 2000);
});

test("đường file đi qua máy chủ của app, không trỏ thẳng vào chỗ vẽ", () => {
  assert.equal(duongFile("http://x/", "abc"), "http://x/me/nep/media/abc/file");
  assert.equal(duongFile("http://x", "abc"), "http://x/me/nep/media/abc/file");
});

test("job id lạ không dựng được đường đi ra ngoài", () => {
  assert.equal(duongFile("http://x", "../../admin"), "http://x/me/nep/media/..%2F..%2Fadmin/file");
});

test("mọi mã từ chối đều có câu người đọc, và không câu nào lộ chi tiết máy chủ", () => {
  for (const [ma, cau] of Object.entries(MEDIA_REFUSALS)) {
    assert.ok(cau.length > 10, ma);
    assert.ok(!cau.includes("proxy") && !cau.includes("token"), `${ma} lộ chi tiết: ${cau}`);
    assert.ok(!cau.includes("—"), `${ma} có em dash`);
  }
});

test("hết hạn mức và mô tả hỏng là thứ PHẢI nói ra, nên không nằm trong bảng im lặng", () => {
  assert.equal(MEDIA_REFUSALS.rate_limited, undefined);
  assert.equal(MEDIA_REFUSALS.cooldown, undefined);
});

test("ảnh Nếp vẽ tải kèm header của người gọi, vì đường file đòi xác thực như mọi route /me", () => {
  const nguon = nguonAnhNep("http://x/", "abc", "nguoi-1");
  assert.equal(nguon.uri, "http://x/me/nep/media/abc/file");
  assert.equal(nguon.headers["X-Actor-ID"], "nguoi-1");
  assert.equal(nguon.headers["X-Actor-Roles"], "member", "chỉ khai đúng vai cần, không bốn vai mặc định");
});

test("màn tiền có câu người đọc riêng cho việc vẽ", () => {
  assert.ok(MEDIA_REFUSALS.nep_lui_man_tien.includes("màn tiền"));
});
