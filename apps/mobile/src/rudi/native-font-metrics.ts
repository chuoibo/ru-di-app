import type { TextStyle } from "react-native";
import type { NativeFontMetrics } from "../../modules/rudi-font-scale";

export interface OriginalTextMetrics {
  fontSize: number;
  lineHeight?: number;
  letterSpacing?: number;
  allowFontScaling: boolean;
  maxFontSizeMultiplier: number;
}
export interface ScalingProps {
  allowFontScaling?: boolean;
  maxFontSizeMultiplier?: number | null;
}
export interface ResolvedTextMetrics {
  original: OriginalTextMetrics;
  style: Pick<TextStyle, "fontSize" | "lineHeight" | "letterSpacing">;
}

/** Resolve original SP values once; inherited values never contain converted DP. */
export function resolveTextMetrics(
  metrics: NativeFontMetrics,
  style: TextStyle,
  props: ScalingProps,
  kind: "text" | "input",
  parent?: OriginalTextMetrics,
): ResolvedTextMetrics | null {
  const fontSize = style.fontSize ?? parent?.fontSize ?? 14;
  const lineHeight = style.lineHeight ?? parent?.lineHeight;
  const letterSpacing = style.letterSpacing ?? parent?.letterSpacing;
  const max = props.maxFontSizeMultiplier ?? parent?.maxFontSizeMultiplier ?? 0;
  if (!Number.isFinite(fontSize) || fontSize <= 0 ||
      (lineHeight !== undefined && (!Number.isFinite(lineHeight) || lineHeight <= 0)) ||
      (letterSpacing !== undefined && !Number.isFinite(letterSpacing)) ||
      !Number.isFinite(max) || (max !== 0 && max < 1)) return null;
  const original: OriginalTextMetrics = {
    fontSize, lineHeight, letterSpacing,
    allowFontScaling: parent?.allowFontScaling ?? props.allowFontScaling !== false,
    maxFontSizeMultiplier: max,
  };
  const convert = (base: number | undefined, capped: boolean): number | undefined | null => {
    if (base === undefined) return undefined;
    if (!original.allowFontScaling || base === 0) return base;
    const dp = metrics.spToDp.find((row) => row.base === base)?.dp;
    if (dp === undefined) return null;
    // RN Text caps font size; TextInput's TextAttributes caps all three metrics.
    return capped && max >= 1 ? Math.min(dp, base * max) : dp;
  };
  const size = convert(fontSize, true);
  const height = convert(lineHeight, kind === "input");
  const tracking = convert(letterSpacing, kind === "input");
  if (size === null || height === null || tracking === null) return null;
  return { original, style: { fontSize: size, lineHeight: height, letterSpacing: tracking } };
}
