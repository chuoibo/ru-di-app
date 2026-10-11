import { translatedAsActor, newAttempt } from "../../api";
import { HoiTaiHang } from "../ui/HoiTaiHang";
import { COMMUNITY_ERRORS } from "./api";
import { Image } from "expo-image";
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { Keyboard, Pressable, ScrollView, View } from "react-native";
import { useRouter } from "expo-router";
import { typography, useRudiTheme } from "../theme";
import { Field, IconButton, RudiButton } from "../ui";
import { AvatarNguoi } from "../ui/AvatarNguoi";
import { RudiIcon } from "../ui/RudiIcon";
import { PressScale } from "../ui/PressScale";
import { SkeletonLines } from "../ui/Skeleton";
import { comment, imageSource, readComments, relativeTime, type Comment, type Media, type Post } from "./api";
import { MediaPicker, type SelectedFile } from "./MediaPicker";
import { MentionPicker } from "./MentionPicker";
import { useCommunityStream } from "./useCommunityStream";
import { CauTaiCho } from "../ui/CauTaiCho";
import { useNativeFontScale } from "../chia-bill/useBillFontScale";
import { NativeFontMetricsProvider, Text, useNativeFontMetricsOwner, useNativeStandaloneTextMetrics } from "../ui/NativeFontMetrics";
export type CommentDraft = { body: string; parent: Comment | null; media: Media[]; mentions: string[]; logical_id: string; pendingFile: SelectedFile | null; person_id: string; post_id: string };
type CommentsProps = {
    expanded?: boolean;
    /** One editor owns both the scrolling thread and a keyboard-safe footer. */
    presentation?: (thread: ReactNode, editor: ReactNode) => ReactNode;
    initialDraft?: CommentDraft | null;
    onDraftChange?: (draft: CommentDraft) => void;
    person: string;
    post: Post;
};
/** One observer spans the community surface, its footer, and every sheet. */
export function CommunityFontMetrics({ children }: { children: ReactNode }) {
    const owned = useNativeFontMetricsOwner();
    return owned ? <>{children}</> : <ObserveCommunityFontMetrics>{children}</ObserveCommunityFontMetrics>;
}
function ObserveCommunityFontMetrics({ children }: { children: ReactNode }) {
    const nativeFont = useNativeFontScale();
    return <NativeFontMetricsProvider snapshot={nativeFont}>{children}</NativeFontMetricsProvider>;
}
export function Comments(props: CommentsProps) {
    const context = `${props.person}:${props.post.id}`;
    const draft = props.initialDraft;
    // Drafts are session-memory values; unowned input has no safe migration.
    const matches = draft?.person_id === props.person && draft?.post_id === props.post.id;
    return <CommunityFontMetrics><CommentsForContext key={context} {...props} initialDraft={matches ? draft : null}/></CommunityFontMetrics>;
}
function CommentsForContext({ person, post, expanded = false, presentation, initialDraft, onDraftChange }: CommentsProps) {
    const { colors } = useRudiTheme();
    const router = useRouter();
    const bodyMetrics = useNativeStandaloneTextMetrics(typography.body);
    const largeText = bodyMetrics === null || (bodyMetrics.fontSize ?? typography.body.fontSize) >= typography.body.fontSize * 1.25;
    const mounted = useRef(true);
    const canCommentNow = useRef(post.can_comment);
    canCommentNow.current = post.can_comment;
    const readSerial = useRef(0);
    const reading = useRef<number | null>(null);
    const cursor = useRef<string | null>(null);
    const [loaded, setLoaded] = useState(false);
    const sending = useRef(false);
    const deleting = useRef(false);
    const [items, setItems] = useState<Comment[]>([]);
    const [pending, setPending] = useState<Comment[]>([]);
    const [next, setNext] = useState<string | null>(null);
    const [body, setBody] = useState(initialDraft?.body ?? "");
    const [parent, setParent] = useState<Comment | null>(initialDraft?.parent ?? null);
    const [media, setMedia] = useState<Media[]>(initialDraft?.media ?? []);
    const [pendingFile, setPendingFile] = useState<SelectedFile | null>(initialDraft?.pendingFile ?? null);
    const [mentions, setMentions] = useState<string[]>(initialDraft?.mentions ?? []);
    const [pickerRevision, setPickerRevision] = useState(0);
    const [tagOpen, setTagOpen] = useState(false);
    const [toolsOpen, setToolsOpen] = useState(false);
    const [commentMenu, setCommentMenu] = useState<string | null>(null);
    const [busy, setBusy] = useState(false);
    const [uploading, setUploading] = useState(false);
    const uploadingNow = useRef(false);
    // QA UI-096 on Cộng đồng: «Xóa» used to remove a comment on one tap. It
    // now asks in the comment's own row first; a refused delete is said there
    // too, not at the send box below.
    const [hoiXoa, setHoiXoa] = useState<string | null>(null);
    const [dangXoa, setDangXoa] = useState<string | null>(null);
    const deletableNow = useRef(new Set<string>());
    deletableNow.current = new Set([...items, ...pending].filter((item) => item.can_delete).map((item) => item.id));
    const [loiXoa, setLoiXoa] = useState<{ id: string; cau: string } | null>(null);
    const xoa = (c: Comment) => {
        if (!mounted.current || deleting.current || !deletableNow.current.has(c.id)) return;
        deleting.current = true;
        setDangXoa(c.id);
        setLoiXoa(null);
        void translatedAsActor(COMMUNITY_ERRORS, `/v2/community/posts/${post.id}/comments/${c.id}`, {
            actorId: person,
            method: "DELETE",
            attempt: newAttempt()
        }).then(() => { if (!mounted.current) return; setHoiXoa(null); return load(); })
            .catch((e) => { if (mounted.current) setLoiXoa({ id: c.id, cau: e instanceof Error ? e.message : String(e) }); })
            .finally(() => { if (mounted.current) { deleting.current = false; setDangXoa(null); } });
    };
    const [fresh, setFresh] = useState(false);
    // Two failures, two places: reading the comments (said at the top, with
    // «Thử lại», QA UI-148) and sending one (said at the box that sent it).
    const [error, setError] = useState<string | null>(null);
    const [loiGui, setLoiGui] = useState<string | null>(null);
    const attempt = useRef(initialDraft?.logical_id ?? newAttempt().key);
    const snapshot = useRef<CommentDraft>({ body, parent, media, mentions, pendingFile, logical_id: attempt.current, person_id: person, post_id: post.id });
    const publish = (patch: Partial<CommentDraft>) => {
        if (!mounted.current) return;
        snapshot.current = { ...snapshot.current, ...patch, logical_id: attempt.current, person_id: person, post_id: post.id };
        onDraftChange?.(snapshot.current);
    };
    // A handoff can unmount this editor before passive effects run. Publish each
    // edit/ACK synchronously; mount publication always reads the latest snapshot.
    useEffect(() => { if (mounted.current) onDraftChange?.(snapshot.current); }, [onDraftChange]);
    const load = useCallback(async (after?: string | null) => {
        // A refresh supersedes every older read; append belongs to this cursor.
        if (!mounted.current || after && (reading.current !== null || cursor.current !== after)) return;
        const serial = ++readSerial.current;
        reading.current = serial;
        const current = () => mounted.current && readSerial.current === serial;
        try {
            const page = await readComments(person, post.id, after);
            if (!current()) return;
            setItems((old) => after ? [...old, ...page.comments.filter((c) => !old.some((x) => x.id === c.id))] : page.comments);
            setPending(page.pending);
            cursor.current = page.next_cursor;
            setNext(page.next_cursor);
            setLoaded(true);
            setFresh(false);
            setError(null);
        } catch (e) {
            // What is already shown stays; the sentence says the reload failed.
            if (current()) setError(e instanceof Error ? e.message : "Chưa đọc được bình luận.");
        } finally {
            if (current()) reading.current = null;
        }
    }, [person, post.id]);
    useLayoutEffect(() => {
        mounted.current = true;
        return () => { mounted.current = false; readSerial.current++; };
    }, []);
    useEffect(() => { void load(); }, [load]);
    // A (re)connection re-reads in place: the list does not blink and the
    // comment being typed stays in its box (QA UI-134).
    useCommunityStream(person, [post.id], (e) => {
        if (!mounted.current) return;
        if (e.kind === "sync") void load();
        else if (e.kind === "post.changed") setFresh(true);
    });
    const send = async () => {
        const input = snapshot.current;
        if (!mounted.current || !canCommentNow.current || sending.current || uploadingNow.current || input.pendingFile !== null || !input.body.trim()) return;
        const logical = input.logical_id;
        sending.current = true; setBusy(true); setLoiGui(null);
        try {
            await comment(person, post.id, input.body, logical, input.parent?.id ?? null, input.media[0]?.id ?? null, input.mentions);
            if (!mounted.current) return;
            // A successful older submission must not erase a newer draft.
            if (attempt.current === logical) {
                attempt.current = newAttempt().key;
                publish({ body: "", parent: null, media: [], mentions: [], ...(!uploadingNow.current ? { pendingFile: null } : {}) });
                setBody("");
                setParent(null);
                setMedia([]);
                setMentions([]);
                // An already admitted picker must finish and emit its own idle.
                if (!uploadingNow.current) {
                    setPendingFile(null);
                    setPickerRevision((value) => value + 1);
                }
            }
            await load();
        } catch (e) {
            if (mounted.current && attempt.current === logical) setLoiGui(e instanceof Error ? e.message : "Chưa gửi được bình luận.");
        } finally {
            if (mounted.current) { sending.current = false; setBusy(false); }
        }
    };
    const thread = <View style={{ gap: 12 }}>
      <View style={{ flexDirection: "row", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: 8 }}>
        <Text style={[typography.h2, { color: colors.ink }]}>Bình luận</Text>
        {!expanded && !presentation ? <RudiButton full={false} compact label="Mở rộng" variant="ghost" onPress={() => router.push(`/community/posts/${post.id}` as never)}/> : null}
      </View>
      {fresh ? <RudiButton compact label="Có bình luận mới" variant="outline" onPress={() => void load()}/> : null}
      <CauTaiCho cau={error} hanhDong={{ label: "Thử lại", onPress: () => void load() }} testID="cong-dong-loi-binh-luan" />
      {!loaded && !error ? <SkeletonLines lines={3}/> : null}
      {loaded && !items.length && !pending.length ? <Text style={[typography.body, { color: colors.inkSoft }]}>Chưa có lời nào ở đây. {post.can_comment ? "Bạn có thể mở đầu chuyện trò." : "Tác giả đã giới hạn bình luận."}</Text> : null}
      <View testID="community-comment-list">
        {[...items, ...pending].map((c) => <View key={c.id} testID={`community-comment-${c.id}`} style={{ paddingTop: 8, marginLeft: c.parent_id ? 24 : 0, gap: 4 }}>
          <View style={{ flexDirection: "row", gap: 8, alignItems: "flex-start" }}>
            <AvatarNguoi personId={c.author_id} name={c.author || "Bạn"} size={32}/>
            <View style={{ flex: 1, minWidth: 0, gap: 4 }}>
              <View testID="community-comment-metadata" style={{ flexDirection: largeText ? "column" : "row", alignItems: largeText ? "flex-start" : "baseline", gap: largeText ? 2 : 8 }}>
                <Text style={[typography.label, { color: colors.ink, flex: largeText ? undefined : 1, minWidth: 0 }]}>{c.author || "Bạn"}</Text>
                <Text style={[typography.note, { color: colors.inkFaint, flexShrink: 0, alignSelf: largeText ? "stretch" : undefined }]}>{relativeTime(c.created_at)}</Text>
              </View>
              {/* Keep the same body hosts while large text uses the whole row. */}
              <View testID="community-comment-content" style={{ gap: largeText ? 6 : 4, marginLeft: largeText ? -40 : 0 }}>
                <Text style={[typography.body, { color: colors.ink }]}>{c.body}</Text>
                {c.media_id ? <Image source={imageSource(person, `/v2/community/media/${c.media_id}`)} cachePolicy="none" style={{ width: 180, height: 180, borderRadius: 12 }}/> : null}
                {c.status !== "approved" ? <Text style={[typography.note, { color: colors.accent }]}>{c.status === "rejected" ? "Bình luận chưa phù hợp" : "Đang chờ duyệt · Chỉ bạn thấy"}</Text> : null}
                <View style={{ flexDirection: "row", alignItems: "center", gap: 4 }}>
                  {!c.parent_id && c.status === "approved" && post.can_comment ? <Pressable accessibilityRole="button" onPress={() => { if (!mounted.current) return; attempt.current = newAttempt().key; publish({ parent: c }); setParent(c); setLoiGui(null); }} style={{ minHeight: 48, minWidth: 48, justifyContent: "center" }}><Text style={[typography.caption, { color: colors.inkSoft }]}>Trả lời</Text></Pressable> : null}
                  {c.can_delete && hoiXoa !== c.id ? <PressScale pressedScale={0.94} haptic="select" accessibilityRole="button" accessibilityLabel={commentMenu === c.id ? "Đóng lựa chọn cho bình luận" : "Thêm lựa chọn cho bình luận"} accessibilityState={{ expanded: commentMenu === c.id }} onPress={() => { if (!mounted.current || !deletableNow.current.has(c.id)) return; setCommentMenu((current) => current === c.id ? null : c.id); }} style={{ width: 48, height: 48, alignItems: "center", justifyContent: "center" }}><RudiIcon name="ellipsis-horizontal" size={22} color={colors.inkSoft}/></PressScale> : null}
                </View>
                {c.can_delete && commentMenu === c.id && hoiXoa !== c.id ? <RudiButton accessibilityLabel={`Xóa bình luận của bạn: «${c.body.slice(0, 40)}»`} compact full={false} label="Xóa" tone="warn" variant="ghost" onPress={() => { if (!mounted.current || !deletableNow.current.has(c.id)) return; setCommentMenu(null); setHoiXoa(c.id); setLoiXoa(null); }}/> : null}
                {c.can_delete && hoiXoa === c.id ? <HoiTaiHang cau="Xóa bình luận này? Không lấy lại được." dangLam={dangXoa === c.id} nhan="Xóa" onDongY={() => xoa(c)} onThoi={() => setHoiXoa(null)}/> : null}
                {loiXoa?.id === c.id ? <CauTaiCho co="nho" cau={loiXoa.cau} hanhDong={c.can_delete ? { label: "Thử lại", onPress: () => xoa(c) } : undefined}/> : null}
              </View>
            </View>
          </View>
          <View style={{ marginLeft: largeText ? 0 : 40, borderBottomWidth: 1, borderColor: colors.line }}/>
        </View>)}
      </View>
      {next ? <RudiButton label="Đọc tiếp bình luận" variant="ghost" onPress={() => void load(next)}/> : null}
    </View>;
    const sendReason = busy ? "Đợi gửi xong rồi viết tiếp." : uploading ? "Đợi ảnh tải xong rồi gửi." : pendingFile !== null ? "Thử lại hoặc bỏ ảnh đang chờ." : null;
    const editor = <MediaPicker
      key={pickerRevision}
      disabled={busy || !post.can_comment}
      canEdit={() => mounted.current && canCommentNow.current && !sending.current}
      initialFile={pendingFile}
      onFileChange={(v) => { if (!mounted.current || v === snapshot.current.pendingFile) return; attempt.current = newAttempt().key; publish({ pendingFile: v }); setPendingFile(v); }}
      person={person}
      media={media}
      onChange={(v) => { if (!mounted.current) return; attempt.current = newAttempt().key; publish({ media: v }); setMedia(v); setLoiGui(null); }}
      video={false}
      onBusy={(v) => { if (mounted.current) { uploadingNow.current = v; setUploading(v); } }}
      renderToolbar={(pick, preparing) => <RudiButton compact label="Ảnh" accessibilityLabel="Thêm ảnh" variant="ghost" disabled={preparing} lyDo={preparing ? "Chờ tệp chuẩn bị xong." : undefined} style={{ flex: largeText ? undefined : 1, minWidth: 0, paddingHorizontal: 8 }} onPress={() => { if (!mounted.current || snapshot.current.pendingFile !== null) return; setToolsOpen(false); setTagOpen(false); pick(); }}/>}
      renderLayout={(_details, attachmentTool, parts) => !post.can_comment ? <Text style={[typography.body, { color: colors.inkSoft }]}>Tác giả đã giới hạn bình luận cho bài này.</Text> : <View testID="community-comment-editor" style={{ gap: 4, paddingTop: 8, borderTopWidth: 1, borderTopColor: colors.line }}>
        <ScrollView testID="community-comment-details" bounces={false} keyboardShouldPersistTaps="handled" showsVerticalScrollIndicator style={{ maxHeight: 72, flexGrow: 0, display: !tagOpen && (parts.preview || parts.notice || parent || loiGui) ? "flex" : "none" }} contentContainerStyle={{ gap: 8, paddingBottom: 4 }}>
          {parent ? <View style={{ flexDirection: "row", alignItems: "center", gap: 8 }}><Text style={[typography.note, { color: colors.inkSoft, flex: 1 }]}>Trả lời {parent.author}</Text><IconButton quiet icon="close" accessibilityLabel="Hủy trả lời" onPress={() => { if (!mounted.current) return; attempt.current = newAttempt().key; publish({ parent: null }); setParent(null); setLoiGui(null); }}/></View> : null}
          {parts.preview}
          {parts.notice}
          <CauTaiCho cau={loiGui} />
        </ScrollView>
        <View testID="community-comment-mentions" style={{ gap: 4, display: tagOpen ? "flex" : "none" }}>
          <View testID="community-comment-mentions-header" style={{ minHeight: 48, flexDirection: "row", alignItems: "center", gap: 8 }}>
            <Text style={[typography.label, { color: colors.ink, flex: 1 }]}>Gắn bạn</Text>
            <IconButton quiet icon="close" accessibilityLabel="Đóng danh sách bạn" onPress={() => { if (mounted.current) setTagOpen(false); }}/>
          </View>
          <ScrollView testID="community-comment-mentions-list" bounces={false} keyboardShouldPersistTaps="handled" showsVerticalScrollIndicator style={{ maxHeight: parts.pending || parts.retry || parts.discard || parts.stop ? 160 : 216, flexGrow: 0 }} contentContainerStyle={{ paddingBottom: 4 }}>
            <MentionPicker person={person} selected={mentions} expanded={tagOpen} onExpandedChange={setTagOpen} hideTrigger onChange={(v) => { if (!mounted.current) return; attempt.current = newAttempt().key; publish({ mentions: v }); setMentions(v); setLoiGui(null); }}/>
          </ScrollView>
        </View>
        <View testID="community-comment-entry" style={{ flexDirection: "row", alignItems: "flex-end", gap: 8 }}>
          <PressScale pressedScale={0.94} haptic="select" accessibilityRole="button" accessibilityLabel={toolsOpen ? "Đóng công cụ bình luận" : "Thêm vào bình luận"} accessibilityState={{ expanded: toolsOpen, disabled: busy || parts.busy }} disabled={busy || parts.busy} onPress={() => { if (!mounted.current || sending.current) return; setTagOpen(false); setToolsOpen((value) => !value); }} style={{ width: 48, height: 48, alignItems: "center", justifyContent: "center" }}><RudiIcon name={toolsOpen ? "close" : "add"} size={22} color={busy || parts.busy ? colors.inkFaint : colors.inkSoft}/></PressScale>
          <View testID="community-comment-field-slot" style={{ flex: 1, minWidth: 0 }}>
            <Field nativeMetrics autoGrowLines={3} multiline accessibilityLabel="Bình luận" testID="community-comment-input" value={body} editable={!busy} numberOfLines={1} style={{ minHeight: 24, maxHeight: 72 }} onFocus={() => { if (mounted.current) setTagOpen(false); }} onChangeText={(v) => { if (!mounted.current) return; attempt.current = newAttempt().key; publish({ body: v }); setBody(v); setLoiGui(null); }} placeholder="Viết…"/>
          </View>
          <IconButton icon="arrow-up" solid tron accessibilityLabel={busy ? "Đang gửi…" : "Gửi bình luận"} disabled={busy || uploading || pendingFile !== null || parts.pending || !body.trim()} loading={busy} onPress={() => void send()}/>
        </View>
        {sendReason && !parts.notice && !tagOpen ? <Text testID="community-comment-send-reason" style={[typography.note, { color: colors.inkSoft }]}>{sendReason}</Text> : null}
        {parts.retry && parts.discard ? <View testID="community-comment-recovery" style={{ flexDirection: "row", alignItems: "stretch", gap: 8 }}>
          <RudiButton compact label="Thử lại" accessibilityLabel="Thử lại tệp đã chọn" variant="outline" disabled={busy} style={{ flex: 1, minWidth: 0, paddingHorizontal: 8 }} onPress={parts.retry}/>
          <RudiButton compact label="Bỏ chọn" accessibilityLabel="Bỏ tệp đang chờ" variant="ghost" disabled={busy} style={{ flex: 1, minWidth: 0, paddingHorizontal: 8 }} onPress={parts.discard}/>
        </View> : parts.stop ? <RudiButton compact label="Dừng chờ tệp" variant="ghost" disabled={busy} onPress={parts.stop}/> : null}
        {!busy && !parts.busy && toolsOpen ? <View testID="community-comment-tools" style={{ flexDirection: largeText ? "column" : "row", alignItems: "stretch", gap: 4 }}>
          {!parts.pending ? attachmentTool : null}
          <RudiButton compact label={mentions.length ? `Gắn ${mentions.length} bạn` : "Gắn bạn"} accessibilityLabel={mentions.length ? `Gắn bạn bè, đã chọn ${mentions.length} người` : "Gắn bạn bè"} variant="ghost" style={{ flex: largeText ? undefined : 1, minWidth: 0, paddingHorizontal: 8 }} onPress={() => { if (!mounted.current || sending.current) return; Keyboard.dismiss(); setToolsOpen(false); setTagOpen(true); }}/>
        </View> : null}
      </View>}
    />;
    return presentation ? presentation(thread, editor) : <View style={{ gap: 16 }}>{thread}{editor}</View>;
}
