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

export type FontScaleSnapshot =
  | { status: "available"; fontScale: number }
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
      return typeof scale === "number" && Number.isFinite(scale) && scale > 0
        ? { status: "available", fontScale: scale }
        : { status: "unavailable", reason: "invalid-native-value" };
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
