import { translatedAsActor, newAttempt } from "../../api";
import { COMMUNITY_ERRORS } from "./api";
import { Ionicons } from "@expo/vector-icons";
import { useFocusEffect, useLocalSearchParams, useRouter } from "expo-router";
import { useCallback, useRef, useState } from "react";
import { FlatList, Pressable, RefreshControl, StyleSheet, Text, View, useWindowDimensions, type ViewToken } from "react-native";
import Animated, { FadeIn, FadeOut } from "react-native-reanimated";
import { duongDangNhap } from "../duong-vao";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { RudiButton, RudiScreen, SearchField, TopBar } from "../ui";
import type { DungDau } from "../ui/DauKhamPha";
import { HangChuTab, type MucChuTab } from "../ui/HangChuTab";
import { useChamLaiTab } from "../ui/cham-lai-tab";
import { CauTaiCho } from "../ui/CauTaiCho";
import { Sheet } from "../ui/Sheet";
import { SkeletonLines } from "../ui/Skeleton";
import { useMotion } from "../ui/useMotion";
import { docGiaoDienAsync } from "../kho";
import { feedback, follow, likePost, mergePosts, readFeed, type FeedMode, type Post, type Preferences } from "./api";
import { coThongBaoMoi, doiTheoDoiTacGia, khoaThongBaoDaXem, type ThongBao } from "./bang-tin";
import { PostCard } from "./PostCard";
import { Comments } from "./Comments";
import { useCommunityStream } from "./useCommunityStream";
import { useViewSignal } from "./useViewSignal";
import { TABLIST, tabState } from "../../ui/a11y";

// On the tab the feed's modes are one row of text tabs (owner's choice, 02/10):
// the three feeds, then the reader's own two lists, which used to hide in the
// settings sheet. The hidden posts stay in the sheet: a list one goes to
// rarely, to take a «Không quan tâm» back.
const CHE_DO: readonly MucChuTab<FeedMode>[] = [
  { id: "for_you", nhan: "Dành cho bạn" },
  { id: "following", nhan: "Đang theo dõi" },
  { id: "trending", nhan: "Thịnh hành" },
  { id: "saved", nhan: "Đã lưu" },
  { id: "mine", nhan: "Bài của tôi" },
];
// The three feeds alone, for the screen's own header when it has no Khám phá header.
const TABS = CHE_DO.slice(0, 3).map((c) => ({ mode: c.id, label: c.nhan }));

/**
 * The community feed. On the tab it is Khám phá's second section (owner's
 * mockup, 01/10): the route hands it Khám phá's header (`dau`), the bell and
 * the feed settings ride on that header's right, the feed's modes are text
 * tabs over the list, and writing a post is the «Tạo» stamp's first card, so the screen
 * draws no title or compose button of its own. Opened on a topic
 * (`/community/topic`, a stack route with no strip and no stamp) it keeps its
 * own header.
 */
