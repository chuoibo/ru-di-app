import { useCallback, useEffect, useState } from "react";
import { HoiTaiHang } from "../ui/HoiTaiHang";
import { Text, View } from "react-native";
import { newAttempt, translatedAsActor } from "../../api";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { RudiButton, RudiScreen, TopBar } from "../ui";
import { COMMUNITY_ERRORS, relativeTime } from "./api";
import { CauTaiCho } from "../ui/CauTaiCho";
import { EmptyState } from "../ui/EmptyState";

export function Keeps() {
  const { phien } = useRudiSession(); const { colors } = useRudiTheme(); const [items, setItems] = useState<{ id: string; body: string; ai_generated: boolean; created_at: string }[]>([]); const [error, setError] = useState<string | null>(null);
  // Read at least once: an empty list before the first answer is not «nothing kept».
  const [daDoc, setDaDoc] = useState(false);
  // Deleting a keep cannot be undone: it asks in its row first, waits while it
  // goes, and a refusal is said under that row, not at the top of the page.
  const [hoiXoa, setHoiXoa] = useState<string | null>(null);
  const [dangXoa, setDangXoa] = useState<string | null>(null);
  const [loiXoa, setLoiXoa] = useState<{ id: string; cau: string } | null>(null);
  const xoa = async (id: string) => {
    if (!phien || dangXoa !== null) return;
    setDangXoa(id); setLoiXoa(null);
    try { await translatedAsActor<void>(COMMUNITY_ERRORS, `/v2/community/keeps/${id}`, { actorId: phien.person_id, method: "DELETE", attempt: newAttempt() }); setHoiXoa(null); await load(); }
    catch (e) { setLoiXoa({ id, cau: e instanceof Error ? e.message : "Chưa xoá được ghi chép này." }); }
    finally { setDangXoa(null); }
  };
  const load = useCallback(async () => { if (!phien) return; try { const page = await translatedAsActor<{ keeps: typeof items }>(COMMUNITY_ERRORS, "/v2/community/keeps", { actorId: phien.person_id, method: "GET" }); setItems(page.keeps); setDaDoc(true); setError(null); } catch (e) { setError(e instanceof Error ? e.message : "Chưa mở được ghi chép."); } }, [phien]); useEffect(() => { void load(); }, [load]);
  return <RudiScreen onRefresh={load}><TopBar title="Điều mình muốn giữ" /><Text style={[typography.body, { color: colors.inkSoft }]}>Những ghi chép riêng của bạn. Chỉ mình bạn đọc được.</Text><CauTaiCho cau={error} hanhDong={{ label: "Thử lại", onPress: () => void load() }} />{daDoc && !error && items.length === 0 ? (
    // Where a keep comes from, so an empty page is a way in rather than a blank (QA UI-145).
    <EmptyState body="Khi Nếp giúp bạn giữ một điều từ một câu chuyện, ghi chép nằm ở đây. Mở một bài trong Cộng đồng rồi chạm «@Nếp · Giúp giữ khoảnh khắc»." kind="first-use" layout="inline" title="Chưa có ghi chép nào" />
  ) : null}{items.map((item) => <View key={item.id} style={{ gap: 12, paddingVertical: 24, borderBottomWidth: 1, borderColor: colors.line }}><Text style={[typography.caption, { color: colors.inkFaint }]}>{relativeTime(item.created_at)}{item.ai_generated ? " · Có Nếp giúp viết" : ""}</Text><Text style={[typography.body, { color: colors.ink }]}>{item.body}</Text>{hoiXoa === item.id ? <HoiTaiHang cau="Xoá ghi chép này? Không lấy lại được." dangLam={dangXoa === item.id} nhan="Xoá" onDongY={() => void xoa(item.id)} onThoi={() => { setHoiXoa(null); setLoiXoa(null); }} /> : <RudiButton accessibilityLabel={`Xoá ghi chép: «${item.body.slice(0, 40)}»`} label="Xóa ghi chép" tone="warn" variant="ghost" full={false} onPress={() => { setHoiXoa(item.id); setLoiXoa(null); }} />}<CauTaiCho cau={loiXoa?.id === item.id ? loiXoa.cau : null} hanhDong={{ label: "Thử lại", onPress: () => void xoa(item.id) }} /></View>)}</RudiScreen>;
}
