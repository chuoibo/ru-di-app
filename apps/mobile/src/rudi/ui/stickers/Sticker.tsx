import { StyleSheet, View } from "react-native";
import Svg, { Path } from "react-native-svg";

import { KHUNG_STICKER, hinhSticker, nhanSticker, type MauSticker } from "../../chat/sticker";
import { mauSang, useRudiTheme } from "../../theme";

export interface StickerProps {
  id: string;
  /** Edge of the square the sticker is drawn in, in dp. */
  size?: number;
  /** A hair of rotation for a tile in the tray; rows draw it straight. */
  tilt?: -2 | -1 | 0 | 1 | 2;
}

/**
 * One sticker, drawn from the shape table in `chat/sticker.ts`.
 *
 * Colours come from the theme so a sticker reads the same on light and dark
 * ground; the only static colour is the ink laid on coral (`mauSang.ink`),
 * because coral is the same in both schemes. `transform` is only spread when
 * there is a tilt -- Reanimated turns `transform: undefined` into `null` and
 * crashes the first press (bài học 2026-09-05).
 */
export function Sticker({ id, size = 120, tilt = 0 }: StickerProps) {
  const { brand, colors } = useRudiTheme();
  // Under 72dp the compact reading, as `Nep.tsx` does: a second drawing, not a scale-down.
  const hinh = hinhSticker(id, { chiTiet: size >= 72 });
  const mau: Record<MauSticker, string> = {
    accent: colors.accent,
    ink: colors.ink,
    split: colors.split,
    card: colors.card,
    coral: brand.coral,
    line: colors.line,
  };
  const trenCoral = hinh.lop.some((l) => l.mau === "coral");
  return (
    <View
      accessibilityLabel={`Sticker: ${nhanSticker(id)}`}
      accessible
      style={[styles.khung, { width: size, height: size }, ...(tilt === 0 ? [] : [{ transform: [{ rotate: `${tilt}deg` }] }])]}
    >
      <Svg height={size} pointerEvents="none" viewBox={`0 0 ${KHUNG_STICKER} ${KHUNG_STICKER}`} width={size}>
        {hinh.lop.map((lop, i) => {
          const to = trenCoral && lop.mau === "ink" ? mauSang.ink : mau[lop.mau];
          // A stroke layer (the art layer's pen) draws round-capped, never filled.
          return lop.net === undefined ? (
            <Path d={lop.d} fill={to} key={i} />
          ) : (
            <Path d={lop.d} fill="none" key={i} stroke={to} strokeLinecap="round" strokeLinejoin="round" strokeWidth={lop.net} />
          );
        })}
      </Svg>
    </View>
  );
}

const styles = StyleSheet.create({
  khung: { alignItems: "center", justifyContent: "center" },
});
