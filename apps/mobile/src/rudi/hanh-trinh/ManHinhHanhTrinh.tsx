/**
 * THESIS: The group unfolds its day as a map and a page from its trip notebook.
 * OWN-WORLD: Existing paper, ink, coral and place sketches; no new palette.
 * STORY: Read the route, inspect a stop, compare changes before keeping them.
 * FIRST VIEWPORT: Geography above a compact, collapsible day page; wide screens
 * keep the page beside the map. Important times remain with their stops.
 * FORM: Approved itinerary extension, code-led; no replacement visual world.
 */
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View, useWindowDimensions } from "react-native";
import { useRouter } from "expo-router";

import { typography, useRudiTheme, mauMocHanhTrinh } from "../theme";
import { RudiButton } from "../ui";
import { BanDo } from "./BanDo";
import { gioTru } from "./duong";
import type { DoanDuongHanhTrinh, HanhTrinh, HoatDongHanhTrinh } from "./mo-hinh";
import { chuKhoangCach, chuThoiGian, tomTatHanhTrinh } from "./tom-tat";
import { kieuBanDo, type MocBanDo } from "./kieu-ban-do";

import { useMotion } from "../ui/useMotion";
import { Canh } from "../ui/art/Canh";
import { KyHoa } from "../ui/art/KyHoa";

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
  actions,
  primaryAction,
  cameraKey,
  fitPoints,
  onGhim,
  onVeLichTrinh,
  chanDuoi = 0,
  dangToiUu = false,
}: {
  hanh: HanhTrinh;
  actions?: ReactNode;
  primaryAction?: ReactNode;
  cameraKey?: string;
  fitPoints?: {lat:number;lng:number}[];
  onGhim?: (point: { lat: number; lng: number }) => void;
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
  const doan = hanh.routeSegments;
  const { width, height, fontScale } = useWindowDimensions();
  const [collapsed, setCollapsed] = useState(false);
  const [availableHeight, setAvailableHeight] = useState(height * 0.65);
  const wide = width >= 840 && fontScale < 1.8;
  const motion = useMotion();
  const padding = useMemo(() => ({ top: 72, left: 40, right: 40, bottom: 40 }), []);

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
  const toi = useMemo(() => {
    if (!mocChon || mocChon.lat === null || mocChon.lng === null || toiDem === 0) return null;
    return { lat: mocChon.lat, lng: mocChon.lng, dem: toiDem };
  }, [mocChon?.id, mocChon?.lat, mocChon?.lng, toiDem]);

  const coMoc = tom.soChangCoViTri > 0;
  // Unrouted drafts only connect the stops in order. Their straight-line
  // geometry must never be presented as a measured road distance.
  const uocLuong = doanHien.length > 0 && doanHien.some((d) => d.nguon === "geodesic");
  const thieu = tom.soChang - tom.soChangCoViTri;
  const tomChu = [
    `${tom.soChangCoViTri} điểm trên bản đồ`,
    !uocLuong && tom.met !== null ? chuKhoangCach(tom.met) : null,
    !uocLuong && tom.giay !== null ? chuThoiGian(tom.giay) : null,
  ]
    .filter((s): s is string => s !== null)
    .join(" · ");

  const mauMoc = [colors.accent];
  const mauCuaMoc = () => colors.accent;

  return (
    <View onLayout={(e) => setAvailableHeight(e.nativeEvent.layout.height)} style={[styles.khung, wide && { flexDirection: "row" }]}>
      <View style={{ flex: 1, minHeight: 0 }}>
      <BanDo
        doan={doanHien.map((d) => ({ id: d.id, polyline: d.polyline, uocLuong: d.nguon === "geodesic", chon: d.id === selectedSegmentId }))}
        fitDem={fitDem}
        cameraKey={cameraKey ?? hanh.activities.map((a) => a.id).join("|")}
        padding={padding}
        fitPoints={fitPoints}
        duration={motion.ms("standard")}
        onGhim={onGhim}
        kieu={kieuBanDo(dark)}
        mauDuong={mau.duong}
        mauDuongMo={mau.duongMo}
        mauMoc={mauMoc}
        mauMocChon={colors.accent}
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
      </View>
      </View>
        <View style={[styles.the, { backgroundColor: colors.paper, borderColor: colors.line, paddingBottom: 12 + chanDuoi, maxHeight: wide ? undefined : availableHeight * (fontScale >= 1.8 ? 0.65 : 0.56), width: wide ? 360 : undefined }]}>
          <Pressable accessibilityRole="button" accessibilityState={{ expanded: !collapsed }} onPress={() => setCollapsed(!collapsed)} style={{ minHeight: 48, flexDirection: "row", justifyContent: "space-between", alignItems: "center" }}>
            <Text style={[typography.label, { color: colors.ink }]}>Trang ngày của hội</Text>
            <Text style={[typography.caption, { color: colors.accent }]}>{collapsed ? "Mở trang" : "Thu gọn"}</Text>
          </Pressable>
          {collapsed ? <Text style={[typography.note, { color: colors.inkSoft }]}>{tomChu}</Text> : <ScrollView style={{ flexShrink: 1 }} keyboardShouldPersistTaps="handled" contentContainerStyle={{ gap: 12, paddingBottom: 8 }}>

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
                <Text style={[typography.note, { color: colors.inkSoft }]}>Chưa có tuyến đường bộ. Nét nối chỉ thể hiện thứ tự điểm hẹn.</Text>
              ) : null}
              {thieu > 0 ? (
                <Text style={[typography.note, { color: colors.inkSoft }]}>
                  {thieu === 1 ? "1 hoạt động khác chưa gắn địa điểm" : `${thieu} hoạt động khác chưa gắn địa điểm`}
                </Text>
              ) : null}
              {onToiUu ? <RudiButton compact disabled={dangToiUu} label="Xem cách đi gọn hơn" loading={dangToiUu} onPress={onToiUu} /> : null}
            </View>
          ) : (
            <View style={styles.khoiThe}>
              <Canh id="tim-khong-ra" width={144} />
              <Text style={[typography.h2, { color: colors.ink }]}>Mở một trang đường mới</Text>
              <Text style={[typography.note, { color: colors.inkSoft }]}>
                Gắn một quán hoặc một địa điểm vào lịch trình, đường đi sẽ hiện ở đây.
              </Text>
              {onVeLichTrinh ? (
                <RudiButton accessibilityLabel="Về Lịch trình" compact label="Về Lịch trình" onPress={onVeLichTrinh} variant="outline" />
              ) : null}
            </View>
          )}
          {actions}
          </ScrollView>}
          {primaryAction}
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
  const rail = useRef<ScrollView>(null);
  const positions = useRef<Record<string, number>>({});
  const selected = mocs.find((m) => m.chon)?.id;
  useEffect(() => { if (selected) rail.current?.scrollTo({ x: Math.max(0, (positions.current[selected] ?? 0) - 16), animated: false }); }, [selected]);
  return (
    <ScrollView
      ref={rail}
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
          onLayout={(e) => { positions.current[moc.id] = e.nativeEvent.layout.x; }}
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
      {moc.category ? <KyHoa loai={moc.category} gon /> : null}
      <Text style={[typography.caption, { color: colors.inkFaint }]}>{moc.gio}</Text>
      <Text testID="hanh-trinh-selected-stop" style={[typography.h2, { color: colors.ink }]}>{moc.tieuDe}</Text>
      {moc.diaChi ? <Text style={[typography.note, { color: colors.inkSoft }]}>{moc.diaChi}</Text> : null}
      {truoc ? (
        <Text style={[typography.note, { color: colors.inkSoft }]}>
          {chang
            ? chang.nguon === "geodesic" ? `Từ ${truoc.tieuDe} · chưa có đường bộ` : `Từ ${truoc.tieuDe} · ${chuKhoangCach(chang.distanceMeters)} · ${chuThoiGian(chang.durationSeconds)}`
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
  const roi = to && doan.nguon !== "geodesic" ? gioTru(to.gio, doan.durationSeconds) : null;
  return (
    <View style={styles.khoiThe}>
      <Text style={[typography.h2, { color: colors.ink }]}>
        {from?.tieuDe ?? "A"} → {to?.tieuDe ?? "B"}
      </Text>
      <Text style={[typography.body, { color: colors.inkSoft }]}>
        {doan.nguon === "geodesic" ? "Chưa có tuyến đường bộ; không tính giờ di chuyển." : `${chuThoiGian(doan.durationSeconds)} · ${chuKhoangCach(doan.distanceMeters)}`}
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
  khung: { flex: 1, minHeight: 0 },
  hangNut: { paddingHorizontal: 12, paddingTop: 8, alignItems: "flex-start" },
  dan: { flex: 1 },
  the: { paddingHorizontal: 16, gap: 8, borderTopWidth: StyleSheet.hairlineWidth },
  khoiThe: { gap: 6 },
  thanh: { marginHorizontal: -14, marginTop: -2 },
  thanhTrong: { paddingHorizontal: 14, gap: 8 },
  chang: { flexDirection: "row", alignItems: "center", gap: 8, paddingVertical: 8, paddingHorizontal: 10, borderWidth: 1, maxWidth: 210 },
  changSo: { width: 24, height: 24, borderRadius: 999, alignItems: "center", justifyContent: "center" },
  changSoChu: { fontSize: 12, fontWeight: "700", lineHeight: 15 },
  changChu: { flexShrink: 1 },
});
