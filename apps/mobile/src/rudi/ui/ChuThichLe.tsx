/**
 * A note written in the margin (ADR-0037 D1, D8): a sentence that belongs
 * beside the thing it is about rather than in a paragraph above it -- «nothing
 * is sent until you press use», «not end-to-end encrypted», how a total was
 * rounded. Still words on the page, always visible: a margin note is a way to
 * place a sentence, never a way to hide one.
 */
import { Ionicons } from "@expo/vector-icons";
import type { ComponentProps, ReactNode } from "react";
import { StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../theme";

type TenIcon = ComponentProps<typeof Ionicons>["name"];

export function ChuThichLe({ children, icon, testID }: { children: ReactNode; icon?: TenIcon; testID?: string }) {
  const { colors } = useRudiTheme();
  return (
    <View style={styles.dong} testID={testID}>
      <View importantForAccessibility="no-hide-descendants" style={[styles.le, { backgroundColor: colors.lineStrong }]} />
      {icon ? <Ionicons color={colors.inkSoft} importantForAccessibility="no" name={icon} size={14} style={styles.icon} /> : null}
      <Text style={[typography.note, styles.chu, { color: colors.inkSoft }]}>{children}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  dong: { flexDirection: "row", alignItems: "flex-start", gap: 8, paddingVertical: 2 },
  le: { width: 2, alignSelf: "stretch", borderRadius: 1, minHeight: 16 },
  icon: { marginTop: 2 },
  chu: { flex: 1 },
});
