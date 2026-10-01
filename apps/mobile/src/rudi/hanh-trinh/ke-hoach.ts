/** The itinerary wire contract and pure draft transforms. */
/**
 * What the line on the map is right now. The page head stamps it, because a
 * straight pencil line and a road look nothing alike to the engine but can
 * look alike to a tired person at 11pm.
 */
export type TrangThaiTuyen = "that" | "uocLuong" | "dangTinh" | "khongTinhDuoc";
import type { BuoiDi, ChangDung, DiemHen, NgayDi } from "../../screens/len-plan/buoi-di";
export type { DiemHen, NgayDi } from "../../screens/len-plan/buoi-di";
export type ChangDi = ChangDung & { day: string | null; duration_minutes: number | null; time_locked: boolean; meeting_point: DiemHen | null };
export type BanNhap = { expected_revision: number; stops: ChangDi[]; days: NgayDi[] };
export type VanDe = { code: string; stop_id: string | null; message: string };
export type TuyenDi = {
  stops: { id: string; at: string | null; arrival_at: string | null; departure_at: string | null; wait_minutes: number }[];
  segments: { from_stop_id: string; to_stop_id: string; distance_meters: number; duration_seconds: number; geometry: [number, number][]; source: "valhalla" }[];
  distance_meters: number; duration_seconds: number; feasible: boolean; issues: VanDe[];
};
export type XemTruoc = { revision: number; day: string; status: "ready" | "incomplete" | "unavailable"; source: { engine: "valhalla"; graph_version: string; traffic: "none" }; current: TuyenDi | null; suggestion: TuyenDi | null; savings: { distance_meters: number; duration_seconds: number } | null; issues: VanDe[] };
export function ngayMacDinh(day: string): NgayDi { return { day, transport_mode: "motorbike", start_at: "08:00", start_stop_id: null, end_stop_id: null, return_to_start: false }; }
export function nhapTuKeo(outing: BuoiDi): BanNhap {
  return { expected_revision: outing.timeline_revision ?? 0, stops: outing.stops.map((s) => ({ ...s, day: s.day === undefined ? (outing.starts_on === outing.ends_on ? outing.starts_on : null) : s.day, duration_minutes: s.duration_minutes ?? null, time_locked: s.time_locked ?? true, meeting_point: s.meeting_point ?? null })), days: outing.days?.length ? outing.days.map((d) => ({...d})) : [ngayMacDinh(outing.starts_on)] };
}
/** Only the requested day changes; stable IDs and other days remain intact. */
export function apDungTuyen(draft: BanNhap, day: string, route: TuyenDi): BanNhap {
  const byId = new Map(draft.stops.map((s) => [s.id, s]));
  const ordered = route.stops.map((s) => { const stop = byId.get(s.id); if (!stop || stop.day !== day || !s.at) throw new Error("Đề xuất không khớp ngày đang xem."); return { ...stop, at: s.at }; });
  const original = draft.stops.filter((s) => s.day === day);
  if (new Set(ordered.map((s) => s.id)).size !== original.length || ordered.length !== original.length) throw new Error("Đề xuất thiếu chặng của ngày.");
  let index = 0;
  return { ...draft, stops: draft.stops.map((s) => s.day === day ? ordered[index++] : s) };
}
export function noiDungGui(draft: BanNhap) {
  return { ...draft, stops: draft.stops.map(({ position: _position, ...s }) => s) };
}

/** Keep day settings and anchors consistent as a stop crosses day boundaries. */
export function suaChang(draft: BanNhap, id: string, change: Partial<ChangDi>): BanNhap {
  const stops = draft.stops.map((s) => s.id === id ? { ...s, ...change } : s);
  const days = draft.days.map((d) => ({ ...d,
    start_stop_id: stops.some((s) => s.id === d.start_stop_id && s.day === d.day) ? d.start_stop_id : null,
    end_stop_id: stops.some((s) => s.id === d.end_stop_id && s.day === d.day) ? d.end_stop_id : null,
  }));
  if (change.day && !days.some((d) => d.day === change.day)) days.push(ngayMacDinh(change.day));
  return { ...draft, stops, days };
}

