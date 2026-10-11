import { ApiError, translatedAsActor, newAttempt } from "../../api";
import { COMMUNITY_ERRORS } from "./api";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { ScrollView, View } from "react-native";
import { Text } from "../ui/NativeFontMetrics";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { Field, RudiButton, RudiScreen, TopBar } from "../ui";
import { Sheet } from "../ui/Sheet";
import { CauTaiCho } from "../ui/CauTaiCho";
import { NoiDungBaoCao } from "../screens/nguoi/NoiDungBaoCao";
import { feedback, follow, getPost, likePost, type Post } from "./api";
import { Comments, type CommentDraft } from "./Comments";
import { PostCard } from "./PostCard";
import { SkeletonLines } from "../ui/Skeleton";
import { useCommunityStream } from "./useCommunityStream";
import { useAdaptiveLayout } from "../ui/useAdaptiveLayout";
import { COT_DOC } from "../adaptive";
import { useSafeAreaInsets } from "react-native-safe-area-context";
export function PostDetail() {
    const { id, report: reportParam } = useLocalSearchParams<{
        id: string;
        report?: string;
    }>();
    const router = useRouter();
    const { phien } = useRudiSession();
    const person = phien?.person_id;
    const { colors } = useRudiTheme();
    const layout = useAdaptiveLayout();
    const insets = useSafeAreaInsets();
    const tablet = layout.sizeClass !== "compact";
    const [post, setPost] = useState<Post | null>(null);
    const [permanent, setPermanent] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [actions, setActions] = useState(false);
    const [report, setReport] = useState(reportParam === "1");
    const comments = useCommentHandoff(person, id);
    const commentContext = JSON.stringify([person ?? null, id ?? null]);
    const bodyScroll = useMemo(() => ({ y: 0 }), [commentContext]);
    const activePostContext = useRef(commentContext);
    activePostContext.current = commentContext;
    const [postContext, setPostContext] = useState<string | null>(null);
    const commentPost = post && post.id === id && postContext === commentContext ? post : null;
    const read = useRef(0);
    const working = useRef(false);
    const keepAttempt = useRef(newAttempt().key);
    const [nep, setNep] = useState(false);
    const [request, setRequest] = useState("");
    const [draft, setDraft] = useState("");
    const [busy, setBusy] = useState(false);
    const [confirmDelete, setConfirmDelete] = useState(false);
    // The Nếp call's own failure, said in the Nếp sheet: the screen's error
    // line sits under the sheet's scrim, where nobody reads it (QA UI-138).
    const [loiNep, setLoiNep] = useState<string | null>(null);
    // A failed re-read keeps the post already on screen; only a first read that
    // fails leaves the page without one.
    const load = useCallback(async () => { if (!person || !id)
        return; const serial = ++read.current; try {
        const result = await getPost(person, id);
        if (read.current !== serial || activePostContext.current !== commentContext) return;
        setPost(result); setPostContext(commentContext);
        setPermanent(false); setError(null);
    }
    catch (e) {
        if (read.current !== serial || activePostContext.current !== commentContext) return;
        const denied = e instanceof ApiError && (e.status === 403 || e.status === 404);
        setPermanent(denied); if (denied) setPost(null);
        setError(e instanceof Error ? e.message : "Chưa mở được bài.");
    } }, [person, id, commentContext]);
    useEffect(() => { setPost(null); setPostContext(null); void load(); return () => { read.current++; }; }, [load]);
    // A (re)connection re-reads the post in place: the card, the comment box
    // with what is being typed, and an open Nếp sheet with its draft all stay
    // (QA UI-134; ADR-0040 «phục hồi sau reconnect, không tự đẩy vị trí cuộn»).
    useCommunityStream(person, id ? [id] : [], (e) => { if (e.kind !== "feed.changed")
        void load(); });
    const goiNep = async () => {
        if (!person || !post || working.current || !request.trim()) return;
        working.current = true;
        setBusy(true);
        setLoiNep(null);
        try {
            const result = await translatedAsActor<{ draft: string }>(COMMUNITY_ERRORS, `/v2/community/posts/${id}/nep`, {
                actorId: person,
                method: "POST",
                body: { confirmed: true, excerpt: post.body, request },
                attempt: newAttempt()
            });
            setDraft(result.draft);
            keepAttempt.current = newAttempt().key;
        }
        catch (e) {
            setLoiNep(e instanceof Error ? e.message : "Nếp chưa trả lời được. Nội dung của bạn vẫn ở đây.");
        }
        finally {
            working.current = false; setBusy(false);
        }
    };
    const act = async (f: () => Promise<unknown>) => { if (working.current)
        return; working.current = true; setBusy(true); setError(null); try {
        await f();
    }
    catch (e) {
        const message = e instanceof Error ? e.message : "Chưa thực hiện được.";
        if (nep) setLoiNep(message); else setError(message);
    }
    finally {
        working.current = false; setBusy(false);
    } };
    // The shared excerpt scrolls; the active editor stays above the IME.
    // A generated draft keeps the original request editable in the reference
    // area, so asking again never silently replaces the person's input.
    const requestField = <Field testID="community-nep-request" label="Bạn muốn Nếp giúp gì?" value={request} onChangeText={setRequest} editable={!busy} multiline placeholder="Viết một lời ngắn để giữ khoảnh khắc này…" />;
    const requestAction = <RudiButton label={busy ? "Nếp đang viết…" : "Đồng ý chia sẻ và gọi Nếp"} disabled={busy || !request.trim()} lyDo={!busy && !request.trim() ? "Viết một lời nhờ cho Nếp trước." : undefined} onPress={() => void goiNep()} />;
    const draftField = <>
      <Text style={[typography.caption, { color: colors.ai }]}>Bản nháp do Nếp viết</Text>
      <Field testID="community-nep-draft" label="Sửa lại theo ý bạn" value={draft} editable={!busy} onChangeText={(value) => { setDraft(value); keepAttempt.current = newAttempt().key; }} multiline />
    </>;
    const draftAction = <RudiButton label="Lưu ghi chép riêng" variant="outline" disabled={busy || !draft.trim()} lyDo={busy ? "Chờ yêu cầu hiện tại xong." : !draft.trim() ? "Viết nội dung ghi chép trước khi lưu." : undefined} onPress={() => void act(async () => {
      await translatedAsActor(COMMUNITY_ERRORS, "/v2/community/keeps", { actorId: person!, method: "POST", attempt: newAttempt(), body: { logical_id: keepAttempt.current, post_id: id, body: draft, ai_generated: true } });
      setNep(false); router.push("/community/keeps" as never);
    })} />;
    // The decision stays outside overflow. The field may scroll at large text;
    // the reference gives up height first, while the primary action stays put.
    const editorFooter = person && post ? <View style={{ flexShrink: 1, gap: 8 }}>
      <ScrollView testID="community-nep-editor" bounces={false} keyboardShouldPersistTaps="handled" showsVerticalScrollIndicator style={{ flexGrow: 0, flexShrink: 1, minHeight: 128, width: "100%" }} contentContainerStyle={{ gap: 8, paddingBottom: 4 }}>
        {draft ? draftField : requestField}
        <CauTaiCho cau={loiNep} hanhDong={{ label: "Thử lại", onPress: () => void goiNep() }} testID="cong-dong-loi-nep" />
      </ScrollView>
      {draft ? draftAction : requestAction}
    </View> : undefined;
    // The comment owner supplies both slots; the editor never lives in overflow.
    // Keep body scrolling local so footer focus cannot trigger body-field reveal.
    const renderSurface = (thread?: ReactNode, editor?: ReactNode) => <RudiScreen material="quiet" cot="doc" padded={false} scroll={false} avoidKeyboard bottomInset={0} contentStyle={{ paddingTop: 0 }} footer={editor} footerInset={insets.bottom + 12} testID="community-post-detail" overlay={<>
    <Sheet open={actions} onClose={() => { setActions(false); setConfirmDelete(false); }} accessibilityLabel="Quản lý bài">{post && person ? <><Text style={[typography.h2, { color: colors.ink }]}>Giữ lại câu chuyện</Text>{/* A refused management step is said in this sheet, where the press was. */}<CauTaiCho cau={actions ? error : null} /><RudiButton label={post.saved ? "Bỏ lưu bài" : "Lưu bài"} onPress={() => void act(async () => { await feedback(person, id, "saved", !post.saved); await load(); setActions(false); })}/>{post.author_id === person ? <>{!post.diary ? <RudiButton label="Sửa bài" variant="outline" onPress={() => { setActions(false); router.push({ pathname: "/community/new", params: { edit: id } } as never); }}/> : null}<RudiButton label="Cất lại cho bạn bè" variant="outline" onPress={() => void act(async () => { await translatedAsActor(COMMUNITY_ERRORS, `/v2/community/posts/${id}/audience`, {
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
        }); await load(); setActions(false); })}/> : null}{confirmDelete ? <><Text style={[typography.body, { color: colors.inkSoft }]}>Bài và toàn bộ bình luận sẽ bị xóa. Không lấy lại được.</Text><RudiButton label="Giữ bài lại" variant="outline" disabled={busy} onPress={() => setConfirmDelete(false)}/></> : null}<RudiButton tone="warn" disabled={busy} label={confirmDelete ? "Xác nhận xóa bài và bình luận" : "Xóa bài"} variant="ghost" onPress={() => { if (!confirmDelete) {
            setConfirmDelete(true);
            return;
        } void act(async () => { await translatedAsActor(COMMUNITY_ERRORS, `/v2/community/posts/${id}`, {
            actorId: person,
            method: "DELETE",
            attempt: newAttempt()
        }); router.replace("/(tabs)/community" as never); }); }}/></> : <RudiButton label="Báo cáo bài" variant="ghost" onPress={() => { setActions(false); setReport(true); }}/>}</> : null}</Sheet>
    {comments.phase !== "inline" && person && commentPost ? <Comments key={comments.writer} initialDraft={comments.draft} onDraftChange={comments.onDraftChange} person={person} post={commentPost} presentation={(thread, editor) => <Sheet open={comments.phase === "sheet"} onClose={comments.close} onClosed={comments.closed} accessibilityLabel="Bình luận" testID="community-comment-sheet" avoidKeyboard prioritizeFooter footer={editor} showsVerticalScrollIndicator>{thread}</Sheet>} /> : null}
    <Sheet open={report} onClose={() => setReport(false)} accessibilityLabel="Báo cáo bài">{person ? <NoiDungBaoCao actorId={person} loai="post" targetId={id} onXong={() => setReport(false)} onThoi={() => setReport(false)}/> : null}</Sheet>
    <Sheet open={nep} onClose={() => { setNep(false); setLoiNep(null); }} accessibilityLabel="Gọi Nếp" avoidKeyboard showsVerticalScrollIndicator testID="community-nep-sheet" prioritizeFooter footer={editorFooter}>
      {person && post ? <View style={{ paddingRight: 8 }}>
        <Text style={[typography.h2, { color: colors.ink }]}>Nhờ Nếp giữ một điều</Text>
        <Text style={[typography.body, { color: colors.inkSoft }]}>Nếp chỉ nhận đoạn dưới đây và yêu cầu của bạn. Bạn xem lại bản nháp trước khi lưu thành ghi chép riêng.</Text>
        <Text style={[typography.body, { color: colors.ink, paddingVertical: 16 }]}>{post.body}</Text>
        {draft ? <>{requestField}{requestAction}</> : null}
      </View> : null}
    </Sheet>
  </>}>
    <ScrollView testID="community-post-body" keyboardShouldPersistTaps="handled" showsVerticalScrollIndicator={false} style={{ flex: 1 }} contentOffset={{ x: 0, y: bodyScroll.y }} onScroll={(event) => { const y = event.nativeEvent.contentOffset.y; if (Number.isFinite(y) && y >= 0) bodyScroll.y = y; }} scrollEventThrottle={16} contentContainerStyle={[{ gap: 18, paddingTop: tablet ? 22 : 8, paddingBottom: 32 }, tablet && { alignSelf: "center", width: "100%", maxWidth: COT_DOC }]}>
    <View style={{ paddingHorizontal: 20 }}><TopBar title="Một câu chuyện"/><CauTaiCho cau={error} hanhDong={permanent ? undefined : { label: "Thử lại", onPress: () => void load() }} testID="cong-dong-loi-bai" />{permanent ? <RudiButton label="Về cộng đồng" variant="outline" onPress={() => router.replace("/(tabs)/community" as never)}/> : null}</View>
    {post && person ? <><PostCard detail post={post} person={person} active busy={busy} onLike={() => void act(async () => setPost(await likePost(person, id, !post.liked)))} onComment={comments.open} onMore={() => setActions(true)} onFollow={() => void act(async () => { await follow(person, "person", post.author_id, !post.following); await load(); })} onSave={() => void act(async () => { await feedback(person, id, "saved", !post.saved); await load(); })}/><View style={{ padding: 20, gap: 24 }}><RudiButton tone="ai" label="@Nếp · Giúp giữ khoảnh khắc" icon="sparkles-outline" variant="outline" onPress={() => setNep(true)}/>{thread}</View></> : !error ? <View style={{ padding: 20 }}><SkeletonLines lines={5}/></View> : null}
    </ScrollView>
  </RudiScreen>;
    return comments.phase === "inline" && person && commentPost
        ? <Comments key={comments.writer} initialDraft={comments.draft} onDraftChange={comments.onDraftChange} person={person} post={commentPost} expanded presentation={renderSurface}/>
        : renderSurface();
}


