import { translatedAsActor, newAttempt } from "../../api";
import { COMMUNITY_ERRORS } from "./api";
import { useRouter } from "expo-router";
import { useCallback, useEffect, useState } from "react";
import { Pressable, Text } from "react-native";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { RudiScreen, TopBar } from "../ui";
import { relativeTime } from "./api";
import { useCommunityStream } from "./useCommunityStream";
type Notification = {
    id: string;
    post_id: string;
    kind: string;
    created_at: string;
};
export function Notifications() {
    const { phien } = useRudiSession();
    const { colors } = useRudiTheme();
    const router = useRouter();
    const [items, setItems] = useState<Notification[]>([]);
    const [error, setError] = useState<string | null>(null);
    const load = useCallback(async () => { if (!phien)
        return; try {
        const page = await translatedAsActor<{
            notifications: Notification[];
        }>(COMMUNITY_ERRORS, "/v2/community/notifications", {
            actorId: phien.person_id,
            method: "GET"
        });
        setItems(page.notifications);
    }
    catch (e) {
        setError(e instanceof Error ? e.message : "Chưa đọc được thông báo.");
    } }, [phien]);
    useEffect(() => { void load(); }, [load]);
    useCommunityStream(phien?.person_id, [], () => { setItems([]); void load(); });
    return <RudiScreen onRefresh={load}><TopBar title="Có người nhớ đến bạn"/>{error ? <Text style={[typography.body, { color: colors.accent }]}>{error}</Text> : null}{items.map((item) => <Pressable key={item.id} accessibilityRole="button" onPress={() => router.push(`/community/posts/${item.post_id}` as never)} style={{ paddingVertical: 20, borderBottomWidth: 1, borderColor: colors.line, gap: 8 }}><Text style={[typography.title, { color: colors.ink }]}>Bạn được nhắc trong một câu chuyện</Text><Text style={[typography.caption, { color: colors.inkFaint }]}>{relativeTime(item.created_at)}</Text></Pressable>)}{!items.length && !error ? <Text style={[typography.body, { color: colors.inkSoft }]}>Những lời nhắc sẽ gặp bạn ở đây.</Text> : null}</RudiScreen>;
}
