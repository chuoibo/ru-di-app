/** Journey summary: integer metres, computed efficiency or nothing. */

import { giayTuMet, haversineMet, type ToaDo } from "./khoang";
import type { DoanDuongHanhTrinh, HoatDongHanhTrinh } from "./mo-hinh";
import { toiUuGanNhat } from "./toi-uu";

export { giayTuMet, haversineMet } from "./khoang";

export type TomTatHanhTrinh = {
  soChang: number;
  soChangCoViTri: number;
  met: number | null;
  giay: number | null;
  hieuSuat: number | null;
};

function tourMet(diem: readonly ToaDo[]): number {
  let tong = 0;
  for (let i = 1; i < diem.length; i++) tong += haversineMet(diem[i - 1], diem[i]);
  return tong;
}

/** Integer percent: (nearest-neighbour length / current length) × 100. Null if < 2 points. */
export function hieuSuatTuyen(diem: readonly ToaDo[]): number | null {
  if (diem.length < 2) return null;
  const hienTai = tourMet(diem);
  if (hienTai === 0) return 100;
  const ganNhat = toiUuGanNhat(diem.map((p, i) => ({ id: String(i), lat: p.lat, lng: p.lng })));
  const theoNn = ganNhat.map((id) => diem[Number(id)]);
  return Math.round((tourMet(theoNn) / hienTai) * 100);
}

export function tomTatHanhTrinh(
  activities: readonly HoatDongHanhTrinh[],
  routeSegments: readonly DoanDuongHanhTrinh[],
): TomTatHanhTrinh {
  const coViTri = activities.filter((a) => a.lat !== null && a.lng !== null);
  const met = routeSegments.length === 0 ? null : routeSegments.reduce((t, d) => t + d.distanceMeters, 0);
  const giay = met === null ? null : routeSegments.reduce((t, d) => t + d.durationSeconds, 0);
  const diem = coViTri.map((a) => ({ lat: a.lat as number, lng: a.lng as number }));
  return {
    soChang: activities.length,
    soChangCoViTri: coViTri.length,
    met,
    giay,
    hieuSuat: hieuSuatTuyen(diem),
  };
}

export function chuKhoangCach(met: number): string {
  if (met < 1000) return `${met} m`;
  const km = Math.round(met / 100) / 10;
  return `${String(km).replace(".", ",")} km`;
}

export function chuThoiGian(giay: number): string {
  const phut = Math.max(1, Math.round(giay / 60));
  if (phut < 60) return `${phut} phút`;
  const gio = Math.floor(phut / 60);
  const du = phut % 60;
  return du === 0 ? `${gio} giờ` : `${gio} giờ ${du} phút`;
}
