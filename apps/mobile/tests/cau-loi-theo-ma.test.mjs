/* The refusal sentence follows the server's code, not a guess from the status.
 *
 * QA 27/09: a mistyped invite code (404 `invite_not_found`) told the person to
 * update the app (UI-019); inviting a member already in the group (409
 * `membership_already_open`) told them an earlier press had not finished
 * (UI-072); a catalogue that answered 503 sent them to check their network
 * (UI-029). Each test below is one of those, through the real
 * `thongDiepNguoiDoc` the API layer calls, plus the shape rules that keep a
 * code added tomorrow off the status guess.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { thongDiepNguoiDoc } from "../dist-test/api.js";
import { cauTheoMa, laTuChoiVinhVien } from "../dist-test/cau-loi-theo-ma.js";

test("mã mời sai nói về mã mời, không bảo cập nhật app (UI-019)", () => {
  const cau = thongDiepNguoiDoc(404, "Invite link is not valid", "invite_not_found");
  assert.match(cau, /^Mã này /);
  assert.match(cau, /gõ sai/);
  assert.doesNotMatch(cau, /cập nhật app/i);
});

test("mời người đã trong nhóm nói đúng điều đó, không nói lần bấm trước (UI-072)", () => {
  const cau = thongDiepNguoiDoc(409, null, "membership_already_open");
  assert.match(cau, /đã ở trong nhóm/);
  assert.doesNotMatch(cau, /lần bấm trước/i);
});

test("409 không mã riêng không còn đổ cho lần bấm trước", () => {
  assert.doesNotMatch(thongDiepNguoiDoc(409, null, "http_409"), /lần bấm trước|đừng bấm lại/i);
  assert.doesNotMatch(thongDiepNguoiDoc(409, null), /lần bấm trước|đừng bấm lại/i);
});

test("dịch vụ tạm ngưng (503 *_unavailable) không bảo kiểm tra mạng; mất mạng thì có (UI-029)", () => {
  const ngung = thongDiepNguoiDoc(503, "provider down", "routing_unavailable");
  assert.doesNotMatch(ngung, /mạng/i);
  assert.match(thongDiepNguoiDoc(0, null, "unreachable"), /mạng/i);
});

test("404 không có mã (route thiếu) vẫn là câu cập nhật app; mọi *_not_found thì không", () => {
  assert.match(thongDiepNguoiDoc(404, "Not Found", "http_404"), /cập nhật app/i);
  for (const ma of ["post_not_found", "outing_not_found", "diary_photo_not_found", "thu_moi_not_found"]) {
    const cau = thongDiepNguoiDoc(404, null, ma);
    assert.match(cau, /^Không tìm thấy /, ma);
    assert.doesNotMatch(cau, /cập nhật app/i, ma);
  }
  assert.match(cauTheoMa("diary_photo_not_found"), /ảnh này trong sổ chuyến đi/);
});

test("câu tiếng Việt của máy chủ vẫn thắng mọi bảng", () => {
  assert.equal(thongDiepNguoiDoc(409, "Cuộc trò chuyện này không còn nhận tin.", "direct_message_unavailable"), "Cuộc trò chuyện này không còn nhận tin.");
});

test("401 nói phiên hết, không nói thiếu quyền trong nhóm", () => {
  assert.match(thongDiepNguoiDoc(401, null, "authentication_required"), /đăng nhập lại/i);
  assert.match(thongDiepNguoiDoc(401, null), /đăng nhập lại/i);
  assert.doesNotMatch(thongDiepNguoiDoc(401, null), /người tạo nhóm/);
});

test("không câu nào in mã, số trạng thái hay tên mã máy", () => {
  for (const ma of ["invite_not_found", "membership_already_open", "paper_wrong_state", "revision_conflict", "paper_expired", "brain_unavailable", "image_too_large", "authentication_required"]) {
    const cau = cauTheoMa(ma);
    assert.ok(cau, ma);
    assert.doesNotMatch(cau, /[a-z]+_[a-z]+|\b\d{3}\b/, `${ma}: ${cau}`);
  }
  assert.equal(cauTheoMa("not_found"), null, "mã không có tên vật thì để trạng thái chọn câu");
  assert.equal(cauTheoMa(undefined), null);
});

test("từ chối vĩnh viễn không mời «Thử lại» (UI-100)", () => {
  assert.equal(laTuChoiVinhVien(404, "post_not_found"), true);
  assert.equal(laTuChoiVinhVien(403, "forbidden"), true);
  assert.equal(laTuChoiVinhVien(409, "paper_wrong_state"), true);
  assert.equal(laTuChoiVinhVien(503, "brain_unavailable"), false);
  assert.equal(laTuChoiVinhVien(0, "unreachable"), false);
  assert.equal(laTuChoiVinhVien(500, "http_500"), false);
});
