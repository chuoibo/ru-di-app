import type { ComponentProps } from "react";
import { StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../theme";
import { Money } from "./Money";

/**
 * One line of a ledger: a name on the left, its amount on the right, a
 * hairline underneath.
 *
 * The settlement and finance pages used to open with a hero metric: a big
 * number in a teal block with a small label over it and a progress bar under
 * it. That is the dashboard the 2026-09-06 report diagnosed. A ledger has no
 * hero; every sum is a row, the tone (teal for money, warn for what is owed)
 * lands on the number only, and `dam` bolds the one row a reader looks for
 * first without changing its size class.
 */
export function DongTien({
  nhan,
  phu,
  vnd,
  tone = "ink",
  dam = false,
  cuoi = false,
  testID,
}: {
  nhan: string;
  phu?: string;
  vnd: number;
  tone?: ComponentProps<typeof Money>["tone"];
  dam?: boolean;
  /** Last row of its ledger: no hairline under it. */
  cuoi?: boolean;
  testID?: string;
}) {
  const { colors } = useRudiTheme();
  return (
    <View style={[styles.dong, { borderBottomColor: colors.line }, cuoi && styles.cuoi]} testID={testID}>
      <View style={styles.flex}>
        <Text style={[dam ? typography.label : typography.body, { color: colors.ink }]}>{nhan}</Text>
        {phu ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{phu}</Text> : null}
      </View>
      <Money size={dam ? "money" : "label"} tone={tone} vnd={vnd} />
    </View>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  dong: { flexDirection: "row", alignItems: "center", gap: 12, minHeight: 52, paddingVertical: 10, borderBottomWidth: StyleSheet.hairlineWidth },
  cuoi: { borderBottomWidth: 0 },
});
