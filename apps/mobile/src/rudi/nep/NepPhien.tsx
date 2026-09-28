import { Pressable, ScrollView, StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../theme";
import { chuHienThi, type TraLoiSong } from "../ai/tra-loi-song";
import { cauTrangThaiNep, type LuotNep } from "./hoi";

/**
 * The turns of one panel session, and the answer being written under them
 * (slice 11, contract §4.1).
 *
 * Three looks for one answer, in the same place so nothing jumps:
 *   - thinking: the question as the next turn, and under it the panel's own
 *     line («Nếp đang nghĩ…», or the engine's sentence for the status code);
 *   - writing: the same question, and the answer growing in the very text
 *     style a finished answer has;
 *   - done: the question and the sealed answer are turns of the session, and
 *     the chips (xong.chips) sit under the answer.
 *
 * No animation: the «thinking» line is still text, as it always was, and a
 * moving indicator waits for `/impeccable` and the DESIGN.md change the
 * contract names (§7). Under Reduce Motion the growing text arrives a
 * sentence at a time instead of in 16-rune steps (`chuHienThi`).
 *
 * Presentational only, so the lab board draws every state without a server.
 */
export function NepPhien({
  luot,
  cauDangHoi,
  song,
  dangHoi,
  dangDo,
  chips,
  giamChuyenDong,
  onChip,
}: {
  luot: readonly LuotNep[];
  cauDangHoi: string | null;
  song: TraLoiSong | null;
  dangHoi: boolean;
  dangDo: string | null;
  chips: readonly string[];
  giamChuyenDong: boolean;
  onChip(chip: string): void;
}) {
  const { colors } = useRudiTheme();
  const chuSong = song ? chuHienThi(song, giamChuyenDong) : "";
  const coGi = luot.length > 0 || cauDangHoi !== null || dangHoi || dangDo !== null;
  if (!coGi) return null;
  return (
    <View style={styles.phien} testID="nep-phien">
      {luot.map((l, i) => (
        <Text
          // The session only ever grows at the end, so the index is stable.
          key={i}
          style={[typography.body, l.vai === "toi" ? styles.cauHoi : null, { color: l.vai === "toi" ? colors.inkSoft : colors.ink }]}
          testID={l.vai === "nep" ? "nep-tra-loi" : undefined}
        >
          {l.chu}
        </Text>
      ))}
      {chips.length > 0 && !dangHoi ? (
        <ScrollView contentContainerStyle={styles.chips} horizontal showsHorizontalScrollIndicator={false} testID="nep-chips">
          {chips.map((c) => (
            <Pressable
              accessibilityRole="button"
              key={c}
              onPress={() => onChip(c)}
              style={[styles.chip, { borderColor: colors.lineStrong, backgroundColor: colors.paper }]}
            >
              <Text style={[typography.caption, { color: colors.ink }]}>{c}</Text>
            </Pressable>
          ))}
        </ScrollView>
      ) : null}
      {cauDangHoi !== null ? (
        <Text style={[typography.body, styles.cauHoi, { color: colors.inkSoft }]} testID="nep-cau-dang-hoi">
          {cauDangHoi}
        </Text>
      ) : null}
      {chuSong !== "" ? (
        <Text style={[typography.body, { color: colors.ink }]} testID="nep-dang-viet">
          {chuSong}
        </Text>
      ) : dangHoi ? (
        <Text style={[typography.body, { color: colors.inkSoft }]} testID="nep-dang-nghi">
          {cauTrangThaiNep(song?.trangThai ?? null)}
        </Text>
      ) : null}
      {dangDo !== null ? (
        <Text style={[typography.body, { color: colors.inkSoft }]} testID="nep-dang-do">
          {dangDo}
        </Text>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  phien: { gap: 8, marginTop: 12 },
  cauHoi: { alignSelf: "flex-end", textAlign: "right" },
  chips: { gap: 8 },
  chip: { borderRadius: 999, borderWidth: StyleSheet.hairlineWidth, paddingHorizontal: 12, paddingVertical: 8 },
});
