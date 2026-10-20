/**
 * Chi tiết địa điểm on a real session (M4): everything the server knows about
 * one catalogue place, said in its own words. A match card only when the
 * model actually scored this place for the group, directions through the
 * phone's map app, and a save that lives on the server. «Thêm vào kèo» picks
 * one of the group's outings.
 *
 * ## The page reads top-down as a decision (UI v2, đợt 4)
 *
 * Picture or glyph → name → the two or three facts that decide (price, how
 * far, open or not) → the way there → the description; the rest (traits,
 * source, why it fits, reviews) follows in order of how often it changes a
 * mind. Nothing is boxed: the 2026-09-06 review counted the hero band, the
 * facts card, the AI card and the reviews card as four floors on one page.
 * The one primary action is «Thêm vào kèo», pinned; saving is its quieter
 * neighbour. There is no rating, price or distance the server did not send.
 */
import { Image } from "expo-image";
import { Ionicons } from "@expo/vector-icons";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { Linking, Modal, Platform, Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { ApiError, thongDiepNguoiDoc } from "../../../api";
import type { Phien } from "../../../phien";
import type { Highlights, PlaceDetail } from "../../../screens/kham-pha/chi-tiet-dia-diem";
import { matchLabel } from "../../../screens/kham-pha/places";
import {
  anhBiaThe,
  boLuuDiaDiem,
  CAU_ANH_BAI_DANG,
  CAU_ANH_NHOM,
  cauDuongDi,
  cauHoatDong,
  cauNguonDuLieu,
  chiTietNgan,
  daoLuu,
  docAnhDiaDiem,
  docAnhNhom,
  docChiTiet,
  docDaLuu,
  dongPhu,
  duongChiDuong,
  duongChiDuongWeb,
  luuDiaDiem,
  nguonAnhDiaDiem,
  cauNguonAnh,
  TIEN_TO_ANH,
  type AnhDiaDiem,
  type AnhNhom,
} from "../../kham-pha/dia-diem";
import { canhChoDiaDiem, vungCua } from "../../kham-pha/canh-ve";
import { cauGio, dongChinh, nhanKhiChat } from "../../kham-pha/chi-tiet-gon";
import { nguonAnh } from "../../ky-niem/ky-niem";
import { typography, useRudiTheme } from "../../theme";
import { AiNote, Chip, Divider, IconButton, Inline, RudiButton, RudiScreen, SectionHeader, TopBar } from "../../ui";
import { ErrorState } from "../../ui/ErrorState";
import { MediaSlot } from "../../ui/MediaSlot";
import { anhDanhMuc, khoaNguon, veKhung, type AnhCoGhiCong, type NguonKhung } from "../../ui/ghi-cong";
import { SkeletonCard, SkeletonGroup, SkeletonLines } from "../../ui/Skeleton";
import { Stamp } from "../../ui/Stamp";
import { PlaceArtwork } from "./HangDiaDiem";
import { StampButton } from "../../ui/StampButton";
import { laPair, tenCuocTroChuyen } from "../../nhan-rieng/nhan-rieng";
import { luiVeVe } from "../../lui-ve";

type Trang = { pha: "dang-doc" } | { pha: "xong"; place: PlaceDetail } | { pha: "hong"; loi: string };

/** A route param is a string or nothing; an array or undefined is nothing. */
function thamSoChuoi(v: unknown): string {
  if (typeof v === "string") return v;
  return "";
}

function loiRaChu(error: unknown): string {
  return error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null);
}

