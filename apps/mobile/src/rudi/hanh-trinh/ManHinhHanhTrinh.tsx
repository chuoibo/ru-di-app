/**
 * Journey view: the day's route on a map, with the same ink route read
 * edge-on underneath it.
 *
 * ## Why the rail, and why it is not a row of dots
 *
 * The map is geography; it cannot be the whole story. Two stops seventy
 * metres apart draw one pin on top of the other at day zoom -- on the
 * emulator (12/09) stop 1 sat entirely behind stop 3 -- and a MapLibre marker
 * puts nothing in the accessibility tree, so a screen reader and a test
 * runner both find an empty map. The rail answers all of that with the
 * product's own language: the hour, the numbered node, the stop, in the order
 * the group agreed on. Selection is shared with the pins, so the two halves
 * always say the same thing.
 *
 * The panel below the rail is the one thing that changes: the day's totals, a
 * chosen stop, or a chosen leg. Nothing floats over the map except the one
 * control that acts on the map.
 */

import { useEffect, useMemo, useRef, useState } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import { useRouter } from "expo-router";

import { typography, useRudiTheme, mauMocHanhTrinh, mauSoMoc } from "../theme";
import { RudiButton } from "../ui";
import { BanDo } from "./BanDo";
import { gioTru, lamGiauDoan, layDoanDuong } from "./duong";
import type { DoanDuongHanhTrinh, HanhTrinh, HoatDongHanhTrinh } from "./mo-hinh";
import { chuKhoangCach, chuThoiGian, tomTatHanhTrinh } from "./tom-tat";
import { kieuBanDo, type MocBanDo } from "./kieu-ban-do";

/** Below this, re-ordering the day is not worth offering as an action. */
const NGUONG_GOI_XEP_LAI = 90;

