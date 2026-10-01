/**
 * The sentence for a refusal the server named, chosen by its `code` before
 * its HTTP status is consulted.
 *
 * `thongDiepNguoiDoc` used to read the status alone, so the status guessed the
 * cause, and guessed wrong where people meet it most (QA 27/09):
 *
 * - a mistyped invite code is a 404 `invite_not_found`, and the person was told
 *   «Cập nhật app rồi thử lại» -- the 404 sentence was written for a route the
 *   server does not have (UI-019);
 * - inviting someone already in the group is a 409 `membership_already_open`,
 *   and the person was told an earlier press had not finished and not to press
 *   again -- every 409 read as the idempotency one (UI-072).
 *
 * Every code the server sends is a decision it already made about the cause;
 * this module turns the ones a person can meet into one sentence each: what
 * failed, why when it is known, and what to do next. Codes are grouped by
 * shape (`*_not_found`, `*_wrong_state`, `*_unavailable`…) so a code added on
 * the server tomorrow still lands on a sentence about its kind of failure
 * rather than on the status guess. A screen's own refusal table still wins
 * over this one; this is the floor under all of them.
 *
 * Nothing here names a code, a status or an address: those are for whoever is
 * debugging. A permanent refusal never invites «Thử lại» -- the screens decide
 * that, from `laTuChoiVinhVien`.
 */

/** What a `<thing>_not_found` is called, longest prefix first. */
const TEN_THU: Record<string, string> = {
  diary_photo: "ảnh này trong sổ chuyến đi",
  guest_obligation: "phần tiền này",
  guest_link: "link này",
  consent_proposal: "lời đề nghị này",
  otp_challenge: "mã này",
  invite: "lời mời này",
  post: "bài này",
  comment: "bình luận này",
  paper: "tờ giấy này",
  notebook: "sổ này",
  outing: "kèo này",
  stop: "chặng này",
  person: "người này",
  photo: "ảnh này",
  media: "ảnh hoặc video này",
  diary: "sổ chuyến đi này",
  place: "địa điểm này",
  message: "tin nhắn này",
  context: "nhóm này",
  vote: "bình chọn này",
  draft: "bản nháp này",
  session: "phiên đăng nhập này",
  invocation: "lời nhờ này",
};

const PHIEN_HET = "Phiên đăng nhập đã hết. Đăng nhập lại để tiếp tục.";
const AI_DO_VUA_SUA = "Ai đó vừa sửa chỗ này trước bạn. Mở lại để xem bản mới nhất rồi sửa tiếp.";

/** Codes whose sentence cannot be read off their shape. */
const CAU_RIENG: Record<string, string> = {
  authentication_required: PHIEN_HET,
  session_not_found: PHIEN_HET,
  membership_required: "Tài khoản này không còn ở trong nhóm, nên chưa làm được việc này.",
  // One answer for typo, expiry, spent and revoked (the server does not say
  // which), so the sentence names them all instead of guessing one.
  invite_not_found:
    "Mã này không mở được lời mời nào: có thể gõ sai, hoặc lời mời đã hết hạn hay đã được dùng. Kiểm tra lại mã, hoặc nhờ người trong nhóm mời lại.",
  invite_already_accepted: "Lời mời này đã được nhận rồi. Nhóm đã có trong danh sách của bạn.",
  invite_already_exists: "Người này đã có một lời mời đang chờ. Nhắc họ mở lời mời đó.",
  membership_already_open: "Người này đã ở trong nhóm, hoặc đã có lời mời đang chờ trả lời.",
  direct_message_unavailable: "Cuộc trò chuyện này không còn nhận tin.",
  revision_conflict: AI_DO_VUA_SUA,
  timeline_conflict: AI_DO_VUA_SUA,
  diary_revision_conflict: AI_DO_VUA_SUA,
  resend_too_soon: "Mã vừa được gửi. Chờ một chút rồi xin mã mới.",
  otp_resend_too_soon: "Mã vừa được gửi. Chờ một chút rồi xin mã mới.",
  otp_too_many_requests: "Số này đã xin mã nhiều lần liền. Chờ vài phút rồi thử lại.",
  otp_too_many_attempts: "Nhập sai mã quá nhiều lần. Xin mã mới rồi nhập lại.",
  otp_code_invalid: "Mã chưa đúng. Kiểm tra lại 6 số trong tin nhắn.",
  rate_limited: "Bạn thao tác khá nhanh. Chờ một lát rồi thử lại.",
  image_too_large: "Ảnh này lớn quá. Chọn ảnh nhỏ hơn rồi thử lại.",
  image_dimensions_too_large: "Ảnh này lớn quá. Chọn ảnh nhỏ hơn rồi thử lại.",
  media_bundle_too_large: "Gửi nhiều ảnh quá trong một lần. Bớt vài ảnh rồi gửi lại.",
};

/**
 * The sentence for `code`, or `null` when the code says nothing a person can
 * use and the status should choose. Case-insensitive, like the tables in
 * `api.ts`.
 */
export function cauTheoMa(code: string | null | undefined): string | null {
  if (!code) return null;
  const ma = code.toLowerCase();
  const rieng = CAU_RIENG[ma];
  if (rieng) return rieng;
  const thu = ma.match(/^([a-z_]+?)_not_found$/)?.[1];
  if (thu) {
    const ten = TEN_THU[thu] ?? TEN_THU[thu.split("_")[0] ?? ""];
    // A missing thing the person was looking at: it was removed, or they no
    // longer belong where it lives. Both are permanent; neither is the app's age.
    if (ten) return `Không tìm thấy ${ten}. Có thể nó đã bị xoá, hoặc bạn không còn quyền xem.`;
    return "Không tìm thấy mục này. Có thể nó đã bị xoá, hoặc bạn không còn quyền xem.";
  }
  if (/_(wrong_state|already_[a-z_]+)$/.test(ma)) {
    return "Việc này không còn làm được ở bước hiện tại. Mở lại màn hình để xem trạng thái mới nhất.";
  }
  if (/_conflict$/.test(ma)) return AI_DO_VUA_SUA;
  if (/_expired$/.test(ma)) return "Mục này đã hết hạn. Mở lại màn hình để xem bản mới nhất.";
  if (/_unavailable$/.test(ma)) {
    // A part of Rủ Đi that answered "not now": the network is fine, so the
    // sentence must not send the person to check it (QA UI-029).
    return "Phần này của Rủ Đi đang tạm ngưng. Thử lại sau ít phút.";
  }
  if (/_too_large$/.test(ma)) return "Tệp này lớn quá. Chọn tệp nhỏ hơn rồi thử lại.";
  return null;
}

/**
 * A refusal that pressing again will not change: the thing is gone, the
 * person may not, the step has passed. A screen offers «Thử lại» only when
 * this is false (QA UI-100: two «Thử lại» under «Bài này không dành cho bạn»).
 */
export function laTuChoiVinhVien(status: number, code: string | null | undefined): boolean {
  const ma = (code ?? "").toLowerCase();
  if (/_not_found$|_wrong_state$|_already_[a-z_]+$|_expired$/.test(ma)) return true;
  if (ma === "membership_required" || ma === "direct_message_unavailable") return true;
  return status === 403 || status === 404 || status === 410;
}