export function PlaceDetailLiveScreen({ phien }: { phien: Phien }) {
  const router = useRouter();
  const params = useLocalSearchParams<{ id?: string }>();
  // The pinned footer must clear the gesture bar (the shell pads top/left/right only).
  const { bottom: menDuoi } = useSafeAreaInsets();
  const placeId = thamSoChuoi(params.id);
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [daLuu, setDaLuu] = useState<string[]>([]);
  const [dangLuu, setDangLuu] = useState(false);
  const luuLock = useRef(false);
  const [thongBao, setThongBao] = useState<string | null>(null);
  // M12: the licensed gallery and the group's own photos, two more requests
  // after the facts. Either failing costs its pictures and nothing else.
  const [anh, setAnh] = useState<AnhDiaDiem[]>([]);
  const [loiAnh, setLoiAnh] = useState<string | null>(null);
  const [anhNhom, setAnhNhom] = useState<AnhNhom[]>([]);

  const nap = useCallback(async () => {
    if (!placeId) {
      setTrang({ pha: "hong", loi: "Thiếu địa điểm để mở." });
      return;
    }
    let place: PlaceDetail;
    try {
      const [p, luu] = await Promise.all([docChiTiet(placeId), docDaLuu(phien.person_id)]);
      place = p;
      setTrang({ pha: "xong", place });
      setDaLuu(luu);
    } catch (error) {
      setTrang({ pha: "hong", loi: loiRaChu(error) });
      return;
    }
    // Ảnh của nhóm: im lặng khi hỏng. Không có ảnh và không đọc được là cùng
    // một màn ở đây, và không có câu nào đúng hơn để nói.
    try {
      setAnhNhom(await docAnhNhom(placeId, phien.person_id));
    } catch {
      setAnhNhom([]);
    }
    // «Chưa tải được» is a different thing to go and check from a place that
    // simply has no photograph, so that one is said.
    if (!place.photosAvailable) {
      setAnh([]);
      setLoiAnh(null);
      return;
    }
    try {
      setAnh(await docAnhDiaDiem(placeId));
      setLoiAnh(null);
    } catch {
      setAnh([]);
      setLoiAnh("Chưa tải được ảnh của nơi này.");
    }
  }, [placeId, phien.person_id]);

  useEffect(() => {
    void nap();
  }, [nap]);

  const doiLuu = async (id: string) => {
    if (luuLock.current) return;
    luuLock.current = true;
    const truoc = daLuu;
    setDangLuu(true);
    setDaLuu(daoLuu(daLuu, id));
    setThongBao(null);
    try {
      if (truoc.includes(id)) await boLuuDiaDiem(phien.person_id, id);
      else await luuDiaDiem(phien.person_id, id);
    } catch (error) {
      setDaLuu(truoc);
      setThongBao(loiRaChu(error));
    } finally {
      luuLock.current = false;
      setDangLuu(false);
    }
  };

  const chiDuong = async (place: PlaceDetail) => {
    try {
      // A browser has no `geo:` handler: the web build opens the map address.
      await Linking.openURL(Platform.OS === "web" ? duongChiDuongWeb(place) : duongChiDuong(place));
    } catch {
      setThongBao("Máy này chưa có ứng dụng bản đồ để chỉ đường.");
    }
  };

  const daLuuChoNay = trang.pha === "xong" && daLuu.includes(trang.place.id);
  return (
    <RudiScreen
      footer={
        trang.pha === "xong" ? (
          // On a place's own page «Lưu» says enough; the pair is sized by its
          // labels and wraps when they cannot share a line. Split 1 : 1.4 it
          // cut «Lưu địa điểm» to «Lưu đ…» at 320 (QA UI-023).
          <Inline gap={10} wrap>
            <RudiButton
              accessibilityLabel={daLuuChoNay ? "Bỏ lưu địa điểm này" : "Lưu địa điểm này"}
              full={false}
              icon={daLuuChoNay ? "heart" : "heart-outline"}
              label={daLuuChoNay ? "Đã lưu" : "Lưu"}
              loading={dangLuu}
              onPress={() => void doiLuu(trang.place.id)}
              style={styles.nutLuu}
              variant={daLuuChoNay ? "soft" : "outline"}
            />
            <RudiButton
              full={false}
              icon="add-circle-outline"
              label="Thêm vào kèo"
              onPress={() => router.push(`/outings/chon?place=${encodeURIComponent(trang.place.id)}` as never)}
              style={styles.nutChinh}
            />
          </Inline>
        ) : null
      }
      bottomInset={110}
      footerInset={14 + menDuoi}
      testID="place-detail-screen"
    >
      <TopBar title="Địa điểm" />
      {trang.pha === "dang-doc" ? (
        <SkeletonGroup style={styles.khung}>
          <SkeletonCard lines={2} media={205} />
          <SkeletonLines lines={4} />
        </SkeletonGroup>
      ) : null}
      {trang.pha === "hong" ? (
        <ErrorState
          body={trang.loi}
          onRetry={() => void nap()}
          secondary={{ label: "Về Khám phá", onPress: () => luiVeVe(router as never, "/explore") }}
          title="Chưa mở được địa điểm"
        />
      ) : null}
      {trang.pha === "xong" ? <ThanChiTiet
        anh={anh}
        anhNhom={anhNhom}
        loiAnh={loiAnh}
        doi={(phien.contexts ?? []).filter((n) => laPair(n) && n.my_state === "active").slice(0, 3).map((n) => ({ id: n.id, ten: tenCuocTroChuyen(n) }))}
        onChiDuong={() => void chiDuong(trang.place)}
        onRu={(id) => router.push(`/groups/${id}/to-giay?ru=1&cho=${encodeURIComponent(trang.place.id)}` as never)}
        onThemAnhNhom={() =>
          router.push(
            `/moments/new?place=${encodeURIComponent(trang.place.id)}&ten=${encodeURIComponent(trang.place.name)}` as never,
          )
        }
        personId={phien.person_id}
        place={trang.place}
        thongBao={thongBao}
      /> : null}
    </RudiScreen>
  );
}

