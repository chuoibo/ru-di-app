/**
 * The clock of «nét mực tự vẽ» (see `veDenDau` in kieu-ban-do.ts).
 *
 * The ink runs once per real route: when Valhalla's answer first arrives for a
 * day, or when the suggestion replaces the current route -- the two moments a
 * person needs to SEE the day, not just read it. Opening the map again on a
 * route already drawn does not replay it (no ceremony on the hundredth open).
 * Reduce Motion: the final frame at once. Content is never hidden: before the
 * first frame the stamps are on the map, only the ink is still to come.
 */

import { useEffect, useRef, useState } from "react";

import { useMotion } from "../ui/useMotion";
import { nhipVe } from "./kieu-ban-do";

/** Inside DESIGN.md's performance ceiling (≤ 1400ms) with room for the last stamp. */
export const THOI_GIAN_VE_MS = 1200;

/** Routes already drawn this session, so a remount does not replay the ink. */
const daVe = new Set<string>();

/** Distinct drawing steps over the whole stroke. */
const BUOC_VE = 24;

/** After the map is ready: the camera's fit (≈300ms) settles before the pen sets down. */
export const TRE_VE_MS = 320;

/**
 * Drawing progress 0..1 for the route identified by `khoa`; null `khoa` (a
 * draft, or no route yet) is always 1 -- a pencil line is not performed.
 *
 * `san`: the map has loaded. Until then a new route waits at 0: the preview
 * often answers before the tiles, and the whole performance ran on a blank
 * map -- the line appeared finished with the first tiles (emulator, 2026-09-29).
 */
export function useNetMuc(khoa: string | null, san = true): number {
  const motion = useMotion();
  const [tien, setTien] = useState<{ khoa: string | null; t: number }>({ khoa: null, t: 1 });
  const frame = useRef<number | null>(null);
  // Decided during render, not after it: a new route must start at 0 on its
  // very first frame. Waiting for the effect let the whole line flash for one
  // frame before it was cut back and redrawn (emulator log, 2026-09-29).
  const moi = khoa !== null && !motion.reduced && !daVe.has(khoa) && tien.khoa !== khoa;
  useEffect(() => {
    if (khoa === null || motion.reduced || daVe.has(khoa)) {
      setTien({ khoa, t: 1 });
      return;
    }
    if (!san) return;
    daVe.add(khoa);
    const batDau = Date.now() + TRE_VE_MS;
    setTien({ khoa, t: 0 });
    let buocTruoc = -1;
    const buoc = () => {
      const x = Math.max(0, Date.now() - batDau) / THOI_GIAN_VE_MS;
      // ≈20 steps a second, not one per frame: each step re-sends the route to
      // the native map, and on a slow GPU 60 a second queued up until the
      // line appeared whole at the end (emulator, 2026-09-29).
      const n = x >= 1 ? BUOC_VE : Math.floor(x * BUOC_VE);
      if (n !== buocTruoc) { buocTruoc = n; setTien({ khoa, t: n >= BUOC_VE ? 1 : nhipVe(n / BUOC_VE) }); }
      if (x < 1) frame.current = requestAnimationFrame(buoc);
    };
    frame.current = requestAnimationFrame(buoc);
    return () => {
      if (frame.current !== null) cancelAnimationFrame(frame.current);
      setTien({ khoa, t: 1 });
    };
  }, [khoa, motion.reduced, san]);
  if (moi) return 0;
  return tien.khoa === khoa ? tien.t : 1;
}
