import { Image } from "expo-image";
import { useRouter } from "expo-router";
import { useEffect, useRef, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import Animated, { useAnimatedStyle, useSharedValue, withTiming } from "react-native-reanimated";
import { newAttempt, taiAnhCaNhanLen } from "../../api";
import { chonAnh, nenVaDung, boAnh } from "../ky-niem/chon-anh";
import { typography, useRudiTheme } from "../theme";
import { Heading, RudiButton, RudiScreen, Segmented, TopBar } from "../ui";
import { ErrorState } from "../ui/ErrorState";
import { EmptyState } from "../ui/EmptyState";
import { Canh } from "../ui/art/Canh";
import { voiTenTrang } from "./ten-trang";
import { SkeletonGroup, SkeletonCard } from "../ui/Skeleton";
import { Sheet } from "../ui/Sheet";
import { useMotion } from "../ui/useMotion";
import { TrangSo } from "../ui/TrangSo";
import { StampButton } from "../ui/StampButton";
import { ONhapMuc } from "../ui/ONhapMuc";
import { CauTaiCho } from "../ui/CauTaiCho";
import { ngayKieuViet } from "../chat/to-hen-chung";
import { useNhuongChoNep } from "../nep/NepProvider";
import { BookView } from "./BookView";
import { diaryFailure, endingWait, togglePhoto, buildDiary, diaryImage, endOuting, initialPhotos, includeSavedPhotos, movePage, publishedPhoto, readDiary, readEnding, readJob, readSources, saveDiary, selectedBundle, type Diary, type DiaryDocument, type DiaryKind, type DiarySource, type Ending } from "./api";
import { toggleState } from "../../ui/a11y";

type FailureSite = "load" | "close" | "compose" | "save" | "upload";
type Failure = ReturnType<typeof diaryFailure> & { site: FailureSite };
const failureID: Record<FailureSite, string> = { load: "diary-load-error", close: "diary-close-error", compose: "diary-compose-error", save: "diary-save-error", upload: "diary-upload-error" };

export function EndingScreen({ person, outing }: { person: string; outing: string }) {
  const insets = useSafeAreaInsets();
  const router = useRouter(); const { colors } = useRudiTheme(); const motion = useMotion();
  const [ending, setEnding] = useState<Ending | null>(null); const [kind, setKind] = useState<DiaryKind>("moment");
  const [source, setSource] = useState<DiarySource | null>(null); const [selected, setSelected] = useState<string[]>([]);
  const [excerpt, setExcerpt] = useState(""); const [document, setDocument] = useState<DiaryDocument | null>(null);
  const [saved, setSaved] = useState<Diary | null>(null); const [audience, setAudience] = useState<Diary["audience"]>("private");
  const [phase, setPhase] = useState<"loading" | "ending" | "sources" | "building" | "editing" | "saved">("loading");
  const [busy, setBusy] = useState(false); const [error, setError] = useState<Failure | null>(null);
  const operation = useRef(false);
  const [edit, setEdit] = useState(false); const [pickTarget, setPickTarget] = useState<number | "cover" | null>(null);
  const mounted = useRef(true); const pending = useRef<{ digest: string; id: string } | null>(null);
  const retry = useRef<(() => Promise<void>) | null>(null);
  const turn = useSharedValue(0); const bookStyle = useAnimatedStyle(() => ({ transform: [{ translateY: turn.value * 14 }, { scale: 1 - turn.value * 0.025 }] }));
  useNhuongChoNep(pickTarget !== null);
  const report = (e: unknown, site: FailureSite) => {
    if (!mounted.current) return;
    const failure = diaryFailure(e);
    setError({ ...failure, message: site === "save" ? failure.code === "diary_unavailable" ? "Chưa xác nhận được lần lưu. Bản đang viết vẫn ở đây; bạn thử lại nhé." : `Lần lưu chưa được xác nhận; bản đang viết vẫn ở đây. ${failure.message}` : failure.message, site });
  };
  const actionError = (site: FailureSite) => <CauTaiCho testID={failureID[site]} cau={error?.site === site ? error.message : null} hanhDong={error?.site === site && error.retryable ? { label: "Thử lại", onPress: () => { if (!operation.current) void retry.current?.(); } } : undefined} />;
  async function loadSources() { const s = await readSources(person, outing); if (!mounted.current) return; setSource(s); setSelected(initialPhotos(s)); }
  async function load() {
    retry.current = load;
    setError(null); setPhase("loading");
    try {
      const e = await readEnding(person, outing); if (!mounted.current) return; setEnding(e); setKind(e.kind);
      if (e.ended_at) {
        await loadSources();
        if (e.diary_id) { const b = await readDiary(person, e.diary_id); if (!mounted.current) return; setSaved(b); setDocument(voiTenTrang(b.document)); setAudience(b.audience); setSource((s) => s ? includeSavedPhotos(s, b) : s); setPhase("editing"); }
        else setPhase("sources");
      } else setPhase("ending");
    } catch (e) { if (mounted.current) report(e, "load"); }
  }
  useEffect(() => { mounted.current = true; void load(); return () => { mounted.current = false; }; }, [person, outing]);
  async function close() {
    if (operation.current) return;
    operation.current = true; retry.current = close; setBusy(true); setError(null);
    try {
      const e = await endOuting(person, outing, kind); if (!mounted.current) return;
      setEnding(e); await loadSources(); if (mounted.current) setPhase("sources");
    } catch (e) {
      report(e, "close");
      if (mounted.current && ["outing_not_started", "organizer_required"].includes(diaryFailure(e).code ?? "")) setEnding((e) => e ? { ...e, can_end: false } : e);
    } finally { operation.current = false; if (mounted.current) setBusy(false); }
  }
  const bundle = source ? selectedBundle(source, selected, excerpt.trim() ? [excerpt.trim()] : []) : null;
  async function compose(ai: boolean) {
    retry.current = () => compose(ai);
    if (!bundle || operation.current) return; operation.current = true; setError(null); setPhase("building");
    const digest = JSON.stringify([bundle, ai]); if (pending.current?.digest !== digest) pending.current = { digest, id: newAttempt().key };
    try {
      let j = await buildDiary(person, outing, bundle, ai, pending.current.id);
      for (let i = 0; i < 90 && (j.status === "queued" || j.status === "running"); i++) {
        await new Promise((resolve) => setTimeout(resolve, 1200)); if (!mounted.current) return; j = await readJob(person, j.id);
      }
      if (!mounted.current) return;
      if (j.status !== "succeeded" || !j.result) { pending.current = null; throw new Error("Nếp chưa xếp xong cuốn sổ. Chất liệu vẫn còn; bạn thử lại hoặc tự xếp trang nhé."); }
      setDocument(voiTenTrang(j.result)); setPhase("editing"); setEdit(false); turn.value = 1; turn.value = withTiming(0, motion.timing("shared", "decelerate"));
    } catch (e) { if (mounted.current) { report(e, "compose"); setPhase("sources"); } } finally { operation.current = false; }
  }
  async function keep() {
    retry.current = keep;
    if (!document || operation.current) return; operation.current = true; setBusy(true); setError(null);
    try { const b = await saveDiary(person, outing, document, saved?.revision ?? 0, audience); if (!mounted.current) return; setSaved(b); setAudience(b.audience); setDocument(b.document); setPhase("saved"); motion.haptic.success(); turn.value = 1; turn.value = withTiming(0, motion.timing("celebrate", "decelerate")); }
    catch (e) { report(e, "save"); } finally { operation.current = false; if (mounted.current) setBusy(false); }
  }
  async function upload() {
    if (operation.current) return;
    operation.current = true; retry.current = upload;
    setBusy(true); setError(null);
    let chosen: Awaited<ReturnType<typeof chonAnh>> = null;
    try {
      chosen = await chonAnh();
      if (!chosen) return;
      const p = await nenVaDung(chosen, (photo) => taiAnhCaNhanLen(photo, person));
      if (!mounted.current) return;
      const id = p.url.split("/").pop()!;
      setSource((s) => s ? { ...s, photos: [...s.photos, { id, url: p.url, caption: "", day: s.ends_on }] } : s);
      setSelected((ids) => togglePhoto(ids, id, 40));
    } catch (e) { report(e, "upload"); if (chosen) await boAnh(chosen); } finally { operation.current = false; if (mounted.current) setBusy(false); }
  }
  function choosePhoto(id: string) {
    if (!document || pickTarget === null || operation.current) return;
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
  const waiting = ending ? endingWait(ending) : null;
  const keepControls = <View testID="diary-save-section" style={[styles.saveSection, { borderColor: colors.line }]}>
        <Heading size="h2" title="Giữ lại câu chuyện" subtitle="Chọn ai được xem, rồi lưu cuốn sổ." />
        <Segmented items={["Chỉ mình tôi", "Công khai"]} selected={audience === "public" ? 1 : 0} onSelect={(i) => { if (!operation.current) { setAudience(i ? "public" : "private"); setError(null); } }} />
        <Text style={[typography.body, { color: colors.inkSoft }]}>{audience === "public" ? "Ảnh và lời trong sổ sẽ ra ngoài hội, kể cả ảnh bạn bè đã đăng. Bạn có thể gỡ ảnh hoặc cất sổ về riêng tư bất cứ lúc nào." : "Sổ nằm trên tường bạn. Chỉ mình bạn mở được."}</Text>
  </View>;
  const saveAction = phase === "editing" && document ? <View style={styles.saveAction}>
    {actionError("save")}
    <StampButton testID="diary-save-button" size="vua" tilt={-1} label={audience === "private" ? "Lưu riêng tư" : "Đăng sổ công khai"} loading={busy} onPress={() => void keep()} />
  </View> : undefined;
  return <RudiScreen footer={saveAction} footerInset={insets.bottom} cot="form" cuonVeDau={`${phase}-${edit}`} header={<TopBar title={phase === "loading" || phase === "ending" ? "Trang cuối" : kind === "trip" ? "Sổ chuyến đi" : "Khoảnh khắc"} />} testID="diary-ending-screen" avoidKeyboard overlay={<Sheet open={pickTarget !== null} onClose={() => setPickTarget(null)} accessibilityLabel="Chọn ảnh cho trang"><View style={styles.section}>
    <Heading size="h2" title={pickTarget === "cover" ? "Tấm nào mở đầu câu chuyện?" : "Ảnh cho trang này"} subtitle={pickTarget === "cover" ? "Chọn một tấm làm bìa. Những ảnh khác vẫn ở trong sổ." : "Tối đa bốn ảnh mỗi trang. Chạm ảnh đã chọn để bỏ."} />
    <View style={styles.grid}>{source?.photos.map((p) => <Pressable key={p.id} accessibilityRole={pickTarget === "cover" ? "radio" : "checkbox"} aria-checked={pickTarget === "cover" ? document?.cover_id === p.id : typeof pickTarget === "number" && document?.pages[pickTarget]?.photo_ids.includes(p.id) === true} accessibilityLabel={`Chọn ảnh ${p.caption || p.day}`} onPress={() => choosePhoto(p.id)} style={styles.tile}><Image source={photo(p.id)} cachePolicy="none" contentFit="cover" style={styles.thumb} /><Text style={[typography.caption, { color: colors.ink }]}>{pickTarget === "cover" && document?.cover_id === p.id ? "Bìa hiện tại" : typeof pickTarget === "number" && document?.pages[pickTarget]?.photo_ids.includes(p.id) ? "Đã chọn" : ngayKieuViet(p.day)}</Text></Pressable>)}</View>
    <RudiButton label="Xong phần ảnh" onPress={() => setPickTarget(null)} /></View>
  </Sheet>}>
    {error?.site === "load" ? error.retryable ? <ErrorState title="Chưa mở được trang cuối" body={error.message} onRetry={() => void load()} /> : <EmptyState kind="failure" title="Chưa mở được trang cuối" body={error.message} action={{ label: "Về lịch hẹn", onPress: () => router.replace("/plan") }} illustration={<Canh id="chua-doc-duoc" width={168} />} /> : null}
    {phase === "loading" && !error ? <SkeletonGroup><SkeletonCard media={240} lines={3} /></SkeletonGroup> : null}
    {phase === "ending" ? <View style={styles.section}>
      <Heading title={waiting === "future" ? "Cuộc đi còn ở phía trước." : waiting === "organizer" ? "Trang cuối đang chờ cả hội." : "Khép cuộc đi, giữ câu chuyện."} subtitle={waiting === "future" ? "Cứ dành thời gian cho cuộc hẹn. Trang cuối vẫn ở đây khi bạn trở về." : waiting === "organizer" ? "Người tổ chức sẽ khép cuộc đi. Sau đó bạn tự chọn những điều muốn giữ." : "Một khoảnh khắc nhỏ, hay cả cuốn sổ cho những ngày đi xa?"} />
      <TrangSo tone="accent" ke={false} style={styles.invitation}>
        <Text style={[typography.h1, { color: colors.ink }]}>{ending?.title}</Text>
        {ending ? <View style={[styles.dates, { borderColor: colors.line }]}>
          <View style={styles.date}><Text style={[typography.caption, { color: colors.inkSoft }]}>{ending.starts_on === ending.ends_on ? "Ngày hẹn" : "Bắt đầu"}</Text><Text style={[typography.title, { color: colors.ink }]}>{ngayKieuViet(ending.starts_on)}</Text></View>
          {ending.starts_on !== ending.ends_on ? <View style={styles.date}><Text style={[typography.caption, { color: colors.inkSoft }]}>Ngày về</Text><Text style={[typography.title, { color: colors.ink }]}>{ngayKieuViet(ending.ends_on)}</Text></View> : null}
        </View> : null}
      </TrangSo>
      {ending?.can_end ? <View style={styles.decision}><Heading size="h2" title="Bạn muốn giữ theo cách nào?" />
      <Segmented items={["Khoảnh khắc", "Sổ chuyến đi"]} selected={kind === "trip" ? 1 : 0} onSelect={(i) => { if (!operation.current) setKind(i ? "trip" : "moment"); }} />
      <Text style={[typography.body, { color: colors.inkSoft }]}>Khép cuộc đi cho cả hội. Sau đó, mỗi người tự chọn những điều muốn giữ trên tường mình.</Text>
      <Text style={[typography.note, { color: colors.inkSoft }]}>{kind === "trip" ? "Mang về vài tấm ảnh. Giữ lại cả một hành trình." : "Không cần đi xa mới có một ngày đáng nhớ."}</Text>
      <StampButton size="vua" tilt={-1} label="Khép cuộc đi" loading={busy} onPress={() => void close()} /></View> : <RudiButton label="Về cuộc hẹn" variant="outline" onPress={() => router.replace(`/outings/${outing}` as never)} />}
      {actionError("close")}
    </View> : null}
    {phase === "sources" && source && bundle ? <View style={styles.section}>
      <Heading title="Mang theo điều gì vào sổ?" subtitle="Ảnh đã được chọn theo ngày đi. Bạn xem lại nhé, nhất là khi hai cuộc hẹn trùng nhau." />
      <Text style={[typography.label, { color: colors.ink }]}>{selected.length} / 40 ảnh đã chọn</Text>
      <View style={styles.grid}>{source.photos.map((p) => <Pressable key={p.id} {...toggleState("checkbox", selected.includes(p.id), () => setSelected((ids) => togglePhoto(ids, p.id, 40)))} accessibilityLabel={`Giữ ảnh ${p.caption || p.day}`} onPress={() => setSelected((ids) => togglePhoto(ids, p.id, 40))} style={[styles.tile, { borderColor: selected.includes(p.id) ? colors.accent : colors.line, borderWidth: 2 }]}>
        <Image source={p.url ? diaryImage(person, p.url) : undefined} cachePolicy="none" contentFit="cover" style={styles.thumb} /><Text style={[typography.caption, { color: colors.ink }]}>{selected.includes(p.id) ? "Đã chọn · " : ""}{ngayKieuViet(p.day)}</Text>{p.caption ? <Text numberOfLines={2} style={[typography.caption, { color: colors.inkSoft }]}>{p.caption}</Text> : null}
      </Pressable>)}</View>
      <RudiButton label="Thêm ảnh từ máy" variant="outline" loading={busy} onPress={() => void upload()} />
      {actionError("upload")}
      <ONhapMuc testID="diary-excerpt-input" label="Một đoạn chuyện muốn gửi cùng" placeholder="Dán trích đoạn bạn chọn, hoặc để trống" multiline maxLength={2000} value={excerpt} onChangeText={setExcerpt} helper="Chat không được tự đọc. Chỉ đoạn bạn đặt ở đây sẽ đi cùng ảnh." />
      <Heading size="h2" title="Nếp sẽ nhận đúng phần này" />
      <Text style={[typography.body, { color: colors.inkSoft }]}>{bundle.title}{"\n"}{ngayKieuViet(bundle.starts_on)} · {ngayKieuViet(bundle.ends_on)}{"\n"}{bundle.photos.length} ảnh · {bundle.excerpts.length} trích đoạn</Text>
      {bundle.places.length ? <Text style={[typography.body, { color: colors.inkSoft }]}>Nơi đã check-in: {bundle.places.join(", ")}</Text> : null}
      {bundle.excerpts.map((v, i) => <Text key={i} style={[typography.body, { color: colors.ink }]}>{v}</Text>)}
      <Text style={[typography.caption, { color: colors.inkSoft }]}>Bấm dựng sổ để chia sẻ phần này với AI. Bản dựng chỉ mình bạn xem; chưa có gì được đăng.</Text>
      {actionError("compose")}
      <StampButton size="vua" tilt={-1} label="Dựng sổ cùng Nếp" disabled={busy} lyDo={busy ? "Ảnh đang được thêm vào sổ." : undefined} onPress={() => void compose(true)} />
      <RudiButton label="Tự xếp trang, không gửi AI" variant="outline" disabled={busy} lyDo={busy ? "Ảnh đang được thêm vào sổ." : undefined} onPress={() => void compose(false)} />
      {document ? <RudiButton label="Về bản đang sửa" variant="ghost" onPress={() => { setError(null); setPhase("editing"); }} /> : null}
    </View> : null}
    {phase === "building" ? <View style={styles.section}><Heading title="Những mẩu chuyện đang thành trang…" subtitle="Ảnh và lời kể đang được xếp vào sổ. Chưa có gì được đăng lên tường." /><SkeletonGroup><SkeletonCard media={220} lines={3} /></SkeletonGroup></View> : null}
    {(phase === "editing" || phase === "saved") && document ? <Animated.View style={[styles.section, bookStyle]}>
      {phase === "saved" ? <Heading title="Đã giữ lại một cuộc đi." subtitle={audience === "private" ? "Cuốn sổ nằm trên tường bạn, chỉ mình bạn mở được." : "Cuốn sổ đã có trên tường bạn, mọi người trong app có thể xem."} /> : null}
      {!edit || phase === "saved" ? <BookView kind={kind} document={document} photo={photo} /> : <Heading title="Viết lại theo cách mình nhớ" subtitle="Đổi lời, chọn ảnh. Cuốn sổ vẫn là câu chuyện của bạn." />}
      {document.ai_generated ? <Text style={[typography.caption, { color: colors.inkSoft }]}>Nếp giúp xếp ảnh và viết lời từ phần bạn chia sẻ. Bạn là người giữ lời cuối.</Text> : null}
      {phase === "saved" ? <><RudiButton label="Về tường nhà mình" onPress={() => router.replace(`/people/${person}` as never)} /><RudiButton label="Mở lại cuốn sổ" variant="outline" onPress={() => setPhase("editing")} /></> : <>
        <RudiButton label={edit ? "Xem như người đọc" : "Sửa theo cách mình nhớ"} variant="outline" disabled={busy} lyDo={busy ? "Đợi thao tác hiện tại hoàn tất nhé." : undefined} onPress={() => { setError(null); setEdit(!edit); }} />
        {edit ? <View style={styles.section} pointerEvents={busy ? "none" : "auto"}>
          <ONhapMuc testID="diary-title-input" label="Tên cuốn sổ" editable={!busy} value={document.title} maxLength={200} onChangeText={(title) => setDocument({ ...document, title })} />
          <ONhapMuc label="Lời mở" editable={!busy} value={document.subtitle} maxLength={500} multiline onChangeText={(subtitle) => setDocument({ ...document, subtitle })} />
          <RudiButton label="Thay ảnh bìa" variant="outline" onPress={() => setPickTarget("cover")} />
          <RudiButton label="Thêm ảnh từ máy" variant="ghost" loading={busy} onPress={() => void upload()} />
          {actionError("upload")}
          {document.pages.map((p, i) => <View key={i} style={[styles.editPage, { borderColor: colors.line }]}>
            <Text style={[typography.h2, { color: colors.ink }]}>Trang {i + 1}</Text>
            <ONhapMuc label="Tên trang" editable={!busy} value={p.heading} maxLength={200} onChangeText={(heading) => setDocument({ ...document, pages: document.pages.map((p, j) => j === i ? { ...p, heading } : p) })} />
            <ONhapMuc label="Chuyện của trang" editable={!busy} value={p.text} multiline maxLength={2000} onChangeText={(text) => setDocument({ ...document, pages: document.pages.map((p, j) => j === i ? { ...p, text } : p) })} />
            <RudiButton label={`Thay ảnh trang ${i + 1}`} variant="outline" onPress={() => setPickTarget(i)} />
            {i > 0 ? <RudiButton label={`Đưa trang ${i + 1} lên trước`} accessibilityLabel={`Đưa trang ${i + 1} lên trước`} variant="ghost" onPress={() => setDocument(movePage(document, i, i - 1))} /> : null}
            <RudiButton label="Bỏ trang này" variant="ghost" disabled={document.pages.length === 1} lyDo={document.pages.length === 1 ? "Giữ ít nhất một trang trong sổ." : undefined} onPress={() => setDocument({ ...document, pages: document.pages.filter((_, j) => j !== i) })} />
          </View>)}
          <RudiButton label="Thêm một trang viết" variant="outline" disabled={document.pages.length >= 24} lyDo={document.pages.length >= 24 ? "Cuốn sổ đã đủ 24 trang." : undefined} onPress={() => setDocument({ ...document, pages: [...document.pages, { layout: "note", heading: "", text: "", photo_ids: [] }] })} />
        </View> : null}
        {keepControls}
        <RudiButton label="Chọn lại chất liệu" variant="ghost" disabled={busy} lyDo={busy ? "Đợi thao tác hiện tại hoàn tất nhé." : undefined} onPress={() => { pending.current = null; setPhase("sources"); }} />
        {saved ? <RudiButton label="Mở bản đã lưu để đối chiếu" variant="ghost" onPress={() => router.push(`/diaries/${saved.id}` as never)} /> : null}
      </>}
    </Animated.View> : null}
  </RudiScreen>;
}
const styles = StyleSheet.create({ section: { gap: 20, paddingBottom: 24, width: "100%", maxWidth: 560, alignSelf: "center" }, invitation: { gap: 20 }, decision: { gap: 16 }, dates: { flexDirection: "row", flexWrap: "wrap", gap: 20, paddingTop: 16, borderTopWidth: StyleSheet.hairlineWidth }, date: { minWidth: 120, flex: 1, gap: 4 }, saveAction: { gap: 12, paddingVertical: 12 }, saveSection: { gap: 16, paddingTop: 24, borderTopWidth: StyleSheet.hairlineWidth }, grid: { flexDirection: "row", flexWrap: "wrap", gap: 10 }, tile: { width: "47%", gap: 5, padding: 4 }, thumb: { width: "100%", aspectRatio: 1 }, editPage: { gap: 12, paddingVertical: 18, borderTopWidth: StyleSheet.hairlineWidth } });
