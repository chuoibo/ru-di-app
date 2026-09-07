/**
 * Where an outing sits in time, for the Lên plan tab.
 *
 * The tab used to list every outing newest-created first, each in the same
 * card, and the fixture printed «46 ngày nữa» from a string. Here the tab
 * reads the dates the server holds and answers the only question the person
 * has when they open it: what is next, and how far away is it. Pure, so the
 * arithmetic (day boundaries, «hôm nay» vs «đang diễn ra», a past trip) is
 * pinned by `tests/rudi-nhip-keo.test.mjs` under bare node.
 */

export type NhipKeo =
  | { kieu: "sap-toi"; conNgay: number }
  | { kieu: "hom-nay" }
  | { kieu: "dang-dien-ra"; conNgay: number }
  | { kieu: "da-qua"; truocNgay: number }
  | { kieu: "khong-ro" };

const NGAY_ISO = /^(\d{4})-(\d{2})-(\d{2})$/;
const MS_NGAY = 86_400_000;

/** Calendar day at UTC midnight, or null for anything that is not YYYY-MM-DD. */
export function ngayUtc(s: string): number | null {
  const m = NGAY_ISO.exec(s.trim());
  if (!m) return null;
  const t = Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
  const d = new Date(t);
  if (d.getUTCFullYear() !== Number(m[1]) || d.getUTCMonth() !== Number(m[2]) - 1 || d.getUTCDate() !== Number(m[3])) return null;
  return t;
}

/** Today as YYYY-MM-DD in the phone's own calendar day. */
export function homNay(now: Date = new Date()): string {
  const y = now.getFullYear();
  const m = String(now.getMonth() + 1).padStart(2, "0");
  const d = String(now.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

/** Whole days between two calendar days, `b - a`. */
function khoangNgay(a: number, b: number): number {
  return Math.round((b - a) / MS_NGAY);
}

export function nhipKeo(startsOn: string, endsOn: string, today: string): NhipKeo {
  const bd = ngayUtc(startsOn);
  const kt = ngayUtc(endsOn) ?? bd;
  const nay = ngayUtc(today);
  if (bd === null || kt === null || nay === null) return { kieu: "khong-ro" };
  const toiBatDau = khoangNgay(nay, bd);
  if (toiBatDau > 0) return { kieu: "sap-toi", conNgay: toiBatDau };
  const toiKetThuc = khoangNgay(nay, kt);
  if (toiBatDau === 0 && toiKetThuc === 0) return { kieu: "hom-nay" };
  if (toiKetThuc >= 0) return { kieu: "dang-dien-ra", conNgay: toiKetThuc };
  return { kieu: "da-qua", truocNgay: -toiKetThuc };
}

/** The one line printed beside a date: «Còn 3 ngày», «Hôm nay», «Đang đi», «Đã qua». */
export function nhanNhip(nhip: NhipKeo): string {
  switch (nhip.kieu) {
    case "sap-toi":
      return nhip.conNgay === 1 ? "Ngày mai" : `Còn ${nhip.conNgay} ngày`;
    case "hom-nay":
      return "Hôm nay";
    case "dang-dien-ra":
      return nhip.conNgay === 0 ? "Đang đi · ngày cuối" : `Đang đi · còn ${nhip.conNgay} ngày`;
    case "da-qua":
      return nhip.truocNgay === 1 ? "Hôm qua" : `${nhip.truocNgay} ngày trước`;
    default:
      return "";
  }
}

export interface KeoCoNgay {
  starts_on: string;
  ends_on: string;
}

/**
 * Split a group's outings into what is ahead and what is behind.
 *
 * Ahead is soonest first, so the first element is the next appointment;
 * behind is most recent first, the way a journal reads backwards. An outing
 * with unreadable dates goes last among the upcoming rather than vanishing.
 */
export function chiaKeo<T extends KeoCoNgay>(keo: readonly T[], today: string): { sapToi: T[]; daQua: T[] } {
  const sapToi: T[] = [];
  const daQua: T[] = [];
  for (const k of keo) {
    if (nhipKeo(k.starts_on, k.ends_on, today).kieu === "da-qua") daQua.push(k);
    else sapToi.push(k);
  }
  const bd = (k: T) => ngayUtc(k.starts_on) ?? Number.POSITIVE_INFINITY;
  const kt = (k: T) => ngayUtc(k.ends_on) ?? ngayUtc(k.starts_on) ?? Number.NEGATIVE_INFINITY;
  sapToi.sort((a, b) => bd(a) - bd(b));
  daQua.sort((a, b) => kt(b) - kt(a));
  return { sapToi, daQua };
}

/** «17» and «tháng 10» for a date stamp; null when the date is unreadable. */
export function dauLich(iso: string): { ngay: string; thang: string } | null {
  const m = NGAY_ISO.exec(iso.trim());
  if (!m || ngayUtc(iso) === null) return null;
  return { ngay: String(Number(m[3])), thang: `tháng ${Number(m[2])}` };
}
