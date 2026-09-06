import { Pressable, StyleSheet, Text, View } from "react-native";

import { STICKER_IDS, nhanSticker } from "../../chat/sticker";
import { typography, useRudiTheme } from "../../theme";
import { Heading } from "../../ui";
import { Sheet } from "../../ui/Sheet";
import { Sticker } from "../../ui/stickers/Sticker";

/**
 * The sticker tray (ADR-0021 §2.1): a sheet with one tile per id in the
 * vocabulary. Tapping a tile sends it as a message and closes the tray. The
 * tiles are labelled with the sticker's words so a flow taps «Đi thôi!» and a
 * screen reader hears the same.
 */
export function KhaySticker({
  open,
  onClose,
  onChon,
}: {
  open: boolean;
  onClose: () => void;
  onChon: (id: string) => void;
}) {
  const { colors, radius } = useRudiTheme();
  return (
    <Sheet accessibilityLabel="Khay sticker" onClose={onClose} open={open} testID="khay-sticker">
      <Heading size="h2" subtitle="Một hình thay cho một câu." title="Sticker" />
      <View style={styles.luoi}>
        {STICKER_IDS.map((id, i) => (
          <Pressable
            accessibilityLabel={nhanSticker(id)}
            accessibilityRole="button"
            key={id}
            onPress={() => onChon(id)}
            style={({ pressed }) => [
              styles.o,
              { borderColor: colors.line, borderRadius: radius.control, backgroundColor: pressed ? colors.accentSoft : colors.card },
            ]}
          >
            <Sticker id={id} size={64} tilt={i % 2 === 0 ? -1 : 1} />
            <Text numberOfLines={1} style={[typography.caption, { color: colors.inkSoft }]}>
              {nhanSticker(id)}
            </Text>
          </Pressable>
        ))}
      </View>
    </Sheet>
  );
}

const styles = StyleSheet.create({
  luoi: { flexDirection: "row", flexWrap: "wrap", gap: 10, paddingBottom: 8 },
  o: { width: "22%", flexGrow: 1, alignItems: "center", gap: 4, paddingVertical: 8, borderWidth: 1 },
});
