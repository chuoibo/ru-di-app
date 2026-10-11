import { useRef, useState, type ReactNode } from "react";
import { PixelRatio, StyleSheet, View, useWindowDimensions, type StyleProp, type TextProps, type TextStyle } from "react-native";
import { Text, useNativeStandaloneTextMetrics } from "./NativeFontMetrics";

export type SealLabelTelemetry = Pick<TextProps, "onLayout" | "onTextLayout">;

/** A finite, invisible measurement column; the visible seal stays cut to its words. */
export function MeasuredSealColumn({ label, textStyle, horizontalInset, allocatedWidth, enabled = true, children }: {
  label: string;
  textStyle: StyleProp<TextStyle>;
  horizontalInset: number;
  /** A row can supply its finite width while this wrapper stays content-sized. */
  allocatedWidth?: number;
  enabled?: boolean;
  children: (width: number | undefined, telemetry?: SealLabelTelemetry) => ReactNode;
}) {
  const [ownColumnWidth, setColumnWidth] = useState(0);
  const [measurement, setMeasurement] = useState<{ key: string; width: number | null } | null>(null);
  const currentKey = useRef("");
  const trace = useRef<{ key: string; seen: Set<string> }>({ key: "", seen: new Set() });
  const { fontScale } = useWindowDimensions();
  const density = PixelRatio.get();
  const original = StyleSheet.flatten(textStyle) ?? {};
  const converted = useNativeStandaloneTextMetrics(textStyle);
  const effective = converted ?? original;
  const columnWidth = allocatedWidth === undefined ? ownColumnWidth : Number.isFinite(allocatedWidth) ? allocatedWidth : 0;
  const available = Math.max(0, columnWidth - horizontalInset);
  const key = JSON.stringify([label, original.fontFamily, original.fontWeight, original.fontStyle,
    original.fontVariant, original.textTransform, effective.fontSize, effective.lineHeight,
    effective.letterSpacing, converted === null ? fontScale : "manual", density, available, enabled]);
  currentKey.current = key;
  const width = enabled && available > 0 && measurement?.key === key
    ? Math.min(measurement.width ?? available, available)
    : undefined;
  const diagnose = process.env.EXPO_PUBLIC_SEAL_FIT_DIAGNOSTICS === "1";
  const emit = (stage: string, geometry: unknown) => {
    if (!diagnose || currentKey.current !== key) return;
    if (trace.current.key !== key) trace.current = { key, seen: new Set() };
    const signature = JSON.stringify([stage, width, geometry]);
    // Bound diagnostics without scheduling layouts or changing fitted width.
    if (trace.current.seen.has(signature) || trace.current.seen.size >= 12) return;
    trace.current.seen.add(signature);
    console.info("[rudi-seal-fit] " + JSON.stringify({ stage, sequence: trace.current.seen.size,
      callbackUtc: new Date().toISOString(), key, label, density,
      fontScale, manualMetrics: converted !== null, columnWidth, horizontalInset,
      available, assignedWidth: width ?? null,
      fontFamily: original.fontFamily, fontWeight: original.fontWeight,
      fontStyle: original.fontStyle, fontVariant: original.fontVariant,
      textTransform: original.textTransform, fontSize: effective.fontSize,
      lineHeight: effective.lineHeight, letterSpacing: effective.letterSpacing, geometry }));
  };
  const visibleTelemetry: SealLabelTelemetry | undefined = diagnose ? {
    onLayout: (event) => emit("visible-layout", event.nativeEvent.layout),
    onTextLayout: (event) => emit("visible-lines", event.nativeEvent.lines),
  } : undefined;
  return (
    <View accessible={false} pointerEvents="box-none" onLayout={(event) => {
      const next = event.nativeEvent.layout.width;
      if (allocatedWidth === undefined && Number.isFinite(next) && next >= 0) setColumnWidth((old) => old === next ? old : next);
    }} style={[styles.column, allocatedWidth === undefined && styles.fullColumn]}>
      <>
      {enabled && available > 0 ? (
        <Text key={key} accessible={false} importantForAccessibility="no-hide-descendants" pointerEvents="none"
          onLayout={diagnose ? (event) => emit("wide-layout", event.nativeEvent.layout) : undefined}
          onTextLayout={(event) => {
            if (currentKey.current !== key) return;
            const lines = event.nativeEvent.lines;
            // Match HoSoSong: only a full single line is intrinsic; otherwise use the finite cap.
            const measured = lines.length === 1 && Number.isFinite(lines[0].width) && lines[0].width > 0
              ? Math.ceil(lines[0].width * density) / density : null;
            emit("wide-lines", { lines, roundedIntrinsicWidth: measured });
            setMeasurement((old) => old?.key === key && old.width === measured ? old : { key, width: measured });
          }} style={[textStyle, styles.measure, { left: horizontalInset / 2, width: available }]}>{label}</Text>
      ) : null}
      {diagnose && enabled && width !== undefined && width > 0 ? (
        <Text key={key + ":fit:" + width} accessible={false} importantForAccessibility="no-hide-descendants" pointerEvents="none"
          onLayout={(event) => emit("fit-layout", event.nativeEvent.layout)}
          onTextLayout={(event) => emit("fit-lines", event.nativeEvent.lines)}
          style={[textStyle, styles.measure, { left: horizontalInset / 2, width }]}>{label}</Text>
      ) : null}
      </>
      {children(width, visibleTelemetry)}
    </View>
  );
}

const styles = StyleSheet.create({
  column: { alignSelf: "flex-start", minWidth: 0 },
  fullColumn: { alignSelf: "stretch", width: "100%" },
  measure: { position: "absolute", opacity: 0 },
});