function ThanChiTiet({
  place,
  anh,
  anhNhom,
  loiAnh,
  personId,
  thongBao,
  onChiDuong,
  onThemAnhNhom,
  doi,
  onRu,
}: {
  place: PlaceDetail;
  anh: AnhDiaDiem[];
  anhNhom: AnhNhom[];
  loiAnh: string | null;
  personId: string;
  thongBao: string | null;
  onChiDuong: () => void;
  onThemAnhNhom: () => void;
  /** The person's two-person notebooks: «Rủ Minh tới đây» starts a sheet with this place on it. */
  doi: { id: string; ten: string }[];
  onRu: (contextId: string) => void;
}) {
  const { colors, space, radius } = useRudiTheme();
  const [moAnh, setMoAnh] = useState<number | null>(null);
  const hl = place.highlights;
  const hop = matchLabel(place.match);
  const coMatch = place.match !== null && place.match.source === "ai";
  // The price is said once, in the key facts below the reason (finish review
  // 21/10); this row keeps the rating and the distance.
  const facts = chiTietNgan(place).filter((muc) => muc.icon === "star" || muc.icon === "navigate-outline");
  // One cover that pages through every picture (finish review 21/10): the
  // cover first, then the gallery, each page carrying exactly its own credit
  // under it (ADR-0017 §2.5). A second full-width strip under the cover put
  // the name below the fold.
  const bia = anhBiaThe(place);
  const trangAnh = useMemo<ViewerPhoto[]>(() => {
    const ra: ViewerPhoto[] = bia ? [{ id: "cover", alt: place.name, nguon: { loai: "danh-muc", anh: bia } }] : [];
    for (const a of anh) {
      if (bia !== null && nguonAnhDiaDiem(a).uri === place.photoUrl) continue;
      ra.push({ id: a.id, alt: a.title ?? (a.license !== null ? "Ảnh có giấy phép chụp quanh đây" : "Ảnh chụp quanh đây"), nguon: { loai: "danh-muc", anh: anhGallery(a) } });
    }
    return ra;
  }, [anh, bia, place.name, place.photoUrl]);
  const cauAnh = cauNguonAnh(anh);
  const viec = cauHoatDong(place.activities);
  const chinh = dongChinh(place, hl, new Date());
  const motaRieng = hl?.taiSaoDangDen != null && place.description !== null && place.description !== hl.taiSaoDangDen ? place.description : null;
  // The drawn stand-in may only paint this place's own region (canh-ve.ts).
  const canh = canhChoDiaDiem({ id: place.id, loai: place.category, vung: vungCua({ diaChi: place.address }) });
  const dau = hop !== null && hop.real ? <View style={styles.badgeOnMedia}><Stamp label={hop.text} nen tilt={-2} tone="ai" /></View> : null;
  return (
    <>
      {trangAnh.length === 0 ? (
        // A place without an honest picture gets the drawing of its kind, in
        // the same frame a photograph would take, so the name stays put.
        <View style={[styles.khungBia, { borderRadius: radius.base }]} testID="place-cover">
          <PlaceArtwork canh={canh} fillFrame loai={place.category} />
          {dau}
        </View>
      ) : (
        <BiaTrang anh={trangAnh} dau={dau} fallback={<PlaceArtwork canh={canh} fillFrame loai={place.category} />} onMo={setMoAnh} />
      )}
      <PhotoViewer photos={trangAnh} index={moAnh} onIndex={setMoAnh} onClose={() => setMoAnh(null)} />
      {loiAnh !== null ? <Text style={[typography.caption, { color: colors.warn }]}>{loiAnh}</Text> : null}
      <View style={styles.dau}>
        <Text style={[typography.h1, { color: colors.ink }]}>{place.name}</Text>
        {dongPhu(place) ? <Text style={[typography.body, { color: colors.inkSoft }]}>{dongPhu(place)}</Text> : null}
        {/* Only what the row actually carries (M9). Each item is one text node
            and the status is a whole badge: at font 1.3 an inline word broke
            across lines («Đang / mở»). */}
        <Inline gap={12} wrap>
          {facts.map((muc) => (
            <Inline gap={6} key={muc.icon}>
              <Ionicons color={muc.icon === "star" ? colors.accent : colors.inkFaint} name={muc.icon} size={15} />
              <Text style={[typography.label, { color: colors.ink }]}>{muc.chu}</Text>
            </Inline>
          ))}
          {place.openNow === null ? null : (
            <Chip label={place.openNow ? "Đang mở" : "Đã đóng"} selected tone={place.openNow ? "accent" : "split"} />
          )}
        </Inline>
      </View>
      <LoiMo hl={hl} mota={place.description} />
      {chinh.length > 0 ? (
        // The ten-second read: the facts that decide, one label and one value a line.
        <View style={[styles.dongChinh, { gap: space.xs }]} testID="place-key-facts">
          {chinh.map((d) => (
            <View key={d.id} style={styles.hangChinh}>
              <Text style={[typography.label, styles.nhanChinh, { color: colors.inkSoft }]}>{d.nhan}</Text>
              <View style={styles.flex}>
                <Text style={[typography.body, { color: colors.ink }]}>{d.gia}</Text>
                {d.phu ? <Text style={[typography.note, { color: colors.inkSoft }]}>{d.phu}</Text> : null}
              </View>
            </View>
          ))}
        </View>
      ) : null}
      <Chips place={place} hl={hl} />
      {/* The way there: the address takes the whole width, directions follow under it. */}
      <View style={[styles.duongDi, { backgroundColor: colors.card, borderColor: colors.line, borderRadius: radius.small }]}>
        <Pressable accessibilityLabel="Mở địa chỉ trên bản đồ" accessibilityRole="button" onPress={onChiDuong} style={({ pressed }) => [styles.diaChi, pressed && styles.bam]}>
          <Ionicons color={colors.accent} name="location-outline" size={20} style={styles.ghimDiaChi} />
          <View style={styles.flex}>
            <Text style={[typography.label, { color: colors.ink }]}>{place.address ?? "Chưa có địa chỉ"}</Text>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>{cauDuongDi(place)}</Text>
          </View>
        </Pressable>
        <RudiButton compact full={false} icon="navigate-outline" label="Chỉ đường" onPress={onChiDuong} style={styles.nutChiDuong} variant="outline" />
      </View>
      {/* From a place to an invitation in one step: the sheet of the two of
          them opens with this place as the main stop (QA 23/09: a place page
          had no way to become an evening for a couple). */}
      {doi.map((d) => (
        // One notebook: its invitation is the stamp. Several: three coral
        // stamps shouted over each other on the live capture, so they are lines.
        doi.length === 1 ? (
          <StampButton key={d.id} label={`Rủ ${d.ten} tới đây`} onPress={() => onRu(d.id)} size="vua" tilt={-1} />
        ) : (
          <RudiButton icon="mail-outline" key={d.id} label={`Rủ ${d.ten} tới đây`} onPress={() => onRu(d.id)} variant="ghost" />
        )
      ))}
      {/* The longer description, when the reason above is a different sentence: its own section, never hidden behind a «Đọc thêm» that reveals another text. */}
      {motaRieng === null ? null : (
        <View style={styles.khoi}>
          <SectionHeader title="Giới thiệu" />
          <Text style={[typography.body, { color: colors.ink }]}>{motaRieng}</Text>
        </View>
      )}
      <ChiMuc hl={hl ?? KHONG_CO_CHI_MUC} openHours={place.openHours} space={space} colors={colors} radius={radius} />
      {cauNguonDuLieu(place) === null ? null : (
        <Text style={[typography.caption, { color: colors.inkFaint }]}>{cauNguonDuLieu(place)}</Text>
      )}
      {/* Where the pictures came from, said once for the whole gallery and away
          from the cover: each page above already carries its own credit line. */}
      {cauAnh === null || trangAnh.length === 0 ? null : (
        <Text style={[typography.caption, { color: colors.inkFaint }]}>{cauAnh}</Text>
      )}
      {/* «Nên làm gì ở đây» (M12b): sentences written when the place was
          imported, out of its own tags; nothing a model wrote on open. */}
      {viec === null ? null : (
        <View style={styles.khoi}>
          <SectionHeader title={viec} />
          <Inline gap={8} wrap>
            {place.activities.map((v) => (
              <Chip key={v} label={v} />
            ))}
          </Inline>
        </View>
      )}
      {coMatch && place.match !== null ? (
        <View style={styles.khoi}>
          <SectionHeader title="Vì sao hợp nhóm?" />
          <AiNote>{place.match.reason}</AiNote>
          {place.match.factors.map((f) => (
            <Text key={f.label} style={[typography.caption, { color: colors.inkSoft }]}>
              {f.label}: {f.detail}
            </Text>
          ))}
        </View>
      ) : null}
      {place.reviews.length > 0 ? (
        <View style={styles.khoi}>
          <SectionHeader title={`${place.reviews.length} nhận xét`} />
          {place.reviews.map((r, i) => (
            <View key={`${r.author}-${i}`}>
              {i > 0 ? <Divider /> : null}
              <View style={styles.nhanXet}>
                <Text style={[typography.label, { color: colors.ink }]}>
                  {r.author} · {r.rating.toFixed(1)}/5
                </Text>
                <Text style={[typography.body, { color: colors.inkSoft }]}>{r.body}</Text>
              </View>
            </View>
          ))}
        </View>
      ) : null}
      {anhNhom.length === 0 ? null : <DaiAnhNhom anh={anhNhom} personId={personId} />}
      <RudiButton
        icon="camera-outline"
        label={anhNhom.length === 0 ? "Thêm ảnh của nhóm ở đây" : "Thêm một ảnh nữa"}
        onPress={onThemAnhNhom}
        variant="outline"
      />
      {thongBao !== null ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>{thongBao}</Text> : null}
    </>
  );
}

