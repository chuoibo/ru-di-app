/**
 * A paper stage (ADR-0037 D1): cut-paper layers that stand up out of the page,
 * far layers first, and lean with a parallax input.
 *
 * The component owns the motion (`mo`, 0 flat .. 1 standing) and hands the
 * same shared value to whichever renderer paints -- Skia when it can, SVG
 * otherwise (`KhungSkia`) -- so the pop-up is one motion whatever draws it.
 * Without a `mo` from the caller, the stage stands itself up once on mount,
 * within `batToiDa` for its layer count; under Reduce Motion it is simply up.
 *
 * `gap` (0..1) folds a standing stage back into the page -- the screen's
 * scroll, when the stage heads a screen. It runs the pop-up backwards: the
 * near layers lie down first, the far ones last, as a book page closing. Under
 * Reduce Motion there is no 3D at all (ADR-0037 D3): the stage stays upright
 * and only fades as it would have folded.
 *
 * Decorative by default. With `coMoTa` (the stage's own sentence), it is one
 * image for a screen reader: the words of the screen stay React Native text
 * next to it, never inside it (ADR-0037 D9).
 */
import { useEffect } from "react";
import { View, type StyleProp, type ViewStyle } from "react-native";
import Animated, { Easing, useAnimatedStyle, useDerivedValue, useSharedValue, withTiming, type SharedValue } from "react-native-reanimated";

import type { SanKhau as SanKhauData } from "../art/san-khau";
import { KhungSkia, luoiSkia } from "./KhungSkia";
import { SanKhauSvg, type SanKhauVeProps } from "./art/SanKhauSvg";
import { useMotion } from "./useMotion";

const SanKhauSkia = luoiSkia<SanKhauVeProps>(() => import("./skia/SanKhauSkia"));

export interface SanKhauProps {
  san: SanKhauData;
  width: number;
  /** The caller's own 0..1; when absent the stage opens itself once on mount. */
  mo?: SharedValue<number>;
  /** 0..1 folds the standing stage back into the page (a scroll, a closing sheet). */
  gap?: SharedValue<number>;
  /** Parallax input in -1..1 (a pan or a tilt, mapped by the caller). */
  thiSai?: SharedValue<number>;
  /** Read the stage's sentence to a screen reader; off, the stage is decoration. */
  coMoTa?: boolean;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

export function SanKhau({ san, width, mo, gap, thiSai, coMoTa = false, style, testID }: SanKhauProps) {
  const motion = useMotion();
  const giam = motion.reduced;
  const moTuMinh = useSharedValue(giam ? 1 : 0);
  const moDung = mo ?? moTuMinh;
  const height = Math.round((width * san.khung.h) / san.khung.w);
  useEffect(() => {
    if (mo) return;
    const ms = motion.sanKhau.batToiDa(san.tang.length);
    if (ms === 0) {
      moTuMinh.value = 1;
      return;
    }
    moTuMinh.value = 0;
    // Linear here: each layer eases on its own inside `gocBatTang`.
    moTuMinh.value = withTiming(1, { duration: ms, easing: Easing.linear, reduceMotion: motion.reanimated });
    // One pop-up per mount: a re-render never replays it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  // The fold a scroll asks for runs the same curve backwards; with Reduce
  // Motion the layers never leave upright and the fold becomes a fade.
  const moHienThi = useDerivedValue(() => {
    const g = gap ? Math.min(1, Math.max(0, gap.value)) : 0;
    return giam ? moDung.value : Math.min(moDung.value, 1 - g);
  });
  const kieuMo = useAnimatedStyle(() => {
    const g = gap ? Math.min(1, Math.max(0, gap.value)) : 0;
    return { opacity: giam ? 1 - g : 1 };
  });
  const props: SanKhauVeProps = { san, width, height, mo: moHienThi, thiSai: giam ? undefined : thiSai };
  return (
    <Animated.View
      accessibilityLabel={coMoTa ? san.moTa : undefined}
      accessibilityRole={coMoTa ? "image" : undefined}
      accessible={coMoTa}
      importantForAccessibility={coMoTa ? "yes" : "no-hide-descendants"}
      style={[{ width, height }, kieuMo, style]}
      testID={testID}
    >
      <View style={{ width, height }}>
        <KhungSkia props={props} skia={SanKhauSkia} svg={<SanKhauSvg {...props} />} />
      </View>
    </Animated.View>
  );
}
