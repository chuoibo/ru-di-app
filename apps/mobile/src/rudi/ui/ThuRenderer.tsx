/**
 * `ui-lab`'s renderer probe: the same scene drawn by Skia when the build can,
 * by react-native-svg otherwise, driven by ONE pair of shared values so the
 * motion is identical whichever renderer paints (see `KhungSkia`). A dev board
 * block, never on a product screen.
 */
import { useEffect } from "react";
import { View } from "react-native";
import Animated, { useAnimatedStyle, useSharedValue, withDelay, withTiming } from "react-native-reanimated";
import Svg, { Path } from "react-native-svg";

import { KHUNG_NEP } from "../art/nep";
import { useRudiTheme } from "../theme";
import { Nep } from "./art/Nep";
import { duongCongS } from "./duong-svg";
import { KhungSkia, luoiSkia } from "./KhungSkia";
import { useMotion } from "./useMotion";
import type { ThuRendererProps } from "./skia/ThuRendererSkia";

const ThuRendererSkia = luoiSkia<ThuRendererProps>(() => import("./skia/ThuRendererSkia"));

function BanSvg({ mo, width, height }: ThuRendererProps) {
  const { colors } = useRudiTheme();
  const dung = useAnimatedStyle(() => ({
    transform: [{ perspective: 480 }, { rotateX: `${(1 - mo.value) * 90}deg` }],
  }));
  const route = duongCongS(Math.max(40, width - height - 16), height - 16, "down").d;
  return (
    <View style={{ width, height, flexDirection: "row" }}>
      <Animated.View style={[{ width: height, height, transformOrigin: "bottom" }, dung]}>
        <Nep pose="moi" size={height} />
      </Animated.View>
      <Svg height={height} style={{ marginLeft: 8 }} width={Math.max(40, width - height - 8)}>
        <Path d={route} fill="none" stroke={colors.accent} strokeLinecap="round" strokeWidth={3} transform="translate(0 8)" />
      </Svg>
    </View>
  );
}

export function ThuRenderer({ lan, width = 320, height = KHUNG_NEP }: { lan: number; width?: number; height?: number }) {
  const motion = useMotion();
  const mo = useSharedValue(0);
  const veToi = useSharedValue(0);
  useEffect(() => {
    mo.value = 0;
    veToi.value = 0;
    mo.value = withTiming(1, motion.timing("shared", "decelerate"));
    veToi.value = withDelay(motion.ms("standard"), withTiming(1, motion.timing("celebrate", "standard")));
  }, [lan, mo, veToi, motion]);
  const props: ThuRendererProps = { mo, veToi, width, height };
  return <KhungSkia props={props} skia={ThuRendererSkia} svg={<BanSvg {...props} />} testID="thu-renderer" />;
}
