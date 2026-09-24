/**
 * The rules of the «Đề nghị sửa» sheet, pure, so a node test can hold them.
 *
 * QA 23/09: the day was a raw ISO string to type («2026-09-26»), the two hour
 * boxes were free text with the same accessible name («Giờ» twice), and the
 * second stop could be earlier than the first. The server's schema refuses a
 * malformed hour (`^([01][0-9]|2[0-3]):[0-5][0-9]$`) with a 422 the person
 * could not read; the sheet now says it before sending.
 */
import { ngayDocDuoc } from "./ngay";

const MS_NGAY = 86_400_000;
const GIO_DUNG = /^([01][0-9]|2[0-3]):[0-5][0-9]$/;

function isoUtc(ms: number): string {
  return new Date(ms).toISOString().slice(0, 10);
}

function msCua(iso: string): number | null {
  const khop = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  if (!khop) return null;
  const ms = Date.UTC(Number(khop[1]), Number(khop[2]) - 1, Number(khop[3]));
  return isoUtc(ms) === iso ? ms : null;
}

/**
 * The days a sheet may propose, as chips: from today (or the sheet's Monday,
 * whichever is later) through the end of the following week. The day already
 * on the sheet is kept even when it falls outside, so opening the sheet never
 * silently changes it.
 */
export function ngayChonDuoc(tuan: string, homNay: string, dangChon: string): string[] {
  const dau = msCua(tuan);
  const nay = msCua(homNay);
  const ra: string[] = [];
  if (dau !== null) {
    const tu = Math.max(dau, nay ?? dau);
    for (let ms = tu; ms < dau + 14 * MS_NGAY; ms += MS_NGAY) ra.push(isoUtc(ms));
  }
  if (msCua(dangChon) !== null && !ra.includes(dangChon)) ra.unshift(dangChon);
  return ra;
}

/** «T7 26/09»: the chip's short name; the long one («Thứ Bảy 26/09») is its label. */
export function ngayNgan(iso: string): string {
  const dai = ngayDocDuoc(iso);
  const khop = /^(Chủ nhật|Thứ (Hai|Ba|Tư|Năm|Sáu|Bảy)) (\d{2}\/\d{2})$/.exec(dai);
  if (!khop) return dai;
  const so: Record<string, string> = { Hai: "T2", Ba: "T3", Tư: "T4", Năm: "T5", Sáu: "T6", Bảy: "T7" };
  return `${khop[2] ? so[khop[2]] : "CN"} ${khop[3]}`;
}

/**
 * What a person types for an hour, as the wire spells it: «1930», «19h30»,
 * «19.30», «7:05» all become «19:30» / «07:05». Anything else is returned as
 * typed, for `loiGio` to name.
 */
export function chuanGio(tho: string): string {
  const s = tho.trim();
  const khop = /^(\d{1,2})\s*[:hH.]?\s*(\d{2})$/.exec(s) ?? /^(\d{1,2})\s*[hH]$/.exec(s);
  if (!khop) return s;
  const gio = Number(khop[1]);
  const phut = khop[2] === undefined ? 0 : Number(khop[2]);
  if (gio > 23 || phut > 59) return s;
  return `${String(gio).padStart(2, "0")}:${String(phut).padStart(2, "0")}`;
}

/** Why an hour will be refused, or null. An empty optional hour is fine. */
export function loiGio(gio: string, batBuoc: boolean): string | null {
  if (gio.trim() === "") return batBuoc ? "Chặng chính cần một giờ." : null;
  return GIO_DUNG.test(gio.trim()) ? null : "Giờ dạng hh:mm, ví dụ 19:30.";
}

/**
 * A stop that has an hour needs its line: the schema refuses an empty `viec`
 * (1..200), and a sheet saying «18:00» and nothing else says nothing.
 */
export function loiViec(viec: string, gio: string): string | null {
  return viec.trim() === "" && gio.trim() !== "" ? "Chặng này cần một dòng việc." : null;
}

/**
 * The next stop comes after the main one. An hour before six in the morning
 * is read as past midnight (22:30 → 00:30 is an evening), so it passes.
 */
export function loiThuTu(gio1: string, gio2: string): string | null {
  if (!GIO_DUNG.test(gio1) || !GIO_DUNG.test(gio2)) return null;
  if (gio2 === gio1 || (gio2 < gio1 && gio2 >= "06:00")) return `Đi tiếp phải sau ${gio1}.`;
  return null;
}

/**
 * The line a stop takes from the kind of place it is at, the same table as
 * the server's draft (`pair_paper._VIEC_THEO_LOAI`); a kind not listed keeps
 * the line it had. «Ăn tối» at a café read wrong (24/09).
 */
export function viecTheoLoai(loai: string | null | undefined, cu: string): string {
  const bang: Record<string, string> = { cafe: "Cà phê", "vui-choi": "Đi chơi", "di-choi-dem": "Đi chơi tối" };
  return (loai && bang[loai]) || cu;
}