/** Nothing to index: the sections still fall back to the one hours line the row itself carries. */
const KHONG_CO_CHI_MUC: Highlights = {
  taiSaoDangDen: null, khongKhi: null, khungGioDep: null, mucGia: null, gioGhiChu: null,
  phuHopVoi: [], khauViPhuHop: [], luuY: [], traiNghiem: [], monChinh: [], monPhaiThu: [],
  gia: null, gioMoCua: [], canDatTruoc: null, trongNgoai: null, thoiLuong: null, danhMuc: [], khiChat: [],
};

/** A gallery photograph with the credit it may be shown under: author and licence, or where the post came from. */
function anhGallery(a: AnhDiaDiem): AnhCoGhiCong {
  if (a.author === null && a.license === null) return anhDanhMuc(nguonAnhDiaDiem(a), { author: null, license: null, source: CAU_ANH_BAI_DANG });
  return anhDanhMuc(nguonAnhDiaDiem(a), { author: a.author, license: a.license, prefix: TIEN_TO_ANH });
}

/** The cover's frame: 16:9 keeps the picture under half of an 844 viewport with its credit. */
const TI_LE_BIA = 16 / 9;

/**
 * One cover that pages: each page is a full-width MediaSlot with its own one
 * credit line under it, so the visible photograph and its credit move
 * together and only one credit is ever on screen. A small «2/5» on the
 * picture says there are more.
 */
