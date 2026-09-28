import { useCallback, useEffect, useState } from "react";
import { Text, View } from "react-native";
import { newAttempt, translatedAsActor } from "../../api";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { RudiButton, RudiScreen, TopBar } from "../ui";
import { COMMUNITY_ERRORS, relativeTime } from "./api";

export function Keeps() {
  const { phien } = useRudiSession(); const { colors } = useRudiTheme(); const [items, setItems] = useState<{ id: string; body: string; ai_generated: boolean; created_at: string }[]>([]); const [error, setError] = useState<string | null>(null);
  const load = useCallback(async () => { if (!phien) return; try { const page = await translatedAsActor<{ keeps: typeof items }>(COMMUNITY_ERRORS, "/v2/community/keeps", { actorId: phien.person_id, method: "GET" }); setItems(page.keeps); } catch (e) { setError(e instanceof Error ? e.message : "Chưa mở được ghi chép."); } }, [phien]); useEffect(() => { void load(); }, [load]);
  return <RudiScreen onRefresh={load}><TopBar title="Điều mình muốn giữ" /><Text style={[typography.body, { color: colors.inkSoft }]}>Những ghi chép riêng của bạn. Chỉ mình bạn đọc được.</Text>{error ? <Text style={[typography.body, { color: colors.accent }]}>{error}</Text> : null}{items.map((item) => <View key={item.id} style={{ gap: 12, paddingVertical: 24, borderBottomWidth: 1, borderColor: colors.line }}><Text style={[typography.caption, { color: colors.inkFaint }]}>{relativeTime(item.created_at)}{item.ai_generated ? " · Có Nếp giúp viết" : ""}</Text><Text style={[typography.body, { color: colors.ink }]}>{item.body}</Text><RudiButton label="Xóa ghi chép" variant="ghost" full={false} onPress={() => { if (!phien) return; void translatedAsActor<void>(COMMUNITY_ERRORS, `/v2/community/keeps/${item.id}`, { actorId: phien.person_id, method: "DELETE", attempt: newAttempt() }).then(load).catch((e) => setError(e.message)); }} /></View>)}</RudiScreen>;
}
