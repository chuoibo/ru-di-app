import { useRouter } from "expo-router";
import { useEffect, useRef, useState } from "react";
import { Platform, Switch, Text, View } from "react-native";
import { ApiError } from "../../../api";
import { changeEmail, changePassword, getAccount, linkGoogle, logoutAll, reauthenticate, setDiscovery, unlinkGoogle, verifyEmail, type Account, type Challenge } from "../../account";
import type { Phien } from "../../../phien";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { OtpBoxes, RudiButton, RudiScreen, TopBar } from "../../ui";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { CuaDangNhap } from "../../ui/CuaDangNhap";
import { GoogleButton } from "../../components/auth/GoogleButton";
/** The server accepts a reauth for five minutes; the screen stops offering the link a little before that. */
const FRESH_MS = 270000;
/** Where a Google button can appear at all (Android Credential Manager, web GIS; iOS is a separate gate). */
const googleHere = () => !!process.env.EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID && (Platform.OS === "android" || Platform.OS === "web");
export function AccountScreen() {
 const router = useRouter(); const { colors } = useRudiTheme(); const { phien, phienDaDoc, datPhien, quenTaiCho } = useRudiSession();
 const [account, setAccount] = useState<Account | null>(null); const [current, setCurrent] = useState(""); const [password, setPassword] = useState(""); const [email, setEmail] = useState("");
 const [challenge, setChallenge] = useState<Challenge | null>(null); const [sentTo, setSentTo] = useState(""); const [code, setCode] = useState(""); const [notice, setNotice] = useState<string | null>(null); const [error, setError] = useState<string | null>(null); const [busy, setBusy] = useState(false); const lock = useRef(false);
 // When this screen last proved who is here: a reauth, or a session the server just rotated (a new session starts fresh).
 const [freshAt, setFreshAt] = useState<number | null>(null); const [now, setNow] = useState(() => Date.now());
 useEffect(() => { if (!phien) return; let live = true; void getAccount(phien.person_id).then(a => { if (live) setAccount(a); }).catch(() => { if (live) setError("Chưa tải được tài khoản."); }); return () => { live = false; }; }, [phien?.token]);
 useEffect(() => { setNow(Date.now()); if (freshAt === null) return; const timer = setTimeout(() => setNow(Date.now()), Math.max(0, freshAt + FRESH_MS - Date.now()) + 50); return () => clearTimeout(timer); }, [freshAt]);
 if (!phienDaDoc) return null;
 if (!phien) return <CuaDangNhap />;
 const person = phien.person_id;
 const fresh = freshAt !== null && now - freshAt < FRESH_MS;
 // A Google challenge is bound to the session that asked for it, and a link needs a fresh reauth: both remount the button.
 const googleKey = `${phien.token}:${freshAt ?? 0}`;
 const rotated = (next: Phien) => { datPhien(next); setFreshAt(Date.now()); };
 const run = async (action: () => Promise<void>) => { if (lock.current) return; lock.current = true; setBusy(true); setError(null); setNotice(null); try { await action(); } catch (e) { setError(e instanceof ApiError ? e.message : "Chưa lưu được. Hãy thử lại."); } finally { setBusy(false); lock.current = false; setCurrent(""); setPassword(""); } };
 return <RudiScreen testID="account-security-screen"><TopBar title="Tài khoản & bảo mật" /><View style={{ gap: 16, maxWidth: 560, alignSelf: "center", width: "100%" }}>
 {account && <><Text style={[typography.h2, { color: colors.ink }]}>@{account.username}</Text><Text style={[typography.body, { color: colors.inkSoft }]}>{account.email || "Chưa có email khôi phục"}</Text>
 <View style={{ flexDirection: "row", alignItems: "center", justifyContent: "space-between" }}><Text style={[typography.body, { color: colors.ink }]}>Cho bạn bè tìm theo username</Text><Switch accessibilityLabel="Cho tìm theo username" disabled={busy} value={account.discoverable_by_username} onValueChange={value => void run(async () => { await setDiscovery(person, value); setAccount({ ...account, discoverable_by_username: value }); })} /></View>
 <Text style={[typography.body, { color: colors.inkSoft }]}>Xác thực lại trước khi đổi mật khẩu, email hoặc liên kết Google. Lượt xác thực có hiệu lực 5 phút.</Text>
 {account.has_password && <><ONhapMuc label="Mật khẩu hiện tại" secureTextEntry autoComplete="current-password" value={current} onChangeText={setCurrent} editable={!busy} /><RudiButton label="Xác thực lại" disabled={busy || !current} lyDo={!current ? "Nhập mật khẩu hiện tại trước đã." : undefined} onPress={() => void run(async () => { await reauthenticate(person, { password: current }); setFreshAt(Date.now()); setNotice("Đã xác thực lại."); })} /></>}
 {account.has_google && <GoogleButton key={googleKey} purpose="reauth" actorId={person} disabled={busy} onError={setError} onProof={proof => run(async () => { await reauthenticate(person, { google: proof }); setFreshAt(Date.now()); setNotice("Đã xác thực lại."); })} />}
 <ONhapMuc label="Mật khẩu mới" helper="8–128 ký tự, tránh mật khẩu phổ biến." secureTextEntry autoComplete="new-password" value={password} onChangeText={setPassword} editable={!busy} /><RudiButton label={account.has_password ? "Đổi mật khẩu" : "Tạo mật khẩu"} variant="outline" disabled={busy || !password} lyDo={!password ? "Nhập mật khẩu mới trước đã." : undefined} onPress={() => void run(async () => { rotated(await changePassword(person, password)); setNotice("Đã đổi mật khẩu và thu hồi các phiên cũ."); })} />
 <ONhapMuc label="Email mới" autoCapitalize="none" autoCorrect={false} keyboardType="email-address" value={email} onChangeText={setEmail} editable={!busy} /><RudiButton label="Gửi mã xác minh email" variant="outline" disabled={busy || !email} lyDo={!email ? "Nhập email mới trước đã." : undefined} onPress={() => void run(async () => { setChallenge(await changeEmail(person, email)); setSentTo(email.trim()); setCode(""); setNotice(`Mã 6 số đã gửi tới ${email.trim()}.`); })} />
 {challenge && <><Text style={[typography.body, { color: colors.inkSoft }]}>{`Nhập mã 6 số gửi tới ${sentTo}. Mã có hiệu lực 5 phút.`}</Text><OtpBoxes length={6} value={code} onChange={setCode} disabled={busy} /><RudiButton label="Xác minh email mới" disabled={busy || code.length !== 6} lyDo={code.length !== 6 ? "Nhập đủ 6 số trong email." : undefined} onPress={() => void run(async () => { rotated(await verifyEmail(person, { challenge_id: challenge.challenge_id, challenge_secret: challenge.challenge_secret, code })); setChallenge(null); setCode(""); setEmail(""); setNotice("Đã đổi email và thu hồi các phiên cũ."); })} /></>}
 {account.has_google ? <RudiButton label="Gỡ liên kết Google" variant="outline" disabled={busy || !account.has_password} lyDo={!account.has_password ? "Tạo mật khẩu trước, để vẫn đăng nhập được khi gỡ Google." : undefined} onPress={() => void run(async () => { rotated(await unlinkGoogle(person)); setNotice("Đã gỡ liên kết Google."); })} />
 : fresh ? <GoogleButton key={googleKey} purpose="link" actorId={person} disabled={busy} onError={setError} onProof={proof => run(async () => { rotated(await linkGoogle(person, proof)); setNotice("Đã liên kết Google."); })} />
 : googleHere() && <Text style={[typography.body, { color: colors.inkSoft }]} testID="account-link-google-wait">Xác thực lại ở trên để liên kết Google.</Text>}
 <RudiButton label="Các phiên đăng nhập" variant="outline" onPress={() => router.push("/settings/phien" as never)} /><RudiButton label="Đăng xuất tất cả thiết bị" variant="outline" disabled={busy} onPress={() => void run(async () => { await logoutAll(person); quenTaiCho(); router.replace("/login" as never); })} />
 <Text style={[typography.caption, { color: colors.inkSoft }]}>Khôi phục tài khoản không khôi phục khóa chat mã hóa trên thiết bị đã mất.</Text></>}
 {notice && <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.ink }]}>{notice}</Text>}{error && <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{error}</Text>}
 </View></RudiScreen>;
}
