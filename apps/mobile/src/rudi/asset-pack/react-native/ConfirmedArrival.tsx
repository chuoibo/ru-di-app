import type { ReactNode } from "react";
import Animated, { useAnimatedStyle, type WithTimingConfig } from "react-native-reanimated";
import { useConfirmedProgress } from "./useConfirmedProgress";

/** An existing image stays visible; only a newly confirmed response settles it. */
export function ConfirmedArrival({ eventId, timing, children }: { eventId: string | null; timing?: WithTimingConfig; children: ReactNode }) {
  const p = useConfirmedProgress(eventId, timing, "arrival", true, 1);
  const style = useAnimatedStyle(() => ({ opacity: .65 + .35 * p.value, transform: [{ translateY: -2 * (1 - p.value) }] }));
  return <Animated.View style={style}>{children}</Animated.View>;
}
