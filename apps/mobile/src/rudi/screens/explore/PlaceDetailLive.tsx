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
import { Ionicons } from "@expo/vector-icons";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Linking, Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { ApiError, thongDiepNguoiDoc } from "../../../api";
import type { Phien } from "../../../phien";
import type { PlaceDetail } from "../../../screens/kham-pha/chi-tiet-dia-diem";
import { matchLabel } from "../../../screens/kham-pha/places";
import {
  anhBiaThe,
  bieuTuongLoai,
  boLuuDiaDiem,
  CAU_ANH_NHOM,
  cauDuongDi,
  cauGia,
  cauHoatDong,
  cauMoCua,
  cauNguonDuLieu,
  chiTietNgan,
  daoLuu,
  docAnhDiaDiem,
  docAnhNhom,
  docChiTiet,
  docDaLuu,
  dongPhu,
  duongChiDuong,
  luuDiaDiem,
  nguonAnhDiaDiem,
  CAU_NGUON_ANH,
  TIEN_TO_ANH,
  type AnhDiaDiem,
  type AnhNhom,
} from "../../kham-pha/dia-diem";
import { nguonAnh } from "../../ky-niem/ky-niem";
import { typography, useRudiTheme } from "../../theme";
import { AiNote, Chip, Divider, Inline, RudiButton, RudiScreen, SectionHeader, TopBar } from "../../ui";
import { ErrorState } from "../../ui/ErrorState";
import { MediaSlot } from "../../ui/MediaSlot";
import { anhDanhMuc } from "../../ui/ghi-cong";
import { SkeletonCard, SkeletonGroup, SkeletonLines } from "../../ui/Skeleton";
import { Stamp } from "../../ui/Stamp";
import { PlaceGlyph } from "./HangDiaDiem";
import { KyHoa } from "../../ui/art/KyHoa";
import { SanKhau } from "../../ui/SanKhau";
import { StampButton } from "../../ui/StampButton";
import { sanKhauKyHoa } from "../../art/san-khau";
import { laPair, tenCuocTroChuyen } from "../../nhan-rieng/nhan-rieng";

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
      setDangLuu(false);
    }
  };

  const chiDuong = async (place: PlaceDetail) => {
    try {
      await Linking.openURL(duongChiDuong(place));
    } catch {
      setThongBao("Máy này chưa có ứng dụng bản đồ để chỉ đường.");
    }
  };

  const daLuuChoNay = trang.pha === "xong" && daLuu.includes(trang.place.id);
  return (
    <RudiScreen
      footer={
        trang.pha === "xong" ? (
          <View style={styles.hanhDong}>
            <View style={styles.flex}>
              <RudiButton
                icon={daLuuChoNay ? "heart" : "heart-outline"}
                label={daLuuChoNay ? "Đã lưu" : "Lưu địa điểm"}
                loading={dangLuu}
                onPress={() => void doiLuu(trang.place.id)}
                variant={daLuuChoNay ? "soft" : "outline"}
              />
            </View>
            <View style={styles.chinh}>
              <RudiButton
                icon="add-circle-outline"
                label="Thêm vào kèo"
                onPress={() => router.push(`/outings/chon?place=${encodeURIComponent(trang.place.id)}` as never)}
              />
            </View>
          </View>
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
          secondary={{ label: "Về Khám phá", onPress: () => router.back() }}
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
  const { colors } = useRudiTheme();
  const [rongSan, setRongSan] = useState(0);
  const hop = matchLabel(place.match);
  const coMatch = place.match !== null && place.match.source === "ai";
  const facts = chiTietNgan(place).filter((muc) => muc.icon !== "time-outline" && muc.icon !== "location-outline");
  // The cover is drawn only with its credit (ADR-0017 §2.5); the gallery
  // strip carries the other licensed photographs, each with its own line.
  const bia = anhBiaThe(place);
  const conLai = bia === null ? anh : anh.filter((a) => nguonAnhDiaDiem(a).uri !== place.photoUrl);
  const viec = cauHoatDong(place.activities);
  return (
    <>
      {/* A place without an honest picture gets no 16:10 frame of nothing (review
          08/09 F01; re-audit 10/09: «một icon bát nhỏ và khoảng trống»): the
          category's paper slot and the seal stand on one row, and the name
          follows at once. With a picture, the frame and the seal on it as before. */}
      {bia === null ? (
        <View style={styles.dauGon}>
          {/* The sketch of the kind of place (category only: live carries no
              tags), then the seal on its own row (review 11/09 A1). */}
          {/* The same sketch, lifted into its three depths as a pop-up stage
              that stands up once (ADR-0037 D1, plan S4). */}
          <View onLayout={(e) => setRongSan(Math.round(e.nativeEvent.layout.width))} style={styles.sanCho}>
            {rongSan > 0 ? <SanKhau coMoTa san={sanKhauKyHoa(place.category, [])} width={Math.min(rongSan, 520)} /> : <KyHoa loai={place.category} tags={[]} />}
          </View>
          {hop !== null && hop.real ? <Stamp label={hop.text} style={styles.dauGonDau} tone="ai" /> : null}
        </View>
      ) : (
        <MediaSlot
          alt={place.name}
          fallback={<PlaceGlyph glyph={bieuTuongLoai(place.category)} loai={place.category} size={56} />}
          nguon={{ loai: "danh-muc", anh: bia }}
          overlay={hop !== null && hop.real ? <View style={styles.badgeOnMedia}><Stamp label={hop.text} nen tilt={-2} tone="ai" /></View> : null}
          ratio={16 / 10}
        />
      )}
      {loiAnh !== null ? <Text style={[typography.caption, { color: colors.warn }]}>{loiAnh}</Text> : null}
      {conLai.length > 0 ? <DaiAnh anh={conLai} /> : null}
      {/* The sentence the pictures are shown on: found by geosearch around the
          venue, licensed, not supplied by the place. Said once under the
          photographs whenever there is at least one (M12, ADR-0017 §2.5). */}
      {bia !== null || conLai.length > 0 ? (
        <Text style={[typography.caption, { color: colors.inkSoft }]}>{CAU_NGUON_ANH}</Text>
      ) : null}
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
      {/* The way there: the address is the row, directions the action beside it. */}
      <View style={[styles.duongDi, { borderTopColor: colors.line, borderBottomColor: colors.line }]}>
        <Pressable accessibilityLabel="Mở địa chỉ trên bản đồ" accessibilityRole="button" onPress={onChiDuong} style={({ pressed }) => [styles.diaChi, pressed && styles.bam]}>
          <Ionicons color={colors.inkFaint} name="location-outline" size={20} />
          <View style={styles.flex}>
            <Text style={[typography.label, { color: colors.ink }]}>{place.address ?? "Chưa có địa chỉ"}</Text>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>{cauDuongDi(place)}</Text>
          </View>
        </Pressable>
        <RudiButton compact full={false} icon="navigate-outline" label="Chỉ đường" onPress={onChiDuong} variant="outline" />
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
          <RudiButton icon="mail-outline" key={d.id} label={`Rủ ${d.ten} tới đây`} onPress={() => onRu(d.id)} variant="outline" />
        )
      ))}
      {place.description ? <Text style={[typography.body, { color: colors.ink }]}>{place.description}</Text> : null}
      <View style={styles.suKien}>
        <View style={styles.hangSuKien}>
          <Ionicons color={colors.inkFaint} name="time-outline" size={18} />
          <Text style={[typography.body, styles.flex, { color: colors.ink }]}>{cauMoCua(place)}</Text>
        </View>
        <View style={styles.hangSuKien}>
          <Ionicons color={colors.inkFaint} name="wallet-outline" size={18} />
          <Text style={[typography.body, styles.flex, { color: colors.ink }]}>{cauGia(place)}</Text>
        </View>
      </View>
      {place.traits.length > 0 ? (
        <Inline gap={8} wrap>
          {place.traits.map((t) => (
            <Chip key={t} label={t} />
          ))}
        </Inline>
      ) : null}
      {cauNguonDuLieu(place) === null ? null : (
        <Text style={[typography.caption, { color: colors.inkFaint }]}>{cauNguonDuLieu(place)}</Text>
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
      <View style={styles.khoi}>
        <SectionHeader title="Vì sao hợp nhóm?" />
        {coMatch && place.match !== null ? (
          <>
            <AiNote>{place.match.reason}</AiNote>
            {place.match.factors.map((f) => (
              <Text key={f.label} style={[typography.caption, { color: colors.inkSoft }]}>
                {f.label}: {f.detail}
              </Text>
            ))}
          </>
        ) : (
          <Text style={[typography.body, { color: colors.inkSoft }]}>
            Chưa có điểm theo gu nhóm cho nơi này. Ở Khám phá, hỏi Rủ Đi AI một câu để nó chấm.
          </Text>
        )}
      </View>
      {place.reviews.length > 0 ? (
        <View style={styles.khoi}>
          <SectionHeader title={`${place.reviews.length} nhận xét`} />
          {place.reviews.map((r, i) => (
            <View key={`${r.author}-${i}`}>
              {i > 0 ? <Divider /> : null}
              <View style={styles.nhanXet}>
                <Text style={[typography.label, { color: colors.ink }]}>
                  {r.author} · {r.rating}/5
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

/**
 * The other licensed photographs, side by side, each carrying its own credit.
 *
 * One strip and not a grid: a licensed photograph is only allowed on this
 * screen while its author and licence are legible next to it, and a caption
 * under a wide frame stays legible where a caption under a thumbnail does not.
 */
function DaiAnh({ anh }: { anh: AnhDiaDiem[] }) {
  const { radius } = useRudiTheme();
  // Built once per list, not per render: each frame keys its «did not load»
  // state on the picture, and a value rebuilt every frame resets that state.
  const co = useMemo(
    () =>
      anh.map((a) => ({
        id: a.id,
        alt: a.title ?? "Ảnh có giấy phép chụp quanh đây",
        anh: anhDanhMuc(nguonAnhDiaDiem(a), { author: a.author, license: a.license, prefix: TIEN_TO_ANH }),
      })),
    [anh],
  );
  return (
    <ScrollView contentContainerStyle={styles.dai} horizontal showsHorizontalScrollIndicator={false} testID="place-photos">
      {co.map((a) => (
        <MediaSlot
          alt={a.alt}
          key={a.id}
          nguon={{ loai: "danh-muc", anh: a.anh }}
          radius={radius.base}
          style={styles.oAnh}
          width="100%"
        />
      ))}
    </ScrollView>
  );
}

/** Photographs a group of the reader's took here (M12 §2.4): private, so no credit line, and only members see them. */
function DaiAnhNhom({ anh, personId }: { anh: AnhNhom[]; personId: string }) {
  const { colors, radius } = useRudiTheme();
  return (
    <View style={styles.khoi}>
      <SectionHeader title="Ảnh của nhóm bạn" />
      {/* Who may see these is said before the pictures, not after: the reader
          meets the rule with the heading, and at font 1.3 a sentence under a
          strip of photographs sat below the fold (board 2026-09-07). */}
      <Text style={[typography.caption, { color: colors.inkFaint }]}>{CAU_ANH_NHOM}</Text>
      <ScrollView contentContainerStyle={styles.dai} horizontal showsHorizontalScrollIndicator={false} testID="place-group-photos">
        {anh.map((a) => {
          const nguon = nguonAnh(a.imageUrl, personId, a.contextId);
          if (nguon === null) return null;
          return (
            <View key={a.id} style={styles.oAnh}>
              <MediaSlot alt={a.caption ?? "Ảnh của nhóm bạn ở đây"} nguon={{ loai: "nhom", source: nguon }} radius={radius.base} width="100%" />
              {a.caption === null ? null : (
                <Text numberOfLines={2} style={[typography.caption, { color: colors.inkSoft }]}>
                  {a.caption}
                </Text>
              )}
            </View>
          );
        })}
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  sanCho: { alignSelf: "stretch", alignItems: "center" },
  dai: { gap: 12, paddingRight: 8 },
  oAnh: { width: 280, gap: 6 },
  flex: { flex: 1 },
  chinh: { flex: 1.4 },
  khung: { gap: 16 },
  badgeOnMedia: { position: "absolute", left: 12, top: 12 },
  dau: { gap: 8 },
  dauGon: { gap: 12 },
  dauGonDau: { alignSelf: "flex-start" },
  duongDi: { flexDirection: "row", alignItems: "center", gap: 10, paddingVertical: 12, borderTopWidth: StyleSheet.hairlineWidth, borderBottomWidth: StyleSheet.hairlineWidth },
  diaChi: { flex: 1, flexDirection: "row", alignItems: "center", gap: 10, minHeight: 48 },
  bam: { opacity: 0.7 },
  suKien: { gap: 10 },
  hangSuKien: { flexDirection: "row", alignItems: "center", gap: 10 },
  khoi: { gap: 8 },
  nhanXet: { gap: 2, paddingVertical: 8 },
  hanhDong: { flexDirection: "row", gap: 10 },
});
