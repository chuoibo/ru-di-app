import { Ionicons } from "@expo/vector-icons";
import { useState } from "react";
import { Pressable, StyleSheet, View } from "react-native";

import type { RandomRound } from "../../chat/e2ee/random-v2";
import { typography, useRudiTheme } from "../../theme";
import { RudiButton } from "../../ui";
import { Text } from "../../ui/NativeFontMetrics";

export function ChatRandomCard({ round, onNew, onView, onCreateOuting, onMenu, readOnly, onOpenPlace }: {
  round: RandomRound; onNew: () => void; onView: () => void; onCreateOuting: () => void; onMenu: () => void; readOnly: boolean;
  onOpenPlace: (id: string) => void;
}) {
  const { colors } = useRudiTheme();
  const [expanded, setExpanded] = useState(false);
  return <View style={[styles.card, { backgroundColor: colors.card, borderColor: colors.lineStrong }]} testID={`chat-random-${round.id}`}>
    <Pressable accessibilityRole="button" accessibilityLabel="Tuỳ chọn lượt chọn ngẫu nhiên" onLongPress={onMenu} onPress={onMenu} style={styles.summary}>
      <View style={styles.title}><Ionicons name={round.mode === "wheel" ? "disc-outline" : "flag-outline"} size={22} color={colors.accent} />
        <Text style={[typography.label, styles.flex, { color: colors.inkSoft }]}>{round.mode === "wheel" ? "Vòng quay chọn" : "Đua vịt chọn"}</Text></View>
      <Text style={[typography.h2, { color: colors.ink }]}>{round.options[round.winner].label}</Text>
      {round.request ? <Text style={[typography.note, { color: colors.inkSoft }]}>{round.request}</Text> : null}
    </Pressable>
    <Pressable accessibilityRole="button" accessibilityState={{ expanded }} onPress={() => setExpanded(!expanded)} style={styles.expand}>
      <Text style={[typography.label, styles.flex, { color: colors.inkSoft }]}>{expanded ? "Thu các lựa chọn" : `Xem ${round.options.length} lựa chọn`}</Text>
      <Ionicons name={expanded ? "chevron-up" : "chevron-down"} size={18} color={colors.inkSoft} />
    </Pressable>
    {expanded ? <View style={styles.choices}>{round.options.map((option, index) => <View key={index} style={styles.choice}>
      <Text style={[typography.note, { color: colors.ink }]}>{index + 1}. {option.label}{index === round.winner ? " · Được chọn" : ""}</Text>
      {option.placeId ? <RudiButton compact full={false} variant="ghost" label="Xem quán đính kèm" icon="open-outline" onPress={() => onOpenPlace(option.placeId!)} /> : null}
    </View>)}
      <Text style={[typography.note, { color: colors.inkSoft }]}>Lượt do người tổ chức tạo. Dẫn nguồn đính kèm không phải xác nhận của AI.</Text>
      {!readOnly ? <RudiButton compact label="Xem lại lượt" variant="ghost" full={false} onPress={onView} /> : null}</View> : null}
    {!readOnly ? <View style={[styles.actions, { borderTopColor: colors.line }]}>
      <RudiButton compact full={false} label="Tạo kèo" onPress={onCreateOuting} />
      <RudiButton compact full={false} variant="ghost" label="Lượt mới" onPress={onNew} />
    </View> : null}
  </View>;
}
const styles = StyleSheet.create({
  card: { width: "100%", borderWidth: 1, borderRadius: 18, overflow: "hidden" }, summary: { gap: 8, padding: 16, minHeight: 48 },
  title: { flexDirection: "row", alignItems: "center", gap: 8 }, flex: { flex: 1, minWidth: 0 },
  expand: { minHeight: 48, flexDirection: "row", alignItems: "center", gap: 8, paddingHorizontal: 16, paddingVertical: 8 },
  choices: { paddingHorizontal: 16, paddingBottom: 12, gap: 8 }, actions: { flexDirection: "row", flexWrap: "wrap", padding: 12, gap: 8, borderTopWidth: 1 },
  choice: { gap: 2 },
});
