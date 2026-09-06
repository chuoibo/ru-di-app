import { useEffect, useState } from "react";
import { Clipboard, Pressable, StyleSheet, Text, View } from "react-native";

import { PHAN_UNG, type LoaiPhanUng, type Tin } from "../../chat/tin-song";
import { typography, useRudiTheme } from "../../theme";
import { ListRow, RudiButton } from "../../ui";
import { Sheet } from "../../ui/Sheet";

/**
 * The long-press menu of one message (ADR-0021 §2.2–2.3): the six reactions
 * (same labels flow 30 taps), reply, copy, and -- for the sender's own text,
 * picture or sticker -- delete, behind a confirmation step inside the sheet
 * rather than an `Alert` (DESIGN.md: no system dialogs for product decisions).
 *
 * `Clipboard` is the React Native core one: deprecated but present in 0.86,
 * and swapping to `expo-clipboard` would cost a native rebuild this slice does
 * not have (ADR-0024 §2.5 batches those).
 */
export function MenuTin({
  tin,
  cuaToi,
  onClose,
  onPhanUng,
  onTraLoi,
  onXoa,
  onBaoCao,
}: {
  tin: Tin | null;
  cuaToi: boolean;
  onClose: () => void;
  onPhanUng: (tin: Tin, kind: LoaiPhanUng) => void;
  onTraLoi: (tin: Tin) => void;
  onXoa: (tin: Tin) => void;
  /** Present from L5; the row is hidden until then. */
  onBaoCao?: (tin: Tin) => void;
}) {
  const { colors } = useRudiTheme();
  const [xacNhanXoa, setXacNhanXoa] = useState(false);
  useEffect(() => {
    if (tin === null) setXacNhanXoa(false);
  }, [tin]);

  const xoaDuoc = cuaToi && tin !== null && (tin.kind === "text" || tin.kind === "image" || tin.kind === "sticker");

  return (
    <Sheet accessibilityLabel="Tin nhắn" onClose={onClose} open={tin !== null} testID="menu-tin">
      {tin === null ? null : xacNhanXoa ? (
        <View style={styles.khoi}>
          <Text style={[typography.title, { color: colors.ink }]}>Xoá tin này?</Text>
          <Text style={[typography.body, { color: colors.inkSoft }]}>
            Mọi người sẽ thấy “Tin nhắn đã bị xoá” thay cho nội dung. Ảnh đã gửi vẫn nằm trong kho ảnh của nhóm.
          </Text>
          <RudiButton label="Xoá tin" onPress={() => onXoa(tin)} variant="outline" />
          <RudiButton label="Giữ lại" onPress={() => setXacNhanXoa(false)} variant="ghost" />
        </View>
      ) : (
        <View style={styles.khoi}>
          <View style={[styles.thanhPhanUng, { borderColor: colors.line }]}>
            {PHAN_UNG.map((p) => (
              <Pressable
                accessibilityLabel={p.nhan}
                accessibilityRole="button"
                key={p.kind}
                onPress={() => onPhanUng(tin, p.kind)}
                style={styles.nutPhanUng}
              >
                <Text style={styles.glyph}>{p.glyph}</Text>
              </Pressable>
            ))}
          </View>
          {tin.kind !== "deleted" ? (
            <ListRow icon="return-up-back-outline" onPress={() => onTraLoi(tin)} title="Trả lời" />
          ) : null}
          {tin.kind === "text" && tin.body ? (
            <ListRow
              icon="copy-outline"
              onPress={() => {
                Clipboard.setString(tin.body ?? "");
                onClose();
              }}
              title="Sao chép"
            />
          ) : null}
          {xoaDuoc ? <ListRow icon="trash-outline" onPress={() => setXacNhanXoa(true)} title="Xoá" /> : null}
          {onBaoCao && !cuaToi ? (
            <ListRow icon="flag-outline" onPress={() => onBaoCao(tin)} title="Báo cáo" />
          ) : null}
        </View>
      )}
    </Sheet>
  );
}

const styles = StyleSheet.create({
  khoi: { gap: 6, paddingBottom: 4 },
  thanhPhanUng: { flexDirection: "row", justifyContent: "space-between", borderWidth: 1, borderRadius: 999, padding: 4, marginBottom: 6 },
  nutPhanUng: { width: 44, height: 44, alignItems: "center", justifyContent: "center" },
  glyph: { fontSize: 22 },
});
