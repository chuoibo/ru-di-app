/**
 * The settlement drawn (ADR-0037 D1, plan S1): the people a transfer names,
 * as paper standees in their own inks, and one ink arrow per transfer from
 * the one who pays to the one who is paid, drawn on once when the page opens
 * (the fourth signature motion, «mực tự vẽ»).
 *
 * No number is on it: the amounts are the list under it, as text, and a
 * screen reader skips the drawing because that list says every arrow in
 * words. Under Reduce Motion the arrows are simply there.
 */
import { useEffect, useState } from "react";
import { StyleSheet, Text, View, type StyleProp, type ViewStyle } from "react-native";
import Animated, { Easing, useAnimatedProps, useSharedValue, withTiming, type SharedValue } from "react-native-reanimated";
import Svg, { Path } from "react-native-svg";

import { mucNguoi, typography, useRudiTheme } from "../theme";
import { HinhNhan } from "./Avatar";
import { soDoChuyen, type ChuyenSoDo, type MuiTen } from "./hinh-tien";
import { useMotion } from "./useMotion";

const PathDong = Animated.createAnimatedComponent(Path);

function Mui({ m, mau, tien }: { m: MuiTen; mau: string; tien: SharedValue<number> }) {
  const than = useAnimatedProps(() => ({ strokeDashoffset: m.dai * (1 - tien.value) }));
  // The head is put down when the pen arrives, not drawn ahead of it.
  const dau = useAnimatedProps(() => ({ strokeOpacity: tien.value > 0.94 ? 1 : 0 }));
  return (
    <>
      <PathDong animatedProps={than} d={m.d} fill="none" stroke={mau} strokeDasharray={[m.dai, m.dai]} strokeLinecap="round" strokeWidth={2.5} />
      <PathDong animatedProps={dau} d={m.dau} fill="none" stroke={mau} strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} />
    </>
  );
}

export function SoDoChuyen({
  nguoi,
  chuyen,
  style,
  testID,
}: {
  /** Names for the ids the transfers use. */
  nguoi: readonly { id: string; ten: string }[];
  chuyen: readonly ChuyenSoDo[];
  style?: StyleProp<ViewStyle>;
  testID?: string;
}) {
  const { dark } = useRudiTheme();
  const motion = useMotion();
  const [w, setW] = useState(0);
  const tien = useSharedValue(motion.reduced ? 1 : 0);
  const coRong = w > 0;
  useEffect(() => {
    if (!coRong) return;
    tien.value = withTiming(1, { duration: motion.sanKhau.batToiDa(4) || motion.ms("shared"), easing: Easing.bezier(0.3, 0, 0.2, 1), reduceMotion: motion.reanimated });
    // Drawn once, the first time the page has a width.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [coRong]);
  const sd = coRong ? soDoChuyen(chuyen, w) : null;
  return (
    <View
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
      onLayout={(e) => setW(Math.round(e.nativeEvent.layout.width))}
      pointerEvents="none"
      style={[{ height: sd?.h ?? 150 }, style]}
      testID={testID}
    >
      {sd ? (
        <>
          <Svg height={sd.h} style={StyleSheet.absoluteFill} width={sd.w}>
            {sd.muiTen.map((m) => (
              <Mui key={`${m.tu}-${m.toi}`} m={m} mau={mucNguoi(m.tu, dark)} tien={tien} />
            ))}
          </Svg>
          {sd.nguoi.map((p) => {
            const ten = nguoi.find((x) => x.id === p.id)?.ten ?? "Thành viên";
            const chu = (
              <Text numberOfLines={1} style={[typography.caption, styles.ten, { color: mucNguoi(p.id, dark) }]}>
                {ten}
              </Text>
            );
            // The top row wears its name above the standee (`hinh-tien.ts`).
            return (
              <View key={p.id} style={[styles.nhan, { left: p.x - 44, top: p.ten === "tren" ? p.y - 46 : p.y - 26 }]}>
                {p.ten === "tren" ? chu : null}
                <HinhNhan name={ten} personId={p.id} size={30} />
                {p.ten === "tren" ? null : chu}
              </View>
            );
          })}
        </>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  nhan: { position: "absolute", width: 88, alignItems: "center", gap: 2 },
  ten: { maxWidth: 88, textAlign: "center" },
});
