/* Một lần gửi logic giữ một cái chìa (ADR-0021 §2.1; review delta 08/09, F32).
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/rudi-chat-hang-cho.test.mjs
 *
 * The claim these tests exist for is not «there is a spinner». It is that a
 * retry and a second send are DIFFERENT ACTS, and that the difference is
 * carried by the `Idempotency-Key`: pressing «Thử lại» replays the key the
 * press minted, so a request the server already accepted but whose answer the
 * client lost cannot become a second message; choosing the same sticker again
 * on purpose mints a new key and is a second message. That distinction lives
 * here, in a pure module, because it cannot be read off a screenshot.
 */
import assert from "node:assert/strict";
import test from "node:test";

import {
  boKhoiHang,
  danhDauLoi,
  danhDauThuLai,
  khoaDungLai,
  themVaoHang,
  thuLaiDuoc,
  timTrongHang,
} from "../dist-test/rudi/chat/hang-cho.js";

const TRICH = { id: "m-1", kind: "text", author_id: "p-2", preview: "Ăn gì nay?" };

function moiGui(khoa, phan = {}) {
  return {
    attempt: { key: khoa, at: 1 },
    kind: "sticker",
    than: "cho-ti",
    traLoi: TRICH,
    trangThai: "dang-gui",
    loi: null,
    thuLaiDuoc: true,
    luc: "2026-09-09T00:00:00Z",
    ...phan,
  };
}

test("thất bại rồi thử lại: cùng một chìa, cùng tin được trả lời", () => {
  let hang = themVaoHang([], moiGui("k-1"));
  hang = danhDauLoi(hang, "k-1", "Không nối được máy chủ.", null);
  const rot = timTrongHang(hang, "k-1");
  assert.equal(rot.trangThai, "that-bai");
  assert.equal(rot.thuLaiDuoc, true);

  hang = danhDauThuLai(hang, "k-1");
  const lai = timTrongHang(hang, "k-1");
  // The whole point: nothing minted a new attempt, and the reply target the
  // press captured is still the one a retry will answer.
  assert.equal(lai.attempt.key, "k-1");
  assert.equal(lai.trangThai, "dang-gui");
  assert.equal(lai.loi, null);
  assert.deepEqual(lai.traLoi, TRICH);
  assert.equal(lai.than, "cho-ti");
  assert.equal(hang.length, 1, "thử lại không tạo hàng thứ hai");
});

test("gửi cố ý lần thứ hai cùng một sticker là hai chìa khác nhau, hai hàng", () => {
  let hang = themVaoHang([], moiGui("k-1"));
  hang = themVaoHang(hang, moiGui("k-2"));
  assert.equal(hang.length, 2);
  assert.deepEqual(hang.map((t) => t.attempt.key), ["k-2", "k-1"], "mới nhất đứng đầu");
  assert.equal(new Set(hang.map((t) => t.attempt.key)).size, 2);
});

test("gửi xong thì rời hàng; hàng khác không bị đụng", () => {
  let hang = themVaoHang(themVaoHang([], moiGui("k-1")), moiGui("k-2"));
  hang = boKhoiHang(hang, "k-1");
  assert.deepEqual(hang.map((t) => t.attempt.key), ["k-2"]);
});

test("xếp lại cùng một chìa không nhân đôi hàng", () => {
  const hang = themVaoHang(themVaoHang([], moiGui("k-1")), moiGui("k-1", { than: "di-thoi" }));
  assert.equal(hang.length, 1);
  assert.equal(hang[0].than, "di-thoi");
});

test("lỗi vĩnh viễn không mời thử lại; lỗi tạm thì có", () => {
  // Bấm lại không sửa được: bản app không có hình, tin trả lời đã mất, đã rời nhóm.
  for (const ma of ["sticker_unknown", "reply_target_deleted", "permission_denied", "message_not_found"]) {
    assert.equal(thuLaiDuoc(ma), false, ma);
  }
  // Máy chủ nói thẳng «đừng bấm nữa»: lần gửi đầu còn đang chạy, hoặc chìa đã dùng.
  for (const ma of ["idempotency_request_in_flight", "idempotency_key_reuse", "invalid_idempotency_key"]) {
    assert.equal(thuLaiDuoc(ma), false, ma);
  }
  // Mất mạng, 500, hết giờ: bấm lại là việc đúng.
  assert.equal(thuLaiDuoc(null), true);
  assert.equal(thuLaiDuoc("unreachable"), true);
  assert.equal(thuLaiDuoc("internal_error"), true);

  const hang = danhDauLoi(themVaoHang([], moiGui("k-1")), "k-1", "Sticker này bản app chưa có.", "sticker_unknown");
  assert.equal(timTrongHang(hang, "k-1").thuLaiDuoc, false);
});

test("bản nháp chữ chỉ dùng lại chìa khi từng byte giống hệt", () => {
  const rot = moiGui("k-1", { kind: "text", than: "Đi ăn nha", traLoi: null, trangThai: "that-bai" });
  // Same words, same (absent) reply target: this is the same send, so the same
  // key -- and the server replays instead of writing twice.
  assert.equal(khoaDungLai(rot, "Đi ăn nha", null).key, "k-1");
  // A different sentence is a different send; reusing the key there would be
  // «422 idempotency_key_reuse», a refusal aimed at somebody who did nothing wrong.
  assert.equal(khoaDungLai(rot, "Đi ăn nhé", null), null);
  // Same words but now answering a message: also a different request body.
  assert.equal(khoaDungLai(rot, "Đi ăn nha", "m-9"), null);
  // Nothing failed, nothing to reuse.
  assert.equal(khoaDungLai(null, "Đi ăn nha", null), null);
  assert.equal(khoaDungLai({ ...rot, trangThai: "dang-gui" }, "Đi ăn nha", null), null);
});

test("timTrongHang trả null cho chìa không còn, nên thử lại không dựng lại tin đã gửi", () => {
  const hang = boKhoiHang(themVaoHang([], moiGui("k-1")), "k-1");
  assert.equal(timTrongHang(hang, "k-1"), null);
});

test("mọi câu người đọc trong module không có gạch dài", () => {
  const hang = danhDauLoi(themVaoHang([], moiGui("k-1")), "k-1", "Không nối được máy chủ.", null);
  assert.ok(!timTrongHang(hang, "k-1").loi.includes("—"));
});
