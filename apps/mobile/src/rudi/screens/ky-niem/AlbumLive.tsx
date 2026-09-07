/**
 * Album chuyến đi on a real session (M6): three depths, all read-only.
 *
 *   kệ (every outing of the group, with the server's counts) → một album
 *   (its photos, the places it reached, the highlights) → thước phim (the
 *   reel: a model may compose it, and the screen says so; `reeled:false`
 *   is a normal answer with a reason, not an error).
 *
 * Every count here is the server's (`AlbumSummary` / `AlbumResponse`); the
 * money line is the trip's split total from the ledger. Photos are read with
 * the caller's headers, like the wall.
 *
 * UI v2 (đợt 7): an album opens on its first photograph, large, right under a
 * short title, and the rest follow as a grid; the places and the reel come
 * after the pictures. Nothing here is called a video.
 */
import { Image } from "expo-image";
import { useRouter } from "expo-router";
import { useEffect, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import type { Phien } from "../../../phien";
import { AlbumError } from "../../../screens/album/album-api";
import {
  cauThongKeAlbum,
  cauThuocPhim,
  layAlbum,
  layDanhSachAlbum,
  layThuocPhim,
  nguonAnh,
  tenDiaDiem,
  type Album,
  type ThuocPhim,
  type TomTatAlbum,
} from "../../ky-niem/ky-niem";
import { typography, useRudiTheme } from "../../theme";
import { AiNote, Heading, ListRow, ResponsiveRow, RudiButton, RudiScreen, SectionHeader, TopBar } from "../../ui";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { KhungAnh } from "../../ui/KhungAnh";
import { PhotoViewer, type ViewerPhoto } from "../../ui/PhotoViewer";
import { SkeletonCard, SkeletonGroup, SkeletonRow } from "../../ui/Skeleton";
import { useAdaptiveLayout } from "../../ui/useAdaptiveLayout";

function loiRaChu(error: unknown): string {
  if (error instanceof AlbumError) return error.message;
  if (error instanceof Error && error.message !== "") return error.message;
  return "Chưa đọc được album từ máy chủ.";
}

function cauKhoang(a: { period_label: string; in_progress: boolean; headcount: number }): string {
  return `${a.period_label}${a.in_progress ? " · đang đi" : ""} · ${a.headcount}\u00a0người`;
}

/* ------------------------------------------------------------------ kệ */

type TrangKe = { pha: "dang-doc" } | { pha: "xong"; albums: TomTatAlbum[] } | { pha: "hong"; loi: string };

export function AlbumNhomLiveScreen({ phien, contextId }: { phien: Phien; contextId: string }) {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const [trang, setTrang] = useState<TrangKe>({ pha: "dang-doc" });

  const doc = async () => {
    try {
      const ds = await layDanhSachAlbum(contextId, phien.person_id);
      setTrang({ pha: "xong", albums: ds.albums });
    } catch (error) {
      setTrang({ pha: "hong", loi: loiRaChu(error) });
    }
  };
  useEffect(() => {
    let song = true;
    void layDanhSachAlbum(contextId, phien.person_id)
      .then((ds) => {
        if (song) setTrang({ pha: "xong", albums: ds.albums });
      })
      .catch((error: unknown) => {
        if (song) setTrang({ pha: "hong", loi: loiRaChu(error) });
      });
    return () => {
      song = false;
    };
  }, [contextId, phien.person_id]);

  return (
    <RudiScreen testID="trip-album-screen">
      <TopBar subtitle="Mỗi kèo một album" title="Album chuyến đi" />
      {trang.pha === "dang-doc" ? (
        <SkeletonGroup>
          <SkeletonRow />
          <SkeletonRow />
        </SkeletonGroup>
      ) : null}
      {trang.pha === "hong" ? <ErrorState body={trang.loi} onRetry={() => void doc()} title="Chưa đọc được album" /> : null}
      {trang.pha === "xong" && trang.albums.length === 0 ? (
        <EmptyState body="Album mọc theo kèo: tạo một kèo ở Lên plan, ảnh và check-in trong những ngày đó sẽ gom về đây." kind="first-use" layout="inline" title="Chưa có kèo nào" />
      ) : null}
      {trang.pha === "xong"
        ? trang.albums.map((a) => (
            <ListRow
              icon={a.in_progress ? "walk-outline" : "albums-outline"}
              key={a.outing_id}
              onPress={() => router.push(`/trips/${a.outing_id}/album?ctx=${contextId}` as never)}
              subtitle={`${cauKhoang(a)} · ${cauThongKeAlbum(a)}`}
              title={a.title}
            />
          ))
        : null}
    </RudiScreen>
  );
}

/* --------------------------------------------------------------- một album */

type TrangAlbum =
  | { pha: "dang-doc" }
  | { pha: "xong"; album: Album; phim: ThuocPhim | null; dangDung: boolean; loiPhim: string | null }
  | { pha: "hong"; loi: string };

export function TripAlbumLiveScreen({ phien, contextId, outingId }: { phien: Phien; contextId: string; outingId: string }) {
  const { colors, radius } = useRudiTheme();
  const { sizeClass } = useAdaptiveLayout();
  // Same rule as the fixture album: a phone reads the lead at 4:3, a tablet
  // gets a band so the grid starts on the first screen.
  const tiLeDan = sizeClass === "compact" ? 4 / 3 : 21 / 9;
  const [trang, setTrang] = useState<TrangAlbum>({ pha: "dang-doc" });
  const me = phien.person_id;
  const [viewer, setViewer] = useState<{ photos: ViewerPhoto[]; index: number; title?: string } | null>(null);

  useEffect(() => {
    let song = true;
    void layAlbum(contextId, outingId, me)
      .then((album) => {
        if (song) setTrang({ pha: "xong", album, phim: null, dangDung: false, loiPhim: null });
      })
      .catch((error: unknown) => {
        if (song) setTrang({ pha: "hong", loi: loiRaChu(error) });
      });
    return () => {
      song = false;
    };
  }, [contextId, outingId, me]);

  const dungPhim = async () => {
    if (trang.pha !== "xong") return;
    setTrang({ ...trang, dangDung: true, loiPhim: null });
    try {
      const phim = await layThuocPhim(contextId, outingId, me);
      setTrang((t) => (t.pha === "xong" ? { ...t, phim, dangDung: false } : t));
    } catch (error) {
      setTrang((t) => (t.pha === "xong" ? { ...t, dangDung: false, loiPhim: loiRaChu(error) } : t));
    }
  };

  if (trang.pha === "dang-doc") {
    return (
      <RudiScreen testID="trip-album-screen">
        <TopBar title="Album" />
        <SkeletonGroup style={styles.khung}>
          <SkeletonCard lines={1} media={240} />
          <SkeletonRow leading={0} />
        </SkeletonGroup>
      </RudiScreen>
    );
  }
  if (trang.pha === "hong") {
    return (
      <RudiScreen testID="trip-album-screen">
        <TopBar title="Album" />
        <ErrorState body={trang.loi} onRetry={() => { setTrang({ pha: "dang-doc" }); void layAlbum(contextId, outingId, me).then((album) => setTrang({ pha: "xong", album, phim: null, dangDung: false, loiPhim: null })).catch((error: unknown) => setTrang({ pha: "hong", loi: loiRaChu(error) })); }} title="Chưa mở được album" />
      </RudiScreen>
    );
  }
  const a = trang.album;
  const photos = a.photos.flatMap((photo) => {
    const source = nguonAnh(photo.image_url, me, contextId);
    return source === null ? [] : [{ id: photo.memory_id, source, caption: photo.caption || "Khoảnh khắc của nhóm" }];
  });
  return (
    <RudiScreen testID="trip-album-screen">
      {viewer ? <PhotoViewer photos={viewer.photos} initialIndex={viewer.index} title={viewer.title} onClose={() => setViewer(null)} /> : null}
      <TopBar title="Album" />
      <Heading title={a.title} subtitle={`${cauKhoang(a)} · ${cauThongKeAlbum(a)}`} />

      {photos.length === 0 ? (
        <EmptyState body="Chưa có ảnh nào trong những ngày của kèo. Thả khoảnh khắc lên tường nhóm là ảnh về đây." kind="first-use" layout="inline" title="Chưa có khoảnh khắc" />
      ) : (
        <>
          {/* The first photograph leads, at reading size; the rest are a grid. */}
          <Pressable accessibilityRole="button" accessibilityLabel={`Mở ảnh: ${photos[0].caption}`} onPress={() => setViewer({ photos, index: 0 })}>
            <KhungAnh chuThich={photos[0].caption} xuatXu={`${a.title} · ${cauKhoang(a)}`}>
              <Image accessibilityLabel={photos[0].caption} contentFit="cover" source={photos[0].source} cachePolicy="none" style={{ width: "100%", aspectRatio: tiLeDan, backgroundColor: colors.line }} />
            </KhungAnh>
          </Pressable>
          {photos.length > 1 ? <SectionHeader title={`${photos.length} khoảnh khắc`} /> : null}
          <ResponsiveRow gap={6} maxColumns={6} minItemWidth={104}>
            {photos.slice(1).map((photo, index) => <Pressable key={photo.id} accessibilityRole="button" accessibilityLabel={`Mở ảnh: ${photo.caption}`}
              onPress={() => setViewer({ photos, index: index + 1 })}>
              <Image accessibilityLabel={photo.caption} contentFit="cover" source={photo.source} cachePolicy="none" style={{ width: "100%", aspectRatio: 1, borderRadius: radius.small, backgroundColor: colors.line }} />
            </Pressable>)}
          </ResponsiveRow>
        </>
      )}

      <SectionHeader title="Chỗ đã tới" />
      {a.places.length === 0 ? <Text style={[typography.caption, { color: colors.inkFaint }]}>Chưa check-in ở đâu trong kèo này.</Text> : null}
      {a.places.map((p) => (
        <ListRow icon="location-outline" key={p.place_id} title={tenDiaDiem(p)} />
      ))}

      <SectionHeader title="Thước phim" />
      {trang.phim === null ? (
        <>
          <RudiButton disabled={trang.dangDung} icon="film-outline" label="Dựng thước phim" loading={trang.dangDung} onPress={() => void dungPhim()} tone="ai" variant="soft" />
          <Text style={[typography.caption, { color: colors.inkSoft }]}>Máy chủ chọn vài cảnh từ kỷ niệm của kèo; có mô hình thì mô hình dựng, không thì nói rõ vì sao chưa có.</Text>
        </>
      ) : (
        <>
          {trang.phim.source === "ai" ? <AiNote>{cauThuocPhim(trang.phim)}</AiNote> : <Text style={[typography.body, { color: colors.ink }]}>{cauThuocPhim(trang.phim)}</Text>}
          {!trang.phim.reeled && trang.phim.reason !== "no_memories" ? (
            // «lúc này» in the reason sentence is only true if the person can try again.
            <RudiButton disabled={trang.dangDung} icon="refresh-outline" label="Dựng lại" loading={trang.dangDung} onPress={() => void dungPhim()} tone="ai" variant="soft" />
          ) : null}
          {trang.phim.title !== null ? <Heading size="h2" title={trang.phim.title} /> : null}
          {trang.phim.reeled && trang.phim.picks.some((pick) => nguonAnh(pick.image_url, me, contextId) !== null) ? <RudiButton label="Xem câu chuyện ảnh" icon="play-outline" tone="ai" onPress={() => {
            if (trang.phim === null) return;
            const scenes = trang.phim.picks.flatMap((pick) => {
              const source = nguonAnh(pick.image_url, me, contextId);
              return source === null ? [] : [{ id: pick.memory_id, source, caption: pick.note }];
            });
            setViewer({ photos: scenes, index: 0, title: "Câu chuyện ảnh · AI chọn cảnh" });
          }} /> : null}
          {trang.phim.picks.map((canh) => {
            const nguon = nguonAnh(canh.image_url, me, contextId);
            return (
              <View key={canh.memory_id} style={[styles.canh, { borderBottomColor: colors.line }]}>
                {nguon !== null ? <Image contentFit="cover" source={nguon} style={[styles.anhCanh, { borderRadius: radius.small, backgroundColor: colors.line }]} /> : null}
                <Text style={[typography.body, { color: colors.ink }]}>{canh.note}</Text>
                {canh.place_name !== null ? <Text style={[typography.caption, { color: colors.inkFaint }]}>{canh.place_name}</Text> : null}
              </View>
            );
          })}
        </>
      )}
      {trang.loiPhim !== null ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{trang.loiPhim}</Text> : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  khung: { gap: 14 },
  chuThich: { marginTop: 6 },
  canh: { gap: 8, paddingVertical: 12, borderBottomWidth: StyleSheet.hairlineWidth },
  anhCanh: { width: "100%", aspectRatio: 4 / 3 },
});
