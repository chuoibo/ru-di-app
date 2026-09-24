/**
 * The kit's text field, without the icon library.
 *
 * Split out of `ui.tsx` so a node test can render it: `ui.tsx` pulls in
 * `expo-image`, `@expo/vector-icons` and Reanimated, none of which load under
 * node, and the one input this app has was the one kit control no test could
 * reach. `ui.tsx` keeps `Field`/`SearchField` as thin wrappers that turn an
 * icon name into the `Ionicons` element handed in here as `leading`.
 *
 * ## A multiline field writes from the top (QA 23/09)
 *
 * Every multiline box in the app started its text in the MIDDLE of the box:
 * the input sat in a wrapper centred on the cross axis, Android's EditText
 * centres its gravity unless `textAlignVertical` says otherwise, and the box
 * had a top padding with no bottom one. `kieuO` is the one place that decides
 * the geometry now -- text and hint at the top-left, the same padding above
 * and below, the height following `numberOfLines`, and a ceiling past which
 * the box scrolls instead of pushing its form off the sheet.
 *
 * ## Focus and errors
 *
 * The field being written in carries a 2dp accent border (the width is taken
 * from the padding, so nothing moves), and an `error` replaces the `helper`
 * line under the box, in the warning colour, announced politely.
 */
import { useState, type ReactNode } from "react";
import { StyleProp, StyleSheet, Text, TextInput, TextInputProps, TextStyle, View, type ViewStyle } from "react-native";

import { typography, useRudiTheme } from "../theme";

export type FieldCoreProps = TextInputProps & {
  label?: string;
  /** The glyph before the input, already an element. */
  leading?: ReactNode;
  trailing?: ReactNode;
  style?: StyleProp<TextStyle>;
  /** A quiet line under the box: format, limit, what the field is for. */
  helper?: string;
  /** What is wrong with the value; replaces `helper` and colours the border. */
  error?: string | null;
};

/** Body text's line height; the multiline box is sized in lines of it. */
const DONG = typography.body.lineHeight;
/** Padding inside the box, above and below the text alike. */
const DEM_DOC = 12;
const DEM_NGANG = 14;
/** Past this many lines a multiline box scrolls. */
const DONG_TOI_DA = 8;

export interface KieuO {
  khung: ViewStyle;
  boc: ViewStyle;
  nhap: TextStyle;
}

/**
 * The geometry of a field, as a pure function so a node test can hold it:
 * `vien` is the border width (2 while focused or in error, else 1), and the
 * padding gives back what the border takes so the text never moves.
 */
export function kieuO({ multiline = false, numberOfLines, vien = 1 }: { multiline?: boolean; numberOfLines?: number; vien?: number }): KieuO {
  const bu = vien - 1;
  if (!multiline) {
    return {
      khung: { minHeight: 52, borderWidth: vien, paddingHorizontal: DEM_NGANG - bu, flexDirection: "row", alignItems: "center", gap: 10 },
      boc: { flex: 1, justifyContent: "center" },
      nhap: { flex: 1, minHeight: 48, paddingVertical: 0, paddingHorizontal: 0 },
    };
  }
  const dong = Math.max(3, Math.min(DONG_TOI_DA, Math.floor(numberOfLines ?? 3)));
  return {
    khung: {
      borderWidth: vien,
      paddingHorizontal: DEM_NGANG - bu,
      paddingVertical: DEM_DOC - bu,
      flexDirection: "row",
      alignItems: "flex-start",
      gap: 10,
    },
    boc: { flex: 1, alignSelf: "stretch", justifyContent: "flex-start" },
    nhap: {
      flex: 1,
      minHeight: dong * DONG,
      maxHeight: DONG_TOI_DA * DONG,
      paddingVertical: 0,
      paddingHorizontal: 0,
      textAlignVertical: "top",
    },
  };
}

export function Field({ label, leading, trailing, multiline, numberOfLines, style, placeholder, helper, error, ...inputProps }: FieldCoreProps) {
  const { colors, radius } = useRudiTheme();
  // Android lays the native hint out at the box's width and lets it WRAP, even
  // in a single-line input, then clips the second line (audit native 09/09,
  // F44: «Tìm quán,» on one line, «món…» cut underneath at font scale 2.0).
  // React Native exposes no ellipsize for TextInput, so a single-line field
  // draws its own placeholder: a one-line Text over the input that truncates
  // with «…» like every other label in the kit, shown while the value is
  // empty. A multiline field keeps the native hint -- wrapping is right there,
  // and `textAlignVertical: "top"` puts it where the caret starts.
  // The accessible name is still the whole sentence, so nothing is lost to a
  // screen reader; the overlay itself is hidden from it to avoid saying it twice.
  const [goDuoc, setGoDuoc] = useState("");
  const [dangNhap, setDangNhap] = useState(false);
  const rong = (inputProps.value ?? goDuoc) === "";
  const placeholderNha = !multiline && placeholder !== undefined;
  const coLoi = typeof error === "string" && error !== "";
  const kieu = kieuO({ multiline, numberOfLines, vien: dangNhap || coLoi ? 2 : 1 });
  const mauVien = coLoi ? colors.warn : dangNhap ? colors.accent : colors.lineStrong;
  return (
    <View style={styles.fieldBlock}>
      {label ? <Text style={[typography.label, { color: colors.ink }]}>{label}</Text> : null}
      <View style={[kieu.khung, { backgroundColor: colors.card, borderColor: mauVien, borderRadius: radius.control }]}>
        {leading}
        <View style={kieu.boc}>
          <TextInput
            {...inputProps}
            accessibilityLabel={inputProps.accessibilityLabel ?? label ?? placeholder}
            aria-invalid={coLoi || undefined}
            multiline={multiline}
            numberOfLines={numberOfLines}
            onBlur={(e) => {
              setDangNhap(false);
              inputProps.onBlur?.(e);
            }}
            onChangeText={(t) => {
              setGoDuoc(t);
              inputProps.onChangeText?.(t);
            }}
            onFocus={(e) => {
              setDangNhap(true);
              inputProps.onFocus?.(e);
            }}
            placeholder={placeholderNha ? undefined : placeholder}
            placeholderTextColor={colors.inkFaint}
            style={[kieu.nhap, typography.body, { color: colors.ink }, style]}
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
      {coLoi ? (
        <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>
          {error}
        </Text>
      ) : helper ? (
        <Text style={[typography.caption, { color: colors.inkSoft }]}>{helper}</Text>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  fieldBlock: { gap: 7 },
  // Fills the input's own box, so the drawn placeholder starts where the caret
  // does; `paddingHorizontal: 0` in `kieuO` is what makes the two edges agree.
  fieldPlaceholder: { position: "absolute", left: 0, right: 0, top: 0, bottom: 0, justifyContent: "center" },
});
