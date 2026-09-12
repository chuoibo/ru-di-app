import type { ReactNode } from "react";
import { StyleSheet, View, type StyleProp, type ViewStyle } from "react-native";

import { useRudiTheme } from "../theme";
import { NepGoc } from "./art/Motif";

/**
 * The two-person notebook's sheet: a letter folded in thirds (spec «Nếp truyền
 * giấy» §1.6, §15.3, §16).
 *
 * ## What it is, in tokens
 *
 * `paper` ground, a hairline edge in `lineStrong` -- not `line`: `line` sits
 * at 1.20:1 on the light ground and 1.50:1 on the measured dark one, below the
 * 3:1 a non-text edge needs, while `lineStrong` reads 4.09:1 and 4.34:1 (spec
 * §16.2). `radius.small`, the radius the other paper in this app has (`KyHoa`,
 * `KhungAnh`): the first cut used `radius.base` and a blind read called three
 * of these «ba tấm thẻ» -- four equal 20dp corners are a card's signature, not
 * a sheet's. No shadow: elevation is declared once, and a sheet has an edge,
 * not a drop. When it carries the folded corner, that corner is SQUARE: a
 * fold cannot start on a rounded corner, and clipping the coral against one
 * (as the first cut did) reads as a badge, not a fold.
 *
 * ## What it is not
 *
 * Not a `Card`. A card holds anything; this holds the rows of one letter,
 * separated by `VetGap`, the crease. And it carries no text of its own -- the
 * rows do -- so it never appears in an accessibility tree as a label.
 *
 * `dan` marks the sheet that is the thing to do NOW: it gets the folded coral
 * corner (`NepGoc`), the one leading mark on the surface. The screen decides
 * which sheet that is; the component never does (spec §16.4).
 */
export function ToGiay({
  children,
  dan = false,
  style,
  testID,
}: {
  children: ReactNode;
  /** This sheet is the leading action on its surface: lay the coral corner on it. */
  dan?: boolean;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}) {
  const { colors, radius, space } = useRudiTheme();
  return (
    <View
      style={[
        styles.to,
        { backgroundColor: colors.paper, borderColor: colors.lineStrong, borderRadius: radius.small, padding: space.md },
        dan && styles.gocVuong,
        style,
      ]}
      testID={testID}
    >
      {children}
      {dan ? <NepGoc size={22} style={styles.goc} /> : null}
    </View>
  );
}

/**
 * The crease between two rows of the letter: one hairline of `paperShade`
 * across the sheet, EDGE TO EDGE. A plain `View`, not SVG -- a crease is a
 * line where the paper was folded, and it reads as one from the three even
 * rows around it, not from any shading trick (spec §16.3: the light scheme has
 * no colour brighter than `paper` to catch light on, so the crease is the same
 * one nét in both schemes). The negative horizontal margin pulls it through
 * the sheet's padding to the `lineStrong` edge: a crease that stops at the
 * padding is a table divider, which is exactly what the first blind read
 * called it.
 */
export function VetGap({ style }: { style?: StyleProp<ViewStyle> }) {
  const { colors, space } = useRudiTheme();
  return <View style={[styles.vet, { backgroundColor: colors.paperShade, marginVertical: space.sm, marginHorizontal: -space.md }, style]} />;
}

const styles = StyleSheet.create({
  to: { alignSelf: "stretch", borderWidth: StyleSheet.hairlineWidth },
  gocVuong: { borderTopRightRadius: 0 },
  goc: { position: "absolute", top: -StyleSheet.hairlineWidth, right: -StyleSheet.hairlineWidth },
  vet: { alignSelf: "stretch", height: StyleSheet.hairlineWidth },
});
