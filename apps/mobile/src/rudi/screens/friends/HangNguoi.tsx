/**
 * One person, the way every friend surface shows one (M2): their avatar
 * (photograph or initials), the name, a caption, and an optional trailing
 * action pair. The friend list and the add-by-phone confirm step share it so a
 * person reads the same on both screens. `HangNguoiCho` is the same
 * silhouette in grey while the server answers.
 */
import { type ReactNode } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { mucNguoi, typography, useRudiTheme } from "../../theme";
import { AvatarNguoi } from "../../ui/AvatarNguoi";
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
  personId,
}: {
  ten: string;
  phu: string;
  duoi?: ReactNode;
  onPress?: () => void;
  /** A friend's id draws their photograph; absent (a stranger's request) stays initials. */
  personId?: string;
}) {
  const { colors, dark } = useRudiTheme();
  const nguoi = (
    <>
      <AvatarNguoi name={ten} personId={personId} size={40} />
      <View style={styles.hangChu}>
        {/* A known person's name is written in their own ink (ADR-0037 D6). */}
        <Text numberOfLines={1} style={[typography.body, { color: personId ? mucNguoi(personId, dark) : colors.ink }]}>
          {ten}
        </Text>
        <Text numberOfLines={2} style={[typography.caption, { color: colors.inkFaint }]}>
          {phu}
        </Text>
      </View>
    </>
  );
  // B8 (QC 24/09): the whole row used to be the profile button with «Nhắn
  // tin» or «Đồng ý» inside it -- a button in a button, invalid HTML on the
  // web and two targets fused for a screen reader. The way to the profile is
  // the person; the actions stand beside it.
  return (
    <View style={styles.hang}>
      {onPress === undefined ? (
        <View style={styles.nguoi}>{nguoi}</View>
      ) : (
        <Pressable accessibilityLabel={`Xem hồ sơ ${ten}`} accessibilityRole="button" onPress={onPress} style={styles.nguoi}>
          {nguoi}
        </Pressable>
      )}
      {duoi ? <View style={styles.duoi}>{duoi}</View> : null}
    </View>
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
  nguoi: { flex: 1, flexDirection: "row", alignItems: "center", gap: 12, minHeight: 44 },
  hangChu: { flex: 1, gap: 2 },
  duoi: { flexShrink: 0, flexDirection: "row", alignItems: "center", gap: 8 },
});
