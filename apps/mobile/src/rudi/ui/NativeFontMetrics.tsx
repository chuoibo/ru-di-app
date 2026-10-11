import { Children, Fragment, createContext, forwardRef, isValidElement, useContext, type ReactNode } from "react";
import {
  StyleSheet, Text as RNText, TextInput as RNTextInput,
  type StyleProp, type TextProps, type TextInputProps, type TextStyle,
} from "react-native";
import type { FontScaleSnapshot, NativeFontMetrics } from "../../../modules/rudi-font-scale";
import { resolveTextMetrics, type OriginalTextMetrics, type ScalingProps } from "../native-font-metrics";

const MetricsContext = createContext<NativeFontMetrics | undefined>(undefined);
type Paragraph = { mode: "native" } | { mode: "manual"; original: OriginalTextMetrics };
const ParagraphContext = createContext<Paragraph | undefined>(undefined);
const NATIVE: Paragraph = { mode: "native" };

/** Always mounted above the bill, including its footer; no global RN defaults. */
export function NativeFontMetricsProvider({ snapshot, children }: { snapshot: FontScaleSnapshot; children: ReactNode }) {
  const metrics = snapshot.status === "available" ? snapshot.fontMetrics : undefined;
  return <MetricsContext.Provider value={metrics}>{children}</MetricsContext.Provider>;
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
export const Text = forwardRef<RNText, TextProps>(function NativeMetricText(props, ref) {
  const metrics = useContext(MetricsContext);
  const parent = useContext(ParagraphContext);
  const resolved = metrics !== undefined && parent?.mode !== "native"
    ? resolveTextMetrics(metrics, flattened(props.style), props, "text", parent?.mode === "manual" ? parent.original : undefined)
    : null;
  const manual = resolved !== null && metrics !== undefined && convertibleChildren(props.children, metrics, resolved.original);
  const paragraph: Paragraph = manual ? { mode: "manual", original: resolved.original } : NATIVE;
  const overrides = manual ? { style: [props.style, resolved.style], allowFontScaling: false } : {};
  return (
    <ParagraphContext.Provider value={paragraph}>
      <RNText {...props} {...overrides} ref={ref} />
    </ParagraphContext.Provider>
  );
});

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