function BiaTrang({ anh, dau, fallback, onMo }: { anh: ViewerPhoto[]; dau: ReactNode; fallback: ReactNode; onMo: (index: number) => void }) {
  const { colors, radius } = useRudiTheme();
  const [rong, setRong] = useState(0);
  return (
    <View onLayout={(e) => setRong(Math.round(e.nativeEvent.layout.width))} testID="place-cover">
      <ScrollView horizontal pagingEnabled scrollEnabled={anh.length > 1} showsHorizontalScrollIndicator={false} testID="place-photos">
        {anh.map((a, i) => (
          <Pressable accessibilityLabel={`Xem ${a.alt} toàn màn hình`} accessibilityRole="button" key={a.id} onPress={() => onMo(i)} style={{ width: rong }}>
            <MediaSlot
              alt={a.alt}
              fallback={fallback}
              nguon={a.nguon}
              overlay={
                <>
                  {i === 0 ? dau : null}
                  {anh.length > 1 ? (
                    <View style={[styles.demAnh, { backgroundColor: colors.card, borderColor: colors.line, borderRadius: radius.small }]}>
                      <Text accessibilityLabel={`Ảnh ${i + 1} trên ${anh.length}`} style={[typography.caption, { color: colors.ink }]}>{`${i + 1}/${anh.length}`}</Text>
                    </View>
                  ) : null}
                </>
              }
              radius={radius.base}
              ratio={TI_LE_BIA}
            />
          </Pressable>
        ))}
      </ScrollView>
    </View>
  );
}

