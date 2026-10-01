/**
 * The cut end of a horizontal row, fading into the paper it lies on.
 *
 * A row that scrolls sideways inside a sheet or a day page is clipped by the
 * scroll box around it, a padding's width inside the paper's edge. A cell cut
 * there showed its outline sliced mid-word, which read as broken rather than
 * as "more this way" (B3 finish review: the day editor's stop chips, the day
 * page's stop rail). Lay this over the row's right end, in the paper's colour.
 */
import { useId } from "react";
import { StyleSheet, View } from "react-native";
import Svg, { Defs, LinearGradient, Rect, Stop } from "react-native-svg";

export function MoNgang({ mau, rong = 28 }: { mau: string; rong?: number }) {
  const id = `mo-ngang-${useId().replace(/[^a-zA-Z0-9_-]/g, "")}`;
  return (
    <View pointerEvents="none" style={[styles.mo, { width: rong }]}>
      <Svg height="100%" width="100%">
        <Defs>
          <LinearGradient id={id} x1="0" x2="1" y1="0" y2="0">
            <Stop offset="0" stopColor={mau} stopOpacity={0} />
            <Stop offset="1" stopColor={mau} stopOpacity={1} />
          </LinearGradient>
        </Defs>
        <Rect fill={`url(#${id})`} height="100%" width="100%" />
      </Svg>
    </View>
  );
}

const styles = StyleSheet.create({
  mo: { position: "absolute", top: 0, bottom: 0, right: 0 },
});
