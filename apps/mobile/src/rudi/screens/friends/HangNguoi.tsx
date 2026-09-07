/**
 * One person, the way every friend surface shows one (M2): a warm initial
 * tile, the name, a caption, and an optional trailing action pair. The friend
 * list and the add-by-phone confirm step share it so a person reads the same
 * on both screens. `HangNguoiCho` is the same silhouette in grey while the
 * server answers.
 */
import { type ReactNode } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../../theme";
import { Avatar } from "../../ui/Avatar";
import { SkeletonGroup, SkeletonRow } from "../../ui/Skeleton";

/**
 * `onPress` turns the row into the way into that person's profile. It stays
 * optional: a row for somebody the reader may not open (a pending request from
 * a stranger) has no destination, and drawing it as a button would promise one.
 */
export function HangNguoi({
  ten,
  phu,
  duoi,
  onPress,
}: {
  ten: string;
  phu: string;
  duoi?: ReactNode;
  onPress?: () => void;
}) {
  const { colors } = useRudiTheme();
  const than = (
    <>
      <Avatar name={ten} size={40} />
      <View style={styles.hangChu}>
        <Text numberOfLines={1} style={[typography.body, { color: colors.ink }]}>
          {ten}
        </Text>
        <Text numberOfLines={2} style={[typography.caption, { color: colors.inkFaint }]}>
          {phu}
        </Text>
      </View>
      {duoi ? <View style={styles.duoi}>{duoi}</View> : null}
    </>
  );
  if (onPress === undefined) return <View style={styles.hang}>{than}</View>;
  return (
    <Pressable
      accessibilityLabel={`Xem hồ sơ ${ten}`}
      accessibilityRole="button"
      onPress={onPress}
      style={styles.hang}
    >
      {than}
    </Pressable>
  );
}

export function HangNguoiCho({ soHang = 3 }: { soHang?: number }) {
  return (
    <SkeletonGroup>
      {Array.from({ length: soHang }, (_, i) => (
        <SkeletonRow key={i} leading={40} />
      ))}
    </SkeletonGroup>
  );
}

const styles = StyleSheet.create({
  hang: { minHeight: 56, flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 8 },
  hangChu: { flex: 1, gap: 2 },
  duoi: { flexShrink: 0, flexDirection: "row", alignItems: "center", gap: 8 },
});