/** «Tại sao đáng đến»: the reason in three lines; «Đọc thêm» only when those three lines actually cut it. */
function LoiMo({ hl, mota }: { hl: Highlights | null; mota: string | null }) {
  const { colors, space } = useRudiTheme();
  const [mo, setMo] = useState(false);
  // The full paragraph is laid out once, invisibly, beside the clamped one; a
  // taller full text is the only proof the clamp hid something. A length
  // guess printed «Đọc thêm» under a complete three-line paragraph.
  const [cao, setCao] = useState<{ du: number; gon: number }>({ du: 0, gon: 0 });
  const dau = hl?.taiSaoDangDen ?? mota;
  if (dau === null || dau === undefined) return null;
  const cat = cao.du > cao.gon + 1;
  return (
    <View style={{ gap: space.xs }}>
      {hl?.taiSaoDangDen != null ? <SectionHeader title="Tại sao đáng đến" /> : null}
      <View>
        <Text numberOfLines={mo ? undefined : 3} onLayout={(e) => { const h = e.nativeEvent.layout.height; setCao((c) => (c.gon === h ? c : { ...c, gon: h })); }} style={[typography.body, { color: colors.ink }]}>{dau}</Text>
        <Text
          accessibilityElementsHidden
          aria-hidden
          importantForAccessibility="no-hide-descendants"
          onLayout={(e) => { const h = e.nativeEvent.layout.height; setCao((c) => (c.du === h ? c : { ...c, du: h })); }}
          pointerEvents="none"
          style={[typography.body, styles.doDai, { color: colors.ink }]}
        >
          {dau}
        </Text>
      </View>
      {cat || mo ? (
        <RudiButton compact full={false} label={mo ? "Thu gọn" : "Đọc thêm"} onPress={() => setMo((v) => !v)} variant="ghost" />
      ) : null}
    </View>
  );
}

/** One quick row of what kind of place this is: category, mood, indoors or out, traits. Each once, each only when sent. */
function Chips({ place, hl }: { place: PlaceDetail; hl: Highlights | null }) {
  const nhan = nhanKhiChat(place, hl);
  if (nhan.length === 0) return null;
  return (
    <Inline gap={8} wrap>
      {nhan.map((t) => (
        <Chip key={t} label={t} />
      ))}
    </Inline>
  );
}

/** The index sections, each only when the server sent something for it. */
function ChiMuc({ hl, openHours, space, colors, radius }: { hl: Highlights; openHours: string | null; space: { xs: number; sm: number; md: number }; colors: ReturnType<typeof useRudiTheme>["colors"]; radius: ReturnType<typeof useRudiTheme>["radius"] }) {
  const muc = (tieuDe: string, noiDung: ReactNode) => (
    <View key={tieuDe} style={{ gap: space.xs }}>
      <SectionHeader title={tieuDe} />
      {noiDung}
    </View>
  );
  const dong = (chu: string, i: number) => (
    <View key={`${i}-${chu}`} style={styles.hangBullet}>
      <Ionicons color={colors.inkFaint} importantForAccessibility="no" name="ellipse" size={6} style={styles.chamBullet} />
      <Text style={[typography.body, styles.flex, { color: colors.ink }]}>{chu}</Text>
    </View>
  );
  const ra: ReactNode[] = [];
  if (hl.monPhaiThu.length > 0) {
    ra.push(muc("Món nên thử", hl.monPhaiThu.map((m) => (
      <View key={m.ten} style={[styles.theMon, { backgroundColor: colors.card, borderColor: colors.line, borderRadius: radius.small, padding: space.sm, gap: 2 }]}>
        <Text style={[typography.label, { color: colors.ink }]}>{m.ten}</Text>
        {[m.moTa, m.huongVi, m.cachAn, m.gia].filter((x): x is string => x !== null).map((x) => (
          <Text key={x} style={[typography.note, { color: colors.inkSoft }]}>{x}</Text>
        ))}
      </View>
    ))));
  } else if (hl.monChinh.length > 0) {
    ra.push(muc("Món nên thử", <Inline gap={8} wrap>{hl.monChinh.map((m) => <Chip key={m} label={m} />)}</Inline>));
  }
  if (hl.khongKhi !== null) ra.push(muc("Không khí", <Text style={[typography.body, { color: colors.ink }]}>{hl.khongKhi}</Text>));
  const hop = [...hl.phuHopVoi, ...hl.khauViPhuHop];
  if (hop.length > 0) ra.push(muc("Hợp với ai", <View style={{ gap: space.xs }}>{hop.map(dong)}</View>));
  // The price is a key fact under the name (dongChinh), never a second section here.
  // The week stays here; today and the best hour are key facts under the name.
  if (hl.gioMoCua.length > 0 || hl.gioGhiChu !== null || openHours !== null) {
    ra.push(muc("Giờ", (
      <>
        {hl.gioMoCua.length > 0
          ? hl.gioMoCua.map((g) => (
              <View key={g.thu} style={styles.hangGio}>
                <Text style={[typography.label, styles.nhanGio, { color: colors.inkSoft }]}>{g.nhan}</Text>
                <Text style={[typography.body, styles.flex, { color: colors.ink }]}>{cauGio(g)}</Text>
              </View>
            ))
          : openHours !== null ? <Text style={[typography.body, { color: colors.ink }]}>{`Giờ mở cửa: ${openHours}`}</Text> : null}
        {hl.gioGhiChu !== null ? <Text style={[typography.note, { color: colors.inkSoft }]}>{hl.gioGhiChu}</Text> : null}
      </>
    )));
  }
  if (hl.traiNghiem.length > 0) ra.push(muc("Trải nghiệm nên thử", <View style={{ gap: space.xs }}>{hl.traiNghiem.map(dong)}</View>));
  if (hl.luuY.length > 0) ra.push(muc("Lưu ý", <View style={{ gap: space.xs }}>{hl.luuY.map(dong)}</View>));
  return <>{ra}</>;
}

