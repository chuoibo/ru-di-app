/** Shared map props: web MapLibre GL and native MapLibre RN consume the same shape. */

import type { ToaDo, TrangThaiChang } from "./mo-hinh";
import { chuThoiGian } from "./tom-tat";
import tokens from "../../../../../packages/shared/tokens.json";

/**
 * Basemap style per theme: RuDi's own sheet over OpenFreeMap vector tiles.
 *
 * The map is the table the day page lies on, so it stays quiet: paper ground,
 * water and parks as a slightly darker wash, streets as white paper strips on
 * a pencil edge. Roads keep a real hierarchy -- an arterial is visibly wider
 * than an alley -- because "which big road do we take" is how a group reads a
 * route. No third-party POIs: the only places on this map are the group's.
 * Sprite-free, and every colour comes from `tokens.json`.
 */
export function kieuBanDo(toi: boolean): string {
  const c = toi ? tokens.color.dark : tokens.color.light;
  const source = { source: "openmaptiles" };
  const name = ["coalesce", ["get", "name:vi"], ["get", "name"], ["get", "name:latin"], ""];
  const halo = { "text-halo-color": c.ground, "text-halo-width": 1.6 };
  // Three road tiers, each a pencil edge under a paper strip. Width at z10 and
  // z17; minor streets only from z13, where they help read a neighbourhood.
  const TANG = [
    { id: "lon", lop: ["motorway", "trunk"], z10: [2.6, 1.6], z17: [22, 17], canh: c.lineStrong, canhMo: 0.5 },
    { id: "chinh", lop: ["primary", "secondary"], z10: [1.8, 1], z17: [17, 13], canh: c.lineStrong, canhMo: 0.4 },
    { id: "pho", lop: ["tertiary", "minor", "service"], z10: [0.8, 0.3], z17: [11, 8], canh: c.line, canhMo: 1, minzoom: 12.5 },
  ] as const;
  const be = (z10: number, z17: number) => ["interpolate", ["exponential", 1.5], ["zoom"], 10, z10, 17, z17];
  const duong = TANG.flatMap((t) => {
    const loc = ["in", ["get", "class"], ["literal", [...t.lop]]];
    const zoom = "minzoom" in t ? { minzoom: t.minzoom } : {};
    const layout = { "line-cap": "round", "line-join": "round" };
    return [
      { id: `canh-${t.id}`, type: "line", ...source, "source-layer": "transportation", ...zoom, filter: loc, layout, paint: { "line-color": t.canh, "line-opacity": t.canhMo, "line-width": be(t.z10[0], t.z17[0]) } },
      { id: `duong-${t.id}`, type: "line", ...source, "source-layer": "transportation", ...zoom, filter: loc, layout, paint: { "line-color": c.card, "line-width": be(t.z10[1], t.z17[1]) } },
    ];
  });
  // Casings first, then all paper strips, so a small street joining an
  // arterial reads as one road network rather than stacked tubes.
  const duongXep = [...duong.filter((l) => l.id.startsWith("canh-")).reverse(), ...duong.filter((l) => l.id.startsWith("duong-")).reverse()];
  return JSON.stringify({
    version: 8,
    name: "Rủ Đi · giấy và đường",
    glyphs: "https://tiles.openfreemap.org/fonts/{fontstack}/{range}.pbf",
    sources: { openmaptiles: { type: "vector", url: "https://tiles.openfreemap.org/planet", attribution: '© <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> · <a href="https://openfreemap.org">OpenFreeMap</a>' } },
    layers: [
      { id: "paper", type: "background", paint: { "background-color": c.ground } },
      { id: "residential", type: "fill", ...source, "source-layer": "landuse", minzoom: 11, filter: ["in", ["get", "class"], ["literal", ["residential", "suburb", "neighbourhood"]]], paint: { "fill-color": c.paperShade, "fill-opacity": 0.22 } },
      { id: "green", type: "fill", ...source, "source-layer": "landcover", paint: { "fill-color": c.paperShade, "fill-opacity": 0.4 } },
      { id: "park", type: "fill", ...source, "source-layer": "park", paint: { "fill-color": c.paperShade, "fill-opacity": 0.6 } },
      { id: "water", type: "fill", ...source, "source-layer": "water", paint: { "fill-color": c.paperShade } },
      { id: "waterway", type: "line", ...source, "source-layer": "waterway", paint: { "line-color": c.paperShade, "line-width": ["interpolate", ["linear"], ["zoom"], 10, 1, 16, 4] } },
      { id: "buildings", type: "fill", ...source, "source-layer": "building", minzoom: 14, paint: { "fill-color": c.line, "fill-opacity": 0.55 } },
      ...duongXep,
      { id: "road-names", type: "symbol", ...source, "source-layer": "transportation_name", minzoom: 13, filter: ["in", ["get", "class"], ["literal", ["trunk", "primary", "secondary", "tertiary", "minor"]]], layout: { "symbol-placement": "line", "text-field": name, "text-font": ["Noto Sans Regular"], "text-size": 12, "symbol-spacing": 280 }, paint: { "text-color": c.inkSoft, ...halo } },
      { id: "water-names", type: "symbol", ...source, "source-layer": "water_name", layout: { "text-field": name, "text-font": ["Noto Sans Italic"], "text-size": 13, "text-max-width": 9 }, paint: { "text-color": c.inkFaint, ...halo } },
      // Wards and districts only: OSM's `neighbourhood` in Vietnamese cities is
      // «Khu phố 37», «Khu phố 57»… and covered the map at street zoom.
      { id: "district-names", type: "symbol", ...source, "source-layer": "place", minzoom: 11, filter: ["in", ["get", "class"], ["literal", ["suburb", "quarter"]]], layout: { "text-field": name, "text-font": ["Noto Sans Regular"], "text-size": 13, "text-max-width": 8 }, paint: { "text-color": c.inkFaint, ...halo } },
      { id: "place-names", type: "symbol", ...source, "source-layer": "place", minzoom: 6, filter: ["in", ["get", "class"], ["literal", ["city", "town", "village"]]], layout: { "text-field": name, "text-font": ["Noto Sans Bold"], "text-size": 15, "text-max-width": 10 }, paint: { "text-color": c.ink, ...halo } },
    ],
  });
}

