/** Road geometry when OSRM answers; geodesic otherwise. Never blocks the view. */

import { giayTuMet, haversineMet, type ToaDo } from "./khoang";
import type { DoanDuongHanhTrinh } from "./mo-hinh";

const OSRM = "https://router.project-osrm.org/route/v1/driving";
const TIMEOUT_MS = 4000;

export function geodesic(from: ToaDo, to: ToaDo): ToaDo[] {
  return [from, to];
}

export function khoaDoan(from: ToaDo, to: ToaDo): string {
  return `${from.lat},${from.lng};${to.lat},${to.lng}`;
}

function geodesicDoan(from: ToaDo, to: ToaDo): Omit<DoanDuongHanhTrinh, "id" | "fromActivityId" | "toActivityId"> {
  const met = haversineMet(from, to);
  return {
    distanceMeters: met,
    durationSeconds: giayTuMet(met),
    transportMode: "motorbike",
    polyline: geodesic(from, to),
    nguon: "geodesic",
  };
}

type OsrmBody = {
  code?: string;
  routes?: { distance?: number; duration?: number; geometry?: { coordinates?: [number, number][] } }[];
};

function docOsrm(body: OsrmBody, from: ToaDo, to: ToaDo): ReturnType<typeof geodesicDoan> | null {
  if (body.code !== "Ok" || !body.routes?.[0]) return null;
  const route = body.routes[0];
  const coords = route.geometry?.coordinates;
  if (!Array.isArray(coords) || coords.length < 2) return null;
  const distance = route.distance;
  const duration = route.duration;
  if (typeof distance !== "number" || typeof duration !== "number" || !Number.isFinite(distance) || !Number.isFinite(duration)) {
    return null;
  }
  return {
    distanceMeters: Math.round(distance),
    durationSeconds: Math.round(duration),
    transportMode: "motorbike",
    polyline: coords.map(([lng, lat]) => ({ lat, lng })),
    nguon: "osrm",
  };
}

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

export async function layDoanDuong(
  from: ToaDo,
  to: ToaDo,
  tuyChon: {
    fetch?: typeof fetch;
    timeoutMs?: number;
    cache?: Map<string, ReturnType<typeof geodesicDoan>>;
  } = {},
): Promise<ReturnType<typeof geodesicDoan>> {
  const khoa = khoaDoan(from, to);
  const cache = tuyChon.cache;
  const san = cache?.get(khoa);
  if (san) return san;

  const fallback = geodesicDoan(from, to);
  const goi = tuyChon.fetch ?? fetch;
  const timeoutMs = tuyChon.timeoutMs ?? TIMEOUT_MS;
  const dieuKhien = new AbortController();
  const url = `${OSRM}/${from.lng},${from.lat};${to.lng},${to.lat}?overview=full&geometries=geojson`;
  let hen: ReturnType<typeof setTimeout> | undefined;
  try {
    const res = await Promise.race([
      goi(url, { signal: dieuKhien.signal }),
      new Promise<never>((_, reject) => {
        hen = setTimeout(() => {
          dieuKhien.abort();
          reject(new Error("osrm-timeout"));
        }, timeoutMs);
      }),
    ]);
    if (!res.ok) {
      cache?.set(khoa, fallback);
      return fallback;
    }
    const body = (await res.json()) as OsrmBody;
    const osrm = docOsrm(body, from, to);
    const ra = osrm ?? fallback;
    cache?.set(khoa, ra);
    return ra;
  } catch {
    cache?.set(khoa, fallback);
    return fallback;
  } finally {
    if (hen !== undefined) clearTimeout(hen);
  }
}

export async function lamGiauDoan(
  segments: readonly DoanDuongHanhTrinh[],
  activities: readonly { id: string; lat: number | null; lng: number | null }[],
  tuyChon?: Parameters<typeof layDoanDuong>[2],
): Promise<DoanDuongHanhTrinh[]> {
  const theoId = new Map(activities.map((a) => [a.id, a]));
  return Promise.all(
    segments.map(async (doan) => {
      const from = theoId.get(doan.fromActivityId);
      const to = theoId.get(doan.toActivityId);
      if (from?.lat == null || from.lng == null || to?.lat == null || to.lng == null) return doan;
      const giau = await layDoanDuong({ lat: from.lat, lng: from.lng }, { lat: to.lat, lng: to.lng }, tuyChon);
      return { ...doan, ...giau };
    }),
  );
}
