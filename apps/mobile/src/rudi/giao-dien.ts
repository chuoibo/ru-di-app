/**
 * Sáng, tối, hay theo hệ thống (L5, ADR-0023 §2.5).
 *
 * Lựa chọn này ở TRÊN MÁY, không lên máy chủ: nó là chuyện của cái màn hình
 * đang cầm, không phải của tài khoản, và một người dùng hai máy có thể muốn
 * hai câu trả lời khác nhau.
 *
 * Lá: không import session, không import kho, không import theme — `theme.ts`
 * import ngược lại module này, nên một chiều duy nhất giữ cho vòng import
 * không khép lại.
 */
export type CheDoGiaoDien = "sang" | "toi" | "he-thong";

export const KHOA_GIAO_DIEN = "rudi.giao-dien.v1";

export const NHAN_GIAO_DIEN: readonly { ma: CheDoGiaoDien; nhan: string }[] = [
  { ma: "sang", nhan: "Sáng" },
  { ma: "toi", nhan: "Tối" },
  { ma: "he-thong", nhan: "Theo hệ thống" },
];

/** Đọc thứ đã lưu. Rác, thiếu, hay chuỗi lạ đều về «theo hệ thống». */
export function docCheDoGiaoDien(raw: unknown): CheDoGiaoDien {
  if (raw === "sang" || raw === "toi" || raw === "he-thong") return raw;
  return "he-thong";
}

/** Màn tối hay không, khi biết lựa chọn của người và của hệ thống. */
export function toiHay(cheDo: CheDoGiaoDien, heThongToi: boolean): boolean {
  if (cheDo === "toi") return true;
  if (cheDo === "sang") return false;
  return heThongToi;
}