/** Light default, kept for callers that have no theme in hand. */
/**
 * Where an empty map opens: central Ho Chi Minh City, where the live
 * catalogue is. A day with any mapped stop fits to its stops instead.
 */
export const TAM_MAC_DINH = { lat: 10.7769, lng: 106.7009 };

export const KIEU_BAN_DO = kieuBanDo(false);

/** Which end of the day a stop anchors, when the day says so. */
export type NeoMoc = "xuat-phat" | "ket-thuc" | "ve" | null;

export type MocBanDo = {
  id: string;
  so: number;
  lat: number;
  lng: number;
  tieuDe: string;
  gio: string;
  chon: boolean;
  /** Null before the outing starts: no stop is "next" on a day not yet begun. */
  trangThai: TrangThaiChang | null;
  neo: NeoMoc;
  /**
   * While the ink is being drawn: `cho` = still waiting above the page,
   * `dong` = the ink just reached it, press it down. Null when nothing plays.
   */
  nhip?: "cho" | "dong" | null;
};

export type DoanBanDo = {
  id: string;
  polyline: ToaDo[];
  chon: boolean;
  uocLuong?: boolean;
  /** Short travel time drawn on the leg («9 phút»); only a routed leg has one. */
  nhan?: string | null;
  /** Lines of the paper tag pinned on the chosen leg, e.g. «1,9 km · 3 phút». */
  the?: string[] | null;
};

/** Every colour the map draws with, resolved from the theme by the host. */
export type MauBanDo = {
  /** Paper face of a stamp and the casing under the ink line. */
  giay: string;
  /** Pencil edge of an ordinary stamp. */
  vien: string;
  /** The one lead tone: route ink, stamp numbers, the next stop. */
  muc: string;
  /** Text on a filled coral stamp. */
  mucTrenMuc: string;
  /** A stop already reached: faded to pencil, never to opacity. */
  mo: string;
  /** Plain ink for labels. */
  chu: string;
  /** The draft: a broken pencil line that shows order, never a road. */
  netChi: string;
  /** `boxShadow` of a stamp pasted on the map (paper height 1). */
  bong: string;
  /** `boxShadow` of the chosen or next stamp, lifted off the page (height 2). */
  bongCao: string;
  nen: string;
};

