import { StatusBar } from "expo-status-bar";
import { useEffect, useRef, type ReactNode } from "react";
import { ActivityIndicator, Platform, StyleSheet, Text, View } from "react-native";

import { useRudiSession } from "../session";
import { useRudiTheme } from "../theme";
import { Wordmark } from "./Wordmark";

/** Available before fonts and credentials, without pretending either is ready. */
export function OpeningApp() {
  const { colors } = useRudiTheme();
  return <View testID="opening-app" accessibilityRole="progressbar" accessibilityLabel="Đang mở Rủ Đi" style={[styles.opening, { backgroundColor: colors.cover }]}>
    <StatusBar style="light" />
    <Wordmark height={48} color={colors.coverInk} />
    <View style={styles.message}>
      <ActivityIndicator color={colors.coverInkSoft} accessible={false} aria-hidden importantForAccessibility="no-hide-descendants" />
      <Text style={[styles.copy, { color: colors.coverInkSoft }]}>Đang mở Rủ Đi…</Text>
    </View>
  </View>;
}

/** Keep the navigator mounted while the session restores; protect its controls. */
export function SessionOpening({ children }: { children: ReactNode }) {
  const { phienDaDoc } = useRudiSession();
  const scene = useRef<View>(null);
  useEffect(() => {
    if (Platform.OS !== "web") return;
    const element = scene.current as unknown as HTMLElement | null;
    if (!element) return;
    element.inert = !phienDaDoc;
    return () => { element.inert = false; };
  }, [phienDaDoc]);
  return <View style={styles.flex}>
    <View ref={scene} style={styles.flex} pointerEvents={phienDaDoc ? "auto" : "none"} aria-hidden={!phienDaDoc} accessibilityElementsHidden={!phienDaDoc} importantForAccessibility={phienDaDoc ? "auto" : "no-hide-descendants"}>{children}</View>
    {!phienDaDoc ? <View style={StyleSheet.absoluteFill}><OpeningApp /></View> : null}
  </View>;
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  opening: { flex: 1, justifyContent: "center", alignItems: "center", gap: 24, padding: 24 },
  message: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", justifyContent: "center", gap: 12 },
  // No custom font is assumed before the font loader has answered.
  copy: { fontSize: 16, lineHeight: 24, textAlign: "center" },
});
