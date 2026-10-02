import { useEffect, useMemo, useState } from "react";
import { StyleSheet, View } from "react-native";
import Animated, { useAnimatedStyle, useSharedValue, withTiming } from "react-native-reanimated";

import { NET_KY_HOA } from "../art/ky-hoa";
import { SAN_THANH_PHO, sanKhauThanhPho } from "../art/thanh-pho";
import { useRudiTheme } from "../theme";
import { SanKhau } from "./SanKhau";
import { useAdaptiveLayout } from "./useAdaptiveLayout";
import { useMotion } from "./useMotion";

/** The widest the city is drawn: past it the strokes thicken and the scene eats the first screen. */
const RONG_VE = 480;
// The column's gap (18) plus the scene's empty top: its sky starts under the
// place line. Folded, the stage gives back exactly one column gap instead.
const KEO_LEN = 32;
const KEO_LEN_GAP = 18;

/**
 * The city as a pop-up stage at the head of Khám phá › Địa điểm (ADR-0037
 * D1), composed as the owner's mockup draws it (finish review 02/10). It
 * rises under the place line above it, so its sky starts there instead of
 * under a band of empty paper; the caller puts that line before this and
 * gives it `zIndex: 1`, so the line stays on top of the sky.
 *
 * On a phone the scene runs edge to edge. On a tablet it is a full-width
 * band: the drawing stays at its own scale in the middle (scaled to the
 * column it doubled its strokes and pushed every place off the first screen)
 * and its ground line runs on to both edges of the column, in the same ink
 * and weight, so the scene sits on one horizon instead of floating in paper.
 *
 * The frame keeps the drawing's shape from the first render, so the list is
 * laid out under it once and never pushed down when the width is measured
 * (QA UI-025); and the stage stays mounted, folding away while `gap` (a search
 * or filter under way) and standing back as it was, so clearing a filter
 * never replays the pop-up (QA UI-026).
 */
export function SanThanhPho({ id, ten, gap = false }: { id: string | null; ten: string; gap?: boolean }) {
  const dienThoai = useAdaptiveLayout().sizeClass === "compact";
  const { colors } = useRudiTheme();
  const motion = useMotion();
  const san = useMemo(() => sanKhauThanhPho(id, ten), [id, ten]);
  const [rong, setRong] = useState(0);
  const rongVe = Math.min(rong, RONG_VE);
  const tiLe = rongVe / san.khung.w;
  const net = NET_KY_HOA.gan * tiLe;
  // The scene's ground line (`mat()` in art/thanh-pho.ts: x 4 .. w-4 at
  // SAN_THANH_PHO), carried past the drawing's own ends to the column's edges.
  const keo = (rong - rongVe) / 2 + 4 * tiLe;
  const chanTroi = { top: SAN_THANH_PHO * tiLe - net / 2, height: net, width: keo, backgroundColor: colors.ink };

  const gapLai = useSharedValue(gap ? 1 : 0);
  useEffect(() => {
    gapLai.value = withTiming(gap ? 1 : 0, { duration: motion.ms("standard"), reduceMotion: motion.reanimated });
    // Only a change of state folds or unfolds; `motion` is read, not watched.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gap, motion.reduced]);
  const cao = rong > 0 ? Math.round((rongVe * san.khung.h) / san.khung.w) : 0;
  const kieu = useAnimatedStyle(() => {
    const mo = 1 - gapLai.value;
    const lui = -KEO_LEN + (KEO_LEN - KEO_LEN_GAP) * gapLai.value;
    return cao === 0 ? { opacity: mo, marginTop: lui } : { height: cao * mo, opacity: mo, marginTop: lui };
  });

  return (
    <Animated.View pointerEvents={gap ? "none" : "auto"} style={[styles.vung, dienThoai && styles.tran, kieu]} testID="san-thanh-pho">
      <View onLayout={(e) => setRong(Math.round(e.nativeEvent.layout.width))} style={styles.bang}>
        {rong > 0 && !dienThoai ? (
          <>
            <View pointerEvents="none" style={[styles.chanTroi, chanTroi, { left: 0 }]} />
            <View pointerEvents="none" style={[styles.chanTroi, chanTroi, { right: 0 }]} />
          </>
        ) : null}
        <View style={[styles.khung, { aspectRatio: san.khung.w / san.khung.h }]}>
          {rong > 0 ? <SanKhau coMoTa={!gap} key={id ?? ten} san={san} width={rongVe} /> : null}
        </View>
      </View>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  vung: { alignSelf: "stretch", overflow: "hidden" },
  // The screen's own gutter (`space.md`) given back so the stage meets both edges.
  tran: { marginHorizontal: -16 },
  bang: { alignItems: "center", alignSelf: "stretch" },
  khung: { alignItems: "center", width: "100%", maxWidth: RONG_VE },
  chanTroi: { position: "absolute", borderRadius: 2 },
});
