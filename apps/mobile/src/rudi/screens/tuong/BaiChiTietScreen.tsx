import { Ionicons } from "@expo/vector-icons";
import { Image } from "expo-image";
import { useFocusEffect, useLocalSearchParams, useRouter } from "expo-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { AppState, FlatList, Modal, Pressable, StyleSheet, Text, View, useWindowDimensions } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { ApiError, attemptFor, type Attempt, type PostAudience } from "../../../api";
import { nguonAnhBai } from "../../nguoi/anh-ca-nhan";
import { cauLucNao, loiRaChu, nhanMuc } from "../../nguoi/ho-so-nguoi";
import { useRudiSession } from "../../session";
import { bongDen, mucTrenAnh, typography, useRudiTheme } from "../../theme";
import {
  dangLaiBai,
  docBaiTuong,
  docBinhLuanTuong,
  docDoiTuong,
  guiTraLoi,
  thichBaiTuong,
  thichBinhLuan,
  type BaiTuong,
  type BinhLuanTuong,
} from "../../tuong/social-v2";
import { Card, Field, IconButton, RudiButton, RudiScreen, TopBar } from "../../ui";
import { Sheet } from "../../ui/Sheet";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonCard, SkeletonRow } from "../../ui/Skeleton";
import { NoiDungBaoCao } from "../nguoi/NoiDungBaoCao";

type PostState = { phase: "loading" } | { phase: "ready"; post: BaiTuong } | { phase: "error"; message: string };
type CommentState = { phase: "loading" } | { phase: "ready"; items: BinhLuanTuong[]; next: string | null; more: boolean } | { phase: "error"; message: string };

