/**
 * «Đăng nhập» on every tab while the person is looking at the demo.
 *
 * Signed out, the content tabs draw the fixture world, and none of them offered
 * a way to sign in: the only exit was Cá nhân → Tài khoản → «Đăng xuất bản trải
 * nghiệm» (QA UI-082, P1). This is one more destination in the tab strip itself
 * -- a column on a phone, an item at the foot of the rail -- so it is on every
 * tab, where the thumb already is, and it costs no height: a band above the bar
 * took 48dp from every screen and pushed the journey map's chooser off the map
 * on a 390×844 phone. Signing in returns to the tab it was pressed on. Live,
 * nothing renders.
 */
import { Ionicons } from "@expo/vector-icons";
import { usePathname, useRouter } from "expo-router";
import { StyleSheet, Text } from "react-native";

import { duongDangNhap } from "../duong-vao";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { PressScale } from "./PressScale";

export function useLaTraiNghiem(): boolean {
  return useRudiSession().cheDo !== "live";
}

export function DaiTraiNghiem({ rail = false }: { rail?: boolean }) {
  const { colors } = useRudiTheme();
  const router = useRouter();
  const pathname = usePathname();
  return (
    <PressScale
      accessibilityLabel="Bạn đang xem bản trải nghiệm. Đăng nhập"
      accessibilityRole="button"
      onPress={() => router.push(duongDangNhap(pathname) as never)}
      pressedScale={0.96}
      style={[styles.cot, rail && styles.railItem]}
      testID="dai-trai-nghiem"
    >
      <Ionicons color={colors.accent} name="log-in-outline" size={24} />
      <Text numberOfLines={2} style={[typography.caption, styles.nhan, { color: colors.accent }]}>
        Đăng nhập
      </Text>
    </PressScale>
  );
}

const styles = StyleSheet.create({
  cot: { alignItems: "center", flex: 1, gap: 2, justifyContent: "center", minHeight: 48, paddingTop: 8 },
  railItem: { flex: 0, height: 72, marginTop: "auto", paddingTop: 0 },
  nhan: { fontSize: 12, lineHeight: 14, textAlign: "center" },
});
