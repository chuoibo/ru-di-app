import { useLayoutEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { PixelRatio, StyleSheet, View, useWindowDimensions, type StyleProp, type TextProps, type TextStyle } from "react-native";
import { Text, useNativeStandaloneTextMetrics } from "./NativeFontMetrics";

export type SealLabelTelemetry = Pick<TextProps, "onLayout" | "onTextLayout">;

type FitMeasurement = {
  epoch: object;
  generation: number;
  token: object;
  attempt: number;
  phase: "upper" | "candidate" | "search" | "done";
  candidate: number | null;
  requested: number;
  accepted: number | null;
  failed: number | null;
  steps: number;
};
const MAX_FIT_STEPS = 12;
const MAX_SEARCH_PIXELS = 65536;

function lineWidths(lines: unknown, cap = Infinity): number[] | null {
  if (!Array.isArray(lines) || lines.length === 0) return null;
  const widths: number[] = [];
  for (const line of lines) {
    if (typeof line !== "object" || line === null) return null;
    const width: unknown = (line as { width?: unknown }).width;
    if (typeof width !== "number" || !Number.isFinite(width) || width <= 0 || width > cap) return null;
    widths.push(width);
  }
  return widths;
}

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
  const [measurement, setMeasurement] = useState<FitMeasurement | null>(null);
  const committed = useRef<{ epoch: object; ordinal: number } | null>(null);
  const ordinal = useRef(0);
  const currentAttempt = useRef<object | null>(null);
  const trace = useRef<{ generation: number; seen: Set<string> }>({ generation: 0, seen: new Set() });
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
  // A new object also distinguishes A -> B -> A from a retired callback for A.
  const epoch = useMemo(() => ({}), [key]);
  const fit = measurement?.epoch === epoch ? measurement : null;
  const canFit = enabled && Number.isFinite(available) && available > 0;
  const pixelRange = available * density;
  const searchable = Number.isFinite(density) && density > 0 && Number.isFinite(pixelRange) && pixelRange <= MAX_SEARCH_PIXELS;
  const width = canFit && fit?.phase === "done" ? fit.accepted ?? available : undefined;
  useLayoutEffect(() => {
    const lease = { epoch, ordinal: ++ordinal.current };
    committed.current = lease;
    return () => { if (committed.current === lease) committed.current = null; };
  }, [epoch]);
  useLayoutEffect(() => {
    const token = fit?.token ?? null;
    currentAttempt.current = token;
    return () => { if (currentAttempt.current === token) currentAttempt.current = null; };
  }, [epoch, fit?.token]);
  const diagnose = process.env.EXPO_PUBLIC_SEAL_FIT_DIAGNOSTICS === "1";
  const emit = (stage: string, geometry: unknown) => {
    const lease = committed.current;
    if (!diagnose || lease?.epoch !== epoch) return;
    if (trace.current.generation !== lease.ordinal) trace.current = { generation: lease.ordinal, seen: new Set() };
    const signature = JSON.stringify([stage, fit?.attempt, width, geometry]);
    // Diagnostics report the active fit controller; logging never advances it.
    if (trace.current.seen.has(signature) || trace.current.seen.size >= 64) return;
    trace.current.seen.add(signature);
    console.info("[rudi-seal-fit] " + JSON.stringify({ stage, sequence: trace.current.seen.size,
      callbackUtc: new Date().toISOString(), key, generation: lease.ordinal, attempt: fit?.attempt ?? null,
      phase: fit?.phase ?? "wide", candidateWidth: fit?.requested ?? null, certifiedWidth: fit?.accepted ?? null,
      label, density, fontScale, manualMetrics: converted !== null, columnWidth, horizontalInset,
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
      {canFit ? (
        <Text key={key} accessible={false} importantForAccessibility="no-hide-descendants" pointerEvents="none"
          onLayout={diagnose ? (event) => emit("wide-layout", event.nativeEvent.layout) : undefined}
          onTextLayout={(event) => {
            const lease = committed.current;
            if (lease?.epoch !== epoch) return;
            const lines = event?.nativeEvent?.lines;
            const widths = lineWidths(lines);
            const intrinsic = widths?.length === 1 && searchable
              ? Math.min(Math.ceil(widths[0] * density) / density, available) : null;
            emit("wide-lines", { lines, roundedIntrinsicWidth: intrinsic });
            setMeasurement((old) => {
              if (committed.current !== lease || old?.epoch === epoch) return old;
              // A line's advance is a candidate, not a certificate at that width.
              return { epoch, generation: lease.ordinal, token: {}, attempt: 1,
                phase: widths === null ? "done" : "upper", candidate: intrinsic,
                requested: available, accepted: null, failed: null, steps: 0 };
            });
          }} style={[textStyle, styles.measure, { left: horizontalInset / 2, width: available }]}>{label}</Text>
      ) : null}
      {canFit && fit && fit.phase !== "done" ? (
        <Text key={key + ":fit:" + fit.attempt + ":" + fit.requested} accessible={false} importantForAccessibility="no-hide-descendants" pointerEvents="none"
          onLayout={diagnose ? (event) => emit("fit-layout", event.nativeEvent.layout) : undefined}
          onTextLayout={(event) => {
            const lease = committed.current;
            if (lease?.epoch !== epoch || lease.ordinal !== fit.generation || currentAttempt.current !== fit.token) return;
            const lines = event?.nativeEvent?.lines;
            const widths = lineWidths(lines, fit.requested);
            const fits = widths?.length === 1;
            emit("fit-lines", lines);
            setMeasurement((old) => {
              if (committed.current !== lease || currentAttempt.current !== fit.token || old?.token !== fit.token) return old;
              const steps = fit.steps + 1;
              const done = (accepted: number | null): FitMeasurement => ({ ...fit, token: {}, phase: "done", accepted, steps });
              // Malformed events or an exhausted search retain a verified upper,
              // or use the finite column honestly when no single line was found.
              if (widths === null) return done(fit.accepted);
              if (fit.phase === "upper") {
                if (!fits) return done(null);
                if (!searchable || fit.candidate === null || fit.candidate >= available) return done(available);
                return { ...fit, token: {}, attempt: fit.attempt + 1, phase: "candidate", requested: fit.candidate, accepted: available, steps };
              }
              if (fit.phase === "candidate" && fits) return done(fit.requested);
              const accepted = fits ? fit.requested : fit.accepted;
              const failed = fits ? fit.failed : fit.requested;
              if (accepted === null || failed === null || steps >= MAX_FIT_STEPS) return done(accepted);
              // First ask the next physical pixel; later requests bisect only
              // the remaining failed/verified bracket, never add guessed padding.
              const next = fit.phase === "candidate"
                ? failed + 1 / density
                : Math.floor((failed * density + accepted * density) / 2) / density;
              if (!Number.isFinite(next) || next <= failed || next >= accepted) return done(accepted);
              return { ...fit, token: {}, attempt: fit.attempt + 1, phase: "search", requested: next, accepted, failed, steps };
            });
          }}
          style={[textStyle, styles.measure, { left: horizontalInset / 2, width: fit.requested }]}>{label}</Text>
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