/** Move within this day's slots; other days and stable stop IDs stay untouched. */
export function doiViTri(draft: BanNhap, id: string, target: "first" | "last" | -1 | 1): BanNhap {
  const stop = draft.stops.find((s) => s.id === id);
  if (!stop?.day) return draft;
  const same = draft.stops.filter((s) => s.day === stop.day);
  const from = same.findIndex((s) => s.id === id);
  const to = target === "first" ? 0 : target === "last" ? same.length - 1 : Math.max(0, Math.min(same.length - 1, from + target));
  same.splice(from, 1); same.splice(to, 0, stop);
  let index = 0;
  return { ...draft, stops: draft.stops.map((s) => s.day === stop.day ? same[index++] : s), days: draft.days.map((d) => d.day !== stop.day ? d : { ...d, start_stop_id: d.start_stop_id === same[0]?.id ? d.start_stop_id : null, end_stop_id: d.end_stop_id === same[same.length - 1]?.id ? d.end_stop_id : null }) };
}

/**
 * A multi-day outing's stops that are on none of its days: saved without a
 * day (the server leaves the day empty for an outing longer than one day) or
 * on a date the outing no longer covers. No day's map shows them (QA UI-032).
 */
export function changChuaXep(draft: BanNhap, days: readonly string[]): ChangDi[] {
  return draft.stops.filter((s) => s.day === null || !days.includes(s.day));
}

/**
 * Put these stops on `day`, after the stops already there (a day's order is
 * its stops' order in the list), in their own order. Only ever on the
 * person's word: an old multi-day schedule is never given a guessed day (see
 * `nhapTuKeo`).
 */
export function xepVaoNgay(draft: BanNhap, ids: readonly string[], day: string): BanNhap {
  const chon = new Set(ids);
  const moved = draft.stops.filter((s) => chon.has(s.id)).map((s) => ({ ...s, day }));
  if (!moved.length) return draft;
  return {
    ...draft,
    stops: [...draft.stops.filter((s) => !chon.has(s.id)), ...moved],
    days: draft.days.some((d) => d.day === day) ? draft.days : [...draft.days, ngayMacDinh(day)],
  };
}

export function xoaChang(draft: BanNhap, id: string): BanNhap {
  return { ...draft, stops: draft.stops.filter((s) => s.id !== id), days: draft.days.map((d) => ({ ...d, start_stop_id: d.start_stop_id === id ? null : d.start_stop_id, end_stop_id: d.end_stop_id === id ? null : d.end_stop_id })) };
}

const THU = ["CN", "T2", "T3", "T4", "T5", "T6", "T7"] as const;

/** «T7 12/10» from an ISO date, read at noon UTC so no timezone shifts the day. */
export function chuNgay(iso: string): string {
  const d = new Date(`${iso}T12:00:00Z`);
  return `${THU[d.getUTCDay()]} ${String(d.getUTCDate()).padStart(2, "0")}/${String(d.getUTCMonth() + 1).padStart(2, "0")}`;
}

/**
 * What the line on the map is. A route answer is a road; a preview that came
 * back without one for a routing reason is «chưa tính được», said as such;
 * anything else (no preview yet, a fixture, stops without a place) is a draft.
 */
export function trangThaiTuyen(tuyen: { fixture: boolean; dangTinh: boolean; coTuyen: boolean; preview: Pick<XemTruoc, "status" | "issues"> | null }): TrangThaiTuyen {
  if (tuyen.fixture) return "uocLuong";
  if (tuyen.coTuyen) return "that";
  if (tuyen.dangTinh && !tuyen.preview) return "dangTinh";
  if (tuyen.preview && (tuyen.preview.status === "unavailable" || tuyen.preview.issues.some((v) => v.code.startsWith("routing_")))) return "khongTinhDuoc";
  return "uocLuong";
}

