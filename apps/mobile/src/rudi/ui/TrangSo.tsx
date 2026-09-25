/**
 * A page of the group's ledger (ADR-0037 D1): ruled paper with punched holes
 * down the binding and a double margin rule in the tone of what is written
 * on it -- teal for money. A settled bill is copied onto it, a collection
 * keeps its rows on it, the finance page is one.
 *
 * The rules, holes and margin are decoration drawn behind React Native
 * content: a screen reader reads only what the screen writes on the page.
 * The page is `card`, never dark `paper`, so every tone reads on it.
 */
import { useState, type ReactNode } from "react";
import { StyleSheet, Text, View, type StyleProp, type TextStyle, type ViewStyle } from "react-native";

import { hinhHoaDon } from "../art/giay";
import { phuMau, typography, useRudiTheme } from "../theme";
import { NenGiay } from "./NenGiay";

/** Left of the margin: the binding's holes. */
const LE = 34;
/** The page's top margin: the first rule sits one line below it. */
const TREN = 12;
/** Distance between two rules; a `DongSo` is exactly one line tall, so words sit on the rules. */
export const DONG_KE = 28;

export function TrangSo({
  children,
  tone = "split",
  dongKe = DONG_KE,
  ke = true,
  style,
  testID,
}: {
  children?: ReactNode;
  tone?: "split" | "accent";
  /** Rule the page; off when every row draws its own line (rows of uneven height). */
  ke?: boolean;
  /** Distance between two rules (dp). */
  dongKe?: number;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}) {
  const { colors, dark } = useRudiTheme();
  const [h, setH] = useState(0);
  const muc = phuMau(colors[tone], dark ? 0.55 : 0.45);
  const soDong = ke ? Math.max(0, Math.floor((h - TREN - 4) / dongKe)) : 0;
  const soLo = Math.max(2, Math.floor(h / 44));
  return (
    <NenGiay
      hinh={(w, hh) => hinhHoaDon(w, hh, { rangTren: false, rangDuoi: false })}
      onLayout={(e) => setH(Math.round(e.nativeEvent.layout.height))}
      style={[styles.trang, style]}
      testID={testID}
    >
      <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants" pointerEvents="none" style={StyleSheet.absoluteFill}>
        {Array.from({ length: soDong }, (_, i) => (
          <View key={`k${i}`} style={[styles.ke, { top: TREN + (i + 1) * dongKe, backgroundColor: colors.line }]} />
        ))}
        <View style={[styles.le, { left: LE - 6, backgroundColor: muc }]} />
        <View style={[styles.le, { left: LE - 3, backgroundColor: muc }]} />
        {Array.from({ length: soLo }, (_, i) => (
          <View key={`l${i}`} style={[styles.lo, { top: ((i + 0.5) * h) / soLo - 5, backgroundColor: colors.ground, borderColor: colors.line }]} />
        ))}
      </View>
      {children}
    </NenGiay>
  );
}

/**
 * One line written on the page, sitting on its rule: what on the left (in a
 * person's ink when it is a person), how much on the right in tabular
 * figures. `dau` is the page's heading line, in condensed caps.
 */
export function DongSo({ trai, phai, mau, dau = false, testID }: { trai: string; phai?: string; mau?: string; dau?: boolean; testID?: string }) {
  const { colors } = useRudiTheme();
  // Condensed caps stack two marks over a capital («SỔ»); a single line is
  // clipped to its box on the web, so the heading gets the room above.
  const chu: TextStyle = dau ? { ...typography.stamp, lineHeight: 18, color: colors.inkSoft } : { ...typography.body, color: mau ?? colors.ink };
  return (
    <View style={styles.dong} testID={testID}>
      <Text numberOfLines={1} style={[chu, styles.trai]}>
        {trai}
      </Text>
      {phai !== undefined ? <Text style={[typography.body, styles.so, { color: colors.ink }]}>{phai}</Text> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  trang: { paddingLeft: LE + 10, paddingRight: 14, paddingTop: TREN, paddingBottom: 14 },
  dong: { minHeight: DONG_KE, flexDirection: "row", alignItems: "center", gap: 8 },
  trai: { flex: 1 },
  so: { fontVariant: ["tabular-nums"] },
  ke: { position: "absolute", left: LE - 6, right: 0, height: StyleSheet.hairlineWidth },
  le: { position: "absolute", top: 0, bottom: 0, width: 1 },
  lo: { position: "absolute", left: 9, width: 10, height: 10, borderRadius: 5, borderWidth: 1 },
});
