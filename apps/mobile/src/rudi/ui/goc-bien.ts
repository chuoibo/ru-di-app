/**
 * A pixel transform origin React Native can parse.
 *
 * Its parser (`processTransformOrigin`) only reads whole `<n>px` tokens: a
 * fractional coordinate such as `196.0909px` splits into several tokens, and
 * on Android more than three is a render error («Transform origin must have
 * exactly 3 values»). Measured 2026-10-05 on a 393dp-wide emulator: opening a
 * place drew the red box instead of the screen, because the stage behind the
 * place is half of a fractional width. Half a pixel of pivot is invisible.
 */
export function gocBien(x: number, y: number): string {
  return `${Math.round(x)}px ${Math.round(y)}px`;
}
