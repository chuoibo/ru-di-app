import { useEffect } from "react";
import { StyleSheet, Text, type StyleProp, type ViewStyle } from "react-native";
import Animated, { Easing, runOnJS, useAnimatedStyle, useSharedValue, withDelay, withSequence, withSpring, withTiming } from "react-native-reanimated";

import { typography, useRudiTheme, type RudiTone } from "../theme";
import { useMotion } from "./useMotion";

export interface StampProps {
  /** Short, factual: «ĐÃ TỚI», «ĐÃ TRẢ», «ĐANG MỞ», «AI GỢI Ý». */
  label: string;
  /** `ink` for a state that is a fact (a plan, a memory, a closed week): mực, not the ask's coral. */
  tone?: RudiTone | "ink";
  /** `ink` for a filled seal (rare: the one state that matters most on the screen). */
  variant?: "outline" | "ink";
  /** A slight rotation makes a seal read as pressed, not printed; 0 for tables. */
  tilt?: -3 | -2 | 0 | 2 | 3;
  /**
   * The state this seal names just became true, by the person's own action:
   * the seal lands (a `celebrate` beat, once) instead of simply being there.
   * Leave false when the screen mounts with the state already true, so a
   * remount never replays the moment.
   */
  dong?: boolean;
  /** A paper chip under the seal, for a seal laid over a photograph. */
  nen?: boolean;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

/**
 * A rubber stamp for a state that is true.
 *
 * DESIGN.md already ruled that a status is a static chip, not inline text.
 * In the journal world that chip is an ink seal: a 2dp border in the tone,
 * condensed caps from the display face, a hair of rotation. It carries a
 * meaning colour (teal = money, orange = the ask, violet = AI) *and* a word,
 * so state never rides on colour alone. Not a control: no role, no press.
 *
 * `dong` is the contract's signature interaction: when a state becomes true
 * under the person's finger, the seal comes down in three beats inside the
 * `celebrate` budget: the strike (scale 1.35 -> 1 in ~130 ms, accelerating,
 * ink still pale), the contact (ink to full in 60 ms, the success haptic
 * fires here, not at the start), then a small sink and settle (1 -> 0.97 ->
 * 1 on the press spring). A single ease-out zoom read as every library's
 * default entrance (wave 8 review). Reduce Motion collapses all three to a
 * static seal; the budget is one landing per event, which the caller keeps
 * by passing `dong` only for the row it just acted on.
 */
export function Stamp({ label, tone = "accent", variant = "outline", tilt = 0, dong = false, nen = false, style, testID }: StampProps) {
  const { colors } = useRudiTheme();
  const motion = useMotion();
  const ink = tone === "ink" ? colors.ink : colors[tone];
  const filled = variant === "ink";
  const onInk = tone === "ink" ? colors.paper : tone === "accent" ? colors.accentInk : tone === "split" ? colors.splitInk : colors.aiInk;
  // `roi` is the drop (0 -> 1 maps scale 1.35 -> 1), `muc` the ink
  // (0.25 -> 1), `lun` the sink after contact (1 -> 0.97 -> 1), each its own
  // value so no phase is read through another's curve.
  const roi = useSharedValue(dong ? 0 : 1);
  const muc = useSharedValue(dong ? 0 : 1);
  const lun = useSharedValue(1);
  useEffect(() => {
    if (!dong) return;
    const lao = motion.reduced ? 0 : 130;
    const cham = motion.reduced ? 0 : 60;
    const chamXong = () => motion.haptic.success();
    roi.value = 0;
    muc.value = 0;
    lun.value = 1;
    roi.value = withTiming(1, { duration: lao, easing: Easing.in(Easing.quad) }, (finished) => {
      if (finished) runOnJS(chamXong)();
    });
    muc.value = withDelay(lao, withTiming(1, { duration: cham }));
    lun.value = withDelay(lao, withSequence(withTiming(0.97, { duration: cham }), withSpring(1, motion.spring.press)));
  }, [dong, motion, roi, muc, lun]);
  const dongXuong = useAnimatedStyle(() => ({
    opacity: 0.25 + 0.75 * muc.value,
    transform: [{ rotate: `${tilt}deg` }, { scale: (1.35 - 0.35 * roi.value) * lun.value }],
  }));
  return (
    <Animated.View
      testID={testID}
      accessibilityLabel={label}
      style={[
        styles.seal,
        {
          borderColor: ink,
          backgroundColor: filled ? ink : nen ? colors.card : "transparent",
        },
        dongXuong,
        style,
      ]}
    >
      <Text style={[typography.stamp, { color: filled ? onInk : ink }]}>{label}</Text>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  seal: {
    alignSelf: "flex-start",
    borderWidth: 2,
    borderRadius: 6,
    paddingHorizontal: 8,
    paddingVertical: 4,
    minHeight: 26,
    justifyContent: "center",
  },
});