export function ManHinhHanhTrinh({
  hanh,
  selectedActivityId,
  selectedSegmentId,
  fitDem,
  toiDem,
  onUserMove,
  onChonMoc,
  onChonDoan,
  onNen,
  onKhop,
  onToiUu,
  onVeLichTrinh,
  chanDuoi = 0,
  dangToiUu = false,
}: {
  hanh: HanhTrinh;
  selectedActivityId: string | null;
  selectedSegmentId: string | null;
  fitDem: number;
  toiDem: number;
  onUserMove: () => void;
  onChonMoc: (id: string) => void;
  onChonDoan: (id: string) => void;
  onNen: () => void;
  onKhop: () => void;
  onToiUu?: () => void;
  onVeLichTrinh?: () => void;
  /** Clearance under the panel: the host's tab bar, when it has one. */
  chanDuoi?: number;
  dangToiUu?: boolean;
}) {
  const router = useRouter();
  const { colors, dark, radius } = useRudiTheme();
  const mau = mauMocHanhTrinh(colors);
  const [doan, setDoan] = useState<DoanDuongHanhTrinh[]>(hanh.routeSegments);
  const cache = useRef(new Map<string, Awaited<ReturnType<typeof layDoanDuong>>>());

  useEffect(() => {
    setDoan(hanh.routeSegments);
    let song = true;
    void lamGiauDoan(hanh.routeSegments, hanh.activities, { cache: cache.current }).then((giau) => {
      if (song) setDoan(giau);
    });
    return () => {
      song = false;
    };
  }, [hanh]);

  const mocs: MocBanDo[] = useMemo(
    () =>
      hanh.activities
        .filter((a): a is HoatDongHanhTrinh & { lat: number; lng: number; so: number } => a.lat !== null && a.lng !== null && a.so !== null)
        .map((a) => ({
          id: a.id,
          so: a.so,
          lat: a.lat,
          lng: a.lng,
          tieuDe: a.tieuDe,
          gio: a.gio,
          chon: a.id === selectedActivityId,
        })),
    [hanh.activities, selectedActivityId],
  );

  const doanHien = doan.length > 0 ? doan : hanh.routeSegments;
  const tom = tomTatHanhTrinh(hanh.activities, doanHien);
  const mocChon = hanh.activities.find((a) => a.id === selectedActivityId) ?? null;
  const doanChon = doanHien.find((d) => d.id === selectedSegmentId) ?? null;
  const toi = (() => {
    if (!mocChon || mocChon.lat === null || mocChon.lng === null || toiDem === 0) return null;
    return { lat: mocChon.lat, lng: mocChon.lng, dem: toiDem };
  })();

  const coMoc = tom.soChangCoViTri > 0;
  // The public OSRM demo answers most of the time and not always. When it does
  // not, every leg is a ruler line and the totals are straight-line estimates
  // -- say so instead of letting «7,3 km» read as a road distance.
  const uocLuong = doanHien.length > 0 && doanHien.every((d) => d.nguon === "geodesic");
  const thieu = tom.soChang - tom.soChangCoViTri;
  const tomChu = [
    `${tom.soChangCoViTri} điểm trên bản đồ`,
    tom.met !== null ? chuKhoangCach(tom.met) : null,
    tom.giay !== null ? chuThoiGian(tom.giay) : null,
  ]
    .filter((s): s is string => s !== null)
    .join(" · ");

  // «Xếp lại» is offered only when there is a real saving to name. A bare
  // percentage told nobody anything; the number people act on is how much
  // shorter the day gets.
  // The saving is named in the unit already on screen, never as a percentage:
  // ADR-0009 quyết định 4 keeps percentages out of what a person reads. The
  // ratio comes from two tours measured the same way; applying it to the
  // distance shown keeps the two numbers reconcilable.
  const gonHon = tom.hieuSuat !== null && tom.hieuSuat < NGUONG_GOI_XEP_LAI ? 100 - tom.hieuSuat : null;
  const botMet = gonHon !== null && tom.met !== null ? Math.round((tom.met * gonHon) / 100) : null;
  const moiXepLai = onToiUu !== undefined && tom.soChangCoViTri >= 3 && botMet !== null && botMet >= 500;

  const mauMoc = [mauSoMoc(colors, 1), mauSoMoc(colors, 2), mauSoMoc(colors, 3)];
  const mauCuaMoc = (so: number) => mauMoc[(so - 1) % mauMoc.length] ?? mau.mocChon;

  return (
    <View style={styles.khung}>
      <BanDo
        doan={doanHien.map((d) => ({ id: d.id, polyline: d.polyline, chon: d.id === selectedSegmentId }))}
        fitDem={fitDem}
        kieu={kieuBanDo(dark)}
        mauDuong={mau.duong}
        mauDuongMo={mau.duongMo}
        mauMoc={mauMoc}
        mauMocChon={mau.mocChon}
        mauMocInk={mau.mocInk}
        mauNen={mau.the}
        mauVien={mau.vien}
        mauVienDuong={mau.vienDuong}
        mocs={mocs}
        onChonDoan={onChonDoan}
        onChonMoc={onChonMoc}
        onNen={onNen}
        onUserMove={onUserMove}
        toi={toi}
      />
      <View pointerEvents="box-none" style={StyleSheet.absoluteFill}>
        {coMoc ? (
          <View pointerEvents="box-none" style={styles.hangNut}>
            <RudiButton accessibilityLabel="Khớp hành trình" compact full={false} icon="scan-outline" label="Khớp hành trình" onPress={onKhop} variant="outline" />
          </View>
        ) : null}
        <View pointerEvents="none" style={styles.dan} />
        <View style={[styles.the, { backgroundColor: colors.card, borderColor: colors.line, borderRadius: radius.base, marginBottom: 12 + chanDuoi }]}>
          {coMoc ? (
            <ThanhChang
              mauCuaMoc={mauCuaMoc}
              mauInk={mau.mocInk}
              mocs={mocs}
              onChon={(id) => (id === selectedActivityId ? onNen() : onChonMoc(id))}
            />
          ) : null}
          {mocChon ? (
            <TheMoc
              chang={doanToi(doanHien, mocChon.id)}
              moc={mocChon}
              onChiTiet={mocChon.placeId ? () => router.push(`/places/${mocChon.placeId}` as never) : undefined}
              truoc={changTruoc(hanh.activities, mocChon.id)}
            />
          ) : doanChon ? (
            <TheDoan activities={hanh.activities} doan={doanChon} />
          ) : coMoc ? (
            <View style={styles.khoiThe}>
              <Text style={[typography.label, { color: colors.ink }]}>{tomChu}</Text>
              {uocLuong ? (
                <Text style={[typography.note, { color: colors.inkSoft }]}>Ước lượng theo đường chim bay.</Text>
              ) : null}
              {thieu > 0 ? (
                <Text style={[typography.note, { color: colors.inkSoft }]}>
                  {thieu === 1 ? "1 hoạt động khác chưa gắn địa điểm" : `${thieu} hoạt động khác chưa gắn địa điểm`}
                </Text>
              ) : null}
              {moiXepLai ? (
                <>
                  <Text style={[typography.note, { color: colors.inkSoft }]}>
                    Xếp lại theo đường gần nhất: ngắn hơn khoảng {chuKhoangCach(botMet ?? 0)}.
                  </Text>
                  <RudiButton accessibilityLabel="Tối ưu lộ trình" compact disabled={dangToiUu} label="Tối ưu lộ trình" loading={dangToiUu} onPress={onToiUu} />
                </>
              ) : tom.soChangCoViTri >= 3 ? (
                <Text style={[typography.note, { color: colors.inkSoft }]}>Thứ tự hiện tại đã gọn rồi.</Text>
              ) : null}
            </View>
          ) : (
            <View style={styles.khoiThe}>
              <Text style={[typography.h2, { color: colors.ink }]}>Ngày này chưa có điểm nào trên bản đồ</Text>
              <Text style={[typography.note, { color: colors.inkSoft }]}>
                Gắn một quán hoặc một địa điểm vào lịch trình, đường đi sẽ hiện ở đây.
              </Text>
              {onVeLichTrinh ? (
                <RudiButton accessibilityLabel="Về Lịch trình" compact label="Về Lịch trình" onPress={onVeLichTrinh} variant="outline" />
              ) : null}
            </View>
          )}
        </View>
      </View>
    </View>
  );
}

