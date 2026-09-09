/**
 * The kit's text field, without the icon library.
 *
 * Split out of `ui.tsx` so a node test can render it: `ui.tsx` pulls in
 * `expo-image`, `@expo/vector-icons` and Reanimated, none of which load under
 * node, and the one input this app has was the one kit control no test could
 * reach. `ui.tsx` keeps `Field`/`SearchField` as thin wrappers that turn an
 * icon name into the `Ionicons` element handed in here as `leading`.
 */
import { useState, type ReactNode } from "react";
import { StyleProp, StyleSheet, Text, TextInput, TextInputProps, TextStyle, View } from "react-native";

import { typography, useRudiTheme } from "../theme";

export type FieldCoreProps = TextInputProps & {
  label?: string;
  /** The glyph before the input, already an element. */
  leading?: ReactNode;
  trailing?: ReactNode;
  style?: StyleProp<TextStyle>;
};

export function Field({ label, leading, trailing, multiline, style, placeholder, ...inputProps }: FieldCoreProps) {
  const { colors, radius } = useRudiTheme();
  // Android lays the native hint out at the box's width and lets it WRAP, even
  // in a single-line input, then clips the second line (audit native 09/09,
  // F44: «Tìm quán,» on one line, «món…» cut underneath at font scale 2.0).
  // React Native exposes no ellipsize for TextInput, so a single-line field
  // draws its own placeholder: a one-line Text over the input that truncates
  // with «…» like every other label in the kit, shown while the value is
  // empty. A multiline field keeps the native hint -- wrapping is right there.
  // The accessible name is still the whole sentence, so nothing is lost to a
  // screen reader; the overlay itself is hidden from it to avoid saying it twice.
  const [goDuoc, setGoDuoc] = useState("");
  const rong = (inputProps.value ?? goDuoc) === "";
  const placeholderNha = !multiline && placeholder !== undefined;
  return (
    <View style={styles.fieldBlock}>
      {label ? <Text style={[typography.label, { color: colors.ink }]}>{label}</Text> : null}
      <View
        style={[
          styles.field,
          multiline && styles.fieldMultiline,
          { backgroundColor: colors.card, borderColor: colors.lineStrong, borderRadius: radius.control },
        ]}
      >
        {leading}
        <View style={styles.fieldInputWrap}>
          <TextInput
            {...inputProps}
            accessibilityLabel={inputProps.accessibilityLabel ?? label ?? placeholder}
            multiline={multiline}
            onChangeText={(t) => {
              setGoDuoc(t);
              inputProps.onChangeText?.(t);
            }}
            placeholder={placeholderNha ? undefined : placeholder}
            placeholderTextColor={colors.inkFaint}
            style={[styles.fieldInput, typography.body, { color: colors.ink }, style]}
          />
          {placeholderNha && rong ? (
            <View importantForAccessibility="no-hide-descendants" pointerEvents="none" style={styles.fieldPlaceholder}>
              <Text numberOfLines={1} style={[typography.body, { color: colors.inkFaint }]}>
                {placeholder}
              </Text>
            </View>
          ) : null}
        </View>
        {trailing}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  fieldBlock: { gap: 7 },
  field: { minHeight: 52, borderWidth: 1, paddingHorizontal: 14, flexDirection: "row", alignItems: "center", gap: 10 },
  fieldMultiline: { minHeight: 108, alignItems: "flex-start", paddingTop: 13 },
  // The input is the node a finger and a screen reader land on, not the box
  // around it: 48dp on its own (Material target), inside the 52dp field.
  fieldInput: { flex: 1, minHeight: 48, paddingVertical: 0, paddingHorizontal: 0 },
  fieldInputWrap: { flex: 1, justifyContent: "center" },
  // Fills the input's own box, so the drawn placeholder starts where the caret
  // does; `paddingHorizontal: 0` above is what makes the two edges agree.
  fieldPlaceholder: { position: "absolute", left: 0, right: 0, justifyContent: "center" },
});
