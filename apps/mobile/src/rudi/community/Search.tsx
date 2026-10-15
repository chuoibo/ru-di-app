import { useLocalSearchParams, useRouter } from "expo-router";
import { useEffect, useRef, useState } from "react";
import { Pressable, Text, View, type TextInput } from "react-native";
import { translatedAsActor } from "../../api";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { Field, IconButton, RudiButton, RudiScreen, TopBar } from "../ui";
import { COMMUNITY_ERRORS, follow } from "./api";
import { CauTaiCho } from "../ui/CauTaiCho";
import { EmptyState } from "../ui/EmptyState";
import { Canh } from "../ui/art/Canh";
import { useKeyboardOpen } from "../ui/useKeyboardOpen";
import { SkeletonGroup, SkeletonRow } from "../ui/Skeleton";

export function Search() {
  const { q } = useLocalSearchParams<{ q?: string }>(); const [query, setQuery] = useState(q ?? ""); const { phien } = useRudiSession(); const router = useRouter(); const { colors } = useRudiTheme(); const [topics, setTopics] = useState<{ name: string; posts: number }[]>([]); const [people, setPeople] = useState<{ id: string; name: string; following: boolean }[]>([]); const [followingTopics, setFollowingTopics] = useState<string[]>([]); const [error, setError] = useState<string | null>(null);
  // The query the lists on screen answer: an empty answer is said only for it,
  // not while the next query is still in flight (QA UI-145).
  const [daTim, setDaTim] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  const [busyFollow, setBusyFollow] = useState<string | null>(null);
  const queryInput = useRef<TextInput>(null);
  // It searches as one types, so «found nothing» shows with the keyboard up:
  // the scene steps aside then, and the advice stays right under the field.
  const banPhim = useKeyboardOpen();
  useEffect(() => { if (!phien) return; let current = true; setError(null); setDaTim(null); const timer = setTimeout(() => { void (async () => { try { const page = await translatedAsActor<{ topics: typeof topics }>(COMMUNITY_ERRORS, `/v2/community/topics?q=${encodeURIComponent(query)}`, { actorId: phien.person_id, method: "GET" }); const follows = await translatedAsActor<{ follows: { kind: string; target: string }[] }>(COMMUNITY_ERRORS, "/v2/community/follows", { actorId: phien.person_id, method: "GET" }); const found = query.trim().length >= 2 ? await translatedAsActor<{ people: typeof people }>(COMMUNITY_ERRORS, `/v2/community/search?q=${encodeURIComponent(query)}`, { actorId: phien.person_id, method: "GET" }) : { people: [] }; if (current) { setTopics(page.topics); setPeople(found.people); setDaTim(query); setFollowingTopics(follows.follows.filter((f) => f.kind === "topic").map((f) => f.target)); setError(null); } } catch (e) { if (current) setError(e instanceof Error ? e.message : "Chưa tìm được chủ đề."); } })(); }, 300); return () => { current = false; clearTimeout(timer); }; }, [query, phien, retry]);
  const changeFollow = async (kind: "person" | "topic", target: string, enabled: boolean) => {
    if (!phien || busyFollow) return;
    setBusyFollow(`${kind}:${target}`); setError(null);
    try { await follow(phien.person_id, kind, target, enabled); if (kind === "topic") setFollowingTopics((items) => enabled ? [...items, target] : items.filter((v) => v !== target)); else setPeople((items) => items.map((item) => item.id === target ? { ...item, following: enabled } : item)); }
    catch (e) { setError(e instanceof Error ? e.message : "Chưa đổi được lựa chọn theo dõi."); }
    finally { setBusyFollow(null); }
  };
  return <RudiScreen material="quiet" cot="form" avoidKeyboard testID="community-search"><TopBar title="Tìm một điều thú vị" /><Field label="Chủ đề hoặc người chia sẻ" oRef={queryInput} value={query} onChangeText={setQuery} placeholder="Cà phê, trekking, tên một người…" trailing={query ? <IconButton icon="close" quiet accessibilityLabel="Làm trống ô tìm kiếm" onPress={() => { setQuery(""); queryInput.current?.focus(); }} /> : undefined} /><CauTaiCho cau={error} hanhDong={{ label: "Thử lại", onPress: () => setRetry((value) => value + 1) }} />{daTim === null && !error ? <SkeletonGroup><SkeletonRow /><SkeletonRow /></SkeletonGroup> : null}{daTim !== null && daTim.trim() !== "" && daTim === query && !error && topics.length === 0 && people.length === 0 ? (
    <EmptyState body="Thử một từ ngắn hơn hoặc không dấu: cà phê, Đà Lạt, trekking, hay tên một người bạn theo dõi." illustration={banPhim ? undefined : <Canh id="tim-khong-ra" width={168} />} kind="no-results" layout="inline" title={`Không tìm thấy chủ đề hay người nào khớp «${daTim.trim()}»`} />
  ) : null}{daTim === query ? <>{topics.length > 0 ? <Text style={[typography.h2, { color: colors.ink }]}>Chủ đề</Text> : null}{topics.map((topic) => <View key={topic.name} style={{ flexDirection: "row", flexWrap: "wrap", alignItems: "center", gap: 12, borderBottomWidth: 1, borderColor: colors.line, paddingVertical: 16 }}><Pressable accessibilityRole="button" style={{ flex: 1, minWidth: 140, minHeight: 48 }} onPress={() => router.push({ pathname: "/community/topic", params: { topic: topic.name } } as never)}><Text style={[typography.title, { color: colors.ink }]}>{topic.name}</Text><Text style={[typography.caption, { color: colors.inkFaint }]}>{topic.posts} câu chuyện</Text></Pressable><RudiButton full={false} compact variant="ghost" disabled={busyFollow !== null} lyDo={busyFollow !== null ? "Chờ lựa chọn đang lưu xong." : undefined} loading={busyFollow === `topic:${topic.name}`} label={followingTopics.includes(topic.name) ? "Đang theo dõi" : "Theo dõi"} onPress={() => void changeFollow("topic", topic.name, !followingTopics.includes(topic.name))} /></View>)}{people.length > 0 ? <Text style={[typography.h2, { color: colors.ink }]}>Người chia sẻ</Text> : null}{people.map((p) => <View key={p.id} style={{ flexDirection: "row", flexWrap: "wrap", alignItems: "center", gap: 12, paddingVertical: 12 }}><Pressable accessibilityRole="button" style={{ flex: 1, minWidth: 140, minHeight: 48, justifyContent: "center" }} onPress={() => router.push(`/people/${p.id}`)}><Text style={[typography.body, { color: colors.ink }]}>{p.name}</Text></Pressable><RudiButton full={false} compact disabled={busyFollow !== null} lyDo={busyFollow !== null ? "Chờ lựa chọn đang lưu xong." : undefined} loading={busyFollow === `person:${p.id}`} label={p.following ? "Đang theo dõi" : "Theo dõi"} variant="ghost" onPress={() => void changeFollow("person", p.id, !p.following)} /></View>)}</> : null}</RudiScreen>;
}