/** How one stamp pin is drawn; web and native both paint exactly this. */
export type HinhTem = {
  co: number;
  nen: string;
  vien: string;
  doVien: number;
  so: string;
  bong: string;
  /** Degrees; stamps lean a little, alternately, like paper pressed by hand. */
  nghieng: number;
  dauTick: boolean;
};

/**
 * The stamp for one stop.
 *
 * - upcoming: paper stamp, pencil edge, coral number;
 * - next stop (`hien-tai`): filled coral, lifted;
 * - reached (`xong`): pencil number and edge plus a tick -- faded by colour and
 *   mark, never by opacity (ADR-0038);
 * - chosen: larger, coral edge, lifted, upright.
 * Size stays ≥ 44dp so the stamp plus its hit slop is a 48dp target.
 */
export function hinhTem(moc: Pick<MocBanDo, "so" | "chon" | "trangThai">, mau: MauBanDo): HinhTem {
  const hienTai = moc.trangThai === "hien-tai";
  const xong = moc.trangThai === "xong";
  return {
    co: moc.chon ? 52 : 44,
    nen: hienTai ? mau.muc : mau.giay,
    vien: moc.chon ? mau.muc : hienTai ? mau.muc : mau.vien,
    doVien: moc.chon ? 3 : 2,
    so: hienTai ? mau.mucTrenMuc : xong ? mau.mo : mau.muc,
    bong: moc.chon || hienTai ? mau.bongCao : mau.bong,
    nghieng: moc.chon ? 0 : moc.so % 2 === 0 ? 3 : -3,
    dauTick: xong,
  };
}

export type BanDoProps = {
  mocs: MocBanDo[];
  doan: DoanBanDo[];
  mau: MauBanDo;
  /** Basemap style JSON; the host picks it from the theme. */
  kieu: string;
  fitDem: number;
  cameraKey?: string;
  fitPoints?: ToaDo[];
  padding?: { top: number; left: number; right: number; bottom: number };
  duration?: number;
  onGhim?: (point: ToaDo) => void;
  /** The chosen stop, and its neighbours in the day so the camera shows it within its day. */
  toi: { lat: number; lng: number; dem: number; ke?: ToaDo[] } | null;
  onUserMove: () => void;
  onChonMoc: (id: string) => void;
  onChonDoan: (id: string) => void;
  onNen: () => void;
  /** The basemap has loaded and can be seen: the ink may start. */
  onSan?: () => void;
};

export const DEM_KHOP = { top: 56, left: 40, right: 40, bottom: 260 };

/** The tick on a reached stop, drawn (never a glyph) in a 20×20 box. */
export const DUONG_TICK = "M5.5 10.5 L8.8 13.8 L14.8 6.6";

/** Words for an anchored end of the day, printed as a small stamp under the pin. */
export function chuNeo(neo: NeoMoc): string | null {
  if (neo === "xuat-phat") return "XUẤT PHÁT";
  if (neo === "ket-thuc") return "KẾT THÚC";
  if (neo === "ve") return "XUẤT PHÁT · VỀ";
  return null;
}

/**
 * What a screen reader hears for one pin: the number, the hour, the name, and
 * every state the stamp shows by shape -- a sighted person reads «đã tới» from
 * the faded stamp and the tick, so the label has to say it in words.
 */
export function nhanMoc(moc: Pick<MocBanDo, "so" | "gio" | "tieuDe" | "trangThai" | "neo">): string {
  const phan = [`Mốc ${moc.so}`, moc.gio, moc.tieuDe];
  if (moc.trangThai === "xong") phan.push("đã tới");
  if (moc.trangThai === "hien-tai") phan.push("điểm tiếp theo");
  const neo = chuNeo(moc.neo);
  if (neo) phan.push(neo.toLocaleLowerCase("vi"));
  return phan.join(", ");
}