/**
 * One picture of the cover's pages or the viewer: a licensed photograph is
 * only allowed on this screen while its author and licence are legible next
 * to it, so each page is a full-width frame with its credit under it, never a
 * thumbnail whose caption would not fit.
 */
type ViewerPhoto = { id: string; alt: string; nguon: Exclude<NguonKhung, null> };

/** Full-screen media keeps attribution and a keyboard/screen-reader alternative to swiping. */
function PhotoViewer({ photos, index, onIndex, onClose }: { photos: ViewerPhoto[]; index: number | null; onIndex: (index: number) => void; onClose: () => void }) {
  const { colors } = useRudiTheme();
  const insets = useSafeAreaInsets();
  const [hong, setHong] = useState(false);
  const [retry, setRetry] = useState(0);
  const [loading, setLoading] = useState(true);
  const touchX = useRef<number | null>(null);
  const photo = index === null ? null : photos[index] ?? null;
  const key = khoaNguon(photo?.nguon ?? null);
  useEffect(() => { setHong(false); setLoading(true); setRetry(0); }, [key]);
  const ve = veKhung(photo?.nguon ?? null, { hong });
  return <Modal visible={photo !== null} onRequestClose={onClose} presentationStyle="fullScreen" animationType="none">
    <View accessibilityViewIsModal style={{ flex: 1, paddingTop: insets.top, paddingBottom: Math.max(insets.bottom, 16), backgroundColor: colors.ground }} testID="place-photo-viewer">
      <View style={{ flexDirection: "row", alignItems: "center", gap: 12, paddingHorizontal: 16 }}><Text style={[typography.title, { color: colors.ink, flex: 1 }]}>{photo?.alt}</Text><IconButton accessibilityLabel="Đóng ảnh toàn màn hình" icon="close" onPress={onClose} /></View>
      <View style={{ flex: 1, marginVertical: 12 }} onTouchStart={(event) => { touchX.current = event.nativeEvent.pageX; }} onTouchEnd={(event) => { if (index !== null && touchX.current !== null) { const distance = event.nativeEvent.pageX - touchX.current; if (Math.abs(distance) > 48) onIndex(Math.max(0, Math.min(photos.length - 1, index + (distance < 0 ? 1 : -1)))); } touchX.current = null; }}>
        {ve.source ? <Image key={`${key}:${retry}`} accessibilityLabel={photo?.alt} contentFit="contain" source={ve.source} style={{ width: "100%", height: "100%" }} onLoad={() => setLoading(false)} onError={() => { setHong(true); setLoading(false); }} /> : <View style={{ flex: 1, alignItems: "center", justifyContent: "center", gap: 12, padding: 16 }}><Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{ve.canhBao ?? "Không có ảnh để mở."}</Text>{photo ? <RudiButton compact label="Tải lại ảnh" variant="outline" onPress={() => { setHong(false); setLoading(true); setRetry((n) => n + 1); }} /> : null}</View>}
      </View>
      {loading && ve.source ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.inkSoft, paddingHorizontal: 16 }]}>Đang tải ảnh…</Text> : null}
      {ve.ghiCong ? <Text style={[typography.caption, { color: colors.inkSoft, paddingHorizontal: 16 }]}>{ve.ghiCong}</Text> : null}
      {photos.length > 1 && index !== null ? <View style={{ flexDirection: "row", alignItems: "center", justifyContent: "space-between", gap: 12, paddingHorizontal: 16 }}><IconButton accessibilityLabel="Ảnh trước" icon="chevron-back" disabled={index === 0} onPress={() => onIndex(index - 1)} /><Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.inkSoft }]}>Ảnh {index + 1} trên {photos.length}</Text><IconButton accessibilityLabel="Ảnh tiếp theo" icon="chevron-forward" disabled={index === photos.length - 1} onPress={() => onIndex(index + 1)} /></View> : null}
    </View>
  </Modal>;
}

