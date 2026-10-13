import { translatedAsActor, newAttempt } from "../../api";
import { COMMUNITY_ERRORS } from "./api";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { Keyboard, Pressable, StyleSheet, View, useWindowDimensions, type LayoutChangeEvent, type TextInput, type TextInputProps } from "react-native";
import { docNhomCuaToi, type NhomTomTat } from "../../phien";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { Field, RudiButton, RudiScreen, TopBar } from "../ui";
import { createPost, getPost, type Audience, type Media } from "./api";
import { loiChuDe } from "./chu-de";
import { CauTaiCho } from "../ui/CauTaiCho";
import { MediaPicker, type SelectedFile } from "./MediaPicker";
import { boNhapBai, docNhapBai, ghiNhapBai, khoaNhapBai, phienNhapBai, type NhapBai } from "./ban-nhap";
import { MentionPicker } from "./MentionPicker";
import { coTuongNhom } from "../so/ban-tinh";
import { KHONG_VIEN_WEB } from "../ui/khong-vien-web";
import { useNativeFontScale } from "../chia-bill/useBillFontScale";
import { NativeFontMetricsProvider, Text, TextInput as NativeMetricTextInput, useNativeTextInputMetrics } from "../ui/NativeFontMetrics";
import { toggleState } from "../../ui/a11y";
import { SkeletonLines } from "../ui/Skeleton";
import { AvatarNguoi } from "../ui/AvatarNguoi";
import { RudiIcon, type IconName } from "../ui/RudiIcon";
/** Who reads a story: the title and the line under it, one place for both forms. */
const NGUOI_DOC: Record<Audience, [string, string]> = {
  friends: ["Riêng tư · Bạn bè", "Chỉ bạn và những người đang kết bạn với bạn."],
  public: ["Công khai · Cộng đồng", "Sau khi được duyệt, bài xuất hiện trên tường và cộng đồng."],
  only_me: ["Chỉ mình tôi", "Giữ riêng câu chuyện này cho bạn."],
  group: ["Một nhóm", "Chỉ thành viên hiện tại của nhóm được xem."],
};

type DraftContent = { body: string; audience: Audience; topics: string; mentions: string[]; media: Media[]; group: string | null };
// Retry equality follows the exact outgoing fields, before server normalization.
function postContent(draft: DraftContent) {
    return { body: draft.body, audience: draft.audience, topics: draft.topics.split(",").map((s) => s.trim()).filter(Boolean), mentions: draft.mentions, media_ids: draft.media.map((m) => m.id), context_id: draft.audience === "group" ? draft.group : null };
}

function draftSignature(draft: NhapBai | null) {
    return draft ? JSON.stringify([draft.body, draft.audience, draft.topics, draft.mentions, draft.media, draft.group, draft.revision, draft.logical_id, draft.pendingFile]) : null;
}

export function Composer() {
    const { phien } = useRudiSession();
    const params = useLocalSearchParams<{ edit?: string; wall?: string }>();
    const scope = khoaNhapBai(params.edit, Boolean(params.wall));
    const lease = phien ? phienNhapBai(phien.person_id) : null;
    return <ComposerForm key={JSON.stringify([phien?.person_id ?? null, scope, lease])} />;
}

