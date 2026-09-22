/**
 * The slip a screen hands Nếp about where the person is standing.
 *
 * `CLAUDE.md` is blunt about the limit: «AI chỉ nhận nội dung được gọi/chia sẻ
 * rõ ràng, không tự đọc chat/gu/lịch sử». Chat v2 is end to end encrypted and
 * the server holds no key, so a context-aware assistant cannot be built by
 * reading the conversation. It is built by each screen DECLARING a slip, and
 * this module is the closed vocabulary of what a slip may contain.
 *
 * The shape is a whitelist twice over, and both halves matter:
 *   - the top level is closed, so a screen cannot smuggle a field through by
 *     inventing a name;
 *   - `soLieu` may only carry COUNTS under known keys, so «how many people are
 *     on this trip» travels and «who they are» does not.
 * Money never travels under any key. That is the same law as
 * «Luật Nếp Đứng Xa Tiền», enforced here on the data rather than on the pixels.
 *
 * `donPhieu` is the gate, and it drops rather than throws: a screen that
 * declares one bad field should lose that field, not lose Nếp. A companion
 * source scan (`tests/nep-phieu-kin.test.mjs`) catches the same mistakes at the
 * call sites, because a runtime filter cannot tell anyone they wrote it wrong.
 *
 * Pure: no React, no routing. `NepProvider.tsx` collects slips, `useNepNguCanh`
 * is the hook a screen calls.
 */
import type { NhipKeo } from "../keo/nhip-keo";
import { LOAI_SO, type LoaiSo } from "../so/ban-tinh";

/** Counts a screen may state. Nouns only, never names, never đồng. */
export const KHOA_SO_LIEU = ["soNguoi", "soChang", "soAnh", "soNgay", "soMuc", "soViec"] as const;
export type KhoaSoLieu = (typeof KHOA_SO_LIEU)[number];

export const GIOI_HAN = Object.freeze({
  tieuDe: 80,
  goiY: 3,
  goiYChu: 80,
  /** A count's label is a word, not a sentence; anything longer is prose. */
  soLieuChu: 24,
});

/** The nhịp vocabulary is closed in `keo/nhip-keo.ts`; repeated here to check it. */
const KIEU_NHIP = ["sap-toi", "hom-nay", "dang-dien-ra", "da-qua", "khong-ro"] as const;

/** Route segments Nếp must stand away from (DESIGN.md «Luật Nếp Đứng Xa Tiền»). */
export const MAN_NEP_LUI = ["finance", "settlements", "batches", "smart-split"] as const;

export interface PhieuNguCanh {
  /** Route id, either as declared (`outings/[id]`) or as walked (`/outings/7`). */
  man: string;
  tieuDe?: string;
  nhip?: NhipKeo;
  loaiSo?: LoaiSo;
  soLieu?: Partial<Record<KhoaSoLieu, string | number>>;
  goiY?: readonly string[];
}

function chuNgan(v: unknown, han: number): string | null {
  if (typeof v !== "string") return null;
  const s = v.trim();
  return s ? s.slice(0, han) : null;
}

/**
 * Match on whole segments. `financial-report` starts with `financ` and is not a
 * money screen; a `startsWith` here would silence Nếp on unrelated routes and
 * nobody would notice, because the failure is Nếp being absent.
 */
export function nepPhaiLui(man: string): boolean {
  if (typeof man !== "string") return false;
  const dau = man.replace(/^\/+/, "").split("/")[0];
  return (MAN_NEP_LUI as readonly string[]).includes(dau);
}

export function donPhieu(tho: unknown): PhieuNguCanh | null {
  if (!tho || typeof tho !== "object" || Array.isArray(tho)) return null;
  const raw = tho as Record<string, unknown>;

  const man = chuNgan(raw.man, 120);
  if (!man) return null;

  const phieu: PhieuNguCanh = { man };

  const tieuDe = chuNgan(raw.tieuDe, GIOI_HAN.tieuDe);
  if (tieuDe) phieu.tieuDe = tieuDe;

  const nhip = raw.nhip as { kieu?: unknown } | undefined;
  if (nhip && typeof nhip === "object" && (KIEU_NHIP as readonly unknown[]).includes(nhip.kieu)) {
    phieu.nhip = nhip as NhipKeo;
  }

  if ((LOAI_SO as readonly unknown[]).includes(raw.loaiSo)) phieu.loaiSo = raw.loaiSo as LoaiSo;

  if (raw.soLieu && typeof raw.soLieu === "object" && !Array.isArray(raw.soLieu)) {
    const nguon = raw.soLieu as Record<string, unknown>;
    const sach: Partial<Record<KhoaSoLieu, string | number>> = {};
    for (const khoa of KHOA_SO_LIEU) {
      const v = nguon[khoa];
      if (typeof v === "number" && Number.isFinite(v)) sach[khoa] = v;
      else {
        const s = chuNgan(v, GIOI_HAN.soLieuChu);
        // A label longer than the limit is prose wearing a count's name; drop
        // it whole rather than truncate, so nothing arrives half-said.
        if (s !== null && typeof v === "string" && v.trim().length <= GIOI_HAN.soLieuChu) sach[khoa] = s;
      }
    }
    // An empty object would still read as «this screen stated its numbers».
    if (Object.keys(sach).length > 0) phieu.soLieu = sach;
  }

  if (Array.isArray(raw.goiY)) {
    const goiY = raw.goiY
      .map((g) => chuNgan(g, GIOI_HAN.goiYChu))
      .filter((g): g is string => g !== null)
      .slice(0, GIOI_HAN.goiY);
    if (goiY.length > 0) phieu.goiY = goiY;
  }

  return phieu;
}
