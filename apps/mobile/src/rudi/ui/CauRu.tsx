/**
 * The invitation written as one sentence with blanks (ADR-0037 D1, plan S0.5):
 * «Rủ Hội Đạp đi [tên kèo] từ [ngày] tới [ngày], [số] người» instead of a
 * stack of labelled boxes. The words are the form's labels; the blanks are the
 * fields, each a separate slot at least a finger wide, and the sentence wraps
 * word by word like any sentence at any text size.
 *
 * A screen reader hears the sentence once (`moTa`, with the blanks as «…»),
 * then each blank by its own label -- not a word at a time.
 */
import type { ReactNode } from "react";
import { StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../theme";
import { tachCau } from "./cau-ru";

export function CauRu({
  mau,
  o,
  moTa,
  co = "lon",
  testID,
}: {
  /** The sentence, with `{ten}` where a blank goes. */
  mau: string;
  /** Each blank's field, by the name used in `mau`. */
  o: Record<string, ReactNode>;
  /** The sentence as a screen reader should hear it before the blanks. */
  moTa: string;
  co?: "vua" | "lon";
  testID?: string;
}) {
  const { colors } = useRudiTheme();
  const chu = co === "lon" ? typography.h2 : typography.body;
  const phan = tachCau(mau, Object.keys(o));
  return (
    <View style={styles.cau} testID={testID}>
      <Text accessibilityRole="text" style={styles.anChoTroNang}>
        {moTa}
      </Text>
      {phan.map((p, i) =>
        p.kieu === "chu" ? (
          <Text importantForAccessibility="no" accessibilityElementsHidden key={i} style={[chu, { color: colors.ink }]}>
            {p.chu}
          </Text>
        ) : (
          <View key={i} style={styles.o}>
            {o[p.ten]}
            {p.sau ? (
              <Text importantForAccessibility="no" accessibilityElementsHidden style={[chu, { color: colors.ink }]}>
                {p.sau}
              </Text>
            ) : null}
          </View>
        ),
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  cau: { flexDirection: "row", flexWrap: "wrap", alignItems: "flex-end", columnGap: 8, rowGap: 12 },
  o: { flexDirection: "row", alignItems: "flex-end", minWidth: 48 },
  // Read by a screen reader, not drawn: the sentence's shape before its blanks.
  anChoTroNang: { position: "absolute", width: 1, height: 1, opacity: 0, overflow: "hidden" },
});
