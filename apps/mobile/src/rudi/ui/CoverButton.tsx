import { Ionicons } from "@expo/vector-icons";
import { StyleSheet, Text, View, type StyleProp, type ViewStyle } from "react-native";

import { typography, useRudiTheme } from "../theme";
import { PressScale } from "./PressScale";

export interface CoverButtonProps {
  label: string;
  onPress: () => void;
  icon?: keyof typeof Ionicons.glyphMap;
  disabled?: boolean;
  loading?: boolean;
  style?: StyleProp<ViewStyle>;
  testID?: string;
  /** `link`: no border, as wide as its words; the quiet second choice under a stamp. */
  variant?: "outline" | "link";
}

/**
 * The quiet action on the indigo cover: no fill, so the border is the whole
 * affordance and it is drawn with `coverLineStrong`, the one token measured
 * ≥ 3:1 against the cover in both schemes (services/api/tests/web/
 * test_contrast_floor.py reads it out of this file). Label in `coverInk`.
 */
export function CoverButton({ label, onPress, icon, disabled, loading, variant = "outline", style, testID }: CoverButtonProps) {
  const { colors } = useRudiTheme();
  // `link`: under the stamp on the cover, a full-width outlined bar outranked
  // the seal it was meant to follow (finish review, 2026-09-06). A link is as
  // wide as its words and keeps the 48dp target through its own height.
  const link = variant === "link";
  return (
    <PressScale
      accessibilityLabel={label}
      accessibilityRole="button"
      accessibilityState={{ disabled: !!disabled, busy: !!loading }}
      disabled={disabled || loading}
      haptic="select"
      onPress={onPress}
      testID={testID}
      style={[styles.quiet, link && styles.link, { borderColor: colors.coverLineStrong, opacity: disabled ? 0.55 : 1 }, style]}
    >
      <View style={styles.row}>
        <Text style={[typography.label, { color: colors.coverInk }]}>{label}</Text>
        {icon ? <Ionicons color={colors.coverInk} name={icon} size={18} /> : null}
      </View>
    </PressScale>
  );
}

const styles = StyleSheet.create({
  quiet: { minHeight: 50, borderWidth: 1, borderRadius: 14, paddingHorizontal: 18, justifyContent: "center" },
  link: { borderWidth: 0, alignSelf: "center", minHeight: 48, paddingHorizontal: 12 },
  row: { flexDirection: "row", alignItems: "center", justifyContent: "center", gap: 6 },
});
