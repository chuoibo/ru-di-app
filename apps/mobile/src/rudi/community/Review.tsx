import { Image } from "expo-image";
import { useCallback, useEffect, useState } from "react";
import { Text, View } from "react-native";
import { newAttempt, translatedAsActor } from "../../api";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { Field, RudiButton, RudiScreen, TopBar } from "../ui";
import { COMMUNITY_ERRORS, imageSource, type Media } from "./api";
import { CommunityVideo } from "./PostCard";
import { nhanTrangThaiDuyet } from "./bang-tin";
import { CauTaiCho } from "../ui/CauTaiCho";

type ReviewItem = { id: string; revision?: number; body: string; status: string; reason: string; media?: Media[]; media_id?: string; comment?: boolean };
export function Review() {
  const { phien } = useRudiSession();
  const { colors } = useRudiTheme();
  const [items, setItems] = useState<ReviewItem[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [reasons, setReasons] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  // A decision that did not land is said under the item it was about.
  const [loiMuc, setLoiMuc] = useState<{ id: string; cau: string } | null>(null);
  const load = useCallback(async () => {
    if (!phien) return;
    try {
      const page = await translatedAsActor<{ posts: ReviewItem[]; comments: ReviewItem[] }>(COMMUNITY_ERRORS, "/v2/community/review", { actorId: phien.person_id, method: "GET" });
      setItems([...page.posts, ...page.comments.map((c) => ({ ...c, comment: true }))]);
      setError(null);
    } catch (e) { setError(e instanceof Error ? e.message : "Chưa mở được hàng đợi."); }
  }, [phien]);
  useEffect(() => { void load(); }, [load]);
  const decide = async (item: ReviewItem, approve: boolean) => {
    if (!phien || busy) return;
    setBusy(true);
    setLoiMuc(null);
    const body = { revision: item.revision, approve, reason: reasons[item.id] ?? "" };
    try {
      if (item.comment) {
        await translatedAsActor(COMMUNITY_ERRORS, `/v2/community/comments/${item.id}/review`, { actorId: phien.person_id, method: "POST", body: { approve, reason: body.reason }, attempt: newAttempt() });
      } else {
        await translatedAsActor(COMMUNITY_ERRORS, `/v2/community/posts/${item.id}/review`, { actorId: phien.person_id, method: "POST", body, attempt: newAttempt() });
      }
      await load();
    } catch (e) { setLoiMuc({ id: item.id, cau: e instanceof Error ? e.message : "Chưa lưu được quyết định." }); }
    finally { setBusy(false); }
  };
  return <RudiScreen onRefresh={load}>
    <TopBar title="Xem xét nội dung" />
    <CauTaiCho cau={error} hanhDong={{ label: "Thử lại", onPress: () => void load() }} />
    {!items.length && !error ? <Text style={[typography.body, { color: colors.inkSoft }]}>Chưa có nội dung cần xem xét.</Text> : null}
    {items.map((item) => <View key={item.id} style={{ paddingVertical: 24, gap: 16, borderBottomWidth: 1, borderColor: colors.line }}>
      <Text style={[typography.caption, { color: colors.inkFaint }]}>{item.comment ? "Bình luận" : `Bài đăng · phiên bản ${item.revision}`} · {nhanTrangThaiDuyet(item.status)}</Text>
      <Text style={[typography.body, { color: colors.ink }]}>{item.body}</Text>
      {phien ? (item.media ?? (item.media_id ? [{ id: item.media_id, type: "image/jpeg", url: `/v2/community/media/${item.media_id}` } as Media] : [])).map((m) => m.type.startsWith("video/")
        ? <CommunityVideo key={m.id} person={phien.person_id} media={m} active />
        : <Image key={m.id} source={imageSource(phien.person_id, m.url)} cachePolicy="none" style={{ width: "100%", height: 260 }} contentFit="contain" />) : null}
      <Field label="Lý do quyết định" value={reasons[item.id] ?? ""} onChangeText={(value) => setReasons((old) => ({ ...old, [item.id]: value }))} multiline placeholder="Ghi rõ căn cứ sau khi xem toàn bộ nội dung…" />
      <RudiButton label="Duyệt công khai" disabled={busy || (reasons[item.id] ?? "").trim().length < 3} lyDo={!busy && (reasons[item.id] ?? "").trim().length < 3 ? "Ghi lý do trước khi duyệt, ít nhất 3 ký tự." : undefined} onPress={() => void decide(item, true)} />
      <RudiButton label="Từ chối" variant="outline" disabled={busy || (reasons[item.id] ?? "").trim().length < 3} lyDo={!busy && (reasons[item.id] ?? "").trim().length < 3 ? "Ghi lý do trước khi từ chối, ít nhất 3 ký tự." : undefined} onPress={() => void decide(item, false)} />
      <CauTaiCho cau={loiMuc?.id === item.id ? loiMuc.cau : null} />
    </View>)}
  </RudiScreen>;
}
