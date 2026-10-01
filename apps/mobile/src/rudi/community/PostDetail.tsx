import { translatedAsActor, newAttempt } from "../../api";
import { COMMUNITY_ERRORS } from "./api";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { Text, View } from "react-native";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { Field, RudiButton, RudiScreen, TopBar } from "../ui";
import { Sheet } from "../ui/Sheet";
import { NoiDungBaoCao } from "../screens/nguoi/NoiDungBaoCao";
import { feedback, follow, getPost, likePost, type Post } from "./api";
import { Comments } from "./Comments";
import { PostCard } from "./PostCard";
import { useCommunityStream } from "./useCommunityStream";
export function PostDetail() {
    const { id, report: reportParam } = useLocalSearchParams<{
        id: string;
        report?: string;
    }>();
    const router = useRouter();
    const { phien } = useRudiSession();
    const person = phien?.person_id;
    const { colors } = useRudiTheme();
    const [post, setPost] = useState<Post | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [actions, setActions] = useState(false);
    const [report, setReport] = useState(reportParam === "1");
    const [commentsOpen, setCommentsOpen] = useState(false);
    const keepAttempt = useRef(newAttempt().key);
    const [nep, setNep] = useState(false);
    const [request, setRequest] = useState("");
    const [draft, setDraft] = useState("");
    const [busy, setBusy] = useState(false);
    const [confirmDelete, setConfirmDelete] = useState(false);
    const load = useCallback(async () => { if (!person || !id)
        return; try {
        setPost(await getPost(person, id));
        setError(null);
    }
    catch (e) {
        setPost(null);
        setError(e instanceof Error ? e.message : "Chưa mở được bài.");
    } }, [person, id]);
    useEffect(() => { void load(); }, [load]);
    useCommunityStream(person, id ? [id] : [], (e) => { if (e.kind === "sync") {
        setPost(null);
        setDraft("");
        setNep(false);
    } if (e.kind !== "feed.changed")
        void load(); });
    const act = async (f: () => Promise<unknown>) => { if (busy)
        return; setBusy(true); setError(null); try {
        await f();
    }
    catch (e) {
        setError(e instanceof Error ? e.message : "Chưa thực hiện được.");
    }
    finally {
        setBusy(false);
    } };
    return <RudiScreen padded={false} testID="community-post-detail" overlay={<>
    <Sheet open={actions} onClose={() => { setActions(false); setConfirmDelete(false); }} accessibilityLabel="Quản lý bài">{post && person ? <><Text style={[typography.h2, { color: colors.ink }]}>Giữ lại câu chuyện</Text><RudiButton label={post.saved ? "Bỏ lưu bài" : "Lưu bài"} onPress={() => void act(async () => { await feedback(person, id, "saved", !post.saved); await load(); setActions(false); })}/>{post.author_id === person ? <>{!post.diary ? <RudiButton label="Sửa bài" variant="outline" onPress={() => { setActions(false); router.push({ pathname: "/community/new", params: { edit: id } } as never); }}/> : null}<RudiButton label="Cất lại cho bạn bè" variant="outline" onPress={() => void act(async () => { await translatedAsActor(COMMUNITY_ERRORS, `/v2/community/posts/${id}/audience`, {
            actorId: person,
            method: "PATCH",
            body: { audience: "friends", revision: post.revision },
            attempt: newAttempt()
        }); await load(); setActions(false); })}/><RudiButton label="Chỉ mình tôi" variant="ghost" onPress={() => void act(async () => { await translatedAsActor(COMMUNITY_ERRORS, `/v2/community/posts/${id}/audience`, {
            actorId: person,
            method: "PATCH",
            body: { audience: "only_me", revision: post.revision },
            attempt: newAttempt()
        }); await load(); setActions(false); })}/>{!post.diary && (post.audience !== "public" || post.status === "legacy") ? <RudiButton label="Chia sẻ lên cộng đồng" variant="outline" onPress={() => void act(async () => { await translatedAsActor(COMMUNITY_ERRORS, `/v2/community/posts/${id}/submit`, {
            actorId: person,
            method: "POST",
            attempt: newAttempt()
        }); await load(); setActions(false); })}/> : null}{["pending", "review", "rejected"].includes(post.status) ? <RudiButton label="Yêu cầu người vận hành xem xét" variant="ghost" onPress={() => void act(async () => { await translatedAsActor(COMMUNITY_ERRORS, `/v2/community/posts/${id}/appeal`, {
            actorId: person,
            method: "POST",
            attempt: newAttempt()
        }); await load(); setActions(false); })}/> : null}<RudiButton label={confirmDelete ? "Xác nhận xóa bài và bình luận" : "Xóa bài"} variant="ghost" onPress={() => { if (!confirmDelete) {
            setConfirmDelete(true);
            return;
        } void act(async () => { await translatedAsActor(COMMUNITY_ERRORS, `/v2/community/posts/${id}`, {
            actorId: person,
            method: "DELETE",
            attempt: newAttempt()
        }); router.replace("/(tabs)/community" as never); }); }}/></> : <RudiButton label="Báo cáo bài" variant="ghost" onPress={() => { setActions(false); setReport(true); }}/>}</> : null}</Sheet>
    <Sheet open={commentsOpen} onClose={() => setCommentsOpen(false)} accessibilityLabel="Bình luận">{person && post ? <Comments person={person} post={post} /> : null}</Sheet>
    <Sheet open={report} onClose={() => setReport(false)} accessibilityLabel="Báo cáo bài">{person ? <NoiDungBaoCao actorId={person} loai="post" targetId={id} onXong={() => setReport(false)} onThoi={() => setReport(false)}/> : null}</Sheet>
    <Sheet open={nep} onClose={() => setNep(false)} accessibilityLabel="Gọi Nếp">{person && post ? <><Text style={[typography.h2, { color: colors.ink }]}>Nhờ Nếp giữ một điều</Text><Text style={[typography.body, { color: colors.inkSoft }]}>Nếp chỉ nhận đoạn dưới đây và yêu cầu của bạn. Bạn xem lại bản nháp trước khi đăng.</Text><Text style={[typography.body, { color: colors.ink, paddingVertical: 16 }]}>{post.body}</Text><Field label="Bạn muốn Nếp giúp gì?" value={request} onChangeText={setRequest} multiline placeholder="Viết một lời ngắn để giữ khoảnh khắc này…"/><RudiButton label={busy ? "Nếp đang viết…" : "Đồng ý chia sẻ và gọi Nếp"} disabled={busy || !request.trim()} onPress={() => void act(async () => { const result = await translatedAsActor<{
            draft: string;
        }>(COMMUNITY_ERRORS, `/v2/community/posts/${id}/nep`, {
            actorId: person,
            method: "POST",
            body: { confirmed: true, excerpt: post.body, request },
            attempt: newAttempt()
        }); setDraft(result.draft); keepAttempt.current = newAttempt().key; })}/>{draft ? <><Text style={[typography.caption, { color: colors.ai }]}>BẢN NHÁP DO NẾP VIẾT</Text><Field label="Sửa lại theo ý bạn" value={draft} onChangeText={(value) => { setDraft(value); keepAttempt.current = newAttempt().key; }} multiline/><RudiButton label="Lưu ghi chép riêng" variant="outline" onPress={() => void act(async () => { await translatedAsActor(COMMUNITY_ERRORS, "/v2/community/keeps", { actorId: person, method: "POST", attempt: newAttempt(), body: { logical_id: keepAttempt.current, post_id: id, body: draft, ai_generated: true } }); setNep(false); router.push("/community/keeps" as never); })} /></> : null}</> : null}</Sheet>
  </>}>
    <View style={{ paddingHorizontal: 20 }}><TopBar title="Một câu chuyện"/>{error ? <><Text accessibilityRole="alert" style={[typography.body, { color: colors.accent }]}>{error}</Text><RudiButton label="Thử lại" variant="ghost" onPress={() => void load()}/></> : null}</View>
    {post && person ? <><PostCard detail post={post} person={person} active busy={busy} onLike={() => void act(async () => setPost(await likePost(person, id, !post.liked)))} onComment={() => setCommentsOpen(true)} onMore={() => setActions(true)} onFollow={() => void act(async () => { await follow(person, "person", post.author_id, !post.following); await load(); })} onSave={() => void act(async () => { await feedback(person, id, "saved", !post.saved); await load(); })}/><View style={{ padding: 20, gap: 24 }}><RudiButton tone="ai" label="@Nếp · Giúp giữ khoảnh khắc" icon="sparkles-outline" variant="outline" onPress={() => setNep(true)}/>{!commentsOpen ? <Comments person={person} post={post} expanded/> : null}</View></> : !error ? <Text style={[typography.body, { color: colors.inkSoft, padding: 20 }]}>Đang mở câu chuyện…</Text> : null}
  </RudiScreen>;
}
