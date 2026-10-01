/**
 * THESIS: The group unfolds its day as a map and a page from its trip notebook.
 * OWN-WORLD: Existing paper, ink, coral and place sketches; no new palette.
 * STORY: Read the route, inspect a stop, compare changes before keeping them.
 * FIRST VIEWPORT: Geography above a compact, collapsible day page; wide screens
 * keep the page beside the map. Important times remain with their stops.
 * FORM: Approved itinerary extension, code-led; no replacement visual world.
 * Inherits the app's roll, never re-rolled: v3 «Sân khấu giấy» (ADR-0037,
 * app/_layout.tsx) on the v2 seed c8e88116.
 * FINISH (bản đồ hành trình): stops are paper stamps that say their state by shape and
 * word (reached = pencil + tick, next = coral, lifted); the plan is one ink
 * line with its minutes on it, a draft is a broken pencil line; the page head
 * names the day and stamps what the line is -- a real road, a draft, or «chưa
 * tính được» -- so a straight line is never read as a road.
 */
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { Platform, Pressable, ScrollView, StyleSheet, Text, View, useWindowDimensions } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import Svg, { Defs, LinearGradient, Rect, Stop } from "react-native-svg";
import { useRouter } from "expo-router";

import { typography, useRudiTheme, mauMocHanhTrinh } from "../theme";
import { RudiButton } from "../ui";
import { Stamp } from "../ui/Stamp";
import { ganTrangThai } from "./chieu";
import { BanDo } from "./BanDo";
import { gioTru } from "./duong";
import type { DoanDuongHanhTrinh, HanhTrinh, HoatDongHanhTrinh } from "./mo-hinh";
import { chuKhoangCach, chuThoiGian, tomTatHanhTrinh } from "./tom-tat";
import { kieuBanDo, nhanDoan, nhanMoc, veDenDau, type DoanBanDo, type MocBanDo, type NeoMoc } from "./kieu-ban-do";
import { useNetMuc } from "./net-muc";

import type { TrangThaiTuyen } from "./ke-hoach";

/** The day's anchors, from its settings (start / end / return to start). */
export type NeoNgay = { xuatPhat: string | null; ketThuc: string | null; veDiemDau: boolean };

import { useMotion } from "../ui/useMotion";
import { useNhuongChoNep } from "../nep/NepProvider";
import { Canh } from "../ui/art/Canh";
import { NenGiay } from "../ui/NenGiay";
import { MoNgang } from "../ui/MoNgang";
import { hinhTrangXe } from "../art/giay";

