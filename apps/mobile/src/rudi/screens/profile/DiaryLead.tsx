import { Image } from "expo-image";
import { useRouter } from "expo-router";
import { useEffect, useMemo, useRef, useState } from "react";
import { StyleSheet, View, useWindowDimensions } from "react-native";
import { tokenPhienHienTai } from "../../../api";
import { KHUNG_CANH, hinhCanh } from "../../art/canh";
import { KHUNG_DAU_CHAN, hinhDauChan } from "../../art/dau-chan";
import { diaryImage, publishedPhoto, type Diary } from "../../diary/api";
import type { DiaryCollection } from "../../diary/useDiaryCollection";
import { typography, useRudiTheme } from "../../theme";
import { Text, useNativeFontMetricsOwner, useNativeStandaloneTextMetrics } from "../../ui/NativeFontMetrics";
import { RudiButton } from "../../ui";
import { SkeletonLines } from "../../ui/Skeleton";
import { VeLop } from "../../ui/art/VeLop";

function JournalBinding() {
  const { colors } = useRudiTheme();
  return (
    <View
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
      pointerEvents="none"
      style={styles.bindingRail}
    >
      <View style={[styles.bindingRule, { backgroundColor: colors.accentSoft }]} />
      <View style={[styles.bindingHole, { top: 18, backgroundColor: colors.ground, borderColor: colors.lineStrong }]} />
      <View style={[styles.bindingHole, { top: 64, backgroundColor: colors.ground, borderColor: colors.lineStrong }]} />
      <View style={[styles.bindingHole, { top: 110, backgroundColor: colors.ground, borderColor: colors.lineStrong }]} />
    </View>
  );
}

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
  const { colors, radius } = useRudiTheme();
  return (
    <View
      testID="profile-diary-error"
      style={[
        styles.journalSheet,
        {
          backgroundColor: colors.card,
          borderColor: colors.lineStrong,
          borderRadius: radius.base,
        },
      ]}
    >
      <JournalBinding />
      <View style={styles.emptyRow}>
        <VeLop height={64} khungH={KHUNG_CANH.h} khungW={KHUNG_CANH.w} lop={hinhCanh("chua-doc-duoc")} width={82} />
        <View style={styles.emptyCopy}>
          <Text accessibilityRole="header" style={[typography.h2, { color: colors.ink }]}>Chưa mở được trang đã giữ</Text>
          <Text accessibilityRole="alert" style={[typography.body, { color: colors.inkSoft }]}>{message}</Text>
          <RudiButton full={false} label="Thử lại" labelAlign="left" variant="ghost" style={styles.textAction} onPress={retry} />
        </View>
      </View>
    </View>
  );
}

export function ProfileDiaryEmpty() {
  const router = useRouter();
  const { colors, radius } = useRudiTheme();
  return (
    <View
      testID="profile-diary-empty"
      style={[
        styles.journalSheet,
        {
          backgroundColor: colors.card,
          borderColor: colors.lineStrong,
          borderRadius: radius.base,
        },
      ]}
    >
      <JournalBinding />
      <View style={styles.emptyRow}>
        <VeLop height={74} khungH={KHUNG_CANH.h} khungW={KHUNG_CANH.w} lop={hinhCanh("chua-co-ky-niem")} width={96} />
        <View style={styles.emptyCopy}>
          <Text accessibilityRole="header" style={[typography.h2, { color: colors.ink }]}>Chưa có trang đã giữ</Text>
          <Text style={[typography.body, { color: colors.inkSoft }]}>Bạn có thể giữ lại cuộc đi từ trang của kèo.</Text>
          <RudiButton full={false} label="Xem kèo của nhóm" labelAlign="left" variant="ghost" style={styles.textAction} onPress={() => router.push("/(tabs)/plan" as never)} />
        </View>
      </View>
    </View>
  );
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
  return (
    <View
      testID="profile-saved-diary"
      style={[
        styles.journalSheet,
        styles.saved,
        {
          backgroundColor: colors.card,
          borderColor: colors.lineStrong,
          borderRadius: radius.base,
        },
      ]}
    >
      <JournalBinding />
      <View style={[styles.content, stack && styles.contentStack]}>
        <View style={[styles.words, stack && styles.wordsStack]}>
          <Text accessibilityRole="header" style={[typography.h1, { color: colors.ink }]}>{diary.document.title}</Text>
          {diary.document.subtitle ? <Text style={[typography.body, { color: colors.inkSoft }]}>{diary.document.subtitle}</Text> : null}
          {excerpt ? <Text numberOfLines={3} style={[typography.body, { color: colors.inkSoft }]}>{excerpt}</Text> : null}
        </View>
        {source && !imageFailed ? (
          <Image accessibilityLabel={`Ảnh bìa ${diary.document.title}`} source={source} contentFit="cover" cachePolicy="none" onError={imageError} style={[styles.cover, stack && styles.coverStack, { borderRadius: radius.control }]} />
        ) : (
          <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants" pointerEvents="none" style={styles.savedStamp}>
            <VeLop height={40} khungH={KHUNG_DAU_CHAN} khungW={KHUNG_DAU_CHAN} lop={hinhDauChan("memories")} width={40} />
          </View>
        )}
      </View>
      {imageFailed ? <Text style={[typography.note, { color: colors.inkSoft }]}>Ảnh chưa mở được. Nội dung vẫn còn đây.</Text> : null}
      <View style={[styles.footer, { borderTopColor: colors.line }]}>
        <Text style={[typography.note, styles.metadata, { color: colors.inkSoft }]}>{`${diary.kind === "trip" ? "Sổ chuyến đi" : "Khoảnh khắc"} · ${diary.audience === "private" ? "Chỉ mình tôi" : "Công khai"}`}</Text>
        <RudiButton full={false} label="Mở trang" labelAlign="left" variant="ghost" style={styles.textAction} onPress={() => router.push(`/diaries/${diary.id}` as never)} />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  lead: { paddingTop: 18, gap: 14 },
  journalSheet: { position: "relative", overflow: "hidden", borderWidth: 1, paddingLeft: 32, paddingRight: 16, paddingTop: 16, paddingBottom: 14 },
  bindingRail: { position: "absolute", top: 0, bottom: 0, left: 0, width: 24 },
  bindingRule: { position: "absolute", top: 0, bottom: 0, left: 22, width: 1.5 },
  bindingHole: { position: "absolute", left: 7, width: 9, height: 9, borderRadius: 5, borderWidth: 1 },
  saved: { gap: 12 },
  content: { flexDirection: "row", alignItems: "flex-start", gap: 16 },
  contentStack: { flexDirection: "column" },
  words: { flex: 1, minWidth: 0, gap: 8 },
  wordsStack: { flex: 0, width: "100%" },
  cover: { width: "30%", aspectRatio: 3 / 4 },
  coverStack: { width: "100%", aspectRatio: 16 / 9 },
  savedStamp: { paddingTop: 2 },
  footer: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", justifyContent: "space-between", gap: 8, paddingTop: 10, borderTopWidth: StyleSheet.hairlineWidth },
  metadata: { flexShrink: 1 },
  emptyRow: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", gap: 14 },
  emptyCopy: { flex: 1, minWidth: 180, gap: 6, alignItems: "flex-start" },
  textAction: { paddingHorizontal: 0, borderWidth: 0, justifyContent: "flex-start", minWidth: 48, minHeight: 48 },
});
