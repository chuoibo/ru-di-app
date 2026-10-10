import { Image, type ImageSource } from "expo-image";
import { useEffect, useState, type ComponentProps } from "react";
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../theme";
import { RudiIcon } from "./RudiIcon";

/** Preserve the photo's slot and authenticated source through byte-load retries. */
export function MemoryPhoto({ source, label, ratio, fit = "contain", onLoad }: {
  source: ImageSource; label: string; ratio: number;
  fit?: "contain" | "cover"; onLoad?: ComponentProps<typeof Image>["onLoad"];
}) {
  const { colors } = useRudiTheme();
  const [failed, setFailed] = useState(false);
  const [loading, setLoading] = useState(true);
  const [attempt, setAttempt] = useState(0);
  const identity = JSON.stringify(source);
  useEffect(() => { setFailed(false); setLoading(true); }, [identity]);
  return (
    <View style={[styles.frame, { aspectRatio: ratio, backgroundColor: colors.card }]}>
      {!failed ? <Image
        key={attempt}
        accessibilityLabel={label}
        cachePolicy="none"
        contentFit={fit}
        onError={() => { setFailed(true); setLoading(false); }}
        onLoad={onLoad}
        onLoadEnd={() => setLoading(false)}
        source={source}
        style={StyleSheet.absoluteFill}
      /> : (
        <ScrollView contentContainerStyle={styles.error} keyboardShouldPersistTaps="handled">
          <RudiIcon color={colors.inkSoft} name="image-outline" size={28} />
          <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.inkSoft, textAlign: "center" }]}>Chưa tải được ảnh</Text>
          <Pressable accessibilityLabel="Thử tải lại ảnh" accessibilityRole="button" onPress={(event) => {
            event.stopPropagation(); setLoading(true); setFailed(false); setAttempt((n) => n + 1);
          }} style={styles.retry}>
            <Text style={[typography.label, { color: colors.accent }]}>Thử lại</Text>
          </Pressable>
        </ScrollView>
      )}
      {loading && !failed ? <ActivityIndicator accessibilityLabel="Đang tải ảnh" color={colors.inkSoft} style={StyleSheet.absoluteFill} /> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  frame: { width: "100%", minHeight: 96, overflow: "hidden" },
  error: { flexGrow: 1, alignItems: "center", justifyContent: "center", gap: 8, padding: 16 },
  retry: { minHeight: 48, minWidth: 64, alignItems: "center", justifyContent: "center", paddingHorizontal: 10 },
});