/**
 * The day's stops, in order, as one scrollable rail.
 *
 * This is the only element that names every stop: pins carry a number and no
 * accessible label, and two of them can land on the same pixel.
 */
function ThanhChang({
  mocs,
  onChon,
  mauCuaMoc,
  mauInk,
}: {
  mocs: readonly MocBanDo[];
  onChon: (id: string) => void;
  mauCuaMoc: (so: number) => string;
  mauInk: string;
}) {
  const { colors, radius } = useRudiTheme();
  return (
    <ScrollView
      contentContainerStyle={styles.thanhTrong}
      horizontal
      showsHorizontalScrollIndicator={false}
      style={styles.thanh}
    >
      {mocs.map((moc) => (
        <Pressable
          accessibilityLabel={`Mốc ${moc.so}, ${moc.gio}, ${moc.tieuDe}`}
          accessibilityRole="button"
          accessibilityState={{ selected: moc.chon }}
          key={moc.id}
          onPress={() => onChon(moc.id)}
          style={[
            styles.chang,
            {
              backgroundColor: moc.chon ? colors.accentSoft : colors.ground,
              borderColor: moc.chon ? colors.accent : colors.line,
              borderRadius: radius.small,
            },
          ]}
        >
          <View style={[styles.changSo, { backgroundColor: mauCuaMoc(moc.so) }]}>
            <Text style={[styles.changSoChu, { color: mauInk }]}>{moc.so}</Text>
          </View>
          <View style={styles.changChu}>
            <Text style={[typography.caption, { color: colors.inkFaint }]}>{moc.gio}</Text>
            <Text numberOfLines={1} style={[typography.label, { color: colors.ink }]}>
              {moc.tieuDe}
            </Text>
          </View>
        </Pressable>
      ))}
    </ScrollView>
  );
}

