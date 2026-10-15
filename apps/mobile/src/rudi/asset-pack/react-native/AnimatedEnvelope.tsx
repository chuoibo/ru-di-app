import { View } from "react-native";
import Animated, { useAnimatedStyle, type WithTimingConfig } from "react-native-reanimated";
import { PaperObject, type PaperPalette } from "./PaperObject";
import { useConfirmedProgress } from "./useConfirmedProgress";
import { usePaperPalette } from "./usePaperPalette";

/** Open until the host supplies an invitation-created event; recipient still decides. */
export function AnimatedEnvelope({ eventKey, confirmedEventId, timing, palette: suppliedPalette, width = 240 }: {
  eventKey?: string | null;
  confirmedEventId?: string | null;
  timing?: WithTimingConfig;
  palette?: PaperPalette;
  width?: number;
}) {
  const themedPalette = usePaperPalette();
  const palette = suppliedPalette ?? themedPalette;
  const p = useConfirmedProgress(eventKey ?? confirmedEventId ?? null, timing, "envelope");
  const height = width * .75;
  const backdrop = useAnimatedStyle(() => ({ opacity: 1 - p.value }));
  const letter = useAnimatedStyle(() => ({ opacity: 1 - p.value, transform: [{ translateY: p.value * width * 44 / 320 }] }));
  const closing = useAnimatedStyle(() => ({ opacity: p.value > 0 ? 1 : 0, transform: [{ perspective: 600 }, { rotateX: `${-145 * (1 - p.value)}deg` }] }));
  const absolute = { position: "absolute" as const, top: 0, left: 0, width, height };
  return <View pointerEvents="none" accessibilityElementsHidden importantForAccessibility="no-hide-descendants" style={{ width, height }}>
    <Animated.View style={[absolute, backdrop]}><PaperObject name="envelope" layer="flap" width={width} palette={palette} /></Animated.View>
    <Animated.View style={[absolute, letter]}><PaperObject name="envelope" layer="letter" width={width} palette={palette} /></Animated.View>
    {(["body", "left-fold", "right-fold", "front", "seal"] as const).map(layer => <View key={layer} style={absolute}><PaperObject name="envelope" layer={layer} width={width} palette={palette} /></View>)}
    <Animated.View style={[absolute, { transformOrigin: [width / 2, width * 89 / 320, 0] }, closing]}><PaperObject name="envelope" layer="closing-flap" width={width} palette={palette} /></Animated.View>
  </View>;
}