function ComposerForm() {
    const router = useRouter();
    const insets = useSafeAreaInsets();
    const { phien } = useRudiSession();
    const { colors } = useRudiTheme();
    const { height, width, fontScale: windowFontScale } = useWindowDimensions();
    const nativeFont = useNativeFontScale();
    const fontScale = nativeFont.status === "available" ? nativeFont.fontScale : windowFontScale;
    const expanded = width >= 600;
    const params = useLocalSearchParams<{
        edit?: string;
        wall?: string;
    }>();
    const person = phien?.person_id ?? "";
    const scope = khoaNhapBai(params.edit, Boolean(params.wall));
    const [lease] = useState(() => phienNhapBai(person));
    const [nhapCu, setNhapCu] = useState(() => docNhapBai(person, scope));
    const [body, setBody] = useState(params.edit ? "" : nhapCu?.body ?? "");
    const [audience, setAudience] = useState<Audience>(params.edit ? "public" : nhapCu?.audience ?? (params.wall ? "friends" : "public"));
    const [topics, setTopics] = useState(params.edit ? "" : nhapCu?.topics ?? "");
    const [mentions, setMentions] = useState<string[]>(params.edit ? [] : nhapCu?.mentions ?? []);
    const [media, setMedia] = useState<Media[]>(params.edit ? [] : nhapCu?.media ?? []);
    const [pendingFile, setPendingFile] = useState<SelectedFile | null>(params.edit ? null : nhapCu?.pendingFile ?? null);
    const [groupError, setGroupError] = useState<string | null>(null);
    const [groupLoading, setGroupLoading] = useState(false);
    const [groupRetry, setGroupRetry] = useState(0);
    const [groups, setGroups] = useState<NhomTomTat[]>([]);
    const [group, setGroup] = useState<string | null>(params.edit ? null : nhapCu?.group ?? null);
    const [audienceOpen, setAudienceOpen] = useState(false);
    const [topicsOpen, setTopicsOpen] = useState(false);
    const [mentionsOpen, setMentionsOpen] = useState(false);
    const [busy, setBusy] = useState(false);
    const [uploading, setUploading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [revision, setRevision] = useState(params.edit ? 0 : nhapCu?.revision ?? 0);
    const [editReady, setEditReady] = useState(!params.edit);
    const [loading, setLoading] = useState(Boolean(params.edit));
    const [retry, setRetry] = useState(0);
    const [dirty, setDirty] = useState(!params.edit && nhapCu !== null);
    const [conflict, setConflict] = useState(false);
    const sending = useRef(false);
    const uploadingNow = useRef(false);
    const submitted = useRef(false);
    const alive = useRef(true);
    const topicsInput = useRef<TextInput>(null);
    const topicsTools = useRef<View>(null);
    const topicsFrame = useRef<number | null>(null);
    const topicsMeasurement = useRef<object | null>(null);
    const topicsRequest = useRef<object | null>(null);
    const topicsFocusedNow = useRef(false);
    const [topicsFocused, setTopicsFocused] = useState(false);
    const [topicsClearance, setTopicsClearance] = useState<{ generation: object; value: number } | null>(null);
    const [initialAttempt] = useState(() => nhapCu?.logical_id ?? newAttempt().key);
    const attempt = useRef(initialAttempt);
    const content = useRef<DraftContent>({ body, audience, topics, mentions, media, group });
    const pendingFileNow = useRef(pendingFile);
    const current = () => alive.current && lease !== null && phienNhapBai(person) === lease;
    const snapshot = (): NhapBai => ({ ...content.current, revision, logical_id: attempt.current, pendingFile: pendingFileNow.current });
    const saveDraft = () => {
        if (current() && editReady && !submitted.current) ghiNhapBai(person, scope, snapshot(), lease);
    };
    useLayoutEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
    useEffect(() => {
        if (!phien || !params.edit) return;
        let active = true;
        setLoading(true); setEditReady(false); setError(null);
        void getPost(person, params.edit).then((p) => {
            if (!active || !current()) return;
            // Read permission and the current base before revealing a saved edit.
            const saved = nhapCu;
            const next = { body: saved?.body ?? p.body, audience: saved?.audience ?? p.audience, topics: saved?.topics ?? p.topics.join(", "), mentions: saved?.mentions ?? p.mentions, media: saved?.media ?? p.media, group: saved ? saved.group : p.context_id };
            content.current = next; pendingFileNow.current = saved?.pendingFile ?? null;
            setBody(next.body); setAudience(next.audience); setTopics(next.topics); setMentions(next.mentions); setMedia(next.media); setRevision(saved?.revision ?? p.revision); setGroup(next.group); setPendingFile(saved?.pendingFile ?? null);
            setConflict(saved !== null && (saved.revision !== p.revision || saved.audience !== p.audience || saved.group !== p.context_id));
            setDirty(saved !== null); setEditReady(true);
        }).catch((e) => { if (active && current()) setError(e instanceof Error ? e.message : "Chưa mở được bản viết."); }).finally(() => { if (active && current()) setLoading(false); });
        return () => { active = false; };
    }, [person, params.edit, retry]);
    useEffect(() => {
        if (!current() || !dirty || !editReady || submitted.current) return;
        saveDraft();
    }, [body, audience, topics, mentions, media, group, revision, pendingFile, dirty, editReady]);
    const discardStaleDraft = () => {
        if (!current() || sending.current) return;
        boNhapBai(person, scope, lease);
        setNhapCu(null); setDirty(false); setConflict(false);
        attempt.current = newAttempt().key;
        setRetry((n) => n + 1);
    };
    useEffect(() => {
        if (!phien) return;
        let current = true; setGroupLoading(true); setGroupError(null);
        void docNhomCuaToi(phien.person_id).then((rows) => { if (current) setGroups(rows.filter((g) => g.my_state === "active" && coTuongNhom(g))); }).catch((e) => { if (current) setGroupError(e instanceof Error ? e.message : "Chưa đọc được nhóm của bạn."); }).finally(() => { if (current) setGroupLoading(false); });
        return () => { current = false; };
    }, [phien, groupRetry]);
    const send = async () => {
        const draft = content.current;
        if (!phien || !current() || conflict || sending.current || loading || uploadingNow.current || pendingFileNow.current !== null || !draft.body.trim() || loiChuDe(draft.topics) || (draft.audience === "group" && !draft.group) || !editReady)
            return;
        sending.current = true;
        setBusy(true);
        setError(null);
        const logical = attempt.current;
        const input = { logical_id: logical, ...postContent(draft), revision };
        // Publish before transport: an immediate ACK may precede the passive save.
        const sentDraft = snapshot();
        const sentSignature = draftSignature(sentDraft);
        ghiNhapBai(person, scope, sentDraft, lease);
        try {
            const post = params.edit ? await translatedAsActor<{
                id: string;
            }>(COMMUNITY_ERRORS, `/v2/community/posts/${params.edit}`, {
                actorId: phien.person_id,
                method: "PUT",
                body: input,
                attempt: newAttempt()
            }) : await createPost(phien.person_id, input);
            // A remounted editor may add pending metadata without changing this payload ID.
            if (draftSignature(docNhapBai(person, scope)) === sentSignature) boNhapBai(person, scope, lease, logical);
            if (!current() || attempt.current !== logical) return;
            submitted.current = true;
            router.replace(`/community/posts/${post.id}` as never);
        }
        catch (e) {
            if (current()) setError(e instanceof Error ? e.message : "Chưa gửi được bài. Bản viết vẫn ở đây.");
        }
        finally {
            if (current()) { sending.current = false; setBusy(false); }
        }
    };
    const change = (patch: Partial<DraftContent>) => {
        if (!current() || sending.current) return false;
        const next = { ...content.current, ...patch };
        if (JSON.stringify(postContent(next)) !== JSON.stringify(postContent(content.current))) {
            attempt.current = newAttempt().key;
        }
        // Keep explicit UI/metadata edits even when their outgoing payload is equal.
        content.current = next; setDirty(true); saveDraft();
        setBody(next.body); setAudience(next.audience); setTopics(next.topics); setMentions(next.mentions); setMedia(next.media); setGroup(next.group);
        return true;
    };
    const rememberFile = (file: SelectedFile | null) => {
        if (!current() || sending.current) return;
        if (pendingFileNow.current === file) return;
        pendingFileNow.current = file; setDirty(true); saveDraft(); setPendingFile(file);
    };
    // The topic box checks itself as the server will (QA UI-133): the sentence
    // sits under the box while typing, and «Gửi» waits with the reason.
    const loiChuDeGo = loiChuDe(topics);
    const counterPresent = Boolean(phien) && !loading && editReady;
    const counterGeneration = useMemo(() => ({}), [counterPresent, nativeFont, fontScale, width]);
    const [counterLayout, setCounterLayout] = useState<{ generation: object; height: number | null }>(() => ({ generation: counterGeneration, height: null }));
    useLayoutEffect(() => {
        // A same-size native text need not emit another layout after metrics change.
        setCounterLayout((previous) => previous.generation === counterGeneration ? previous : { generation: counterGeneration, height: counterPresent ? previous.height : null });
    }, [counterGeneration, counterPresent]);
    const recordCounterLayout = (event: LayoutChangeEvent) => {
        const { width: counterWidth, height: counterHeight } = event.nativeEvent.layout;
        if (!counterPresent || !current() || !Number.isFinite(counterWidth) || counterWidth <= 0 || !Number.isFinite(counterHeight) || counterHeight <= 0) return;
        setCounterLayout((previous) => !current() || previous.generation !== counterGeneration || previous.height === counterHeight ? previous : { generation: counterGeneration, height: counterHeight });
    };
    const topicsPresent = Boolean(phien) && !loading && editReady && Boolean(topicsOpen || topics || loiChuDeGo);
    const topicsGeneration = useMemo(() => ({}), [topicsPresent, topicsFocused, topics, loiChuDeGo, nativeFont, fontScale, width]);
    const measureTopics = () => {
        if (!topicsPresent || !topicsFocused || topicsMeasurement.current !== topicsGeneration || !current()) return;
        if (topicsFrame.current !== null) cancelAnimationFrame(topicsFrame.current);
        const input = topicsInput.current;
        const tools = topicsTools.current;
        if (!input || !tools) return;
        const request = {};
        topicsRequest.current = request;
        const active = () => topicsRequest.current === request && topicsMeasurement.current === topicsGeneration && topicsFocusedNow.current && current() && topicsInput.current === input && topicsTools.current === tools && input.isFocused();
        const clear = () => { if (active()) setTopicsClearance((previous) => active() ? null : previous); };
        // Native layout includes the whole helper/error, without guessing its lines.
        topicsFrame.current = requestAnimationFrame(() => {
            if (!active()) return;
            topicsFrame.current = requestAnimationFrame(() => {
                if (!active()) return;
                topicsFrame.current = null;
                tools.measure((_x, _y, toolsWidth, toolsHeight) => {
                    if (!active()) return;
                    if (!Number.isFinite(toolsWidth) || toolsWidth <= 0 || !Number.isFinite(toolsHeight) || toolsHeight <= 0) { clear(); return; }
                    input.measureLayout(tools, (x, y, inputWidth, inputHeight) => {
                        if (!active()) return;
                        const tail = toolsHeight - y - inputHeight;
                        if (![x, y, inputWidth, inputHeight, tail].every(Number.isFinite) || x < 0 || y < 0 || inputWidth <= 0 || inputHeight <= 0 || tail < 0) { clear(); return; }
                        const value = tail + 16;
                        if (!Number.isFinite(value)) { clear(); return; }
                        setTopicsClearance((previous) => !active() || (previous?.generation === topicsGeneration && previous.value === value) ? previous : { generation: topicsGeneration, value });
                    }, clear);
                });
            });
        });
    };
    useLayoutEffect(() => {
        topicsMeasurement.current = topicsPresent && topicsFocused ? topicsGeneration : null;
        measureTopics();
        return () => {
            if (topicsMeasurement.current === topicsGeneration) topicsMeasurement.current = null;
            topicsRequest.current = null;
            if (topicsFrame.current !== null) { cancelAnimationFrame(topicsFrame.current); topicsFrame.current = null; }
        };
    }, [topicsGeneration]);
    const bodyClearance = counterLayout.height === null ? 16 : counterLayout.height + styles.writing.gap + 16;
    const focusBottomInset = topicsFocused ? topicsClearance?.generation === topicsGeneration ? topicsClearance.value : 16 : bodyClearance;
    if (!phien)
        return <RudiScreen><TopBar title="Kể một khoảnh khắc"/><Text style={[typography.body, { color: colors.ink }]}>Đăng nhập để viết câu chuyện của bạn.</Text></RudiScreen>;
    const chooseAudience = (value: Audience) => { if (change({ audience: value })) setAudienceOpen(false); };
    const choice = (value: Audience, title: string, detail: string) => <Pressable key={value} {...toggleState("radio", audience === value, () => chooseAudience(value))} disabled={busy} onPress={() => chooseAudience(value)} style={[styles.audienceChoice, { borderColor: colors.line }]}><View style={styles.audienceChoiceTitle}><Text style={[typography.label, styles.flex, { color: audience === value ? colors.accent : colors.ink }]}>{title}</Text>{audience === value ? <RudiIcon name="checkmark" color={colors.accent} size={20} /> : null}</View><Text style={[typography.note, { color: colors.inkSoft }]}>{detail}</Text></Pressable>;
    if (loading || !editReady) return <RudiScreen material="quiet" cot="form"><TopBar title="Viết tiếp câu chuyện"/>{loading ? <SkeletonLines lines={5}/> : <CauTaiCho cau={error ?? "Chưa mở được bản viết."} hanhDong={{ label: "Thử lại", onPress: () => setRetry((n) => n + 1) }}/>}</RudiScreen>;
    const audienceLabel = { friends: "Bạn bè", public: "Cộng đồng", only_me: "Chỉ mình tôi", group: groups.find((g) => g.id === group)?.display_name ?? "Một nhóm" }[audience];
    const cannotSend = conflict || busy || uploading || pendingFile !== null || !body.trim() || (audience === "group" && !group) || loiChuDeGo !== null;
    return <NativeFontMetricsProvider snapshot={nativeFont}><RudiScreen material="quiet" avoidKeyboard focusBottomInset={focusBottomInset} cot="form" contentStyle={styles.content} bottomInset={0} footerInset={insets.bottom + 12} testID="community-composer" header={<TopBar title={params.edit ? "Sửa bài viết" : "Bài viết mới"}/>} footer={<View style={styles.footer}>
      {/* A refusal from the server is said right above the button that sent it, never at the end of the scroll (QA UI-133). */}
      <CauTaiCho cau={conflict ? "Bài đã thay đổi ở nơi khác. Nháp của bạn vẫn được giữ; mở bản mới sẽ bỏ nháp này." : error} hanhDong={conflict ? { label: "Bỏ nháp và mở bản mới", onPress: discardStaleDraft } : undefined} testID="cong-dong-loi-gui" />
      <RudiButton compact label={busy ? "Đang gửi…" : params.edit ? "Gửi bản sửa" : audience === "public" ? "Gửi lên cộng đồng" : "Đăng lên tường"} loading={busy} disabled={cannotSend} style={cannotSend ? { borderStyle: "solid", borderWidth: 1, backgroundColor: colors.card } : undefined} lyDo={conflict ? "Đối chiếu bản mới trước khi gửi nháp này." : uploading ? "Chờ tệp chuẩn bị xong trước khi gửi." : pendingFile !== null ? "Thử lại hoặc bỏ tệp đang chờ." : !body.trim() ? "Viết nội dung để gửi." : loiChuDeGo !== null ? "Sửa phần chủ đề trước khi gửi." : audience === "group" && !group ? "Chọn một nhóm trước khi gửi." : undefined} onPress={() => void send()}/>
    </View>}>
    <View style={styles.authorCluster}>
    <Pressable accessibilityRole="button" accessibilityLabel={`Người đọc: ${audienceLabel}`} accessibilityState={{ expanded: audienceOpen, disabled: Boolean(params.edit) || busy }} disabled={Boolean(params.edit) || busy} onPress={() => { Keyboard.dismiss(); setAudienceOpen(!audienceOpen); }} style={styles.author}>
      <AvatarNguoi name={phien.profile?.display_name ?? "Bạn"} personId={phien.person_id} size={44} />
      <View style={styles.authorText}>
        <Text style={[typography.title, { color: colors.ink }]}>{phien.profile?.display_name ?? "Bạn"}</Text>
        <View style={[styles.audienceTrigger, { backgroundColor: colors.card, borderColor: colors.lineStrong }]}>
          <RudiIcon name={audience === "public" ? "globe-outline" : audience === "only_me" ? "lock-closed-outline" : "people-outline"} color={colors.inkSoft} size={16} />
          <Text style={[typography.caption, styles.audienceLabel, { color: colors.inkSoft }]}>{audienceLabel}</Text>
          <RudiIcon name={params.edit ? "lock-closed-outline" : "chevron-down"} color={colors.inkSoft} size={16} />
        </View>
      </View>
    </Pressable>
    <Text style={[typography.note, styles.authorDescription, fontScale >= 1.5 && styles.authorDescriptionWide, { color: colors.inkSoft }]}>{audience === "public" ? "Bài được duyệt trước khi xuất hiện trên cộng đồng." : NGUOI_DOC[audience][1]}</Text>
    {/* An edit keeps who reads the story: only that choice is shown, locked,
        with where the change is made (QA UI-091 on Cộng đồng, TC-N14-SUA-KHOA);
        no «Lựa chọn khác» opening more choices that cannot be taken. */}
    {audienceOpen && !params.edit ? <View accessibilityRole="radiogroup" style={[styles.audienceOptions, { backgroundColor: colors.card, borderColor: colors.line }]}>{(Object.keys(NGUOI_DOC) as Audience[]).map((value) => choice(value, ...NGUOI_DOC[value]))}</View> : null}
    {params.edit ? <Text style={[typography.note, { color: colors.inkSoft }]} testID="cong-dong-nguoi-doc-khoa">Bản sửa giữ người đọc như lúc đăng. Muốn đổi người đọc, mở «Thêm lựa chọn cho bài» (⋯) ở trang bài.</Text> : null}
    </View>
    {audience === "group" && !params.edit ? <><CauTaiCho cau={groupError} hanhDong={{ label: "Thử lại nhóm của bạn", onPress: () => setGroupRetry((n) => n + 1) }}/>{groupLoading ? <SkeletonLines lines={2}/> : !groupError && !groups.length ? <Text style={[typography.body, { color: colors.inkSoft }]}>Bạn chưa ở nhóm đang hoạt động có tường chung. Chọn một mức người đọc khác để đăng.</Text> : null}{groups.map((g) => <RudiButton key={g.id} label={g.display_name} variant={group === g.id ? "solid" : "outline"} onPress={() => change({ group: g.id })}/>)}</> : null}
    <View style={styles.stage}>
    <View style={[styles.writing, { borderColor: colors.line }]}>
      <ComposerBodyInput testID="community-body" accessibilityLabel="Nội dung bài đăng" placeholder="Bạn muốn kể điều gì?" placeholderTextColor={colors.inkSoft} multiline maxLength={5000} value={body} editable={!busy} onChangeText={(value) => change({ body: value })} style={[typography.body, KHONG_VIEN_WEB, styles.body, { color: colors.ink, maxHeight: expanded ? undefined : Math.max(420, height), minHeight: Math.min(240, Math.max(112, height * 0.22)) }]} />
      <Text onLayout={recordCounterLayout} style={[typography.note, styles.counter, { color: colors.inkSoft }]}>{body.length}/5000</Text>
    </View>
    <View ref={topicsTools} collapsable={false} onLayout={measureTopics} style={styles.tools}>
      <MediaPicker compact disabled={busy} canEdit={() => current() && !sending.current} initialFile={pendingFile} onFileChange={rememberFile} person={phien.person_id} media={media} onChange={(v) => change({ media: v })} onBusy={(v) => { if (current()) { uploadingNow.current = v; setUploading(v); } }} renderToolbar={(pick, picking) => <View style={[styles.toolRow, fontScale >= 1.5 && styles.toolStack, { backgroundColor: colors.card, borderColor: colors.line }]}>
        <ComposerTool icon="images-outline" label={picking ? "Đang chuẩn bị…" : "Ảnh/video"} accessibilityLabel="Thêm ảnh hoặc video" disabled={busy || picking} onPress={() => { Keyboard.dismiss(); pick(); }} stacked={fontScale >= 1.5} />
        <ComposerTool icon="pricetag-outline" label="Chủ đề" accessibilityLabel={topics ? "Sửa chủ đề" : "Chủ đề"} expanded={topicsOpen} selected={Boolean(topics) || topicsOpen} onPress={() => { Keyboard.dismiss(); setTopicsOpen(!topicsOpen); }} stacked={fontScale >= 1.5} />
        <ComposerTool icon="at-outline" label={mentions.length ? `Gắn bạn (${mentions.length})` : "Gắn bạn"} accessibilityLabel="Tag bạn bè" expanded={mentionsOpen} selected={mentions.length > 0 || mentionsOpen} onPress={() => { Keyboard.dismiss(); setMentionsOpen(!mentionsOpen); }} stacked={fontScale >= 1.5} />
      </View>} />
      <MentionPicker hideTrigger expanded={mentionsOpen} onExpandedChange={setMentionsOpen} person={phien.person_id} selected={mentions} onChange={(v) => change({ mentions: v })}/>
      {topicsOpen || topics || loiChuDeGo ? <Field nativeMetrics oRef={topicsInput} onLayout={measureTopics} onFocus={() => { if (current()) { topicsFocusedNow.current = true; setTopicsFocused(true); measureTopics(); } }} onBlur={() => { topicsFocusedNow.current = false; if (current()) setTopicsFocused(false); }} error={loiChuDeGo} helper={loiChuDeGo === null ? "Tối đa 5 chủ đề, ngăn cách bằng dấu phẩy." : undefined} label="Chủ đề" placeholder="Cà phê, đi bộ, Đà Lạt…" value={topics} onChangeText={(v) => change({ topics: v })}/> : null}
    </View>
    </View>
  </RudiScreen></NativeFontMetricsProvider>;
}

function ComposerBodyInput(props: TextInputProps) {
  const metrics = useNativeTextInputMetrics(props.style, props);
  const original = StyleSheet.flatten(props.style) ?? {};
  const vertical = typeof original.paddingVertical === "number" ? original.paddingVertical : 0;
  const top = typeof original.paddingTop === "number" ? original.paddingTop : vertical;
  const bottom = typeof original.paddingBottom === "number" ? original.paddingBottom : vertical;
  const minimum = metrics === null ? null : Math.max(typeof original.minHeight === "number" ? original.minHeight : 0, metrics.linePitch + top + bottom);
  const geometry = minimum === null ? undefined : {
    minHeight: minimum,
    maxHeight: typeof original.maxHeight === "number" ? Math.max(original.maxHeight, minimum) : original.maxHeight,
  };
  return <NativeMetricTextInput {...props} style={geometry ? [props.style, geometry] : props.style} />;
}

function ComposerTool({ icon, label, accessibilityLabel, onPress, selected, expanded, disabled, stacked }: { icon: IconName; label: string; accessibilityLabel: string; onPress: () => void; selected?: boolean; expanded?: boolean; disabled?: boolean; stacked: boolean }) {
  const { colors } = useRudiTheme();
  return <Pressable accessibilityRole="button" accessibilityLabel={accessibilityLabel} accessibilityState={{ expanded, disabled }} disabled={disabled} onPress={onPress} style={({ pressed }) => [styles.tool, stacked && styles.toolWide, { backgroundColor: selected ? colors.accentSoft : "transparent", opacity: disabled ? 0.5 : pressed ? 0.65 : 1 }]}>
    <RudiIcon name={icon} color={selected ? colors.accent : colors.inkSoft} size={stacked ? 21 : 18} />
    <Text style={[typography.caption, styles.toolLabel, stacked && styles.flex, { color: selected ? colors.accent : colors.inkSoft }]}>{label}</Text>
  </Pressable>;
}

const styles = StyleSheet.create({
  flex: { flex: 1, minWidth: 0 },
  footer: { gap: 8 },
  authorCluster: { gap: 4 },
  author: { flexDirection: "row", alignItems: "flex-start", gap: 12, minHeight: 56 },
  authorText: { flex: 1, minWidth: 0, gap: 2 },
  authorDescription: { marginLeft: 56 },
  authorDescriptionWide: { marginLeft: 0 },
  audienceTrigger: { flexDirection: "row", alignItems: "center", alignSelf: "flex-start", maxWidth: "100%", gap: 6, minHeight: 24, paddingHorizontal: 6, paddingVertical: 2, borderWidth: StyleSheet.hairlineWidth, borderRadius: 6 },
  audienceLabel: { flexShrink: 1, minWidth: 0 },
  audienceOptions: { borderWidth: 1, borderRadius: 12, paddingHorizontal: 14 },
  audienceChoice: { minHeight: 64, paddingVertical: 12, borderBottomWidth: StyleSheet.hairlineWidth, gap: 4 },
  audienceChoiceTitle: { flexDirection: "row", alignItems: "center", gap: 8 },
  // Free height belongs to the writing canvas at every width; long tools stay scrollable.
  content: { flexGrow: 1, flexShrink: 0, flexBasis: "auto" },
  stage: { gap: 8, flexGrow: 1, flexShrink: 0, flexBasis: "auto" },
  writing: { gap: 4, flexGrow: 1, flexShrink: 0, flexBasis: "auto", borderTopWidth: StyleSheet.hairlineWidth, paddingTop: 10 },
  body: { flexGrow: 1, flexShrink: 0, flexBasis: "auto", textAlignVertical: "top", paddingHorizontal: 0, paddingVertical: 4, lineHeight: typography.body.lineHeight },
  counter: { textAlign: "right", fontVariant: ["tabular-nums"] },
  tools: { gap: 8 },
  toolRow: { flexDirection: "row", gap: 8, padding: 4, borderWidth: StyleSheet.hairlineWidth, borderRadius: 12 },
  toolStack: { flexDirection: "column" },
  tool: { flex: 1, flexDirection: "row", minWidth: 48, minHeight: 48, alignItems: "center", justifyContent: "center", gap: 6, paddingVertical: 8, borderRadius: 8 },
  toolLabel: { flexShrink: 1, minWidth: 0 },
  toolWide: { flex: 0, flexDirection: "row", justifyContent: "flex-start", minHeight: 48, paddingHorizontal: 4, gap: 12 },
});
