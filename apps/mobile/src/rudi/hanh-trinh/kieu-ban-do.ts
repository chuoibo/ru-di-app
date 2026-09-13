/** Shared map props: web MapLibre GL and native MapLibre RN consume the same shape. */

import type { ToaDo } from "./mo-hinh";
import tokens from "../../../../../packages/shared/tokens.json";

/**
 * Basemap style per theme. OpenFreeMap, OSM data.
 *
 * A white Positron sheet under RuDi's navy dark theme read as two products
 * glued together; `fiord` is the blue-grey sibling that sits in the same
 * family as the dark palette's paper.
 */
export function kieuBanDo(toi: boolean): string {
  const c = toi ? tokens.color.dark : tokens.color.light;
  const source = { source: "openmaptiles" };
  const name = ["coalesce", ["get", "name:vi"], ["get", "name"], ["get", "name:latin"], ""];
  const text = { "text-color": c.inkSoft, "text-halo-color": c.ground, "text-halo-width": 1.5 };
  // An owned, sprite-free style: labels use actual OSM names and never depend
  // on a missing icon or the upstream style's bilingual-name assumptions.
  return JSON.stringify({
    version: 8,
    name: "Rủ Đi · giấy và đường",
    glyphs: "https://tiles.openfreemap.org/fonts/{fontstack}/{range}.pbf",
    sources: { openmaptiles: { type: "vector", url: "https://tiles.openfreemap.org/planet", attribution: '© <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> · <a href="https://openfreemap.org">OpenFreeMap</a>' } },
    layers: [
      { id: "paper", type: "background", paint: { "background-color": c.ground } },
      { id: "green", type: "fill", ...source, "source-layer": "landcover", paint: { "fill-color": c.paperShade, "fill-opacity": 0.35 } },
      { id: "water", type: "fill", ...source, "source-layer": "water", paint: { "fill-color": c.paperShade } },
      { id: "waterway", type: "line", ...source, "source-layer": "waterway", paint: { "line-color": c.paperShade, "line-width": 2 } },
      { id: "buildings", type: "fill", ...source, "source-layer": "building", minzoom: 14, paint: { "fill-color": c.line, "fill-opacity": 0.65 } },
      { id: "road-edge", type: "line", ...source, "source-layer": "transportation", filter: ["!=", "class", "rail"], layout: { "line-cap": "round", "line-join": "round" }, paint: { "line-color": c.line, "line-width": ["interpolate", ["linear"], ["zoom"], 10, 1, 16, 8] } },
      { id: "roads", type: "line", ...source, "source-layer": "transportation", filter: ["!=", "class", "rail"], layout: { "line-cap": "round", "line-join": "round" }, paint: { "line-color": c.card, "line-width": ["interpolate", ["linear"], ["zoom"], 10, 0.5, 16, 5] } },
      { id: "road-names", type: "symbol", ...source, "source-layer": "transportation_name", minzoom: 12, layout: { "symbol-placement": "line", "text-field": name, "text-font": ["Noto Sans Regular"], "text-size": 12, "symbol-spacing": 250 }, paint: text },
      { id: "water-names", type: "symbol", ...source, "source-layer": "water_name", layout: { "text-field": name, "text-font": ["Noto Sans Italic"], "text-size": 14, "text-max-width": 9 }, paint: text },
      { id: "place-names", type: "symbol", ...source, "source-layer": "place", minzoom: 6, filter: ["in", "class", "city", "town", "village", "suburb", "neighbourhood"], layout: { "text-field": name, "text-font": ["Noto Sans Regular"], "text-size": 14, "text-max-width": 10 }, paint: { ...text, "text-color": c.ink } },
    ],
  });
}

/** Light default, kept for callers that have no theme in hand. */
export const KIEU_BAN_DO = kieuBanDo(false);

export type MocBanDo = {
  id: string;
  so: number;
  lat: number;
  lng: number;
  tieuDe: string;
  gio: string;
  chon: boolean;
};

export type DoanBanDo = {
  id: string;
  polyline: ToaDo[];
  chon: boolean;
  uocLuong?: boolean;
};

export type BanDoProps = {
  mocs: MocBanDo[];
  doan: DoanBanDo[];
  mauMoc: readonly string[];
  mauMocInk: string;
  mauMocChon: string;
  mauDuong: string;
  mauDuongMo: string;
  mauVien: string;
  /** Casing under the route line: the journey reads as a drawn path, not a road. */
  mauVienDuong: string;
  mauNen: string;
  /** Basemap style URL; the host picks it from the theme. */
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
    properties: { id: string; chon: number; uocLuong: number; thuTu: number };
    geometry: { type: "LineString"; coordinates: [number, number][] };
  }[];
};

export function tapHop(doan: readonly DoanBanDo[]): TapHopDuong {
  return {
    type: "FeatureCollection",
    features: doan.map((d, i) => ({
      type: "Feature",
      properties: { id: d.id, chon: d.chon ? 1 : 0, uocLuong: d.uocLuong ? 1 : 0, thuTu: i },
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
