import { translatedAsActor, newAttempt } from "../../api";
import { COMMUNITY_ERRORS } from "./api";
import { Image } from "expo-image";
import { useCallback, useEffect, useRef, useState } from "react";
import { Pressable, Text, View } from "react-native";
import { useRouter } from "expo-router";
import { typography, useRudiTheme } from "../theme";
import { Field, RudiButton } from "../ui";
import { Avatar } from "../ui/Avatar";
import { comment, imageSource, readComments, relativeTime, type Comment, type Media, type Post } from "./api";
import { MediaPicker } from "./MediaPicker";
import { MentionPicker } from "./MentionPicker";
import { useCommunityStream } from "./useCommunityStream";
export function Comments({ person, post, expanded = false }: {
    expanded?: boolean;
    person: string;
    post: Post;
}) {
    const { colors } = useRudiTheme();
    const router = useRouter();
    const [items, setItems] = useState<Comment[]>([]);
    const [pending, setPending] = useState<Comment[]>([]);
    const [next, setNext] = useState<string | null>(null);
    const [body, setBody] = useState("");
    const [parent, setParent] = useState<Comment | null>(null);
    const [media, setMedia] = useState<Media[]>([]);
    const [mentions, setMentions] = useState<string[]>([]);
    const [busy, setBusy] = useState(false);
    const [uploading, setUploading] = useState(false);
    const [fresh, setFresh] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const attempt = useRef(newAttempt().key);
    const load = useCallback(async (after?: string | null) => { try {
        const page = await readComments(person, post.id, after);
        setItems((old) => after ? [...old, ...page.comments.filter((c) => !old.some((x) => x.id === c.id))] : page.comments);
        setPending(page.pending);
        setNext(page.next_cursor);
        setFresh(false);
        setError(null);
    }
    catch (e) {
        setError(e instanceof Error ? e.message : "Chưa đọc được bình luận.");
        setItems([]);
        setPending([]);
    } }, [person, post.id]);
    useEffect(() => { void load(); }, [load]);
    useCommunityStream(person, [post.id], (e) => { if (e.kind === "sync") {
        setItems([]);
        void load();
    }
    else if (e.kind === "post.changed")
        setFresh(true); });
    const send = async () => { if (busy || uploading)
        return; setBusy(true); setError(null); try {
        await comment(person, post.id, body, attempt.current, parent?.id ?? null, media[0]?.id ?? null, mentions);
        attempt.current = newAttempt().key;
        setBody("");
        setParent(null);
        setMedia([]);
        setMentions([]);
        await load();
    }
    catch (e) {
        setError(e instanceof Error ? e.message : "Chưa gửi được bình luận.");
    }
    finally {
        setBusy(false);
    } };
    return <View style={{ gap: 16 }}><View style={{ flexDirection: "row", justifyContent: "space-between", alignItems: "center" }}><Text style={[typography.h2, { color: colors.ink }]}>Chuyện trò cùng nhau</Text>{!expanded ? <RudiButton full={false} compact label="Mở rộng" variant="ghost" onPress={() => router.push(`/community/posts/${post.id}` as never)}/> : null}</View>
    {fresh ? <RudiButton compact label="Có bình luận mới" variant="outline" onPress={() => void load()}/> : null}
    {error ? <Text accessibilityRole="alert" style={[typography.body, { color: colors.accent }]}>{error}</Text> : null}
    {[...items, ...pending].map((c) => <View key={c.id} style={{ paddingVertical: 12, marginLeft: c.parent_id ? 24 : 0, borderBottomWidth: 1, borderColor: colors.line, gap: 8 }}><View style={{ flexDirection: "row", gap: 10, alignItems: "center" }}><Avatar name={c.author || "Bạn"} size={32}/><Text style={[typography.label, { color: colors.ink }]}>{c.author || "Bạn"}</Text><Text style={[typography.caption, { color: colors.inkFaint }]}>{relativeTime(c.created_at)}</Text></View><Text style={[typography.body, { color: colors.ink }]}>{c.body}</Text>{c.media_id ? <Image source={imageSource(person, `/v2/community/media/${c.media_id}`)} cachePolicy="none" style={{ width: 180, height: 180, borderRadius: 12 }}/> : null}{c.status !== "approved" ? <Text style={[typography.caption, { color: colors.accent }]}>{c.status === "rejected" ? "Bình luận chưa phù hợp" : "Đang chờ duyệt · Chỉ bạn thấy"}</Text> : null}<View style={{ flexDirection: "row", gap: 20 }}>{!c.parent_id && c.status === "approved" && post.can_comment ? <Pressable accessibilityRole="button" onPress={() => { setParent(c); attempt.current = newAttempt().key; }} style={{ minHeight: 48, minWidth: 48, justifyContent: "center" }}><Text style={[typography.caption, { color: colors.inkSoft }]}>Trả lời</Text></Pressable> : null}{c.can_delete ? <RudiButton compact full={false} label="Xóa" variant="ghost" onPress={() => { void translatedAsActor(COMMUNITY_ERRORS, `/v2/community/posts/${post.id}/comments/${c.id}`, {
        actorId: person,
        method: "DELETE",
        attempt: newAttempt()
    }).then(() => load()).catch((e) => setError(e.message)); }}/> : null}</View></View>)}
    {next ? <RudiButton label="Đọc tiếp bình luận" variant="ghost" onPress={() => void load(next)}/> : null}
    {post.can_comment ? <><Text style={[typography.label, { color: colors.ink }]}>{parent ? `Trả lời ${parent.author}` : "Thêm một lời"}</Text>{parent ? <RudiButton compact label="Hủy trả lời" variant="ghost" onPress={() => setParent(null)}/> : null}<Field multiline label="Bình luận" value={body} onChangeText={(v) => { setBody(v); attempt.current = newAttempt().key; }} placeholder="Chia sẻ một gợi ý, một lời hẹn…"/><MediaPicker person={person} media={media} onChange={(v) => { setMedia(v); attempt.current = newAttempt().key; }} video={false} onBusy={setUploading}/><MentionPicker person={person} selected={mentions} onChange={(v) => { setMentions(v); attempt.current = newAttempt().key; }}/><RudiButton label={busy ? "Đang gửi…" : "Gửi bình luận"} disabled={busy || uploading || !body.trim()} onPress={() => void send()}/></> : <Text style={[typography.body, { color: colors.inkSoft }]}>Tác giả đã giới hạn bình luận cho bài này.</Text>}
  </View>;
}
