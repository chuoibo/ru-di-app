import { translatedAsActor, newAttempt } from "../../api";
import { HoiTaiHang } from "../ui/HoiTaiHang";
import { COMMUNITY_ERRORS } from "./api";
import { Image } from "expo-image";
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { Pressable, ScrollView, Text, View } from "react-native";
import { useRouter } from "expo-router";
import { typography, useRudiTheme } from "../theme";
import { Field, IconButton, RudiButton } from "../ui";
import { AvatarNguoi } from "../ui/AvatarNguoi";
import { SkeletonLines } from "../ui/Skeleton";
import { comment, imageSource, readComments, relativeTime, type Comment, type Media, type Post } from "./api";
import { MediaPicker, type SelectedFile } from "./MediaPicker";
import { MentionPicker } from "./MentionPicker";
import { useCommunityStream } from "./useCommunityStream";
import { CauTaiCho } from "../ui/CauTaiCho";
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
export function Comments(props: CommentsProps) {
    const context = `${props.person}:${props.post.id}`;
    const draft = props.initialDraft;
    // Drafts are session-memory values; unowned input has no safe migration.
    const matches = draft?.person_id === props.person && draft?.post_id === props.post.id;
    return <CommentsForContext key={context} {...props} initialDraft={matches ? draft : null}/>;
}
function CommentsForContext({ person, post, expanded = false, presentation, initialDraft, onDraftChange }: CommentsProps) {
    const { colors } = useRudiTheme();
    const router = useRouter();
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
    const [busy, setBusy] = useState(false);
    const [uploading, setUploading] = useState(false);
    const uploadingNow = useRef(false);
    // QA UI-096 on Cộng đồng: «Xóa» used to remove a comment on one tap. It
    // now asks in the comment's own row first; a refused delete is said there
    // too, not at the send box below.
    const [hoiXoa, setHoiXoa] = useState<string | null>(null);
    const [dangXoa, setDangXoa] = useState<string | null>(null);
    const [loiXoa, setLoiXoa] = useState<{ id: string; cau: string } | null>(null);
    const xoa = (c: Comment) => {
        if (!mounted.current || deleting.current) return;
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
    const thread = <View style={{ gap: 16 }}><View style={{ flexDirection: "row", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: 8 }}><Text style={[typography.h2, { color: colors.ink }]}>Bình luận</Text>{!expanded && !presentation ? <RudiButton full={false} compact label="Mở rộng" variant="ghost" onPress={() => router.push(`/community/posts/${post.id}` as never)}/> : null}</View>
    {fresh ? <RudiButton compact label="Có bình luận mới" variant="outline" onPress={() => void load()}/> : null}
    <CauTaiCho cau={error} hanhDong={{ label: "Thử lại", onPress: () => void load() }} testID="cong-dong-loi-binh-luan" />
    {!loaded && !error ? <SkeletonLines lines={3}/> : null}
    {loaded && !items.length && !pending.length ? <Text style={[typography.body, { color: colors.inkSoft }]}>Chưa có lời nào ở đây. {post.can_comment ? "Bạn có thể mở đầu chuyện trò." : "Tác giả đã giới hạn bình luận."}</Text> : null}
    {[...items, ...pending].map((c) => <View key={c.id} style={{ paddingVertical: 12, marginLeft: c.parent_id ? 24 : 0, borderBottomWidth: 1, borderColor: colors.line, gap: 8 }}><View style={{ flexDirection: "row", flexWrap: "wrap", gap: 10, alignItems: "center" }}><AvatarNguoi personId={c.author_id} name={c.author || "Bạn"} size={32}/><Text style={[typography.label, { color: colors.ink, flex: 1, minWidth: 100 }]}>{c.author || "Bạn"}</Text><Text style={[typography.caption, { color: colors.inkFaint }]}>{relativeTime(c.created_at)}</Text></View><Text style={[typography.body, { color: colors.ink }]}>{c.body}</Text>{c.media_id ? <Image source={imageSource(person, `/v2/community/media/${c.media_id}`)} cachePolicy="none" style={{ width: 180, height: 180, borderRadius: 12 }}/> : null}{c.status !== "approved" ? <Text style={[typography.caption, { color: colors.accent }]}>{c.status === "rejected" ? "Bình luận chưa phù hợp" : "Đang chờ duyệt · Chỉ bạn thấy"}</Text> : null}<View style={{ flexDirection: "row", gap: 20 }}>{!c.parent_id && c.status === "approved" && post.can_comment ? <Pressable accessibilityRole="button" onPress={() => { if (!mounted.current) return; attempt.current = newAttempt().key; publish({ parent: c }); setParent(c); setLoiGui(null); }} style={{ minHeight: 48, minWidth: 48, justifyContent: "center" }}><Text style={[typography.caption, { color: colors.inkSoft }]}>Trả lời</Text></Pressable> : null}{c.can_delete && hoiXoa !== c.id ? <RudiButton accessibilityLabel={`Xóa bình luận của bạn: «${c.body.slice(0, 40)}»`} compact full={false} label="Xóa" tone="warn" variant="ghost" onPress={() => { setHoiXoa(c.id); setLoiXoa(null); }}/> : null}</View>{hoiXoa === c.id ? <HoiTaiHang cau="Xóa bình luận này? Không lấy lại được." dangLam={dangXoa === c.id} nhan="Xóa" onDongY={() => xoa(c)} onThoi={() => setHoiXoa(null)}/> : null}{loiXoa?.id === c.id ? <CauTaiCho co="nho" cau={loiXoa.cau} hanhDong={{ label: "Thử lại", onPress: () => xoa(c) }}/> : null}</View>)}
    {next ? <RudiButton label="Đọc tiếp bình luận" variant="ghost" onPress={() => void load(next)}/> : null}
    </View>;
    const sendReason = busy ? "Đợi gửi xong rồi viết tiếp." : uploading ? "Đợi ảnh tải xong rồi gửi." : pendingFile !== null ? "Thử lại hoặc bỏ ảnh đang chờ." : !body.trim() ? "Viết vài chữ trước khi gửi." : null;
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
      renderToolbar={(pick, preparing) => <IconButton icon="images-outline" quiet accessibilityLabel="Thêm ảnh" disabled={preparing} loading={uploading} onPress={pick}/>}
      renderLayout={(details, attachmentTool) => !post.can_comment ? <Text style={[typography.body, { color: colors.inkSoft }]}>Tác giả đã giới hạn bình luận cho bài này.</Text> : <View testID="community-comment-editor" style={{ gap: 8, paddingTop: 12, borderTopWidth: 1, borderTopColor: colors.line }}>
        <ScrollView testID="community-comment-details" bounces={false} keyboardShouldPersistTaps="handled" showsVerticalScrollIndicator style={{ maxHeight: 96, flexGrow: 0, display: details || parent || tagOpen || loiGui || sendReason ? "flex" : "none" }} contentContainerStyle={{ gap: 8, paddingBottom: 4 }}>
          {parent ? <View style={{ flexDirection: "row", alignItems: "center", gap: 8 }}><Text style={[typography.caption, { color: colors.inkSoft, flex: 1 }]}>Trả lời {parent.author}</Text><IconButton quiet icon="close" accessibilityLabel="Hủy trả lời" onPress={() => { if (!mounted.current) return; attempt.current = newAttempt().key; publish({ parent: null }); setParent(null); setLoiGui(null); }}/></View> : null}
          {details}
          <MentionPicker person={person} selected={mentions} expanded={tagOpen} onExpandedChange={setTagOpen} hideTrigger onChange={(v) => { if (!mounted.current) return; attempt.current = newAttempt().key; publish({ mentions: v }); setMentions(v); setLoiGui(null); }}/>
          <CauTaiCho cau={loiGui} />
          {sendReason ? <Text testID="community-comment-send-reason" style={[typography.caption, { color: colors.inkSoft }]}>{sendReason}</Text> : null}
        </ScrollView>
        <Field multiline accessibilityLabel="Bình luận" testID="community-comment-input" value={body} editable={!busy} numberOfLines={1} style={{ minHeight: 24, maxHeight: 72 }} onChangeText={(v) => { if (!mounted.current) return; attempt.current = newAttempt().key; publish({ body: v }); setBody(v); setLoiGui(null); }} placeholder="Viết bình luận…"/>
        <View testID="community-comment-tools" style={{ flexDirection: "row", alignItems: "center", gap: 4 }}>
          {attachmentTool}
          <IconButton icon="at-outline" quiet selected={tagOpen || mentions.length > 0} accessibilityLabel={mentions.length ? `Gắn bạn bè, đã chọn ${mentions.length} người` : "Gắn bạn bè"} onPress={() => setTagOpen((value) => !value)}/>
          {mentions.length ? <Text style={[typography.caption, { color: colors.accent, backgroundColor: colors.accentSoft, borderRadius: 10, paddingHorizontal: 8 }]}>{mentions.length}</Text> : null}
          <View style={{ flex: 1 }}/>
          <IconButton icon="arrow-up" solid tron accessibilityLabel={busy ? "Đang gửi…" : "Gửi bình luận"} disabled={busy || uploading || pendingFile !== null || !body.trim()} loading={busy} onPress={() => void send()}/>
        </View>
      </View>}
    />;
    return presentation ? presentation(thread, editor) : <View style={{ gap: 16 }}>{thread}{editor}</View>;
}
