/**
 * The close contract of a tray drawn inside a screen (not a `Sheet`): Escape
 * and the browser's Back on the web, the system Back on Android, and focus
 * handed back to whatever opened it. The chat's tools tray had Escape and
 * Android Back by hand; the shared-plan tray beside it had none, so Escape did
 * nothing there (QA UI-066), and on the web Back from either left the chat
 * (UI-038).
 */
import { useEffect, useRef } from "react";
import { BackHandler, Platform } from "react-native";

import { dangKyLuiWeb } from "./lui-web";

export function useDongKhay(mo: boolean, dong: () => void): void {
  const dongRef = useRef(dong);
  dongRef.current = dong;
  useEffect(() => {
    if (!mo) return;
    if (Platform.OS === "android") {
      const sub = BackHandler.addEventListener("hardwareBackPress", () => {
        dongRef.current();
        return true;
      });
      return () => sub.remove();
    }
    if (Platform.OS !== "web" || typeof document === "undefined") return;
    const nguoiMo = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      dongRef.current();
    };
    document.addEventListener("keydown", onKey, true);
    const boLui = dangKyLuiWeb(() => dongRef.current());
    return () => {
      document.removeEventListener("keydown", onKey, true);
      boLui();
      if (nguoiMo?.isConnected) nguoiMo.focus({ preventScroll: true });
    };
  }, [mo]);
}
