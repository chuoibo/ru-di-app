import { Ionicons } from "@expo/vector-icons";
import { useEffect, useState } from "react";
import { Pressable, StyleSheet, View } from "react-native";

import type { RandomPlayableRound } from "../../chat/e2ee/random-live";
import { useRandomLiveTimeline, type RandomRoomClock } from "../../chat/e2ee/useRandomLive";
import { typography, useRudiTheme } from "../../theme";
import { RudiButton } from "../../ui";
import { Text } from "../../ui/NativeFontMetrics";
import { RandomGame } from "./RandomGame";

export function ChatRandomCard({ round, acceptedAt, clock, onNew, onView, onCreateOuting, onMenu, readOnly, onOpenPlace }: {
  round: RandomPlayableRound; acceptedAt: string; clock: RandomRoomClock; onNew: () => void; onView: (replay: boolean) => void;
  onCreateOuting: () => void; onMenu: () => void; readOnly: boolean;
  onOpenPlace: (id: string) => void;
}) {
  const { colors } = useRudiTheme();
  const [expanded, setExpanded] = useState(false);
  const [cardWidth, setCardWidth] = useState(0);
  const [watchedRound, setWatchedRound] = useState<string | null>(null);
  const live = useRandomLiveTimeline(round.v === 2 ? round : null, acceptedAt, clock);
  const active = live?.phase === "countdown" || live?.phase === "racing";
  useEffect(() => { if (active) setWatchedRound(round.id); }, [active, round.id]);
  const revealed = round.v === 1 || live?.reveal === true;
  const status = live?.phase === "countdown" ? `Bắt đầu sau ${Math.ceil(live.remainingMs / 1000)} giây`
    : live?.phase === "racing" ? "Lượt đang diễn theo lịch chung" : live?.reason ?? "Lượt đã khóa";
  // Keep the finishing pose for a viewer who watched; old history stays compact.
  const showGame = active || live?.phase === "finished" && (expanded || watchedRound === round.id);
  return <View onLayout={(event) => {
    const width = event.nativeEvent.layout.width;
    if (Number.isFinite(width) && width > 0) setCardWidth(width);
  }} style={[styles.card, { backgroundColor: colors.card, borderColor: colors.lineStrong }]} testID={`chat-random-${round.id}`}>
    <Pressable accessibilityRole="button" accessibilityLabel="Tuỳ chọn lượt chọn ngẫu nhiên" onLongPress={onMenu} onPress={onMenu} style={styles.summary}>
      <View style={styles.title}><Ionicons name={round.mode === "wheel" ? "disc-outline" : "flag-outline"} size={22} color={colors.accent} />
        <Text style={[typography.label, styles.flex, { color: colors.inkSoft }]}>{round.mode === "wheel" ? "Vòng quay chọn" : "Đua vịt chọn"}</Text></View>
      {revealed ? <Text style={[typography.h2, { color: colors.ink }]}>{round.options[round.winner].label}</Text>
        : <Text style={[typography.note, { color: colors.inkSoft }]}>{status}</Text>}
      {round.request ? <Text style={[typography.note, { color: colors.inkSoft }]}>{round.request}</Text> : null}
    </Pressable>
    {round.v === 2 && live && showGame && cardWidth > 32 ? <RandomGame mode={round.mode} options={round.options} round={round} live={live}
      width={Math.min(280, cardWidth - 32)} compact /> : null}
    <Pressable accessibilityRole="button" accessibilityState={{ expanded }} onPress={() => setExpanded(!expanded)} style={styles.expand}>
      <Text style={[typography.label, styles.flex, { color: colors.inkSoft }]}>{expanded ? "Thu các lựa chọn" : `Xem ${round.options.length} lựa chọn`}</Text>
      <Ionicons name={expanded ? "chevron-up" : "chevron-down"} size={18} color={colors.inkSoft} />
    </Pressable>
    {expanded ? <View style={styles.choices}>{round.options.map((option, index) => <View key={index} style={styles.choice}>
      <Text style={[typography.note, { color: colors.ink }]}>{index + 1}. {option.label}{revealed && index === round.winner ? " · Được chọn" : ""}</Text>
      {option.placeId ? <RudiButton compact full={false} variant="ghost" label="Xem quán đính kèm" icon="open-outline" onPress={() => onOpenPlace(option.placeId!)} /> : null}
    </View>)}
      <Text style={[typography.note, { color: colors.inkSoft }]}>Lượt do người tổ chức tạo. Dẫn nguồn đính kèm không phải xác nhận của AI.</Text>
      <Text style={[typography.note, { color: colors.inkSoft }]}>{round.v === 1 ? "Lượt cũ · xem lại riêng trên máy này." : "Lịch chung dành cho client đã cập nhật. Mở muộn sẽ vào pha hiện tại."}</Text>
      {!readOnly ? <RudiButton compact label={revealed ? "Xem lại trên máy này" : "Mở lượt"} variant="ghost" full={false} onPress={() => onView(revealed)} /> : null}</View> : null}
    {!readOnly ? <View style={[styles.actions, { borderTopColor: colors.line }]}>
      {revealed ? <RudiButton compact full={false} label="Tạo kèo" onPress={onCreateOuting} /> : null}
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
