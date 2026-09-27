/**
 * A postage stamp (ADR-0037 D1): a badge is a stamp in the passport, its rim
 * perforated all the way round. An earned stamp is printed paper; one not yet
 * earned is only its dashed outline, the place it will go.
 */
import type { ReactNode } from "react";
import { StyleSheet, View, type StyleProp, type ViewStyle } from "react-native";

import { hinhTem } from "../art/giay";
import { khungBo } from "../art/net";
import { NenGiay } from "./NenGiay";

export function Tem({
  children,
  rong = 84,
  cao = 100,
  khoa = false,
  style,
  testID,
  accessibilityLabel,
}: {
  children?: ReactNode;
  rong?: number;
  cao?: number;
  /** Not earned yet: the outline only. */
  khoa?: boolean;
  style?: StyleProp<ViewStyle>;
  testID?: string;
  accessibilityLabel?: string;
}) {
  return (
    <NenGiay
      accessibilityLabel={accessibilityLabel}
      hinh={(w, h) => {
        const vien = hinhTem(w, h, 8, 2.4);
        return { nen: vien, vien, them: khoa ? [] : [{ d: khungBo(7, 7, w - 14, h - 14, 2), mau: "bong", net: 1 }] };
      }}
      style={[styles.tem, { width: rong, height: cao }, style]}
      testID={testID}
      to={khoa ? "trong" : "card"}
      vienDut={khoa}
    >
      <View style={styles.giua}>{children}</View>
    </NenGiay>
  );
}

const styles = StyleSheet.create({
  tem: { padding: 10 },
  giua: { flex: 1, alignItems: "center", justifyContent: "center", gap: 4 },
});