/** The time printed on a routed leg; a draft leg gets nothing, since it is not a road. */
export function nhanDoan(doan: { nguon: string; durationSeconds: number }): string | null {
  return doan.nguon === "geodesic" ? null : chuThoiGian(doan.durationSeconds);
}

export function hopGioi(mocs: readonly { lat: number; lng: number }[]): [number, number, number, number] | null {
  if (mocs.length === 0) return null;
  let west = mocs[0].lng;
  let east = mocs[0].lng;
  let south = mocs[0].lat;
  let north = mocs[0].lat;
  for (const m of mocs) {
    west = Math.min(west, m.lng);
    east = Math.max(east, m.lng);
    south = Math.min(south, m.lat);
    north = Math.max(north, m.lat);
  }
  if (west === east) {
    west -= 0.012;
    east += 0.012;
  }
  if (south === north) {
    south -= 0.012;
    north += 0.012;
  }
  return [west, south, east, north];
}

export type TapHopDuong = {
  type: "FeatureCollection";
  features: {
    type: "Feature";
    properties: { id: string; chon: number; mo: number; uocLuong: number; thuTu: number; nhan: string; trung?: number };
    geometry: { type: "LineString"; coordinates: [number, number][] } | { type: "Point"; coordinates: [number, number] };
  }[];
};

/**
 * The legs as one GeoJSON source.
 *
 * `mo` (faded) is set only while ANOTHER leg is selected: with nothing chosen
 * the plan is one continuous ink line, and fading every leg by default made
 * the group's own route read like one more road on the basemap.
 */
export function tapHop(doan: readonly DoanBanDo[]): TapHopDuong {
  const coChon = doan.some((d) => d.chon);
  const net = doan.flatMap((d, i) => d.polyline.length < 2 ? [] : [{
      type: "Feature" as const,
      properties: {
        id: d.id,
        chon: d.chon ? 1 : 0,
        mo: coChon && !d.chon ? 1 : 0,
        uocLuong: d.uocLuong ? 1 : 0,
        thuTu: i,
        nhan: d.uocLuong ? "" : (d.nhan ?? ""),
        trung: d.uocLuong && quayLai(doan, i) ? 1 : 0,
      },
      geometry: {
        type: "LineString" as const,
        coordinates: d.polyline.map((p): [number, number] => [p.lng, p.lat]),
      },
    }]);
  // Each routed leg's minutes sit on a point halfway along it. A label placed
  // along the line lost to the basemap's street names and to curves: one leg
  // in three was labelled on a tablet, none on a phone (review, 2026-09-29).
  const nhan = net.flatMap((f) => {
    if (!f.properties.nhan) return [];
    const giua = giuaDoan(doan[f.properties.thuTu].polyline);
    return giua ? [{ ...f, geometry: { type: "Point" as const, coordinates: [giua.lng, giua.lat] as [number, number] } }] : [];
  });
  return { type: "FeatureCollection", features: [...net, ...nhan] };
}

/**
 * A draft leg that walks back along an earlier draft leg (1 → 2, then 2 → 3
 * with 3 beside 1): two dashed strokes a few px apart read as a railway on a
 * wide screen (web 1280, 2026-09-29). The pencil draws the way once.
 */
function quayLai(doan: readonly DoanBanDo[], i: number): boolean {
  const gan = (a: ToaDo | undefined, b: ToaDo | undefined) => !!a && !!b && Math.abs(a.lat - b.lat) < 0.002 && Math.abs(a.lng - b.lng) < 0.002;
  const d = doan[i];
  const dau = d.polyline[0];
  const cuoi = d.polyline[d.polyline.length - 1];
  return doan.slice(0, i).some((t) => t.uocLuong && gan(t.polyline[0], cuoi) && gan(t.polyline[t.polyline.length - 1], dau));
}

/** Fit the actual travelled geometry, including detours beyond the stops. */
export function hopHanhTrinh(mocs: BanDoProps["mocs"], doan: BanDoProps["doan"]) {
  return hopGioi([...mocs, ...doan.flatMap((leg) => leg.polyline)]);
}

