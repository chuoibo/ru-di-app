import { translatedAsActor, newAttempt } from "../../api";
import { HoiTaiHang } from "../ui/HoiTaiHang";
import { COMMUNITY_ERRORS } from "./api";
import { Image } from "expo-image";
import { useCallback, useEffect, useRef, useState } from "react";
import { Pressable, Text, View } from "react-native";
import { useRouter } from "expo-router";
import { typography, useRudiTheme } from "../theme";
import { Field, RudiButton } from "../ui";
import { AvatarNguoi } from "../ui/AvatarNguoi";
import { SkeletonLines } from "../ui/Skeleton";
import { comment, imageSource, readComments, relativeTime, type Comment, type Media, type Post } from "./api";
import { MediaPicker, type SelectedFile } from "./MediaPicker";
import { MentionPicker } from "./MentionPicker";
import { useCommunityStream } from "./useCommunityStream";
import { CauTaiCho } from "../ui/CauTaiCho";
export type CommentDraft = { body: string; parent: Comment | null; media: Media[]; mentions: string[]; logical_id: string; pendingFile: SelectedFile | null };
export function Comments({ person, post, expanded = false, initialDraft, onDraftChange }: {
    expanded?: boolean;
    initialDraft?: CommentDraft | null;
    onDraftChange?: (draft: CommentDraft) => void;
    person: string;
    post: Post;
}) {
    const { colors } = useRudiTheme();
    const router = useRouter();
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
    const [busy, setBusy] = useState(false);
    const [uploading, setUploading] = useState(false);
    // QA UI-096 on Cộng đồng: «Xóa» used to remove a comment on one tap. It
    // now asks in the comment's own row first; a refused delete is said there
    // too, not at the send box below.
    const [hoiXoa, setHoiXoa] = useState<string | null>(null);
    const [dangXoa, setDangXoa] = useState<string | null>(null);
    const [loiXoa, setLoiXoa] = useState<{ id: string; cau: string } | null>(null);
    const xoa = (c: Comment) => {
        if (deleting.current) return;
        deleting.current = true;
        setDangXoa(c.id);
        setLoiXoa(null);
        void translatedAsActor(COMMUNITY_ERRORS, `/v2/community/posts/${post.id}/comments/${c.id}`, {
            actorId: person,
            method: "DELETE",
            attempt: newAttempt()
        }).then(() => { setHoiXoa(null); return load(); }).catch((e) => setLoiXoa({ id: c.id, cau: e instanceof Error ? e.message : String(e) })).finally(() => { deleting.current = false; setDangXoa(null); });
    };
    const [fresh, setFresh] = useState(false);
    // Two failures, two places: reading the comments (said at the top, with
    // «Thử lại», QA UI-148) and sending one (said at the box that sent it).
    const [error, setError] = useState<string | null>(null);
    const [loiGui, setLoiGui] = useState<string | null>(null);
    const attempt = useRef(initialDraft?.logical_id ?? newAttempt().key);
    useEffect(() => { onDraftChange?.({ body, parent, media, mentions, pendingFile, logical_id: attempt.current }); }, [body, parent, media, mentions, pendingFile, onDraftChange]);
    const load = useCallback(async (after?: string | null) => { try {
        const page = await readComments(person, post.id, after);
        setItems((old) => after ? [...old, ...page.comments.filter((c) => !old.some((x) => x.id === c.id))] : page.comments);
        setPending(page.pending);
        setNext(page.next_cursor);
        setLoaded(true);
        setFresh(false);
        setError(null);
    }
    catch (e) {
        // What is already shown stays; the sentence says the reload failed.
        setError(e instanceof Error ? e.message : "Chưa đọc được bình luận.");
    } }, [person, post.id]);
    useEffect(() => { setLoaded(false); setItems([]); setPending([]); void load(); }, [load]);
    // A (re)connection re-reads in place: the list does not blink and the
    // comment being typed stays in its box (QA UI-134).
    useCommunityStream(person, [post.id], (e) => { if (e.kind === "sync") {
        void load();
    }
    else if (e.kind === "post.changed")
        setFresh(true); });
    const send = async () => { if (sending.current || uploading || !body.trim())
        return; sending.current = true; setBusy(true); setLoiGui(null); try {
        await comment(person, post.id, body, attempt.current, parent?.id ?? null, media[0]?.id ?? null, mentions);
        attempt.current = newAttempt().key;
        setBody("");
        setParent(null);
        setMedia([]);
        setMentions([]);
        await load();
    }
    catch (e) {
        setLoiGui(e instanceof Error ? e.message : "Chưa gửi được bình luận.");
    }
    finally {
        sending.current = false; setBusy(false);
    } };
    return <View style={{ gap: 16 }}><View style={{ flexDirection: "row", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: 8 }}><Text style={[typography.h2, { color: colors.ink }]}>Chuyện trò cùng nhau</Text>{!expanded ? <RudiButton full={false} compact label="Mở rộng" variant="ghost" onPress={() => router.push(`/community/posts/${post.id}` as never)}/> : null}</View>
    {fresh ? <RudiButton compact label="Có bình luận mới" variant="outline" onPress={() => void load()}/> : null}
    <CauTaiCho cau={error} hanhDong={{ label: "Thử lại", onPress: () => void load() }} testID="cong-dong-loi-binh-luan" />
    {!loaded && !error ? <SkeletonLines lines={3}/> : null}
    {loaded && !items.length && !pending.length ? <Text style={[typography.body, { color: colors.inkSoft }]}>Chưa có lời nào ở đây. {post.can_comment ? "Bạn có thể mở đầu chuyện trò." : "Tác giả đã giới hạn bình luận."}</Text> : null}
    {[...items, ...pending].map((c) => <View key={c.id} style={{ paddingVertical: 12, marginLeft: c.parent_id ? 24 : 0, borderBottomWidth: 1, borderColor: colors.line, gap: 8 }}><View style={{ flexDirection: "row", flexWrap: "wrap", gap: 10, alignItems: "center" }}><AvatarNguoi personId={c.author_id} name={c.author || "Bạn"} size={32}/><Text style={[typography.label, { color: colors.ink, flex: 1, minWidth: 100 }]}>{c.author || "Bạn"}</Text><Text style={[typography.caption, { color: colors.inkFaint }]}>{relativeTime(c.created_at)}</Text></View><Text style={[typography.body, { color: colors.ink }]}>{c.body}</Text>{c.media_id ? <Image source={imageSource(person, `/v2/community/media/${c.media_id}`)} cachePolicy="none" style={{ width: 180, height: 180, borderRadius: 12 }}/> : null}{c.status !== "approved" ? <Text style={[typography.caption, { color: colors.accent }]}>{c.status === "rejected" ? "Bình luận chưa phù hợp" : "Đang chờ duyệt · Chỉ bạn thấy"}</Text> : null}<View style={{ flexDirection: "row", gap: 20 }}>{!c.parent_id && c.status === "approved" && post.can_comment ? <Pressable accessibilityRole="button" onPress={() => { setParent(c); attempt.current = newAttempt().key; }} style={{ minHeight: 48, minWidth: 48, justifyContent: "center" }}><Text style={[typography.caption, { color: colors.inkSoft }]}>Trả lời</Text></Pressable> : null}{c.can_delete && hoiXoa !== c.id ? <RudiButton accessibilityLabel={`Xóa bình luận của bạn: «${c.body.slice(0, 40)}»`} compact full={false} label="Xóa" tone="warn" variant="ghost" onPress={() => { setHoiXoa(c.id); setLoiXoa(null); }}/> : null}</View>{hoiXoa === c.id ? <HoiTaiHang cau="Xóa bình luận này? Không lấy lại được." dangLam={dangXoa === c.id} nhan="Xóa" onDongY={() => xoa(c)} onThoi={() => setHoiXoa(null)}/> : null}{loiXoa?.id === c.id ? <CauTaiCho co="nho" cau={loiXoa.cau} hanhDong={{ label: "Thử lại", onPress: () => xoa(c) }}/> : null}</View>)}
    {next ? <RudiButton label="Đọc tiếp bình luận" variant="ghost" onPress={() => void load(next)}/> : null}
    {post.can_comment ? <><Text style={[typography.label, { color: colors.ink }]}>{parent ? `Trả lời ${parent.author}` : "Thêm một lời"}</Text>{parent ? <RudiButton compact label="Hủy trả lời" variant="ghost" onPress={() => setParent(null)}/> : null}<Field multiline label="Bình luận" value={body} editable={!busy} onChangeText={(v) => { setBody(v); attempt.current = newAttempt().key; }} placeholder="Chia sẻ một gợi ý, một lời hẹn…"/><MediaPicker initialFile={pendingFile} onFileChange={setPendingFile} person={person} media={media} onChange={(v) => { setMedia(v); attempt.current = newAttempt().key; }} video={false} onBusy={setUploading}/><MentionPicker person={person} selected={mentions} onChange={(v) => { setMentions(v); attempt.current = newAttempt().key; }}/><CauTaiCho cau={loiGui} /><RudiButton label={busy ? "Đang gửi…" : "Gửi bình luận"} disabled={busy || uploading || !body.trim()} lyDo={uploading ? "Đợi ảnh tải xong rồi gửi." : !body.trim() ? "Viết vài chữ trước khi gửi." : undefined} onPress={() => void send()}/></> : <Text style={[typography.body, { color: colors.inkSoft }]}>Tác giả đã giới hạn bình luận cho bài này.</Text>}
  </View>;
}
