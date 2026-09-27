import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { typography, useRudiTheme } from "../theme";
import { Heading, RudiButton } from "../ui";
import { ErrorState } from "../ui/ErrorState";
import { BookView } from "./BookView";
import { diaryImage, listDiaries, publishedPhoto, type Diary } from "./api";

export function DiaryWall({ person, owner }: { person: string; owner: string }) {
  const router = useRouter(); const { colors } = useRudiTheme();
  const [books, setBooks] = useState<Diary[]>([]); const [next, setNext] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null); const [loading, setLoading] = useState(true);
  const load = useCallback(async (before?: string) => {
    setLoading(true); setError(null);
    try { const page = await listDiaries(person, owner, before); setBooks((old) => before ? [...old, ...page.diaries] : page.diaries); setNext(page.next_cursor); }
    catch (e) { setError(e instanceof Error ? e.message : "Chưa mở được những trang đã giữ."); }
    finally { setLoading(false); }
  }, [person, owner]);
  useFocusEffect(useCallback(() => { void load(); }, [load]));
  return <View style={styles.wall} testID="diary-wall">
    <Heading size="h2" title="Những ngày muốn giữ" subtitle={person === owner ? "Những cuộc đi trở thành chuyện của bạn." : undefined} />
    {error ? <ErrorState title="Sổ chưa mở được" body={error} onRetry={() => void load()} /> : null}
    {!loading && !error && !books.length ? <Text style={[typography.body, { color: colors.inkSoft }]}>{person === owner ? "Cuộc đi khép lại, một trang mới sẽ ở đây. Mở cuộc đi đã qua để giữ khoảnh khắc đầu tiên." : "Chưa có trang nào được mở cho mọi người."}</Text> : null}
    {loading && !books.length ? <Text style={[typography.body, { color: colors.inkSoft }]}>Đang mở những trang đã giữ…</Text> : null}
    {books.map((b, i) => <View key={b.id} style={styles.entry}>
      {b.kind === "moment" && (i === 0 || books[i - 1].created_at.slice(0, 7) !== b.created_at.slice(0, 7)) ? <Text style={[typography.label, { color: colors.inkSoft }]}>{new Date(b.created_at).toLocaleDateString("vi-VN", { month: "long", year: "numeric" })}</Text> : null}
      <Pressable accessibilityRole="button" accessibilityLabel={`Mở ${b.document.title}`} onPress={() => router.push(`/diaries/${b.id}` as never)}>
        <BookView compact kind={b.kind} document={b.document} photo={(id) => diaryImage(person, publishedPhoto(b.id, id))} />
      </Pressable>
      <Text style={[typography.caption, { color: colors.inkSoft }]}>{b.kind === "trip" ? "Sổ chuyến đi" : "Khoảnh khắc"} · {b.audience === "private" ? "Chỉ mình tôi" : "Công khai"}</Text>
    </View>)}
    {next ? <RudiButton label="Mở những trang trước" variant="outline" loading={loading} onPress={() => void load(next)} /> : null}
  </View>;
}
const styles = StyleSheet.create({ wall: { gap: 24, paddingVertical: 20 }, entry: { gap: 10 } });