/** Sparse directional chevrons follow real geometry. */
export function muiTenDoan(doan: readonly DoanBanDo[]) {
  return doan.flatMap((leg) => {
    if (leg.uocLuong || leg.polyline.length < 2) return [];
    // A third of the way along: halfway is where the leg's minutes sit.
    const dau = catDuong(leg.polyline, 0.33);
    const a = dau[dau.length - 2] ?? leg.polyline[0]; const b = dau[dau.length - 1] ?? leg.polyline[1];
    const heading = Math.atan2((b.lng - a.lng) * Math.cos(a.lat * Math.PI / 180), b.lat - a.lat) * 180 / Math.PI;
    return [{ id: leg.id, lat: b.lat, lng: b.lng, heading }];
  });
}

/**
 * Layer paint shared by web and native, so the two platforms cannot drift:
 * the paper casing, the ink line, the pencil draft and the time labels.
 */
export function lopDuong(mau: MauBanDo) {
  // Line layers draw the legs only; the label points are for `nhan`.
  const DUONG = ["==", ["geometry-type"], "LineString"];
  const chon = ["==", ["get", "chon"], 1];
  const mo = ["==", ["get", "mo"], 1];
  return {
    vien: {
      filter: ["all", DUONG, ["!=", ["get", "uocLuong"], 1]],
      layout: { "line-cap": "round", "line-join": "round" },
      paint: { "line-color": mau.giay, "line-opacity": 0.95, "line-width": ["case", chon, 13, 10] },
    },
    duong: {
      filter: ["all", DUONG, ["!=", ["get", "uocLuong"], 1]],
      layout: { "line-cap": "round", "line-join": "round" },
      paint: {
        "line-color": mau.muc,
        "line-opacity": ["case", mo, 0.42, 1],
        "line-width": ["case", chon, 7, 5],
      },
    },
    nhap: {
      filter: ["all", DUONG, ["==", ["get", "uocLuong"], 1], ["!=", ["get", "trung"], 1]],
      layout: { "line-cap": "round" },
      paint: { "line-color": mau.netChi, "line-width": 2.5, "line-dasharray": [1.2, 2.2], "line-opacity": ["case", mo, 0.45, 0.9] },
    },
    nhan: {
      // The chosen leg carries its paper tag instead of the small label. The
      // label is ours to keep: it overlaps basemap names, never the reverse.
      filter: ["all", ["==", ["geometry-type"], "Point"], ["!=", ["get", "nhan"], ""], ["!=", ["get", "chon"], 1]],
      minzoom: 10,
      layout: {
        "symbol-placement": "point",
        "text-field": ["get", "nhan"],
        "text-font": ["Noto Sans Bold"],
        "text-size": 12,
        "text-offset": [0, -1.3],
        "text-allow-overlap": true,
        "text-ignore-placement": false,
      },
      paint: { "text-color": mau.chu, "text-halo-color": mau.giay, "text-halo-width": 2.4 },
    },
  } as const;
}

/* ---------------------------------------------------------------------------
 * «Nét mực tự vẽ»: the one authored moment of this screen. When a real route
 * arrives (or the suggestion replaces the current one) the ink runs along the
 * actual streets from the first stop to the last, and each stamp presses
 * down as the ink reaches it. Pure pieces here; the clock lives in net-muc.ts.
 * ------------------------------------------------------------------------ */

const R_DAT = 6_371_000;

function met(a: ToaDo, b: ToaDo): number {
  const r = Math.PI / 180;
  const dLat = (b.lat - a.lat) * r;
  const dLng = (b.lng - a.lng) * r;
  const h = Math.sin(dLat / 2) ** 2 + Math.cos(a.lat * r) * Math.cos(b.lat * r) * Math.sin(dLng / 2) ** 2;
  return 2 * R_DAT * Math.asin(Math.min(1, Math.sqrt(h)));
}

/** Length of a polyline in metres (float; this is drawing, not a displayed figure). */
export function doDai(polyline: readonly ToaDo[]): number {
  let tong = 0;
  for (let i = 1; i < polyline.length; i++) tong += met(polyline[i - 1], polyline[i]);
  return tong;
}

