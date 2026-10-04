import { useRouter } from "expo-router";
import { useEffect, useRef, useState } from "react";
import { Text, View } from "react-native";
import { ApiError } from "../../../api";
import { registerAccount, resetConfirm, resetRequest, verifyAccount, type Challenge } from "../../account";
import { typography, useRudiTheme } from "../../theme";
import { OtpBoxes, RudiButton, RudiScreen, TopBar } from "../../ui";
import { ONhapMuc } from "../../ui/ONhapMuc";
export function RegisterScreen({ reset = false }: { reset?: boolean }) {
 const router = useRouter(); const { colors } = useRudiTheme(); const lock = useRef(false);
 const [username, setUsername] = useState(""); const [email, setEmail] = useState(""); const [password, setPassword] = useState("");
 const [challenge, setChallenge] = useState<Challenge | null>(null); const [code, setCode] = useState(""); const [sentAt, setSentAt] = useState(0);
 const [seconds, setSeconds] = useState(0); const [busy, setBusy] = useState(false); const [error, setError] = useState<string | null>(null); const [done, setDone] = useState(false);
 useEffect(() => { const tick = setInterval(() => setSeconds(Math.max(0, Math.ceil((sentAt + 60000 - Date.now()) / 1000))), 1000); return () => clearInterval(tick); }, [sentAt]);
 const run = async (action: () => Promise<void>) => { if (lock.current) return; lock.current = true; setBusy(true); setError(null); try { await action(); } catch (e) { setError(e instanceof ApiError ? e.message : "Chưa thực hiện được. Hãy thử lại."); } finally { setBusy(false); lock.current = false; } };
 const request = () => run(async () => { setChallenge(reset ? await resetRequest(email) : await registerAccount(username, email, password)); setPassword(""); setCode(""); setSentAt(Date.now()); setSeconds(60); });
 const verify = () => run(async () => { if (!challenge) return; const proof = { challenge_id: challenge.challenge_id, challenge_secret: challenge.challenge_secret, code }; if (reset) await resetConfirm(proof, password); else await verifyAccount(proof); setPassword(""); setCode(""); setChallenge(null); setDone(true); });
 return <RudiScreen testID={reset ? "reset-password-screen" : "register-screen"}><TopBar title={reset ? "Khôi phục tài khoản" : "Tạo tài khoản"} /><View style={{ gap: 16, width: "100%", maxWidth: 560, alignSelf: "center" }}>
 {done ? <><Text style={[typography.body, { color: colors.ink }]}>{reset ? "Mật khẩu đã đổi. Các phiên cũ đã được đăng xuất. Hãy đăng nhập lại." : "Email đã xác minh. Bạn có thể đăng nhập bằng tên tài khoản và mật khẩu."}</Text><RudiButton label="Đăng nhập" onPress={() => router.replace("/login" as never)} /></> : <>
 {!challenge && <>{!reset && <ONhapMuc label="Tên tài khoản" helper="3–32 chữ, số, dấu chấm hoặc gạch dưới. Bạn bè tìm bạn bằng tên này." autoCapitalize="none" autoCorrect={false} autoComplete="username" value={username} onChangeText={setUsername} editable={!busy} testID="register-username" />}<ONhapMuc label="Email" helper="Dùng để xác minh và khôi phục tài khoản." keyboardType="email-address" autoCapitalize="none" autoCorrect={false} autoComplete="email" value={email} onChangeText={setEmail} editable={!busy} testID="register-email" /></>}
 {(!reset && !challenge || reset && challenge) && <ONhapMuc label={reset ? "Mật khẩu mới" : "Mật khẩu"} helper="15–128 ký tự. Có thể dùng một câu dài và riêng biệt." secureTextEntry autoComplete="new-password" textContentType="newPassword" value={password} onChangeText={setPassword} editable={!busy} testID="register-password" />}
 {challenge ? <><Text style={[typography.body, { color: colors.inkSoft }]}>{reset ? "Nếu email thuộc tài khoản có mật khẩu, bạn sẽ nhận mã xác minh." : "Kiểm tra hộp thư của bạn và nhập mã 6 số. Mã có hiệu lực 5 phút."}</Text><Text style={[typography.label, { color: colors.ink }]}>Nhập mã 6 số</Text><OtpBoxes length={6} disabled={busy} value={code} onChange={setCode} />
 <RudiButton label={reset ? "Đổi mật khẩu" : "Xác minh email"} disabled={busy || code.length !== 6 || reset && !password} loading={busy} onPress={() => void verify()} /><RudiButton label={seconds > 0 ? `Gửi lại sau ${seconds} giây` : "Xin mã mới"} variant="outline" disabled={busy || seconds > 0} onPress={() => { setChallenge(null); setCode(""); setPassword(""); }} /></> : <RudiButton label={reset ? "Gửi mã khôi phục" : "Gửi mã xác minh"} loading={busy} disabled={busy || !email || !reset && (!username || !password)} onPress={() => void request()} />}
 {error && <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{error}</Text>}</>}
 </View></RudiScreen>;
}
