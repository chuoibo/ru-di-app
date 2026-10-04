/** Account proofs stay in component memory and request bodies, never routes or replay keys. */
import { translatedAnonymous, translatedAsActor } from "../api";
import { chonNhomMacDinh, ghiNho, type Phien } from "../phien";
export type Challenge = { challenge_id: string; challenge_secret: string; expires_in_seconds: number; resend_after_seconds: number };
export type Proof = Pick<Challenge, "challenge_id" | "challenge_secret"> & { code?: string };
export type GoogleChallenge = Proof & { nonce: string };
export type GoogleProof = Proof & { id_token: string };
export type GoogleRegistration = Proof & { registration_required: true };
export type Account = { username: string; email: string; has_password: boolean; has_google: boolean; discoverable_by_username: boolean };
const errors = {
 credentials_invalid: "Tên tài khoản hoặc mật khẩu chưa đúng.", username_invalid: "Tên tài khoản gồm 3–32 chữ, số, dấu chấm hoặc gạch dưới.",
 password_weak: "Dùng mật khẩu 8–128 ký tự, tránh mật khẩu phổ biến.", email_invalid: "Địa chỉ email chưa đúng.",
 account_already_exists: "Tên tài khoản hoặc email đã được dùng. Hãy đăng nhập hoặc chọn tên khác.", email_unavailable: "Email này đã được dùng.",
 code_invalid: "Mã chưa đúng. Bạn có tối đa 5 lần thử.", challenge_invalid: "Lượt xác minh đã hết hạn hoặc không còn hiệu lực. Hãy bắt đầu lại.",
 challenge_resend_limited: "Chờ ít nhất 60 giây trước khi gửi lại mã.", auth_rate_limited: "Bạn đã thử nhiều lần. Hãy chờ một phút rồi thử lại.",
 reauthentication_required: "Hãy xác thực lại trước khi đổi cài đặt bảo mật.", last_login_method: "Hãy tạo mật khẩu với email đã xác minh trước khi gỡ Google.",
 verified_email_required: "Hãy xác minh email trước khi tạo mật khẩu.", google_already_linked: "Tài khoản đã có liên kết Google.",
 google_nonce_invalid: "Lượt Google không còn hiệu lực. Hãy bấm lại nút Google.", google_token_invalid: "Google chưa xác nhận được lượt này. Hãy thử lại.",
 auth_busy: "Máy chủ đang bận. Hãy thử lại sau ít giây.", auth_temporarily_unavailable: "Đăng nhập tạm gián đoạn. Hãy thử lại sau.",
};
export const canonicalUsername = (value: string) => value.trim().replace(/^@/, "").toLowerCase();
export const validUsername = (value: string) => /^[a-z0-9._]{3,32}$/.test(canonicalUsername(value));
async function remember(wire: Phien) { const session = chonNhomMacDinh(wire); await ghiNho(session); return session; }
export async function loginAccount(username: string, password: string) { return remember(await translatedAnonymous<Phien>(errors, "/auth/login", { method: "POST", body: { username: canonicalUsername(username), password } })); }
export function registerAccount(username: string, email: string, password: string) { return translatedAnonymous<Challenge>(errors, "/auth/register", { method: "POST", body: { username: canonicalUsername(username), email, password } }); }
export function verifyAccount(proof: Proof) { return translatedAnonymous<{ verified: boolean }>(errors, "/auth/register/verify", { method: "POST", body: proof }); }
export function resetRequest(email: string) { return translatedAnonymous<Challenge>(errors, "/auth/password/reset/request", { method: "POST", body: { email } }); }
export function resetConfirm(proof: Proof, password: string) { return translatedAnonymous<{ reset: boolean }>(errors, "/auth/password/reset/confirm", { method: "POST", body: { ...proof, password } }); }
export function googleChallenge(purpose: "login" | "link" | "reauth", actorId?: string) { return purpose === "login" ? translatedAnonymous<GoogleChallenge>(errors, "/auth/google/challenge", { method: "POST", body: { purpose } }) : translatedAsActor<GoogleChallenge>(errors, "/auth/google/challenge", { actorId: actorId!, method: "POST", body: { purpose } }); }
export async function googleLogin(proof: GoogleProof) { const wire = await translatedAnonymous<Phien | GoogleRegistration>(errors, "/auth/google", { method: "POST", body: proof }); return "registration_required" in wire ? wire : remember(wire); }
export async function googleRegister(proof: Proof, username: string) { return remember(await translatedAnonymous<Phien>(errors, "/auth/google/register", { method: "POST", body: { ...proof, username: canonicalUsername(username) } })); }
export function getAccount(actorId: string) { return translatedAsActor<Account>(errors, "/people/me/account", { actorId, method: "GET" }); }
export function reauthenticate(actorId: string, proof: { password: string } | { google: GoogleProof }) { return translatedAsActor(errors, "/people/me/account/reauth", { actorId, method: "POST", body: proof }); }
export async function changePassword(actorId: string, password: string) { return remember(await translatedAsActor<Phien>(errors, "/people/me/account/password", { actorId, method: "PUT", body: { password } })); }
export function changeEmail(actorId: string, email: string) { return translatedAsActor<Challenge>(errors, "/people/me/account/email", { actorId, method: "POST", body: { email } }); }
export async function verifyEmail(actorId: string, proof: Proof) { return remember(await translatedAsActor<Phien>(errors, "/people/me/account/email/verify", { actorId, method: "POST", body: proof })); }
export async function linkGoogle(actorId: string, proof: GoogleProof) { return remember(await translatedAsActor<Phien>(errors, "/people/me/account/google", { actorId, method: "POST", body: proof })); }
export async function unlinkGoogle(actorId: string) { return remember(await translatedAsActor<Phien>(errors, "/people/me/account/google", { actorId, method: "DELETE" })); }
export function setDiscovery(actorId: string, discoverable: boolean) { return translatedAsActor(errors, "/people/me/account/discovery", { actorId, method: "PUT", body: { discoverable_by_username: discoverable } }); }
export function logoutAll(actorId: string) { return translatedAsActor(errors, "/sessions/all", { actorId, method: "DELETE" }); }