function changTruoc(danh: readonly HoatDongHanhTrinh[], id: string): HoatDongHanhTrinh | null {
  const mapped = danh.filter((a) => a.lat !== null);
  const i = mapped.findIndex((a) => a.id === id);
  if (i <= 0) return null;
  return mapped[i - 1];
}

/** The leg that ends at this stop, once it exists. */
function doanToi(doan: readonly DoanDuongHanhTrinh[], id: string): DoanDuongHanhTrinh | null {
  return doan.find((d) => d.toActivityId === id) ?? null;
}

function TheMoc({
  moc,
  truoc,
  chang,
  onChiTiet,
}: {
  moc: HoatDongHanhTrinh;
  truoc: HoatDongHanhTrinh | null;
  chang: DoanDuongHanhTrinh | null;
  onChiTiet?: () => void;
}) {
  const { colors } = useRudiTheme();
  return (
    <View style={styles.khoiThe}>
      <Text style={[typography.caption, { color: colors.inkFaint }]}>{moc.gio}</Text>
      <Text style={[typography.h2, { color: colors.ink }]}>{moc.tieuDe}</Text>
      {moc.diaChi ? <Text style={[typography.note, { color: colors.inkSoft }]}>{moc.diaChi}</Text> : null}
      {truoc ? (
        <Text style={[typography.note, { color: colors.inkSoft }]}>
          {chang
            ? `Từ ${truoc.tieuDe} · ${chuKhoangCach(chang.distanceMeters)} · ${chuThoiGian(chang.durationSeconds)}`
            : `Từ ${truoc.tieuDe}`}
        </Text>
      ) : (
        <Text style={[typography.note, { color: colors.inkSoft }]}>Điểm đầu tiên trên bản đồ của ngày</Text>
      )}
      {onChiTiet ? <RudiButton compact label="Xem chi tiết" onPress={onChiTiet} variant="outline" /> : null}
    </View>
  );
}

function TheDoan({ doan, activities }: { doan: DoanDuongHanhTrinh; activities: readonly HoatDongHanhTrinh[] }) {
  const { colors } = useRudiTheme();
  const from = activities.find((a) => a.id === doan.fromActivityId);
  const to = activities.find((a) => a.id === doan.toActivityId);
  const roi = to ? gioTru(to.gio, doan.durationSeconds) : null;
  return (
    <View style={styles.khoiThe}>
      <Text style={[typography.h2, { color: colors.ink }]}>
        {from?.tieuDe ?? "A"} → {to?.tieuDe ?? "B"}
      </Text>
      <Text style={[typography.body, { color: colors.inkSoft }]}>
        {chuThoiGian(doan.durationSeconds)} · {chuKhoangCach(doan.distanceMeters)}
        {doan.nguon === "geodesic" ? " · đường chim bay" : ""}
      </Text>
      {roi && to ? (
        <Text style={[typography.note, { color: colors.inkSoft }]}>
          Rời khoảng {roi} để tới lúc {to.gio}
        </Text>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  khung: { flex: 1, minHeight: 280 },
  hangNut: { paddingHorizontal: 12, paddingTop: 8, alignItems: "flex-start" },
  dan: { flex: 1 },
  the: { margin: 12, padding: 14, gap: 10, borderWidth: StyleSheet.hairlineWidth },
  khoiThe: { gap: 6 },
  thanh: { marginHorizontal: -14, marginTop: -2 },
  thanhTrong: { paddingHorizontal: 14, gap: 8 },
  chang: { flexDirection: "row", alignItems: "center", gap: 8, paddingVertical: 8, paddingHorizontal: 10, borderWidth: 1, maxWidth: 210 },
  changSo: { width: 24, height: 24, borderRadius: 999, alignItems: "center", justifyContent: "center" },
  changSoChu: { fontSize: 12, fontWeight: "700", lineHeight: 15 },
  changChu: { flexShrink: 1 },
});
