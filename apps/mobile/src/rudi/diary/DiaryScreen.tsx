import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useMemo, useRef, useState } from "react";
import { Text, View } from "react-native";
import { typography, useRudiTheme } from "../theme";
import { IconButton, RudiButton, RudiScreen, TopBar } from "../ui";
import { Sheet } from "../ui/Sheet";
import { ErrorState } from "../ui/ErrorState";
import { SkeletonLines } from "../ui/Skeleton";
import { CauTaiCho } from "../ui/CauTaiCho";
import { BookView } from "./BookView";
import { diaryImage, publishedPhoto, readDiary, removeDiary, privatizeDiary, type Diary } from "./api";
import { newAttempt, translatedAsActor } from "../../api";
import { COMMUNITY_ERRORS } from "../community/api";
export function DiaryScreen({ person, id }: { person: string; id: string }) {
  const router = useRouter(); const { colors } = useRudiTheme(); const [book, setBook] = useState<Diary | null>(null);
  const [error, setError] = useState<string | null>(null); const [busy, setBusy] = useState(false); const [deleting, setDeleting] = useState(false);
  const [management, setManagement] = useState(false);
  const working = useRef(false); const request = useRef(0);
  const shareAttempt = useMemo(() => newAttempt().key, [id, book?.revision]);
  const load = useCallback(async () => { const serial = ++request.current; setError(null); try { const result = await readDiary(person, id); if (request.current === serial) setBook(result); } catch (e) { if (request.current === serial) setError(e instanceof Error ? e.message : "Chưa mở được cuốn sổ."); } }, [person, id]);
  useFocusEffect(useCallback(() => { setBook(null); void load(); return () => { request.current++; }; }, [load]));
  async function change(remove: boolean) {
    if (!book || working.current) return; working.current = true; setBusy(true); setError(null);
    try { if (remove) { await removeDiary(person, id); setDeleting(false); if (router.canGoBack()) router.back(); else router.replace("/(tabs)/profile" as never); } else { setBook(await privatizeDiary(person, book)); } }
    catch (e) { setError(e instanceof Error ? e.message : "Chưa cất được sổ. Bạn thử lại nhé."); } finally { working.current = false; setBusy(false); }
  }
  async function share() {
    if (!book || working.current) return;
    working.current = true; setBusy(true); setError(null);
    try { const post = await translatedAsActor<{ id: string }>(COMMUNITY_ERRORS, `/v2/community/diaries/${id}/share`, { actorId: person, method: "POST", attempt: newAttempt(), body: { logical_id: shareAttempt, revision: book.revision, confirmed: true, topics: [book.kind === "trip" ? "du lịch" : "khoảnh khắc"] } }); router.push(`/community/posts/${post.id}` as never); }
    catch (e) { setError(e instanceof Error ? e.message : "Chưa gửi được sổ. Cuốn sổ vẫn ở đây."); }
    finally { working.current = false; setBusy(false); }
  }
  return <RudiScreen material="quiet" cot="doc" testID="diary-reader-screen" header={<TopBar title={book?.kind === "moment" ? "Khoảnh khắc" : "Sổ chuyến đi"} right={book?.owner_id === person ? <IconButton quiet icon="create-outline" accessibilityLabel="Sửa cuốn sổ" disabled={busy} onPress={() => router.push(`/outings/${book.outing_id}/ending` as never)} /> : undefined} />} overlay={<Sheet open={deleting} onClose={() => { if (!busy) setDeleting(false); }} accessibilityLabel="Xóa cuốn sổ">
    <Text style={[typography.h2, { color: colors.ink }]}>Bỏ cuốn sổ này khỏi tường?</Text>
    <Text style={[typography.body, { color: colors.inkSoft }]}>Ảnh gốc trong hội vẫn còn. Cuốn sổ và những bản sửa của riêng bạn sẽ bị xóa.</Text>
    <CauTaiCho cau={deleting ? error : null}/>
    <RudiButton label="Xóa cuốn sổ" loading={busy} onPress={() => void change(true)} tone="warn" variant="outline" /><RudiButton label="Giữ sổ lại" disabled={busy} variant="ghost" onPress={() => setDeleting(false)} />
  </Sheet>}>
    {!book && error ? <ErrorState title="Sổ chưa mở được" body={error} onRetry={() => void load()} /> : null}
    {!book && !error ? <SkeletonLines lines={5}/> : null}
    {book ? <View style={{ gap: 24 }}>
      <Text style={[typography.caption, { color: colors.inkSoft }]}>{book.audience === "private" ? "Chỉ mình tôi" : "Công khai"}{book.document.ai_generated ? " · Có Nếp giúp viết" : ""}</Text>
      <BookView paged kind={book.kind} document={book.document} photo={(photo) => diaryImage(person, publishedPhoto(id, photo))} />
      {!deleting ? <CauTaiCho cau={error}/> : null}
      {book.owner_id === person ? <View style={{ gap: 12, paddingTop: 8 }}>
        <RudiButton label="Quản lý sổ" icon="ellipsis-horizontal" compact variant="ghost" expanded={management} onPress={() => setManagement(!management)} />
        {management ? <>
        {book.audience === "public" ? <><Text style={[typography.body, { color: colors.inkSoft }]}>Gửi phiên bản này gồm lời viết và ảnh lên cộng đồng để duyệt. Người trong cộng đồng sẽ đọc được khi bài được duyệt.</Text><RudiButton label="Gửi sổ lên cộng đồng để duyệt" variant="outline" loading={busy} onPress={() => void share()} /><RudiButton label="Cất về riêng tư" variant="outline" disabled={busy} onPress={() => void change(false)} /></> : null}
        <RudiButton label="Xóa cuốn sổ" disabled={busy} tone="warn" variant="ghost" onPress={() => setDeleting(true)} /></> : null}
      </View> : null}
    </View> : null}
  </RudiScreen>;
}
