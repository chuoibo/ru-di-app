/**
 * A flap folded over the secondary words (ADR-0037 D1, D8): how a split is
 * worked out, the rules of a role, a tip -- words a person may want once and
 * then never again. The flap says what is under it; lifting it (a tap, Enter,
 * a screen reader's activate) shows the words on the paper's inner face.
 *
 * What must be READ before acting never goes under a flap (ADR-0037 D8,
 * «Nói Rõ Trước Khi Bấm»): the sentence one agrees to, the consequence of what
 * cannot be undone, what a number is and is not, the privacy line where data is
 * collected, «not end-to-end encrypted», and every error. Those stay on the page.
 */
import { Ionicons } from "@expo/vector-icons";
import { useState, type ReactNode } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import Animated, { FadeIn } from "react-native-reanimated";

import { typography, useRudiTheme } from "../theme";
import { useMotion } from "./useMotion";

export function NapGiay({ tieuDe, children, moSan = false, testID }: { tieuDe: string; children: ReactNode; moSan?: boolean; testID?: string }) {
  const { colors, radius } = useRudiTheme();
  const motion = useMotion();
  const [mo, setMo] = useState(moSan);
  return (
    <View style={styles.khoi} testID={testID}>
      <Pressable
        accessibilityLabel={tieuDe}
        accessibilityRole="button"
        accessibilityState={{ expanded: mo }}
        aria-expanded={mo}
        onPress={() => setMo((cu) => !cu)}
        style={({ pressed }) => [styles.nap, { backgroundColor: colors.card, borderColor: colors.lineStrong, borderRadius: radius.small, opacity: pressed ? 0.85 : 1 }]}
      >
        <Text style={[typography.label, styles.chu, { color: colors.ink }]}>{tieuDe}</Text>
        <Ionicons color={colors.inkSoft} importantForAccessibility="no" name={mo ? "chevron-up" : "chevron-down"} size={18} />
        {/* the flap's folded corner: what says «lift me» before any word does */}
        <View importantForAccessibility="no-hide-descendants" pointerEvents="none" style={[styles.goc, { borderTopColor: colors.paperShade, borderLeftColor: colors.card }]} />
      </Pressable>
      {mo ? (
        <Animated.View entering={FadeIn.duration(motion.ms("standard")).reduceMotion(motion.reanimated)} style={[styles.trong, { backgroundColor: colors.paperShade, borderRadius: radius.small }]}>
          {children}
        </Animated.View>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  khoi: { gap: 0 },
  nap: { minHeight: 48, flexDirection: "row", alignItems: "center", gap: 8, borderWidth: 1, paddingHorizontal: 14, paddingVertical: 10, overflow: "hidden" },
  chu: { flex: 1 },
  goc: { position: "absolute", right: 0, top: 0, width: 0, height: 0, borderTopWidth: 14, borderLeftWidth: 14 },
  trong: { marginTop: -6, paddingTop: 16, paddingBottom: 12, paddingHorizontal: 14, gap: 6 },
});
