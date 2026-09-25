/**
 * The big seal pressed onto the page when a state becomes true (ADR-0037 D1,
 * D3): «ĐÃ GHI SỔ», «SỔ ĐÃ MỞ», «ĐÃ VỀ». A double rim, condensed caps, a
 * pressed tilt, and ink that took unevenly (the measured ink tile, the same
 * one the ask's seal wears), in the tone of what it certifies.
 *
 * `dong` lands it on `useNhipDau`'s three beats; `tre` holds the strike back
 * so the seal meets the puppet's hand (`dong-dau` strikes at 300 + 130 ms).
 * The words are React Native text: the seal is read, not looked at.
 */
import { StyleSheet, Text, View, type StyleProp, type ViewStyle } from "react-native";
import Animated, { useAnimatedStyle } from "react-native-reanimated";

import { displayFace, useRudiTheme } from "../theme";
import { Grain } from "./Grain";
import { useNhipDau } from "./useNhipDau";

export function DauLon({
  nhan,
  tone = "split",
  dong = false,
  tre = 0,
  tilt = -6,
  co = "lon",
  style,
  testID,
}: {
  nhan: string;
  tone?: "split" | "accent" | "ink";
  dong?: boolean;
  tre?: number;
  tilt?: number;
  /** `vua` for a seal pressed onto a page beside Nếp rather than across the screen. */
  co?: "lon" | "vua";
  style?: StyleProp<ViewStyle>;
  testID?: string;
}) {
  const { colors } = useRudiTheme();
  const muc = tone === "ink" ? colors.ink : colors[tone];
  const { roi, muc: an, lun } = useNhipDau(dong, { tre });
  const kieu = useAnimatedStyle(() => ({
    opacity: 0.2 + 0.8 * an.value,
    transform: [{ rotate: `${tilt}deg` }, { scale: (1.4 - 0.4 * roi.value) * lun.value }],
  }));
  return (
    <Animated.View accessibilityLabel={nhan} accessibilityRole="text" style={[styles.dau, { borderColor: muc }, kieu, style]} testID={testID}>
      <View pointerEvents="none" style={[styles.trong, co === "vua" && styles.trongVua, { borderColor: muc }]}>
        <View pointerEvents="none" style={styles.muc}>
          <Grain material="mucIn" opacity={0.18} />
        </View>
        <Text style={[styles.chu, co === "vua" && styles.chuVua, { color: muc }]}>{nhan}</Text>
      </View>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  dau: { alignSelf: "center", borderWidth: 3, borderRadius: 10, padding: 3 },
  trong: { borderWidth: 1, borderRadius: 7, paddingHorizontal: 18, paddingVertical: 8, overflow: "hidden", alignItems: "center" },
  muc: { position: "absolute", left: 0, right: 0, top: 0, bottom: 0 },
  chu: { fontFamily: displayFace.condensedBold, fontSize: 24, lineHeight: 28, letterSpacing: 2.2, textTransform: "uppercase" },
  trongVua: { paddingHorizontal: 12, paddingVertical: 5 },
  chuVua: { fontSize: 18, lineHeight: 22, letterSpacing: 1.6 },
});
