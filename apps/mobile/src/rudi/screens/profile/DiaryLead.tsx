import { Image } from "expo-image";
import { useRouter } from "expo-router";
import { useEffect, useMemo, useRef, useState } from "react";
import { StyleSheet, View, useWindowDimensions } from "react-native";
import { tokenPhienHienTai } from "../../../api";
import { diaryImage, publishedPhoto, type Diary } from "../../diary/api";
import type { DiaryCollection } from "../../diary/useDiaryCollection";
import { typography, useRudiTheme } from "../../theme";
import { Text, useNativeFontMetricsOwner, useNativeStandaloneTextMetrics } from "../../ui/NativeFontMetrics";
import { RudiButton } from "../../ui";
import { SkeletonLines } from "../../ui/Skeleton";

/** Saved content leads; collection state never invents a journey or an excerpt. */
export function DiaryLead({ person, collection }: { person: string; collection: DiaryCollection }) {
  const { colors } = useRudiTheme();
  const first = collection.books[0];
  return <View testID="profile-diary-lead" style={styles.lead}>
    {collection.error ? <ProfileDiaryFailure message={collection.error} retry={collection.retry} /> : null}
    {first ? <SavedDiary person={person} diary={first} /> : collection.loading ? <View accessibilityLabel="Đang đọc trang đã giữ"><SkeletonLines lines={3} /></View> : !collection.error ? <ProfileDiaryEmpty /> : null}
    {first && collection.loading ? <Text accessibilityLiveRegion="polite" style={[typography.note, { color: colors.inkSoft }]}>Đang cập nhật trang đã giữ…</Text> : null}
  </View>;
}

export function ProfileDiaryFailure({ message, retry }: { message: string; retry: () => void }) {
  const { colors } = useRudiTheme();
  return <View testID="profile-diary-error" style={styles.empty}>
    <Text accessibilityRole="header" style={[typography.h2, { color: colors.ink }]}>Chưa mở được trang đã giữ</Text>
    <Text accessibilityRole="alert" style={[typography.body, { color: colors.inkSoft }]}>{message}</Text>
    <RudiButton full={false} label="Thử lại" variant="ghost" onPress={retry} />
  </View>;
}

export function ProfileDiaryEmpty() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  return <View testID="profile-diary-empty" style={styles.empty}>
    <Text accessibilityRole="header" style={[typography.h2, { color: colors.ink }]}>Chưa có trang đã giữ</Text>
    <Text style={[typography.body, { color: colors.inkSoft }]}>Bạn có thể giữ lại cuộc đi từ trang của kèo.</Text>
    <RudiButton full={false} label="Xem kèo của nhóm" variant="ghost" onPress={() => router.push("/(tabs)/plan" as never)} />
  </View>;
}

function SavedDiary({ person, diary }: { person: string; diary: Diary }) {
  const router = useRouter();
  const { colors, radius } = useRudiTheme();
  const { width, fontScale } = useWindowDimensions();
  const owned = useNativeFontMetricsOwner();
  const metrics = useNativeStandaloneTextMetrics(typography.body);
  const large = metrics ? (metrics.fontSize ?? typography.body.fontSize) >= typography.body.fontSize * 1.25 : owned || fontScale >= 1.25;
  const stack = width < 360 || large;
  const cover = diary.document.cover_id;
  const token = tokenPhienHienTai();
  const source = useMemo(() => cover ? diaryImage(person, publishedPhoto(diary.id, cover)) : undefined, [person, diary.id, cover, token]);
  const sourceEpoch = useRef({ source, generation: 0 });
  const alive = useRef(true);
  if (sourceEpoch.current.source !== source) {
    sourceEpoch.current = { source, generation: sourceEpoch.current.generation + 1 };
  }
  const imageGeneration = sourceEpoch.current.generation;
  useEffect(() => {
    alive.current = true;
    return () => { alive.current = false; };
  }, []);
  const [failedSource, setFailedSource] = useState<typeof source>();
  const imageFailed = source !== undefined && failedSource === source;
  const imageError = () => {
    // A retired image must not clear a newer source's accepted failure state.
    if (!alive.current || sourceEpoch.current.source !== source || sourceEpoch.current.generation !== imageGeneration) return;
    setFailedSource(source);
  };
  const excerpt = diary.document.pages.find(page => page.text.trim() !== "")?.text;
  return <View testID="profile-saved-diary" style={styles.saved}>
    <View style={[styles.content, stack && styles.contentStack]}>
      <View style={[styles.words, stack && styles.wordsStack]}>
        <Text accessibilityRole="header" style={[typography.h1, { color: colors.ink }]}>{diary.document.title}</Text>
        {diary.document.subtitle ? <Text style={[typography.body, { color: colors.inkSoft }]}>{diary.document.subtitle}</Text> : null}
        {excerpt ? <Text numberOfLines={3} style={[typography.body, { color: colors.inkSoft }]}>{excerpt}</Text> : null}
      </View>
      {source && !imageFailed ? <Image accessibilityLabel={`Ảnh bìa ${diary.document.title}`} source={source} contentFit="cover" cachePolicy="none" onError={imageError} style={[styles.cover, stack && styles.coverStack, { borderRadius: radius.control }]} /> : null}
    </View>
    {imageFailed ? <Text style={[typography.note, { color: colors.inkSoft }]}>Ảnh chưa mở được. Nội dung vẫn còn đây.</Text> : null}
    <View style={styles.footer}>
      <Text style={[typography.note, styles.metadata, { color: colors.inkSoft }]}>{`${diary.kind === "trip" ? "Sổ chuyến đi" : "Khoảnh khắc"} · ${diary.audience === "private" ? "Chỉ mình tôi" : "Công khai"}`}</Text>
      <RudiButton full={false} label="Mở trang" variant="ghost" onPress={() => router.push(`/diaries/${diary.id}` as never)} />
    </View>
  </View>;
}

const styles = StyleSheet.create({
  lead: { paddingTop: 24, gap: 16 },
  saved: { gap: 12 },
  content: { flexDirection: "row", alignItems: "flex-start", gap: 20 },
  contentStack: { flexDirection: "column" },
  words: { flex: 1, minWidth: 0, gap: 10 },
  wordsStack: { flex: 0, width: "100%" },
  cover: { width: "30%", aspectRatio: 3 / 4 },
  coverStack: { width: "100%", aspectRatio: 16 / 9 },
  footer: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", justifyContent: "space-between", gap: 8 },
  metadata: { flexShrink: 1 },
  empty: { gap: 10, alignItems: "flex-start" },
});
