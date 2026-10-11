import { requireOptionalNativeModule, type EventSubscription } from "expo-modules-core";
import { Platform } from "react-native";

export type FontScaleUnavailableReason =
  | "unsupported-platform"
  | "module-unavailable"
  | "module-inactive"
  | "context-unavailable"
  | "observer-unavailable"
  | "invalid-native-value"
  | "read-failed"
  | "subscription-failed";

/** The complete numeric SP vocabulary of the currently scoped bill UI. */
export const BILL_SP_BASES = [-0.65, -0.3, -0.15, 0.4, 0.8, 1.6,
  11, 12, 13, 14, 15, 17, 18, 19, 21, 22, 23, 24, 27, 28, 34] as const;

export interface NativeFontMetrics {
  density: number;
  spToDp: readonly { base: number; dp: number }[];
}

/** An old binary or malformed optional table keeps the normal RN text path. */
export function readNativeFontMetrics(value: unknown): NativeFontMetrics | undefined {
  if (typeof value !== "object" || value === null) return undefined;
  const table = value as Record<string, unknown>;
  if (typeof table.density !== "number" || !Number.isFinite(table.density) || table.density <= 0 ||
      !Array.isArray(table.spToDp) || table.spToDp.length !== BILL_SP_BASES.length) return undefined;
  const rows: { base: number; dp: number }[] = [];
  for (const base of BILL_SP_BASES) {
    const matches = table.spToDp.filter((row: unknown) => typeof row === "object" && row !== null &&
      (row as Record<string, unknown>).base === base);
    if (matches.length !== 1) return undefined;
    const dp: unknown = (matches[0] as Record<string, unknown>).dp;
    if (typeof dp !== "number" || !Number.isFinite(dp) || (base > 0 && dp <= 0) || (base < 0 && dp >= 0)) return undefined;
    rows.push({ base, dp });
  }
  return { density: table.density, spToDp: rows };
}

export function sameNativeFontMetrics(a: NativeFontMetrics | undefined, b: NativeFontMetrics | undefined): boolean {
  return a === b || (a !== undefined && b !== undefined && Object.is(a.density, b.density) &&
    a.spToDp.length === b.spToDp.length && a.spToDp.every((row, i) =>
      Object.is(row.base, b.spToDp[i]?.base) && Object.is(row.dp, b.spToDp[i]?.dp)));
}

export type FontScaleSnapshot =
  | { status: "available"; fontScale: number; fontMetrics?: NativeFontMetrics }
  | { status: "unavailable"; reason: FontScaleUnavailableReason };

type FontScaleNative = {
  getFontScale(): unknown;
  addListener(eventName: "onFontScaleChanged", listener: () => void): EventSubscription;
};

const nativeModule = (() => {
  if (Platform.OS !== "android") return null;
  try {
    return requireOptionalNativeModule<FontScaleNative>("RudiFontScale");
  } catch {
    return null;
  }
})();

function missingModule(): FontScaleSnapshot {
  return {
    status: "unavailable",
    reason: Platform.OS === "android" ? "module-unavailable" : "unsupported-platform",
  };
}

/** Read current Android resources; never invent a scale when unavailable. */
export function readFontScale(): FontScaleSnapshot {
  if (nativeModule === null) return missingModule();
  try {
    const value: unknown = nativeModule.getFontScale();
    if (typeof value !== "object" || value === null) {
      return { status: "unavailable", reason: "invalid-native-value" };
    }
    const snapshot = value as Record<string, unknown>;
    if (snapshot.status === "available") {
      const scale = snapshot.fontScale;
      if (typeof scale !== "number" || !Number.isFinite(scale) || scale <= 0) {
        return { status: "unavailable", reason: "invalid-native-value" };
      }
      const fontMetrics = readNativeFontMetrics(snapshot.fontMetrics);
      return fontMetrics === undefined
        ? { status: "available", fontScale: scale }
        : { status: "available", fontScale: scale, fontMetrics };
    }
    if (snapshot.status === "unavailable") {
      switch (snapshot.reason) {
        case "module-inactive":
        case "context-unavailable":
        case "observer-unavailable":
        case "invalid-native-value":
        case "read-failed":
          return { status: "unavailable", reason: snapshot.reason };
      }
    }
    return { status: "unavailable", reason: "invalid-native-value" };
  } catch {
    return { status: "unavailable", reason: "read-failed" };
  }
}

export type FontScaleObservation =
  | { status: "available"; remove(): void }
  | { status: "unavailable"; reason: FontScaleUnavailableReason };

/** Events carry no scale, so queued notifications cannot apply old values. */
export function observeFontScale(onChange: () => void): FontScaleObservation {
  if (nativeModule === null) {
    const missing = missingModule();
    return { status: "unavailable", reason: missing.status === "unavailable" ? missing.reason : "module-unavailable" };
  }
  try {
    const subscription = nativeModule.addListener("onFontScaleChanged", onChange);
    if (!subscription || typeof subscription.remove !== "function") {
      return { status: "unavailable", reason: "subscription-failed" };
    }
    return { status: "available", remove: () => subscription.remove() };
  } catch {
    return { status: "unavailable", reason: "subscription-failed" };
  }
}
