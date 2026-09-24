/**
 * The two number boxes of a bill line, as rules a node test can hold.
 *
 * QA 23/09: the quantity box could not be emptied to type again -- «12»,
 * Backspace gave «1», Backspace again was refused (an empty quantity does not
 * parse), so typing «2» made «12» -- and the sum box showed the raw «350000»
 * under a row saying «350.000đ». A box now keeps what the person is typing as
 * a DRAFT, feeds the bill only what parses, and on leaving the box shows the
 * committed value again, formatted the way the rest of the app writes money.
 *
 * Money stays integer đồng throughout (`parseAmountVnd`, `formatVnd`); the
 * «mỗi phần» hint is printed only when the line divides exactly, so no
 * fraction of a đồng is ever shown as if it were a price.
 */
import { formatVnd, parseAmountVnd } from "../../../../../packages/shared/money.mjs";

export type KieuO = "so-luong" | "tien";

/** What a box shows when nobody is typing in it. */
export function hienO(kieu: KieuO, giaTri: number): string {
  if (kieu === "so-luong") return String(giaTri);
  return giaTri === 0 ? "" : formatVnd(giaTri);
}

/** Why a draft cannot be committed, in words, or null. */
export function loiO(kieu: KieuO, nhap: string): string | null {
  const kq = parseAmountVnd(nhap);
  if (!kq.ok) {
    if (kq.reason === "empty") return kieu === "so-luong" ? "Ít nhất 1 phần." : null;
    return kq.reason === "too-large" ? "Số này lớn quá." : "Chỉ gõ số thôi.";
  }
  if (kieu === "so-luong" && kq.value === 0) return "Ít nhất 1 phần.";
  if (kieu === "so-luong" && kq.value > 99) return "Tối đa 99 phần một dòng.";
  return null;
}

/**
 * The line under the sum box: what the number means, and the per-portion
 * price when the line has several and divides exactly.
 */
export function goiYTien(soLuong: number, thanhTien: number): string {
  if (soLuong > 1 && thanhTien > 0 && thanhTien % soLuong === 0) {
    return `Tổng cả dòng · ${formatVnd(thanhTien / soLuong)}đ mỗi phần.`;
  }
  return "Tổng cả dòng, không phải giá một phần.";
}

/** «12 phần» under a line's name; nothing for a single portion. */
export function cauSoPhan(soLuong: number): string | null {
  return soLuong > 1 ? `${soLuong} phần` : null;
}
