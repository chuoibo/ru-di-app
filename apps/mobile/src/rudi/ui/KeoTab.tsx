/**
 * A pull tab: the paper tongue a pop-up book sticks out of a page, pulled to
 * make something happen (ADR-0037 D1, D7).
 *
 * Every pop-up gesture has a tap and an accessibility twin (D7), so the tab is
 * three doors to one callback:
 *   - pull it along its direction past 60% of its travel and let go;
 *   - tap it (or press Enter on the web) -- it is a real button;
 *   - a screen reader's activate action.
 *
 * While the finger pulls, `tien` (0..1) reports how far, so the caller can
 * drive a stage with the same hand (a pull that half-raises a pop-up). The
 * tab moves only under the finger and settles back when released: it never
 * animates by itself (the «Control Đứng Yên» rule, ADR-0037 D3).
 */
import { Ionicons } from "@expo/vector-icons";
import { useMemo } from "react";
import { Pressable, StyleSheet, Text, View, type StyleProp, type ViewStyle } from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import Animated, { runOnJS, useAnimatedStyle, useSharedValue, withSpring, type SharedValue } from "react-native-reanimated";

import { typography, useRudiTheme } from "../theme";
import { useMotion } from "./useMotion";

/** How far (dp) the tab travels under the finger. */
const QUANG = 44;
/** Let go past this share of the travel and the pull counts. */
const NGUONG = 0.6;

export interface KeoTabProps {
  /** The words on the tab: what pulling it does. */
  nhan: string;
  /** What happens, for a screen reader (the hint after the label). */
  goiY?: string;
  onKeo: () => void;
  /** Written while the finger pulls: 0 at rest .. 1 at full travel. */
  tien?: SharedValue<number>;
  huong?: "phai" | "xuong";
  disabled?: boolean;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

export function KeoTab({ nhan, goiY, onKeo, tien, huong = "phai", disabled = false, style, testID }: KeoTabProps) {
  const { colors } = useRudiTheme();
  const motion = useMotion();
  const dich = useSharedValue(0);
  const quaNguong = useSharedValue(false);
  const daDem = useSharedValue(false);
  // Any drag that became a pull -- counted or not -- owns its touch: the web
  // still delivers a click to the button under the pointer when it lifts, and
  // a short pull used to fire as a tap (evidence run 24/09). Cleared when the
  // next touch begins, so a touch the OS cancelled never swallows a real tap.
  const vuaKeo = useSharedValue(false);
  const ngang = huong === "phai";

  const chon = () => motion.haptic.select();
  const daKeo = () => onKeo();
  const cuChi = useMemo(
    () =>
      Gesture.Pan()
        .enabled(!disabled)
        .activeOffsetX(ngang ? [-8, 8] : [-1000, 1000])
        .activeOffsetY(ngang ? [-1000, 1000] : [-8, 8])
        .onBegin(() => {
          vuaKeo.value = false;
        })
        .onStart(() => {
          vuaKeo.value = true;
        })
        .onUpdate((e) => {
          const t = Math.max(0, Math.min(QUANG, ngang ? e.translationX : e.translationY));
          dich.value = t;
          const p = t / QUANG;
          if (tien) tien.value = p;
          if (p >= NGUONG && !quaNguong.value) {
            quaNguong.value = true;
            runOnJS(chon)();
          } else if (p < NGUONG) {
            quaNguong.value = false;
          }
        })
        .onEnd(() => {
          if (dich.value / QUANG >= NGUONG) {
            daDem.value = true;
            runOnJS(daKeo)();
          }
        })
        .onFinalize(() => {
          quaNguong.value = false;
          dich.value = withSpring(0, motion.spring.settle);
          // A pull that counted hands `tien` to the caller, which carries it on
          // (a stage finishing its rise); one that did not falls back to rest.
          if (tien && !daDem.value) tien.value = withSpring(0, motion.spring.settle);
          daDem.value = false;
        }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [disabled, ngang, tien, motion.spring.settle],
  );
  const kieu = useAnimatedStyle(() => ({ transform: ngang ? [{ translateX: dich.value }] : [{ translateY: dich.value }] }));

  const bam = () => {
    if (vuaKeo.value) {
      vuaKeo.value = false;
      return;
    }
    onKeo();
  };

  return (
    <GestureDetector gesture={cuChi}>
      <Animated.View style={[styles.cho, kieu, style]}>
        <Pressable
          accessibilityActions={[{ name: "activate" }]}
          accessibilityHint={goiY}
          accessibilityLabel={nhan}
          accessibilityRole="button"
          accessibilityState={{ disabled }}
          disabled={disabled}
          onAccessibilityAction={(e) => {
            if (e.nativeEvent.actionName === "activate") onKeo();
          }}
          onPress={bam}
          style={({ pressed }) => [
            styles.tab,
            ngang ? styles.tabNgang : styles.tabDoc,
            { backgroundColor: colors.card, borderColor: colors.lineStrong, opacity: disabled ? 0.55 : pressed ? 0.85 : 1 },
          ]}
          testID={testID}
        >
          <View importantForAccessibility="no-hide-descendants" style={[styles.bang, ngang ? styles.bangNgang : styles.bangDoc, { backgroundColor: colors.accentSoft, borderColor: colors.accent }]}>
            {[0, 1, 2].map((i) => (
              <View key={i} style={[ngang ? styles.ranhDoc : styles.ranhNgang, { backgroundColor: colors.accent }]} />
            ))}
          </View>
          <Text numberOfLines={2} style={[typography.label, styles.chu, { color: colors.ink }]}>
            {nhan}
          </Text>
          <Ionicons color={colors.inkSoft} importantForAccessibility="no" name={ngang ? "chevron-forward" : "chevron-down"} size={18} />
        </Pressable>
      </Animated.View>
    </GestureDetector>
  );
}

const styles = StyleSheet.create({
  cho: { alignSelf: "flex-start" },
  tab: { minHeight: 48, flexDirection: "row", alignItems: "center", gap: 10, borderWidth: 1, paddingRight: 12 },
  // Flat on the side it leaves the page from, rounded on the end the finger takes.
  tabNgang: { borderTopRightRadius: 24, borderBottomRightRadius: 24, borderTopLeftRadius: 4, borderBottomLeftRadius: 4 },
  tabDoc: { borderBottomLeftRadius: 20, borderBottomRightRadius: 20, borderTopLeftRadius: 4, borderTopRightRadius: 4 },
  bang: { alignSelf: "stretch", alignItems: "center", justifyContent: "center", gap: 3, borderRightWidth: 1 },
  bangNgang: { width: 22, flexDirection: "row" },
  bangDoc: { width: 22 },
  ranhDoc: { width: 2, height: 14, borderRadius: 1 },
  ranhNgang: { width: 12, height: 2, borderRadius: 1 },
  chu: { flexShrink: 1 },
});
