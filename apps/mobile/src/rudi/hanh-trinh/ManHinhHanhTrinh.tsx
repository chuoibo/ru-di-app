/**
 * THESIS: The group unfolds its day as a map and a page from its trip notebook.
 * OWN-WORLD: Existing paper, ink, coral and place sketches; no new palette.
 * STORY: Read the route, inspect a stop, compare changes before keeping them.
 * FIRST VIEWPORT: Geography above a compact, collapsible day page; wide screens
 * keep the page beside the map. Important times remain with their stops.
 * FORM: Approved itinerary extension, code-led; no replacement visual world.
 * FINISH (M7 bản đồ): stops are paper stamps that say their state by shape and
 * word (reached = pencil + tick, next = coral, lifted); the plan is one ink
 * line with its minutes on it, a draft is a broken pencil line; the page head
 * names the day and stamps what the line is -- a real road, a draft, or «chưa
 * tính được» -- so a straight line is never read as a road.
 */
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View, useWindowDimensions } from "react-native";
import { useRouter } from "expo-router";

import { typography, useRudiTheme, mauMocHanhTrinh } from "../theme";
import { RudiButton } from "../ui";
import { Stamp } from "../ui/Stamp";
import { ganTrangThai } from "./chieu";
import { BanDo } from "./BanDo";
import { gioTru } from "./duong";
import type { DoanDuongHanhTrinh, HanhTrinh, HoatDongHanhTrinh } from "./mo-hinh";
import { chuKhoangCach, chuThoiGian, tomTatHanhTrinh } from "./tom-tat";
import { kieuBanDo, nhanDoan, nhanMoc, type MocBanDo, type NeoMoc } from "./kieu-ban-do";

import type { TrangThaiTuyen } from "./ke-hoach";

/** The day's anchors, from its settings (start / end / return to start). */
export type NeoNgay = { xuatPhat: string | null; ketThuc: string | null; veDiemDau: boolean };

import { useMotion } from "../ui/useMotion";
import { useNhuongChoNep } from "../nep/NepProvider";
import { Canh } from "../ui/art/Canh";
import { NenGiay } from "../ui/NenGiay";
import { hinhTrangXe } from "../art/giay";

