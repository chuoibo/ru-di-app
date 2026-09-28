/**
 * A thermal receipt (ADR-0037 D1): the paper a bill is, so a bill on screen is
 * that paper and not a form about it. The top and/or bottom edge is torn to
 * teeth; lines are printed in the system face with tabular figures, the way a
 * till prints them, and the cuts between sections are dotted.
 *
 * It carries no words of its own: `TieuDeHoaDon`, `DongHoaDon` and `VachCat`
 * are what a screen prints on it, all React Native text.
 */
import type { ReactNode } from "react";
import { StyleSheet, Text, View, type StyleProp, type ViewStyle } from "react-native";
import Svg, { Path } from "react-native-svg";

import { duongDut, hinhHoaDon } from "../art/giay";
import { typography, useRudiTheme, type CaoGiay } from "../theme";
import { NenGiay } from "./NenGiay";

const SAU = 5;

export function HoaDonGiay({
  children,
  rangTren = false,
  rangDuoi = true,
  cao = 1,
  style,
  testID,
}: {
  children?: ReactNode;
  rangTren?: boolean;
  rangDuoi?: boolean;
  cao?: CaoGiay;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}) {
  return (
    <NenGiay
      cao={cao}
      chen={{ tren: rangTren ? SAU : 0, duoi: rangDuoi ? SAU : 0 }}
      hinh={(w, h) => {
        const hd = hinhHoaDon(w, h, { rangTren, rangDuoi, buoc: 11, sau: SAU });
        return { nen: hd.nen, vien: hd.vien };
      }}
      style={[{ paddingTop: (rangTren ? SAU : 0) + 14, paddingBottom: (rangDuoi ? SAU : 0) + 16 }, styles.giay, style]}
      testID={testID}
    >
      {children}
    </NenGiay>
  );
}

/** The receipt's head: the place (condensed caps) and a line under it. */
export function TieuDeHoaDon({ ten, phu }: { ten: string; phu?: string }) {
  const { colors } = useRudiTheme();
  return (
    <View style={styles.dau}>
      <Text style={[typography.stamp, styles.ten, { color: colors.ink }]}>{ten}</Text>
      {phu ? <Text style={[typography.note, { color: colors.inkSoft, textAlign: "center" }]}>{phu}</Text> : null}
    </View>
  );
}

/** One printed line: what on the left, how much on the right, in tabular figures. */
export function DongHoaDon({ trai, phai, phu, dam = false, testID }: { trai: ReactNode; phai?: ReactNode; phu?: string; dam?: boolean; testID?: string }) {
  const { colors } = useRudiTheme();
  const chu = dam ? typography.title : typography.body;
  return (
    <View style={styles.dong} testID={testID}>
      <View style={styles.dongTren}>
        <View style={styles.trai}>{typeof trai === "string" ? <Text style={[chu, { color: colors.ink }]}>{trai}</Text> : trai}</View>
        {phai !== undefined ? (
          typeof phai === "string" ? (
            <Text style={[chu, styles.so, { color: colors.ink }]}>{phai}</Text>
          ) : (
            phai
          )
        ) : null}
      </View>
      {phu ? <Text style={[typography.note, { color: colors.inkSoft }]}>{phu}</Text> : null}
    </View>
  );
}

/** The dotted cut between two parts of a receipt. */
export function VachCat() {
  const { colors } = useRudiTheme();
  return (
    <View importantForAccessibility="no-hide-descendants" style={styles.vach}>
      <Svg height={2} preserveAspectRatio="none" viewBox="0 0 300 2" width="100%">
        <Path d={duongDut([1, 1], [299, 1], 4, 3)} stroke={colors.inkFaint} strokeLinecap="round" strokeWidth={1.2} />
      </Svg>
    </View>
  );
}

const styles = StyleSheet.create({
  giay: { paddingHorizontal: 16, gap: 8 },
  dau: { alignItems: "center", gap: 2, paddingBottom: 2 },
  ten: { fontSize: 15, lineHeight: 18, letterSpacing: 1.6, textAlign: "center" },
  dong: { gap: 2 },
  dongTren: { flexDirection: "row", alignItems: "flex-start", gap: 12 },
  trai: { flex: 1 },
  so: { fontVariant: ["tabular-nums"], textAlign: "right" },
  vach: { height: 2, marginVertical: 2 },
});
