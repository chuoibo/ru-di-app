/**
 * The ten-second read of a place's page: the few facts that decide, each said
 * once, and the row of what kind of place it is, each said once.
 *
 * The price used to print twice (a «Giá» section under a chip row that also
 * carried duration and booking), and the chip row printed «yên tĩnh» beside
 * «Yên tĩnh» because the index and the traits spell a tag their own way
 * (finish review 21/10). Pure, so a node test reads what the page prints.
 */
import type { GioTheoThu, Highlights } from "../../screens/kham-pha/chi-tiet-dia-diem";
import type { Place } from "../../screens/kham-pha/places";
import { cauGia } from "./dia-diem";

/** One line of the key facts: a short label and its value, with an optional quieter line under it. */
export type DongChinh = { id: "gia" | "hom-nay" | "gio-dep" | "thoi-luong" | "dat-truoc"; nhan: string; gia: string; phu?: string };

/** Weekday codes the server sends in `gio_mo_cua[].thu`, indexed by `Date#getDay()`. */
export const MA_THU = ["sun", "mon", "tue", "wed", "thu", "fri", "sat"] as const;

/** Today's opening, or null when the week the server sent has no line for today. */
export function gioHomNay(gio: readonly GioTheoThu[], now: Date): GioTheoThu | null {
  const ma = MA_THU[now.getDay()];
  return gio.find((g) => g.thu === ma) ?? null;
}

/** «06:00 – 21:30», with the overnight mark when the server sends one. */
export function cauGio(g: GioTheoThu): string {
  return `${g.mo} – ${g.dong}${g.quaDem ? " (qua đêm)" : ""}`;
}

/** The price band of the index, saying «Ước khoảng» when the server calls it an estimate. */
export function cauKhoangGia(g: NonNullable<Highlights["gia"]>): string {
  const k = (v: number) => `${Math.round(v / 1000)}`;
  const dai = g.maxVnd === null || g.maxVnd === g.minVnd ? `${k(g.minVnd)}k` : `${k(g.minVnd)}–${k(g.maxVnd)}k`;
  return `${g.uoc ? "Ước khoảng" : "Khoảng"} ${dai}/người`;
}

export function cauThoiLuong(t: NonNullable<Highlights["thoiLuong"]>): string {
  if (t.min !== null && t.max !== null && t.min !== t.max) return `Khoảng ${t.min}–${t.max} phút`;
  return `Khoảng ${t.min ?? t.max} phút`;
}

/**
 * The key facts, in reading order, each only when the server sent it: the
 * price (the index's band first, else the row's own), today's hours and the
 * best hour, how long a visit takes, whether to book.
 */
export function dongChinh(place: Pick<Place, "priceMinVnd" | "priceMaxVnd">, hl: Highlights | null, now: Date): DongChinh[] {
  const ra: DongChinh[] = [];
  const mucGia = hl?.mucGia ?? undefined;
  if (hl?.gia) ra.push({ id: "gia", nhan: "Giá", gia: cauKhoangGia(hl.gia), phu: mucGia });
  else if (place.priceMinVnd !== null && place.priceMaxVnd !== null) ra.push({ id: "gia", nhan: "Giá", gia: cauGia(place), phu: mucGia });
  else if (mucGia !== undefined) ra.push({ id: "gia", nhan: "Giá", gia: mucGia });
  const homNay = hl ? gioHomNay(hl.gioMoCua, now) : null;
  if (homNay !== null) ra.push({ id: "hom-nay", nhan: "Hôm nay", gia: cauGio(homNay) });
  if (hl?.khungGioDep) ra.push({ id: "gio-dep", nhan: "Giờ đẹp nhất", gia: hl.khungGioDep });
  if (hl?.thoiLuong) ra.push({ id: "thoi-luong", nhan: "Thời gian", gia: cauThoiLuong(hl.thoiLuong) });
  if (hl?.canDatTruoc) ra.push({ id: "dat-truoc", nhan: "Đặt chỗ", gia: hl.canDatTruoc.nhan });
  return ra;
}

/** A tag's identity regardless of case, diacritics, «đ» and spacing. */
export function khoaNhan(s: string): string {
  return s.normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/đ/gi, "d").toLowerCase().replace(/\s+/g, " ").trim();
}

/** Each tag once, the first spelling kept, at most `toiDa`. */
export function nhanKhongTrung(ds: readonly string[], toiDa = 10): string[] {
  const da = new Set<string>();
  const ra: string[] = [];
  for (const t of ds) {
    const k = khoaNhan(t);
    if (k === "" || da.has(k)) continue;
    da.add(k);
    ra.push(t.trim());
    if (ra.length >= toiDa) break;
  }
  return ra;
}

/** The chip row: what kind of place this is (category, mood, indoors/outdoors, traits) -- never a fact already in the key facts. */
export function nhanKhiChat(place: Pick<Place, "traits">, hl: Highlights | null): string[] {
  return nhanKhongTrung([
    ...(hl?.danhMuc.map((d) => d.nhan) ?? []),
    ...(hl?.khiChat.map((d) => d.nhan) ?? []),
    ...(hl?.trongNgoai ? [hl.trongNgoai.nhan] : []),
    ...place.traits,
  ]);
}
