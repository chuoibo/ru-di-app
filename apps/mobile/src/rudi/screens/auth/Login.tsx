/** Password and provider proofs are kept in memory until the request finishes. */
import { Redirect, useLocalSearchParams, useRouter } from "expo-router";
import { useRef, useState } from "react";
import { Text, View } from "react-native";
import { ApiError } from "../../../api";
import { googleLogin, googleRegister, loginAccount, type GoogleRegistration } from "../../account";
import { duongTiep, manSauDangNhap } from "../../duong-vao";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { Logo, RudiButton, RudiScreen } from "../../ui";
import { CoverBand } from "../../ui/CoverBand";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { GoogleButton } from "../../components/auth/GoogleButton";
export function LoginScreen() {
 const router = useRouter(); const { colors } = useRudiTheme(); const { phien, phienDaDoc, datPhien } = useRudiSession();
 const tiep = duongTiep(useLocalSearchParams<{ tiep?: string }>().tiep) ?? undefined;
 const [username, setUsername] = useState(""); const [password, setPassword] = useState(""); const [registration, setRegistration] = useState<GoogleRegistration | null>(null);
 const [error, setError] = useState<string | null>(null); const [busy, setBusy] = useState(false); const lock = useRef(false);
 const run = async (action: () => Promise<void>) => { if (lock.current) return; lock.current = true; setBusy(true); setError(null); try { await action(); } catch (e) { setError(e instanceof ApiError ? e.message : "Chưa đăng nhập được. Hãy thử lại."); } finally { setPassword(""); setBusy(false); lock.current = false; } };
 const finish = (session: Awaited<ReturnType<typeof loginAccount>>) => { datPhien(session); router.replace(manSauDangNhap(session, tiep) as never); };
 if (!phienDaDoc) return null;
 if (phien) return <Redirect href={manSauDangNhap(phien, tiep) as never} />;
 return <RudiScreen surface="cover" testID="login-screen"><CoverBand onBack><Logo compact ink={colors.coverInk} /><Text style={[typography.h1, { color: colors.coverInk }]}>Chào bạn</Text></CoverBand>
 <View style={{ gap: 16, width: "100%", maxWidth: 560, alignSelf: "center" }}>
 <Text style={[typography.body, { color: colors.ink }]}>{registration ? "Chọn tên tài khoản để hoàn tất đăng ký Google." : "Đăng nhập để cùng bạn bè lên kế hoạch đi chơi."}</Text>
 <ONhapMuc label="Tên tài khoản" placeholder="@ten_cua_ban" autoCapitalize="none" autoCorrect={false} autoComplete="username" value={username} onChangeText={setUsername} editable={!busy} testID="account-username" />
 {!registration && <ONhapMuc label="Mật khẩu" secureTextEntry autoComplete="current-password" textContentType="password" value={password} onChangeText={setPassword} editable={!busy} onSubmitEditing={() => void run(async () => finish(await loginAccount(username, password)))} testID="account-password" />}
 {error && <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{error}</Text>}
 <RudiButton label={registration ? "Tạo tài khoản Google" : "Đăng nhập"} loading={busy} disabled={busy || !username || (!registration && !password)} onPress={() => void run(async () => finish(registration ? await googleRegister({ challenge_id: registration.challenge_id, challenge_secret: registration.challenge_secret }, username) : await loginAccount(username, password)))} />
 {!registration && <><GoogleButton purpose="login" disabled={busy} onProof={(proof) => run(async () => { const out = await googleLogin(proof); if ("registration_required" in out) { setRegistration(out); setUsername(""); } else finish(out); })} onError={setError} />
 <RudiButton label="Tạo tài khoản" variant="outline" disabled={busy} onPress={() => router.push("/register" as never)} /><RudiButton label="Quên mật khẩu?" variant="outline" disabled={busy} onPress={() => router.push("/reset-password" as never)} /></>}
 {registration && <RudiButton label="Bắt đầu lại" variant="outline" onPress={() => { setRegistration(null); setError(null); }} />}
 </View></RudiScreen>;
}
