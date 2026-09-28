/**
 * The art layer drawn with Skia: the same `LopVe[]` that `ui/art/VeLop.tsx`
 * draws with react-native-svg, the same colour roles, the same stroke rules.
 * Nothing about the geometry is Skia's own -- it stays in the pure builders of
 * `src/rudi/art/*.ts`, which is what keeps the node tests (Java-style path
 * parse, bounds, the fold-ink sweep, the sha256 lock of Nếp) valid for both
 * renderers.
 *
 * Paths are parsed once: `Skia.Path.MakeFromSVGString` is a C++ parse per call,
 * and a stage re-renders on every frame of its pop-up. The cache is keyed by the
 * `d` string (the builders are deterministic, so the same shape is the same
 * string) and bounded, dropping the oldest entry first.
 *
 * Only reachable through a dynamic import (see `ui/KhungSkia.tsx`).
 */
import { Group, Path, Skia, StrokeCap, StrokeJoin, type SkPath } from "@shopify/react-native-skia";
import type { SharedValue } from "react-native-reanimated";

import type { LopVe, MauVe } from "../../art/net";
import type { RudiPalette } from "../../theme";
import { mauLop } from "../art/VeLop";

const TRAN_CACHE = 500;
const cacheDuong = new Map<string, SkPath>();

export function duongSkia(d: string): SkPath {
  const co = cacheDuong.get(d);
  if (co) {
    cacheDuong.delete(d);
    cacheDuong.set(d, co);
    return co;
  }
  const moi = Skia.Path.MakeFromSVGString(d) ?? Skia.Path.Make();
  cacheDuong.set(d, moi);
  if (cacheDuong.size > TRAN_CACHE) {
    const cuNhat = cacheDuong.keys().next().value;
    if (cuNhat !== undefined) cacheDuong.delete(cuNhat);
  }
  return moi;
}

const cacheBong = new Map<string, SkPath>();

/**
 * The silhouette of a layer list as ONE path: every fill, and every stroke
 * turned into its outline at its own width, round-capped as it is drawn. A
 * cut-paper layer casts its shadow with this -- one blurred draw per layer
 * instead of one per stroke. Cached like the paths it is built from.
 */
export function hinhBongLop(lop: readonly LopVe[]): SkPath {
  const khoa = lop.map((l) => `${l.net ?? 0}|${l.d}`).join("\n");
  const co = cacheBong.get(khoa);
  if (co) return co;
  const bong = Skia.Path.Make();
  for (const l of lop) {
    // The cached path is shared: outline a copy, never the original.
    const p = duongSkia(l.d).copy();
    if (l.net && l.net > 0) {
      const vien = p.stroke({ width: l.net, cap: StrokeCap.Round, join: StrokeJoin.Round });
      if (vien) bong.addPath(vien);
    } else {
      bong.addPath(p);
    }
  }
  cacheBong.set(khoa, bong);
  if (cacheBong.size > 120) {
    const cuNhat = cacheBong.keys().next().value;
    if (cuNhat !== undefined) cacheBong.delete(cuNhat);
  }
  return bong;
}

/**
 * One layer list as Skia paths. `veToi` (0..1) trims every STROKE from its
 * start -- the ink drawing itself on; fills are shown whole, since a fill has no
 * direction to draw along (ADR-0037 D3: decorative only, text never waits).
 */
export function LopSkia({
  lop,
  colors,
  doiMau,
  veToi,
  opacity,
}: {
  lop: readonly LopVe[];
  colors: RudiPalette;
  doiMau?: Partial<Record<MauVe, string>>;
  veToi?: SharedValue<number> | number;
  opacity?: SharedValue<number> | number;
}) {
  return (
    <Group opacity={opacity ?? 1}>
      {lop.map((l, i) => {
        const mau = doiMau?.[l.mau] ?? mauLop(colors, l.mau);
        const duong = duongSkia(l.d);
        if (l.net && l.net > 0) {
          return (
            <Path
              color={mau}
              end={veToi ?? 1}
              key={i}
              path={duong}
              start={0}
              strokeCap="round"
              strokeJoin="round"
              strokeWidth={l.net}
              style="stroke"
            />
          );
        }
        return <Path color={mau} key={i} path={duong} style="fill" />;
      })}
    </Group>
  );
}
