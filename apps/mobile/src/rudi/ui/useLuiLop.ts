/**
 * Back closes a layer a screen draws itself, not the screen (QA UI-108,
 * UI-110): a panel of Cá nhân, the second step of deleting an account. On the
 * web through `dangKyLuiWeb` (the browser's Back, before the router sees
 * it); on Android through the hardware Back. Registered while `mo` is true.
 */
import { useEffect, useRef } from "react";
import { BackHandler } from "react-native";

import { dangKyLuiWeb } from "./lui-web";

export function useLuiLop(mo: boolean, dong: () => void): void {
  const dongRef = useRef(dong);
  dongRef.current = dong;
  useEffect(() => {
    if (!mo) return;
    const boWeb = dangKyLuiWeb(() => dongRef.current());
    const sub = BackHandler.addEventListener("hardwareBackPress", () => {
      dongRef.current();
      return true;
    });
    return () => {
      boWeb();
      sub.remove();
    };
  }, [mo]);
}
