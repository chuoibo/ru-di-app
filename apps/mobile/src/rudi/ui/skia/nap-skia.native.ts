/**
 * The native side of `nap-skia.ts`: Skia is compiled into the dev client, so
 * there is nothing to download -- only a question to ask before the first
 * `require`.
 *
 * `@shopify/react-native-skia` installs its JSI bindings when its setup module
 * is evaluated and throws if the native module is missing. A dev client built
 * before this campaign has no Skia; importing the library there would take the
 * whole app down at the first stage. So the module is probed with the
 * non-enforcing `TurboModuleRegistry.get`, and a missing module means «draw in
 * SVG», with one dev warning that the client needs a rebuild -- never a crash.
 */
import { TurboModuleRegistry } from "react-native";

import type { TrangThaiSkia } from "./nap-skia";

export type { TrangThaiSkia } from "./nap-skia";

declare const process: { env: Record<string, string | undefined> };
declare const __DEV__: boolean | undefined;

const TAT_SKIA_QA: boolean = process.env.EXPO_PUBLIC_QA_TAT_SKIA === "1";

let daBao = false;

function coModuleSkia(): boolean {
  if (TAT_SKIA_QA) return false;
  try {
    return TurboModuleRegistry.get("RNSkiaModule") != null;
  } catch {
    return false;
  }
}

const CO_SKIA = coModuleSkia();

function baoMotLan() {
  if (CO_SKIA || daBao || TAT_SKIA_QA) return;
  daBao = true;
  if (typeof __DEV__ === "boolean" && __DEV__) {
    console.warn("Rủ Đi: dev client chưa có Skia (cần dựng lại); sân khấu vẽ bằng SVG.");
  }
}

export function coWebGL(): boolean {
  return false;
}

export function napSkia(): Promise<boolean> {
  baoMotLan();
  return Promise.resolve(CO_SKIA);
}

export function trangThaiSkia(): TrangThaiSkia {
  return CO_SKIA ? "san-sang" : "loi";
}

export function useTrangThaiSkia(): TrangThaiSkia {
  return trangThaiSkia();
}
