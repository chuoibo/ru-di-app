import { useCallback, useEffect, useRef, useState } from "react";
import { HoiTaiHang } from "../ui/HoiTaiHang";
import { StyleSheet, View } from "react-native";
import { Text } from "../ui/NativeFontMetrics";
import { newAttempt, translatedAsActor } from "../../api";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { Card, IconButton, RudiButton, RudiScreen, TopBar } from "../ui";
import { COMMUNITY_ERRORS, relativeTime } from "./api";
import { CauTaiCho } from "../ui/CauTaiCho";
import { EmptyState } from "../ui/EmptyState";
import { useRouter } from "expo-router";
import { SkeletonGroup, SkeletonRow } from "../ui/Skeleton";
import { CommunityFontMetrics } from "./Comments";

export function Keeps() {
  return <CommunityFontMetrics><KeepsContent /></CommunityFontMetrics>;
}

function KeepsContent() {
  const router = useRouter();
  const { phien } = useRudiSession(); const { colors } = useRudiTheme(); const [items, setItems] = useState<{ id: string; body: string; ai_generated: boolean; created_at: string }[]>([]); const [error, setError] = useState<string | null>(null);
  // Read at least once: an empty list before the first answer is not «nothing kept».
  const deleting = useRef(false); const reading = useRef(0);
  const [daDoc, setDaDoc] = useState(false);
  // Deleting a keep cannot be undone: it asks in its row first, waits while it
  // goes, and a refusal is said under that row, not at the top of the page.
  const [hoiXoa, setHoiXoa] = useState<string | null>(null);
  const [dangXoa, setDangXoa] = useState<string | null>(null);
  const [loiXoa, setLoiXoa] = useState<{ id: string; cau: string } | null>(null);
  const xoa = async (id: string) => {
    if (!phien || deleting.current) return;
    deleting.current = true;
    setDangXoa(id); setLoiXoa(null);
    try { await translatedAsActor<void>(COMMUNITY_ERRORS, `/v2/community/keeps/${id}`, { actorId: phien.person_id, method: "DELETE", attempt: newAttempt() }); setHoiXoa(null); await load(); }
    catch (e) { setLoiXoa({ id, cau: e instanceof Error ? e.message : "Chưa xoá được ghi chép này." }); }
    finally { deleting.current = false; setDangXoa(null); }
  };
  const load = useCallback(async () => { if (!phien) return; const serial = ++reading.current; try { const page = await translatedAsActor<{ keeps: typeof items }>(COMMUNITY_ERRORS, "/v2/community/keeps", { actorId: phien.person_id, method: "GET" }); if (reading.current !== serial) return; setItems(page.keeps); setDaDoc(true); setError(null); } catch (e) { if (reading.current !== serial) return; setError(e instanceof Error ? e.message : "Chưa mở được ghi chép."); } }, [phien]); useEffect(() => { setItems([]); setDaDoc(false); setError(null); void load(); return () => { reading.current++; }; }, [load]);
  return (
    <RudiScreen material="quiet" onRefresh={load} testID="community-keeps">
      <View style={styles.header}>
        <TopBar title="Điều mình muốn giữ" />
        <Text style={[typography.body, styles.fullText, { color: colors.inkSoft }]}>
          Những ghi chép riêng của bạn. Chỉ mình bạn đọc được.
        </Text>
      </View>
      <KeepsFailure cau={error} onRetry={() => void load()} testID="community-keeps-read-error" />
      {!daDoc && !error ? <SkeletonGroup><SkeletonRow /><SkeletonRow /></SkeletonGroup> : null}
      {daDoc && !error && items.length === 0 ? (
        // An empty page explains the same way in; its copy shares this surface's owner.
        <EmptyState nativeMetrics layout="inline" style={styles.empty} body="Nhờ Nếp giữ một điều từ câu chuyện bạn chọn. Ghi chép chỉ mình bạn đọc được." kind="first-use" title="Chưa có ghi chép nào" action={{ label: "Tới cộng đồng", onPress: () => router.push("/(tabs)/community" as never) }} />
      ) : null}
      <View style={styles.list}>
        {items.map((item) => (
          <Card key={item.id} style={styles.card}>
            <Text style={[typography.caption, styles.fullText, { color: colors.inkSoft }]}>
              {relativeTime(item.created_at)}{item.ai_generated ? " · Có Nếp giúp viết" : ""}
            </Text>
            <Text style={[typography.body, styles.fullText, { color: colors.ink, lineHeight: 28 }]}>{item.body}</Text>
            <View style={[styles.actions, { borderTopColor: colors.line }]}>
              {hoiXoa === item.id ? (
                <View style={styles.fullWidth}>
                  <HoiTaiHang nativeMetrics cau="Xoá ghi chép này? Không lấy lại được." dangLam={dangXoa === item.id} nhan="Xoá" onDongY={() => void xoa(item.id)} onThoi={() => { setHoiXoa(null); setLoiXoa(null); }} />
                </View>
              ) : (
                <IconButton accessibilityLabel={`Xoá ghi chép: «${item.body.slice(0, 40)}»`} icon="trash-outline" quiet onPress={() => { setHoiXoa(item.id); setLoiXoa(null); }} />
              )}
            </View>
            <KeepsFailure cau={loiXoa?.id === item.id ? loiXoa.cau : null} onRetry={() => void xoa(item.id)} testID="community-keeps-delete-error" />
          </Card>
        ))}
      </View>
    </RudiScreen>
  );
}

/** Keep the refusal at reading width; recovery takes its own trailing row. */
function KeepsFailure({ cau, onRetry, testID }: { cau: string | null; onRetry: () => void; testID: string }) {
  if (!cau) return null;
  return (
    <View style={styles.failure} testID={testID}>
      <CauTaiCho cau={cau} />
      <View style={styles.retry}>
        <RudiButton compact full={false} label="Thử lại" variant="outline" style={styles.retryButton} onPress={onRetry} />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  header: { gap: 8 },
  fullText: { width: "100%", minWidth: 0 },
  fullWidth: { width: "100%" },
  empty: { alignItems: "stretch" },
  list: { gap: 16, width: "100%", maxWidth: 620, alignSelf: "center" },
  // A private note is a quiet reading card, without the ledger's punched margin.
  card: { gap: 12, shadowOpacity: 0, elevation: 0 },
  actions: { borderTopWidth: StyleSheet.hairlineWidth, paddingTop: 4, alignItems: "flex-end" },
  failure: { gap: 8, alignSelf: "stretch" },
  retry: { alignItems: "flex-end" },
  retryButton: { maxWidth: "100%" },
});
