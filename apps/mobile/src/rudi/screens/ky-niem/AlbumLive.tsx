import { AssetIllustration } from "../../ui/AssetIllustration";
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
 *
 * 2026-09-08 (report 07/09 §9.18): the grid became days. After the lead, the
 * photographs are gathered by their Vietnam calendar day, laid in pairs, and
 * an odd last print goes wide, so the album has a rhythm instead of a wall of
 * equal squares. The dates and the group are written once, in the heading;
 * the print carries only its own caption and provenance.
 */
import { Image } from "expo-image";
import { useRouter } from "expo-router";
import { useEffect, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import type { Phien } from "../../../phien";
import { AlbumError } from "../../../screens/album/album-api";
import { docKeoCuaNhom } from "../../keo/keo";
import { homNay } from "../../keo/nhip-keo";
import { khoangNgayChuyen } from "../../ngay-viet";
import {
  cauKeAlbumRong,
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
import { AiNote, Heading, ListRow, RudiButton, RudiScreen, SectionHeader, TopBar } from "../../ui";
import { AlbumAnh } from "./AlbumAnh";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { PhotoViewer, type ViewerPhoto } from "../../ui/PhotoViewer";
import { SkeletonCard, SkeletonGroup, SkeletonRow } from "../../ui/Skeleton";
import { useAdaptiveLayout } from "../../ui/useAdaptiveLayout";
import { PaperObject } from "../../asset-pack/react-native/PaperObject";
import { PressScale } from "../../ui/PressScale";
import { RudiIcon } from "../../ui/RudiIcon";

function loiRaChu(error: unknown): string {
  if (error instanceof AlbumError) return error.message;
  if (error instanceof Error && error.message !== "") return error.message;
  return "Chưa mở được album. Kéo xuống để thử lại.";
}

/** The trip's days when known (QA UI-103), else the server's year label. */
function cauKhoang(a: { period_label: string; in_progress: boolean; headcount: number }, ngay = ""): string {
  return `${ngay || a.period_label}${a.in_progress ? " · đang đi" : ""} · ${a.headcount}\u00a0người`;
}

/**
 * Each outing's days, by id (QA UI-103). The album routes still have Python
 * as their parity oracle, so the days are not added to their wire (ADR-0031):
 * they come from the group's outings, the list «Lên plan» reads. A failed
 * read leaves the year label as it was; it is a caption, not the page.
 */
function useNgayChuyen(contextId: string, personId: string): Record<string, string> {
  const [ngay, setNgay] = useState<Record<string, string>>({});
  useEffect(() => {
    let song = true;
    docKeoCuaNhom(contextId, personId).then(
      (keo) => {
        if (song) setNgay(Object.fromEntries(keo.map((k) => [k.id, khoangNgayChuyen(k.starts_on, k.ends_on)])));
      },
      () => undefined,
    );
    return () => {
      song = false;
    };
  }, [contextId, personId]);
  return ngay;
}

/* ------------------------------------------------------------------ kệ */

type TrangKe = { pha: "dang-doc" } | { pha: "xong"; albums: TomTatAlbum[] } | { pha: "hong"; loi: string };

export function AlbumNhomLiveScreen({ phien, contextId }: { phien: Phien; contextId: string }) {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const [trang, setTrang] = useState<TrangKe>({ pha: "dang-doc" });
  // B4: an empty shelf asks the plan list whether an outing is merely ahead.
  const [keoNhom, setKeoNhom] = useState<{ title: string; starts_on: string }[]>([]);
  const cauRong = cauKeAlbumRong(keoNhom, homNay());
  const ngayChuyen = useNgayChuyen(contextId, phien.person_id);

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
        if (song && ds.albums.length === 0) {
          // Only a sentence depends on it: a failed read keeps «Chưa có kèo nào».
          void docKeoCuaNhom(contextId, phien.person_id).then((keo) => {
            if (song) setKeoNhom(keo);
          }, () => undefined);
        }
      })
      .catch((error: unknown) => {
        if (song) setTrang({ pha: "hong", loi: loiRaChu(error) });
      });
    return () => {
      song = false;
    };
  }, [contextId, phien.person_id]);

  return (
    <RudiScreen material="quiet" cot="doc" testID="trip-album-screen">
      <TopBar subtitle="Mỗi kèo một album" title="Album chuyến đi" />
      {trang.pha === "dang-doc" ? (
        <SkeletonGroup>
          <SkeletonRow />
          <SkeletonRow />
        </SkeletonGroup>
      ) : null}
      {trang.pha === "hong" ? <ErrorState body={trang.loi} onRetry={() => void doc()} title="Chưa đọc được album" /> : null}
      {trang.pha === "xong" && trang.albums.length === 0 ? (
        <EmptyState
          body={cauRong.than}
          illustration={<AssetIllustration name="spot-memory" width={240} height={204} />}
          kind="first-use"
          layout="inline"
          title={cauRong.tieuDe}
        />
      ) : null}
      {trang.pha === "xong"
        ? trang.albums.map((a) => (
            <PressScale
              accessibilityLabel={`Mở album ${a.title}`}
              accessibilityRole="button"
              key={a.outing_id}
              onPress={() => router.push(`/trips/${a.outing_id}/album?ctx=${contextId}` as never)}
              pressedScale={0.985}
              style={[styles.albumShelf, { borderColor: colors.line }, !a.cover && styles.emptyAlbumShelf]}
            >
              <AlbumCover album={a} contextId={contextId} personId={phien.person_id} />
              <View style={styles.albumWords}>
                <Text style={[typography.title, { color: colors.ink }]}>{a.title}</Text>
                <Text style={[typography.caption, { color: colors.inkSoft }]}>{cauKhoang(a, ngayChuyen[a.outing_id])}</Text>
                <Text style={[typography.caption, { color: colors.inkSoft }]}>{cauThongKeAlbum(a)}</Text>
              </View>
              <RudiIcon color={colors.inkSoft} name="chevron-forward" size={20} />
            </PressScale>
          ))
        : null}
    </RudiScreen>
  );
}

