import { StyleSheet, View, useWindowDimensions, type StyleProp, type ViewStyle } from "react-native";
import Svg, { Path } from "react-native-svg";

import { chuLon } from "../../adaptive";
import { KHUNG_KY_HOA, TI_LE_KY_HOA, hinhKyHoa, moTaKyHoa } from "../../art/ky-hoa";
import { useRudiTheme } from "../../theme";
import { mauLop } from "./VeLop";

export interface KyHoaProps {
  /** Catalogue category id (`quan-an-local`, `cafe`, …); unknown → a folded tag on the floor. */
  loai?: string;
  /** Tags the place really carries; at most two become props. Live rows pass none. */
  tags?: readonly string[];
  /** The 4:1 reading; default follows large text (`chuLon`). */
  gon?: boolean;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

/**
 * «Ký hoạ trong sổ»: the ink sketch a place without a photo gets — a sheet of
 * `paper` the width of its column, 3:1 by default and 4:1 at large text, the
 * one drawing cropped to the frame (`slice`) rather than letterboxed. It is
 * an image with one sentence for a screen reader and carries no text, so the
 * Explore XML gate (one promise, one reason, one description) does not see it.
 */
export function KyHoa({ loai, tags = [], gon, style, testID }: KyHoaProps) {
  const { colors, radius } = useRudiTheme();
  const { fontScale } = useWindowDimensions();
  const gonThat = gon ?? chuLon(fontScale);
  const lop = hinhKyHoa(loai, tags, { gon: gonThat });
  return (
    <View
      accessibilityLabel={moTaKyHoa(loai, tags, { gon: gonThat })}
      accessibilityRole="image"
      style={[
        styles.to,
        { aspectRatio: gonThat ? TI_LE_KY_HOA.gon : TI_LE_KY_HOA.day, backgroundColor: colors.paper, borderColor: colors.line, borderRadius: radius.small },
        style,
      ]}
      testID={testID ?? "ky-hoa"}
    >
      <Svg height="100%" pointerEvents="none" preserveAspectRatio="xMidYMid slice" viewBox={`0 0 ${KHUNG_KY_HOA.w} ${KHUNG_KY_HOA.h}`} width="100%">
        {lop.map((l, i) => {
          const mau = mauLop(colors, l.mau);
          return l.net === undefined ? (
            <Path d={l.d} fill={mau} key={i} />
          ) : (
            <Path d={l.d} fill="none" key={i} stroke={mau} strokeLinecap="round" strokeLinejoin="round" strokeWidth={l.net} />
          );
        })}
      </Svg>
    </View>
  );
}

const styles = StyleSheet.create({
  to: { alignSelf: "stretch", overflow: "hidden", borderWidth: StyleSheet.hairlineWidth },
});
