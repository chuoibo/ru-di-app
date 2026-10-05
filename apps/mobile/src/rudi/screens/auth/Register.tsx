import { useLocalSearchParams, useRouter } from "expo-router";
import { useEffect, useRef, useState } from "react";
import { Text, View } from "react-native";
import { ApiError } from "../../../api";
import { handUsernameToLogin, registerAccount, resetConfirm, resetRequest, verifyAccount, type Challenge } from "../../account";
import { duongTiep } from "../../duong-vao";
import { typography, useRudiTheme } from "../../theme";
import { OtpBoxes, RudiButton, RudiScreen, TopBar } from "../../ui";
import { ONhapMuc } from "../../ui/ONhapMuc";
/** The password stays in memory until the code is verified, so «Xin mã mới» can resend the same registration. */
export function RegisterScreen({ reset = false }: { reset?: boolean }) {
 const router = useRouter(); const { colors } = useRudiTheme(); const lock = useRef(false); const tiep = duongTiep(useLocalSearchParams<{ tiep?: string }>().tiep);
 const [username, setUsername] = useState(""); const [email, setEmail] = useState(""); const [password, setPassword] = useState("");
 const [challenge, setChallenge] = useState<Challenge | null>(null); const [sentTo, setSentTo] = useState(""); const [code, setCode] = useState(""); const [resendAt, setResendAt] = useState(0);
 const [seconds, setSeconds] = useState(0); const [busy, setBusy] = useState(false); const [error, setError] = useState<string | null>(null); const [done, setDone] = useState(false);
 useEffect(() => { const tick = setInterval(() => setSeconds(Math.max(0, Math.ceil((resendAt - Date.now()) / 1000))), 1000); return () => clearInterval(tick); }, [resendAt]);
 const run = async (action: () => Promise<void>) => { if (lock.current) return; lock.current = true; setBusy(true); setError(null); try { await action(); } catch (e) { setError(e instanceof ApiError ? e.message : "Chưa thực hiện được. Hãy thử lại."); } finally { setBusy(false); lock.current = false; } };
 const request = () => run(async () => { const next = reset ? await resetRequest(email) : await registerAccount(username, email, password); const wait = next.resend_after_seconds > 0 ? next.resend_after_seconds : 60; setChallenge(next); setSentTo(email.trim()); setCode(""); setResendAt(Date.now() + wait * 1000); setSeconds(wait); });
 const verify = () => run(async () => { if (!challenge) return; const proof = { challenge_id: challenge.challenge_id, challenge_secret: challenge.challenge_secret, code }; if (reset) await resetConfirm(proof, password); else await verifyAccount(proof); setPassword(""); setCode(""); setChallenge(null); setDone(true); });
 return <RudiScreen testID={reset ? "reset-password-screen" : "register-screen"}><TopBar title={reset ? "Khôi phục tài khoản" : "Tạo tài khoản"} /><View style={{ gap: 16, width: "100%", maxWidth: 560, alignSelf: "center" }}>
 {done ? <><Text style={[typography.body, { color: colors.ink }]}>{reset ? "Mật khẩu đã đổi. Các phiên cũ đã được đăng xuất. Hãy đăng nhập lại." : "Email đã xác minh. Bạn có thể đăng nhập bằng tên tài khoản và mật khẩu."}</Text><RudiButton label="Đăng nhập" onPress={() => { handUsernameToLogin(reset ? null : username); router.replace((tiep ? `/login?tiep=${encodeURIComponent(tiep)}` : "/login") as never); }} /></> : <>
 {!challenge && <>{!reset && <ONhapMuc label="Tên tài khoản" helper="3–32 chữ, số, dấu chấm hoặc gạch dưới. Bạn bè tìm bạn bằng tên này." autoCapitalize="none" autoCorrect={false} autoComplete="username" value={username} onChangeText={setUsername} editable={!busy} testID="register-username" />}<ONhapMuc label="Email" helper="Dùng để xác minh và khôi phục tài khoản." keyboardType="email-address" autoCapitalize="none" autoCorrect={false} autoComplete="email" value={email} onChangeText={setEmail} editable={!busy} testID="register-email" /></>}
 {(!reset && !challenge || reset && challenge) && <ONhapMuc label={reset ? "Mật khẩu mới" : "Mật khẩu"} helper="8–128 ký tự. Nên dùng một câu dài và riêng biệt." secureTextEntry autoComplete="new-password" textContentType="newPassword" value={password} onChangeText={setPassword} editable={!busy} testID="register-password" />}
 {challenge ? <><Text style={[typography.body, { color: colors.inkSoft }]} testID="register-sent-to">{reset ? `Nếu ${sentTo} thuộc một tài khoản có mật khẩu, mã xác minh đã được gửi tới đó. Mã có hiệu lực 5 phút.` : `Mã 6 số đã gửi tới ${sentTo}. Mã có hiệu lực 5 phút.`}</Text><Text style={[typography.label, { color: colors.ink }]}>Nhập mã 6 số</Text><OtpBoxes length={6} disabled={busy} value={code} onChange={setCode} />
 <RudiButton label={reset ? "Đổi mật khẩu" : "Xác minh email"} disabled={busy || code.length !== 6 || reset && !password} lyDo={code.length !== 6 ? "Nhập đủ 6 số trong email." : reset && !password ? "Nhập mật khẩu mới trước đã." : undefined} loading={busy} onPress={() => void verify()} />
 <RudiButton label="Xin mã mới" variant="outline" disabled={busy || seconds > 0} lyDo={seconds > 0 ? `Gửi lại được sau ${seconds} giây.` : undefined} onPress={() => void request()} />
 <RudiButton label={reset ? "Dùng email khác" : "Sửa thông tin"} variant="ghost" disabled={busy} onPress={() => { setChallenge(null); setCode(""); setError(null); }} /></> : <RudiButton label={reset ? "Gửi mã khôi phục" : "Gửi mã xác minh"} loading={busy} disabled={busy || !email || !reset && (!username || !password)} lyDo={!reset && !username ? "Nhập tên tài khoản trước đã." : !email ? "Nhập email trước đã." : !reset && !password ? "Nhập mật khẩu trước đã." : undefined} onPress={() => void request()} />}
 {error && <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{error}</Text>}</>}
 </View></RudiScreen>;
}
