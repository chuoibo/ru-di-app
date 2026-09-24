/** Shared itinerary projection: one day of stops, two views. */

export type TrangThaiChang = "xong" | "hien-tai" | "sap-toi";

export type PhuongTien = "walk" | "bike" | "motorbike" | "car" | "transit";

export type ToaDo = { lat: number; lng: number };

export type HoatDongHanhTrinh = {
  id: string;
  /** 1-based map number, or null when the stop has no coordinates. */
  so: number | null;
  gio: string;
  tieuDe: string;
  tenDiaDiem: string | null;
  diaChi: string | null;
  category?: string;
  placeId: string | null;
  lat: number | null;
  lng: number | null;
};

export type DoanDuongHanhTrinh = {
  id: string;
  fromActivityId: string;
  toActivityId: string;
  distanceMeters: number;
  durationSeconds: number;
  transportMode: PhuongTien;
  polyline: ToaDo[];
  nguon: "osrm" | "geodesic" | "valhalla";
};

export type HanhTrinh = {
  activities: HoatDongHanhTrinh[];
  routeSegments: DoanDuongHanhTrinh[];
};

export type ChoChieu = {
  id: string;
  name: string;
  /** Null together, or not at all: a quarter of the catalogue has no
   *  coordinates. Widened rather than filtered, because this same list feeds
   *  the place search when planning an outing, and dropping those places would
   *  make a quarter of the catalogue impossible to add to a plan. The map side
   *  already checks `Number.isFinite` before drawing anything. */
  lat: number | null;
  lng: number | null;
  address?: string | null;
  category?: string;
};

export type SlotChieu = {
  time: string;
  title: string;
  placeId?: string;
};

export type NgayChieu = {
  day: string;
  items: SlotChieu[];
};

export type ChangChieu = {
  meeting_point?: {lat:number;lng:number;label:string} | null;
  id: string;
  at: string;
  label: string;
  place_name: string | null;
  place_id: string | null;
};
