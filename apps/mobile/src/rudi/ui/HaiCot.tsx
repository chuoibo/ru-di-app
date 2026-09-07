/**
 * Two panes on a wide window, one column on a phone.
 *
 * `twoPane` comes from the adaptive contract (`adaptive.ts`): expanded, or a
 * medium window that is tall enough. The left pane is the work (a list being
 * edited), the right pane is what that work adds up to -- kept beside it so
 * the reader never scrolls away from the thing they are deciding about. On a
 * phone the panes stack in the order given, as direct children of the screen's
 * column, so the screen's own rhythm (gap 18) applies unchanged.
 */
import type { ReactNode } from "react";
import { StyleSheet, View } from "react-native";

import { useRudiTheme } from "../theme";
import { useAdaptiveLayout } from "./useAdaptiveLayout";

export interface HaiCotProps {
  trai: ReactNode;
  phai: ReactNode;
  /** Flex weights, left then right. 3:2 puts the work first. */
  tyLe?: [number, number];
  /** The right pane only exists beside the left one: on a phone it is dropped, not stacked. */
  phaiChiKhiRong?: boolean;
}

export function HaiCot({ trai, phai, tyLe = [3, 2], phaiChiKhiRong = false }: HaiCotProps) {
  const layout = useAdaptiveLayout();
  const { space } = useRudiTheme();
  if (!layout.twoPane) {
    return (
      <>
        {trai}
        {phaiChiKhiRong ? null : phai}
      </>
    );
  }
  return (
    <View style={[styles.hang, { gap: space.lg }]}>
      <View style={[styles.cot, { flex: tyLe[0] }]}>{trai}</View>
      <View style={[styles.cot, { flex: tyLe[1] }]}>{phai}</View>
    </View>
  );
}

const styles = StyleSheet.create({
  hang: { flexDirection: "row", alignItems: "flex-start" },
  // The same vertical rhythm as the screen column (`screenInner` gap 18).
  cot: { gap: 18, minWidth: 0 },
});
