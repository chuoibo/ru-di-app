import { StyleSheet, View, type StyleProp, type ViewStyle } from "react-native";
import Svg, { Path } from "react-native-svg";

import type { LopVe, MauVe } from "../../art/net";
import { useRudiTheme, type RudiPalette } from "../../theme";

/**
 * The palette roles of `art/net.ts`, resolved from the theme: paper is the
 * `paper` token (the card tone by day; by night a sheet lighter than the
 * cloth ground, L* 22.7 over 9.2), the shaded fold is `paperShade` (always
 * darker than the face — `line` was lighter than `card` in the dark scheme,
 * which turned the fold inside out and the figure into a line diagram; review
 * 11/09 A3), ink is ink, the corner is the accent. A figure keeps its
 * silhouette on both grounds (report 07/09 §8.1: paper as a surface logic,
 * not a yellow sheet under a black filter).
 */
export function mauLop(colors: RudiPalette, mau: MauVe): string {
  switch (mau) {
    case "giay":
      return colors.paper;
    case "bong":
      return colors.paperShade;
    case "gap":
      return colors.accent;
    case "mo":
      return colors.accentSoft;
    case "split":
      return colors.split;
    case "ai":
      return colors.ai;
    case "muc":
    default:
      return colors.ink;
  }
}

export interface VeLopProps {
  lop: readonly LopVe[];
  /** The frame the layers were authored in. */
  khungW: number;
  khungH: number;
  width: number;
  height: number;
  /** Swap one role for another colour, e.g. ink for accent on a selected tile. */
  doiMau?: Partial<Record<MauVe, string>>;
  /** A frame narrower than the authored one, `x y w h`; default the whole frame. */
  viewBox?: string;
  /** Informative art carries a label and is an image; decorative art is hidden from the tree. */
  accessibilityLabel?: string;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

/**
 * Draws a layer list from the art tables. Strokes get round caps and joins so
 * a pen line ends like a pen line; fills are flat. `transform` never reaches
 * a style here, so there is no `transform: undefined` for Reanimated to trip on.
 */
export function VeLop({ lop, khungW, khungH, width, height, doiMau, viewBox, accessibilityLabel, style, testID }: VeLopProps) {
  const { colors } = useRudiTheme();
  const decorative = accessibilityLabel === undefined;
  return (
    <View
      accessibilityElementsHidden={decorative}
      accessibilityLabel={accessibilityLabel}
      accessibilityRole={decorative ? undefined : "image"}
      importantForAccessibility={decorative ? "no-hide-descendants" : "yes"}
      style={[styles.khung, { width, height }, style]}
      testID={testID}
    >
      <Svg height={height} pointerEvents="none" viewBox={viewBox ?? `0 0 ${khungW} ${khungH}`} width={width}>
        {lop.map((l, i) => {
          const mau = doiMau?.[l.mau] ?? mauLop(colors, l.mau);
          return l.net === undefined ? (
            <Path d={l.d} fill={mau} key={i} />
          ) : (
            <Path d={l.d} fill="none" key={i} stroke={mau} strokeLinecap="round" strokeLinejoin="round" strokeWidth={l.net} />
          );
        })}
      </Svg>
    </View>
  );
}

const styles = StyleSheet.create({
  khung: { alignItems: "center", justifyContent: "center" },
});
