import { Image } from "expo-image";
import { useState } from "react";
import { StyleSheet, Text, View } from "react-native";
import type { ImageSource } from "expo-image";
import { typography, useRudiTheme } from "../theme";
import { ToGiay } from "../ui/ToGiay";
import { ngayKieuViet } from "../chat/to-hen-chung";
import type { DiaryDocument, DiaryKind } from "./api";

/** One reading surface, shared by the private editor and the published book. */
export function BookView({ document, photo, compact = false, kind = "trip" }: { document: DiaryDocument; photo: (id: string) => ImageSource | undefined; compact?: boolean; kind?: DiaryKind }) {
  const { colors } = useRudiTheme();
  const [width, setWidth] = useState(0);
  return <View style={styles.book} onLayout={(event) => setWidth(event.nativeEvent.layout.width)}>
    {kind === "moment" ? <View style={[styles.moment, { borderColor: colors.lineStrong }]}>
      <View style={[styles.dot, { backgroundColor: colors.accent }]} />
      {document.cover_id ? <Image accessibilityLabel={`Ảnh khoảnh khắc ${document.title}`} source={photo(document.cover_id)} contentFit="cover" cachePolicy="none" style={styles.momentPhoto} /> : null}
      <Text style={[typography.h2, { color: colors.ink }]}>{document.title}</Text>
      {document.subtitle ? <Text style={[typography.note, { color: colors.inkSoft }]}>{document.subtitle}</Text> : null}
    </View> : <ToGiay dan style={styles.cover}>
      {document.cover_id ? <Image accessibilityLabel={`Ảnh bìa ${document.title}`} source={photo(document.cover_id)} contentFit="cover" cachePolicy="none" style={[styles.coverPhoto, compact && styles.compactPhoto]} /> : null}
      <View style={styles.coverWords}>
        <Text style={[compact ? typography.h2 : typography.h1, { color: colors.ink }]}>{document.title}</Text>
        <Text style={[typography.body, { color: colors.inkSoft }]}>{document.subtitle}</Text>
      </View>
    </ToGiay>}
    {!compact ? document.pages.map((page, i) => <View key={i} style={[styles.page, { borderColor: colors.line }]}>
      {page.heading ? <Text style={[typography.h2, { color: colors.ink }]}>{/^\d{4}-\d{2}-\d{2}$/.test(page.heading) ? `Ngày ${ngayKieuViet(page.heading)}` : page.heading}</Text> : null}
      <View style={page.layout === "collage" ? styles.collage : styles.single}>
        {page.photo_ids.map((id) => <View key={id} style={page.layout === "collage" ? { width: Math.max(0, (width - 8) / 2), height: Math.max(0, (width - 8) / 2) * 4 / 3 } : styles.pagePhoto}>
          <Image accessibilityLabel={`Ảnh trang ${i + 1}`} source={photo(id)} contentFit="cover" cachePolicy="none" style={StyleSheet.absoluteFill} />
        </View>)}
      </View>
      {page.text ? <Text style={[typography.body, { color: colors.inkSoft, lineHeight: 28 }]}>{page.text}</Text> : null}
      <Text style={[typography.caption, { color: colors.inkFaint, alignSelf: "flex-end" }]}>{i + 1} / {document.pages.length}</Text>
    </View>) : null}
  </View>;
}
const styles = StyleSheet.create({
  book: { gap: 28 }, cover: { overflow: "hidden", padding: 0 }, coverPhoto: { width: "100%", aspectRatio: 4 / 3 },
  compactPhoto: { aspectRatio: 16 / 9 },
  coverWords: { padding: 22, gap: 12 }, page: { gap: 16, paddingBottom: 24, borderBottomWidth: StyleSheet.hairlineWidth },
  collage: { flexDirection: "row", flexWrap: "wrap", gap: 8 }, single: { gap: 8 }, pagePhoto: { width: "100%", aspectRatio: 4 / 3 },
  moment: { borderLeftWidth: 1, paddingLeft: 20, gap: 12, paddingBottom: 12 },
  dot: { position: "absolute", top: 0, left: -4, width: 7, height: 7, borderRadius: 4 },
  momentPhoto: { width: "100%", aspectRatio: 3 / 2 },
});
