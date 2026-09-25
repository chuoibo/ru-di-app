/**
 * The notebook opened flat on a wide window (ADR-0037 D1, plan S0.5): two
 * pages with the spine between them -- the stage on the left page and the
 * work on the right, or the list and what it adds up to. On anything narrower
 * than `expanded` (840 dp) the pages stack in order, as direct children of the
 * screen's column, so the screen keeps its own rhythm.
 */
import type { ReactNode } from "react";
import { StyleSheet, View } from "react-native";

import { useRudiTheme } from "../theme";
import { useAdaptiveLayout } from "./useAdaptiveLayout";

export function SoMoHaiTrang({ trai, phai, tyLe = [1, 1] }: { trai: ReactNode; phai: ReactNode; tyLe?: [number, number] }) {
  const layout = useAdaptiveLayout();
  const { colors, space } = useRudiTheme();
  if (layout.sizeClass !== "expanded") {
    return (
      <>
        {trai}
        {phai}
      </>
    );
  }
  return (
    <View style={styles.trai}>
      <View style={[styles.trang, { flex: tyLe[0], paddingRight: space.lg }]}>{trai}</View>
      {/* The spine: a crease and the shade either side of it, where the two pages bend in. */}
      <View importantForAccessibility="no-hide-descendants" pointerEvents="none" style={styles.gay}>
        <View style={[styles.bong, { backgroundColor: colors.paperShade, opacity: 0.55 }]} />
        <View style={[styles.nep, { backgroundColor: colors.lineStrong }]} />
        <View style={[styles.bong, { backgroundColor: colors.paperShade, opacity: 0.55 }]} />
      </View>
      <View style={[styles.trang, { flex: tyLe[1], paddingLeft: space.lg }]}>{phai}</View>
    </View>
  );
}

const styles = StyleSheet.create({
  trai: { flexDirection: "row", alignItems: "stretch" },
  trang: { gap: 18, minWidth: 0 },
  gay: { flexDirection: "row", alignSelf: "stretch" },
  bong: { width: 6 },
  nep: { width: StyleSheet.hairlineWidth * 2 },
});
