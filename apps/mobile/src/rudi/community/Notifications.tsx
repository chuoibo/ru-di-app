import { translatedAsActor } from "../../api";
import { COMMUNITY_ERRORS } from "./api";
import { useRouter } from "expo-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { Pressable, Text, View } from "react-native";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { RudiScreen, TopBar } from "../ui";
import { relativeTime } from "./api";
import { useCommunityStream } from "./useCommunityStream";
import { cauThongBao, khoaThongBaoDaXem, type ThongBao } from "./bang-tin";
import { ghiGiaoDienAsync } from "../kho";
import { CauTaiCho } from "../ui/CauTaiCho";
import { EmptyState } from "../ui/EmptyState";
import { SkeletonGroup, SkeletonRow } from "../ui/Skeleton";
import { AssetIllustration } from "../ui/AssetIllustration";
type Notification = ThongBao;
export function Notifications() {
    const { phien } = useRudiSession();
    const { colors } = useRudiTheme();
    const router = useRouter();
    const [items, setItems] = useState<Notification[]>([]);
    const [error, setError] = useState<string | null>(null);
    const [loaded, setLoaded] = useState(false);
    const request = useRef(0);
    const load = useCallback(async () => { if (!phien)
        return; const serial = ++request.current; try {
        const page = await translatedAsActor<{
            notifications: Notification[];
        }>(COMMUNITY_ERRORS, "/v2/community/notifications", {
            actorId: phien.person_id,
            method: "GET"
        });
        if (serial !== request.current) return;
        setItems(page.notifications);
        setLoaded(true);
        setError(null);
        // This phone has now seen the list: the bell on the feed loses its dot.
        void ghiGiaoDienAsync(khoaThongBaoDaXem(phien.person_id), new Date().toISOString());
    }
    catch (e) {
        if (serial !== request.current) return;
        setError(e instanceof Error ? e.message : "Chưa đọc được thông báo.");
    } }, [phien]);
    useEffect(() => { setLoaded(false); setItems([]); void load(); return () => { request.current++; }; }, [load]);
    // Re-read in place: the list does not blink on every (re)connection.
    useCommunityStream(phien?.person_id, [], () => { void load(); });
    return <RudiScreen material="quiet" onRefresh={load} testID="community-notifications"><TopBar title="Thông báo"/>
      <CauTaiCho cau={error} hanhDong={{ label: "Thử lại", onPress: () => void load() }} />
      {!loaded && !error ? <SkeletonGroup><SkeletonRow /><SkeletonRow /><SkeletonRow /></SkeletonGroup> : null}
      <View style={{ width: "100%", maxWidth: 620, alignSelf: "center" }}>{items.map((item) => <Pressable key={item.id} accessibilityRole="button" accessibilityLabel={cauThongBao(item)} onPress={() => router.push(`/community/posts/${item.post_id}` as never)} style={{ minHeight: 48, paddingVertical: 20, borderBottomWidth: 1, borderColor: colors.line, gap: 8 }}><Text style={[typography.title, { color: colors.ink }]}>{cauThongBao(item)}</Text>{item.excerpt ? <Text numberOfLines={2} style={[typography.note, { color: colors.inkSoft }]}>«{item.excerpt}»</Text> : null}<Text style={[typography.caption, { color: colors.inkFaint }]}>{relativeTime(item.created_at)}</Text></Pressable>)}</View>
      {loaded && !items.length && !error ? <EmptyState kind="first-use" title="Chưa có thông báo" body="Khi có người nhắc đến bạn hoặc trả lời câu chuyện, lời nhắc sẽ gặp bạn ở đây." illustration={<AssetIllustration name="spot-keeps" width={240} height={200} />} action={{ label: "Về cộng đồng", onPress: () => router.replace("/(tabs)/community" as never) }} /> : null}
    </RudiScreen>;
}
