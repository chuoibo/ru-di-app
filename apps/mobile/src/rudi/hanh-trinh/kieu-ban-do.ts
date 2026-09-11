/** Shared map props: web MapLibre GL and native MapLibre RN consume the same shape. */

import type { ToaDo } from "./mo-hinh";

export const KIEU_BAN_DO = "https://tiles.openfreemap.org/styles/positron";

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
  mauNen: string;
  fitDem: number;
  toi: { lat: number; lng: number; dem: number } | null;
  onUserMove: () => void;
  onChonMoc: (id: string) => void;
  onChonDoan: (id: string) => void;
  onNen: () => void;
};

export const DEM_KHOP = { top: 56, left: 40, right: 40, bottom: 200 };

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
    properties: { id: string; chon: number };
    geometry: { type: "LineString"; coordinates: [number, number][] };
  }[];
};

export function tapHop(doan: readonly DoanBanDo[]): TapHopDuong {
  return {
    type: "FeatureCollection",
    features: doan.map((d) => ({
      type: "Feature",
      properties: { id: d.id, chon: d.chon ? 1 : 0 },
      geometry: {
        type: "LineString",
        coordinates: d.polyline.map((p) => [p.lng, p.lat]),
      },
    })),
  };
}
