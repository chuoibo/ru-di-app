import { Children, Fragment, createContext, forwardRef, isValidElement, useContext, type ReactNode } from "react";
import {
  StyleSheet, Text as RNText, TextInput as RNTextInput,
  type StyleProp, type TextProps, type TextInputProps, type TextStyle,
} from "react-native";
import type { FontScaleSnapshot, NativeFontMetrics } from "../native-font-snapshot";
import { resolveTextMetrics, type OriginalTextMetrics, type ScalingProps } from "../native-font-metrics";

const MetricsContext = createContext<NativeFontMetrics | undefined>(undefined);
// An explicit provider owns native fallback as well as converted metrics.
const MetricsOwnerContext = createContext(false);
type Paragraph = { mode: "native" } | { mode: "manual"; original: OriginalTextMetrics };
const ParagraphContext = createContext<Paragraph | undefined>(undefined);
const NATIVE: Paragraph = { mode: "native" };

/** Always mounted above the surface, including its footer; no global RN defaults. */
export function NativeFontMetricsProvider({ snapshot, children }: { snapshot: FontScaleSnapshot; children: ReactNode }) {
  const metrics = snapshot.status === "available" ? snapshot.fontMetrics : undefined;
  return <MetricsOwnerContext.Provider value={true}><MetricsContext.Provider value={metrics}>{children}</MetricsContext.Provider></MetricsOwnerContext.Provider>;
}

/** Presence stays true when a provider has no usable native metrics. */
export function useNativeFontMetricsOwner(): boolean {
  return useContext(MetricsOwnerContext);
}

function flattened(style: StyleProp<TextStyle>): TextStyle {
  return StyleSheet.flatten(style) ?? {};
}

// Opaque inline components and raw RN Text keep the entire paragraph on RN's
// original path. Never convert descendants underneath a native-scaled root.
function convertibleChildren(children: ReactNode, metrics: NativeFontMetrics, parent: OriginalTextMetrics): boolean {
  return Children.toArray(children).every((child) => {
    if (typeof child === "string" || typeof child === "number") return true;
    if (!isValidElement<{ children?: ReactNode; style?: StyleProp<TextStyle> } & ScalingProps>(child)) return false;
    if (child.type === Fragment) return convertibleChildren(child.props.children, metrics, parent);
    if (child.type !== Text) return false;
    const resolved = resolveTextMetrics(metrics, flattened(child.props.style), child.props, "text", parent);
    return resolved !== null && convertibleChildren(child.props.children, metrics, resolved.original);
  });
}

export type Text = RNText;
export interface NativeMetricTextProps extends TextProps {
  /** Replace only a nonselectable native text leaf when its converted metrics change. */
  invalidateOnMetricsChange?: boolean;
}
export const Text = forwardRef<RNText, NativeMetricTextProps>(function NativeMetricText(props, ref) {
  const { invalidateOnMetricsChange, ...nativeProps } = props;
  const metrics = useContext(MetricsContext);
  const parent = useContext(ParagraphContext);
  const resolved = metrics !== undefined && parent?.mode !== "native"
    ? resolveTextMetrics(metrics, flattened(props.style), props, "text", parent?.mode === "manual" ? parent.original : undefined)
    : null;
  const manual = resolved !== null && metrics !== undefined && convertibleChildren(props.children, metrics, resolved.original);
  const paragraph: Paragraph = manual ? { mode: "manual", original: resolved.original } : NATIVE;
  const overrides = manual ? { style: [props.style, resolved.style], allowFontScaling: false } : {};
  // Android can retain a stale glyph run after a hot metrics change. Keep this
  // opt-in limited to a standalone nonselectable leaf; never replace paragraphs.
  const metricKey = invalidateOnMetricsChange && parent === undefined && !props.selectable && manual &&
    (typeof props.children === "string" || typeof props.children === "number")
    ? JSON.stringify([resolved.style.fontSize, resolved.style.lineHeight, resolved.style.letterSpacing])
    : undefined;
  return (
    <ParagraphContext.Provider value={paragraph}>
      <RNText {...nativeProps} {...overrides} key={metricKey} ref={ref} />
    </ParagraphContext.Provider>
  );
});

/** Effective standalone label metrics for an opt-in measurement key; no native reads. */
export function useNativeStandaloneTextMetrics(style: StyleProp<TextStyle>) {
  const metrics = useContext(MetricsContext);
  return metrics === undefined ? null : resolveTextMetrics(metrics, flattened(style), {}, "text")?.style ?? null;
}

/** Shared with the ruled input's geometry; this hook never reads native state. */
export function useNativeTextInputMetrics(style: StyleProp<TextStyle>, props: ScalingProps) {
  const metrics = useContext(MetricsContext);
  // Fabric and native TextInput attributes cap line height differently. Keep
  // capped/natural/unknown inputs on their original path rather than guess.
  if (metrics === undefined || (props.maxFontSizeMultiplier ?? 0) >= 1) return null;
  const finalStyle = flattened(style);
  const resolved = resolveTextMetrics(metrics, finalStyle, props, "input");
  if (resolved === null || resolved.style.lineHeight === undefined) return null;
  const linePitch = Math.ceil(resolved.style.lineHeight * metrics.density) / metrics.density;
  const { fontFamily, fontWeight, fontStyle, fontVariant, textTransform } = finalStyle;
  const placeholderStyle: TextStyle = { fontFamily, fontWeight, fontStyle, fontVariant, textTransform, ...resolved.style };
  return { ...resolved, linePitch, placeholderStyle };
}

export type TextInput = RNTextInput;
export const TextInput = forwardRef<RNTextInput, TextInputProps>(function NativeMetricTextInput(props, ref) {
  const resolved = useNativeTextInputMetrics(props.style, props);
  const overrides = resolved === null ? {} : { style: [props.style, resolved.style], allowFontScaling: false };
  return <RNTextInput {...props} {...overrides} ref={ref} />;
});
