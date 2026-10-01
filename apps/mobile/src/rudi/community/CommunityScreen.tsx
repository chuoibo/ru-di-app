import { translatedAsActor, newAttempt } from "../../api";
import { COMMUNITY_ERRORS } from "./api";
import { Ionicons } from "@expo/vector-icons";
import { useFocusEffect, useLocalSearchParams, useRouter } from "expo-router";
import { useCallback, useRef, useState } from "react";
import { FlatList, Pressable, RefreshControl, StyleSheet, Text, TextInput, View, type ViewToken } from "react-native";
import Animated, { FadeIn, FadeOut } from "react-native-reanimated";
import { duongDangNhap } from "../duong-vao";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { RudiButton, RudiScreen } from "../ui";
import { Sheet } from "../ui/Sheet";
import { SkeletonLines } from "../ui/Skeleton";
import { useMotion } from "../ui/useMotion";
import { KHONG_VIEN_WEB } from "../ui/khong-vien-web";
import { feedback, follow, likePost, mergePosts, readFeed, type FeedMode, type Post, type Preferences } from "./api";
import { PostCard } from "./PostCard";
import { Comments } from "./Comments";
import { useCommunityStream } from "./useCommunityStream";
import { useViewSignal } from "./useViewSignal";
const TABS: {
    mode: FeedMode;
    label: string;
}[] = [{ mode: "for_you", label: "Dành cho bạn" }, { mode: "following", label: "Đang theo dõi" }, { mode: "trending", label: "Thịnh hành" }];
export function CommunityScreen() {
    const router = useRouter();
    const params = useLocalSearchParams<{
        topic?: string;
    }>();
    const { phien } = useRudiSession();
    const person = phien?.person_id;
    const { colors } = useRudiTheme();
    const motion = useMotion();
    const [mode, setMode] = useState<FeedMode>("for_you");
    const [posts, setPosts] = useState<Post[]>([]);
    const [next, setNext] = useState<string | null>(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [settings, setSettings] = useState(false);
    const [prefs, setPrefs] = useState<Preferences | null>(null);
    const [fresh, setFresh] = useState(false);
    const [selected, setSelected] = useState<Post | null>(null);
    const [comments, setComments] = useState<Post | null>(null);
    const [busy, setBusy] = useState<string | null>(null);
    const [active, setActive] = useState<string | null>(null);
    const [search, setSearch] = useState("");
    const [searchFocused, setSearchFocused] = useState(false);
    const list = useRef<FlatList<Post>>(null);
    const generation = useRef(0);
    const fetching = useRef(false);
    const personRef = useRef(person);
    personRef.current = person;
    useViewSignal(person, active, prefs?.personalized === true);
    const load = useCallback(async (after?: string | null) => {
        if (!person || fetching.current)
            return;
        fetching.current = true;
        const seq = ++generation.current;
        setLoading(true);
        setError(null);
        try {
            const page = await readFeed(person, mode, after, params.topic);
            if (seq !== generation.current)
                return;
            setPosts((old) => after ? mergePosts(old, page.posts) : page.posts);
            setNext(page.next_cursor);
            if (!after)
                setFresh(false);
        }
        catch (e) {
            if (seq === generation.current)
                setError(e instanceof Error ? e.message : "Chưa đọc được bảng tin.");
        }
        finally {
            fetching.current = false;
            if (seq === generation.current)
                setLoading(false);
        }
    }, [person, mode, params.topic]);
    useFocusEffect(useCallback(() => { fetching.current = false; setPosts([]); void load(); if (person)
        void translatedAsActor<Preferences>(COMMUNITY_ERRORS, "/v2/community/preferences", {
            actorId: person,
            method: "GET"
        }).then(setPrefs).catch(() => { }); return () => { generation.current++; setActive(null); }; }, [load, person]));
    const refresh = async () => { await load(); list.current?.scrollToOffset({ offset: 0, animated: !motion.reduced }); };
    const connected = useCommunityStream(person, [], (e) => { if (e.kind === "sync") {
        setPosts([]);
        void load();
    }
    else
        setFresh(true); });
    const update = (p: Post) => { setPosts((items) => items.map((item) => item.id === p.id ? p : item)); };
    const act = async (task: () => Promise<unknown>) => { try {
        await task();
    }
    catch (e) {
        setError(e instanceof Error ? e.message : "Chưa thực hiện được thao tác.");
    } };
    const like = async (p: Post) => { if (!person || busy)
        return; setBusy(p.id); update({ ...p, liked: !p.liked, likes: Math.max(0, p.likes + (p.liked ? -1 : 1)) }); try {
        update(await likePost(person, p.id, !p.liked));
    }
    catch (e) {
        update(p);
        setError(e instanceof Error ? e.message : "Chưa lưu được lượt thích.");
    }
    finally {
        setBusy(null);
    } };
    const viewable = useRef(({ viewableItems }: {
        viewableItems: ViewToken<Post>[];
    }) => { const first = viewableItems.find((v) => v.isViewable); setActive(first?.item.id ?? null); });
    const consent = async (enabled: boolean) => { if (!person)
        return; await act(async () => { setPrefs(await translatedAsActor<Preferences>(COMMUNITY_ERRORS, "/v2/community/preferences", {
        actorId: person,
        method: "PUT",
        body: { personalized: enabled },
        attempt: newAttempt()
    })); await load(); }); };
    if (!person)
        return <RudiScreen><Text style={[typography.display, { color: colors.ink }]}>Những cuộc đi, những câu chuyện.</Text><Text style={[typography.body, { color: colors.inkSoft }]}>Đăng nhập để gặp cộng đồng Rủ Đi và kể về ngày của bạn.</Text><RudiButton label="Đăng nhập" onPress={() => router.push(duongDangNhap("/community") as never)}/></RudiScreen>;
    return <RudiScreen scroll={false} padded={false} bottomInset={0} testID="community-screen" overlay={<>
    <Sheet open={settings} onClose={() => setSettings(false)} accessibilityLabel="Bảng tin của bạn"><Text style={[typography.h1, { color: colors.ink }]}>Bảng tin của bạn</Text><Text style={[typography.body, { color: colors.inkSoft }]}>Chỉ dùng những gì bạn xem và tương tác trong cộng đồng. Bạn luôn có thể đổi ý.</Text><RudiButton label={prefs?.personalized ? "Tắt cá nhân hóa" : "Bật cá nhân hóa"} onPress={() => void consent(!prefs?.personalized)}/><RudiButton label="Xóa lịch sử đề xuất" variant="outline" onPress={() => void act(async () => { await translatedAsActor(COMMUNITY_ERRORS, "/v2/community/history", {
            actorId: person,
            method: "DELETE",
            attempt: newAttempt()
        }); await load(); setSettings(false); })}/><RudiButton label="Bài đã lưu" variant="ghost" onPress={() => { setMode("saved"); setSettings(false); }}/><RudiButton label="Bài của tôi · Trạng thái duyệt" variant="ghost" onPress={() => { setMode("mine"); setSettings(false); }}/><RudiButton label="Điều mình muốn giữ" variant="ghost" onPress={() => { setSettings(false); router.push("/community/keeps" as never); }} /><RudiButton label="Thông báo" variant="ghost" onPress={() => { setSettings(false); router.push("/community/notifications" as never); }}/></Sheet>
    <Sheet open={selected !== null} onClose={() => setSelected(null)} accessibilityLabel="Lựa chọn cho bài đăng">{selected ? <><Text style={[typography.h2, { color: colors.ink }]}>Câu chuyện này</Text><Text style={[typography.body, { color: colors.inkSoft }]}>{selected.why}</Text><RudiButton label={selected.saved ? "Bỏ lưu" : "Lưu để đọc lại"} onPress={() => void act(async () => { await feedback(person, selected.id, "saved", !selected.saved); update({ ...selected, saved: !selected.saved }); setSelected(null); })}/><RudiButton label="Không quan tâm" variant="outline" onPress={() => void act(async () => { await feedback(person, selected.id, "hidden", true); setPosts((items) => items.filter((p) => p.id !== selected.id)); setSelected(null); })}/>{selected.author_id === person ? <RudiButton label="Mở bài và quản lý" variant="ghost" onPress={() => { router.push(`/community/posts/${selected.id}` as never); setSelected(null); }}/> : <RudiButton label="Báo cáo bài viết" variant="ghost" onPress={() => { router.push({ pathname: "/community/posts/[id]", params: { id: selected.id, report: "1" } } as never); setSelected(null); }}/>}</> : null}</Sheet>
    <Sheet open={comments !== null} onClose={() => setComments(null)} accessibilityLabel="Bình luận">{comments ? <Comments person={person} post={comments}/> : null}</Sheet>
  </>}>
    <View style={styles.readingColumn}>
    <View style={[styles.header, { borderBottomColor: colors.line }]}>
      <View style={styles.headingRow}><View style={{ flex: 1 }}><Text style={[typography.display, { color: colors.ink }]}>{params.topic ?? "Cộng đồng"}</Text><Text style={[typography.caption, { color: colors.inkFaint }]}>{connected ? "Những câu chuyện đang tiếp nối" : "Kết nối những cuộc đi"}</Text></View><Pressable accessibilityRole="button" accessibilityLabel="Cài đặt bảng tin" onPress={() => setSettings(true)} style={styles.icon}><Ionicons name="options-outline" size={24} color={colors.ink}/></Pressable><Pressable accessibilityRole="button" accessibilityLabel="Đăng khoảnh khắc" onPress={() => router.push("/community/new" as never)} style={[styles.icon, { backgroundColor: colors.accent, borderRadius: 16 }]}><Ionicons name="create-outline" size={24} color={colors.accentInk}/></Pressable></View>
      <View style={styles.tabs}>{TABS.map((t) => <Pressable key={t.mode} accessibilityRole="tab" accessibilityState={{ selected: mode === t.mode }} onPress={() => { setMode(t.mode); motion.haptic.select(); }} style={[styles.tab, { borderBottomColor: mode === t.mode ? colors.accent : "transparent" }]}><Text style={[typography.label, { color: mode === t.mode ? colors.accent : colors.inkFaint }]}>{t.label}</Text></Pressable>)}</View>
    </View>
    {fresh ? <Animated.View entering={FadeIn.duration(motion.ms("standard")).reduceMotion(motion.reanimated)} exiting={FadeOut.duration(motion.ms("standard")).reduceMotion(motion.reanimated)} style={styles.newPosts}><RudiButton compact label="Bảng tin có cập nhật" icon="arrow-up" onPress={() => void refresh()}/></Animated.View> : null}
    <FlatList ref={list} data={posts} keyExtractor={(p) => p.id} renderItem={({ item }) => <PostCard post={item} person={person} active={active === item.id} busy={busy === item.id} onLike={() => void like(item)} onComment={() => setComments(item)} onMore={() => setSelected(item)} onFollow={() => void act(async () => { await follow(person, "person", item.author_id, !item.following); update({ ...item, following: !item.following }); })} onTopic={(topic) => router.push({ pathname: "/community/topic", params: { topic } } as never)}/>} viewabilityConfig={{ itemVisiblePercentThreshold: 60, minimumViewTime: 1000 }} onViewableItemsChanged={viewable.current} initialNumToRender={5} windowSize={7} maxToRenderPerBatch={5} removeClippedSubviews={false} contentContainerStyle={styles.list} refreshControl={<RefreshControl refreshing={loading && posts.length > 0} onRefresh={() => void refresh()} tintColor={colors.accent}/>} onEndReached={() => { if (next && !loading)
        void load(next); }} onEndReachedThreshold={0.4} ListHeaderComponent={<View style={styles.intro}>{mode === "mine" || mode === "saved" ? <Text style={[typography.h2, { color: colors.ink }]}>{mode === "mine" ? "Những điều bạn đã kể" : "Để dành cho một ngày"}</Text> : null}<View style={[styles.search, { borderColor: searchFocused ? colors.accent : colors.line }]}><Ionicons name="search-outline" size={19} color={colors.inkFaint}/><TextInput accessibilityLabel="Tìm chủ đề" placeholder="Đi đâu, ăn gì, trải nghiệm gì?" placeholderTextColor={colors.inkFaint} onFocus={() => setSearchFocused(true)} onBlur={() => setSearchFocused(false)} style={[typography.body, KHONG_VIEN_WEB, { flex: 1, color: colors.ink }]} value={search} onChangeText={setSearch} onSubmitEditing={() => { if (search.trim())
        router.push({ pathname: "/community/search", params: { q: search.trim() } } as never); }} returnKeyType="search"/></View>{prefs && !prefs.asked ? <View style={[styles.consent, { backgroundColor: colors.paper }]}><Text style={[typography.h2, { color: colors.ink }]}>Một góc hợp với bạn</Text><Text style={[typography.body, { color: colors.inkSoft }]}>Cho phép học từ tương tác cộng đồng? Không đọc chat hay sổ riêng.</Text><View style={styles.row}><RudiButton full={false} compact label="Cá nhân hóa" onPress={() => void consent(true)}/><Pressable accessibilityRole="button" onPress={() => void consent(false)} style={styles.later}><Text style={[typography.label, { color: colors.ink }]}>Để sau</Text></Pressable></View></View> : null}{error ? <View accessibilityRole="alert" style={styles.notice}><Text style={[typography.body, { color: colors.accent }]}>{error}</Text><RudiButton compact label="Thử lại" variant="ghost" onPress={() => void load()}/></View> : null}</View>} ListEmptyComponent={loading ? <View style={styles.intro}><SkeletonLines lines={4}/></View> : !error ? <View style={styles.empty}><Ionicons name="trail-sign-outline" size={42} color={colors.accent}/><Text style={[typography.h1, { color: colors.ink }]}>{mode === "following" ? "Câu chuyện bắt đầu từ một người" : "Một ngày đáng kể"}</Text><Text style={[typography.body, { color: colors.inkSoft }]}>{mode === "following" ? "Theo dõi tác giả hoặc chủ đề bạn thích. Những cuộc đi của họ sẽ gặp bạn ở đây." : "Một quán nhỏ, một cung đường, một buổi đi chơi. Kể điều bạn muốn giữ lại."}</Text><RudiButton label="Kể khoảnh khắc đầu tiên" onPress={() => router.push("/community/new" as never)}/></View> : null} ListFooterComponent={loading && posts.length ? <Text style={[typography.caption, styles.intro, { color: colors.inkFaint }]}>Đang mở thêm câu chuyện…</Text> : null}/>
    </View>
  </RudiScreen>;
}
const styles = StyleSheet.create({ readingColumn: { flex: 1, width: "100%", maxWidth: 560, alignSelf: "center" }, header: { paddingHorizontal: 20, paddingTop: 12, borderBottomWidth: StyleSheet.hairlineWidth }, headingRow: { flexDirection: "row", alignItems: "center", gap: 8 }, icon: { width: 48, height: 48, alignItems: "center", justifyContent: "center" }, tabs: { flexDirection: "row", marginTop: 14, gap: 18 }, tab: { minHeight: 48, justifyContent: "center", borderBottomWidth: 2 }, list: { paddingBottom: 32 }, intro: { padding: 20, gap: 16 }, search: { flexDirection: "row", gap: 12, alignItems: "center", minHeight: 48, borderBottomWidth: 1, paddingBottom: 8 }, consent: { padding: 20, gap: 12, borderRadius: 16 }, row: { flexDirection: "row", gap: 12 }, later: { minHeight: 48, paddingHorizontal: 16, justifyContent: "center" }, notice: { gap: 8 }, empty: { margin: 24, paddingVertical: 36, gap: 20, alignItems: "flex-start" }, newPosts: { position: "absolute", top: 136, alignSelf: "center", zIndex: 3 } });
