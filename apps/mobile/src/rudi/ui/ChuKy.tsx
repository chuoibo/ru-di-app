/**
 * One signature line of a pact (plan S2, ADR-0037 D1): the two-person
 * notebook opens when both have signed. A signed line carries the name in
 * the display face with a pen flourish under it, which draws itself once when
 * the signature is new (`dong`) -- the fourth signature motion, ink drawing
 * itself. An unsigned line is the dashed rule with who it waits for.
 *
 * The name and the waiting sentence are React Native text; the flourish is
 * decoration a screen reader skips. The line says, in words, whether it is
 * signed: never a drawing alone.
 */
import { useEffect, useState } from "react";
import { StyleSheet, Text, View, type StyleProp, type ViewStyle } from "react-native";
import Animated, { Easing, useAnimatedProps, useSharedValue, withDelay, withTiming } from "react-native-reanimated";
import Svg, { Path } from "react-native-svg";

import { duongDut, netChuKy } from "../art/giay";
import { displayFace, typography, useRudiTheme } from "../theme";
import { useMotion } from "./useMotion";

const PathDong = Animated.createAnimatedComponent(Path);
/** Long enough for any flourish up to 400 dp wide; the dash only has to cover it. */
const DAI_NET = 900;

export function ChuKy({
  ten,
  vaiTro,
  choChu,
  dong = false,
  tre = 0,
  style,
  testID,
}: {
  /** Who signed; null while the line waits. */
  ten: string | null;
  /** Under the line: «Người đề nghị», «Người đồng ý». */
  vaiTro: string;
  /** What an unsigned line says: «Chờ Minh ký». */
  choChu: string;
  /** The signature is new: the flourish draws itself (once per mount). */
  dong?: boolean;
  tre?: number;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}) {
  const { colors } = useRudiTheme();
  const motion = useMotion();
  const [w, setW] = useState(0);
  const ve = useSharedValue(dong && !motion.reduced ? 0 : 1);
  useEffect(() => {
    if (!dong || motion.reduced) return;
    // The same line may have been drawn blank a moment ago: start the pen at 0.
    ve.value = 0;
    ve.value = withDelay(tre, withTiming(1, { duration: motion.sanKhau.batToiDa(4) || motion.ms("shared"), easing: Easing.bezier(0.4, 0, 0.2, 1), reduceMotion: motion.reanimated }));
    // Drawn once, when the signature arrives.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dong]);
  const net = useAnimatedProps(() => ({ strokeDashoffset: DAI_NET * (1 - ve.value) }));
  const daKy = ten !== null;
  return (
    <View accessibilityLabel={daKy ? `${vaiTro}: ${ten} đã ký` : `${vaiTro}: ${choChu}`} accessible onLayout={(e) => setW(Math.round(e.nativeEvent.layout.width))} style={[styles.khoi, style]} testID={testID}>
      <View style={styles.dong}>
        {daKy ? (
          <Text numberOfLines={1} style={[styles.ten, { color: colors.ink }]}>
            {ten}
          </Text>
        ) : (
          <Text numberOfLines={1} style={[typography.note, { color: colors.inkSoft }]}>
            {choChu}
          </Text>
        )}
      </View>
      {w > 0 ? (
        <Svg height={16} pointerEvents="none" width={w}>
          {daKy ? (
            <PathDong animatedProps={net} d={netChuKy(w, 16)} fill="none" stroke={colors.ink} strokeDasharray={[DAI_NET, DAI_NET]} strokeLinecap="round" strokeWidth={1.8} />
          ) : (
            <Path d={duongDut([0, 12], [w, 12], 5, 4)} fill="none" stroke={colors.lineStrong} strokeWidth={1.2} />
          )}
        </Svg>
      ) : (
        <View style={styles.giuCho} />
      )}
      <Text style={[typography.caption, { color: colors.inkSoft }]}>{vaiTro}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  khoi: { flex: 1, minWidth: 120, gap: 0 },
  dong: { minHeight: 30, justifyContent: "flex-end" },
  // A signature leans the way handwriting does; the words stay plain text.
  ten: { fontFamily: displayFace.extraBold, fontSize: 20, lineHeight: 26, transform: [{ skewX: "-8deg" }] },
  giuCho: { height: 16 },
});
