import { Image, type ImageSource } from "expo-image";
import { useEffect, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { chiaAlbumTheoNgay } from "../../ky-niem/ky-niem";
import { typography, useRudiTheme } from "../../theme";
import { ResponsiveRow, SectionHeader } from "../../ui";
import { KhungAnh } from "../../ui/KhungAnh";
import { Canh } from "../../ui/art/Canh";

/**
 * A photograph of the group's own album. It owes no author and no licence,
 * and it cannot be a catalogue photograph: `AnhCoGhiCong` yields its address
 * only together with its credit sentence (F31).
 */
export interface AnhAlbumHienThi {
  id: string;
  source: ImageSource;
  caption: string;
  created_at: string;
}

/**
 * A photograph that says so when it fails to load, keeping its place.
 *
 * The failed slot draws the empty-frame scene, the same object a place with
 * no picture gets, with the sentence as its caption: a flat tinted rectangle
 * stood in for a broken address for a whole board run, and a tinted rectangle
 * is the one material this world does not use (review 08/09 F01, and the
 * finish review of this batch). The ground is `card`, the paper a print is
 * made of, not `accentSoft`: on the dark theme the coral tint turns into a
 * maroon slab the size of a photograph (measured on emulator 08/09).
 */
function AnhHoacHong({ source, caption, ratio, radius }: { source: ImageSource; caption: string; ratio: number; radius: number }) {
  const { colors } = useRudiTheme();
  const [hong, setHong] = useState(false);
  useEffect(() => setHong(false), [source]);
  if (hong) {
    return (
      <View accessibilityLabel={`Chưa tải được ảnh: ${caption}`} style={[styles.hong, { aspectRatio: ratio, borderRadius: radius, backgroundColor: colors.card }]}>
        <Canh id="chua-co-anh" width={132} />
        <Text style={[typography.note, { color: colors.inkFaint }]}>Chưa tải được ảnh</Text>
      </View>
    );
  }
  return <Image accessibilityLabel={caption} cachePolicy="none" contentFit="cover" onError={() => setHong(true)} source={source} style={{ width: "100%", aspectRatio: ratio, borderRadius: radius, backgroundColor: colors.line }} />;
}

/**
 * The photographs of one album: the first leads on a print that carries its
 * own caption and the kept-corner fold; the rest are gathered by Vietnam
 * calendar day, in pairs, an odd last print wide. Whether the album spans
 * days is decided over every photograph, lead included (`chiaAlbumTheoNgay`).
 *
 * Pure of data: the live screen hands it what the server returned, the
 * synthetic lab hands it invented states (0/1/2 photos, two days with the lead
 * the only print of the first, an unreadable stamp, a long caption), so the
 * layout is measured on the renderer that ships.
 */
export function AlbumAnh({ photos, tiLeDan, onMo }: { photos: readonly AnhAlbumHienThi[]; tiLeDan: number; onMo: (index: number) => void }) {
  const { colors, radius } = useRudiTheme();
  const { nhieuNgay, nhanDan, sau: theoNgay } = chiaAlbumTheoNgay(photos);
  if (photos.length === 0) return null;
  const dan = photos[0];
  const oAnh = (photo: AnhAlbumHienThi & { viTri: number }, ratio: number) => (
    <Pressable accessibilityLabel={`Mở ảnh: ${photo.caption}`} accessibilityRole="button" key={photo.id} onPress={() => onMo(photo.viTri)}>
      <AnhHoacHong caption={photo.caption} radius={radius.small} ratio={ratio} source={photo.source} />
    </Pressable>
  );
  return (
    <>
      {photos.length > 1 ? <SectionHeader title={`${photos.length} khoảnh khắc`} /> : null}
      {nhieuNgay && nhanDan !== null ? <Text style={[typography.note, { color: colors.inkFaint }]}>{nhanDan}</Text> : null}
      <Pressable accessibilityLabel={`Mở ảnh: ${dan.caption}`} accessibilityRole="button" onPress={() => onMo(0)}>
        <KhungAnh chuThich={dan.caption} dauGiu xuatXu="Ảnh của nhóm">
          <AnhHoacHong caption={dan.caption} radius={4} ratio={tiLeDan} source={dan.source} />
        </KhungAnh>
      </Pressable>
      {theoNgay.map((ngay) => {
        const chan = ngay.anh.length - (ngay.anh.length % 2);
        return (
          <View key={ngay.ngay ?? "chua-ro"} style={styles.ngay}>
            {nhieuNgay ? <Text style={[typography.note, { color: colors.inkFaint }]}>{ngay.nhan}</Text> : null}
            {chan > 0 ? (
              <ResponsiveRow gap={6} maxColumns={4} minItemWidth={150}>
                {ngay.anh.slice(0, chan).map((photo) => oAnh(photo, 1))}
              </ResponsiveRow>
            ) : null}
            {/* An odd last print goes wide: the day ends on a beat, not a gap. */}
            {chan < ngay.anh.length ? oAnh(ngay.anh[ngay.anh.length - 1], tiLeDan) : null}
          </View>
        );
      })}
    </>
  );
}

const styles = StyleSheet.create({
  ngay: { gap: 6 },
  hong: { width: "100%", alignItems: "center", justifyContent: "center", gap: 4 },
});
