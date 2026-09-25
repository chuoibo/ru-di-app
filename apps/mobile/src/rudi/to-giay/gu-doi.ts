/**
 * Taste in a couple's notebook, as sentences (ADR-0034 §2.1–2.2).
 *
 * The server decides what may be seen: `theirs` only when the other person
 * turned `chia_gu` on, `common` only when both did, and the whole block is
 * null outside «Một đôi». This module only words it: ids become the labels the
 * personalization screen draws, an id the app does not know is left out rather
 * than printed raw, and every sentence names the other person.
 */
import { SO_THICH } from "../../screens/vao-cua/so-thich";

/** `taste` of `GET /contexts/{id}/notebook`. */
export interface GuSo {
  mine_shared: boolean;
  theirs_shared: boolean;
  theirs: readonly string[];
  common: readonly string[];
}

/** «A», «A và B», «A, B và C». */
export function noiDanhSach(nhan: readonly string[]): string {
  if (nhan.length <= 1) return nhan[0] ?? "";
  return `${nhan.slice(0, -1).join(", ")} và ${nhan[nhan.length - 1]}`;
}

function nhanCua(ids: readonly string[]): string[] {
  const out: string[] = [];
  for (const id of ids) {
    const muc = SO_THICH.find((m) => m.id === id);
    if (muc) out.push(muc.nhan);
  }
  return out;
}

export interface CauGu {
  /** What both like; null until both have shared. */
  chung: string | null;
  /** What the other person likes; null until they have shared. */
  cuaHo: string | null;
  /** Where my own switch stands, in one line. */
  cuaToi: string;
}

export function cauGu(gu: GuSo | null | undefined, tenNguoiKia: string): CauGu | null {
  if (!gu) return null;
  const ten = tenNguoiKia.trim() || "Người ấy";
  const cuaHo = gu.theirs_shared ? nhanCua(gu.theirs) : null;
  const chung = gu.mine_shared && gu.theirs_shared ? nhanCua(gu.common) : null;
  return {
    chung: chung === null ? null : chung.length > 0 ? `Hai bạn cùng thích ${noiDanhSach(chung)}.` : "Hai bạn chưa trùng gu nào. Một dịp để rủ nhau thử cái mới.",
    cuaHo: cuaHo === null ? null : cuaHo.length > 0 ? `${ten} thích ${noiDanhSach(cuaHo)}.` : `${ten} chưa chọn gu nào.`,
    cuaToi: gu.mine_shared ? `${ten} thấy gu của bạn trong sổ này.` : "Gu của bạn đang để riêng.",
  };
}
