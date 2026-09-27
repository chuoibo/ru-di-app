/**
 * The table a bill lies on, seen from above (ADR-0037 D1, plan S1 «bàn nhìn
 * từ trên xuống»): a board of paper with its rim and its shadow, a glass of
 * iced tea and a pair of chopsticks in the corner, and whatever the screen
 * puts on it -- the empty receipt, Nếp standing by with the camera.
 *
 * Decoration only: a screen reader skips the table and reads what is on it,
 * the props never take a touch, and nothing here moves.
 */
import { useState, type ReactNode } from "react";
import { StyleSheet, View, type LayoutChangeEvent, type StyleProp, type ViewStyle } from "react-native";
import Svg, { Circle, Ellipse, Line } from "react-native-svg";

import { bongGiay, phuMau, useRudiTheme } from "../theme";

/** Where the corner props sit, from the table's size: pure, so the layout is the same on every run. */
export function daoCuBan(w: number): { coc: { x: number; y: number; r: number }; dua: [number, number, number, number][] } {
  const r = Math.max(14, Math.min(20, w * 0.05));
  const coc = { x: w - r - 22, y: r + 20, r };
  const x0 = w - 2 * r - 66;
  return {
    coc,
    dua: [
      [x0, 22, x0 + 30, 22 + 2 * r + 34],
      [x0 + 9, 20, x0 + 38, 20 + 2 * r + 36],
    ],
  };
}

export function BanAn({ children, style, testID }: { children?: ReactNode; style?: StyleProp<ViewStyle>; testID?: string }) {
  const { colors, dark } = useRudiTheme();
  const [w, setW] = useState(0);
  const dc = w > 0 ? daoCuBan(w) : null;
  return (
    <View onLayout={(e: LayoutChangeEvent) => setW(Math.round(e.nativeEvent.layout.width))} style={[styles.ban, style]} testID={testID}>
      <View
        accessibilityElementsHidden
        importantForAccessibility="no-hide-descendants"
        pointerEvents="none"
        style={[StyleSheet.absoluteFill, styles.mat, { backgroundColor: colors.paperShade, borderColor: colors.line }, bongGiay(1, dark)]}
      >
        {dc ? (
          <Svg height={dc.coc.y + dc.coc.r + 60} width={w}>
            {dc.dua.map(([x1, y1, x2, y2], i) => (
              <Line key={i} stroke={colors.inkSoft} strokeLinecap="round" strokeWidth={3} x1={x1} x2={x2} y1={y1} y2={y2} />
            ))}
            {/* The glass from above: its shadow, the rim, the tea, one ice cube's glint. */}
            <Circle cx={dc.coc.x + 2} cy={dc.coc.y + 3} fill={phuMau(colors.ink, dark ? 0.3 : 0.08)} r={dc.coc.r + 1} />
            <Circle cx={dc.coc.x} cy={dc.coc.y} fill={colors.card} r={dc.coc.r} stroke={colors.line} strokeWidth={1.5} />
            <Circle cx={dc.coc.x} cy={dc.coc.y} fill={phuMau(colors.split, dark ? 0.3 : 0.16)} r={dc.coc.r - 4} />
            <Ellipse cx={dc.coc.x - dc.coc.r * 0.3} cy={dc.coc.y - dc.coc.r * 0.3} fill={colors.card} rx={dc.coc.r * 0.28} ry={dc.coc.r * 0.2} />
          </Svg>
        ) : null}
      </View>
      {children}
    </View>
  );
}

const styles = StyleSheet.create({
  ban: { padding: 16, paddingTop: 20 },
  mat: { borderRadius: 26, borderWidth: 1 },
});
