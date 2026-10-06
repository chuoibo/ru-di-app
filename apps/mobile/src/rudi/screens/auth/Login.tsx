/** Password and provider proofs are kept in memory until the request finishes. */
import { Redirect, useLocalSearchParams, useRouter } from "expo-router";
import { useRef, useState } from "react";
import { StyleSheet, Text, View, useWindowDimensions } from "react-native";
import { AssetIllustration } from "../../asset-pack/react-native/AssetIllustration";
import { ApiError } from "../../../api";
import { googleLogin, googleRegister, loginAccount, usernameForLogin, type GoogleRegistration } from "../../account";
import { duongTiep, manSauDangNhap } from "../../duong-vao";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { Logo, RudiButton, RudiScreen } from "../../ui";
import { CoverBand } from "../../ui/CoverBand";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { PasswordField } from "../../ui/PasswordField";
import { GoogleButton } from "../../components/auth/GoogleButton";
export function LoginScreen() {
 const router = useRouter(); const { colors } = useRudiTheme(); const { phien, phienDaDoc, datPhien } = useRudiSession();
 const { width, height, fontScale } = useWindowDimensions();
 const artWidth = Math.min(512, width - 32); const showArt = height >= 760 && fontScale < 1.3;
 const tiep = duongTiep(useLocalSearchParams<{ tiep?: string }>().tiep) ?? undefined;
 const [username, setUsername] = useState(() => usernameForLogin() ?? ""); const [password, setPassword] = useState(""); const [registration, setRegistration] = useState<GoogleRegistration | null>(null);
 const [error, setError] = useState<string | null>(null); const [busy, setBusy] = useState(false); const lock = useRef(false);
 const run = async (action: () => Promise<void>) => { if (lock.current) return; lock.current = true; setBusy(true); setError(null); try { await action(); } catch (e) { setError(e instanceof ApiError ? e.message : "Chưa đăng nhập được. Hãy thử lại."); } finally { setPassword(""); setBusy(false); lock.current = false; } };
 const finish = (session: Awaited<ReturnType<typeof loginAccount>>) => { datPhien(session); router.replace(manSauDangNhap(session, tiep) as never); };
 if (!phienDaDoc) return null;
 if (phien) return <Redirect href={manSauDangNhap(phien, tiep) as never} />;
 return <RudiScreen surface="cover" material="quiet" avoidKeyboard cot="form" testID="login-screen"><CoverBand onBack underStatusBar compact><Logo compact ink={colors.coverInk} /><Text style={[typography.h1, { color: colors.coverInk }]}>Chào bạn</Text></CoverBand>
 <View style={styles.form}>
 <Text style={[typography.body, styles.intro, { color: colors.ink }]}>{registration ? "Chọn tên tài khoản để hoàn tất đăng ký Google." : "Đăng nhập để cùng bạn bè lên kế hoạch đi chơi."}</Text>
 <ONhapMuc label="Tên tài khoản" placeholder="@ten_cua_ban" autoCapitalize="none" autoCorrect={false} autoComplete="username" value={username} onChangeText={setUsername} editable={!busy} testID="account-username" />
 {!registration && <PasswordField label="Mật khẩu" placeholder="Nhập mật khẩu" autoComplete="current-password" textContentType="password" value={password} onChangeText={setPassword} editable={!busy} onSubmitEditing={() => void run(async () => finish(await loginAccount(username, password)))} testID="account-password" />}
 {error && <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{error}</Text>}
 <RudiButton label={registration ? "Tạo tài khoản Google" : "Đăng nhập"} loading={busy} disabled={busy || !username || (!registration && !password)} lyDo={!username ? (registration ? "Chọn tên tài khoản trước đã." : "Nhập tên tài khoản trước đã.") : !registration && !password ? "Nhập mật khẩu trước đã." : undefined} onPress={() => void run(async () => finish(registration ? await googleRegister({ challenge_id: registration.challenge_id, challenge_secret: registration.challenge_secret }, username) : await loginAccount(username, password)))} />
 {!registration && <><GoogleButton purpose="login" disabled={busy} onProof={(proof) => run(async () => { const out = await googleLogin(proof); if ("registration_required" in out) { setRegistration(out); setUsername(""); } else finish(out); })} onError={setError} />
 <RudiButton label="Tạo tài khoản" variant="outline" disabled={busy} lyDo={busy ? "Đang gửi yêu cầu tài khoản." : undefined} onPress={() => router.push((tiep ? `/register?tiep=${encodeURIComponent(tiep)}` : "/register") as never)} /><RudiButton label="Quên mật khẩu?" variant="outline" disabled={busy} lyDo={busy ? "Đang gửi yêu cầu tài khoản." : undefined} onPress={() => router.push((tiep ? `/reset-password?tiep=${encodeURIComponent(tiep)}` : "/reset-password") as never)} /></>}
 {registration && <RudiButton label="Bắt đầu lại" variant="outline" disabled={busy} lyDo={busy ? "Đang gửi yêu cầu tài khoản." : undefined} onPress={() => { setRegistration(null); setError(null); }} />}
 {showArt ? <View pointerEvents="none" style={[styles.art, { width: artWidth }]}>
  <AssetIllustration name="scene-cafe" width={Math.round(artWidth * 0.78)} height={156} fit="cover" style={styles.cafe} />
  <AssetIllustration name="decor-branch" width={142} height={112} resolution="small" style={styles.branch} />
 </View> : null}
 </View></RudiScreen>;
}

const styles = StyleSheet.create({
 form: { gap: 16, width: "100%", maxWidth: 560, alignSelf: "center", flexGrow: 1 },
 intro: { paddingTop: 6, paddingBottom: 2 },
 art: { height: 170, alignSelf: "center", marginTop: 4 },
 cafe: { position: "absolute", right: 0, bottom: 0, borderTopLeftRadius: 48, borderBottomRightRadius: 24 },
 branch: { position: "absolute", left: -8, bottom: -8, transform: [{ rotate: "-18deg" }] },
});
