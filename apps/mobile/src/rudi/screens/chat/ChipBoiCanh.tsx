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

import { nhanVai, type BoiCanh } from "../../ai/boi-canh";
import { XEM, cauXem, cauXemCach, chuChip } from "../../chat/chip-boi-canh";
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
      <Ionicons color={colors.ai} name="sparkles" size={15} />
      <Text accessibilityLiveRegion="polite" style={[typography.caption, styles.cau, { color: colors.ink }]} testID="chat-boi-canh">{chu.cau}</Text>
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
        <ScrollView style={styles.xem} testID="chat-boi-canh-luot">
          <Text style={[typography.body, { color: colors.ink }]}>{cauXem(goi, haiNguoi)}</Text>
          <Text style={[typography.caption, { color: colors.inkSoft }]}>{cauXemCach(haiNguoi)}</Text>
          {goi.luot.map((l) => (
            <Text key={l.id} style={[typography.caption, { color: colors.ink }]} testID="chat-boi-canh-muc">{`${nhanVai(l)}: ${l.chu}`}</Text>
          ))}
        </ScrollView>
      )}
    </Sheet>
  );
}

const styles = StyleSheet.create({
  chip: { flexDirection: "row", alignItems: "center", flexWrap: "wrap", gap: 8, borderWidth: 1, borderRadius: 14, paddingHorizontal: 12, paddingVertical: 4, marginBottom: 6 },
  cau: { flexShrink: 1 },
  // 48dp touch targets on a one-line chip: the height comes from hitSlop.
  nut: { minHeight: 32, justifyContent: "center" },
  xem: { gap: 8 },
});
