/**
 * The front door, on the real API.
 *
 * Until this screen existed the only button here that did anything was «Vào
 * bản trải nghiệm»; Google, Apple and the phone field set an error string and
 * stopped. ADR-0016 opens two doors that need no invitation, and this is the
 * first: a phone number, a six-digit code, a session (`POST /auth/otp/*`).
 *
 * ## What is deliberately not here
 *
 * - A password. There is none anywhere in the product, so there is nothing to
 *   forget and no «Quên mật khẩu?» to offer.
 * - The number in a route param or a log. It goes into ONE request body and
 *   into `otp-dang-cho.ts` (memory only) for the code screen to echo masked.
 * - The fixture door on a shipped build. The «Vào bản trải nghiệm…» button is
 *   rendered only when `CUA_FIXTURE_DEV` is true -- a development build with
 *   `EXPO_PUBLIC_RUDI_FIXTURE=1` -- because the Maestro table and the design
 *   measurements need it and nobody with a real account should ever land on it.
 *
 * Google is available only in configured native builds. Apple is not offered.
 *
 * ## The page after the cover (UI v2)
 *
 * The cover of the journal continues for its first third -- indigo band with
 * the logo, «Chào bạn» in the display face and one sentence -- then the paper
 * begins and the form sits directly on it. No card around a single field: a
 * frame around a frame was the tell the audit named.
 *
 * The band gives way to the keyboard: once the IME is up on a compact window
 * it drops its sentence and shrinks, so the field, its error and «Gửi mã»
 * stay in view (2026-09-06 review: the tall cover pushed the form under the
 * keyboard). The error prints directly under the field it is about.
 */
import { useRouter } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { useRef, useState } from "react";
import { Platform, StyleSheet, Text, View } from "react-native";

import { ApiError, thongDiepNguoiDoc } from "../../../api";
import { dangNhapGoogle, guiOtp } from "../../../phien";
import { googleConfigured, googleSession } from "../../google";
import { manSauDangNhap } from "../../duong-vao";
import { useRudiSession } from "../../session";
import { chuanHoaSo } from "../../../screens/vao-cua/danh-tinh";
import { CUA_FIXTURE_DEV } from "../../cua-fixture";
import { datOtpDangCho } from "../../otp-dang-cho";
import { typography, useRudiTheme } from "../../theme";
import { DemoBadge, Logo, RudiButton, RudiScreen } from "../../ui";
import { CoverBand } from "../../ui/CoverBand";
import { StampButton } from "../../ui/StampButton";
import { useAdaptiveLayout } from "../../ui/useAdaptiveLayout";
import { useKeyboardOpen } from "../../ui/useKeyboardOpen";
import { ONhapMuc } from "../../ui/ONhapMuc";

type Trang = { pha: "nhap" } | { pha: "dang-gui" } | { pha: "hong"; loi: string };

const googleWebClientId = process.env.EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID?.trim();
const googleIosClientId = process.env.EXPO_PUBLIC_GOOGLE_IOS_CLIENT_ID?.trim();