/** The shelf uses the server's own cover, or an honest empty photo frame. */
function AlbumCover({ album, contextId, personId }: { album: TomTatAlbum; contextId: string; personId: string }) {
  const { colors } = useRudiTheme();
  const [failed, setFailed] = useState(false);
  useEffect(() => setFailed(false), [album.cover?.image_url]);
  const source = album.cover ? nguonAnh(album.cover.image_url, personId, contextId) : null;
  if (!source || failed) return (
    <View style={[album.cover ? styles.unreadCover : styles.emptyCover, { backgroundColor: colors.paperShade, borderColor: colors.line }]}>
      <PaperObject name="photo-stack" width={64} />
      {failed ? <Text style={[typography.caption, { color: colors.inkSoft }]}>Ảnh chưa mở được.</Text> : null}
    </View>
  );
  return <Image accessibilityLabel={album.cover?.caption || `Ảnh bìa ${album.title}`} contentFit="cover" onError={() => setFailed(true)} source={source} style={styles.shelfCover} />;
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
  const ngayChuyen = useNgayChuyen(contextId, me);
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
      <RudiScreen material="quiet" cot="doc" testID="trip-album-screen">
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
      <RudiScreen material="quiet" cot="doc" testID="trip-album-screen">
        <TopBar title="Album" />
        <ErrorState body={trang.loi} onRetry={() => { setTrang({ pha: "dang-doc" }); void layAlbum(contextId, outingId, me).then((album) => setTrang({ pha: "xong", album, phim: null, dangDung: false, loiPhim: null })).catch((error: unknown) => setTrang({ pha: "hong", loi: loiRaChu(error) })); }} title="Chưa mở được album" />
      </RudiScreen>
    );
  }
  const a = trang.album;
  const photos = a.photos.flatMap((photo) => {
    const source = nguonAnh(photo.image_url, me, contextId);
    return source === null ? [] : [{ id: photo.memory_id, source, caption: photo.caption || "Khoảnh khắc của nhóm", created_at: photo.created_at }];
  });
  return (
    <RudiScreen material="quiet" cot="doc" testID="trip-album-screen">
      {viewer ? <PhotoViewer photos={viewer.photos} initialIndex={viewer.index} title={viewer.title} onClose={() => setViewer(null)} /> : null}
      <TopBar title="Album" />
      <Heading title={a.title} subtitle={`${cauKhoang(a, ngayChuyen[outingId])} · ${cauThongKeAlbum(a)}`} />

      {photos.length === 0 ? (
        <EmptyState
          body="Thả khoảnh khắc lên tường nhóm trong những ngày này là ảnh về đây."
          illustration={<AssetIllustration name="spot-memory" width={240} height={204} />}
          kind="first-use"
          layout="inline"
          title="Chưa có khoảnh khắc"
        />
      ) : (
        <AlbumAnh onMo={(index) => setViewer({ photos, index })} photos={photos} tiLeDan={tiLeDan} />
      )}

      <SectionHeader title="Chỗ đã tới" />
      {a.places.length === 0 ? <Text style={[typography.caption, { color: colors.inkFaint }]}>Chưa check-in ở đâu trong kèo này.</Text> : null}
      {a.places.map((p) => (
        <ListRow icon="location-outline" key={p.place_id} title={tenDiaDiem(p)} />
      ))}

      <SectionHeader title="Thước phim" />
      {trang.phim === null ? (
        <>
          {/* The button and its AI tone say what this is; the answer sentence
              (`cauThuocPhim`) says who composed it or why nothing was. The
              paragraph that explained the model here is gone (report §4.3). */}
          <RudiButton disabled={trang.dangDung} icon="film-outline" label="Dựng thước phim" loading={trang.dangDung} onPress={() => void dungPhim()} tone="ai" variant="soft" />
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
  albumShelf: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", gap: 16, paddingVertical: 20, borderBottomWidth: StyleSheet.hairlineWidth },
  emptyAlbumShelf: { flexWrap: "nowrap" },
  albumWords: { flex: 1, minWidth: 0, gap: 6 },
  shelfCover: { width: "100%", aspectRatio: 4 / 3, borderRadius: 3 },
  emptyCover: { width: 84, height: 104, borderWidth: StyleSheet.hairlineWidth, borderStyle: "dashed", alignItems: "center", justifyContent: "center", borderRadius: 3 },
  unreadCover: { width: "100%", aspectRatio: 4 / 3, borderWidth: StyleSheet.hairlineWidth, alignItems: "center", justifyContent: "center", gap: 8, borderRadius: 3 },
  khung: { gap: 14 },
  ngay: { gap: 6 },
  chuThich: { marginTop: 6 },
  canh: { gap: 8, paddingVertical: 12, borderBottomWidth: StyleSheet.hairlineWidth },
  anhCanh: { width: "100%", aspectRatio: 4 / 3 },
});