/** Depth of the torn edge; the page overlaps the map by twice this. */
const XE = 3;
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
  tuyen = "uocLuong",
  phuongTien,
  tieuDeTrang,
  daToiIds = [],
  dangDi = false,
  neo,
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
  tuyen?: TrangThaiTuyen;
  /** «XE MÁY», «Ô TÔ», «ĐI BỘ»: printed on the real-road stamp. */
  phuongTien?: string;
  /** «Ngày 1 · T7 12/10»; absent on a single-day fixture. */
  tieuDeTrang?: string;
  /** Stops somebody in the group has checked in at. */
  daToiIds?: readonly string[];
  /** The outing is happening today, so one stop is "next". */
  dangDi?: boolean;
  neo?: NeoNgay;
}) {
  const router = useRouter();
  const { colors, dark, radius } = useRudiTheme();
  const mau = useMemo(() => mauMocHanhTrinh(colors, dark), [colors, dark]);
  const kieu = useMemo(() => kieuBanDo(dark), [dark]);
  const doan = hanh.routeSegments;
  const { width, height, fontScale } = useWindowDimensions();
  const [collapsed, setCollapsed] = useState(false);
  const [availableHeight, setAvailableHeight] = useState(height * 0.65);
  const wide = width >= 840 && fontScale < 1.8;
  const motion = useMotion();
  // The map runs edge to edge and pans under a finger at the right edge too,
  // and its attribution sits 8dp in: there is no margin here, so Nếp makes room.
  useNhuongChoNep(true);
  // Room for half a stamp (24) above the web attribution strip (≈26) at the
  // bottom, and below «Khớp hành trình» at the top: a fitted stop is never
  // under a control or the credit line.
  const padding = useMemo(() => ({ top: 72, left: 40, right: 40, bottom: 64 }), []);

  const mocs: MocBanDo[] = useMemo(() => {
    const coViTri = hanh.activities.filter(
      (a): a is HoatDongHanhTrinh & { lat: number; lng: number; so: number } => a.lat !== null && a.lng !== null && a.so !== null,
    );
    const trangThai = daToiIds.length > 0 || dangDi ? ganTrangThai(coViTri.map((a) => a.id), daToiIds, dangDi) : null;
    const neoCua = (id: string): NeoMoc => {
      if (!neo) return null;
      if (neo.xuatPhat === id) return neo.veDiemDau ? "ve" : "xuat-phat";
      if (neo.ketThuc === id) return "ket-thuc";
      return null;
    };
    return coViTri.map((a, i) => ({
      id: a.id,
      so: a.so,
      lat: a.lat,
      lng: a.lng,
      tieuDe: a.tieuDe,
      gio: a.gio,
      chon: a.id === selectedActivityId,
      trangThai: trangThai ? trangThai[i] : null,
      neo: neoCua(a.id),
    }));
  }, [hanh.activities, selectedActivityId, daToiIds.join("|"), dangDi, neo?.xuatPhat, neo?.ketThuc, neo?.veDiemDau]);

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

  const dauTuyen =
    tuyen === "that" ? `ĐƯỜNG THẬT${phuongTien ? ` · ${phuongTien}` : ""}`
    : tuyen === "dangTinh" ? "ĐANG TÍNH ĐƯỜNG"
    : tuyen === "khongTinhDuoc" ? "CHƯA TÍNH ĐƯỜNG"
    : "NÉT NHÁP";

  return (
    <View onLayout={(e) => setAvailableHeight(e.nativeEvent.layout.height)} style={[styles.khung, wide && { flexDirection: "row" }]}>
      <View style={{ flex: 1, minHeight: 0 }}>
      <BanDo
        doan={doanHien.map((d) => ({ id: d.id, polyline: d.polyline, uocLuong: d.nguon === "geodesic", chon: d.id === selectedSegmentId, nhan: nhanDoan(d) }))}
        fitDem={fitDem}
        cameraKey={cameraKey ?? hanh.activities.map((a) => a.id).join("|")}
        padding={padding}
        fitPoints={fitPoints}
        duration={motion.ms("standard")}
        onGhim={onGhim}
        kieu={kieu}
        mau={mau}
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
        {/* The day page is a page torn out of the trip notebook and laid over
            the map: the torn edge and its binding holes face the map. It is a
            `card` sheet, not `paper`: its accent and faint text stay legible
            at night (D14). */}
        <NenGiay
          cao={2}
          hinh={(w, h) => {
            const t = hinhTrangXe(w, h, wide ? "trai" : "tren", XE);
            return { nen: t.nen, vien: t.vien, them: t.lo };
          }}
          style={[styles.the, wide ? styles.theRong : styles.theHep, { paddingBottom: 12 + chanDuoi, maxHeight: wide ? undefined : availableHeight * (fontScale >= 1.8 ? 0.65 : 0.56), width: wide ? 360 : undefined }]}
        >
          {/* The page head: which day, what the line on the map is, and the
              day's numbers. The stamp is the answer to «is that a road?». */}
          <View style={styles.dauTrang}>
            <View style={styles.dauChu}>
              {tieuDeTrang ? <Text accessibilityRole="header" style={[typography.h2, { color: colors.ink }]}>{tieuDeTrang}</Text> : null}
              {coMoc ? <Text style={[typography.label, styles.so, { color: colors.inkSoft }]}>{tomChu}</Text> : null}
            </View>
            {coMoc ? <Stamp label={dauTuyen} tone={tuyen === "that" ? "accent" : "ink"} tilt={tuyen === "that" ? -2 : 0} testID="hanh-trinh-dau-tuyen" /> : null}
          </View>
          <Pressable accessibilityRole="button" accessibilityState={{ expanded: !collapsed }} onPress={() => setCollapsed(!collapsed)} style={styles.hangGap}>
            <Text style={[typography.label, { color: colors.ink }]}>Các chặng trong ngày</Text>
            <Text style={[typography.caption, { color: colors.accent }]}>{collapsed ? "Mở trang" : "Thu gọn"}</Text>
          </Pressable>
          {collapsed ? null : <ScrollView style={{ flexShrink: 1 }} keyboardShouldPersistTaps="handled" contentContainerStyle={{ gap: 12, paddingBottom: 8 }}>

          {coMoc ? (
            <ThanhChang
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
              {tuyen === "khongTinhDuoc" ? (
                <Text accessibilityLiveRegion="polite" style={[typography.note, { color: colors.ink }]}>
                  Chưa tính được đường đi lúc này. Thứ tự các điểm vẫn ở đây; thử «Tính lại đường» sau ít phút.
                </Text>
              ) : tuyen === "dangTinh" ? (
                <Text style={[typography.note, { color: colors.inkSoft }]}>Đang tính đường đi thật cho cả ngày…</Text>
              ) : uocLuong ? (
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
              <Text style={[typography.h2, { color: colors.ink }]}>Ngày này chưa có điểm nào trên bản đồ</Text>
              <Text style={[typography.note, { color: colors.inkSoft }]}>
                Gắn một quán hoặc một địa điểm vào lịch trình, đường đi sẽ hiện ở đây.
              </Text>
            </View>
          )}
          {actions}
          </ScrollView>}
          {!coMoc && onVeLichTrinh ? (
            <RudiButton accessibilityLabel="Về Lịch trình" compact label="Về Lịch trình" onPress={onVeLichTrinh} variant="outline" />
          ) : coMoc ? primaryAction : null}
        </NenGiay>
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
}: {
  mocs: readonly MocBanDo[];
  onChon: (id: string) => void;
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
          accessibilityLabel={nhanMoc(moc)}
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
          {/* The rail repeats the pin's stamp in small: same number, same state. */}
          <View
            style={[
              styles.changSo,
              moc.trangThai === "hien-tai"
                ? { backgroundColor: colors.accent, borderColor: colors.accent }
                : { backgroundColor: colors.card, borderColor: moc.trangThai === "xong" ? colors.lineStrong : colors.accent },
            ]}
          >
            <Text
              style={[
                styles.changSoChu,
                { color: moc.trangThai === "hien-tai" ? colors.accentInk : moc.trangThai === "xong" ? colors.inkFaint : colors.accent },
              ]}
            >
              {moc.so}
            </Text>
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
  the: { paddingHorizontal: 16, gap: 8 },
  // The torn edge and the holes (13dp in) sit over the map's last few dp.
  theHep: { marginTop: -XE * 2, paddingTop: XE * 2 + 12 },
  theRong: { marginLeft: -XE * 2, paddingLeft: XE * 2 + 18, paddingTop: 8 },
  khoiThe: { gap: 6 },
  thanh: { marginHorizontal: -14, marginTop: -2 },
  thanhTrong: { paddingHorizontal: 14, gap: 8 },
  chang: { flexDirection: "row", alignItems: "center", gap: 8, paddingVertical: 8, paddingHorizontal: 10, borderWidth: 1, maxWidth: 210 },
  changSo: { minWidth: 26, height: 26, borderRadius: 6, borderWidth: 1.5, alignItems: "center", justifyContent: "center", paddingHorizontal: 4 },
  changSoChu: { fontSize: 13, fontWeight: "800", lineHeight: 16, fontVariant: ["tabular-nums"] },
  dauTrang: { flexDirection: "row", alignItems: "flex-start", justifyContent: "space-between", gap: 12, flexWrap: "wrap" },
  dauChu: { flexShrink: 1, gap: 2, minWidth: 160 },
  so: { fontVariant: ["tabular-nums"] },
  hangGap: { minHeight: 48, flexDirection: "row", justifyContent: "space-between", alignItems: "center" },
  changChu: { flexShrink: 1 },
});
