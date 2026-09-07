/**
 * The «Ai có gì» pane: one row per person on the paper, a hairline between,
 * the dishes as a caption under the name; the dishes nobody has yet close the
 * list in the warning colour, because that is the one thing left to do here.
 */
import { StyleSheet, Text, View } from "react-native";

import { cauMonCuaNguoi, type AiCoGi as BangAiCoGi } from "../chia-bill/ai-co-gi";
import { typography, useRudiTheme } from "../theme";
import { SectionHeader } from "../ui";

export function AiCoGi({ bang }: { bang: BangAiCoGi }) {
  const { colors } = useRudiTheme();
  return (
    <View>
      <SectionHeader title="Ai có gì" />
      {bang.nguoi.map((n) => (
        <View key={n.id} style={[styles.hang, { borderBottomColor: colors.line }]}>
          <Text style={[typography.label, { color: colors.ink }]}>{n.ten}</Text>
          <Text style={[typography.caption, { color: n.mon.length === 0 ? colors.inkFaint : colors.inkSoft }]}>{cauMonCuaNguoi(n)}</Text>
        </View>
      ))}
      <Text accessibilityLiveRegion="polite" style={[typography.caption, styles.cuoi, { color: bang.chuaChon.length === 0 ? colors.inkSoft : colors.warn }]}>
        {bang.chuaChon.length === 0
          ? "Món nào cũng đã có người."
          : `${bang.chuaChon.length} món chưa có người: ${bang.chuaChon.join(", ")}`}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  hang: { gap: 2, minHeight: 52, justifyContent: "center", paddingVertical: 8, borderBottomWidth: StyleSheet.hairlineWidth },
  cuoi: { paddingTop: 10 },
});
