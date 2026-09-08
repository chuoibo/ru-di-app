/**
 * Size classes for the RuDi shell, one contract for every screen.
 *
 * Before this module the shell had a single breakpoint, `700`, typed by hand in
 * four places (`ui.tsx`, the tab layout, a fixture hero). Each place meant
 * something slightly different by it, and none of them knew about height, so a
 * phone turned sideways was laid out like a tablet and a tablet in split-screen
 * like a phone. The classes below follow the Android window size classes
 * (compact < 600dp, medium 600–839dp, expanded ≥ 840dp) and are computed from
 * the *current window*, never from the device, so rotation and split-screen
 * re-classify on the fly.
 *
 * Pure: `dict` in, `dict` out, no React. `ui/useAdaptiveLayout.ts` is the hook
 * that feeds it the window; this file is compiled for the node tests so the
 * boundaries are measured, not eyeballed.
 */

export type SizeClass = "compact" | "medium" | "expanded";
export type HeightClass = "short" | "regular";

/** Android window size class boundaries, in dp. */
export const SIZE_CLASS_BREAKPOINTS = Object.freeze({
  medium: 600,
  expanded: 840,
});

/** Below this height (dp) a phone is on its side or a sheet is fighting the IME. */
export const SHORT_HEIGHT = 480;

/** Measure the actual content box, after rail and gutters, not the display. */
/**
 * `maxColumns` defaults to 3, the most a row of cards can carry; a wall of
 * album tiles passes more so a tablet shows six thumbnails, not three posters.
 */
export function gridFor(contentWidth: number, minItemWidth = 250, gap = 12, maxColumns = 3) {
  const width = Number.isFinite(contentWidth) ? Math.max(0, contentWidth) : 0;
  const minimum = Number.isFinite(minItemWidth) ? Math.max(1, minItemWidth) : 250;
  const spacing = Number.isFinite(gap) ? Math.max(0, gap) : 12;
  const cap = Number.isFinite(maxColumns) ? Math.max(1, Math.floor(maxColumns)) : 3;
  const columns = Math.max(1, Math.min(cap, Math.floor((width + spacing) / (minimum + spacing))));
  // Whole dp: three exact thirds round up to a pixel each on the device and the
  // last column wraps (album grid, 2026-09-06). A dp of slack per row costs nothing.
  return { columns, itemWidth: Math.max(0, Math.floor((width - spacing * (columns - 1)) / columns)) };
}

export interface AdaptiveLayout {
  sizeClass: SizeClass;
  heightClass: HeightClass;
  /** Media/grid columns a screen may use for cards and album tiles. */
  columns: 1 | 2 | 3;
  /** Horizontal screen gutter (dp), on the 4pt scale of tokens.json. */
  gutter: 16 | 24 | 36;
  /** Navigation is a left rail instead of a bottom bar. */
  rail: boolean;
  /** A list may show its detail beside it instead of pushing a new screen. */
  twoPane: boolean;
  /** Widest measure a single column of content may take (dp). */
  maxContent: number;
}

export function sizeClassFor(width: number): SizeClass {
  if (!Number.isFinite(width) || width < SIZE_CLASS_BREAKPOINTS.medium) return "compact";
  if (width < SIZE_CLASS_BREAKPOINTS.expanded) return "medium";
  return "expanded";
}

export function heightClassFor(height: number): HeightClass {
  return Number.isFinite(height) && height < SHORT_HEIGHT ? "short" : "regular";
}

/** The whole layout contract for one window size. */
export function layoutFor(width: number, height: number): AdaptiveLayout {
  const sizeClass = sizeClassFor(width);
  const heightClass = heightClassFor(height);
  switch (sizeClass) {
    case "expanded":
      return {
        sizeClass,
        heightClass,
        columns: 3,
        gutter: 36,
        rail: true,
        twoPane: true,
        maxContent: 1200,
      };
    case "medium":
      return {
        sizeClass,
        heightClass,
        columns: 2,
        gutter: 24,
        rail: true,
        // A short medium window (a tablet in landscape split, a foldable half
        // open) has no room for a detail pane beside a list.
        twoPane: heightClass === "regular",
        maxContent: 960,
      };
    default:
      return {
        sizeClass,
        heightClass,
        columns: 1,
        gutter: 16,
        rail: false,
        twoPane: false,
        maxContent: width,
      };
  }
}

/** The tab bar's height at font scale 1.0, the four destinations' labels on one line. */
export const TAB_BAR_HEIGHT = 64;

/**
 * The tab bar's height at a given font scale. Labels are allowed two lines
 * rather than an ellipsis («Khám …», «Lên pl…» measured at 2.0 by the review
 * of 08/09), so the bar grows with the text instead of locking the text to
 * fit the bar. Linear above 1.15, clamped at 2.0; screens under the tabs read
 * the same number through `RudiScreen bottomInset="tab"`.
 */
export function tabBarHeight(fontScale: number): number {
  const scale = Number.isFinite(fontScale) ? Math.min(Math.max(fontScale, 1), 2) : 1;
  return TAB_BAR_HEIGHT + Math.round(Math.max(0, scale - 1.15) * 44);
}
