import { AssetIllustration } from "../ui/AssetIllustration";
import { useRouter } from "expo-router";
import { useEffect, useState, type ReactNode } from "react";
import { Pressable, StyleSheet, View } from "react-native";
import { Text } from "../ui/NativeFontMetrics";
import { SkeletonLines } from "../ui/Skeleton";
import { docKeoCuaNhom } from "../keo/keo";
import { homNay } from "../keo/nhip-keo";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { Heading, RudiButton } from "../ui";
import { EmptyState } from "../ui/EmptyState";
import { ErrorState } from "../ui/ErrorState";
import { thangNamVN } from "../ngay-viet";
import { loiVaoTrangDau, type LoiVaoTrangDau } from "./trang-dau";
import { BookView } from "./BookView";
import { diaryImage, publishedPhoto } from "./api";
import { useDiaryCollection, type DiaryCollection } from "./useDiaryCollection";

type WallProps = { person: string; owner: string; collection?: DiaryCollection; empty?: ReactNode; failure?: ReactNode };

export function DiaryWall(props: WallProps) {
  return props.collection ? <DiaryShelf {...props} collection={props.collection} /> : <OwnedDiaryWall {...props} />;
}

function OwnedDiaryWall(props: WallProps) {
  const collection = useDiaryCollection(props.person, props.owner);
  return <DiaryShelf {...props} collection={collection} />;
}

function DiaryShelf({ person, owner, collection, empty, failure }: WallProps & { collection: DiaryCollection }) {
  const router = useRouter(); const { colors } = useRudiTheme();
  const { books, next, error, loading } = collection;
  return <View style={styles.wall} testID="diary-wall">
    <Heading size="h2" title="Những ngày muốn giữ" subtitle={person === owner ? "Những cuộc đi trở thành chuyện của bạn." : undefined} />
    {error ? failure ?? <ErrorState title="Sổ chưa mở được" body={error} onRetry={collection.retry} /> : null}
    {!loading && !error && !books.length ? empty ?? (person === owner ? <KeTrong /> : <Text style={[typography.body, { color: colors.inkSoft }]}>Chưa có trang nào được mở cho mọi người.</Text>) : null}
    {loading && !books.length ? <SkeletonLines lines={3}/> : null}
    {books.map((b, i) => <View key={b.id} style={styles.entry}>
      {b.kind === "moment" && (i === 0 || books[i - 1].created_at.slice(0, 7) !== b.created_at.slice(0, 7)) ? <Text style={[typography.label, { color: colors.inkSoft }]}>{thangNamVN(b.created_at)}</Text> : null}
      <Pressable accessibilityRole="button" accessibilityLabel={`Mở ${b.document.title}`} onPress={() => router.push(`/diaries/${b.id}` as never)}>
        <BookView compact kind={b.kind} document={b.document} photo={(id) => diaryImage(person, publishedPhoto(b.id, id))} />
      </Pressable>
      <Text style={[typography.caption, { color: colors.inkSoft }]}>{b.kind === "trip" ? "Sổ chuyến đi" : "Khoảnh khắc"} · {b.audience === "private" ? "Chỉ mình tôi" : "Công khai"}</Text>
    </View>)}
    {next ? <RudiButton label="Mở những trang trước" variant="outline" loading={loading} onPress={collection.more} /> : null}
  </View>;
}
/**
 * The owner's empty shelf (QA UI-153): one action to where a first page
 * starts. The group's outings are read once; until they answer, the button
 * waits rather than offering a door it may have to take back. A failed read
 * still leaves a way: the group's outings.
 */
function KeTrong() {
  const router = useRouter();
  const { phien } = useRudiSession();
  const [loi, setLoi] = useState<LoiVaoTrangDau | null>(null);
  const contextId = phien?.context_id ?? null;
  const personId = phien?.person_id ?? null;
  useEffect(() => {
    let conDung = true;
    if (contextId === null || personId === null) { setLoi({ kieu: "plan" }); return; }
    docKeoCuaNhom(contextId, personId)
      .then((keo) => { if (conDung) setLoi(loiVaoTrangDau(keo, homNay())); })
      .catch(() => { if (conDung) setLoi({ kieu: "plan" }); });
    return () => { conDung = false; };
  }, [contextId, personId]);
  const action = loi === null
    ? { label: "Mở cuộc đi vừa qua", onPress: () => {}, loading: true }
    : loi.kieu === "keo"
      ? { label: `Mở «${loi.ten}»`, onPress: () => router.push(`/outings/${loi.id}` as never) }
      : { label: "Xem kèo của nhóm", onPress: () => router.push("/(tabs)/plan" as never) };
  return <EmptyState
    action={action}
    body={loi?.kieu === "keo" ? "Mở cuộc đi đã qua rồi chạm «Giữ lại cuộc đi»: nó thành trang đầu tiên của bạn." : "Khi một cuộc đi khép lại, giữ nó lại từ màn của kèo: nó thành trang đầu tiên ở đây."}
    illustration={<AssetIllustration name="spot-memory" width={220} height={188} />}
    kind="first-use"
    layout="inline"
    testID="ke-trong"
    title="Chưa giữ ngày nào"
  />;
}

const styles = StyleSheet.create({ wall: { gap: 24, paddingVertical: 20 }, entry: { gap: 10 } });
