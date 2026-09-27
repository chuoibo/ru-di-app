/**
 * The month a tear-off calendar leafs through (plan S0.5, `ChonNgayLich`).
 *
 * Pure date arithmetic on the calendar the person sees -- day, month, year,
 * no clock and no time zone -- so a date typed or picked is the same string
 * the existing validators read (`dd/mm/yyyy`, parsed by `ngayVeISO`). Weeks
 * start on Monday, as a Vietnamese calendar prints them.
 */
export interface NgayLich {
  ngay: number;
  thang: number;
  nam: number;
}

export interface OLich extends NgayLich {
  trongThang: boolean;
}

export const THU_NGAN = ["T2", "T3", "T4", "T5", "T6", "T7", "CN"] as const;

export function laNamNhuan(nam: number): boolean {
  return (nam % 4 === 0 && nam % 100 !== 0) || nam % 400 === 0;
}

export function soNgayTrongThang(thang: number, nam: number): number {
  if (thang === 2) return laNamNhuan(nam) ? 29 : 28;
  return [4, 6, 9, 11].includes(thang) ? 30 : 31;
}

/** 0 = Monday .. 6 = Sunday, for a calendar date (Sakamoto's method, no Date object). */
export function thuTrongTuan(n: NgayLich): number {
  const t = [0, 3, 2, 5, 0, 3, 5, 1, 4, 6, 2, 4];
  const y = n.thang < 3 ? n.nam - 1 : n.nam;
  const cn0 = (y + Math.floor(y / 4) - Math.floor(y / 100) + Math.floor(y / 400) + t[n.thang - 1] + n.ngay) % 7; // 0 = Sunday
  return (cn0 + 6) % 7;
}

export function congNgay(n: NgayLich, so: number): NgayLich {
  let { ngay, thang, nam } = n;
  ngay += so;
  while (ngay > soNgayTrongThang(thang, nam)) {
    ngay -= soNgayTrongThang(thang, nam);
    thang += 1;
    if (thang > 12) {
      thang = 1;
      nam += 1;
    }
  }
  while (ngay < 1) {
    thang -= 1;
    if (thang < 1) {
      thang = 12;
      nam -= 1;
    }
    ngay += soNgayTrongThang(thang, nam);
  }
  return { ngay, thang, nam };
}

/** Six weeks of a month, Monday first: the grid a picker lays out (42 cells, always). */
export function luoiThang(thang: number, nam: number): OLich[] {
  const dau = { ngay: 1, thang, nam };
  const lui = thuTrongTuan(dau);
  const batDau = congNgay(dau, -lui);
  const o: OLich[] = [];
  for (let i = 0; i < 42; i += 1) {
    const n = congNgay(batDau, i);
    o.push({ ...n, trongThang: n.thang === thang && n.nam === nam });
  }
  return o;
}

export function thangSau(thang: number, nam: number, buoc: 1 | -1): { thang: number; nam: number } {
  const t = thang + buoc;
  if (t > 12) return { thang: 1, nam: nam + 1 };
  if (t < 1) return { thang: 12, nam: nam - 1 };
  return { thang: t, nam };
}

const hai = (n: number) => (n < 10 ? `0${n}` : String(n));

/** The typed form every date field reads back: `dd/mm/yyyy`. */
export function dinhDangNgay(n: NgayLich): string {
  return `${hai(n.ngay)}/${hai(n.thang)}/${n.nam}`;
}

/** `dd/mm/yyyy` (one-digit day or month accepted) as a real calendar date, or null. */
export function docNgay(chu: string): NgayLich | null {
  const m = /^\s*(\d{1,2})\/(\d{1,2})\/(\d{4})\s*$/.exec(chu);
  if (!m) return null;
  const ngay = Number(m[1]);
  const thang = Number(m[2]);
  const nam = Number(m[3]);
  if (thang < 1 || thang > 12 || ngay < 1 || ngay > soNgayTrongThang(thang, nam)) return null;
  return { ngay, thang, nam };
}

export function cungNgay(a: NgayLich | null, b: NgayLich | null): boolean {
  return !!a && !!b && a.ngay === b.ngay && a.thang === b.thang && a.nam === b.nam;
}

export function soSanhNgay(a: NgayLich, b: NgayLich): number {
  return a.nam - b.nam || a.thang - b.thang || a.ngay - b.ngay;
}

/** «Tháng 9, 2026». */
export function tenThang(thang: number, nam: number): string {
  return `Tháng ${thang}, ${nam}`;
}

/** What a leaf says under its big number: «Thứ Sáu» / «Chủ nhật». */
export function tenThu(n: NgayLich): string {
  const i = thuTrongTuan(n);
  return i === 6 ? "Chủ nhật" : `Thứ ${["Hai", "Ba", "Tư", "Năm", "Sáu", "Bảy"][i]}`;
}
