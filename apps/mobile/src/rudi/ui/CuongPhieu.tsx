/**
 * A stub torn off a receipt (ADR-0037 D1): each person's share of a split is
 * one of these, the torn edge on top where it came off the bill, a strip of
 * the person's own ink down its side (D6) so the stub says whose it is before
 * a word is read. `noi` lifts the stub that matters most (yours) off the page
 * a paper height higher.
 */
import type { ReactNode } from "react";
import { StyleSheet, View, type StyleProp, type ViewStyle } from "react-native";

import { hinhCuong } from "../art/giay";
import { bienDoiDuong } from "../art/net";
import { NenGiay } from "./NenGiay";

const XE = 3;

export function CuongPhieu({
  children,
  mau,
  noi = false,
  style,
  testID,
}: {
  children?: ReactNode;
  /** The person's ink (`mucNguoi`): the strip down the stub's side. */
  mau?: string;
  noi?: boolean;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}) {
  return (
    <NenGiay
      cao={noi ? 2 : 1}
      chen={{ tren: XE * 2 }}
      hinh={(w, h) => {
        // `hinhCuong` tears the bottom; a stub came off the TOP of its bill.
        const c = hinhCuong(w, h, XE);
        const lat = [1, 0, 0, -1, 0, h] as const;
        return { nen: bienDoiDuong(c.nen, lat), vien: bienDoiDuong(c.vien, lat) };
      }}
      style={[styles.giay, style]}
      testID={testID}
    >
      {mau ? <View pointerEvents="none" style={[styles.soc, { backgroundColor: mau }]} /> : null}
      {children}
    </NenGiay>
  );
}

const styles = StyleSheet.create({
  giay: { paddingTop: XE * 2 + 12, paddingBottom: 14, paddingLeft: 20, paddingRight: 16, gap: 6 },
  soc: { position: "absolute", left: 8, top: XE * 2 + 10, bottom: 12, width: 4, borderRadius: 2 },
});
