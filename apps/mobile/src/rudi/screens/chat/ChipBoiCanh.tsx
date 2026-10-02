/**
 * The chip above the send button while the message being typed asks Rủ Đi AI
 * (ADR-0046 §2.3, proposed): «Kèm {n} tin gần đây · Xem · Chỉ gửi lời nhờ».
 *
 * It is the preview ADR-0036 §2.5 requires above the send button, folded to
 * one line. Every word comes from `chat/chip-boi-canh.ts`, which reads the
 * count from the bundle that will be sent, and «Xem» lists exactly that bundle
 * with the same pieces the AI tray's «Mình đang thấy» block used. Existing
 * components and tokens only; the finished look is a later slice.
 *
 * «Xem» opens `TamXemBoiCanh`, which the SCREEN mounts at its root, beside its
 * other sheets. A `Sheet` fills its nearest parent; mounted inside the chip it
 * filled the chip (lab 2026-09-28: a 356x40 scrim, the panel rising from the
 * chip over the thread, the rest of the screen neither dimmed nor blocked).
 */
import { Ionicons } from "@expo/vector-icons";
import { Pressable, ScrollView, StyleSheet, Text, View } from "react-native";

import type { BoiCanh } from "../../ai/boi-canh";
import { XEM, cauXem, cauXemCach, chuChip, gomTheoNguoi } from "../../chat/chip-boi-canh";
import { typography, useRudiTheme } from "../../theme";
import { Sheet } from "../../ui/Sheet";

export function ChipBoiCanh({ goi, kemTin, sanSang, onDoi, onXem, haiNguoi = false }: {
  /** The bundle that would go now, or null when this server takes none. */
  goi: BoiCanh | null;
  kemTin: boolean;
  sanSang: boolean;
  onDoi: (kemTin: boolean) => void;
  /** «Xem»: the screen opens its `TamXemBoiCanh` for this same bundle. */
  onXem: () => void;
  /** A two-person conversation: the words speak of two people, not a group. */
  haiNguoi?: boolean;
}) {
  const { colors } = useRudiTheme();
  const chu = chuChip(goi, kemTin, sanSang, haiNguoi);
  return (
    <View style={[styles.chip, { backgroundColor: colors.aiSoft, borderColor: colors.line }]} testID="chat-chip-boi-canh">
      {/* The mark and its sentence are one unit that never wraps apart: the
          sentence shrinks and breaks inside itself. The two actions are the
          unit that moves to a line of their own when the chip is narrow
          (QA UI-167: at 320dp the ✦ stood alone on the first line). */}
      <View style={styles.cauKhoi}>
        <Ionicons color={colors.ai} name="sparkles" size={15} style={styles.dau} />
        <Text accessibilityLabel={chu.nhanDoc} accessibilityLiveRegion="polite" style={[typography.note, styles.cau, { color: colors.ink }]} testID="chat-boi-canh">{chu.cau}</Text>
      </View>
      {(chu.xem && goi !== null) || chu.doi !== null ? (
        <View style={styles.nutKhoi}>
          {chu.xem && goi !== null ? (
            <Pressable accessibilityRole="button" accessibilityLabel="Xem những tin sẽ gửi kèm" hitSlop={12} onPress={onXem} style={styles.nut} testID="chat-boi-canh-mo">
              <Text style={[typography.label, { color: colors.ai }]}>{XEM}</Text>
            </Pressable>
          ) : null}
          {chu.doi !== null ? (
            <Pressable accessibilityRole="button" hitSlop={12} onPress={() => onDoi(!kemTin)} style={styles.nut} testID="chat-boi-canh-doi">
              <Text style={[typography.label, { color: colors.ai }]}>{chu.doi}</Text>
            </Pressable>
          ) : null}
        </View>
      ) : null}
    </View>
  );
}

