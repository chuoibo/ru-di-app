/**
 * Motion tokens for the RuDi shell, read from `packages/shared/tokens.json`.
 *
 * The previous `motion` block (press/fade/settle) was declared and never read:
 * every animation in the shell was a `pressed` opacity or a navigator default,
 * and the direction contract promised "motion <= 220ms" against numbers no
 * code consulted. This module is the single place durations and easings are
 * named, so a screen never types `220` on its own.
 *
 * Four steps, four jobs:
 *   instant   press and chip feedback, paired with a haptic
 *   standard  state and content transitions, accordions, skeleton -> content
 *   shared    card -> detail, plan -> timeline, photo -> viewer
 *   celebrate one-shot moments only (outing locked, bill done, badge opened)
 *
 * Two rules the tests pin: Reduce Motion collapses every step but `instant`
 * to zero, and a money count-up may only start once the domain state is
 * valid -- animation never stands in for data on a money screen.
 *
 * Pure: no React, no Reanimated. `ui/useMotion.ts` maps these to Reanimated on
 * the UI thread and reads the system Reduce Motion setting.
 */
import tokens from "../../../../packages/shared/tokens.json";

export type MotionStep = "instant" | "standard" | "shared" | "celebrate";
export type EasingName = "standard" | "decelerate" | "accelerate";
/** cubic-bezier(x1, y1, x2, y2) */
export type Bezier = readonly [number, number, number, number];

const spec = tokens.motion;

export const MOTION_MS: Readonly<Record<MotionStep, number>> = Object.freeze({
  instant: spec.instant,
  standard: spec.standard,
  shared: spec.shared,
  celebrate: spec.celebrate,
});

export const EASING: Readonly<Record<EasingName, Bezier>> = Object.freeze({
  standard: spec.easing.standard as unknown as Bezier,
  decelerate: spec.easing.decelerate as unknown as Bezier,
  accelerate: spec.easing.accelerate as unknown as Bezier,
});

/**
 * Navigator transitions the shell uses. `"none"` is the instant cut the OS
 * «Remove animations» setting asks for.
 */
export type StackAnimation = "slide_from_right" | "slide_from_bottom" | "fade" | "none";

/**
 * The stack animation to actually run, honouring Reduce Motion.
 *
 * The three OS animation scales at 0 do NOT stop a react-native-screens push
 * on Android: the stack animates its Fragments with `android.view.animation`,
 * which no scale touches, so a screen kept sliding in with every scale read
 * back as 0 (Codex re-audit 10/09, R1). The app has to ask for the cut itself,
 * and it asks here, from the same `reduceMotion` bit every other duration uses.
 */
export function stackAnimation(wanted: Exclude<StackAnimation, "none">, reduceMotion: boolean): StackAnimation {
  return reduceMotion ? "none" : wanted;
}

/** The duration to actually run, honouring Reduce Motion. */
export function durationFor(step: MotionStep, reduceMotion: boolean): number {
  if (reduceMotion && step !== "instant") return 0;
  return MOTION_MS[step];
}

/**
 * How long a money figure may count up: `standard` once the domain state is
 * valid, otherwise nothing. A number that animates before the server has
 * confirmed it is a number the screen invented for a few hundred milliseconds.
 */
export function moneyCountUpMs(domainStateValid: boolean, reduceMotion: boolean): number {
  if (!domainStateValid) return 0;
  return durationFor("standard", reduceMotion);
}

/**
 * Composite budgets of the paper stage (ADR-0037 D3). Not a fifth step:
 * `MOTION_MS` stays exactly four. A pop-up is `shared` plus a 40 ms stagger per
 * layer, capped at four staggered layers; a Nếp performance never runs past
 * `dien` and never holds input; a page turn is `shared`.
 */
export const NGAN_SACH_SAN_KHAU: Readonly<{ batTang: number; batToiDa: number; dien: number; lat: number }> = Object.freeze({
  batTang: spec.sanKhau.batTang,
  batToiDa: spec.sanKhau.batToiDa,
  dien: spec.sanKhau.dien,
  lat: spec.sanKhau.lat,
});

/** How far apart layer `i` starts, in ms; layers past the fourth start with the fourth. */
export function treTang(i: number, reduceMotion: boolean): number {
  if (reduceMotion) return 0;
  return NGAN_SACH_SAN_KHAU.batTang * Math.max(0, Math.min(3, Math.floor(i)));
}

/** The whole pop-up for `soTang` layers, never past `batToiDa`; zero under Reduce Motion. */
export function batToiDa(soTang: number, reduceMotion: boolean): number {
  if (reduceMotion) return 0;
  const tong = MOTION_MS.shared + treTang(Math.max(1, soTang) - 1, false);
  return Math.min(NGAN_SACH_SAN_KHAU.batToiDa, tong);
}

/**
 * `celebrate` is a budget of one per event, not a style. The first call for a
 * key wins; every later call for the same key is an ordinary `standard`
 * transition. Callers keep the `seen` set for the lifetime of the screen.
 */
export function celebrateOnce(seen: Set<string>, eventKey: string, reduceMotion: boolean): number {
  if (seen.has(eventKey)) return durationFor("standard", reduceMotion);
  seen.add(eventKey);
  return durationFor("celebrate", reduceMotion);
}
