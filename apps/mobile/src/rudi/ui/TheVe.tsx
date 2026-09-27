/**
 * A ticket (ADR-0037 D1): a plan (kèo) is a ticket for the group -- the main
 * part carries what and when, the stub on the right carries the one thing a
 * stub carries (a date, a count, a stamp), and the perforation between them
 * runs between two notches bitten out of the edges.
 */
import type { ReactNode } from "react";
import { StyleSheet, View, type StyleProp, type ViewStyle } from "react-native";

import { duongDut, hinhVe } from "../art/giay";
import type { CaoGiay } from "../theme";
import { NenGiay } from "./NenGiay";

const R = 12;
const R_KHUYET = 8;

export function TheVe({
  children,
  cuong,
  tiLeCat = 0.72,
  cao = 1,
  style,
  testID,
}: {
  children?: ReactNode;
  /** What the stub carries. */
  cuong?: ReactNode;
  /** Where the tear runs, as a share of the width. */
  tiLeCat?: number;
  cao?: CaoGiay;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}) {
  const ti = Math.min(0.85, Math.max(0.5, tiLeCat));
  return (
    <NenGiay
      boGoc={R}
      cao={cao}
      hinh={(w, h) => {
        const v = hinhVe(w, h, { r: R, rKhuyet: R_KHUYET, xCat: w * ti });
        return { nen: v.nen, vien: v.vien, them: [{ d: duongDut([v.xCat, R_KHUYET + 4], [v.xCat, h - R_KHUYET - 4], 3, 3), mau: "muc", net: 1.2 }] };
      }}
      style={[styles.theVe, style]}
      testID={testID}
    >
      <View style={[styles.chinh, { flex: ti }]}>{children}</View>
      <View style={[styles.cuong, { flex: 1 - ti }]}>{cuong}</View>
    </NenGiay>
  );
}

const styles = StyleSheet.create({
  theVe: { flexDirection: "row", minHeight: 96 },
  chinh: { paddingVertical: 16, paddingLeft: 18, paddingRight: 14, gap: 6 },
  cuong: { paddingVertical: 12, paddingHorizontal: 10, alignItems: "center", justifyContent: "center", gap: 4 },
});
