import { type StyleProp, type TextStyle } from "react-native";
import { Text } from "../../ui/NativeFontMetrics";
import Animated, { useAnimatedStyle, type WithTimingConfig } from "react-native-reanimated";
import { useConfirmedProgress } from "./useConfirmedProgress";

/** A ledger-written visual; it never declares that money was received. */
export function LedgerStamp({ confirmedEventId, timing, color, textStyle }: {
  confirmedEventId: string | null;
  timing?: WithTimingConfig;
  color: string;
  textStyle?: StyleProp<TextStyle>;
}) {
  const p = useConfirmedProgress(confirmedEventId, timing, "ledger");
  const style = useAnimatedStyle(() => ({ opacity: p.value, transform: [{ scale: 1.04 - p.value * .04 }, { rotate: "-5deg" }] }));
  if (!confirmedEventId) return null;
  return <Animated.View style={[{ borderWidth: 1.5, borderColor: color, paddingHorizontal: 8, paddingVertical: 4, alignSelf: "flex-start" }, style]}>
    <Text style={[{ color, fontSize: 12, fontWeight: "700" }, textStyle]}>ĐÃ GHI SỔ</Text>
  </Animated.View>;
}
