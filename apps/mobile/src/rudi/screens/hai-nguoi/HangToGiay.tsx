import { Ionicons } from "@expo/vector-icons";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../../theme";
import { type ToGiay, cauTrangThai } from "../../to-giay/to-giay";
import { ThuGapBa } from "../../ui/art/Motif";

/**
 * «Tờ giấy của hai mình»: the one pinned line under the pair chat's title,
 * the same idiom `Group.tsx` uses for the outing pin (icon, two lines, a
 * chevron) so the row reads as the room's fixed line and not as a message.
 *
 * The icon is the letter motif (`ThuGapBa`, no coral): the coral belongs to
 * the open sheet on the paper surface, one leading mark per surface (spec
 * §16.4), and this row only says there IS a sheet. The second line is the
 * state in words from `cauTrangThai`, so the chat tells the reader whether
 * something waits for them without opening the notebook.
 */
export function HangToGiay({
  toMo,
  toiId,
  onPress,
  tieuDe = "Tờ giấy của hai mình",
  cauMo = "Đi đâu không?",
  testID,
}: {
  toMo: ToGiay | undefined;
  toiId: string;
  onPress: () => void;
  /** The notebook kind's own name for the paper surface (`banTinhCua(...).tuVung.tenKhongGian`). */
  tieuDe?: string;
  /** The notebook kind's opening line, shown when no sheet is on the table. */
  cauMo?: string;
  testID?: string;
}) {
  const { colors, radius } = useRudiTheme();
  const phu = toMo ? cauTrangThai(toMo, toiId) : `Chưa có tờ nào tuần này. ${cauMo}`;
  return (
    <Pressable
      accessibilityLabel={`${tieuDe}. ${phu}`}
      accessibilityRole="button"
      onPress={onPress}
      style={({ pressed }) => [styles.hang, { borderColor: colors.line }, pressed && styles.mo]}
      testID={testID ?? "hang-to-giay"}
    >
      <View style={[styles.icon, { backgroundColor: colors.paper, borderColor: colors.lineStrong, borderRadius: radius.small }]}>
        <ThuGapBa height={20} width={26} />
      </View>
      <View style={styles.chu}>
        <Text numberOfLines={1} style={[typography.label, { color: colors.ink }]}>{tieuDe}</Text>
        <Text numberOfLines={2} style={[typography.caption, { color: colors.inkSoft }]}>{phu}</Text>
      </View>
      <Ionicons color={colors.inkFaint} name="chevron-forward" size={18} />
    </Pressable>
  );
}

const styles = StyleSheet.create({
  hang: { flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 10, paddingHorizontal: 12, borderBottomWidth: StyleSheet.hairlineWidth },
  mo: { opacity: 0.8 },
  icon: { width: 40, height: 40, alignItems: "center", justifyContent: "center", borderWidth: StyleSheet.hairlineWidth },
  chu: { flex: 1, gap: 2 },
});
