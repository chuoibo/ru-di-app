import { Image } from "expo-image";
import { useRouter } from "expo-router";
import { useEffect, useRef, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import Animated, { useAnimatedStyle, useSharedValue, withTiming } from "react-native-reanimated";
import { newAttempt, taiAnhCaNhanLen } from "../../api";
import { chonAnh, nenVaDung, boAnh } from "../ky-niem/chon-anh";
import { typography, useRudiTheme } from "../theme";
import { Field, Heading, RudiButton, RudiScreen, Segmented, TopBar } from "../ui";
import { ErrorState } from "../ui/ErrorState";
import { SkeletonGroup, SkeletonCard } from "../ui/Skeleton";
import { Sheet } from "../ui/Sheet";
import { useMotion } from "../ui/useMotion";
import { ToGiay } from "../ui/ToGiay";
import { RouteLine } from "../ui/RouteLine";
import { ngayKieuViet } from "../chat/to-hen-chung";
import { useNhuongChoNep } from "../nep/NepProvider";
import { BookView } from "./BookView";
import { togglePhoto, buildDiary, diaryImage, endOuting, initialPhotos, movePage, publishedPhoto, readDiary, readEnding, readJob, readSources, saveDiary, selectedBundle, type Diary, type DiaryDocument, type DiaryKind, type DiarySource, type Ending } from "./api";

/**
 * THESIS: A trip becomes a book the owner can keep in their own words.
 * OWN-WORLD: Existing paper, ink, cloth and coral fold; no new visual identity.
 * STORY: Close together, choose what to share, compose privately, keep personally.
 * FIRST VIEWPORT: One invitation to keep this outing, one clear next action.
 * FORM: A reading page with a cover; editing tools follow rather than cover it.
 * FINISH: Inspect native light/dark, large text and reduced motion, then review.
 */
export function EndingScreen({ person, outing }: { person: string; outing: string }) {
  const router = useRouter(); const { colors } = useRudiTheme(); const motion = useMotion();
  const [ending, setEnding] = useState<Ending | null>(null); const [kind, setKind] = useState<DiaryKind>("moment");
  const [source, setSource] = useState<DiarySource | null>(null); const [selected, setSelected] = useState<string[]>([]);
  const [excerpt, setExcerpt] = useState(""); const [document, setDocument] = useState<DiaryDocument | null>(null);
  const [saved, setSaved] = useState<Diary | null>(null); const [audience, setAudience] = useState<Diary["audience"]>("private");
  const [phase, setPhase] = useState<"loading" | "ending" | "sources" | "building" | "editing" | "saved">("loading");
  const [busy, setBusy] = useState(false); const [error, setError] = useState<string | null>(null);
  const [edit, setEdit] = useState(false); const [pickTarget, setPickTarget] = useState<number | "cover" | null>(null);
  const mounted = useRef(true); const pending = useRef<{ digest: string; id: string } | null>(null);
  const retry = useRef<(() => Promise<void>) | null>(null);
  const turn = useSharedValue(0); const bookStyle = useAnimatedStyle(() => ({ transform: [{ translateY: turn.value * 14 }, { scale: 1 - turn.value * 0.025 }] }));
  useNhuongChoNep(pickTarget !== null);
  const report = (e: unknown) => setError(e instanceof Error ? e.message : "Chưa giữ được trang này. Bạn thử lại nhé.");
  async function loadSources() { const s = await readSources(person, outing); if (!mounted.current) return; setSource(s); setSelected(initialPhotos(s)); }
  async function load() {
    retry.current = load;
    setError(null); setPhase("loading");
    try {
      const e = await readEnding(person, outing); if (!mounted.current) return; setEnding(e); setKind(e.kind);
      if (e.ended_at) {
        await loadSources();
        if (e.diary_id) { const b = await readDiary(person, e.diary_id); if (!mounted.current) return; setSaved(b); setDocument(b.document); setAudience(b.audience); setPhase("editing"); }
        else setPhase("sources");
      } else setPhase("ending");
    } catch (e) { if (mounted.current) report(e); }
  }
  useEffect(() => { mounted.current = true; void load(); return () => { mounted.current = false; }; }, [person, outing]);
  async function close() { retry.current = close; setBusy(true); setError(null); try { const e = await endOuting(person, outing, kind); setEnding(e); await loadSources(); setPhase("sources"); } catch (e) { report(e); } finally { setBusy(false); } }
  const bundle = source ? selectedBundle(source, selected, excerpt.trim() ? [excerpt.trim()] : []) : null;
  async function compose(ai: boolean) {
    retry.current = () => compose(ai);
    if (!bundle) return; setError(null); setPhase("building");
    const digest = JSON.stringify([bundle, ai]); if (pending.current?.digest !== digest) pending.current = { digest, id: newAttempt().key };
    try {
      let j = await buildDiary(person, outing, bundle, ai, pending.current.id);
      for (let i = 0; i < 90 && (j.status === "queued" || j.status === "running"); i++) {
        await new Promise((resolve) => setTimeout(resolve, 1200)); if (!mounted.current) return; j = await readJob(person, j.id);
      }
      if (!mounted.current) return;
      if (j.status !== "succeeded" || !j.result) { pending.current = null; throw new Error("Nếp chưa xếp xong cuốn sổ. Chất liệu vẫn còn; bạn thử lại hoặc tự xếp trang nhé."); }
      setDocument(j.result); setPhase("editing"); setEdit(false); turn.value = 1; turn.value = withTiming(0, motion.timing("shared", "decelerate"));
    } catch (e) { if (mounted.current) { report(e); setPhase("sources"); } }
  }
  async function keep() {
    retry.current = keep;
    if (!document) return; setBusy(true); setError(null);
    try { const b = await saveDiary(person, outing, document, saved?.revision ?? 0, audience); setSaved(b); setPhase("saved"); motion.haptic.success(); turn.value = 1; turn.value = withTiming(0, motion.timing("celebrate", "decelerate")); }
    catch (e) { report(e); } finally { setBusy(false); }
  }
  async function upload() {
    retry.current = upload;
    setBusy(true); setError(null);
    let chosen: Awaited<ReturnType<typeof chonAnh>> = null;
    try {
      chosen = await chonAnh();
      if (!chosen) return;
      const p = await nenVaDung(chosen, (photo) => taiAnhCaNhanLen(photo, person));
      const id = p.url.split("/").pop()!;
      setSource((s) => s ? { ...s, photos: [...s.photos, { id, url: p.url, caption: "", day: s.ends_on }] } : s);
      setSelected((ids) => togglePhoto(ids, id, 40));
    } catch (e) { report(e); if (chosen) await boAnh(chosen); } finally { setBusy(false); }
  }
  function choosePhoto(id: string) {
    if (!document || pickTarget === null) return;
    if (pickTarget === "cover") { setDocument({ ...document, cover_id: id }); setPickTarget(null); return; }
    const pages = document.pages.map((p, i) => {
      if (i !== pickTarget) return p;
      const ids = togglePhoto(p.photo_ids, id, 4);
      return { ...p, photo_ids: ids, layout: ids.length > 1 ? "collage" as const : ids.length ? "photo" as const : "note" as const };
    }); setDocument({ ...document, pages });
  }
  const photo = (id: string) => {
    const p = source?.photos.find((p) => p.id === id);
    return p?.url ? diaryImage(person, p.url) : saved ? diaryImage(person, publishedPhoto(saved.id, id)) : undefined;
  };
  return <RudiScreen key={phase} testID="diary-ending-screen" avoidKeyboard overlay={<Sheet open={pickTarget !== null} onClose={() => setPickTarget(null)} accessibilityLabel="Chọn ảnh cho trang">
    <Heading size="h2" title={pickTarget === "cover" ? "Tấm nào mở đầu câu chuyện?" : "Ảnh cho trang này"} subtitle={pickTarget === "cover" ? "Chọn một tấm làm bìa. Những ảnh khác vẫn ở trong sổ." : "Tối đa bốn ảnh mỗi trang. Chạm ảnh đã chọn để bỏ."} />
    <View style={styles.grid}>{source?.photos.map((p) => <Pressable key={p.id} accessibilityRole="button" accessibilityLabel={`Chọn ảnh ${p.caption || p.day}`} onPress={() => choosePhoto(p.id)} style={styles.tile}><Image source={photo(p.id)} cachePolicy="none" contentFit="cover" style={styles.thumb} /><Text style={[typography.caption, { color: colors.ink }]}>{typeof pickTarget === "number" && document?.pages[pickTarget]?.photo_ids.includes(p.id) ? "Đã chọn" : ngayKieuViet(p.day)}</Text></Pressable>)}</View>
    <RudiButton label="Xong phần ảnh" onPress={() => setPickTarget(null)} />
  </Sheet>}>
    <TopBar title={kind === "trip" ? "Sổ chuyến đi" : "Khoảnh khắc"} />
    {error ? <ErrorState title="Mình thử lại nhé" body={error} onRetry={() => { if (!busy) void retry.current?.(); }} /> : null}
    {phase === "loading" && !error ? <SkeletonGroup><SkeletonCard media={240} lines={3} /></SkeletonGroup> : null}
    {phase === "ending" ? <View style={styles.section}>
      <Heading title="Cuộc đi khép lại. Câu chuyện còn đây." subtitle="Giữ một khoảnh khắc nhỏ, hay dành cả cuốn sổ cho những ngày đi xa?" />
      <ToGiay dan style={styles.invitation}>
        <Text style={[typography.h1, { color: colors.ink }]}>{ending?.title}</Text>
        {ending ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{ngayKieuViet(ending.starts_on)}{ending.starts_on !== ending.ends_on ? ` đến ${ngayKieuViet(ending.ends_on)}` : ""}</Text> : null}
        <View style={{ alignItems: "center" }}><RouteLine width={240} height={86} stops={3} color={colors.inkSoft} activeColor={colors.accent} /></View>
        <Text style={[typography.note, { color: colors.inkSoft }]}>{kind === "trip" ? "Mang về vài tấm ảnh. Giữ lại cả một hành trình." : "Không cần đi xa mới có một ngày đáng nhớ."}</Text>
      </ToGiay>
      <Segmented items={["Khoảnh khắc", "Sổ chuyến đi"]} selected={kind === "trip" ? 1 : 0} onSelect={(i) => setKind(i ? "trip" : "moment")} />
      <Text style={[typography.body, { color: colors.inkSoft }]}>Khép cuộc đi cho cả hội. Sau đó, mỗi người tự chọn những điều muốn giữ trên tường mình.</Text>
      {ending?.can_end ? <RudiButton label="Khép cuộc đi" loading={busy} onPress={() => void close()} /> : <Text style={[typography.body, { color: colors.inkSoft }]}>Người tổ chức sẽ khép cuộc đi. Bạn quay lại đây để giữ kỷ niệm nhé.</Text>}
    </View> : null}
    {phase === "sources" && source && bundle ? <View style={styles.section}>
      <Heading title="Mang theo điều gì vào sổ?" subtitle="Ảnh đã được chọn theo ngày đi. Bạn xem lại nhé, nhất là khi hai cuộc hẹn trùng nhau." />
      <Text style={[typography.label, { color: colors.ink }]}>{selected.length} / 40 ảnh đã chọn</Text>
      <View style={styles.grid}>{source.photos.map((p) => <Pressable key={p.id} accessibilityRole="checkbox" accessibilityState={{ checked: selected.includes(p.id) }} accessibilityLabel={`Giữ ảnh ${p.caption || p.day}`} onPress={() => setSelected((ids) => togglePhoto(ids, p.id, 40))} style={[styles.tile, { borderColor: selected.includes(p.id) ? colors.accent : colors.line, borderWidth: 2 }]}>
        <Image source={p.url ? diaryImage(person, p.url) : undefined} cachePolicy="none" contentFit="cover" style={styles.thumb} /><Text style={[typography.caption, { color: colors.ink }]}>{selected.includes(p.id) ? "Đã chọn · " : ""}{ngayKieuViet(p.day)}</Text>{p.caption ? <Text numberOfLines={2} style={[typography.caption, { color: colors.inkSoft }]}>{p.caption}</Text> : null}
      </Pressable>)}</View>
      <RudiButton label="Thêm ảnh từ máy" variant="outline" loading={busy} onPress={() => void upload()} />
      <Field testID="diary-excerpt-input" label="Một đoạn chuyện muốn gửi cùng" placeholder="Dán trích đoạn bạn chọn, hoặc để trống" multiline maxLength={2000} value={excerpt} onChangeText={setExcerpt} helper="Chat không được tự đọc. Chỉ đoạn bạn đặt ở đây sẽ đi cùng ảnh." />
      <Heading size="h2" title="Nếp sẽ nhận đúng phần này" />
      <Text style={[typography.body, { color: colors.inkSoft }]}>{bundle.title}{"\n"}{ngayKieuViet(bundle.starts_on)} · {ngayKieuViet(bundle.ends_on)}{"\n"}{bundle.photos.length} ảnh · {bundle.excerpts.length} trích đoạn</Text>
      {bundle.places.length ? <Text style={[typography.body, { color: colors.inkSoft }]}>Nơi đã check-in: {bundle.places.join(", ")}</Text> : null}
      {bundle.excerpts.map((v, i) => <Text key={i} style={[typography.body, { color: colors.ink }]}>{v}</Text>)}
      <Text style={[typography.caption, { color: colors.inkSoft }]}>Bấm dựng sổ để chia sẻ phần này với AI. Bản dựng chỉ mình bạn xem; chưa có gì được đăng.</Text>
      <RudiButton label="Dựng sổ cùng Nếp" disabled={busy} onPress={() => void compose(true)} />
      <RudiButton label="Tự xếp trang, không gửi AI" variant="outline" disabled={busy} onPress={() => void compose(false)} />
      {document ? <RudiButton label="Về bản đang sửa" variant="ghost" onPress={() => { setError(null); setPhase("editing"); }} /> : null}
    </View> : null}
    {phase === "building" ? <View style={styles.section}><Heading title="Những mẩu chuyện đang thành trang…" subtitle="Ảnh và lời kể đang được xếp vào sổ. Chưa có gì được đăng lên tường." /><SkeletonGroup><SkeletonCard media={220} lines={3} /></SkeletonGroup></View> : null}
    {(phase === "editing" || phase === "saved") && document ? <Animated.View style={[styles.section, bookStyle]}>
      {phase === "saved" ? <Heading title="Đã giữ lại một cuộc đi." subtitle={audience === "private" ? "Cuốn sổ nằm trên tường bạn, chỉ mình bạn mở được." : "Cuốn sổ đã có trên tường bạn, mọi người trong app có thể xem."} /> : null}
      <BookView kind={kind} document={document} photo={photo} />
      {document.ai_generated ? <Text style={[typography.caption, { color: colors.inkSoft }]}>Nếp giúp xếp ảnh và viết lời từ phần bạn chia sẻ. Bạn là người giữ lời cuối.</Text> : null}
      {phase === "saved" ? <><RudiButton label="Về tường nhà mình" onPress={() => router.replace(`/people/${person}` as never)} /><RudiButton label="Mở lại cuốn sổ" variant="outline" onPress={() => setPhase("editing")} /></> : <>
        <RudiButton label={edit ? "Xem như người đọc" : "Sửa theo cách mình nhớ"} variant="outline" onPress={() => setEdit(!edit)} />
        {edit ? <View style={styles.section}>
          <Field testID="diary-title-input" label="Tên cuốn sổ" value={document.title} maxLength={200} onChangeText={(title) => setDocument({ ...document, title })} />
          <Field label="Lời mở" value={document.subtitle} maxLength={500} multiline onChangeText={(subtitle) => setDocument({ ...document, subtitle })} />
          <RudiButton label="Thay ảnh bìa" variant="outline" onPress={() => setPickTarget("cover")} />
          <RudiButton label="Thêm ảnh từ máy" variant="ghost" loading={busy} onPress={() => void upload()} />
          {document.pages.map((p, i) => <View key={i} style={[styles.editPage, { borderColor: colors.line }]}>
            <Text style={[typography.h2, { color: colors.ink }]}>Trang {i + 1}</Text>
            <Field label="Tên trang" value={p.heading} maxLength={200} onChangeText={(heading) => setDocument({ ...document, pages: document.pages.map((p, j) => j === i ? { ...p, heading } : p) })} />
            <Field label="Chuyện của trang" value={p.text} multiline maxLength={2000} onChangeText={(text) => setDocument({ ...document, pages: document.pages.map((p, j) => j === i ? { ...p, text } : p) })} />
            <RudiButton label={`Thay ảnh trang ${i + 1}`} variant="outline" onPress={() => setPickTarget(i)} />
            <RudiButton label={`Đưa trang ${i + 1} lên trước`} accessibilityLabel={`Đưa trang ${i + 1} lên trước`} variant="ghost" disabled={i === 0} onPress={() => setDocument(movePage(document, i, i - 1))} />
            <RudiButton label="Bỏ trang này" variant="ghost" disabled={document.pages.length === 1} onPress={() => setDocument({ ...document, pages: document.pages.filter((_, j) => j !== i) })} />
          </View>)}
          <RudiButton label="Thêm một trang viết" variant="outline" disabled={document.pages.length >= 24} onPress={() => setDocument({ ...document, pages: [...document.pages, { layout: "note", heading: "", text: "", photo_ids: [] }] })} />
        </View> : null}
        <Heading size="h2" title="Bạn muốn giữ cho ai xem?" />
        <Segmented items={["Chỉ mình tôi", "Công khai"]} selected={audience === "public" ? 1 : 0} onSelect={(i) => setAudience(i ? "public" : "private")} />
        {audience === "public" ? <Text style={[typography.body, { color: colors.inkSoft }]}>Ảnh và lời trong sổ sẽ ra ngoài hội, kể cả ảnh bạn bè đã đăng. Bạn có thể gỡ ảnh hoặc cất sổ về riêng tư bất cứ lúc nào.</Text> : null}
        <RudiButton label={audience === "private" ? "Lưu riêng tư" : "Đăng sổ công khai"} loading={busy} onPress={() => void keep()} />
        <RudiButton label="Chọn lại chất liệu" variant="ghost" onPress={() => { pending.current = null; setPhase("sources"); }} />
        {saved ? <RudiButton label="Mở bản đã lưu để đối chiếu" variant="ghost" onPress={() => router.push(`/diaries/${saved.id}` as never)} /> : null}
      </>}
    </Animated.View> : null}
  </RudiScreen>;
}
const styles = StyleSheet.create({ section: { gap: 20, paddingBottom: 24 }, invitation: { padding: 24, gap: 14 }, grid: { flexDirection: "row", flexWrap: "wrap", gap: 10 }, tile: { width: "47%", gap: 5, padding: 4 }, thumb: { width: "100%", aspectRatio: 1 }, editPage: { gap: 12, paddingVertical: 18, borderTopWidth: StyleSheet.hairlineWidth } });
