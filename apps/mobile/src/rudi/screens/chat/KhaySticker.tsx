import { Pressable, StyleSheet, Text, View, useWindowDimensions } from "react-native";

import { nhanSticker, stickerChoKhay, type StickerId } from "../../chat/sticker";
import { typography, useRudiTheme } from "../../theme";
import { Heading } from "../../ui";
import { Sheet } from "../../ui/Sheet";
import { Sticker } from "../../ui/stickers/Sticker";

/**
 * The sticker tray (ADR-0021 §2.1): a sheet with one tile per id in the
 * vocabulary. Tapping a tile sends it as a message and closes the tray. The
 * tiles are labelled with the sticker's words so a flow taps «Đi thôi!» and a
 * screen reader hears the same.
 *
 * The label is allowed TWO lines and the grid gives up columns as the reader's
 * text grows. «Cà phê không?» was arriving as «Cà phê khôn…» in a 22%-wide
 * tile with `numberOfLines={1}` (review delta 08/09): the eight words are a
 * locked vocabulary shared with the server, so the layout is what has to give,
 * not the words. Two lines are reserved whether or not a label needs them, so
 * eight tiles stay the same height and the grid does not comb.
 */
export function KhaySticker({
  open,
  onClose,
  onChon,
  haiNguoi = false,
}: {
  open: boolean;
  onClose: () => void;
  onChon: (id: string) => void;
  /** A two-person conversation: the four for two follow under their own heading (ADR-0034). */
  haiNguoi?: boolean;
}) {
  const { colors, radius } = useRudiTheme();
  const { fontScale } = useWindowDimensions();
  // Four across at the default text size, then fewer as the words get bigger.
  // Below four the grid is still even: eight tiles divide by two and by four.
  // Written as literals rather than composed: `tests/receipt.test.mjs` reads
  // every «…%» a build can produce, because ADR-0009 forbids showing the model
  // a confidence percentage, and a computed one would land on that list.
  const beRong = fontScale >= 1.6 ? "48.5%" : fontScale >= 1.25 ? "31.3%" : "22.7%";
  return (
    <Sheet accessibilityLabel="Khay sticker" onClose={onClose} open={open} testID="khay-sticker">
      <Heading size="h2" subtitle="Một hình thay cho một câu." title="Sticker" />
      {nhom(stickerChoKhay(haiNguoi).chung)}
      {haiNguoi ? (
        <>
          <Text accessibilityRole="header" style={[typography.label, { color: colors.ink }]}>
            Cho hai người
          </Text>
          {nhom(stickerChoKhay(haiNguoi).doi)}
        </>
      ) : null}
    </Sheet>
  );

  function nhom(ids: readonly StickerId[]) {
    return (
      <View style={styles.luoi}>
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
            <Text numberOfLines={2} style={[typography.caption, styles.nhan, { color: colors.inkSoft }]}>
              {nhanSticker(id)}
            </Text>
          </Pressable>
        ))}
      </View>
    );
  }
}

const styles = StyleSheet.create({
  luoi: { flexDirection: "row", flexWrap: "wrap", gap: 10, paddingBottom: 8 },
  o: { alignItems: "center", gap: 4, paddingVertical: 8, paddingHorizontal: 2, borderWidth: 1 },
  // Two lines of caption, reserved: the tiles keep one height whether a label
  // wraps or not, so the grid stays a grid at every text size.
  nhan: { textAlign: "center", minHeight: 36 },
});
