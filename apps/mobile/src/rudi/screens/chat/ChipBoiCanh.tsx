/**
 * The chip above the send button while the message being typed asks Rủ Đi AI
 * (ADR-0039 §2.3, proposed): «Kèm {n} tin gần đây · Xem · Chỉ gửi lời nhờ».
 *
 * It is the preview ADR-0036 §2.5 requires above the send button, folded to
 * one line. Every word comes from `chat/chip-boi-canh.ts`, which reads the
 * count from the bundle that will be sent, and «Xem» lists exactly that bundle
 * with the same pieces the AI tray's «Mình đang thấy» block used. Existing
 * components and tokens only; the finished look is a later slice.
 */
import { Ionicons } from "@expo/vector-icons";
import { useState } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View } from "react-native";

import { nhanVai, type BoiCanh } from "../../ai/boi-canh";
import { XEM, cauXem, chuChip } from "../../chat/chip-boi-canh";
import { typography, useRudiTheme } from "../../theme";
import { Sheet } from "../../ui/Sheet";

export function ChipBoiCanh({ goi, kemTin, sanSang, onDoi }: {
  /** The bundle that would go now, or null when this server takes none. */
  goi: BoiCanh | null;
  kemTin: boolean;
  sanSang: boolean;
  onDoi: (kemTin: boolean) => void;
}) {
  const { colors } = useRudiTheme();
  const [xem, setXem] = useState(false);
  const chu = chuChip(goi, kemTin, sanSang);
  return (
    <View style={[styles.chip, { backgroundColor: colors.aiSoft, borderColor: colors.line }]} testID="chat-chip-boi-canh">
      <Ionicons color={colors.ai} name="sparkles" size={15} />
      <Text accessibilityLiveRegion="polite" style={[typography.caption, styles.cau, { color: colors.ink }]} testID="chat-boi-canh">{chu.cau}</Text>
      {chu.xem && goi !== null ? (
        <Pressable accessibilityRole="button" accessibilityLabel="Xem những tin sẽ gửi kèm" hitSlop={12} onPress={() => setXem(true)} style={styles.nut} testID="chat-boi-canh-mo">
          <Text style={[typography.label, { color: colors.ai }]}>{XEM}</Text>
        </Pressable>
      ) : null}
      {chu.doi !== null ? (
        <Pressable accessibilityRole="button" hitSlop={12} onPress={() => onDoi(!kemTin)} style={styles.nut} testID="chat-boi-canh-doi">
          <Text style={[typography.label, { color: colors.ai }]}>{chu.doi}</Text>
        </Pressable>
      ) : null}
      <Sheet accessibilityLabel="Những tin sẽ gửi kèm lời nhờ" onClose={() => setXem(false)} open={xem && goi !== null}>
        {goi === null ? null : (
          <ScrollView style={styles.xem} testID="chat-boi-canh-luot">
            <Text style={[typography.body, { color: colors.ink }]}>{cauXem(goi)}</Text>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>
              Ảnh đi bằng chú thích, sticker đi bằng chữ «Sticker», tin đã xoá đi bằng một dòng nói là đã xoá. Tên hiển thị của các thành viên đi kèm để AI biết ai nói gì, còn chữ trong tin nhắn thì đi nguyên văn.
            </Text>
            {goi.luot.map((l) => (
              <Text key={l.id} style={[typography.caption, { color: colors.ink }]} testID="chat-boi-canh-muc">{`${nhanVai(l)}: ${l.chu}`}</Text>
            ))}
          </ScrollView>
        )}
      </Sheet>
    </View>
  );
}

const styles = StyleSheet.create({
  chip: { flexDirection: "row", alignItems: "center", flexWrap: "wrap", gap: 8, borderWidth: 1, borderRadius: 14, paddingHorizontal: 12, paddingVertical: 4, marginBottom: 6 },
  cau: { flexShrink: 1 },
  // 48dp touch targets on a one-line chip: the height comes from hitSlop.
  nut: { minHeight: 32, justifyContent: "center" },
  xem: { gap: 8 },
});
