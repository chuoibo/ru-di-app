/**
 * What a person was about to post, kept while they step away (QA UI-097):
 * «Thả khoảnh khắc» and «Đăng story» lost the picked photo and the words on
 * every way out, Back or a tab, and came back empty.
 *
 * In memory for the life of the app, like the bill draft on a phone
 * (`chia-bill/nhap-bill.ts`), one per screen, person and group. The picked
 * file stays in the picker's cache while a draft holds it; posting, «Bỏ ảnh»
 * or «Bỏ bản nháp» is what lets it go. Nothing is sent anywhere.
 */
import type { TempPhoto } from "./chon-anh";

export type NhapDang = { anh: TempPhoto | null; chu: string };

const kho = new Map<string, NhapDang>();

export const khoaNhapDang = (man: "khoanh-khac" | "story", person: string, nhom: string | null) =>
  `${man}:${person}:${nhom ?? "-"}`;

/** The draft left on this screen, or null when there is nothing worth keeping. */
export function docNhapDang(khoa: string): NhapDang | null {
  return kho.get(khoa) ?? null;
}

/** Keeps the draft; an empty one (no photo, no words) is not a draft. */
export function ghiNhapDang(khoa: string, nhap: NhapDang): void {
  if (nhap.anh === null && nhap.chu.trim() === "") kho.delete(khoa);
  else kho.set(khoa, nhap);
}

export function boNhapDang(khoa: string): void {
  kho.delete(khoa);
}
