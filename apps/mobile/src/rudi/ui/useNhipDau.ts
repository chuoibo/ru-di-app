/**
 * The three beats of a stamp coming down (the `Stamp` contract), shared by
 * every seal that lands: the small `Stamp`, the page's big seal and the beat
 * the puppet's hand keeps (ADR-0037 D3, plan S0.4).
 *
 *   1. the strike: `roi` 0 -> 1 over `NHIP_DAU.lao` (130 ms), accelerating --
 *      a seal falls, it does not glide;
 *   2. the contact: `muc` 0 -> 1 over `NHIP_DAU.cham` (60 ms), the ink taking,
 *      and the success haptic fires HERE, not at the start;
 *   3. the sink: `lun` 1 -> 0.97 -> 1 on the press spring.
 *
 * `tre` holds the strike back so a seal can land on another clock's beat (the
 * puppet raises its stamp for 300 ms first). Reduce Motion lands everything
 * at once, still with the haptic, and nothing replays unless `dong` changes.
 */
import { useEffect } from "react";
import { Easing, runOnJS, useSharedValue, withDelay, withSequence, withSpring, withTiming, type SharedValue } from "react-native-reanimated";

import { NHIP_DAU } from "../san-khau/dong-hoc";
import { useMotion } from "./useMotion";

export interface NhipDau {
  roi: SharedValue<number>;
  muc: SharedValue<number>;
  lun: SharedValue<number>;
}

export function useNhipDau(dong: boolean, tuyChon: { tre?: number; rung?: boolean } = {}): NhipDau {
  const { tre = 0, rung = true } = tuyChon;
  const motion = useMotion();
  const roi = useSharedValue(dong ? 0 : 1);
  const muc = useSharedValue(dong ? 0 : 1);
  const lun = useSharedValue(1);
  useEffect(() => {
    if (!dong) return;
    const lao = motion.reduced ? 0 : NHIP_DAU.lao;
    const cham = motion.reduced ? 0 : NHIP_DAU.cham;
    const truoc = motion.reduced ? 0 : Math.max(0, tre);
    const chamXong = () => {
      if (rung) motion.haptic.success();
    };
    roi.value = 0;
    muc.value = 0;
    lun.value = 1;
    roi.value = withDelay(
      truoc,
      withTiming(1, { duration: lao, easing: Easing.in(Easing.quad) }, (finished) => {
        if (finished) runOnJS(chamXong)();
      }),
    );
    muc.value = withDelay(truoc + lao, withTiming(1, { duration: cham }));
    lun.value = withDelay(truoc + lao, withSequence(withTiming(0.97, { duration: cham }), withSpring(1, motion.spring.press)));
  }, [dong, motion, roi, muc, lun, tre, rung]);
  return { roi, muc, lun };
}
