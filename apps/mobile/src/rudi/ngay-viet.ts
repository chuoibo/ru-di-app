/**
 * One way of writing a day and a time across the app (QA UI-104): the wall
 * wrote «10:07 28-09» (the vi-VN locale joins day and month with a hyphen),
 * settings wrote «28/9/2026» with no zero, and everything else «28/09». The
 * day is the phone's local day, as every other date on screen.
 */
const hai = (n: number) => String(n).padStart(2, "0");

/** «28/09/2026», or "" for a value that is not a date. */
export function ngayVN(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return `${hai(d.getDate())}/${hai(d.getMonth() + 1)}/${d.getFullYear()}`;
}

/** «10:07 · 28/09», a moment on a wall or a print; the year when it is not this one. */
export function gioNgayVN(iso: string, bayGio: Date = new Date()): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const ngay = `${hai(d.getDate())}/${hai(d.getMonth() + 1)}${d.getFullYear() === bayGio.getFullYear() ? "" : `/${d.getFullYear()}`}`;
  return `${hai(d.getHours())}:${hai(d.getMinutes())} · ${ngay}`;
}

/**
 * An outing's days, «28 - 29/09» (QA UI-103): the album shelf wrote only the
 * year, so two trips of one year could not be told apart by time. Calendar
 * days as the server keeps them (`YYYY-MM-DD`), hyphen as `nhanKhoangNgay`;
 * the year only when it is not this one. "" when a day cannot be read.
 */
export function khoangNgayChuyen(starts: string, ends: string, namNay: number = new Date().getFullYear()): string {
  const tach = (s: string) => {
    const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(s);
    return m ? { y: Number(m[1]), m: m[2], d: m[3] } : null;
  };
  const a = tach(starts);
  const b = tach(ends) ?? a;
  if (a === null || b === null) return "";
  const nam = (y: number) => (y === namNay ? "" : `/${y}`);
  if (a.y !== b.y) return `${a.d}/${a.m}/${a.y} - ${b.d}/${b.m}/${b.y}`;
  if (a.m === b.m && a.d === b.d) return `${a.d}/${a.m}${nam(a.y)}`;
  if (a.m === b.m) return `${a.d} - ${b.d}/${b.m}${nam(a.y)}`;
  return `${a.d}/${a.m} - ${b.d}/${b.m}${nam(a.y)}`;
}

/** «Tháng 9, 2026», a month heading; written by hand, as Hermes and the browser name months differently. */
export function thangNamVN(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return `Tháng ${d.getMonth() + 1}, ${d.getFullYear()}`;
}
