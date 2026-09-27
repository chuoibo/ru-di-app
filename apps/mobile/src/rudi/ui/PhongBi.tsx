/**
 * A letter pulled out of its envelope (ADR-0037 D1): an invitation, a
 * request, a note for the two-person notebook. The letter (the children, on a
 * sheet) stands in front of the envelope's back and its open flap; the
 * envelope's pocket covers the letter's foot, so the letter reads as held in
 * it. The pocket only ever covers the sheet's own bottom margin: nothing the
 * letter says goes under it.
 */
import { useState, type ReactNode } from "react";
import { StyleSheet, View, type StyleProp, type ViewStyle } from "react-native";
import Svg, { Path } from "react-native-svg";

import { daGiac, netGay } from "../art/net";
import { bongGiay, useRudiTheme } from "../theme";

const TUI = 54;
const NAP = 30;
const LE = 12;

export function PhongBi({ children, style, testID }: { children?: ReactNode; style?: StyleProp<ViewStyle>; testID?: string }) {
  const { colors, dark, radius } = useRudiTheme();
  const [w, setW] = useState(0);
  return (
    <View onLayout={(e) => setW(Math.round(e.nativeEvent.layout.width))} style={[styles.khoi, style]} testID={testID}>
      {w > 0 ? (
        <Svg height={TUI + NAP} pointerEvents="none" style={styles.sau} viewBox={`0 0 ${w} ${TUI + NAP}`} width={w}>
          {/* the envelope's back and its open flap, behind the letter */}
          <Path d={daGiac([[0, NAP], [w / 2, 0], [w, NAP]])} fill={colors.paperShade} stroke={colors.lineStrong} strokeLinejoin="round" strokeWidth={1} />
          <Path d={daGiac([[0, NAP], [w, NAP], [w, NAP + TUI], [0, NAP + TUI]])} fill={colors.paperShade} />
        </Svg>
      ) : null}
      <View style={[styles.thu, { backgroundColor: colors.card, borderColor: colors.lineStrong, borderRadius: radius.small }, bongGiay(1, dark)]}>{children}</View>
      {w > 0 ? (
        <Svg height={TUI} pointerEvents="none" style={styles.tui} viewBox={`0 0 ${w} ${TUI}`} width={w}>
          {/* the pocket, in front of the letter's foot, with its two side folds */}
          <Path d={daGiac([[0, 0], [w, 0], [w, TUI], [0, TUI]])} fill={colors.card} stroke={colors.lineStrong} strokeWidth={1} />
          <Path d={netGay([[0, TUI], [w / 2, TUI * 0.42], [w, TUI]])} fill="none" stroke={colors.lineStrong} strokeLinejoin="round" strokeWidth={1} />
          <Path d={netGay([[0, 0], [w * 0.12, TUI * 0.5]])} fill="none" stroke={colors.line} strokeWidth={1} />
          <Path d={netGay([[w, 0], [w * 0.88, TUI * 0.5]])} fill="none" stroke={colors.line} strokeWidth={1} />
        </Svg>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  khoi: { paddingTop: 6 },
  sau: { position: "absolute", left: 0, bottom: 0 },
  // The letter ends a pocket's depth above the envelope's foot and keeps an
  // empty bottom margin the pocket can cover.
  thu: { marginHorizontal: LE, marginBottom: TUI * 0.45, borderWidth: 1, padding: 16, paddingBottom: TUI * 0.55 + 12, gap: 10 },
  tui: { position: "absolute", left: 0, bottom: 0 },
});