export function LoginScreen() {
  const router = useRouter();
  const { colors, space } = useRudiTheme();
  const layout = useAdaptiveLayout();
  const banPhim = useKeyboardOpen();
  const [phone, setPhone] = useState("");
  const [trang, setTrang] = useState<Trang>({ pha: "nhap" });
  const [thongBao, setThongBao] = useState<string | null>(null);
  const { datPhien } = useRudiSession();
  const googleLock = useRef(false);
  const [googleBusy, setGoogleBusy] = useState(false);
  const hasGoogle = googleConfigured(Platform.OS, googleWebClientId, googleIosClientId);

  const vaoGoogle = async () => {
    if (!hasGoogle || googleLock.current) return;
    googleLock.current = true;
    setGoogleBusy(true);
    setThongBao(null);
    try {
      // Load native code only when that platform's configured door is used.
      const { GoogleSignin, isErrorWithCode, statusCodes } = await import("@react-native-google-signin/google-signin");
      GoogleSignin.configure({ webClientId: googleWebClientId, iosClientId: googleIosClientId });
      try {
        if (Platform.OS === "android") await GoogleSignin.hasPlayServices({ showPlayServicesUpdateDialog: true });
        const phien = await googleSession(() => GoogleSignin.signIn(), dangNhapGoogle);
        if (phien !== null) {
          datPhien(phien);
          router.replace(manSauDangNhap(phien) as never);
        }
      } catch (error) {
        if (isErrorWithCode(error) && error.code === statusCodes.SIGN_IN_CANCELLED) return;
        throw error;
      }
    } catch (error) {
      setThongBao(error instanceof ApiError ? error.message : "Chưa đăng nhập được với Google. Bạn có thể thử lại hoặc dùng số điện thoại.");
    } finally {
      googleLock.current = false;
      setGoogleBusy(false);
    }
  };

  const gui = async () => {
    if (googleLock.current) return;
    setThongBao(null);
    const sach = phone.trim();
    if (chuanHoaSo(sach) === null) {
      setTrang({ pha: "hong", loi: "Chưa đúng dạng số di động Việt Nam: 10 chữ số, bắt đầu bằng 0." });
      return;
    }
    setTrang({ pha: "dang-gui" });
    googleLock.current = true;
    try {
      const daGui = await guiOtp(sach);
      datOtpDangCho({
        challengeId: daGui.challenge_id,
        phone: sach,
        guiLaiLuc: Date.now() + daGui.resend_after_seconds * 1000,
      });
      setTrang({ pha: "nhap" });
      router.push("/otp");
    } catch (error) {
      setTrang({
        pha: "hong",
        loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
      });
    } finally {
      googleLock.current = false;
    }
  };

  const dangGui = trang.pha === "dang-gui" || googleBusy;
  const compact = layout.sizeClass === "compact";
  const bleed = compact ? space.md : space.lg;
  // The short band: only when the keyboard has actually taken the room.
  const gon = banPhim && compact;

  return (
    <RudiScreen contentStyle={styles.screen} surface="cover" testID="login-screen">
      <StatusBar style="light" />
      <CoverBand bleed={bleed} compact={gon} onBack style={styles.band} underStatusBar>
        <Logo compact ink={colors.coverInk} />
        <Text style={[gon ? typography.h1 : typography.display, styles.chao, { color: colors.coverInk }]}>Chào bạn</Text>
        {gon ? null : (
          <Text style={[typography.body, styles.dan, { color: colors.coverInkSoft }]}>
            Nhập số di động để nhận mã 6 số qua tin nhắn. Chưa có tài khoản thì Rủ Đi tạo luôn, không cần mật khẩu.
          </Text>
        )}
      </CoverBand>
      {/* One reading width for the whole column: on a tablet the field group and the
          buttons below it used to sit on two different grids. */}
      <View style={styles.column}>
      <View style={styles.form}>
        {/* The number written on one pen line, large, as on an envelope (ADR-0037 D1, plan S7). */}
        <ONhapMuc
          accessibilityLabel="Ô số điện thoại"
          autoCapitalize="none"
          autoComplete="tel"
          co="lon"
          editable={!dangGui}
          keyboardType="phone-pad"
          label="Số điện thoại"
          onChangeText={(text) => {
            setPhone(text);
            if (trang.pha === "hong") setTrang({ pha: "nhap" });
          }}
          onSubmitEditing={() => void gui()}
          placeholder="Số di động của bạn"
          returnKeyType="send"
          textContentType="telephoneNumber"
          value={phone}
        />
        {trang.pha === "hong" ? (
          // Beside the field it is about, before the action: the person reads
          // what to fix where they are about to fix it.
          <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{trang.loi}</Text>
        ) : null}
        <StampButton disabled={dangGui} label="Gửi mã" loading={dangGui} onPress={() => void gui()} size="vua" testID="login-gui-ma" tilt={-1} />
      </View>
      <View style={styles.orRow}>
        <View style={[styles.orLine, { backgroundColor: colors.line }]} />
        <Text style={[typography.caption, { color: colors.inkFaint }]}>hoặc</Text>
        <View style={[styles.orLine, { backgroundColor: colors.line }]} />
      </View>
      <View style={styles.khac}>
        {hasGoogle ? <RudiButton
          icon="logo-google"
          label="Tiếp tục với Google"
          disabled={dangGui}
          loading={googleBusy}
          onPress={() => void vaoGoogle()}
          variant="outline"
        /> : null}
        <RudiButton
          icon="mail-open-outline"
          label="Tôi có lời mời"
          onPress={() => router.push("/moi")}
          variant="outline"
        />
        {thongBao ? (
          <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.inkSoft }]}>{thongBao}</Text>
        ) : null}
      </View>
      {CUA_FIXTURE_DEV ? (
        // Development builds only, and only when the operator asked. A store
        // build has neither switch and never renders this block.
        <View style={styles.cuaDev}>
          <DemoBadge label="Cửa dev: dữ liệu demo" />
          <RudiButton
            label="Vào bản trải nghiệm Team Đà Lạt"
            onPress={() => router.push("/personalization")}
            variant="soft"
          />
        </View>
      ) : null}
      {/* No Terms/Privacy claim until those pages exist to link to: a sentence
          that names documents nobody can open is a claim, not a footer. */}
      <Text style={[typography.caption, styles.phapLy, { color: colors.inkFaint }]}>
        Số điện thoại chỉ dùng để gửi mã và không hiển thị cho người khác.
      </Text>
      </View>
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  screen: { gap: 20 },
  band: { gap: 8 },
  chao: { marginTop: 4 },
  dan: { maxWidth: 520 },
  column: { gap: 20, maxWidth: 560, width: "100%", alignSelf: "center" },
  form: { gap: 12 },
  orRow: { flexDirection: "row", alignItems: "center", gap: 10 },
  orLine: { flex: 1, height: StyleSheet.hairlineWidth },
  khac: { gap: 10 },
  cuaDev: { gap: 8, alignItems: "center" },
  phapLy: { textAlign: "center", paddingHorizontal: 18 },
});
