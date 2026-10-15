import { NavigationContext } from "expo-router/build/react-navigation/core/NavigationContext";
import { useContext, useEffect, useState } from "react";
import { AppState } from "react-native";

/** Visual work stops when its route blurs or the app leaves the foreground. */
export function useScreenActive(focused = true): boolean {
  const navigation = useContext(NavigationContext);
  const [routeFocused, setRouteFocused] = useState(() => navigation?.isFocused() ?? true);
  const [foreground, setForeground] = useState(() => AppState.currentState !== "background" && AppState.currentState !== "inactive");
  useEffect(() => {
    setRouteFocused(navigation?.isFocused() ?? true);
    const focus = navigation?.addListener("focus", () => setRouteFocused(true));
    const blur = navigation?.addListener("blur", () => setRouteFocused(false));
    return () => { focus?.(); blur?.(); };
  }, [navigation]);
  useEffect(() => {
    const subscription = AppState.addEventListener("change", state => setForeground(state === "active"));
    return () => subscription.remove();
  }, []);
  return focused && routeFocused && foreground;
}
