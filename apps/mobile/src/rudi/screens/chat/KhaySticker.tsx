import { useState } from "react";
import { Pressable, StyleSheet, View, useWindowDimensions } from "react-native";

import { nhanSticker, stickerChoKhay, type StickerId } from "../../chat/sticker";
import { typography, useRudiTheme } from "../../theme";
import { Heading } from "../../ui";
import { Sheet } from "../../ui/Sheet";
import { Sticker } from "../../ui/stickers/Sticker";
import { Text, useNativeStandaloneTextMetrics } from "../../ui/NativeFontMetrics";

/**
 * The sticker tray (ADR-0021 §2.1): a sheet with one tile per id in the
 * vocabulary. Tapping a tile sends it as a message and closes the tray. The
 * tiles are labelled with the sticker's words so a flow taps «Đi thôi!» and a
 * screen reader hears the same.
 *
 * The vocabulary is locked, so labels grow naturally rather than lose words.
 * Measured columns include their gaps and the artwork's padding; percentages
 * previously overflowed the row and left three narrow columns on a phone.
 * Native caption metrics share the chat's owner and update during hot changes.
 */
export function KhaySticker({
  open,
  onClose,
  onChon,
  capDoi = false,
}: {
  open: boolean;
  onClose: () => void;
  onChon: (id: string) => void;
  /**
   * A couple (`cap_doi`: both turned «Một đôi» on): the four for two follow
   * under their own heading (ADR-0034). A friends' two-person chat gets a
   * group's eight (owner decision 2026-09-28).
   */
  capDoi?: boolean;
}) {
  const { colors, radius } = useRudiTheme();
  const { width } = useWindowDimensions();
  const [rongLuoi, setRongLuoi] = useState<number | null>(null);
  const caption = useNativeStandaloneTextMetrics(typography.caption);
  const budget = caption === null ? null : Math.max(
    (caption.fontSize ?? typography.caption.fontSize) / typography.caption.fontSize,
    (caption.lineHeight ?? typography.caption.lineHeight) / typography.caption.lineHeight,
  );
  const rong = Math.max(0, rongLuoi ?? Math.min(width, 640) - 32);
  // Four or two balanced columns; unknown metrics keep natural full-width text.
  const toiThieu = Math.max(96, Math.ceil(50 * (budget ?? 1)) + 18);
  const cot = budget === null ? 1 : rong >= 4 * toiThieu + 3 * 10 ? 4 : rong >= 2 * toiThieu + 10 ? 2 : 1;
  const beRong = Math.max(0, Math.floor((rong - (cot - 1) * 10) / cot * 10) / 10);
  const haiDong = caption?.lineHeight === undefined ? 0 : Math.ceil(caption.lineHeight * 2);
  return (
    <Sheet accessibilityLabel="Khay sticker" onClose={onClose} open={open} testID="khay-sticker">
      <Heading size="h2" subtitle="Một hình thay cho một câu." title="Sticker" />
      {nhom(stickerChoKhay(capDoi).chung)}
      {capDoi ? (
        <>
          <Text accessibilityRole="header" style={[typography.label, { color: colors.ink }]}>
            Cho hai người
          </Text>
          {nhom(stickerChoKhay(capDoi).doi)}
        </>
      ) : null}
    </Sheet>
  );

  function nhom(ids: readonly StickerId[]) {
    return (
      <View onLayout={(e) => { const w = Math.floor(e.nativeEvent.layout.width); if (w > 0 && w !== rongLuoi) setRongLuoi(w); }} style={styles.luoi}>
        {/* Each tile is a well of `ground` inside the `card` sheet, so a sticker
            drawn in paper (`card`) shows its faces here exactly as it does in a
            bubble on the page; on `card` tiles the paper vanished on the dark
            scheme (finish review 08/09). */}
        {ids.map((id, i) => (
          <Pressable
            accessibilityLabel={nhanSticker(id)}
            accessibilityRole="button"
            key={id}
            onPress={() => onChon(id)}
            style={({ pressed }) => [
              styles.o,
              { width: beRong, borderColor: colors.line, borderRadius: radius.control, backgroundColor: pressed ? colors.accentSoft : colors.ground },
            ]}
          >
            <Sticker id={id} size={64} tilt={i % 2 === 0 ? -1 : 1} />
            <Text style={[typography.caption, styles.nhan, { minHeight: haiDong, color: colors.inkSoft }]}>
              {nhanSticker(id)}
            </Text>
          </Pressable>
        ))}
      </View>
    );
  }
}

const styles = StyleSheet.create({
  luoi: { flexDirection: "row", flexWrap: "wrap", alignItems: "stretch", justifyContent: "center", gap: 10, paddingBottom: 8 },
  o: { alignItems: "center", gap: 8, paddingVertical: 10, paddingHorizontal: 8, borderWidth: 1 },
  // Stretch the paragraph to its tile and let the tallest label size its row.
  nhan: { alignSelf: "stretch", textAlign: "center" },
});