/**
 * The «Xem» sheet: exactly the turns of the bundle the chip counted. Mount it
 * at the screen's root (a sibling of the thread and the composer, like
 * `KhaySticker`), so it covers and dims the whole screen and Back, Escape, the
 * scrim and «Đóng bảng» all close it.
 */
export function TamXemBoiCanh({ goi, open, onClose, haiNguoi = false }: {
  goi: BoiCanh | null;
  open: boolean;
  onClose: () => void;
  haiNguoi?: boolean;
}) {
  const { colors } = useRudiTheme();
  return (
    <Sheet accessibilityLabel="Những tin sẽ gửi kèm lời nhờ" onClose={onClose} open={open && goi !== null} testID="chat-boi-canh-tam">
      {goi === null ? null : (
        <View style={styles.xem} testID="chat-boi-canh-luot">
          {/* The promise stays in view while the transcript scrolls: it is
              the one sentence the sheet exists to keep (ADR-0036 §2.5). */}
          <Text style={[typography.body, { color: colors.ink }]}>{cauXem(goi, haiNguoi)}</Text>
          <ScrollView contentContainerStyle={styles.banGhi} style={[styles.cuon, { borderColor: colors.line }]}>
            {gomTheoNguoi(goi.luot).map((doan) => (
              <View key={doan.key} style={[styles.doan, doan.cuaToi && styles.doanToi]}>
                <Text style={[typography.caption, { color: colors.inkSoft }]}>{doan.nguoi}</Text>
                {doan.luot.map((l) => (
                  <View
                    accessibilityLabel={`${doan.nguoi}: ${l.chu}`}
                    accessible
                    key={l.id}
                    style={[styles.bongNho, { backgroundColor: doan.cuaToi ? colors.accentSoft : colors.card, borderColor: colors.line }]}
                    testID="chat-boi-canh-muc"
                  >
                    <Text style={[typography.note, { color: colors.ink }]}>{l.chu}</Text>
                  </View>
                ))}
              </View>
            ))}
          </ScrollView>
          <Text style={[typography.note, { color: colors.inkSoft }]}>{cauXemCach(haiNguoi)}</Text>
        </View>
      )}
    </Sheet>
  );
}

const styles = StyleSheet.create({
  chip: { flexDirection: "row", alignItems: "center", flexWrap: "wrap", columnGap: 12, rowGap: 0, borderWidth: 1, borderRadius: 14, paddingHorizontal: 12, paddingVertical: 4, marginBottom: 6 },
  // Grows into the line and shrinks before anything wraps; 140dp is the least
  // a sentence keeps before the actions give way to a line of their own.
  cauKhoi: { flexDirection: "row", alignItems: "center", gap: 8, flexGrow: 1, flexShrink: 1, flexBasis: 140, minHeight: 26 },
  dau: { flexShrink: 0 },
  cau: { flexShrink: 1 },
  nutKhoi: { flexDirection: "row", alignItems: "center", gap: 14, marginLeft: "auto" },
  // A real 48dp target on a 36dp chip: the button is 48 tall and gives 10dp
  // back above and below, so the chip keeps one line's height while the box a
  // finger (or a measuring tool) finds is the full 48 (DESIGN.md; QA measured
  // 32 when the height came from hitSlop, which the web does not have).
  nut: { minHeight: 48, marginVertical: -10, justifyContent: "center", paddingHorizontal: 2 },
  xem: { gap: 10, flexShrink: 1 },
  // The transcript scrolls inside the sheet; the sentences above and below it stay.
  cuon: { flexShrink: 1, borderTopWidth: StyleSheet.hairlineWidth, borderBottomWidth: StyleSheet.hairlineWidth },
  banGhi: { gap: 12, paddingVertical: 12 },
  doan: { gap: 3, alignItems: "flex-start", maxWidth: "88%" },
  doanToi: { alignSelf: "flex-end", alignItems: "flex-end" },
  bongNho: { borderWidth: StyleSheet.hairlineWidth, borderRadius: 12, paddingHorizontal: 10, paddingVertical: 6 },
});
