import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useMemo, useState } from "react";
import { Text, View } from "react-native";
import { typography, useRudiTheme } from "../theme";
import { RudiButton, RudiScreen, TopBar } from "../ui";
import { Sheet } from "../ui/Sheet";
import { ErrorState } from "../ui/ErrorState";
import { BookView } from "./BookView";
import { diaryImage, publishedPhoto, readDiary, removeDiary, privatizeDiary, type Diary } from "./api";
import { newAttempt, translatedAsActor } from "../../api";
import { COMMUNITY_ERRORS } from "../community/api";
export function DiaryScreen({ person, id }: { person: string; id: string }) {
  const router = useRouter(); const { colors } = useRudiTheme(); const [book, setBook] = useState<Diary | null>(null);
  const [error, setError] = useState<string | null>(null); const [busy, setBusy] = useState(false); const [deleting, setDeleting] = useState(false);
  const shareAttempt = useMemo(() => newAttempt().key, [id, book?.revision]);
  const load = useCallback(async () => { setError(null); try { setBook(await readDiary(person, id)); } catch (e) { setBook(null); setError(e instanceof Error ? e.message : "Chưa mở được cuốn sổ."); } }, [person, id]);
  useFocusEffect(useCallback(() => { void load(); }, [load]));
  async function change(remove: boolean) {
    if (!book) return; setBusy(true); setError(null);
    // Opened from a link there may be nothing behind this screen, and back()
    // would leave the person on a book that no longer exists.
    try { if (remove) { await removeDiary(person, id); setDeleting(false); if (router.canGoBack()) router.back(); else router.replace("/(tabs)/profile" as never); } else { setBook(await privatizeDiary(person, book)); } }
    catch (e) { setError(e instanceof Error ? e.message : "Chưa cất được sổ. Bạn thử lại nhé."); } finally { setBusy(false); }
  }
  return <RudiScreen testID="diary-reader-screen" overlay={<Sheet open={deleting} onClose={() => setDeleting(false)} accessibilityLabel="Xóa cuốn sổ">
    <Text style={[typography.h2, { color: colors.ink }]}>Bỏ cuốn sổ này khỏi tường?</Text>
    <Text style={[typography.body, { color: colors.inkSoft }]}>Ảnh gốc trong hội vẫn còn. Cuốn sổ và những bản sửa của riêng bạn sẽ bị xóa.</Text>
    <RudiButton label="Xóa cuốn sổ" loading={busy} onPress={() => void change(true)} tone="warn" variant="outline" /><RudiButton label="Giữ sổ lại" variant="ghost" onPress={() => setDeleting(false)} />
  </Sheet>}>
    <TopBar title={book?.kind === "moment" ? "Khoảnh khắc" : "Sổ chuyến đi"} />
    {error ? <ErrorState title="Sổ chưa mở được" body={error} onRetry={() => void load()} /> : null}
    {!book && !error ? <Text style={[typography.body, { color: colors.inkSoft }]}>Đang mở sổ…</Text> : null}
    {book ? <View style={{ gap: 24 }}><BookView kind={book.kind} document={book.document} photo={(photo) => diaryImage(person, publishedPhoto(id, photo))} />
      <Text style={[typography.caption, { color: colors.inkSoft }]}>{book.audience === "private" ? "Chỉ mình tôi" : "Công khai"}{book.document.ai_generated ? " · Có Nếp giúp viết" : ""}</Text>
      {book.owner_id === person ? <><RudiButton label="Sửa cuốn sổ" onPress={() => router.push(`/outings/${book.outing_id}/ending` as never)} />
        {book.audience === "public" ? <RudiButton label="Gửi sổ lên cộng đồng để duyệt" variant="outline" disabled={busy} onPress={() => { setBusy(true); void translatedAsActor<{ id: string }>(COMMUNITY_ERRORS, `/v2/community/diaries/${id}/share`, { actorId: person, method: "POST", attempt: newAttempt(), body: { logical_id: shareAttempt, revision: book.revision, confirmed: true, topics: [book.kind === "trip" ? "du lịch" : "khoảnh khắc"] } }).then((p) => router.push(`/community/posts/${p.id}` as never)).catch((e) => setError(e.message)).finally(() => setBusy(false)); }} /> : null}
        {book.audience === "public" ? <RudiButton label="Cất về riêng tư" variant="outline" loading={busy} onPress={() => void change(false)} /> : null}
        <RudiButton label="Xóa cuốn sổ" tone="warn" variant="ghost" onPress={() => setDeleting(true)} /></> : null}
    </View> : null}
  </RudiScreen>;
}
