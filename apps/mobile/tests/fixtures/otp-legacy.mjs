/**
 * The number as the OTP screen shows it: last three digits, the rest hidden.
 *
 * The person typed it a moment ago, so this is not secrecy from them; it is so
 * a screenshot, a screen recording or somebody reading over a shoulder gets
 * three digits and not a phone number.
 */
export function cheSo(phone) {
    const so = phone.replace(/\D/g, "");
    if (so.length < 4)
        return "số của bạn";
    return "••• ••• " + so.slice(-3);
}
let dangCho = null;
export function datOtpDangCho(moi) {
    dangCho = moi;
}
export function layOtpDangCho() {
    return dangCho;
}
export function xoaOtpDangCho() {
    dangCho = null;
}
