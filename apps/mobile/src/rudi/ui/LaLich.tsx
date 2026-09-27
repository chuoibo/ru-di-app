/**
 * One leaf of a tear-off calendar, small enough to lay a fortnight of them in
 * a row (ADR-0037 D1, plan S2 «dãy lá lịch»): the weekday on the leaf's band,
 * the day big, the month under it. Choosing a day is taking its leaf: the
 * band turns coral and the leaf stands a paper height up.
 *
 * A radio among its row: its accessible name is the day in words
 * («Thứ Bảy 26/09»), the state is `checked`, and the whole leaf is the target
 * (56 × 72 dp). The words are text; nothing here is drawn.
 */
import { StyleSheet, Text, View } from "react-native";

import { bongGiay, displayFace, typography, useRudiTheme } from "../theme";
import { PressScale } from "./PressScale";

/** «T7 26/09» → band «T7», day «26», month «Th 09»; anything else stays whole. */
export function phanLa(ngan: string): { thu: string; ngay: string; thang: string } {
  const khop = /^(\S+) (\d{2})\/(\d{2})$/.exec(ngan.trim());
  if (!khop) return { thu: "", ngay: ngan, thang: "" };
  return { thu: khop[1], ngay: khop[2], thang: `Th ${khop[3]}` };
}

export function LaLich({
  ngan,
  nhan,
  chon,
  onPress,
  testID,
}: {
  /** The short day («T7 26/09»), split onto the leaf. */
  ngan: string;
  /** The day in words, the accessible name. */
  nhan: string;
  chon: boolean;
  onPress: () => void;
  testID?: string;
}) {
  const { colors, dark } = useRudiTheme();
  const la = phanLa(ngan);
  return (
    <PressScale
      accessibilityLabel={nhan}
      accessibilityRole="radio"
      accessibilityState={{ checked: chon }}
      aria-checked={chon}
      haptic="select"
      onPress={onPress}
      style={[styles.la, { backgroundColor: colors.card, borderColor: chon ? colors.accent : colors.lineStrong }, chon && bongGiay(2, dark)]}
      testID={testID}
    >
      <View style={[styles.dai, { backgroundColor: chon ? colors.accent : colors.paperShade }]}>
        <Text style={[typography.stamp, { color: chon ? colors.accentInk : colors.ink }]}>{la.thu}</Text>
      </View>
      <Text style={[styles.so, { color: colors.ink }]}>{la.ngay}</Text>
      <Text style={[typography.caption, { color: colors.inkSoft }]}>{la.thang}</Text>
    </PressScale>
  );
}

const styles = StyleSheet.create({
  la: { width: 56, minHeight: 72, borderWidth: 1, borderRadius: 6, overflow: "hidden", alignItems: "center", paddingBottom: 4 },
  dai: { alignSelf: "stretch", alignItems: "center", paddingVertical: 2 },
  so: { fontFamily: displayFace.extraBold, fontSize: 22, lineHeight: 26, fontVariant: ["tabular-nums"], marginTop: 2 },
});
