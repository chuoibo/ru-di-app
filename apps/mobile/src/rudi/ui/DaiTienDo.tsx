/**
 * How far a collection has come, as a strip of teal tape laid along a dashed
 * track (ADR-0037 D1): the tape's length is the share of transfers the server
 * counted as arrived, torn at its end like tape pulled off the roll.
 *
 * Never alone and never read: the screen writes the count beside it
 * («{n}/{m} lượt chuyển đã về»), so the strip is decoration a screen reader
 * skips. No percentage is printed anywhere.
 */
import { useState } from "react";
import { View, type StyleProp, type ViewStyle } from "react-native";
import Svg, { Path, Rect } from "react-native-svg";

import { useRudiTheme } from "../theme";
import { duongWashiXeMep } from "./duong-svg";
import { dayBang } from "./hinh-tien";

const CAO = 16;

export function DaiTienDo({ da, tong, style, testID }: { da: number; tong: number; style?: StyleProp<ViewStyle>; testID?: string }) {
  const { brand, colors } = useRudiTheme();
  const [w, setW] = useState(0);
  const day = dayBang(da, tong, w);
  return (
    <View
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
      onLayout={(e) => setW(Math.round(e.nativeEvent.layout.width))}
      style={[{ height: CAO + 4 }, style]}
      testID={testID}
    >
      {w > 0 ? (
        <Svg height={CAO + 4} width={w}>
          <Rect fill="none" height={CAO - 4} rx={4} stroke={colors.lineStrong} strokeDasharray={[5, 4]} strokeWidth={1.2} width={w - 2} x={1} y={4} />
          {day > 0 ? <Path d={duongWashiXeMep(day, CAO + 2)} fill={brand.teal} fillOpacity={0.9} /> : null}
        </Svg>
      ) : null}
    </View>
  );
}
