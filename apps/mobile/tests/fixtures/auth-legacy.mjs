// Historical ADR-0014/0016 wire oracle, from d3f74730. No runtime imports this file.
import { translatedAnonymous, newAttempt } from "../../dist-test/api.js";
import { ghiNho, chonNhomMacDinh } from "../../dist-test/phien.js";
const CAU_MA_MOI_KHONG_MO = "Lời mời này không còn hiệu lực. Nhờ người trong nhóm gửi lại mã mới.";
const LOI_DOI_LOI_MOI = {
    // 404 is every refusal this route makes: expired, revoked, already spent,
    // never existed. The server answers them identically on purpose, so the
    // sentence here must not pretend to know which one happened.
    http_404: CAU_MA_MOI_KHONG_MO,
    // The same refusal as Go names it. Without this line the table missed, the
    // status chose, and a mistyped code read «Cập nhật app» (QA UI-019).
    invite_not_found: CAU_MA_MOI_KHONG_MO,
    http_422: "Mã lời mời không đúng định dạng.",
};
export async function doiLoiMoiLayPhien(maLoiMoi, kho) {
    const phien = await translatedAnonymous(LOI_DOI_LOI_MOI, "/sessions", {
        method: "POST",
        body: { invite_token: maLoiMoi },
        // Minted here, on the press. A retry of a dropped answer replays the same
        // session instead of spending a secret that is already gone.
        attempt: newAttempt(),
    });
    await ghiNho(phien, kho);
    return phien;
}
const LOI_OTP_GUI = {
    phone_required: "Nhập số điện thoại.",
    phone_not_mobile: "Chưa đúng dạng số di động Việt Nam.",
    otp_resend_too_soon: "Mã vừa được gửi. Đợi một chút rồi gửi lại.",
    otp_too_many_requests: "Số này vừa nhận nhiều mã. Thử lại sau ít phút.",
    rate_limited: "Thử lại sau một phút.",
    identity_key_missing: "Rủ Đi chưa sẵn sàng cho đăng nhập. Thử lại sau ít phút.",
    sms_unavailable: "Chưa gửi được tin nhắn lúc này, thử lại sau.",
};
const LOI_OTP_XAC_MINH = {
    otp_challenge_not_found: "Mã không còn hiệu lực. Xin mã mới.",
    otp_too_many_attempts: "Sai quá nhiều lần. Xin mã mới.",
    challenge_id_invalid: "Lượt xin mã bị lỗi. Xin mã mới.",
    rate_limited: "Thử lại sau một phút.",
    identity_key_missing: "Rủ Đi chưa sẵn sàng cho đăng nhập. Thử lại sau ít phút.",
};
export async function guiOtp(phone) {
    return translatedAnonymous(LOI_OTP_GUI, "/auth/otp/request", {
        method: "POST",
        body: { phone },
        attempt: newAttempt(),
    });
}
export async function xacMinhOtp(challengeId, phone, code, kho) {
    const wire = await translatedAnonymous(LOI_OTP_XAC_MINH, "/auth/otp/verify", {
        method: "POST",
        body: { challenge_id: challengeId, phone, code },
        attempt: newAttempt(),
    });
    const phien = chonNhomMacDinh(wire);
    await ghiNho(phien, kho);
    return phien;
}
export async function dangNhapGoogle(idToken, kho) {
    const wire = await translatedAnonymous({}, "/auth/google", {
        method: "POST",
        body: { id_token: idToken },
        attempt: newAttempt(),
    });
    const phien = chonNhomMacDinh(wire);
    await ghiNho(phien, kho);
    return phien;
}
