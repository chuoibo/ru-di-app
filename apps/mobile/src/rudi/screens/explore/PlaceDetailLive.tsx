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
import { useCallback, useEffect, useState } from "react";
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
import { SkeletonCard, SkeletonGroup, SkeletonLines } from "../../ui/Skeleton";
import { Stamp } from "../../ui/Stamp";
import { PlaceGlyph } from "./HangDiaDiem";

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
        onChiDuong={() => void chiDuong(trang.place)}
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
}: {
  place: PlaceDetail;
  anh: AnhDiaDiem[];
  anhNhom: AnhNhom[];
  loiAnh: string | null;
  personId: string;
  thongBao: string | null;
  onChiDuong: () => void;
  onThemAnhNhom: () => void;
}) {
  const { colors } = useRudiTheme();
  const hop = matchLabel(place.match);
  const coMatch = place.match !== null && place.match.source === "ai";
  const facts = chiTietNgan(place).filter((muc) => muc.icon !== "time-outline" && muc.icon !== "location-outline");
  // The cover is drawn only with its credit (ADR-0017 §2.5); the gallery
  // strip carries the other licensed photographs, each with its own line.
  const bia = anhBiaThe(place);
  const conLai = bia === null ? anh : anh.filter((a) => nguonAnhDiaDiem(a).uri !== bia.nguon.uri);
  const viec = cauHoatDong(place.activities);
  return (
    <>
      <MediaSlot
        alt={place.name}
        attribution={bia === null || place.photoAuthor === null || place.photoLicense === null ? undefined : { author: place.photoAuthor, license: place.photoLicense, prefix: TIEN_TO_ANH }}
        fallback={<PlaceGlyph glyph={bieuTuongLoai(place.category)} size={56} />}
        overlay={hop !== null && hop.real ? <View style={styles.badgeOnMedia}><Stamp label={hop.text} nen tilt={-2} tone="ai" /></View> : null}
        ratio={16 / 10}
        source={bia === null ? null : bia.nguon}
      />
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
  return (
    <ScrollView contentContainerStyle={styles.dai} horizontal showsHorizontalScrollIndicator={false} testID="place-photos">
      {anh.map((a) => (
        <MediaSlot
          alt={a.title ?? "Ảnh có giấy phép chụp quanh đây"}
          attribution={{ author: a.author, license: a.license, prefix: TIEN_TO_ANH }}
          key={a.id}
          radius={radius.base}
          source={nguonAnhDiaDiem(a)}
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
      <ScrollView contentContainerStyle={styles.dai} horizontal showsHorizontalScrollIndicator={false} testID="place-group-photos">
        {anh.map((a) => {
          const nguon = nguonAnh(a.imageUrl, personId, a.contextId);
          if (nguon === null) return null;
          return (
            <View key={a.id} style={styles.oAnh}>
              <MediaSlot alt={a.caption ?? "Ảnh của nhóm bạn ở đây"} radius={radius.base} source={nguon} width="100%" />
              {a.caption === null ? null : (
                <Text numberOfLines={2} style={[typography.caption, { color: colors.inkSoft }]}>
                  {a.caption}
                </Text>
              )}
            </View>
          );
        })}
      </ScrollView>
      <Text style={[typography.caption, { color: colors.inkFaint }]}>{CAU_ANH_NHOM}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  dai: { gap: 12, paddingRight: 8 },
  oAnh: { width: 280, gap: 6 },
  flex: { flex: 1 },
  chinh: { flex: 1.4 },
  khung: { gap: 16 },
  badgeOnMedia: { position: "absolute", left: 12, top: 12 },
  dau: { gap: 8 },
  duongDi: { flexDirection: "row", alignItems: "center", gap: 10, paddingVertical: 12, borderTopWidth: StyleSheet.hairlineWidth, borderBottomWidth: StyleSheet.hairlineWidth },
  diaChi: { flex: 1, flexDirection: "row", alignItems: "center", gap: 10, minHeight: 48 },
  bam: { opacity: 0.7 },
  suKien: { gap: 10 },
  hangSuKien: { flexDirection: "row", alignItems: "center", gap: 10 },
  khoi: { gap: 8 },
  nhanXet: { gap: 2, paddingVertical: 8 },
  hanhDong: { flexDirection: "row", gap: 10 },
});
