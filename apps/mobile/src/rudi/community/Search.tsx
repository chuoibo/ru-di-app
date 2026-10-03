import { useLocalSearchParams, useRouter } from "expo-router";
import { useEffect, useState } from "react";
import { Pressable, Text, View } from "react-native";
import { translatedAsActor } from "../../api";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { Field, RudiButton, RudiScreen, TopBar } from "../ui";
import { COMMUNITY_ERRORS, follow } from "./api";
import { CauTaiCho } from "../ui/CauTaiCho";
import { EmptyState } from "../ui/EmptyState";
import { Canh } from "../ui/art/Canh";
import { useKeyboardOpen } from "../ui/useKeyboardOpen";

export function Search() {
  const { q } = useLocalSearchParams<{ q?: string }>(); const [query, setQuery] = useState(q ?? ""); const { phien } = useRudiSession(); const router = useRouter(); const { colors } = useRudiTheme(); const [topics, setTopics] = useState<{ name: string; posts: number }[]>([]); const [people, setPeople] = useState<{ id: string; name: string; following: boolean }[]>([]); const [followingTopics, setFollowingTopics] = useState<string[]>([]); const [error, setError] = useState<string | null>(null);
  // The query the lists on screen answer: an empty answer is said only for it,
  // not while the next query is still in flight (QA UI-145).
  const [daTim, setDaTim] = useState<string | null>(null);
  // It searches as one types, so «found nothing» shows with the keyboard up:
  // the scene steps aside then, and the advice stays right under the field.
  const banPhim = useKeyboardOpen();
  useEffect(() => { if (!phien) return; let current = true; const timer = setTimeout(() => { void (async () => { try { const page = await translatedAsActor<{ topics: typeof topics }>(COMMUNITY_ERRORS, `/v2/community/topics?q=${encodeURIComponent(query)}`, { actorId: phien.person_id, method: "GET" }); const follows = await translatedAsActor<{ follows: { kind: string; target: string }[] }>(COMMUNITY_ERRORS, "/v2/community/follows", { actorId: phien.person_id, method: "GET" }); const found = query.trim().length >= 2 ? await translatedAsActor<{ people: typeof people }>(COMMUNITY_ERRORS, `/v2/community/search?q=${encodeURIComponent(query)}`, { actorId: phien.person_id, method: "GET" }) : { people: [] }; if (current) { setTopics(page.topics); setPeople(found.people); setDaTim(query); setFollowingTopics(follows.follows.filter((f) => f.kind === "topic").map((f) => f.target)); setError(null); } } catch (e) { if (current) setError(e instanceof Error ? e.message : "Chưa tìm được chủ đề."); } })(); }, 300); return () => { current = false; clearTimeout(timer); }; }, [query, phien]);
  return <RudiScreen><TopBar title="Tìm một điều thú vị" /><Field label="Chủ đề hoặc người chia sẻ" value={query} onChangeText={setQuery} placeholder="Cà phê, trekking, tên một người…" /><CauTaiCho cau={error} />{daTim !== null && daTim.trim() !== "" && daTim === query && !error && topics.length === 0 && people.length === 0 ? (
    <EmptyState body="Thử một từ ngắn hơn hoặc không dấu: cà phê, Đà Lạt, trekking, hay tên một người bạn theo dõi." illustration={banPhim ? undefined : <Canh id="tim-khong-ra" width={168} />} kind="no-results" layout="inline" title={`Không tìm thấy chủ đề hay người nào khớp «${daTim.trim()}»`} />
  ) : null}{topics.length > 0 ? <Text style={[typography.h2, { color: colors.ink }]}>Chủ đề</Text> : null}{topics.map((topic) => <View key={topic.name} style={{ flexDirection: "row", alignItems: "center", borderBottomWidth: 1, borderColor: colors.line, paddingVertical: 16 }}><Pressable accessibilityRole="button" style={{ flex: 1, minHeight: 48 }} onPress={() => router.push({ pathname: "/community/topic", params: { topic: topic.name } } as never)}><Text style={[typography.title, { color: colors.ink }]}>{topic.name}</Text><Text style={[typography.caption, { color: colors.inkFaint }]}>{topic.posts} câu chuyện</Text></Pressable><RudiButton full={false} compact variant="ghost" label={followingTopics.includes(topic.name) ? "Đang theo dõi" : "Theo dõi"} onPress={() => { if (!phien) return; const enabled = !followingTopics.includes(topic.name); void follow(phien.person_id, "topic", topic.name, enabled).then(() => setFollowingTopics((items) => enabled ? [...items, topic.name] : items.filter((v) => v !== topic.name))).catch((e) => setError(e.message)); }} /></View>)}{people.length > 0 ? <Text style={[typography.h2, { color: colors.ink }]}>Người chia sẻ</Text> : null}{people.map((p) => <View key={p.id} style={{ flexDirection: "row", alignItems: "center", paddingVertical: 12 }}><Pressable accessibilityRole="button" style={{ flex: 1, minHeight: 48, justifyContent: "center" }} onPress={() => router.push(`/people/${p.id}`)}><Text style={[typography.body, { color: colors.ink }]}>{p.name}</Text></Pressable><RudiButton full={false} compact label={p.following ? "Đang theo dõi" : "Theo dõi"} variant="ghost" onPress={() => { if (!phien) return; void follow(phien.person_id, "person", p.id, !p.following).then(() => setPeople((items) => items.map((x) => x.id === p.id ? { ...x, following: !x.following } : x))).catch((e) => setError(e.message)); }} /></View>)}</RudiScreen>;
}