type CommentPhase = "inline" | "sheet" | "closing";
/** One writer remains alive through the sheet's real close completion. */
function useCommentHandoff(person: string | undefined, post: string | undefined) {
    const owner = useMemo(() => ({ live: false, writer: 0, phase: "inline" as CommentPhase, draft: null as CommentDraft | null }), [person, post]);
    const active = useRef(owner);
    active.current = owner;
    const [surface, setSurface] = useState(() => ({ owner, phase: "inline" as CommentPhase, writer: 0 }));
    const view = surface.owner === owner ? surface : { owner, phase: "inline" as CommentPhase, writer: 0 };
    const writer = view.writer;
    const current = useCallback(() => owner.live && active.current === owner, [owner]);
    useLayoutEffect(() => {
        owner.live = true;
        return () => { owner.live = false; };
    }, [owner]);
    const onDraftChange = useCallback((draft: CommentDraft) => {
        if (!current() || owner.writer !== writer || draft.person_id !== person || draft.post_id !== post) return;
        owner.draft = draft;
    }, [owner, writer, person, post, current]);
    const open = useCallback(() => {
        // Serialize closing: an old completion must never hide a reopened sheet.
        if (!current() || !person || !post || owner.writer !== writer || owner.phase !== "inline") return;
        owner.phase = "sheet"; owner.writer++;
        setSurface({ owner, phase: owner.phase, writer: owner.writer });
    }, [owner, writer, person, post, current]);
    const close = useCallback(() => {
        if (!current() || owner.writer !== writer || owner.phase !== "sheet") return;
        owner.phase = "closing";
        setSurface({ owner, phase: owner.phase, writer: owner.writer });
    }, [owner, writer, current]);
    const closed = useCallback(() => {
        if (!current() || owner.writer !== writer || owner.phase !== "closing") return;
        // Retire the sheet writer before the inline editor can publish.
        owner.writer++; owner.phase = "inline";
        setSurface({ owner, phase: owner.phase, writer: owner.writer });
    }, [owner, writer, current]);
    return { phase: view.phase, writer, draft: owner.draft, onDraftChange, open, close, closed };
}
