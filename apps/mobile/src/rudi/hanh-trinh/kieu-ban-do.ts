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
      { id: "district-names", type: "symbol", ...source, "source-layer": "place", minzoom: 11, filter: ["in", ["get", "class"], ["literal", ["suburb", "quarter", "neighbourhood"]]], layout: { "text-field": name, "text-font": ["Noto Sans Regular"], "text-size": 13, "text-max-width": 8 }, paint: { "text-color": c.inkFaint, ...halo } },
      { id: "place-names", type: "symbol", ...source, "source-layer": "place", minzoom: 6, filter: ["in", ["get", "class"], ["literal", ["city", "town", "village"]]], layout: { "text-field": name, "text-font": ["Noto Sans Bold"], "text-size": 15, "text-max-width": 10 }, paint: { "text-color": c.ink, ...halo } },
    ],
  });
}

/** Light default, kept for callers that have no theme in hand. */
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
};

export type DoanBanDo = {
  id: string;
  polyline: ToaDo[];
  chon: boolean;
  uocLuong?: boolean;
  /** Short travel time drawn on the leg («9 phút»); only a routed leg has one. */
  nhan?: string | null;
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
  toi: { lat: number; lng: number; dem: number } | null;
  onUserMove: () => void;
  onChonMoc: (id: string) => void;
  onChonDoan: (id: string) => void;
  onNen: () => void;
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
    properties: { id: string; chon: number; mo: number; uocLuong: number; thuTu: number; nhan: string };
    geometry: { type: "LineString"; coordinates: [number, number][] };
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
  return {
    type: "FeatureCollection",
    features: doan.map((d, i) => ({
      type: "Feature",
      properties: {
        id: d.id,
        chon: d.chon ? 1 : 0,
        mo: coChon && !d.chon ? 1 : 0,
        uocLuong: d.uocLuong ? 1 : 0,
        thuTu: i,
        nhan: d.uocLuong ? "" : (d.nhan ?? ""),
      },
      geometry: {
        type: "LineString",
        coordinates: d.polyline.map((p) => [p.lng, p.lat]),
      },
    })),
  };
}

/** Fit the actual travelled geometry, including detours beyond the stops. */
export function hopHanhTrinh(mocs: BanDoProps["mocs"], doan: BanDoProps["doan"]) {
  return hopGioi([...mocs, ...doan.flatMap((leg) => leg.polyline)]);
}

/** Sparse directional chevrons follow real geometry. */
export function muiTenDoan(doan: readonly DoanBanDo[]) {
  return doan.flatMap((leg) => {
    if (leg.uocLuong || leg.polyline.length < 2) return [];
    const i = Math.max(1, Math.floor(leg.polyline.length / 2));
    const a = leg.polyline[i - 1]; const b = leg.polyline[i];
    const heading = Math.atan2((b.lng - a.lng) * Math.cos(a.lat * Math.PI / 180), b.lat - a.lat) * 180 / Math.PI;
    return [{ id: leg.id, lat: b.lat, lng: b.lng, heading }];
  });
}

/**
 * Layer paint shared by web and native, so the two platforms cannot drift:
 * the paper casing, the ink line, the pencil draft and the time labels.
 */
export function lopDuong(mau: MauBanDo) {
  const chon = ["==", ["get", "chon"], 1];
  const mo = ["==", ["get", "mo"], 1];
  return {
    vien: {
      filter: ["!=", ["get", "uocLuong"], 1],
      layout: { "line-cap": "round", "line-join": "round" },
      paint: { "line-color": mau.giay, "line-opacity": 0.95, "line-width": ["case", chon, 13, 10] },
    },
    duong: {
      filter: ["!=", ["get", "uocLuong"], 1],
      layout: { "line-cap": "round", "line-join": "round" },
      paint: {
        "line-color": mau.muc,
        "line-opacity": ["case", mo, 0.42, 1],
        "line-width": ["case", chon, 7, 5],
      },
    },
    nhap: {
      filter: ["==", ["get", "uocLuong"], 1],
      layout: { "line-cap": "round" },
      paint: { "line-color": mau.netChi, "line-width": 2.5, "line-dasharray": [1.2, 2.2], "line-opacity": ["case", mo, 0.45, 0.9] },
    },
    nhan: {
      filter: ["all", ["!=", ["get", "uocLuong"], 1], ["!=", ["get", "nhan"], ""]],
      minzoom: 12.5,
      layout: {
        "symbol-placement": "line-center",
        "text-field": ["get", "nhan"],
        "text-font": ["Noto Sans Bold"],
        "text-size": 12,
        "text-allow-overlap": false,
        "text-padding": 6,
      },
      paint: { "text-color": mau.chu, "text-halo-color": mau.giay, "text-halo-width": 2.4 },
    },
  } as const;
}
