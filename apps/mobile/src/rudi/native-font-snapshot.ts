/** Shared data only: UI compilation must not pull in an Expo native module. */
export type FontScaleUnavailableReason =
  | "unsupported-platform"
  | "module-unavailable"
  | "module-inactive"
  | "context-unavailable"
  | "observer-unavailable"
  | "invalid-native-value"
  | "read-failed"
  | "subscription-failed";

export interface NativeFontMetrics {
  density: number;
  spToDp: readonly { base: number; dp: number }[];
}

export type FontScaleSnapshot =
  | { status: "available"; fontScale: number; fontMetrics?: NativeFontMetrics }
  | { status: "unavailable"; reason: FontScaleUnavailableReason };
