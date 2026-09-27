/**
 * Loading CanvasKit on the web, once, without ever blocking a first paint
 * (ADR-0037 D10).
 *
 * `@shopify/react-native-skia` builds its web API from `global.CanvasKit` the
 * moment `Skia.web.js` is evaluated, so on the web nothing that imports the
 * library may run before CanvasKit has loaded. This module is the only place
 * that decides when that is. Everything drawn with Skia is reached through a
 * dynamic `import()` behind `napSkia()` (see `ui/KhungSkia.tsx`); until then --
 * and forever, when the browser has no WebGL or the load fails -- the same
 * geometry is drawn by react-native-svg.
 *
 * A browser without WebGL never downloads the 8 MB wasm at all: the probe runs
 * first. `EXPO_PUBLIC_QA_TAT_SKIA=1` forces the SVG path for evidence runs (the
 * canary of `tools/xem-san-khau.mjs` must then report `svg`). The variable is
 * read as a plain `process.env.X` member on purpose (`tests/env-inlining.test.mjs`).
 */
import { useSyncExternalStore } from "react";

declare const process: { env: Record<string, string | undefined> };

export type TrangThaiSkia = "chua" | "dang-nap" | "san-sang" | "loi";

const TAT_SKIA_QA: boolean = process.env.EXPO_PUBLIC_QA_TAT_SKIA === "1";

let trangThai: TrangThaiSkia = "chua";
const nguoiNghe = new Set<() => void>();
let lanNap: Promise<boolean> | null = null;

function dat(moi: TrangThaiSkia) {
  trangThai = moi;
  for (const nghe of nguoiNghe) nghe();
}

/** Whether this browser can give CanvasKit a WebGL context at all. */
export function coWebGL(): boolean {
  if (typeof document === "undefined") return false;
  try {
    const canvas = document.createElement("canvas");
    return Boolean(canvas.getContext("webgl2") ?? canvas.getContext("webgl"));
  } catch {
    return false;
  }
}

/** Start loading (idempotent); resolves `true` once Skia can draw, `false` if it never will. */
export function napSkia(): Promise<boolean> {
  if (lanNap !== null) return lanNap;
  if (TAT_SKIA_QA || !coWebGL()) {
    dat("loi");
    lanNap = Promise.resolve(false);
    return lanNap;
  }
  dat("dang-nap");
  lanNap = import("@shopify/react-native-skia/lib/module/web/LoadSkiaWeb")
    .then(({ LoadSkiaWeb }) => LoadSkiaWeb({ locateFile: (tep: string) => `/${tep}` }))
    .then(
      () => {
        dat("san-sang");
        return true;
      },
      () => {
        dat("loi");
        return false;
      },
    );
  return lanNap;
}

export function trangThaiSkia(): TrangThaiSkia {
  return trangThai;
}

function dangKy(nghe: () => void) {
  nguoiNghe.add(nghe);
  return () => {
    nguoiNghe.delete(nghe);
  };
}

/** The load state, for a wrapper that shows SVG until Skia is ready. */
export function useTrangThaiSkia(): TrangThaiSkia {
  return useSyncExternalStore(dangKy, trangThaiSkia, trangThaiSkia);
}
