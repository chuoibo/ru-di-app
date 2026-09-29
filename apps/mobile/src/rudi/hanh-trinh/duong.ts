/**
 * Line helpers for the journey map.
 *
 * Road geometry comes only from the server's self-hosted Valhalla preview
 * (ADR-0028: no public routing service ever sees a stop). What the client
 * draws on its own is the draft: a straight pencil line that shows order,
 * never a road.
 */

import type { ToaDo } from "./khoang";

export function geodesic(from: ToaDo, to: ToaDo): ToaDo[] {
  return [from, to];
}

export function khoaDoan(from: ToaDo, to: ToaDo): string {
  return `${from.lat},${from.lng};${to.lat},${to.lng}`;
}

/** `hh:mm` minus a number of seconds, wrapping around midnight. */
export function gioTru(hhmm: string, giay: number): string {
  const [hChu, mChu] = hhmm.split(":");
  const h = Number(hChu);
  const m = Number(mChu);
  let tong = h * 3600 + m * 60 - giay;
  const ngay = 24 * 3600;
  tong = ((tong % ngay) + ngay) % ngay;
  const gio = Math.floor(tong / 3600);
  const phut = Math.floor((tong % 3600) / 60);
  return `${String(gio).padStart(2, "0")}:${String(phut).padStart(2, "0")}`;
}