/** Depth of the torn edge; the page overlaps the map by twice this. */
const XE = 3;
import { KyHoa } from "../ui/art/KyHoa";
import { giuState } from "../../ui/a11y";

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
  dangTimCho = false,
  chonPhuongTien,
  nhuongNep = true,
  chuaXep,
  goiYGhim,
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
  /** The outing's own places are still being read: say so, never «chưa có». */
  dangTimCho?: boolean;
  /** The transport choice, shown in the page head beside what it changes. */
  chonPhuongTien?: ReactNode;
  /**
   * The map is on screen. A host that keeps it mounted under another view
   * (the outing's Lịch trình) passes false there, or Nếp stays away from a
   * screen with no map on it (QA UI-037).
   */
  nhuongNep?: boolean;
  /**
   * The outing's stops on none of its days, by name, and the person's way to
   * put them on this one. Without it a three-day outing showed every day empty
   * and told the person to add the places they had already added (QA UI-032).
   */
  chuaXep?: { ten: readonly string[]; onXep: () => void };
  /** How to drop a pin on this map; said only while the map is on screen. */
  goiYGhim?: string;
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
  useNhuongChoNep(nhuongNep);
  // Room for half a stamp (24) above the web attribution strip (≈26) at the
  // bottom, and below the 44dp «Khớp hành trình» stamp at the top: a fitted
  // stop is never under a control or the credit line.
  const padding = useMemo(() => ({ top: 64, left: 40, right: 40, bottom: 64 }), []);

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
    const i = mocs.findIndex((m) => m.id === mocChon.id);
    const ke = [mocs[i - 1], mocs[i + 1]].filter((m): m is MocBanDo => m !== undefined).map((m) => ({ lat: m.lat, lng: m.lng }));
    return { lat: mocChon.lat, lng: mocChon.lng, dem: toiDem, ke };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- neighbours are read at the tick of toiDem
  }, [mocChon?.id, mocChon?.lat, mocChon?.lng, toiDem]);
  const trang = useRef<ScrollView>(null);
  // A chosen stop's card opens at the top of the page, where the strip was:
  // below the fold, choosing a stop looked like nothing happened.
  useEffect(() => { if (selectedActivityId) trang.current?.scrollTo({ y: 0, animated: false }); }, [selectedActivityId]);

  const coMoc = tom.soChangCoViTri > 0;
  // An empty day has nothing to fold away from: the fold row is not drawn
  // there, and a page folded on another day opens again.
  const gapLai = collapsed && coMoc;
  // An empty day in a short window: the day page takes all the room and the
  // map, which has nothing to show, steps out whole. A strip of map would
  // carry no credit line, which the map must never lose (B3 probe).
  const anBanDo = !coMoc && !dangTimCho && availableHeight < 560;
  const soChuaXep = chuaXep?.ten.length ?? 0;
  const tenChuaXep = chuaXep
    ? chuaXep.ten.length > 3 ? `${chuaXep.ten.slice(0, 3).join(", ")} và ${chuaXep.ten.length - 3} chặng khác` : chuaXep.ten.join(", ")
    : "";
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

  // «Nét mực tự vẽ»: a real route (never a draft) is drawn once when it
  // arrives, stamps pressing down as the ink reaches them. The key is the
  // route's identity, so switching to the suggestion draws the new line.
  const doanBanDo: (DoanBanDo & { tu: string; den: string })[] = useMemo(
    () =>
      doanHien.map((d) => {
        const chon = d.id === selectedSegmentId;
        const toi = chon ? hanh.activities.find((a) => a.id === d.toActivityId) : undefined;
        const roi = chon && toi && d.nguon !== "geodesic" ? gioTru(toi.gio, d.durationSeconds) : null;
        return {
          id: d.id,
          tu: d.fromActivityId,
          den: d.toActivityId,
          polyline: d.polyline,
          uocLuong: d.nguon === "geodesic",
          chon,
          nhan: nhanDoan(d),
          the: chon && d.nguon !== "geodesic"
            ? [`${chuKhoangCach(d.distanceMeters)} · ${chuThoiGian(d.durationSeconds)}`, ...(roi && toi ? [`Rời ${roi} để tới lúc ${toi.gio}`] : [])]
            : null,
        };
      }),
    [doanHien, selectedSegmentId, hanh.activities],
  );
  const khoaVe = doanHien.length > 0 && doanHien.every((d) => d.nguon !== "geodesic")
    ? doanHien.map((d) => { const cuoi = d.polyline[d.polyline.length - 1]; return `${d.id}:${d.polyline.length}:${cuoi ? `${cuoi.lat.toFixed(5)},${cuoi.lng.toFixed(5)}` : ""}`; }).join("|")
    : null;
  const [banDoSan, setBanDoSan] = useState(false);
  // The pen also waits for the stamps it presses: while the outing's places
  // are still being read the ink ran across a map with no stops on it
  // (emulator, 2026-09-29).
  const tienDo = useNetMuc(khoaVe, banDoSan && !dangTimCho);
  const dangVe = tienDo < 1 ? veDenDau(doanBanDo, tienDo) : null;
  const mocVe: MocBanDo[] = dangVe ? mocs.map((m) => ({ ...m, nhip: dangVe.daCham.has(m.id) ? "dong" : "cho" })) : mocs;
  const doanVe: DoanBanDo[] = dangVe ? dangVe.doan : doanBanDo.map(({ tu: _tu, den: _den, ...d }) => d);

  const dauTuyen =
    tuyen === "that" ? `ĐƯỜNG THẬT${phuongTien ? ` · ${phuongTien}` : ""}`
    : tuyen === "dangTinh" ? "ĐANG TÍNH ĐƯỜNG"
    : tuyen === "khongTinhDuoc" ? "CHƯA TÍNH ĐƯỜNG"
    : "NÉT NHÁP";

  return (
    <View onLayout={(e) => setAvailableHeight(e.nativeEvent.layout.height)} style={[styles.khung, wide && { flexDirection: "row" }]}>
      {/* Clipped: the map is as tall as this room, never hanging under the page. */}
      <View style={{ flex: 1, minHeight: 0, overflow: "hidden", display: anBanDo ? "none" : "flex" }}>
      <BanDo
        doan={doanVe}
        fitDem={fitDem}
        cameraKey={cameraKey ?? hanh.activities.map((a) => a.id).join("|")}
        padding={padding}
        fitPoints={fitPoints}
        duration={motion.ms("standard")}
        onGhim={onGhim}
        kieu={kieu}
        mau={mau}
        mocs={mocVe}
        onChonDoan={onChonDoan}
        onChonMoc={onChonMoc}
        onNen={onNen}
        onUserMove={onUserMove}
        onSan={() => setBanDoSan(true)}
        toi={toi}
      />
      <View pointerEvents="box-none" style={StyleSheet.absoluteFill}>
        {coMoc ? (
          <View pointerEvents="box-none" style={styles.hangNut}>
            {/* A 44dp stamp, not a labelled button: the label took a third
                of a phone's map and pushed the fit down (review, 2026-09-29). */}
            <Pressable
              accessibilityLabel="Khớp hành trình"
              accessibilityRole="button"
              hitSlop={4}
              onPress={onKhop}
              style={({ pressed }) => [styles.nutKhop, { backgroundColor: mau.giay, borderColor: mau.vien, boxShadow: pressed ? mau.bong : mau.bongCao }]}
            >
              <Ionicons color={mau.muc} name="scan-outline" size={22} />
            </Pressable>
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
          style={[styles.the, wide ? styles.theRong : styles.theHep, !coMoc && styles.theCoLai, { paddingBottom: 12 + chanDuoi, maxHeight: wide || Platform.OS !== "web" || !coMoc ? undefined : availableHeight * 0.6, width: wide ? 360 : undefined }]}
        >
          {/* The page head, three short rows: which day (and the fold), the
              day's numbers beside the stamp that says what the line is, and
              how the group gets around. At 390×844 a taller head left no
              room at all for the stop strip, the one list that names every
              stop (web, 2026-09-29). */}
          <View style={styles.dauTrang}>
            <Text accessibilityRole="header" numberOfLines={1} style={[typography.h2, styles.dauChu, { color: colors.ink }]}>{tieuDeTrang ?? "Trang ngày"}</Text>
            {coMoc ? <Stamp label={dauTuyen} tone={tuyen === "that" ? "accent" : "ink"} tilt={tuyen === "that" ? -2 : 0} testID="hanh-trinh-dau-tuyen" /> : null}
          </View>
          {coMoc ? (
            <View style={styles.hangSo}>
              <Text style={[typography.label, styles.so, styles.dauChu, { color: colors.inkSoft }]}>{tomChu}</Text>
              <Pressable accessibilityLabel={gapLai ? "Mở trang ngày" : "Thu gọn trang ngày"} accessibilityRole="button" aria-expanded={!gapLai} hitSlop={12} onPress={() => setCollapsed(!collapsed)} style={styles.nutGap}>
                <Text style={[typography.caption, { color: colors.accent }]}>{gapLai ? "Mở trang" : "Thu gọn"}</Text>
              </Pressable>
            </View>
          ) : null}
          {coMoc && !gapLai ? chonPhuongTien : null}
          {/* A fixed height on a phone: a card that grew on selection shrank
              the map after the camera had framed the stop, and the next stop
              ended up cut by the page edge (review, 2026-09-29).
              Never squeezed below one stop strip: the head and the button
              must not leave the list that names every stop with no height.
              Only this middle scrolls and only it is capped: capping the
              whole page pushed «Xem cách đi gọn hơn» under the gesture bar
              at font 1.3 (emulator, 2026-09-29), and on a phone the map
              keeps the rest -- about half the screen. A browser has no
              gesture bar and a page cap that holds; there the page is capped
              and this middle takes what is left. */}
          {gapLai ? null : <ScrollView ref={trang} showsVerticalScrollIndicator={Platform.OS !== "web"} style={{ flexShrink: 1, minHeight: coMoc ? 84 * Math.min(fontScale, 1.3) : 0, ...(wide || Platform.OS === "web" || !coMoc ? {} : { height: Math.max(92 * Math.min(fontScale, 1.3), availableHeight * 0.2) }) }} keyboardShouldPersistTaps="handled" contentContainerStyle={{ gap: 8, paddingBottom: 8 }}>

          {mocChon ? (
            <TheMoc
              chang={doanToi(doanHien, mocChon.id)}
              moc={mocChon}
              onChiTiet={mocChon.placeId ? () => router.push(`/places/${mocChon.placeId}` as never) : undefined}
              truoc={changTruoc(hanh.activities, mocChon.id)}
            />
          ) : null}
          {coMoc ? <Text style={[typography.label, { color: colors.ink }]}>Các chặng trong ngày</Text> : null}
          {coMoc ? (
            <ThanhChang
              doc={wide}
              mocs={mocs}
              onChon={(id) => (id === selectedActivityId ? onNen() : onChonMoc(id))}
            />
          ) : null}
          {mocChon ? null : doanChon ? (
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
              {chuaXep && soChuaXep > 0 ? (
                <View style={styles.hangChuaXep}>
                  <Text style={[typography.note, styles.flex, { color: colors.inkSoft }]}>{soChuaXep} chặng chưa xếp ngày: {tenChuaXep}.</Text>
                  <RudiButton compact full={false} label="Xếp vào ngày này" onPress={chuaXep.onXep} variant="ghost" />
                </View>
              ) : null}
              {onToiUu ? <RudiButton compact disabled={dangToiUu} label="Xem cách đi gọn hơn" loading={dangToiUu} onPress={onToiUu} /> : null}
            </View>
          ) : dangTimCho ? (
            <Text accessibilityLiveRegion="polite" style={[typography.note, { color: colors.inkSoft }]}>Đang tìm các quán của kèo trên bản đồ…</Text>
          ) : (
            <View style={styles.khoiThe}>
              {/* The drawing only where the window has room for it: on a
                  short one it pushed the sentence and «Về Lịch trình» off the
                  page (QA UI-033, 390×460). */}
              {availableHeight >= 560 ? <Canh id="tim-khong-ra" width={144} /> : null}
              {chuaXep && soChuaXep > 0 ? (
                <>
                  <Text style={[typography.h2, { color: colors.ink }]}>Ngày này chưa có chặng nào</Text>
                  <Text accessibilityLiveRegion="polite" style={[typography.note, { color: colors.inkSoft }]}>
                    {soChuaXep} chặng của kèo chưa xếp ngày: {tenChuaXep}.
                  </Text>
                  <RudiButton label={soChuaXep === 1 ? "Xếp chặng này vào ngày này" : `Xếp cả ${soChuaXep} chặng vào ngày này`} onPress={chuaXep.onXep} />
                  {goiYGhim && !anBanDo ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{goiYGhim}</Text> : null}
                </>
              ) : (
                <>
                  <Text style={[typography.h2, { color: colors.ink }]}>Ngày này chưa có điểm nào trên bản đồ</Text>
                  <Text style={[typography.note, { color: colors.inkSoft }]}>
                    Gắn một quán hoặc một địa điểm vào lịch trình, đường đi sẽ hiện ở đây.
                  </Text>
                  {goiYGhim && !anBanDo ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{goiYGhim}</Text> : null}
                </>
              )}
            </View>
          )}
          {actions}
          {goiYGhim && coMoc ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{goiYGhim}</Text> : null}
          </ScrollView>}
          {/* The scroll's lower edge fades into the page, so a control cut by
              it reads as "more below", not as a broken half button. */}
          {/* Only over the stop list: on an empty day it faded out the
              explanation's last line (QA UI-033). */}
          {gapLai || wide || !coMoc ? null : (
            <View pointerEvents="none" style={styles.mo}>
              <Svg height="100%" width="100%">
                <Defs>
                  <LinearGradient id="mo-trang" x1="0" x2="0" y1="0" y2="1">
                    <Stop offset="0" stopColor={colors.card} stopOpacity={0} />
                    <Stop offset="0.7" stopColor={colors.card} stopOpacity={1} />
                  </LinearGradient>
                </Defs>
                <Rect fill="url(#mo-trang)" height="100%" width="100%" />
              </Svg>
            </View>
          )}
          {/* Always there on an empty day (QA UI-033), but quiet: the switch
              above says the same, and the day's own action leads (B3 finish
              review). */}
          {!coMoc && onVeLichTrinh ? (
            <RudiButton accessibilityLabel="Về Lịch trình" compact label="Về Lịch trình" onPress={onVeLichTrinh} variant="ghost" />
          ) : coMoc ? primaryAction : null}
        </NenGiay>
    </View>
  );
}

/**
 * The day's stops, in order: a scrollable rail on a phone, ruled rows of the
 * notebook page on a wide screen.
 *
 * This is the only element that names every stop: pins carry a number and no
 * accessible label, and two of them can land on the same pixel. Cells are not
 * cards -- the page is the card (DESIGN: no card inside a card); only the
 * chosen stop is marked, in the one lead tone.
 */
function ThanhChang({
  mocs,
  onChon,
  doc = false,
}: {
  mocs: readonly MocBanDo[];
  onChon: (id: string) => void;
  /** Wide page: one ruled row per stop instead of the rail. */
  doc?: boolean;
}) {
  const { colors, radius } = useRudiTheme();
  const rail = useRef<ScrollView>(null);
  const positions = useRef<Record<string, number>>({});
  const selected = mocs.find((m) => m.chon)?.id;
  useEffect(() => { if (selected && !doc) rail.current?.scrollTo({ x: Math.max(0, (positions.current[selected] ?? 0) - 16), animated: false }); }, [selected, doc]);
  const o = mocs.map((moc, i) => (
        <Pressable
          accessibilityLabel={nhanMoc(moc)}
          accessibilityRole="button"
          {...giuState(moc.chon)}
          key={moc.id}
          onLayout={(e) => { positions.current[moc.id] = e.nativeEvent.layout.x; }}
          onPress={() => onChon(moc.id)}
          style={[
            styles.chang,
            doc ? styles.changDoc : styles.changNgang,
            {
              backgroundColor: moc.chon ? colors.accentSoft : "transparent",
              borderColor: moc.chon ? colors.accent : "transparent",
              borderRadius: radius.small,
            },
            doc && !moc.chon && i < mocs.length - 1 ? { borderBottomColor: colors.line, borderRadius: 0 } : null,
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
            <Text numberOfLines={doc ? 2 : 1} style={[typography.label, { color: colors.ink }]}>
              {moc.tieuDe}
            </Text>
          </View>
        </Pressable>
  ));
  if (doc) return <View>{o}</View>;
  // Faded at its cut end into the page, so a stop cut by the edge reads as
  // "more" (B3 finish review: «Tối nu»).
  return (
    <View>
      <ScrollView
        ref={rail}
        contentContainerStyle={styles.thanhTrong}
        horizontal
        showsHorizontalScrollIndicator={false}
        style={styles.thanh}
      >
        {o}
      </ScrollView>
      <MoNgang mau={colors.card} />
    </View>
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
      {/* The place's sketch closes the card: first, it filled the whole
          short page and hid the name (412, 2026-09-29). */}
      {moc.category ? <KyHoa loai={moc.category} gon /> : null}
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
  nutKhop: { width: 44, height: 44, borderRadius: 8, borderWidth: 2, alignItems: "center", justifyContent: "center", transform: [{ rotate: "-2deg" }] },
  dan: { flex: 1 },
  the: { paddingHorizontal: 16, gap: 8 },
  // The torn edge and the holes (13dp in) sit over the map's last few dp.
  theHep: { marginTop: -XE * 2, paddingTop: XE * 2 + 12 },
  // An empty day's page takes the room it needs and, in a short window,
  // shrinks to it: the middle scrolls, the head and «Về Lịch trình» stay
  // (QA UI-033: uncapped and unshrinkable, the button left a 390×460 window).
  theCoLai: { flexShrink: 1, minHeight: 0 },
  theRong: { marginLeft: -XE * 2, paddingLeft: XE * 2 + 18, paddingTop: 8 },
  khoiThe: { gap: 6 },
  hangChuaXep: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", columnGap: 8 },
  flex: { flex: 1, minWidth: 160 },
  mo: { height: 40, marginTop: -48 },
  // To the page's own edge (its padding is 16), so a cut cell reads as "more".
  thanh: { marginHorizontal: -16, marginTop: -2 },
  thanhTrong: { paddingHorizontal: 16, gap: 8 },
  chang: { flexDirection: "row", alignItems: "center", gap: 8, paddingVertical: 8, paddingHorizontal: 10, borderWidth: 1 },
  changNgang: { maxWidth: 210 },
  changDoc: { minHeight: 56 },
  changSo: { minWidth: 26, height: 26, borderRadius: 6, borderWidth: 1.5, alignItems: "center", justifyContent: "center", paddingHorizontal: 4 },
  changSoChu: { fontSize: 13, fontWeight: "800", lineHeight: 16, fontVariant: ["tabular-nums"] },
  // Wraps rather than clips: at 800dp the stamp ran past the page edge.
  dauTrang: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", justifyContent: "space-between", columnGap: 12, rowGap: 4, minHeight: 44 },
  dauChu: { flexShrink: 1 },
  hangSo: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", gap: 10 },
  nutGap: { minHeight: 36, minWidth: 64, alignItems: "flex-end", justifyContent: "center" },
  so: { fontVariant: ["tabular-nums"] },
  changChu: { flexShrink: 1 },
});
