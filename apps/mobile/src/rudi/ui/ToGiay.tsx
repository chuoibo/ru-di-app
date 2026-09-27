import { LinearGradient } from "expo-linear-gradient";
import type { ReactNode } from "react";
import { StyleSheet, View, type StyleProp, type ViewStyle } from "react-native";
import Svg, { Line, Polygon, Polyline } from "react-native-svg";

import { phuMau, useRudiTheme } from "../theme";

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
 * not a drop.
 *
 * ## The folded corner is a FOLD, not a badge
 *
 * `dan` marks the sheet that is the thing to do NOW; it gets the coral corner,
 * the one leading mark on the surface (spec §16.4). Two cuts got this wrong
 * the same way: a coral triangle laid INSIDE a corner whose edge still ran to
 * the right angle, and two blind reads called it «badge», «nhãn dán», and on
 * the dark ground «notification» (finish reviews 12/09). Paper folded at the
 * corner is MISSING that corner: the sheet's edge turns along the diagonal,
 * whatever is behind the sheet shows through the cut, and the coral is the
 * back of the flap lying on the face. `GocGapThat` draws exactly that: an
 * erasing triangle in the ground colour over the corner, the flap, the cut
 * edge in `lineStrong` continuing the sheet's own edge, and the flap's two
 * free edges in ink. The corner is square because a fold cannot start on a
 * rounded one.
 *
 * `nen` is what shows through the cut. It defaults to the screen ground; a
 * sheet laid on any other surface names it.
 *
 * ## What it is not
 *
 * Not a `Card`. A card holds anything; this holds the rows of one letter,
 * separated by `VetGap`, the crease. And it carries no text of its own -- the
 * rows do -- so it never appears in an accessibility tree as a label. The
 * screen decides which sheet is `dan`; the component never does.
 */
export function ToGiay({
  children,
  dan = false,
  nen,
  style,
  testID,
}: {
  children: ReactNode;
  /** This sheet is the leading action on its surface: fold its corner in coral. */
  dan?: boolean;
  /** The colour behind the sheet, seen through the cut corner. Defaults to the screen ground. */
  nen?: string;
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
      {dan ? <GocGapThat c={GOC_GAP} gap={colors.accent} muc={colors.ink} nen={nen ?? colors.ground} vien={colors.lineStrong} /> : null}
    </View>
  );
}

/** Side of the folded corner, in dp: the size `KhungAnh` folds its print at. */
const GOC_GAP = 22;

/**
 * The corner of the sheet, folded. Sits on the sheet's outer corner (the
 * negative hairline offsets align it with the OUTSIDE of the border, so the
 * erasing triangle covers the border's own corner too). Order matters: erase,
 * flap, cut edge, flap edges.
 */
function GocGapThat({ c, gap, muc, nen, vien }: { c: number; gap: string; muc: string; nen: string; vien: string }) {
  // Inset the flap's free edges by half their stroke so the whole line lands
  // inside the viewport instead of being clipped to a half-width hairline.
  const o = 0.5;
  return (
    <Svg height={c} pointerEvents="none" style={styles.goc} width={c}>
      <Polygon fill={nen} points={`0,0 ${c},0 ${c},${c}`} />
      <Polygon fill={gap} points={`0,0 0,${c} ${c},${c}`} />
      <Line stroke={vien} strokeWidth={StyleSheet.hairlineWidth} x1={0} x2={c} y1={0} y2={c} />
      <Polyline fill="none" points={`${o},0 ${o},${c - o} ${c},${c - o}`} stroke={muc} strokeWidth={1} />
    </Svg>
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
  const { colors, dark, space } = useRudiTheme();
  // UI v3 (ADR-0037, plan S2 «gấp ba thật»): the panel after a crease lies at
  // a slight angle to the light, so a soft shade falls from the crease into
  // it and fades within a few dp. Drawn under the crease, taking no height.
  return (
    <View style={[styles.vet, { backgroundColor: colors.paperShade, marginVertical: space.sm, marginHorizontal: -space.md }, style]}>
      <LinearGradient colors={[phuMau(colors.ink, dark ? 0.18 : 0.06), phuMau(colors.ink, 0)]} pointerEvents="none" style={styles.bongGap} />
    </View>
  );
}

const styles = StyleSheet.create({
  to: { alignSelf: "stretch", borderWidth: StyleSheet.hairlineWidth },
  gocVuong: { borderTopRightRadius: 0 },
  bongGap: { position: "absolute", left: 0, right: 0, top: 1, height: 9 },
  goc: { position: "absolute", top: -StyleSheet.hairlineWidth, right: -StyleSheet.hairlineWidth },
  vet: { alignSelf: "stretch", height: StyleSheet.hairlineWidth },
});
