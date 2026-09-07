import type { ReactNode } from "react";
import { StyleSheet, Text, View, type StyleProp, type ViewStyle } from "react-native";

import { cardShadow, typography, useRudiTheme } from "../theme";

/**
 * A print stuck onto the page: the photograph in a paper frame with a thick
 * lower margin, and the provenance («ai · ở đâu · khi nào») written on it.
 *
 * The journal world keeps photographs as prints, not as bleed images: card
 * paper, a hairline, a soft offset shadow because a print sits ON the page
 * (a stamp sits IN it). The lead photograph of an album and a wall post get
 * the frame; small grid tiles stay bare. `chuThich` is the sentence the
 * person wrote, `xuatXu` the line the app knows to be true.
 */
export function KhungAnh({
  children,
  chuThich,
  xuatXu,
  tilt = 0,
  style,
  testID,
}: {
  children: ReactNode;
  chuThich?: string;
  xuatXu?: string;
  /** A hair of rotation for a print laid by hand; 0 in lists. */
  tilt?: -2 | -1 | 0 | 1 | 2;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}) {
  const { colors, radius } = useRudiTheme();
  return (
    <View
      style={[
        styles.khung,
        cardShadow,
        { backgroundColor: colors.card, borderColor: colors.line, borderRadius: radius.small },
        tilt !== 0 && { transform: [{ rotate: `${tilt}deg` }] },
        style,
      ]}
      testID={testID}
    >
      <View style={[styles.anh, { backgroundColor: colors.line }]}>{children}</View>
      {chuThich ? <Text style={[typography.body, styles.chu, { color: colors.ink }]}>{chuThich}</Text> : null}
      {xuatXu ? <Text numberOfLines={1} style={[typography.caption, styles.chu, { color: colors.inkFaint }]}>{xuatXu}</Text> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  khung: { padding: 8, paddingBottom: 14, borderWidth: StyleSheet.hairlineWidth, gap: 8 },
  anh: { borderRadius: 4, overflow: "hidden" },
  chu: { paddingHorizontal: 4 },
});
