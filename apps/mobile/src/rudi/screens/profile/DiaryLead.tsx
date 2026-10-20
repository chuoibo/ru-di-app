import { Image } from "expo-image";
import { useRouter } from "expo-router";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { StyleSheet, View, useWindowDimensions, type StyleProp, type ViewStyle } from "react-native";
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

/** The binding rail's width: holes and margin rule live in this left gutter. */
const RAY_GAY = 24;

/**
 * One ledger sheet: the binding rail is a flow sibling, so nothing overlaps
 * the content. A centred sheet (`giua`) also pads its right by the rail's
 * width, so its axis is the card's axis: the rail alone pushed the empty
 * state's centre about 11 css px right of the card's (finish review 21/10).
 */
function JournalSheet({ testID, children, style, giua = false }: { testID: string; children: ReactNode; style?: StyleProp<ViewStyle>; giua?: boolean }) {
  const { colors, radius, space } = useRudiTheme();
  return (
    <View testID={testID} style={[styles.journalSheet, { backgroundColor: colors.card, borderColor: colors.lineStrong, borderRadius: radius.base }]}>
      <JournalBinding />
      <View style={[styles.sheetBody, { padding: space.md }, giua && { paddingRight: space.md + RAY_GAY }, style]}>{children}</View>
    </View>
  );
}

/** Saved content leads; collection state never invents a journey or an excerpt. */
export function DiaryLead({ person, collection, compact = false }: { person: string; collection: DiaryCollection; compact?: boolean }) {
  const { colors, space } = useRudiTheme();
  const first = collection.books[0];
  return <View testID="profile-diary-lead" style={[styles.lead, { paddingTop: space.md }]}>
    {collection.error ? <ProfileDiaryFailure message={collection.error} retry={collection.retry} /> : null}
    {first ? <SavedDiary person={person} diary={first} /> : collection.loading ? <View accessibilityLabel="Đang đọc trang đã giữ"><SkeletonLines lines={3} /></View> : !collection.error ? <ProfileDiaryEmpty compact={compact} /> : null}
    {first && collection.loading ? <Text accessibilityLiveRegion="polite" style={[typography.note, { color: colors.inkSoft }]}>Đang cập nhật trang đã giữ…</Text> : null}
  </View>;
}

export function ProfileDiaryFailure({ message, retry }: { message: string; retry: () => void }) {
  const { colors } = useRudiTheme();
  return (
    <JournalSheet testID="profile-diary-error">
      <View style={styles.emptyRow}>
        <VeLop height={64} khungH={KHUNG_CANH.h} khungW={KHUNG_CANH.w} lop={hinhCanh("chua-doc-duoc")} width={82} />
        <View style={styles.emptyCopy}>
          <Text accessibilityRole="header" style={[typography.h2, { color: colors.ink }]}>Chưa mở được trang đã giữ</Text>
          <Text accessibilityRole="alert" style={[typography.body, { color: colors.inkSoft }]}>{message}</Text>
          <RudiButton full={false} label="Thử lại" labelAlign="left" variant="ghost" style={styles.textAction} onPress={retry} />
        </View>
      </View>
    </JournalSheet>
  );
}

/**
 * No kept page yet. On the shelf («Trang đã giữ») it is the whole answer; on
 * «Hồ sơ» it follows the facts (`compact`): a smaller drawing and a quieter
 * heading, because an empty diary must not lead a profile tab.
 */
export function ProfileDiaryEmpty({ compact = false }: { compact?: boolean }) {
  const router = useRouter();
  const { colors, space } = useRudiTheme();
  return (
    <JournalSheet giua testID="profile-diary-empty" style={compact ? { paddingVertical: space.sm } : undefined}>
      {/* One centred axis: the drawing, the words and the door share it, and
          the sheet's padding puts that axis on the card's centre. */}
      <View style={[styles.emptyColumn, { gap: compact ? space.xs : space.sm }]}>
        <VeLop height={compact ? 48 : 74} khungH={KHUNG_CANH.h} khungW={KHUNG_CANH.w} lop={hinhCanh("chua-co-ky-niem")} width={compact ? 62 : 96} />
        <Text accessibilityRole="header" style={[compact ? typography.title : typography.h2, styles.centred, { color: colors.ink }]}>Chưa có trang đã giữ</Text>
        {/* The last two words are held together, so the line never ends on «kèo.» alone. */}
        <Text style={[compact ? typography.note : typography.body, styles.centred, { color: colors.inkSoft }]}>{"Bạn có thể giữ lại cuộc đi từ trang của\u00a0kèo."}</Text>
        <RudiButton compact={compact} full={false} label="Xem kèo của nhóm" variant="ghost" onPress={() => router.push("/(tabs)/plan" as never)} />
      </View>
    </JournalSheet>
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
    <JournalSheet testID="profile-saved-diary" style={styles.saved}>
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
    </JournalSheet>
  );
}

const styles = StyleSheet.create({
  lead: { gap: 14 },
  journalSheet: { flexDirection: "row", overflow: "hidden", borderWidth: 1 },
  sheetBody: { flex: 1, minWidth: 0 },
  bindingRail: { width: 24, alignSelf: "stretch" },
  emptyColumn: { alignItems: "center" },
  centred: { textAlign: "center", maxWidth: 420 },
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