export function BaiChiTietScreen() {
  const router = useRouter();
  const { colors, radius, space } = useRudiTheme();
  const insets = useSafeAreaInsets();
  const { height: windowHeight } = useWindowDimensions();
  const { phien, phienDaDoc } = useRudiSession();
  const params = useLocalSearchParams<{ id?: string; photo?: string }>();
  const postId = params.id ?? "";
  const openPhotoOnArrival = params.photo === "1";
  const actor = phien?.person_id ?? "";
  const attempts = useRef<Record<string, Attempt>>({});
  const changeCursor = useRef<string | null>(null);
  const commentRead = useRef(0);
  const [post, setPost] = useState<PostState>({ phase: "loading" });
  const [comments, setComments] = useState<CommentState>({ phase: "loading" });
  const [draft, setDraft] = useState("");
  const [replyTo, setReplyTo] = useState<{ id: string; name: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [photoError, setPhotoError] = useState(false);
  const [viewerOpen, setViewerOpen] = useState(false);
  const [viewerCommentsOpen, setViewerCommentsOpen] = useState(false);
  const photoOpenedOnArrival = useRef(false);
  const [shareOpen, setShareOpen] = useState(false);
  const [reportOpen, setReportOpen] = useState(false);

  useEffect(() => { photoOpenedOnArrival.current = false; }, [postId]);
  useEffect(() => {
    if (!openPhotoOnArrival || photoOpenedOnArrival.current || post.phase !== "ready" || post.post.id !== postId || !post.post.image_url) return;
    photoOpenedOnArrival.current = true;
    setViewerOpen(true);
    setViewerCommentsOpen(true);
  }, [openPhotoOnArrival, post, postId]);

  const loadPost = useCallback(async (silent = false) => {
    if (!actor || !postId) return;
    if (!silent) setPost({ phase: "loading" });
    try { setPost({ phase: "ready", post: await docBaiTuong(postId, actor) }); }
    catch (cause) {
      if (!silent || (cause instanceof ApiError && (cause.status === 403 || cause.status === 404))) {
        setPost({ phase: "error", message: loiRaChu(cause) });
        setViewerOpen(false);
        setViewerCommentsOpen(false);
      }
    }
  }, [actor, postId]);

  const loadComments = useCallback(async (silent = false) => {
    if (!actor || !postId) return;
    const read = ++commentRead.current;
    if (!silent) setComments({ phase: "loading" });
    try {
      const page = await docBinhLuanTuong(postId, actor);
      if (read !== commentRead.current) return;
      setComments({ phase: "ready", items: page.comments, next: page.next_cursor, more: page.has_more });
    } catch (cause) {
      if (read !== commentRead.current) return;
      if (!silent || (cause instanceof ApiError && (cause.status === 403 || cause.status === 404))) {
        setComments({ phase: "error", message: loiRaChu(cause) });
      }
    }
  }, [actor, postId]);

  useFocusEffect(useCallback(() => {
    void loadPost();
    void loadComments();
    const foreground = AppState.addEventListener("change", (state) => {
      if (state !== "active") return;
      changeCursor.current = null;
      void Promise.all([loadPost(true), loadComments(true)]);
    });
    return () => foreground.remove();
  }, [loadPost, loadComments]));

  const wallOwner = (post.phase === "ready" && post.post.author_id) || "";
  useFocusEffect(useCallback(() => {
    if (!wallOwner || !actor) return;
    changeCursor.current = null;
    let active = true;
    const watch = async () => {
      while (active) {
        try {
          const changes = await docDoiTuong(wallOwner, actor, changeCursor.current);
          if (!active) return;
          changeCursor.current = changes.next_cursor;
          if (changes.events.length === 0 || changes.events.some((event) => event.post_id === postId)) {
            await Promise.all([loadPost(true), loadComments(true)]);
          }
        } catch {
          if (!active) return;
          await new Promise((resolve) => setTimeout(resolve, 3000));
        }
      }
    };
    void watch();
    return () => { active = false; };
  }, [actor, wallOwner, postId, loadPost, loadComments]));

  if (!phienDaDoc) return null;

  const likePost = async () => {
    if (post.phase !== "ready" || busy) return;
    setBusy(true); setError(null);
    try {
      const result = await thichBaiTuong(postId, actor, !post.post.liked);
      setPost({ phase: "ready", post: { ...post.post, liked: result.liked, like_count: result.like_count } });
    } catch (cause) { setError(loiRaChu(cause)); }
    finally { setBusy(false); }
  };

  const likeComment = async (item: BinhLuanTuong) => {
    if (busy) return;
    setBusy(true); setError(null);
    try { await thichBinhLuan(item.id, actor, !item.liked); await loadComments(true); }
    catch (cause) { setError(loiRaChu(cause)); }
    finally { setBusy(false); }
  };

  const sendComment = async () => {
    if (post.phase !== "ready" || !post.post.can_comment || !draft.trim() || busy) return;
    setBusy(true); setError(null);
    try {
      await guiTraLoi(postId, replyTo?.id ?? null, draft, actor, attemptFor(attempts.current, `comment:${postId}:${draft.trim()}:${replyTo?.id ?? "root"}`));
      setDraft(""); setReplyTo(null);
      await Promise.all([loadPost(true), loadComments(true)]);
    } catch (cause) { setError(loiRaChu(cause)); }
    finally { setBusy(false); }
  };

  const loadMore = async () => {
    if (comments.phase !== "ready" || !comments.more || !comments.next || busy) return;
    const read = commentRead.current;
    setBusy(true);
    try {
      const page = await docBinhLuanTuong(postId, actor, comments.next);
      if (read !== commentRead.current) return;
      setComments((current) => {
        if (current.phase !== "ready") return current;
        const known = new Set(current.items.map((item) => item.id));
        return { phase: "ready", items: [...current.items, ...page.comments.filter((item) => !known.has(item.id))], next: page.next_cursor, more: page.has_more };
      });
    } catch (cause) {
      if (read === commentRead.current) setError(loiRaChu(cause));
    }
    finally { setBusy(false); }
  };

  const repost = async (audience: PostAudience) => {
    if (busy) return;
    setBusy(true); setError(null); setNotice(null);
    try {
      await dangLaiBai(postId, audience, actor, attemptFor(attempts.current, `repost:${postId}:${audience}`));
      setShareOpen(false);
      setNotice("Đã chia sẻ lên tường của bạn.");
    } catch (cause) { setError(loiRaChu(cause)); }
    finally { setBusy(false); }
  };

  const commentRow = (item: BinhLuanTuong, nested = false) => (
    <View key={item.id} style={[styles.comment, nested && styles.reply, { borderColor: colors.line }]}>
      <View style={styles.commentHeading}>
        <Text numberOfLines={1} style={[typography.label, { color: colors.ink, flex: 1 }]}>{item.author_id === actor ? "Bạn" : item.author_display_name}</Text>
        <Text style={[typography.note, { color: colors.inkFaint }]}>{cauLucNao(item.created_at)}</Text>
      </View>
      <Text style={[typography.body, { color: colors.ink }]}>{item.body}</Text>
      <View style={styles.commentActions}>
        <Pressable accessibilityLabel={`${item.liked ? "Bỏ thích" : "Thích"} bình luận`} accessibilityRole="button" accessibilityState={{ selected: item.liked }} disabled={busy} onPress={() => void likeComment(item)} style={styles.touchAction}>
          <Ionicons color={item.liked ? colors.accent : colors.inkSoft} name={item.liked ? "heart" : "heart-outline"} size={18} />
          <Text style={[typography.caption, { color: item.liked ? colors.accent : colors.inkSoft }]}>{item.like_count > 0 ? item.like_count : "Thích"}</Text>
        </Pressable>
        {post.phase === "ready" && post.post.can_comment ? (
          <Pressable accessibilityLabel={`Trả lời ${item.author_display_name}`} accessibilityRole="button" onPress={() => {
            let targetId = item.id;
            if (nested && item.parent_id) targetId = item.parent_id;
            setReplyTo({ id: targetId, name: item.author_display_name });
          }} style={styles.touchAction}>
            <Ionicons color={colors.inkSoft} name="chatbubble-outline" size={17} />
            <Text style={[typography.caption, { color: colors.inkSoft }]}>Trả lời</Text>
          </Pressable>
        ) : null}
      </View>
    </View>
  );

  const composer = post.phase === "ready" && post.post.can_comment ? (
    <View style={[styles.composer, { borderColor: colors.line }]}>
      {replyTo ? (
        <Pressable accessibilityRole="button" accessibilityLabel="Huỷ trả lời" onPress={() => setReplyTo(null)}>
          <Text style={[typography.caption, { color: colors.accent }]}>Đang trả lời {replyTo.name} · Huỷ</Text>
        </Pressable>
      ) : null}
      <View style={styles.composeRow}>
        <View style={{ flex: 1 }}><Field accessibilityLabel="Viết bình luận" onChangeText={setDraft} placeholder={replyTo ? "Viết lời đáp…" : "Viết điều bạn muốn nói…"} value={draft} /></View>
        <IconButton accessibilityLabel="Gửi bình luận" disabled={busy || !draft.trim()} icon="arrow-up" onPress={() => void sendComment()} solid />
      </View>
    </View>
  ) : post.phase === "ready" ? <Text style={[typography.note, { color: colors.inkFaint }]}>Chủ tường đã đóng bình luận cho bài này.</Text> : null;

  const commentsBlock = (
    <View style={{ gap: space.sm }}>
      {comments.phase === "loading" ? <SkeletonRow lines={2} /> : null}
      {comments.phase === "error" ? <ErrorState body={comments.message} onRetry={() => void loadComments()} title="Chưa đọc được bình luận" /> : null}
      {comments.phase === "ready" && comments.items.length === 0 ? <EmptyState kind="first-use" layout="inline" title="Chưa có lời nhắn nào" body="Bạn có thể mở lời đầu tiên ở đây." /> : null}
      {comments.phase === "ready" ? comments.items.map((item) => (
        <View key={item.id} style={{ gap: space.xs }}>
          {commentRow(item)}
          {item.replies.map((reply) => commentRow(reply, true))}
        </View>
      )) : null}
      {comments.phase === "ready" && comments.more ? <RudiButton label="Đọc thêm bình luận" loading={busy} onPress={() => void loadMore()} variant="ghost" /> : null}
    </View>
  );

  return (
    <RudiScreen scroll={false} testID="bai-chi-tiet-screen">
      <TopBar title="Trang viết" />
      <FlatList
        contentContainerStyle={{ gap: space.lg, paddingBottom: space.xl }}
        data={[] as string[]}
        renderItem={() => null}
        ListHeaderComponent={
          <View style={{ gap: space.lg }}>
            {post.phase === "loading" ? <SkeletonCard lines={3} media={0} /> : null}
            {post.phase === "error" ? <ErrorState body={post.message} onRetry={() => void loadPost()} title="Chưa mở được bài" /> : null}
            {post.phase === "ready" ? (
              <Card style={styles.postCard}>
                <Pressable accessibilityLabel={`Xem hồ sơ ${post.post.author_display_name}`} accessibilityRole="button" onPress={() => router.push(`/people/${post.post.author_id}` as never)} style={styles.author}>
                  <View style={[styles.authorMark, { backgroundColor: colors.accentSoft }]}><Ionicons color={colors.accent} name="book-outline" size={23} /></View>
                  <View style={{ flex: 1 }}>
                    <Text style={[typography.title, { color: colors.ink }]}>{post.post.author_display_name}</Text>
                    <Text style={[typography.note, { color: colors.inkFaint }]}>{cauLucNao(post.post.created_at)} · {nhanMuc(post.post.audience)}</Text>
                  </View>
                </Pressable>
                <Text style={[typography.body, { color: colors.ink }]}>{post.post.body}</Text>
                {post.post.image_url ? (
                  <Pressable accessibilityLabel="Mở ảnh toàn màn hình và bình luận" accessibilityRole="button" onPress={() => { setViewerOpen(true); setViewerCommentsOpen(true); }} style={{ borderRadius: radius.base, overflow: "hidden" }}>
                    {photoError ? <View style={[styles.image, styles.imageError, { backgroundColor: colors.line }]}><Text style={[typography.note, { color: colors.inkFaint }]}>Chưa tải được ảnh</Text></View> : <Image accessibilityLabel="Ảnh bài đăng" contentFit="cover" onError={() => setPhotoError(true)} source={nguonAnhBai(post.post.image_url, actor)} style={styles.image} />}
                  </Pressable>
                ) : null}
                {post.post.is_repost ? (
                  <View style={[styles.origin, { borderColor: colors.lineStrong }]}>
                    <Text style={[typography.label, { color: colors.ink }]}>{post.post.origin?.author_display_name ?? "Bài gốc không còn xem được"}</Text>
                    {post.post.origin ? <Text style={[typography.body, { color: colors.inkSoft }]}>{post.post.origin.body}</Text> : null}
                  </View>
                ) : null}
                <Text style={[typography.note, { color: colors.inkFaint }]}>{post.post.like_count} thích · {post.post.comment_count} bình luận</Text>
                <View style={[styles.postActions, { borderTopColor: colors.line }]}>
                  <Pressable accessibilityLabel={post.post.liked ? "Bỏ thích bài" : "Thích bài"} accessibilityRole="button" accessibilityState={{ selected: post.post.liked }} disabled={busy} onPress={() => void likePost()} style={styles.postAction}>
                    <Ionicons color={post.post.liked ? colors.accent : colors.inkSoft} name={post.post.liked ? "heart" : "heart-outline"} size={22} />
                    <Text style={[typography.label, { color: post.post.liked ? colors.accent : colors.inkSoft }]}>Thích</Text>
                  </Pressable>
                  <Pressable accessibilityLabel="Chia sẻ bài" accessibilityRole="button" onPress={() => setShareOpen(true)} style={styles.postAction}>
                    <Ionicons color={colors.inkSoft} name="arrow-redo-outline" size={22} />
                    <Text style={[typography.label, { color: colors.inkSoft }]}>Chia sẻ</Text>
                  </Pressable>
                </View>
              </Card>
            ) : null}
            <Text style={[typography.h2, { color: colors.ink }]}>Lời nhắn dưới trang</Text>
            {commentsBlock}
            {composer}
            {error ? <Text accessibilityLiveRegion="polite" style={[typography.note, { color: colors.warn }]}>{error}</Text> : null}
            {notice ? <Text accessibilityLiveRegion="polite" style={[typography.note, { color: colors.accent }]}>{notice}</Text> : null}
            {post.phase === "ready" && post.post.author_id !== actor ? <RudiButton label="Báo cáo bài này" icon="flag-outline" onPress={() => setReportOpen(true)} variant="ghost" /> : null}
          </View>
        }
      />
      <Sheet accessibilityLabel="Chia sẻ bài" onClose={() => setShareOpen(false)} open={shareOpen}>
        <View style={{ gap: space.md }}>
          <Text style={[typography.h2, { color: colors.ink }]}>Chia sẻ trang viết</Text>
          <Text style={[typography.note, { color: colors.inkSoft }]}>Chọn ai được thấy bài đăng lại trên tường của bạn.</Text>
          <Text style={[typography.note, { color: colors.inkSoft }]}>Nội dung bài gốc chỉ hiện cho người đã có quyền xem bài đó.</Text>
          <RudiButton label="Bạn bè của tôi" onPress={() => void repost("friends")} />
          <RudiButton label="Mọi người" onPress={() => void repost("public")} variant="outline" />
          <RudiButton label="Chỉ mình tôi" onPress={() => void repost("only_me")} variant="outline" />
          <RudiButton disabled label="Gửi vào chat" onPress={() => undefined} variant="ghost" />
          <Text style={[typography.note, { color: colors.inkFaint }]}>Gửi bài vào chat sẽ mở khi chat mã hoá đầu cuối sẵn sàng.</Text>
        </View>
      </Sheet>
      <Sheet accessibilityLabel="Báo cáo bài" onClose={() => setReportOpen(false)} open={reportOpen}>
        {post.phase === "ready" ? <NoiDungBaoCao actorId={actor} loai="post" onThoi={() => setReportOpen(false)} onXong={() => setReportOpen(false)} targetId={postId} /> : null}
      </Sheet>
      <Modal animationType="fade" onRequestClose={() => { setViewerCommentsOpen(false); setViewerOpen(false); }} statusBarTranslucent visible={viewerOpen}>
        <View style={[styles.viewer, { backgroundColor: bongDen }]}>
          <Pressable accessibilityLabel="Đóng ảnh" accessibilityRole="button" onPress={() => { setViewerCommentsOpen(false); setViewerOpen(false); }} style={[styles.viewerClose, { top: insets.top + 8 }]}><Ionicons color={mucTrenAnh} name="close" size={28} /></Pressable>
          {post.phase === "ready" && post.post.image_url ? <View style={viewerCommentsOpen ? { height: Math.round(windowHeight * 0.66), justifyContent: "center" } : styles.viewerImage}><Image accessibilityLabel="Ảnh toàn màn hình" contentFit="contain" source={nguonAnhBai(post.post.image_url, actor)} style={viewerCommentsOpen ? { width: "100%", height: Math.max(180, Math.round(windowHeight * 0.32)) } : styles.viewerImage} /></View> : null}
          <Pressable accessibilityLabel="Mở bình luận ảnh" accessibilityRole="button" onPress={() => setViewerCommentsOpen(true)} style={[styles.viewerComments, { bottom: insets.bottom + 16 }]}><Ionicons color={mucTrenAnh} name="chatbubble-outline" size={21} /><Text style={[typography.label, { color: mucTrenAnh }]}>Bình luận</Text></Pressable>
          <Sheet accessibilityLabel="Bình luận ảnh" onClose={() => setViewerCommentsOpen(false)} open={viewerCommentsOpen}>
            <View style={{ gap: space.md }}><Text style={[typography.h2, { color: colors.ink }]}>Lời nhắn dưới ảnh</Text>{commentsBlock}{composer}</View>
          </Sheet>
        </View>
      </Modal>
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  postCard: { gap: 16 },
  author: { flexDirection: "row", alignItems: "center", gap: 11, minHeight: 48 },
  authorMark: { width: 44, height: 44, borderRadius: 13, alignItems: "center", justifyContent: "center" },
  image: { width: "100%", aspectRatio: 4 / 3 },
  imageError: { alignItems: "center", justifyContent: "center" },
  origin: { borderWidth: 1, borderRadius: 12, padding: 13, gap: 6 },
  postActions: { flexDirection: "row", borderTopWidth: StyleSheet.hairlineWidth, paddingTop: 8 },
  postAction: { flex: 1, minHeight: 48, flexDirection: "row", alignItems: "center", justifyContent: "center", gap: 8 },
  comment: { gap: 6, borderBottomWidth: StyleSheet.hairlineWidth, paddingVertical: 11 },
  reply: { marginLeft: 28, paddingLeft: 13, borderLeftWidth: 1 },
  commentHeading: { flexDirection: "row", alignItems: "center", gap: 8 },
  commentActions: { flexDirection: "row", gap: 16 },
  touchAction: { flexDirection: "row", gap: 6, minHeight: 44, alignItems: "center", minWidth: 68 },
  composer: { gap: 8, borderTopWidth: StyleSheet.hairlineWidth, paddingTop: 12 },
  composeRow: { flexDirection: "row", alignItems: "flex-end", gap: 8 },
  viewer: { flex: 1 },
  viewerClose: { position: "absolute", right: 20, zIndex: 4, minWidth: 48, minHeight: 48, alignItems: "center", justifyContent: "center" },
  viewerImage: { flex: 1 },
  viewerComments: { position: "absolute", alignSelf: "center", minHeight: 48, flexDirection: "row", gap: 9, alignItems: "center", paddingHorizontal: 22 },
});
