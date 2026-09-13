/** Project a timeline day (fixture or live stops) onto the shared journey model. */

import { geodesic } from "./duong";
import { giayTuMet, haversineMet } from "./khoang";
import type {
  ChangChieu,
  ChoChieu,
  DoanDuongHanhTrinh,
  HanhTrinh,
  HoatDongHanhTrinh,
  NgayChieu,
  SlotChieu,
  TrangThaiChang,
} from "./mo-hinh";

export function idSlot(slot: SlotChieu, _index: number): string {
  return slot.placeId ?? `gio:${slot.time}:${slot.title}`;
}

function choHopLe(place: ChoChieu | undefined): place is ChoChieu {
  return place !== undefined && Number.isFinite(place.lat) && Number.isFinite(place.lng);
}

function doanGiua(from: HoatDongHanhTrinh, to: HoatDongHanhTrinh): DoanDuongHanhTrinh | null {
  if (from.lat === null || from.lng === null || to.lat === null || to.lng === null) return null;
  const a = { lat: from.lat, lng: from.lng };
  const b = { lat: to.lat, lng: to.lng };
  const met = haversineMet(a, b);
  return {
    id: `${from.id}->${to.id}`,
    fromActivityId: from.id,
    toActivityId: to.id,
    distanceMeters: met,
    durationSeconds: giayTuMet(met),
    transportMode: "motorbike",
    polyline: geodesic(a, b),
    nguon: "geodesic",
  };
}

function danhSoVaNoi(activities: HoatDongHanhTrinh[]): HanhTrinh {
  let so = 0;
  for (const a of activities) {
    if (a.lat !== null && a.lng !== null) {
      so += 1;
      a.so = so;
    }
  }
  const mapped = activities.filter((a) => a.lat !== null && a.lng !== null);
  const routeSegments: DoanDuongHanhTrinh[] = [];
  for (let i = 1; i < mapped.length; i++) {
    const doan = doanGiua(mapped[i - 1], mapped[i]);
    if (doan) routeSegments.push(doan);
  }
  return { activities, routeSegments };
}

export function chieuTuNgay(ngay: NgayChieu, places: readonly ChoChieu[]): HanhTrinh {
  const theoId = new Map(places.map((p) => [p.id, p]));
  const activities: HoatDongHanhTrinh[] = ngay.items.map((item, index) => {
    const place = item.placeId ? theoId.get(item.placeId) : undefined;
    const hop = choHopLe(place);
    return {
      id: idSlot(item, index),
      so: null,
      gio: item.time,
      tieuDe: item.title,
      tenDiaDiem: hop ? place.name : null,
      diaChi: hop ? (place.address ?? null) : null,
      category: hop ? place.category : undefined,
      placeId: item.placeId ?? null,
      lat: hop ? place.lat : null,
      lng: hop ? place.lng : null,
    };
  });
  return danhSoVaNoi(activities);
}

export function chieuTuChang(stops: readonly ChangChieu[], places: readonly ChoChieu[]): HanhTrinh {
  const theoId = new Map(places.map((p) => [p.id, p]));
  const activities: HoatDongHanhTrinh[] = stops.map((stop) => {
    const place = stop.place_id ? theoId.get(stop.place_id) : undefined;
    const hop = choHopLe(place);
    return {
      id: stop.id,
      so: null,
      gio: stop.at,
      tieuDe: stop.label,
      tenDiaDiem: hop ? place.name : stop.place_name,
      diaChi: hop ? (place.address ?? null) : null,
      category: hop ? place.category : undefined,
      placeId: stop.place_id,
      lat: hop ? place.lat : stop.meeting_point?.lat ?? null,
      lng: hop ? place.lng : stop.meeting_point?.lng ?? null,
    };
  });
  return danhSoVaNoi(activities);
}

/**
 * Permute the mapped slots, but every slot keeps the clock it already had.
 *
 * Reordering stops by geography must not reorder the day: moving a whole row
 * carries its `time` with it, and a day then reads 12:30 -> 20:00 -> 18:00.
 * The hours are the plan the group agreed on; only *what happens at* an hour
 * moves. Measured on the emulator 2026-09-12: without this, one tap on
 * «Toi uu lo trinh» printed a night market at 20:00 before a BBQ at 18:00.
 */
export function ganMappedVaoCho<T extends SlotChieu>(items: readonly T[], mappedIds: readonly string[]): T[] {
  const viTri: number[] = [];
  const theoId = new Map<string, T>();
  items.forEach((item, i) => {
    const id = idSlot(item, i);
    if (mappedIds.includes(id)) {
      viTri.push(i);
      theoId.set(id, item);
    }
  });
  const ra = items.map((item) => ({ ...item }));
  mappedIds.forEach((id, k) => {
    const goc = theoId.get(id);
    const cho = viTri[k];
    if (goc === undefined || cho === undefined) return;
    ra[cho] = { ...goc, time: items[cho].time };
  });
  return ra;
}

/**
 * Permute items whose `id` is in `mappedIds`; every other row stays put.
 *
 * `giuGio` names the field that belongs to the SLOT rather than to the stop --
 * `at` on a live outing. It is restored from the destination, so the clock of
 * the day survives a reorder; see `ganMappedVaoCho`.
 */
export function ganMappedTheoId<T extends { id: string }>(
  items: readonly T[],
  mappedIds: readonly string[],
  giuGio?: keyof T & string,
): T[] {
  const viTri: number[] = [];
  const theoId = new Map<string, T>();
  items.forEach((item, i) => {
    if (mappedIds.includes(item.id)) {
      viTri.push(i);
      theoId.set(item.id, item);
    }
  });
  const ra = items.map((item) => ({ ...item }));
  mappedIds.forEach((id, k) => {
    const goc = theoId.get(id);
    const cho = viTri[k];
    if (goc === undefined || cho === undefined) return;
    ra[cho] = giuGio === undefined ? { ...goc } : { ...goc, [giuGio]: items[cho][giuGio] };
  });
  return ra;
}

export function ganTrangThai(
  ids: readonly string[],
  daToiIds: readonly string[],
  dangDi: boolean,
): TrangThaiChang[] {
  if (!dangDi) return ids.map(() => "sap-toi");
  const da = new Set(daToiIds);
  let daGapHienTai = false;
  return ids.map((id) => {
    if (da.has(id)) return "xong";
    if (!daGapHienTai) {
      daGapHienTai = true;
      return "hien-tai";
    }
    return "sap-toi";
  });
}
