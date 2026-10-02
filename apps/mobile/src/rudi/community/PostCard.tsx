import { Ionicons } from "@expo/vector-icons";
import { Image } from "expo-image";
import { useRouter } from "expo-router";
import { memo, useEffect, useState } from "react";
import { AppState, Platform, Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import { useVideoPlayer, VideoView } from "expo-video";
import { BASE_URL } from "../../api";
import { headerNguoiGoi } from "../../danh-tinh";
import { typography, useRudiTheme } from "../theme";
import { Avatar } from "../ui/Avatar";
import { PressScale } from "../ui/PressScale";
import { useMotion } from "../ui/useMotion";
import { imageSource, relativeTime, type Media, type Post } from "./api";
import { PhotoViewer } from "../ui/PhotoViewer";
import { BookView } from "../diary/BookView";
import { chiaSe, type KetQuaChiaSe } from "../web/chia-se";
import { giuState } from "../../ui/a11y";
import { thanBiCat } from "./bang-tin";

export function CommunityVideo({ media, person, active, khung }: { media: Media; person: string; active: boolean; khung?: { width: number; height: number } }) {
  const { colors } = useRudiTheme();
  const [webSource, setWebSource] = useState<{ key: string; uri: string } | null>(null);
  const [videoError, setVideoError] = useState(false);
  const sourceKey = `${person}:${media.id}`;
  useEffect(() => {
    if (Platform.OS !== "web" || !active) return;
    const abort = new AbortController();
    let objectURL: string | null = null;
    setVideoError(false);
    const load = async () => {
      // HTML video cannot attach bearer headers; use an authorized ephemeral blob.
      const headers = headerNguoiGoi(person);
      const response = await fetch(
        BASE_URL + `/v2/community/media/${media.id}`, { headers, cache: "no-store", signal: abort.signal });
      if (!response.ok || Number(response.headers.get("Content-Length")) > 64 * 1024 * 1024) throw new Error("video_unavailable");
      const blob = await response.blob();
      if (abort.signal.aborted) return;
      if (blob.size > 64 * 1024 * 1024) throw new Error("video_too_large");
      objectURL = URL.createObjectURL(blob);
      setWebSource({ key: sourceKey, uri: objectURL });
    };
    void load().catch(() => { if (!abort.signal.aborted) setVideoError(true); });
    return () => { abort.abort(); if (objectURL) URL.revokeObjectURL(objectURL); setWebSource(null); };
  }, [active, media.id, person, sourceKey]);
  const headers = headerNguoiGoi(person);
  const webURI = active && webSource?.key === sourceKey ? webSource.uri : null;
  const player = useVideoPlayer(
    Platform.OS === "web" ? webURI : { uri: BASE_URL + media.url, headers, useCaching: false }, (p) => { p.loop = false; p.muted = true; });
  useEffect(() => { if (!active) player.pause(); const subscription = AppState.addEventListener("change", (state) => { if (state !== "active") player.pause(); }); return () => subscription.remove(); }, [active, player]);
  return <View><VideoView player={player} nativeControls fullscreenOptions={{ enable: true }} style={khung ?? styles.media} contentFit="contain" />{videoError ? <Text accessibilityRole="alert" style={[typography.caption, { color: colors.inkSoft, padding: 12 }]}>Chưa mở được video. Mở lại câu chuyện để thử lại nhé.</Text> : null}</View>;
}
export function Action({ icon, label, accessibilityLabel, onPress, selected = false, disabled = false }: { icon: keyof typeof Ionicons.glyphMap; label: string; accessibilityLabel?: string; onPress: () => void; selected?: boolean; disabled?: boolean }) {
  const { colors } = useRudiTheme();
  return <PressScale accessibilityRole="button" accessibilityLabel={accessibilityLabel ?? label} {...giuState(Boolean(selected))} aria-disabled={disabled} disabled={disabled} onPress={onPress} style={[styles.action, disabled && { opacity: 0.45 }]}>
    <Ionicons name={icon} size={21} color={selected ? colors.accent : colors.inkSoft} />
    <Text style={[typography.caption, { color: selected ? colors.accent : colors.inkSoft }]}>{label}</Text>
  </PressScale>;
}
export const PostCard = memo(function PostCard({ post, person, active = false, onLike, onComment, onMore, onTopic, onFollow, busy = false, detail = false }: {
  post: Post; person: string; active?: boolean; onLike: () => void; onComment: () => void; onMore: () => void; onTopic?: (topic: string) => void; onFollow: () => void; busy?: boolean; detail?: boolean;
}) {
  const { colors } = useRudiTheme(); const router = useRouter(); const motion = useMotion(); const [expanded, setExpanded] = useState(detail); const [photo, setPhoto] = useState<Media | null>(null);
  // A body that already shows whole opens the post on the first tap; only a
  // cut one spends that tap on the rest of itself (QA UI-139).
  const biCat = thanBiCat(post.body);
  // The album's own width: a frame never wider than the space it sits in
  // (QA UI-143: a fixed 296 in a 288 album lost 8px at 320dp).
  const [rongAlbum, setRongAlbum] = useState(0);
  const rongKhung = rongAlbum > 0 ? Math.min(296, rongAlbum) : 296;
  const khung = { width: rongKhung, height: Math.round((rongKhung * 330) / 296) };
  // What «Chia sẻ» just did, said on the button itself for a few seconds (QA UI-136).
  const [daChiaSe, setDaChiaSe] = useState<string | null>(null);
  useEffect(() => {
    if (daChiaSe === null) return;
    const hen = setTimeout(() => setDaChiaSe(null), 4000);
    return () => clearTimeout(hen);
  }, [daChiaSe]);
  return <View testID={`community-post-${post.id}`} style={[styles.post, { borderBottomColor: colors.line }]}>
    <View style={styles.identity}>
      <Pressable accessibilityRole="button" accessibilityLabel={`Hồ sơ ${post.author}`} onPress={() => router.push(`/people/${post.author_id}`)} style={styles.avatarTarget}><Avatar name={post.author} size={42} /></Pressable>
      <View style={styles.identityText}><Text style={[typography.title, { color: colors.ink }]}>{post.author}</Text><Text style={[typography.caption, { color: colors.inkFaint }]}>{relativeTime(post.created_at)} · {post.audience === "public" ? "Cộng đồng" : post.audience === "friends" ? "Bạn bè" : post.audience === "group" ? "Trong nhóm" : "Chỉ mình tôi"}</Text></View>
      {post.author_id !== person ? <Pressable accessibilityRole="button" accessibilityLabel={post.following ? "Bỏ theo dõi tác giả" : "Theo dõi tác giả"} onPress={onFollow} style={styles.follow}><Ionicons name={post.following ? "checkmark" : "add"} size={20} color={colors.accent} /></Pressable> : null}
      <Pressable accessibilityRole="button" accessibilityLabel="Thêm lựa chọn cho bài" onPress={onMore} style={styles.follow}><Ionicons name="ellipsis-horizontal" size={20} color={colors.inkSoft} /></Pressable>
    </View>
    {post.author_id === person && ["pending", "review", "rejected"].includes(post.status) ? <View style={[styles.status, { backgroundColor: colors.accentSoft }]}><Ionicons name={post.status === "rejected" ? "alert-circle-outline" : "time-outline"} size={17} color={colors.accent} /><Text style={[typography.caption, { color: colors.accent }]}>{post.status === "rejected" ? "Chưa phù hợp cộng đồng · Có thể sửa hoặc yêu cầu xem xét" : "Đang chờ duyệt · Bản mới chưa xuất hiện công khai"}</Text></View> : null}
    <Pressable accessibilityRole="button" accessibilityLabel="Đọc toàn bộ câu chuyện" onPress={() => { if (!expanded && biCat) setExpanded(true); else if (!detail) router.push(`/community/posts/${post.id}` as never); }}>
      <Text numberOfLines={expanded ? undefined : 6} style={[typography.body, styles.body, { color: colors.ink }]}>{post.body}</Text>
      {!expanded && biCat ? <Text style={[typography.label, { color: colors.accent }]}>Đọc tiếp</Text> : null}
    </Pressable>
    {post.diary ? <BookView compact={!detail} kind={post.diary_kind} document={post.diary} photo={(id) => imageSource(person, `/v2/community/media/${id}`)} /> : null}
    {!post.diary && post.media.length ? <ScrollView horizontal onLayout={(e) => { const w = Math.floor(e.nativeEvent.layout.width); if (w > 0 && w !== rongAlbum) setRongAlbum(w); }} pagingEnabled showsHorizontalScrollIndicator={false} style={styles.album} contentContainerStyle={{ gap: 8 }}>
      {post.media.map((m) => <View key={m.id} style={[styles.mediaFrame, { width: rongKhung, backgroundColor: colors.paperShade }]}>{m.type.startsWith("video/") ? <CommunityVideo media={m} person={person} active={active} khung={khung} /> : <Pressable accessibilityRole="button" accessibilityLabel="Mở ảnh khoảnh khắc" onPress={() => setPhoto(m)}><Image source={imageSource(person, m.url)} accessibilityLabel="Ảnh trong bài đăng" cachePolicy="none" contentFit="cover" style={khung} transition={Platform.OS === "web" ? 0 : motion.ms("standard")} /></Pressable>}</View>)}
    </ScrollView> : null}
    {photo ? <PhotoViewer title="Ảnh khoảnh khắc" photos={post.media.filter((m) => m.type.startsWith("image/")).map((m) => ({ id: m.id, source: imageSource(person, m.url), caption: post.body }))} initialIndex={post.media.filter((m) => m.type.startsWith("image/")).findIndex((m) => m.id === photo.id)} onClose={() => setPhoto(null)} /> : null}
    {post.topics.length ? <View style={styles.topics}>{post.topics.map((t) => <Pressable key={t} accessibilityRole="button" onPress={() => onTopic ? onTopic(t) : router.push({ pathname: "/community/topic", params: { topic: t } } as never)} style={[styles.topic, { backgroundColor: colors.accentSoft }]}><Text style={[typography.caption, { color: colors.accent }]}>{t}</Text></Pressable>)}</View> : null}
    <View style={styles.actions}>
      <Action icon={post.liked ? "heart" : "heart-outline"} label={`${post.likes || "Thích"}`} accessibilityLabel={`${post.liked ? "Bỏ thích bài" : "Thích bài"}, ${post.likes} lượt thích`} selected={post.liked} disabled={busy} onPress={onLike} />
      <Action icon="chatbubble-outline" label={`${post.comments || "Bình luận"}`} accessibilityLabel={`Mở bình luận, ${post.comments} bình luận`} onPress={onComment} />
      <Action icon="paper-plane-outline" label={daChiaSe ?? "Chia sẻ"} accessibilityLabel={daChiaSe ?? "Chia sẻ bài"} onPress={() => { void chiaSe({ title: "Bài trên Rủ Đi", url: linkBai(post.id) }).then((kq) => setDaChiaSe(cauChiaSe(kq))); }} />
    </View>
  </View>;
});
const styles = StyleSheet.create({
  post: { paddingHorizontal: 20, paddingTop: 24, paddingBottom: 16, borderBottomWidth: StyleSheet.hairlineWidth, gap: 14 },
  avatarTarget: { minWidth: 48, minHeight: 48, alignItems: "center", justifyContent: "center" },
  identity: { flexDirection: "row", alignItems: "center", gap: 10 }, identityText: { flex: 1, gap: 2 }, follow: { width: 48, height: 48, alignItems: "center", justifyContent: "center" },
  body: { lineHeight: 26 }, status: { padding: 12, borderRadius: 8, flexDirection: "row", alignItems: "center", gap: 8 },
  album: { marginHorizontal: -4 }, mediaFrame: { borderRadius: 14, overflow: "hidden" }, media: { width: 296, height: 330 }, expandedPhoto: { height: 480 },
  topics: { flexDirection: "row", flexWrap: "wrap", gap: 8 }, topic: { paddingHorizontal: 12, paddingVertical: 10, borderRadius: 8, minHeight: 48, minWidth: 48, justifyContent: "center" },
  actions: { flexDirection: "row", gap: 12, flexWrap: "wrap" }, action: { flexDirection: "row", alignItems: "center", gap: 7, minHeight: 48, minWidth: 56, paddingHorizontal: 2 },
});

/**
 * The post's address to share. The web build shares the page it is on, which a
 * browser can open; the native build still has only the app scheme until the
 * product has a public https host for posts (proposal in the UI/UX handoff).
 */
function linkBai(id: string): string {
  if (Platform.OS === "web" && typeof window !== "undefined") return `${window.location.origin}/community/posts/${id}`;
  return `rudi://community/posts/${id}`;
}

function cauChiaSe(ketQua: KetQuaChiaSe): string | null {
  if (ketQua === "da-chep") return "Đã chép link";
  if (ketQua === "khong-duoc") return "Chưa chia sẻ được";
  return null;
}
