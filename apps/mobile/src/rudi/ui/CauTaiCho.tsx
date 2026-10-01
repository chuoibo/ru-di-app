/**
 * The error sentence, said where the finger just was.
 *
 * DESIGN.md allows one kind of error: a `warn` sentence in place -- no toast,
 * no modal. The kit had no component for "in place", so ~60 screens wrote their
 * own line and put it where the layout had room: the top of a long page, the
 * foot of a scroll view. QA measured the result as sentences nobody saw -- at
 * y −2855 on the batch screen (UI-049), under the bottom edge on the community
 * composer (UI-133), at the end of a form whose button is a sticky footer
 * (UI-051). The screen looked as if nothing had happened.
 *
 * `CauTaiCho` is that sentence as a primitive:
 *
 * - it is rendered by the caller directly under the control that failed (or in
 *   a sticky footer just above the button), so position is the caller's one
 *   decision and the rest is fixed;
 * - it is announced politely, once, as it appears;
 * - on the web, if the caller's layout still leaves it outside the window, it
 *   scrolls the nearest scroller just enough to show it;
 * - it may carry one way forward (`hanhDong`) -- «Nhập tay», «Thử lại» -- and
 *   a screen offers «Thử lại» only for a refusal that pressing again can change
 *   (`laTuChoiVinhVien`);
 * - it fades in over `standard`, and appears at once under Reduce Motion.
 *
 * A failed reload never replaces data already on screen; the sentence sits
 * beside it (QA UI-030, UI-077, UI-083). That is the caller's half of the
 * contract, and the reason this is a line and not a state.
 */
import { Ionicons } from "@expo/vector-icons";
import { useEffect, useRef } from "react";
import { Platform, Pressable, StyleSheet, Text, View, type StyleProp, type ViewStyle } from "react-native";
import Animated, { FadeIn } from "react-native-reanimated";

import { typography, useRudiTheme } from "../theme";
import { useMotion } from "./useMotion";

export type CauTaiChoProps = {
  /** The sentence; `null` renders nothing, so a screen can keep one slot. */
  cau: string | null | undefined;
  /** One way forward, beside the sentence. */
  hanhDong?: { label: string; onPress: () => void };
  style?: StyleProp<ViewStyle>;
  testID?: string;
};

export function CauTaiCho({ cau, hanhDong, style, testID }: CauTaiChoProps) {
  const { colors } = useRudiTheme();
  const motion = useMotion();
  const ref = useRef<View>(null);

  // Web only: a sentence the layout still left off-screen is brought in, by
  // the smallest scroll that shows it. Native callers place it in view.
  useEffect(() => {
    if (!cau || Platform.OS !== "web") return;
    const element = ref.current as unknown as HTMLElement | null;
    if (!element?.getBoundingClientRect) return;
    const frame = requestAnimationFrame(() => {
      const r = element.getBoundingClientRect();
      if (r.top < 0 || r.bottom > window.innerHeight) {
        element.scrollIntoView({ block: "nearest", behavior: motion.reduced ? "auto" : "smooth" });
      }
    });
    return () => cancelAnimationFrame(frame);
  }, [cau, motion.reduced]);

  if (!cau) return null;
  return (
    <Animated.View
      entering={FadeIn.duration(motion.ms("standard")).reduceMotion(motion.reanimated)}
      ref={ref}
      style={[styles.hang, style]}
      testID={testID}
    >
      <View accessibilityLiveRegion="polite" aria-live="polite" style={styles.cau}>
        <Ionicons color={colors.warn} name="alert-circle-outline" size={18} style={styles.dau} />
        <Text style={[typography.body, styles.chu, { color: colors.warn }]}>{cau}</Text>
      </View>
      {hanhDong ? (
        <Pressable accessibilityRole="button" onPress={hanhDong.onPress} style={styles.hanhDong}>
          <Text style={[typography.label, styles.hanhDongChu, { color: colors.ink }]}>{hanhDong.label}</Text>
        </Pressable>
      ) : null}
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  hang: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", columnGap: 12, rowGap: 4 },
  cau: { flexDirection: "row", alignItems: "flex-start", gap: 6, flexShrink: 1, flexGrow: 1, flexBasis: 220 },
  // The icon sits on the first line's centre, not the paragraph's.
  dau: { marginTop: 3 },
  chu: { flexShrink: 1 },
  hanhDong: { minHeight: 48, justifyContent: "center", paddingHorizontal: 4 },
  hanhDongChu: { textDecorationLine: "underline" },
});
