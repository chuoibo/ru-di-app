/**
 * A post's album pages one full-width picture at a time (owner's choice,
 * 02/10). The page in view comes from the scroll offset, which the web build
 * reports through onScroll and native through the momentum end alike.
 */
export function trangAlbum(offsetX: number, rong: number, soTrang: number): number {
  if (rong <= 0 || soTrang <= 0) return 0;
  return Math.min(soTrang - 1, Math.max(0, Math.round(offsetX / rong)));
}

/** «2/3» over the picture; nothing for a single picture. */
export function nhanTrang(i: number, soTrang: number): string | null {
  return soTrang > 1 ? `${i + 1}/${soTrang}` : null;
}

/** A page of the album, width over height: the owner's mockup draws 4:3. */
export const TI_LE_ALBUM = 4 / 3;

/** The picture's frame for a column `rong` wide. */
export function kichTrangAlbum(rong: number): { width: number; height: number } {
  return { width: rong, height: Math.round(rong / TI_LE_ALBUM) };
}
