/** Account proofs stay in component memory and request bodies, never routes or replay keys. */
import { ApiError, translatedAnonymous, translatedAsActor } from "../api";
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
 account_already_exists: "Tên tài khoản hoặc email đã được dùng. Hãy đăng nhập hoặc chọn tên khác.", email_unavailable: "Email này đã được một tài khoản khác dùng.",
 code_invalid: "Mã chưa đúng. Bạn có tối đa 5 lần thử.", challenge_invalid: "Lượt xác minh đã hết hạn hoặc không còn hiệu lực. Hãy bắt đầu lại.",
 challenge_resend_limited: "Chờ ít nhất 60 giây trước khi gửi lại mã.", auth_rate_limited: "Bạn đã thử nhiều lần. Hãy chờ một phút rồi thử lại.",
 challenge_quota_reached: "Bạn đã xin mã quá nhiều lần. Hãy thử lại sau 15 phút.", challenge_attempts_exhausted: "Đã nhập sai mã quá nhiều lần. Vì an toàn, hãy thử lại sau 24 giờ.",
 mail_unavailable: "Hôm nay hệ thống đã gửi hết lượt email. Hãy thử lại vào ngày mai.", google_unavailable: "Đăng nhập Google đang tạm gián đoạn. Hãy thử lại sau.",
 reauthentication_required: "Hãy xác thực lại trước khi đổi cài đặt bảo mật.", managed_account_required: "Phiên này không gắn với tài khoản Rủ Đi nào. Hãy đăng nhập bằng tài khoản của bạn.", last_login_method: "Hãy tạo mật khẩu với email đã xác minh trước khi gỡ Google.",
 verified_email_required: "Hãy xác minh email trước khi tạo mật khẩu.", google_already_linked: "Tài khoản đã có liên kết Google.",
 google_nonce_invalid: "Lượt Google không còn hiệu lực. Hãy bấm lại nút Google.", google_token_invalid: "Google chưa xác nhận được lượt này. Hãy thử lại.",
 auth_busy: "Máy chủ đang bận. Hãy thử lại sau ít giây.", auth_temporarily_unavailable: "Đăng nhập tạm gián đoạn. Hãy thử lại sau.",
};
/** On `/auth/login` the limiter counts failed passwords, not requests; the other doors keep the general sentence. */
const loginErrors = { ...errors, auth_rate_limited: "Bạn đã nhập sai nhiều lần. Hãy chờ 15 phút rồi thử lại, hoặc đặt lại mật khẩu bằng email." };
export const canonicalUsername = (value: string) => value.trim().replace(/^@/, "").toLowerCase();
export const validUsername = (value: string) => /^[a-z0-9._]{3,32}$/.test(canonicalUsername(value));
async function remember(wire: Phien) { const session = chonNhomMacDinh(wire); await ghiNho(session); usernameChoDangNhap = null; return session; }
export async function loginAccount(username: string, password: string) { return remember(await translatedAnonymous<Phien>(loginErrors, "/auth/login", { method: "POST", body: { username: canonicalUsername(username), password } })); }
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
/** What each Google button says, read aloud on Android and printed above it outside sign-in. */
export const googleLabel = { login: "Đăng nhập bằng Google", link: "Liên kết Google", reauth: "Xác thực bằng Google" } as const;
/** A Google failure in words: the server's refusal, Android's «no account on this phone», or a plain retry. */
export function googleErrorMessage(error: unknown) {
 if (error instanceof ApiError) return error.message;
 if ((error as { code?: unknown } | null)?.code === "ERR_NO_GOOGLE_ACCOUNT") return "Máy chưa có tài khoản Google. Thêm tài khoản Google trong Cài đặt của máy rồi thử lại.";
 return "Chưa kết nối được với Google. Hãy thử lại.";
}
/** The username a finished registration hands to the sign-in door, in memory only, until a sign-in succeeds.
 *  Read without consuming: a development render calls a state initializer twice. */
let usernameChoDangNhap: string | null = null;
export function handUsernameToLogin(username: string | null) { usernameChoDangNhap = username === null ? null : canonicalUsername(username) || null; }
export function usernameForLogin() { return usernameChoDangNhap; }
export function logoutAll(actorId: string) { return translatedAsActor(errors, "/sessions/all", { actorId, method: "DELETE" }); }
