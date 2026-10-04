/**
 * A list of people that keeps each action beside its name.
 *
 * One column on a phone, a hairline between neighbours from the text column.
 * Where two columns of at least 280dp fit (a tablet's reading column), the rows
 * sit in two, each with its own hairline: a single 590dp row put «Nhắn tin»
 * and «Đặt làm quản trị» 330 to 680px from the name they act on (QA UI-081).
 * The width is the list's own, measured, never the window's.
 */
import { useState, type ReactNode } from "react";
import { StyleSheet, View } from "react-native";

import { gridFor } from "../adaptive";
import { useRudiTheme } from "../theme";

export function LuoiNguoi({ hang, vachTrong = true, testID }: {
  /** A row, or a row drawn for the layout it lands in (`luoi`: two columns). */
  hang: { key: string; node: ReactNode | ((luoi: boolean) => ReactNode) }[];
  /** Draw the hairline between rows; a row that draws its own border passes false. */
  vachTrong?: boolean;
  testID?: string;
}) {
  const { colors } = useRudiTheme();
  const [rong, setRong] = useState(0);
  const { columns, itemWidth } = gridFor(rong, 280, 16, 2);
  const luoi = columns > 1;
  return (
    <View onLayout={(e) => setRong(Math.round(e.nativeEvent.layout.width))} style={luoi ? styles.luoi : styles.cot} testID={testID}>
      {hang.map((h, i) => (
        <View
          key={h.key}
          style={[
            luoi && { width: itemWidth },
            vachTrong && luoi && [styles.vachLuoi, { borderBottomColor: colors.line }],
          ]}
        >
          {vachTrong && !luoi && i > 0 ? <View style={[styles.vachCot, { backgroundColor: colors.line }]} /> : null}
          {typeof h.node === "function" ? h.node(luoi) : h.node}
        </View>
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  cot: { paddingVertical: 2 },
  luoi: { flexDirection: "row", flexWrap: "wrap", columnGap: 16, paddingVertical: 2 },
  vachLuoi: { borderBottomWidth: StyleSheet.hairlineWidth },
  // The hairline starts at the text column (avatar 40 + gap 12), as before.
  vachCot: { height: StyleSheet.hairlineWidth, marginLeft: 52 },
});
