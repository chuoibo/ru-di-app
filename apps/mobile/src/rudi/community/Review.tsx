import { Image } from "expo-image";
import { useRouter } from "expo-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { Text, View } from "react-native";
import { ApiError, newAttempt, translatedAsActor } from "../../api";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { Field, RudiButton, RudiScreen, TopBar } from "../ui";
import { COMMUNITY_ERRORS, imageSource, type Media } from "./api";
import { CommunityVideo } from "./PostCard";
import { nhanTrangThaiDuyet } from "./bang-tin";
import { CauTaiCho } from "../ui/CauTaiCho";
import { EmptyState } from "../ui/EmptyState";
import { Canh } from "../ui/art/Canh";
import { HoiTaiHang } from "../ui/HoiTaiHang";
import { SkeletonGroup, SkeletonRow } from "../ui/Skeleton";

type ReviewItem = { id: string; revision?: number; body: string; status: string; reason: string; media?: Media[]; media_id?: string; comment?: boolean };
export function Review() {
  const { phien } = useRudiSession();
  const { colors } = useRudiTheme();
  const router = useRouter();
  const working = useRef(false); const reading = useRef(0);
  const [items, setItems] = useState<ReviewItem[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [reasons, setReasons] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [forbidden, setForbidden] = useState(false);
  const [confirm, setConfirm] = useState<{ id: string; approve: boolean } | null>(null);
  // A decision that did not land is said under the item it was about.
  const [loiMuc, setLoiMuc] = useState<{ id: string; cau: string } | null>(null);
  const load = useCallback(async () => {
    if (!phien) return;
    const serial = ++reading.current;
    try {
      const page = await translatedAsActor<{ posts: ReviewItem[]; comments: ReviewItem[] }>(COMMUNITY_ERRORS, "/v2/community/review", { actorId: phien.person_id, method: "GET" });
      if (reading.current !== serial) return;
      setItems([...page.posts, ...page.comments.map((c) => ({ ...c, comment: true }))]);
      setLoaded(true);
      setForbidden(false);
      setError(null);
    } catch (e) { if (reading.current !== serial) return; setForbidden(e instanceof ApiError && e.status === 403); setError(e instanceof Error ? e.message : "Chưa mở được hàng đợi."); }
  }, [phien]);
  useEffect(() => { setItems([]); setLoaded(false); setForbidden(false); setError(null); void load(); return () => { reading.current++; }; }, [load]);
  const decide = async (item: ReviewItem, approve: boolean) => {
    if (!phien || working.current || (reasons[item.id] ?? "").trim().length < 3) return;
    working.current = true;
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
      setConfirm(null);
    } catch (e) { setLoiMuc({ id: item.id, cau: e instanceof Error ? e.message : "Chưa lưu được quyết định." }); }
    finally { working.current = false; setBusy(false); }
  };
  return <RudiScreen material="quiet" onRefresh={forbidden ? undefined : load} testID="community-review">
    <TopBar title="Xem xét nội dung" />
    {forbidden ? <EmptyState kind="permission" illustration={<Canh id="bo-loc-che-het" width={112} nep={false} />} title="Bạn chưa có quyền xem xét nội dung" body="Trang này dành cho người vận hành được cấp quyền. Bạn vẫn có thể đọc và chia sẻ trong cộng đồng." action={{ label: "Quay lại", onPress: () => { if (router.canGoBack()) router.back(); else router.replace("/(tabs)/community" as never); } }} /> : <CauTaiCho cau={error} hanhDong={{ label: "Thử lại", onPress: () => void load() }} />}
    {!loaded && !error ? <SkeletonGroup><SkeletonRow /><SkeletonRow /></SkeletonGroup> : null}
    {loaded && !items.length && !error ? <EmptyState kind="first-use" illustration={<Canh id="chua-co-bai" width={112} nep={false} />} title="Hàng đợi đã được xem hết" body="Nội dung cần quyết định sẽ xuất hiện ở đây." /> : null}
    {!forbidden ? items.map((item) => <View key={item.id} style={{ paddingVertical: 24, gap: 16, borderBottomWidth: 1, borderColor: colors.line }}>
      <Text style={[typography.caption, { color: colors.inkFaint }]}>{item.comment ? "Bình luận" : `Bài đăng · phiên bản ${item.revision}`} · {nhanTrangThaiDuyet(item.status)}</Text>
      <Text style={[typography.body, { color: colors.ink }]}>{item.body}</Text>
      {phien ? (item.media ?? (item.media_id ? [{ id: item.media_id, type: "image/jpeg", url: `/v2/community/media/${item.media_id}` } as Media] : [])).map((m) => m.type.startsWith("video/")
        ? <CommunityVideo key={m.id} person={phien.person_id} media={m} active />
        : <Image key={m.id} source={imageSource(phien.person_id, m.url)} cachePolicy="none" style={{ width: "100%", height: 260 }} contentFit="contain" />) : null}
      <Field label="Lý do quyết định" value={reasons[item.id] ?? ""} onChangeText={(value) => setReasons((old) => ({ ...old, [item.id]: value }))} multiline placeholder="Ghi rõ căn cứ sau khi xem toàn bộ nội dung…" />
      {confirm?.id === item.id ? <HoiTaiHang cau={confirm.approve ? "Duyệt nội dung này công khai với lý do đã ghi?" : "Từ chối nội dung này với lý do đã ghi?"} nhan={confirm.approve ? "Duyệt công khai" : "Từ chối"} dangLam={busy} onDongY={() => void decide(item, confirm.approve)} onThoi={() => setConfirm(null)} /> : <>
      <RudiButton label="Duyệt công khai" disabled={busy || (reasons[item.id] ?? "").trim().length < 3} lyDo={!busy && (reasons[item.id] ?? "").trim().length < 3 ? "Ghi lý do trước khi duyệt, ít nhất 3 ký tự." : undefined} onPress={() => setConfirm({ id: item.id, approve: true })} />
      <RudiButton label="Từ chối" variant="outline" disabled={busy || (reasons[item.id] ?? "").trim().length < 3} lyDo={!busy && (reasons[item.id] ?? "").trim().length < 3 ? "Ghi lý do trước khi từ chối, ít nhất 3 ký tự." : undefined} onPress={() => setConfirm({ id: item.id, approve: false })} /></>}
      <CauTaiCho cau={loiMuc?.id === item.id ? loiMuc.cau : null} />
    </View>) : null}
  </RudiScreen>;
}
