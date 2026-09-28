import { Ionicons } from "@expo/vector-icons";
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { BADGE_TITLES, docHanhTrinh, type JourneySnapshot } from "../../ky-niem/achievement-routes";
import { typography, useRudiTheme } from "../../theme";
import { BadgeArt } from "../../ui/BadgeArt";

export function HanhTrinhTeaser({ personId }: { personId: string }) {
  const router = useRouter();
  const { colors, radius } = useRudiTheme();
  const [book, setBook] = useState<JourneySnapshot | null | undefined>(undefined);

  useFocusEffect(useCallback(() => {
    let active = true;
    void docHanhTrinh(personId).then((snapshot) => {
      if (active) setBook(snapshot);
    }).catch(() => {
      if (active) setBook(null);
    });
    return () => { active = false; };
  }, [personId]));

  const chapter = book?.chapters[0];
  let targetId: string | undefined;
  if (book?.active_run) targetId = book.active_run.ending_id;
  else if (chapter) targetId = chapter.target_ending_id;
  const target = book?.candidates.find((choice) => choice.id === targetId);
  const lastBadge = book?.earned_badges.at(-1);
  const title = chapter?.title ?? target?.title ?? (lastBadge ? `Dấu mốc mới: ${BADGE_TITLES[lastBadge.id] ?? "Một hành trình"}` : book === null ? "Chưa mở được sổ hành trình" : book === undefined ? "Đang mở sổ hành trình…" : "Cuốn sổ của bạn còn nhiều ngã rẽ");
  const line = chapter?.line ?? target?.description ?? (lastBadge ? "Huy hiệu này ở lại với bạn. Chọn một ngã rẽ để câu chuyện tiếp tục theo cách của mình." : book === null ? "Chạm để thử mở lại những ngã rẽ của bạn." : book === undefined ? "Một chút nữa, trang mới của bạn sẽ hiện ra." : "Chọn một lối đi để chặng tiếp theo mang dấu riêng của bạn.");
  const progress = target?.requirements.map((item) => `${item.label} ${Math.min(item.have, item.need)}/${item.need}`).join(" · ");

  return (
    <Pressable
      accessibilityLabel={`${title}. Mở sổ hành trình`}
      accessibilityRole="button"
      onPress={() => router.push("/achievements")}
      style={({ pressed }) => [styles.cover, { backgroundColor: colors.cover, borderRadius: radius.base, opacity: pressed ? 0.86 : 1 }]}
    >
      <View style={[styles.rule, { backgroundColor: colors.coverLineStrong }]} />
      <View style={styles.row}>
        <View style={styles.copy}>
          <Text style={[typography.h2, { color: colors.coverInk }]}>{title}</Text>
          <Text style={[typography.body, { color: colors.coverInkSoft }]}>{line}</Text>
        </View>
        {target ? <BadgeArt badgeId={target.id} label={target.title} size={88} state={target.earned ? "unlocked" : "progress"} /> : lastBadge ? <BadgeArt badgeId={lastBadge.id} label={BADGE_TITLES[lastBadge.id] ?? "Huy hiệu"} size={88} state="unlocked" /> : null}
      </View>
      {progress ? <Text style={[typography.note, { color: colors.coverInkSoft }]}>{progress}</Text> : null}
      <View style={[styles.footer, { borderTopColor: colors.coverLineStrong }]}>
        <Text style={[typography.label, { color: colors.coverInk, flex: 1 }]}>{book ? `${book.earned_badges.length} huy hiệu · ${book.mp4_credits.available} lượt dựng MP4` : "Mở sổ hành trình"}</Text>
        <Ionicons color={colors.coverInk} name="arrow-forward" size={20} />
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  cover: { padding: 20, gap: 16, overflow: "hidden" },
  rule: { height: 2, width: 88 },
  row: { flexDirection: "row", alignItems: "center", gap: 14 },
  copy: { flex: 1, gap: 8 },
  footer: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", gap: 10, borderTopWidth: StyleSheet.hairlineWidth, paddingTop: 13 },
});
