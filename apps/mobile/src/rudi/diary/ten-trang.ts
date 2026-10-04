/**
 * A page title as people read it (QA UI-154): a bare date, which is how a
 * page's day used to be stored («2026-09-29»), reads «Ngày 29/09/2026»; a
 * title somebody wrote stays exactly as written. The server writes new pages
 * this way (`book.TenTrang`); this reads the books saved before.
 */
import { ngayKieuViet } from "../chat/to-hen-chung";

export function tenTrang(heading: string): string {
  return /^\d{4}-\d{2}-\d{2}$/.test(heading) && ngayKieuViet(heading) !== heading ? `Ngày ${ngayKieuViet(heading)}` : heading;
}

/** A book's pages with every title readable, for the editor's boxes. */
export function voiTenTrang<T extends { pages: { heading: string }[] }>(doc: T): T {
  return { ...doc, pages: doc.pages.map((p) => ({ ...p, heading: tenTrang(p.heading) })) };
}
