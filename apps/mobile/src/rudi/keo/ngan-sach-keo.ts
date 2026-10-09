import { MAX_AMOUNT_VND, parseAmountVnd } from "../../../../../packages/shared/money.mjs";

export const BUOC_NGAN_SACH = 25000;
export const TRAN_THANH_NGAN_SACH = 2000000;

/** Quantize the position first; every resulting money value is integer đồng. */
export function nganSachTuViTri(x: number, width: number): number | null {
  if (!Number.isFinite(x) || !Number.isFinite(width) || width <= 0) return null;
  const step = Math.round(Math.max(0, Math.min(1, x / width)) * (TRAN_THANH_NGAN_SACH / BUOC_NGAN_SACH));
  return step * BUOC_NGAN_SACH;
}

/** An exact typed amount may exceed the slider's range; never clamp it. */
export function docNganSach(chu: string): number | null {
  const parsed = parseAmountVnd(chu);
  return parsed.ok ? parsed.value : null;
}

export function dichNganSach(amount: number | null, direction: -1 | 1): number {
  if (amount !== null && amount > TRAN_THANH_NGAN_SACH) return Math.max(0, Math.min(MAX_AMOUNT_VND, amount + direction * BUOC_NGAN_SACH));
  return Math.max(0, Math.min(TRAN_THANH_NGAN_SACH, (amount ?? 0) + direction * BUOC_NGAN_SACH));
}
