/**
 * Turning a page between the steps of one task (ADR-0037 D1, D3): the bill's
 * five steps, a two-page form. The page that is done lifts and turns over the
 * spine; the next page is already lying underneath.
 *
 * The NEW page never moves: it is laid out in place from the first frame, its
 * fields and buttons exactly where the finger will find them («Luật Control
 * Đứng Yên»: no control turns in 3D). What turns is the OLD page -- the very
 * same mounted page, kept under its own key in the same wrapper, so nothing in
 * it re-mounts or re-fetches -- drawn over the new one with no pointer events
 * and hidden from screen readers, turning away about the left edge going
 * forward, the right edge going back, within `lat` (300 ms). Under Reduce
 * Motion it is a cut.
 */
import { useEffect, useRef, useState, type ReactNode } from "react";
import { StyleSheet, View, type StyleProp, type ViewStyle } from "react-native";
import Animated, { Easing, runOnJS, useAnimatedStyle, useSharedValue, withTiming } from "react-native-reanimated";

import { bongGiay, useRudiTheme } from "../theme";
import { useMotion } from "./useMotion";

function VoTrang({ node, lat, huong, onXong }: { node: ReactNode; lat: boolean; huong: 1 | -1; onXong: () => void }) {
  const { colors, dark } = useRudiTheme();
  const motion = useMotion();
  const goc = useSharedValue(0);
  useEffect(() => {
    if (!lat) return;
    goc.value = 0;
    goc.value = withTiming(1, { duration: motion.sanKhau.lat, easing: Easing.in(Easing.quad), reduceMotion: motion.reanimated }, (xong) => {
      if (xong) runOnJS(onXong)();
    });
    // The turn starts when this page becomes the one turning, once.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lat]);
  const kieu = useAnimatedStyle(() =>
    lat
      ? { opacity: 1 - goc.value * 0.35, transform: [{ perspective: 1200 }, { rotateY: `${huong * -92 * goc.value}deg` }] }
      : { opacity: 1, transform: [{ perspective: 1200 }, { rotateY: "0deg" }] },
  );
  return (
    <Animated.View
      importantForAccessibility={lat ? "no-hide-descendants" : "auto"}
      pointerEvents={lat ? "none" : "auto"}
      style={[
        lat ? [StyleSheet.absoluteFill, styles.lat, { backgroundColor: colors.ground, transformOrigin: huong === 1 ? "left center" : "right center" }, bongGiay(2, dark)] : null,
        kieu,
      ]}
    >
      {node}
    </Animated.View>
  );
}

export function LatTrang({
  khoa,
  thuTu,
  children,
  style,
  testID,
}: {
  /** Which page this is; a new key turns the page. */
  khoa: string;
  /** The page's place in the task, so going back turns the other way. */
  thuTu: number;
  children: ReactNode;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}) {
  const motion = useMotion();
  // The page as last committed: what turns away when the key changes.
  const daVe = useRef({ khoa, node: children, thuTu });
  const [dangLat, setDangLat] = useState<{ khoa: string; node: ReactNode; huong: 1 | -1 } | null>(null);
  const [khoaTruoc, setKhoaTruoc] = useState(khoa);
  if (khoaTruoc !== khoa) {
    setKhoaTruoc(khoa);
    if (!motion.reduced && daVe.current.khoa !== khoa) {
      setDangLat({ khoa: daVe.current.khoa, node: daVe.current.node, huong: thuTu >= daVe.current.thuTu ? 1 : -1 });
    }
  }
  useEffect(() => {
    daVe.current = { khoa, node: children, thuTu };
  });
  const lat = dangLat && dangLat.khoa !== khoa ? dangLat : null;
  return (
    <View style={[styles.khung, style]} testID={testID}>
      <VoTrang huong={1} key={khoa} lat={false} node={children} onXong={() => undefined} />
      {lat ? <VoTrang huong={lat.huong} key={lat.khoa} lat node={lat.node} onXong={() => setDangLat(null)} /> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  khung: { position: "relative" },
  lat: { backfaceVisibility: "hidden" },
});
