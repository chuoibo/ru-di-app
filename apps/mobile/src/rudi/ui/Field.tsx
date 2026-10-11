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
import { useLayoutEffect, useMemo, useRef, useState, type ReactNode, type Ref } from "react";
import { StyleProp, StyleSheet, Text, TextInput, TextInputProps, TextStyle, View, type LayoutChangeEvent, type TextLayoutEvent, type ViewStyle } from "react-native";

import { typography, useRudiTheme } from "../theme";
import { KHONG_VIEN_WEB } from "./khong-vien-web";
import { Text as NativeMetricText, TextInput as NativeMetricTextInput, useNativeTextInputMetrics } from "./NativeFontMetrics";

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
  /** The input, for a screen that must put the cursor in it (as `ONhapMuc`). */
  oRef?: Ref<TextInput>;
  /** Opt in under the surface's existing native font metrics owner. */
  nativeMetrics?: boolean;
  /** Grow a controlled multiline input to measured native rows, up to this limit. */
  autoGrowLines?: number;
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

export function Field({ label, leading, trailing, multiline, numberOfLines, style, placeholder, helper, error, oRef, nativeMetrics = false, autoGrowLines, ...inputProps }: FieldCoreProps) {
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
  const [inputWidth, setInputWidth] = useState<number | null>(null);
  const [measuredRows, setMeasuredRows] = useState<{ generation: object; rows: number } | null>(null);
  const activeMeasurement = useRef<object | null>(null);
  const mirrorSequence = useRef(0);
  const rong = (inputProps.value ?? goDuoc) === "";
  const placeholderNha = !multiline && placeholder !== undefined;
  const coLoi = typeof error === "string" && error !== "";
  const kieu = kieuO({ multiline, numberOfLines, vien: dangNhap || coLoi ? 2 : 1 });
  const mauVien = coLoi ? colors.warn : dangNhap ? colors.accent : colors.lineStrong;
  const inputStyle: StyleProp<TextStyle> = [kieu.nhap, typography.body, { color: colors.ink }, KHONG_VIEN_WEB, style];
  const metrics = useNativeTextInputMetrics(inputStyle, inputProps);
  const original = StyleSheet.flatten(inputStyle);
  const nativeGeometry: TextStyle = {};
  if (nativeMetrics && metrics !== null) {
    const basePitch = original.lineHeight;
    if (typeof basePitch === "number" && Number.isFinite(basePitch) && basePitch > 0) {
      // Keep the caller's one/three/eight-line bounds in native line pitches.
      const ratio = metrics.linePitch / basePitch;
      if (typeof original.minHeight === "number" && Number.isFinite(original.minHeight) && original.minHeight > 0) nativeGeometry.minHeight = original.minHeight * ratio;
      if (typeof original.maxHeight === "number" && Number.isFinite(original.maxHeight) && original.maxHeight > 0) nativeGeometry.maxHeight = original.maxHeight * ratio;
    }
  }
  const padding = (edge: "Top" | "Bottom" | "Left" | "Right") => {
    const side = { Top: original.paddingTop, Bottom: original.paddingBottom, Left: original.paddingLeft, Right: original.paddingRight }[edge];
    const value = side ?? original[edge === "Top" || edge === "Bottom" ? "paddingVertical" : "paddingHorizontal"] ?? original.padding ?? 0;
    return typeof value === "number" && Number.isFinite(value) && value >= 0 ? value : null;
  };
  const border = (edge: "Top" | "Bottom" | "Left" | "Right") => {
    const side = { Top: original.borderTopWidth, Bottom: original.borderBottomWidth, Left: original.borderLeftWidth, Right: original.borderRightWidth }[edge];
    const value = side ?? original.borderWidth ?? 0;
    return typeof value === "number" && Number.isFinite(value) && value >= 0 ? value : null;
  };
  const topPadding = padding("Top"), bottomPadding = padding("Bottom"), leftPadding = padding("Left"), rightPadding = padding("Right");
  const topBorder = border("Top"), bottomBorder = border("Bottom"), leftBorder = border("Left"), rightBorder = border("Right");
  const growLimit = typeof autoGrowLines === "number" && Number.isInteger(autoGrowLines) && autoGrowLines >= 1 && autoGrowLines <= DONG_TOI_DA ? autoGrowLines : null;
  // Track width through a temporary metrics fallback, without changing its geometry.
  const observeWidth = nativeMetrics && multiline === true && typeof inputProps.value === "string" && growLimit !== null;
  const growEnabled = observeWidth && metrics !== null &&
    topPadding !== null && bottomPadding !== null && leftPadding !== null && rightPadding !== null &&
    topBorder !== null && bottomBorder !== null && leftBorder !== null && rightBorder !== null &&
    original.height === undefined && original.paddingStart === undefined && original.paddingEnd === undefined && original.borderStartWidth === undefined && original.borderEndWidth === undefined &&
    Number.isFinite(metrics.linePitch) && metrics.linePitch > 0 &&
    typeof nativeGeometry.minHeight === "number" && Number.isFinite(nativeGeometry.minHeight) &&
    typeof nativeGeometry.maxHeight === "number" && Number.isFinite(nativeGeometry.maxHeight) && nativeGeometry.maxHeight >= nativeGeometry.minHeight;
  const body = inputProps.value ?? "";
  const contentWidth = inputWidth !== null && leftPadding !== null && rightPadding !== null && leftBorder !== null && rightBorder !== null ? inputWidth - leftPadding - rightPadding - leftBorder - rightBorder : null;
  const breakStrategy = inputProps.textBreakStrategy ?? "simple";
  const includeFontPadding = original.includeFontPadding ?? true;
  // A revision contains no draft text in its key or diagnostic representation.
  const generation = useMemo(() => ({}), [growEnabled, body, contentWidth, growLimit, metrics?.linePitch,
    metrics?.style.fontSize, metrics?.style.lineHeight, metrics?.style.letterSpacing,
    original.fontFamily, original.fontWeight, original.fontStyle, JSON.stringify(original.fontVariant), original.textTransform,
    original.textAlign, original.writingDirection, includeFontPadding, breakStrategy, topPadding, bottomPadding, topBorder, bottomBorder]);
  const mirrorKey = useMemo(() => ++mirrorSequence.current, [generation]);
  useLayoutEffect(() => {
    activeMeasurement.current = generation;
    // An unavailable/capped path or an empty draft cannot donate rows on recovery.
    if (!growEnabled || body === "") setMeasuredRows(previous => previous === null ? previous : null);
    return () => { if (activeMeasurement.current === generation) activeMeasurement.current = null; };
  }, [generation, growEnabled, body]);
  // Keep the last row count provisionally until this revision is measured; empty clears immediately.
  const rows = body === "" ? 1 : Math.min(growLimit ?? 1, measuredRows?.rows ?? 1);
  const growGeometry: TextStyle = {};
  if (growEnabled && metrics !== null && topPadding !== null && bottomPadding !== null && topBorder !== null && bottomBorder !== null) {
    growGeometry.height = Math.max(nativeGeometry.minHeight as number, Math.min(nativeGeometry.maxHeight as number, rows * metrics.linePitch + topPadding + bottomPadding + topBorder + bottomBorder));
    growGeometry.flex = 0;
  }
  const onInputLayout = (event: LayoutChangeEvent) => {
    inputProps.onLayout?.(event);
    if (activeMeasurement.current !== generation) return;
    const width = event?.nativeEvent?.layout?.width;
    if (typeof width !== "number" || !Number.isFinite(width) || width <= 0) return;
    setInputWidth(previous => activeMeasurement.current === generation && previous !== width ? width : previous);
  };
  const onMeasuredText = (event: TextLayoutEvent) => {
    if (!growEnabled || activeMeasurement.current !== generation) return;
    const lines = event?.nativeEvent?.lines;
    if (!Array.isArray(lines) || lines.length === 0) return;
    let bottom = -Infinity;
    for (const line of lines) {
      if (line === null || typeof line !== "object" || !Number.isFinite(line.x) || !Number.isFinite(line.y) || line.y < 0 || !Number.isFinite(line.width) || line.width < 0 ||
        !Number.isFinite(line.height) || line.height <= 0 || !Number.isFinite(line.y + line.height)) return;
      // RN divides the pixel top and height separately through Float32. Their
      // JS sum can round past the next adjacent native top by a few Float32 ULPs.
      if (Number.isFinite(bottom)) {
        const roundoff = 4 * 2 ** -23 * Math.max(1, Math.abs(bottom), Math.abs(line.y));
        if (bottom - line.y > roundoff) return;
      }
      bottom = line.y + line.height;
    }
    const nextRows = Math.min(growLimit as number, lines.length);
    setMeasuredRows(previous => activeMeasurement.current !== generation || (previous?.generation === generation && previous.rows === nextRows) ? previous : { generation, rows: nextRows });
  };
  const Input = nativeMetrics ? NativeMetricTextInput : TextInput;
  const CopyText = nativeMetrics ? NativeMetricText : Text;
  return (
    <View style={styles.fieldBlock}>
      {label ? <CopyText style={[typography.label, { color: colors.ink }]}>{label}</CopyText> : null}
      <View style={[kieu.khung, { backgroundColor: colors.card, borderColor: mauVien, borderRadius: radius.control }]}>
        {leading}
        <View style={kieu.boc}>
          <Input
            {...inputProps}
            {...(observeWidth ? { onLayout: onInputLayout } : {})}
            {...(growEnabled ? { textBreakStrategy: breakStrategy } : {})}
            ref={oRef}
            accessibilityLabel={inputProps.accessibilityLabel ?? label ?? placeholder}
            aria-invalid={coLoi || undefined}
            multiline={multiline}
            numberOfLines={growEnabled ? rows : numberOfLines}
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
            style={nativeMetrics ? [inputStyle, nativeGeometry, growGeometry] : inputStyle}
          />
          {growEnabled && metrics !== null && body !== "" && contentWidth !== null && Number.isFinite(contentWidth) && contentWidth > 0 ? (
            <Text
              key={mirrorKey}
              testID={`${inputProps.testID ?? "field"}-measurement`}
              accessible={false}
              importantForAccessibility="no-hide-descendants"
              accessibilityElementsHidden
              pointerEvents="none"
              selectable={false}
              allowFontScaling={false}
              textBreakStrategy={breakStrategy}
              android_hyphenationFrequency="none"
              numberOfLines={growLimit as number}
              ellipsizeMode="clip"
              onTextLayout={onMeasuredText}
              style={[metrics.placeholderStyle, { includeFontPadding, textAlign: original.textAlign, writingDirection: original.writingDirection,
                position: "absolute", left: (leftPadding as number) + (leftBorder as number), top: 0, opacity: 0, width: contentWidth, padding: 0 }]}
            >{body}</Text>
          ) : null}
          {placeholderNha && rong ? (
            <View importantForAccessibility="no-hide-descendants" pointerEvents="none" style={styles.fieldPlaceholder}>
              <Text numberOfLines={1} allowFontScaling={nativeMetrics && metrics !== null ? false : undefined} style={[typography.body, { color: colors.inkFaint }, nativeMetrics && metrics?.placeholderStyle]}>
                {placeholder}
              </Text>
            </View>
          ) : null}
        </View>
        {trailing}
      </View>
      {coLoi ? (
        // An alert by role so it is found as an error, announced politely so
        // it never cuts off what the person is typing.
        <CopyText accessibilityLiveRegion="polite" accessibilityRole="alert" aria-live="polite" style={[typography.caption, { color: colors.warn }]}>
          {error}
        </CopyText>
      ) : helper ? (
        <CopyText style={[typography.caption, { color: colors.inkSoft }]}>{helper}</CopyText>
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
