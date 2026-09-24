import * as Haptics from "expo-haptics";
import { useEffect, useMemo, useState } from "react";
import { AccessibilityInfo } from "react-native";
import {
  Easing,
  ReduceMotion,
  useReducedMotion,
  type WithSpringConfig,
  type WithTimingConfig,
} from "react-native-reanimated";

import { EASING, NGAN_SACH_SAN_KHAU, batToiDa, durationFor, treTang, type EasingName, type MotionStep } from "../motion";

/**
 * The shell's motion tokens, bound to Reanimated and to the system Reduce
 * Motion setting. Every animated primitive reads this hook; no screen calls
 * `withTiming` with a number of its own.
 */
export interface MotionKit {
  /** The OS asked for less motion; durations other than `instant` are 0. */
  reduced: boolean;
  ms(step: MotionStep): number;
  timing(step: MotionStep, easing?: EasingName): WithTimingConfig;
  spring: { press: WithSpringConfig; settle: WithSpringConfig };
  haptic: { select(): void; impact(): void; success(): void };
  /**
   * The same answer for Reanimated's layout animations (`FadeIn.reduceMotion(...)`):
   * `Always` when reduced, `Never` otherwise -- never `System`, which Reanimated
   * reads once at startup and so misses a setting changed mid-session.
   */
  reanimated: ReduceMotion;
  /**
   * The paper-stage budgets (ADR-0037 D3): a pop-up's per-layer delay and total,
   * both zero under Reduce Motion, and the raw numbers for a performance or a page
   * turn. Composite budgets, not a fifth step.
   */
  sanKhau: {
    treTang(i: number): number;
    batToiDa(soTang: number): number;
    dien: number;
    lat: number;
  };
}

export function useMotion(): MotionKit {
  const initialReduced = useReducedMotion();
  const [reduced, setReduced] = useState(initialReduced);
  useEffect(() => {
    let mounted = true;
    void AccessibilityInfo.isReduceMotionEnabled().then((value) => { if (mounted) setReduced(value); });
    const subscription = AccessibilityInfo.addEventListener("reduceMotionChanged", setReduced);
    return () => { mounted = false; subscription.remove(); };
  }, []);
  return useMemo(() => {
    const ms = (step: MotionStep) => durationFor(step, reduced);
    // Reanimated's `ReduceMotion.System` is read once when the app starts, so a
    // spring kept springing after the setting changed mid-session (re-audit
    // 10/09, R1: the «Tạo mới» sheet still took 7 frames with every scale at 0).
    // The live bit above decides; Reanimated is only told the answer.
    const giam = reduced ? ReduceMotion.Always : ReduceMotion.Never;
    const bezier = (name: EasingName) => {
      const [x1, y1, x2, y2] = EASING[name];
      return Easing.bezier(x1, y1, x2, y2);
    };
    return {
      reduced,
      ms,
      reanimated: giam,
      timing: (step, easing = "standard") => ({
        duration: ms(step),
        easing: bezier(easing),
        reduceMotion: giam,
      }),
      spring: {
        // Press: quick, slightly overdamped, so a tap never wobbles.
        press: { damping: 18, stiffness: 260, mass: 0.6, reduceMotion: giam },
        // Settle: a sheet or a card coming to rest.
        settle: { damping: 20, stiffness: 180, mass: 0.8, reduceMotion: giam },
      },
      sanKhau: {
        treTang: (i: number) => treTang(i, reduced),
        batToiDa: (soTang: number) => batToiDa(soTang, reduced),
        dien: reduced ? 0 : NGAN_SACH_SAN_KHAU.dien,
        lat: reduced ? 0 : NGAN_SACH_SAN_KHAU.lat,
      },
      haptic: {
        select: () => void Haptics.selectionAsync(),
        impact: () => void Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Medium),
        success: () => void Haptics.notificationAsync(Haptics.NotificationFeedbackType.Success),
      },
    };
  }, [reduced]);
}
