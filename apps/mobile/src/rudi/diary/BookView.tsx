import { Image } from "expo-image";
import { useEffect, useState } from "react";
import { StyleSheet, Text, View } from "react-native";
import type { ImageSource } from "expo-image";
import { typography, useRudiTheme } from "../theme";
import { RudiButton } from "../ui";
import { SoBia } from "../ui/SoBia";
import { TrangSo } from "../ui/TrangSo";
import { KhungAnh } from "../ui/KhungAnh";
import { Washi } from "../ui/Washi";
import { tenTrang } from "./ten-trang";
import type { DiaryDocument, DiaryKind } from "./api";

/** One reading surface, shared by the private editor and the published book. */
export function BookView({ document, photo, compact = false, kind = "trip", paged = false }: { document: DiaryDocument; photo: (id: string) => ImageSource | undefined; compact?: boolean; kind?: DiaryKind; paged?: boolean }) {
  const { colors } = useRudiTheme();
  const [pageIndex, setPageIndex] = useState(0);
  useEffect(() => { setPageIndex(0); }, [document]);
  const [width, setWidth] = useState(0);
  const [coverHeight, setCoverHeight] = useState(240);
  const coverWidth = Math.max(1, Math.min(width, 420));
  return <View style={styles.book} onLayout={(event) => setWidth(event.nativeEvent.layout.width)}>
    {kind === "moment" ? <View style={styles.moment}>
      <Washi height={18} tilt={-2} style={styles.tape} />
      {document.cover_id ? <KhungAnh tilt={-1}><BookPhoto accessibilityLabel={`Ảnh khoảnh khắc ${document.title}`} source={photo(document.cover_id)} contentFit="cover" cachePolicy="none" style={styles.momentPhoto} /></KhungAnh> : null}
      <Text style={[typography.h2, { color: colors.ink }]}>{document.title}</Text>
      {document.subtitle ? <Text style={[typography.note, { color: colors.inkSoft }]}>{document.subtitle}</Text> : null}
    </View> : <SoBia ten={[]} rong={coverWidth} cao={coverHeight + 48} style={styles.cover} nhan={
      <View onLayout={(event) => setCoverHeight(Math.ceil(event.nativeEvent.layout.height))} style={[styles.coverLabel, { backgroundColor: colors.card }]}>
        {document.cover_id ? <BookPhoto accessibilityLabel={`Ảnh bìa ${document.title}`} source={photo(document.cover_id)} contentFit="cover" cachePolicy="none" style={[styles.coverPhoto, compact && styles.compactPhoto]} /> : null}
        <View style={styles.coverWords}>
          <Text style={[compact ? typography.h2 : typography.h1, { color: colors.ink }]}>{document.title}</Text>
          {document.subtitle ? <Text style={[typography.body, { color: colors.inkSoft }]}>{document.subtitle}</Text> : null}
        </View>
      </View>
    } />}
    {!compact ? document.pages.map((page, i) => !paged || i === pageIndex ? <TrangSo key={i} tone="accent" ke={false} style={styles.page}>
      {page.heading ? <Text style={[typography.h2, { color: colors.ink }]}>{tenTrang(page.heading)}</Text> : null}
      <View style={page.layout === "collage" ? styles.collage : styles.single}>
        {page.photo_ids.map((id) => <View key={id} style={page.layout === "collage" ? { width: Math.max(0, (width - 66) / 2), height: Math.max(0, (width - 66) / 2) * 4 / 3 } : styles.pagePhoto}>
          <BookPhoto accessibilityLabel={`Ảnh trang ${i + 1}`} source={photo(id)} contentFit="cover" cachePolicy="none" style={StyleSheet.absoluteFill} />
        </View>)}
      </View>
      {page.text ? <Text style={[typography.body, { color: colors.inkSoft, lineHeight: 28 }]}>{page.text}</Text> : null}
      <Text style={[typography.caption, { color: colors.inkFaint, alignSelf: "flex-end" }]}>{i + 1} / {document.pages.length}</Text>
    </TrangSo> : null) : null}
    {!compact && paged && document.pages.length > 1 ? <View style={styles.navigation}><RudiButton full={false} label="Trang trước" variant="outline" disabled={pageIndex === 0} lyDo={pageIndex === 0 ? "Bạn đang ở trang đầu tiên." : undefined} onPress={() => setPageIndex((i) => Math.max(0, i - 1))}/><Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.inkSoft }]}>Trang {pageIndex + 1} / {document.pages.length}</Text><RudiButton full={false} label="Trang sau" variant="outline" disabled={pageIndex >= document.pages.length - 1} lyDo={pageIndex >= document.pages.length - 1 ? "Bạn đang ở trang cuối cùng." : undefined} onPress={() => setPageIndex((i) => Math.min(document.pages.length - 1, i + 1))}/></View> : null}
  </View>;
}
function BookPhoto(props: React.ComponentProps<typeof Image>) {
  const { colors } = useRudiTheme();
  const [failed, setFailed] = useState(false);
  useEffect(() => { setFailed(false); }, [props.source]);
  if (failed || !props.source) return <View style={[props.style, { alignItems: "center", justifyContent: "center", backgroundColor: colors.paperShade, padding: 16 }]}><Text style={[typography.caption, { color: colors.inkSoft }]}>Ảnh chưa mở được. Nội dung cuốn sổ vẫn còn.</Text></View>;
  return <Image {...props} onError={() => setFailed(true)}/>;
}
const styles = StyleSheet.create({
  navigation: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", justifyContent: "space-between", gap: 12 },
  book: { gap: 28, width: "100%", maxWidth: 560, alignSelf: "center" }, cover: { alignSelf: "center" }, coverLabel: { padding: 8, gap: 8 }, coverPhoto: { width: "100%", aspectRatio: 4 / 3 },
  compactPhoto: { aspectRatio: 16 / 9 },
  coverWords: { padding: 8, gap: 12 }, page: { gap: 16, paddingBottom: 24 },
  collage: { flexDirection: "row", flexWrap: "wrap", gap: 8 }, single: { gap: 8 }, pagePhoto: { width: "100%", aspectRatio: 4 / 3 },
  moment: { gap: 16, paddingTop: 10, paddingHorizontal: 8, paddingBottom: 12 },
  tape: { alignSelf: "center", marginBottom: -22, zIndex: 1 },
  momentPhoto: { width: "100%", aspectRatio: 3 / 2 },
});
