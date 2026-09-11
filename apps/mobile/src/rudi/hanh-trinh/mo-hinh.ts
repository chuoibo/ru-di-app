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
  nguon: "osrm" | "geodesic";
};

export type HanhTrinh = {
  activities: HoatDongHanhTrinh[];
  routeSegments: DoanDuongHanhTrinh[];
};

export type ChoChieu = {
  id: string;
  name: string;
  lat: number;
  lng: number;
  address?: string | null;
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
  id: string;
  at: string;
  label: string;
  place_name: string | null;
  place_id: string | null;
};