/** Photographs a group of the reader's took here (M12 §2.4): private, so no credit line, and only members see them. */
function DaiAnhNhom({ anh, personId }: { anh: AnhNhom[]; personId: string }) {
  const { colors, radius } = useRudiTheme();
  const [index, setIndex] = useState<number | null>(null);
  const co = useMemo<ViewerPhoto[]>(() => anh.flatMap((a) => { const source = nguonAnh(a.imageUrl, personId, a.contextId); return source ? [{ id: a.id, alt: a.caption ?? "Ảnh của nhóm bạn ở đây", nguon: { loai: "nhom" as const, source } }] : []; }), [anh, personId]);
  return (
    <View style={styles.khoi}>
      <SectionHeader title="Ảnh của nhóm bạn" />
      {/* Who may see these is said before the pictures, not after: the reader
          meets the rule with the heading, and at font 1.3 a sentence under a
          strip of photographs sat below the fold (board 2026-09-07). */}
      <Text style={[typography.caption, { color: colors.inkFaint }]}>{CAU_ANH_NHOM}</Text>
      <ScrollView contentContainerStyle={styles.dai} horizontal showsHorizontalScrollIndicator={false} testID="place-group-photos">
        {co.map((a, i) => <Pressable key={a.id} accessibilityRole="button" accessibilityLabel={`Xem ${a.alt} toàn màn hình`} onPress={() => setIndex(i)} style={styles.oAnh}><MediaSlot alt={a.alt} nguon={a.nguon} radius={radius.base} width="100%" /><Text style={[typography.caption, { color: colors.inkSoft }]}>{a.alt}</Text></Pressable>)}
      </ScrollView>
      <PhotoViewer photos={co} index={index} onIndex={setIndex} onClose={() => setIndex(null)} />
    </View>
  );
}

const styles = StyleSheet.create({
  khungBia: { width: "100%", aspectRatio: TI_LE_BIA, overflow: "hidden" },
  demAnh: { position: "absolute", right: 10, top: 10, paddingHorizontal: 8, paddingVertical: 2, borderWidth: StyleSheet.hairlineWidth },
  doDai: { position: "absolute", left: 0, right: 0, top: 0, opacity: 0 },
  dongChinh: {},
  hangChinh: { flexDirection: "row", alignItems: "baseline", gap: 12 },
  nhanChinh: { width: 96 },
  dai: { gap: 12, paddingRight: 8 },
  oAnh: { width: 280, gap: 6 },
  flex: { flex: 1, minWidth: 0 },
  nutLuu: { flexGrow: 1, flexShrink: 0, maxWidth: "100%" },
  nutChinh: { flexGrow: 2, flexShrink: 0, maxWidth: "100%" },
  khung: { gap: 16 },
  badgeOnMedia: { position: "absolute", left: 12, top: 12 },
  dau: { gap: 8 },
  duongDi: { gap: 8, paddingHorizontal: 14, paddingVertical: 12, borderWidth: 1 },
  diaChi: { flexDirection: "row", alignItems: "flex-start", gap: 10, minHeight: 48 },
  ghimDiaChi: { marginTop: 1 },
  nutChiDuong: { alignSelf: "flex-start", marginLeft: 30 },
  bam: { opacity: 0.7 },
  khoi: { gap: 8 },
  hangBullet: { flexDirection: "row", alignItems: "flex-start", gap: 10 },
  chamBullet: { marginTop: 9 },
  hangGio: { flexDirection: "row", alignItems: "baseline", gap: 12 },
  nhanGio: { width: 84 },
  theMon: { borderWidth: StyleSheet.hairlineWidth },
  nhanXet: { gap: 2, paddingVertical: 8 },
});