export function CommunityScreen({ dau }: { dau?: DungDau } = {}) {
  const router = useRouter();
  const params = useLocalSearchParams<{
    topic?: string;
  }>();
  const { phien } = useRudiSession();
  const person = phien?.person_id;
  const { colors } = useRudiTheme();
  const motion = useMotion();
  // Below 360dp three 48dp actions beside the title left it 124px and broke
  // «Cộng đồng» in two: the actions rise above it, the large title keeps the
  // page's width (the large-title bar of a phone's own apps).
  const hep = useWindowDimensions().width < 360;
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
  // Posts hidden here this visit: the card gives way to one line with
  // «Hoàn tác» in its own place, instead of vanishing without a word (QA UI-141).
  const [daAn, setDaAn] = useState<ReadonlySet<string>>(new Set());
  // «Xóa lịch sử đề xuất» asks first, then says it is done (QA UI-141).
  const [xacNhanXoa, setXacNhanXoa] = useState(false);
  const [daXoaLichSu, setDaXoaLichSu] = useState(false);
  // The topic page follows its topic in place (QA UI-146).
  const [theoDoiChuDe, setTheoDoiChuDe] = useState<boolean | null>(null);
  // The bell: a dot when a notification is newer than this phone last looked (QA UI-147).
  const [coMoi, setCoMoi] = useState(false);
  // The «Bảng tin có cập nhật» band sits under the header as laid out, not at a
  // fixed 136: at 320 the subtitle wrapped and the band covered a tab (QA UI-143).
  const [caoDau, setCaoDau] = useState(136);
  const list = useRef<FlatList<Post>>(null);
  const generation = useRef(0);
  const fetching = useRef(false);
  // Which feed is on screen. Coming back to the same one keeps it, its scroll
  // and every card opened to the end; a different one starts over (QA UI-135).
  const dangHien = useRef<string | null>(null);
  // The reader's place in the list, kept while the screen is in front. The web
  // resets a hidden list's scroll when a post page covers it, so coming back
  // puts the reader where they were (QA UI-135).
  const viTriCuon = useRef(0);
  const dangTruoc = useRef(false);
  // The stream's first `sync` is the connection opening, not news: only a
  // later one (a reconnection that may have missed changes) offers a refresh.
  const daNoi = useRef(false);
  const personRef = useRef(person);
  personRef.current = person;
  const topic = typeof params.topic === "string" && params.topic !== "" ? params.topic : undefined;
  useViewSignal(person, active, prefs?.personalized === true);
  const load = useCallback(async (after?: string | null) => {
    if (!person || fetching.current) return;
    fetching.current = true;
    const seq = ++generation.current;
    setLoading(true);
    setError(null);
    try {
      const page = await readFeed(person, mode, after, topic);
      if (seq !== generation.current) return;
      setPosts((old) => (after ? mergePosts(old, page.posts) : page.posts));
      setNext(page.next_cursor);
      if (!after) setFresh(false);
    } catch (e) {
      if (seq === generation.current) setError(e instanceof Error ? e.message : "Chưa đọc được bảng tin.");
    } finally {
      fetching.current = false;
      if (seq === generation.current) setLoading(false);
    }
  }, [person, mode, topic]);
  useFocusEffect(useCallback(() => {
    fetching.current = false;
    const khoa = `${person ?? ""}|${mode}|${topic ?? ""}`;
    dangTruoc.current = true;
    if (dangHien.current !== khoa) {
      dangHien.current = khoa;
      viTriCuon.current = 0;
      setPosts([]);
      setDaAn(new Set());
      void load();
    } else if (viTriCuon.current > 0) {
      const y = viTriCuon.current;
      requestAnimationFrame(() => list.current?.scrollToOffset({ offset: y, animated: false }));
    }
    if (person) {
      void translatedAsActor<Preferences>(COMMUNITY_ERRORS, "/v2/community/preferences", { actorId: person, method: "GET" }).then(setPrefs).catch(() => { });
      void translatedAsActor<{ notifications: ThongBao[] }>(COMMUNITY_ERRORS, "/v2/community/notifications", { actorId: person, method: "GET" })
        .then(async (page) => setCoMoi(coThongBaoMoi(page.notifications, await docGiaoDienAsync(khoaThongBaoDaXem(person)))))
        .catch(() => { });
      if (topic) {
        void translatedAsActor<{ follows: { kind: string; target: string }[] }>(COMMUNITY_ERRORS, "/v2/community/follows", { actorId: person, method: "GET" })
          .then((page) => setTheoDoiChuDe(page.follows.some((f) => f.kind === "topic" && f.target === topic)))
          .catch(() => setTheoDoiChuDe(null));
      }
    }
    return () => { dangTruoc.current = false; generation.current++; setActive(null); };
  }, [load, person, mode, topic]));
  // On the tab, tapping the lit Khám phá column again goes back to the top of the feed.
  useChamLaiTab(dau ? () => { viTriCuon.current = 0; list.current?.scrollToOffset({ offset: 0, animated: !motion.reduced }); } : null);
  const refresh = async () => { await load(); list.current?.scrollToOffset({ offset: 0, animated: !motion.reduced }); };
  // A (re)connection is not news by itself: it offers the refresh band instead
  // of rebuilding the list under the reader (ADR-0040: never push the scroll).
  const connected = useCommunityStream(person, [], (e) => {
    if (e.kind === "sync") {
      const lanDau = !daNoi.current;
      daNoi.current = true;
      if (posts.length === 0 && !loading) void load();
      else if (!lanDau) setFresh(true);
      return;
    }
    setFresh(true);
  });
  const update = (p: Post) => { setPosts((items) => items.map((item) => (item.id === p.id ? p : item))); };
  const act = async (task: () => Promise<unknown>) => {
    try {
      await task();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Chưa thực hiện được thao tác.");
    }
  };
  const like = async (p: Post) => {
    if (!person || busy) return;
    setBusy(p.id);
    update({ ...p, liked: !p.liked, likes: Math.max(0, p.likes + (p.liked ? -1 : 1)) });
    try {
      update(await likePost(person, p.id, !p.liked));
    } catch (e) {
      update(p);
      setError(e instanceof Error ? e.message : "Chưa lưu được lượt thích.");
    } finally {
      setBusy(null);
    }
  };
  const theoDoiTacGia = async (p: Post) => {
    if (!person) return;
    await act(async () => {
      await follow(person, "person", p.author_id, !p.following);
      setPosts((items) => doiTheoDoiTacGia(items, p.author_id, !p.following));
    });
  };
  const anBai = async (p: Post) => {
    if (!person) return;
    setSelected(null);
    await act(async () => {
      await feedback(person, p.id, "hidden", true);
      setDaAn((s) => new Set([...s, p.id]));
    });
  };
  const hoanTacAn = async (p: Post) => {
    if (!person) return;
    await act(async () => {
      await feedback(person, p.id, "hidden", false);
      setDaAn((s) => { const n = new Set(s); n.delete(p.id); return n; });
    });
  };
  const boAn = async (p: Post) => {
    if (!person) return;
    setSelected(null);
    await act(async () => {
      await feedback(person, p.id, "hidden", false);
      setPosts((items) => items.filter((x) => x.id !== p.id));
    });
  };
  const viewable = useRef(({ viewableItems }: {
    viewableItems: ViewToken<Post>[];
  }) => { const first = viewableItems.find((v) => v.isViewable); setActive(first?.item.id ?? null); });
  const consent = async (enabled: boolean) => {
    if (!person) return;
    await act(async () => {
      setPrefs(await translatedAsActor<Preferences>(COMMUNITY_ERRORS, "/v2/community/preferences", {
        actorId: person,
        method: "PUT",
        body: { personalized: enabled },
        attempt: newAttempt(),
      }));
      await load();
    });
  };
  const xoaLichSu = async () => {
    if (!person) return;
    await act(async () => {
      await translatedAsActor(COMMUNITY_ERRORS, "/v2/community/history", { actorId: person, method: "DELETE", attempt: newAttempt() });
      setXacNhanXoa(false);
      setDaXoaLichSu(true);
      await load();
    });
  };
  const doiTheoDoiChuDe = async () => {
    if (!person || !topic || theoDoiChuDe === null) return;
    const bat = !theoDoiChuDe;
    await act(async () => {
      await follow(person, "topic", topic, bat);
      setTheoDoiChuDe(bat);
    });
  };
  const dongCaiDat = () => { setSettings(false); setXacNhanXoa(false); setDaXoaLichSu(false); };
  if (!person)
    return <RudiScreen header={dau?.()}><View style={dau ? styles.moiVao : undefined}><Text style={[typography.display, { color: colors.ink }]}>Những cuộc đi, những câu chuyện.</Text><Text style={[typography.body, { color: colors.inkSoft }]}>Đăng nhập để gặp cộng đồng Rủ Đi và kể về ngày của bạn.</Text><RudiButton label="Đăng nhập" onPress={() => router.push(duongDangNhap("/community") as never)}/></View></RudiScreen>;
  // A mode with a tab names itself in the row; only the hidden posts need a title.
  const tieuDeRieng = mode === "hidden" ? "Những bài bạn đã ẩn" : !dau && mode === "mine" ? "Những điều bạn đã kể" : !dau && mode === "saved" ? "Để dành cho một ngày" : null;
  const nutThongBao = <Pressable accessibilityRole="button" accessibilityLabel={coMoi ? "Thông báo, có điều mới" : "Thông báo"} onPress={() => { setCoMoi(false); router.push("/community/notifications" as never); }} style={styles.icon}>
    <Ionicons name="notifications-outline" size={24} color={colors.ink}/>
    {coMoi ? <View style={[styles.cham, { backgroundColor: colors.accent, borderColor: colors.ground }]} /> : null}
  </Pressable>;
  const nutCaiDat = <Pressable accessibilityRole="button" accessibilityLabel="Cài đặt bảng tin" onPress={() => setSettings(true)} style={styles.icon}><Ionicons name="options-outline" size={24} color={colors.ink}/></Pressable>;
  const oTim = <SearchField accessibilityLabel="Tìm chủ đề" placeholder="Đi đâu, ăn gì, trải nghiệm gì?" value={search} onChangeText={setSearch} onSubmitEditing={() => { if (search.trim()) router.push({ pathname: "/community/search", params: { q: search.trim() } } as never); }}/>;
  return <RudiScreen header={dau?.(<View style={styles.hangNut}>{nutThongBao}{nutCaiDat}</View>)} scroll={false} padded={false} bottomInset={0} testID="community-screen" overlay={<>
    <Sheet open={settings} onClose={dongCaiDat} accessibilityLabel="Bảng tin của bạn">
      <Text style={[typography.h1, { color: colors.ink }]}>Bảng tin của bạn</Text>
      <Text style={[typography.body, { color: colors.inkSoft }]}>Chỉ dùng những gì bạn xem và tương tác trong cộng đồng. Bạn luôn có thể đổi ý.</Text>
      <RudiButton label={prefs?.personalized ? "Tắt cá nhân hóa" : "Bật cá nhân hóa"} onPress={() => void consent(!prefs?.personalized)}/>
      {xacNhanXoa ? (
        // One step to undo nothing: what goes, and what stays, before it goes.
        <View style={styles.xacNhan} testID="cong-dong-xac-nhan-xoa">
          <Text style={[typography.body, { color: colors.ink }]}>Xoá những gì bảng tin đã học từ lượt xem và tương tác của bạn? Bài đã lưu, bài đã ẩn và người bạn theo dõi vẫn giữ nguyên.</Text>
          <RudiButton label="Xoá lịch sử" tone="warn" variant="outline" onPress={() => void xoaLichSu()}/>
          <RudiButton label="Giữ lại" variant="ghost" onPress={() => setXacNhanXoa(false)}/>
        </View>
      ) : (
        <RudiButton label="Xóa lịch sử đề xuất" variant="outline" onPress={() => { setDaXoaLichSu(false); setXacNhanXoa(true); }}/>
      )}
      {daXoaLichSu ? <Text accessibilityLiveRegion="polite" style={[typography.note, { color: colors.inkSoft }]}>Đã xoá lịch sử đề xuất. Bảng tin bắt đầu lại từ những gì mới.</Text> : null}
      {/* «Đã lưu» and «Bài của tôi» are tabs now, and the bell sits on the header. */}
      <RudiButton label="Bài đã ẩn" variant="ghost" onPress={() => { setMode("hidden"); dongCaiDat(); }}/>
      <RudiButton label="Điều mình muốn giữ" variant="ghost" onPress={() => { dongCaiDat(); router.push("/community/keeps" as never); }} />
    </Sheet>
    <Sheet open={selected !== null} onClose={() => setSelected(null)} accessibilityLabel="Lựa chọn cho bài đăng">{selected ? <>
      <Text style={[typography.h2, { color: colors.ink }]}>Câu chuyện này</Text>
      <Text style={[typography.body, { color: colors.inkSoft }]}>{selected.why}</Text>
      <RudiButton label={selected.saved ? "Bỏ lưu" : "Lưu để đọc lại"} onPress={() => void act(async () => { await feedback(person, selected.id, "saved", !selected.saved); update({ ...selected, saved: !selected.saved }); setSelected(null); })}/>
      {mode === "hidden"
        ? <RudiButton label="Bỏ ẩn" variant="outline" onPress={() => void boAn(selected)}/>
        : <RudiButton label="Không quan tâm" variant="outline" onPress={() => void anBai(selected)}/>}
      {selected.author_id === person ? <RudiButton label="Mở bài và quản lý" variant="ghost" onPress={() => { router.push(`/community/posts/${selected.id}` as never); setSelected(null); }}/> : <RudiButton label="Báo cáo bài viết" variant="ghost" onPress={() => { router.push({ pathname: "/community/posts/[id]", params: { id: selected.id, report: "1" } } as never); setSelected(null); }}/>}
    </> : null}</Sheet>
    <Sheet open={comments !== null} onClose={() => setComments(null)} accessibilityLabel="Bình luận">{comments ? <Comments person={person} post={comments}/> : null}</Sheet>
  </>}>
    <View style={styles.readingColumn}>
    {dau ? null : <View onLayout={(e) => setCaoDau(Math.round(e.nativeEvent.layout.height))} style={[styles.header, topic ? styles.headerChuDe : null, { borderBottomColor: colors.line }]}>
      {topic ? (
        // A topic is a page pushed onto the feed: it has the way back every
        // other community page has, and follows its topic where it stands
        // (QA UI-146). No feed tabs: this list is the topic.
        <>
          <TopBar title={topic} />
          <View style={styles.chuDeHang}>
            <Text style={[typography.note, styles.flex, { color: colors.inkSoft }]}>Những câu chuyện gắn chủ đề này.</Text>
            {theoDoiChuDe !== null ? (
              <RudiButton compact full={false} icon={theoDoiChuDe ? "checkmark" : "add"} label={theoDoiChuDe ? "Đang theo dõi" : "Theo dõi chủ đề"} onPress={() => void doiTheoDoiChuDe()} variant={theoDoiChuDe ? "ghost" : "outline"} />
            ) : null}
          </View>
        </>
      ) : (
        <>
          {/* The title stays first in reading order; on a narrow window only
              the actions' place on screen changes. */}
          <View style={[styles.headingRow, hep && styles.headingRowHep]}>
            <Text style={[typography.display, styles.flex, { color: colors.ink }]}>Cộng đồng</Text>
            <View style={[styles.hangNut, hep && styles.hangNutHep]}>
              {nutThongBao}
              {nutCaiDat}
              <Pressable accessibilityRole="button" accessibilityLabel="Đăng khoảnh khắc" onPress={() => router.push("/community/new" as never)} style={[styles.icon, { backgroundColor: colors.accent, borderRadius: 16 }]}><Ionicons name="create-outline" size={24} color={colors.accentInk}/></Pressable>
            </View>
          </View>
          {/* Under the row, the page's width: beside three 48dp actions the
              line broke at 390dp and left «nối» alone on a second line. */}
          <Text style={[typography.caption, styles.dongPhu, { color: colors.inkFaint }]}>{connected ? "Những câu chuyện đang tiếp nối" : "Kết nối những cuộc đi"}</Text>
          <View {...TABLIST} style={styles.tabs}>{TABS.map((t) => <Pressable key={t.mode} {...tabState(mode === t.mode)} onPress={() => { setMode(t.mode); motion.haptic.select(); }} style={[styles.tab, { borderBottomColor: mode === t.mode ? colors.accent : "transparent" }]}><Text style={[typography.label, { color: mode === t.mode ? colors.accent : colors.inkFaint }]}>{t.label}</Text></Pressable>)}</View>
        </>
      )}
    </View>}
    {fresh ? <Animated.View entering={FadeIn.duration(motion.ms("standard")).reduceMotion(motion.reanimated)} exiting={FadeOut.duration(motion.ms("standard")).reduceMotion(motion.reanimated)} style={[styles.newPosts, { top: dau ? 8 : caoDau + 8 }]}><RudiButton compact label="Bảng tin có cập nhật" icon="arrow-up" onPress={() => void refresh()}/></Animated.View> : null}
    <FlatList
      ref={list}
      data={posts}
      keyExtractor={(p) => p.id}
      renderItem={({ item }) => daAn.has(item.id) ? (
        // The card's own place, kept: one line says what happened and how to take it back.
        <View style={[styles.daAn, { borderBottomColor: colors.line }]} testID={`community-hidden-${item.id}`}>
          <Text accessibilityLiveRegion="polite" style={[typography.note, styles.flex, { color: colors.inkSoft }]}>Đã ẩn bài của {item.author}. Bài này không hiện trong bảng tin của bạn nữa.</Text>
          <RudiButton compact full={false} label="Hoàn tác" onPress={() => void hoanTacAn(item)} variant="ghost"/>
        </View>
      ) : (
        <PostCard post={item} person={person} active={active === item.id} busy={busy === item.id} onLike={() => void like(item)} onComment={() => setComments(item)} onMore={() => setSelected(item)} onFollow={() => void theoDoiTacGia(item)} onSave={() => void act(async () => { await feedback(person, item.id, "saved", !item.saved); if (mode === "saved" && item.saved) setPosts((items) => items.filter((p) => p.id !== item.id)); else update({ ...item, saved: !item.saved }); })} onTopic={(t) => router.push({ pathname: "/community/topic", params: { topic: t } } as never)}/>
      )}
      viewabilityConfig={{ itemVisiblePercentThreshold: 60, minimumViewTime: 1000 }}
      onViewableItemsChanged={viewable.current}
      initialNumToRender={5}
      windowSize={7}
      maxToRenderPerBatch={5}
      removeClippedSubviews={false}
      contentContainerStyle={styles.list}
      refreshControl={<RefreshControl refreshing={loading && posts.length > 0} onRefresh={() => void refresh()} tintColor={colors.accent}/>}
      onEndReached={() => { if (next && !loading) void load(next); }}
      onEndReachedThreshold={0.4}
      onScroll={(e) => { if (dangTruoc.current) viTriCuon.current = e.nativeEvent.contentOffset.y; }}
      scrollEventThrottle={64}
      ListHeaderComponent={<View style={styles.intro}>
        {/* Khám phá › Cộng đồng, in the mockup's order: the search field as on
            Địa điểm, then the feed tabs as chips that scroll rather than clip. */}
        {dau ? oTim : null}
        {dau ? <HangChuTab muc={CHE_DO} chon={CHE_DO.some((c) => c.id === mode) ? mode : null} onChon={(m) => { setMode(m); motion.haptic.select(); }} /> : null}
        {tieuDeRieng ? <Text style={[typography.h2, { color: colors.ink }]}>{tieuDeRieng}</Text> : null}
        {dau || topic ? null : oTim}
        {/* Asked where it changes something: only «Dành cho bạn» learns from what one reads. */}
        {prefs && !prefs.asked && !topic && mode === "for_you" ? <View style={[styles.consent, { backgroundColor: colors.paper }]}><Text style={[typography.h2, { color: colors.ink }]}>Một góc hợp với bạn</Text><Text style={[typography.body, { color: colors.inkSoft }]}>Cho phép học từ tương tác cộng đồng? Không đọc chat hay sổ riêng.</Text><View style={styles.row}><RudiButton full={false} compact label="Cá nhân hóa" onPress={() => void consent(true)}/><Pressable accessibilityRole="button" onPress={() => void consent(false)} style={styles.later}><Text style={[typography.label, { color: colors.ink }]}>Để sau</Text></Pressable></View></View> : null}
        {/* Said where the list is, with the way on; a failed reload keeps the cards already shown. */}
        <CauTaiCho cau={error} hanhDong={{ label: "Thử lại", onPress: () => void load() }} testID="cong-dong-loi-bang-tin" />
      </View>}
      ListEmptyComponent={loading ? <View style={styles.intro}><SkeletonLines lines={4}/></View> : !error ? (
        mode === "hidden" ? <View style={styles.empty}><Ionicons name="eye-off-outline" size={42} color={colors.accent}/><Text style={[typography.h1, { color: colors.ink }]}>Chưa ẩn bài nào</Text><Text style={[typography.body, { color: colors.inkSoft }]}>Bài bạn chọn «Không quan tâm» nằm ở đây. Chạm «…» trên một bài để bỏ ẩn.</Text></View>
        // The reader's own two lists say what goes in them, and how.
        : mode === "saved" ? <View style={styles.empty}><Ionicons name="bookmark-outline" size={42} color={colors.accent}/><Text style={[typography.h1, { color: colors.ink }]}>Chưa lưu bài nào</Text><Text style={[typography.body, { color: colors.inkSoft }]}>Chạm dấu lưu ở cuối một bài để để dành đọc lại.</Text></View>
        : mode === "mine" ? <View style={styles.empty}><Ionicons name="create-outline" size={42} color={colors.accent}/><Text style={[typography.h1, { color: colors.ink }]}>Bạn chưa kể chuyện nào</Text><Text style={[typography.body, { color: colors.inkSoft }]}>Bài bạn viết hiện ở đây, kèm trạng thái duyệt của từng bài.</Text><RudiButton label="Viết bài" onPress={() => router.push("/community/new" as never)}/></View>
        : <View style={styles.empty}><Ionicons name="trail-sign-outline" size={42} color={colors.accent}/><Text style={[typography.h1, { color: colors.ink }]}>{mode === "following" ? "Câu chuyện bắt đầu từ một người" : "Một ngày đáng kể"}</Text><Text style={[typography.body, { color: colors.inkSoft }]}>{mode === "following" ? "Theo dõi tác giả hoặc chủ đề bạn thích. Những cuộc đi của họ sẽ gặp bạn ở đây." : "Một quán nhỏ, một cung đường, một buổi đi chơi. Kể điều bạn muốn giữ lại."}</Text><RudiButton label="Kể khoảnh khắc đầu tiên" onPress={() => router.push("/community/new" as never)}/></View>
      ) : null}
      ListFooterComponent={loading && posts.length ? <Text style={[typography.caption, styles.intro, { color: colors.inkFaint }]}>Đang mở thêm câu chuyện…</Text> : null}
    />
    </View>
  </RudiScreen>;
}
const styles = StyleSheet.create({
  moiVao: { paddingTop: 16, gap: 18 },
  readingColumn: { flex: 1, width: "100%", maxWidth: 560, alignSelf: "center" },
  header: { paddingHorizontal: 20, paddingTop: 12, borderBottomWidth: StyleSheet.hairlineWidth },
  headerChuDe: { paddingHorizontal: 16, paddingTop: 0, paddingBottom: 8 },
  headingRow: { flexDirection: "row", alignItems: "center", gap: 4 },
  headingRowHep: { paddingTop: 52 },
  hangNut: { flexDirection: "row", alignItems: "center", gap: 4 },
  hangNutHep: { position: "absolute", top: 0, right: 0 },
  dongPhu: { marginTop: -4 },
  flex: { flex: 1 },
  icon: { width: 48, height: 48, alignItems: "center", justifyContent: "center" },
  // The bell's dot: a coral stamp on the bell's shoulder, ringed in the ground colour.
  cham: { position: "absolute", top: 11, right: 11, width: 10, height: 10, borderRadius: 5, borderWidth: 2 },
  tabs: { flexDirection: "row", marginTop: 14, gap: 18 },
  tab: { minHeight: 48, justifyContent: "center", borderBottomWidth: 2 },
  chuDeHang: { flexDirection: "row", alignItems: "center", gap: 12, paddingHorizontal: 4 },
  list: { paddingBottom: 32 },
  // 16 like the posts and Khám phá's header above, so one gutter runs down the screen.
  intro: { padding: 16, gap: 16 },
  consent: { padding: 20, gap: 12, borderRadius: 16 },
  row: { flexDirection: "row", gap: 12 },
  later: { minHeight: 48, paddingHorizontal: 16, justifyContent: "center" },
  xacNhan: { gap: 8 },
  empty: { margin: 24, paddingVertical: 36, gap: 20, alignItems: "flex-start" },
  daAn: { flexDirection: "row", alignItems: "center", gap: 8, paddingHorizontal: 16, paddingVertical: 12, borderBottomWidth: StyleSheet.hairlineWidth },
  newPosts: { position: "absolute", alignSelf: "center", zIndex: 3 },
});
