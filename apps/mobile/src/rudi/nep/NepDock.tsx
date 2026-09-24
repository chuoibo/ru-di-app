import { useEffect } from "react";
import { Pressable, StyleSheet, Text, View, useWindowDimensions } from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import Animated, { runOnJS, useAnimatedStyle, useSharedValue, withSpring, withTiming } from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { TAB_BAR_HEIGHT } from "../adaptive";
import { typography, useRudiTheme } from "../theme";
import { Nep } from "../ui/art/Nep";
import { useMotion } from "../ui/useMotion";
import {
  NEP_DIA,
  NEP_MEP_CAO,
  beRongMep,
  ghimVaoRay,
  rayDoc,
  tyLeTuY,
  yTuTyLe,
} from "./dock-vi-tri";
import { useNep } from "./NepProvider";
import { nepHien } from "./trang-thai";

/** How long one peeked line stays before it retracts on its own. */
const HE_MS = 4000;
/** Past this, a horizontal drag means «tuck Nếp away» rather than «move it». */
const KEO_AN_DP = 56;
const KEO_AN_TOC = 700;

/**
 * Nếp on the edge of the screen.
 *
 * The bottom of the shell is already spoken for: `ui/RudiTabBar.tsx` puts the
 * create stamp in the middle of the bar as a real column, so the usual
 * bottom-right chat bubble would sit on the one control the bar cannot lose.
 * Nếp therefore rides a vertical rail on the right edge, and `dock-vi-tri.ts`
 * is the only place that knows where the rail ends.
 *
 * The hiding gesture is the product idea: Nếp carries a sheet of paper, so
 * swiping it away tucks the sheet into the edge of the notebook and leaves a
 * paper edge showing. That edge is also the whole notification vocabulary:
 * when something is waiting it grows a second layer and warms one step, once,
 * with no repeat and no badge. DESIGN.md forbids Nếp acting as chrome, and a
 * red dot is chrome. Someone watching will notice; someone who is not will not
 * be interrupted.
 *
 * On money, error and conflict screens the provider forces `an` and refuses to
 * peek (ADR-0033). What remains is a 6dp paper edge with no face and no
 * character, which is a door back to Nếp rather than Nếp standing beside a
 * settlement.
 */
export function NepDock() {
  const { dock, tyLe, daDocDia, gui, datTyLe } = useNep();
  const { colors } = useRudiTheme();
  const motion = useMotion();
  const insets = useSafeAreaInsets();
  const { height, width } = useWindowDimensions();

  const ray = rayDoc({
    cao: height,
    dinh: insets.top,
    // Reserved even on routes without tabs: sitting a little high costs
    // nothing, sitting on the tab bar costs the create button.
    day: TAB_BAR_HEIGHT + insets.bottom,
  });

  const y = useSharedValue(yTuTyLe(tyLe, ray));
  const keoX = useSharedValue(0);

  // Follow the stored position once the disk has answered, and follow the rail
  // when the window changes (rotation, split screen).
  useEffect(() => {
    y.value = withSpring(yTuTyLe(tyLe, ray), motion.spring.settle);
  }, [tyLe, ray.tren, ray.duoi, y, motion, ray]);

  // One peeked line retracts by itself. A bubble that waited for a tap would
  // become a second permanent element on every screen.
  useEffect(() => {
    if (dock.trangThai !== "he") return;
    const t = setTimeout(() => gui({ kieu: "het-gio-he" }), HE_MS);
    return () => clearTimeout(t);
  }, [dock.trangThai, gui]);

  const keo = Gesture.Pan()
    .onUpdate((e) => {
      y.value = ghimVaoRay(yTuTyLe(tyLe, ray) + e.translationY, ray);
      keoX.value = Math.max(0, e.translationX);
    })
    .onEnd((e) => {
      runOnJS(datTyLe)(tyLeTuY(y.value, ray));
      if (e.translationX > KEO_AN_DP || e.velocityX > KEO_AN_TOC) runOnJS(gui)({ kieu: "vuot-ra" });
      keoX.value = withSpring(0, motion.spring.settle);
    });

  const dangAn = dock.trangThai === "an";
  const rong = dangAn ? beRongMep(dock.coViec) : NEP_DIA;
  const cao = dangAn ? NEP_MEP_CAO : NEP_DIA;

  const kieuDock = useAnimatedStyle(() => ({
    transform: [{ translateY: y.value }, { translateX: keoX.value }],
  }));

  // The panel covers the dock anyway, and an icon sliding under a sheet reads
  // as a bug rather than as depth. The same for any other open sheet, and for
  // the screens Nếp is absent from (`trang-thai.ts` rules 3 and 4).
  if (!nepHien(dock)) return null;

  const nhan = dangAn
    ? dock.coViec
      ? "Nếp đang giấu và có tin mới, chạm để mở"
      : "Nếp đang giấu, chạm để mở"
    : "Mở Nếp";

  return (
    <Animated.View pointerEvents="box-none" style={[styles.lop, { width }, kieuDock]}>
      {dock.trangThai === "he" ? (
        <Pressable
          accessibilityRole="button"
          onPress={() => gui({ kieu: "cham" })}
          style={[styles.bongBong, { backgroundColor: colors.aiSoft, borderColor: colors.ai }]}
          testID="nep-bong-bong"
        >
          <Text numberOfLines={1} style={[typography.body, { color: colors.ink }]}>
            Mình có việc này, xem không?
          </Text>
        </Pressable>
      ) : null}

      <GestureDetector gesture={keo}>
        <Pressable
          accessibilityLabel={nhan}
          accessibilityRole="button"
          hitSlop={{ top: 12, bottom: 12, left: 24, right: 12 }}
          onPress={() => gui({ kieu: "cham" })}
          style={[
            dangAn ? styles.mep : styles.dia,
            {
              width: rong,
              height: cao,
              backgroundColor: dangAn && !dock.coViec ? colors.aiSoft : colors.ai,
            },
          ]}
          testID={dangAn ? "nep-mep" : "nep-dia"}
        >
          {dangAn ? null : <Nep pose="doi" size={44} testID="nep-hinh" />}
        </Pressable>
      </GestureDetector>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  lop: {
    position: "absolute",
    top: 0,
    left: 0,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "flex-end",
    gap: 8,
    paddingRight: 0,
  },
  dia: {
    borderRadius: NEP_DIA / 2,
    alignItems: "center",
    justifyContent: "center",
    marginRight: 12,
  },
  mep: {
    borderTopLeftRadius: 6,
    borderBottomLeftRadius: 6,
  },
  bongBong: {
    maxWidth: 240,
    borderRadius: 14,
    borderWidth: StyleSheet.hairlineWidth,
    paddingHorizontal: 12,
    paddingVertical: 8,
  },
});
