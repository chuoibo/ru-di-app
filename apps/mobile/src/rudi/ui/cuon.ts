/**
 * The scroll of the screen a paper stage heads (ADR-0037 D1, plan S0.3).
 *
 * `RudiScreen` with a `canh` publishes its scroll offset here as a shared
 * value, so the stage can fold flat as the list moves and the compact bar can
 * take the title once the big one has gone under it -- both on the UI thread,
 * neither re-rendering the screen. Outside such a screen the value is absent
 * and a stage simply stands.
 */
import { createContext, useContext } from "react";
import type { SharedValue } from "react-native-reanimated";

export interface CuonManHinh {
  /** Vertical scroll offset of the screen's list (dp). */
  cuonY: SharedValue<number>;
  /**
   * Where the stage's big title ends, in the list's own coordinates: once the
   * list has scrolled past it, the compact bar shows the title. Written by the
   * stage on layout; very large until then, so the bar starts empty.
   */
  nguongTieuDe: SharedValue<number>;
}

export const CuonContext = createContext<CuonManHinh | null>(null);

export function useCuonManHinh(): CuonManHinh | null {
  return useContext(CuonContext);
}
