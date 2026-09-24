/**
 * Parallax for a paper stage (ADR-0037 D3): a -1..1 input the stage turns into
 * a sideways slide per layer (`lechThiSai`: far layers barely move, near ones
 * most), so the eye reads depth from the difference.
 *
 * `useThiSaiKeo` is the finger: a horizontal drag on the stage leans it, and
 * letting go settles it back with the shell's `settle` spring. It is motion
 * tied to a hand, never a loop: it stops when the hand stops (ADR-0037 D3).
 * The pan only claims a clearly sideways drag, so the list under the stage
 * keeps every vertical scroll. Under Reduce Motion the gesture is off and the
 * input stays 0.
 */
import { useMemo } from "react";
import { Gesture, type PanGesture } from "react-native-gesture-handler";
import { useSharedValue, withSpring, type SharedValue } from "react-native-reanimated";

import { useMotion } from "./useMotion";

/** A drag this far (dp) leans the stage all the way. */
const KEO_HET = 96;

export function useThiSaiKeo(bat = true): { thiSai: SharedValue<number>; cuChi: PanGesture } {
  const motion = useMotion();
  const thiSai = useSharedValue(0);
  const veCho = motion.spring.settle;
  const cuChi = useMemo(
    () =>
      Gesture.Pan()
        .enabled(bat && !motion.reduced)
        .activeOffsetX([-12, 12])
        .failOffsetY([-10, 10])
        .onUpdate((e) => {
          thiSai.value = Math.max(-1, Math.min(1, e.translationX / KEO_HET));
        })
        .onFinalize(() => {
          thiSai.value = withSpring(0, veCho);
        }),
    [bat, motion.reduced, thiSai, veCho],
  );
  return { thiSai, cuChi };
}