/** The first `phan` (0..1) of a polyline by length, cut mid-vertex when needed. */
export function catDuong(polyline: readonly ToaDo[], phan: number): ToaDo[] {
  if (phan >= 1 || polyline.length < 2) return [...polyline];
  if (phan <= 0) return [];
  const dich = doDai(polyline) * phan;
  const ra: ToaDo[] = [polyline[0]];
  let da = 0;
  for (let i = 1; i < polyline.length; i++) {
    const d = met(polyline[i - 1], polyline[i]);
    if (da + d >= dich) {
      const f = d === 0 ? 0 : (dich - da) / d;
      const a = polyline[i - 1];
      const b = polyline[i];
      ra.push({ lat: a.lat + (b.lat - a.lat) * f, lng: a.lng + (b.lng - a.lng) * f });
      return ra;
    }
    da += d;
    ra.push(polyline[i]);
  }
  return ra;
}

/**
 * The day at drawing progress `t` (0..1): legs in order, the ink cut at the
 * same share of the whole day's length, and which stops it has reached. A
 * draft (pencil) day is never drawn this way -- it is not a road.
 */
export function veDenDau(
  doan: readonly (DoanBanDo & { tu: string; den: string })[],
  t: number,
): { doan: DoanBanDo[]; daCham: Set<string> } {
  const daCham = new Set<string>();
  const dai = doan.map((d) => doDai(d.polyline));
  const tong = dai.reduce((a, b) => a + b, 0);
  if (t >= 1 || tong === 0) {
    for (const d of doan) { daCham.add(d.tu); daCham.add(d.den); }
    return { doan: doan.map(({ tu: _tu, den: _den, ...d }) => d), daCham };
  }
  let conLai = tong * Math.max(0, t);
  const ra: DoanBanDo[] = [];
  doan.forEach(({ tu, den, ...d }, i) => {
    if (conLai > 0 || i === 0) daCham.add(tu);
    const phan = dai[i] === 0 ? 1 : Math.min(1, conLai / dai[i]);
    if (phan >= 1) daCham.add(den);
    ra.push({ ...d, polyline: catDuong(d.polyline, phan), nhan: phan >= 1 ? d.nhan : null });
    conLai = Math.max(0, conLai - dai[i]);
  });
  return { doan: ra, daCham };
}

/** Where a leg's paper tag sits: halfway along its length, not between its ends. */
export function giuaDoan(polyline: readonly ToaDo[]): ToaDo | null {
  if (polyline.length === 0) return null;
  const nua = catDuong(polyline, 0.5);
  return nua[nua.length - 1] ?? polyline[0];
}

/**
 * A pen's pace: it sets down gently, runs evenly, lifts gently. An ease-out
 * curve drew four fifths of the line in the first 400ms, so the eye saw a
 * line appear, not a line being written (emulator, 2026-09-29). No overshoot.
 */
export function nhipVe(x: number): number {
  const t = Math.min(1, Math.max(0, x));
  return 0.5 - Math.cos(Math.PI * t) / 2;
}

/**
 * The face of a stamp that stands for several stops on one spot. It keeps the
 * most urgent state among them: the next stop stays coral even when a
 * neighbour shares its pixel, and an anchor keeps its XUẤT PHÁT / KẾT THÚC
 * tag. Erasing the state made a phone's «1 · 2» read as two plain stops
 * (review, 2026-09-29). Reached only when every one of them is.
 */
export function mocChum(nhom: readonly MocBanDo[]): MocBanDo {
  const trangThai = nhom.some((m) => m.trangThai === "hien-tai") ? "hien-tai"
    : nhom.every((m) => m.trangThai === "xong") ? "xong"
    : null;
  return { ...nhom[0], chon: false, trangThai, neo: nhom.find((m) => m.neo)?.neo ?? null,
    // It presses down when the ink reaches the last of them.
    nhip: nhom.some((m) => m.nhip === "cho") ? "cho" : nhom.some((m) => m.nhip) ? "dong" : null };
}
