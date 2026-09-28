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
    cuaToi: gu.mine_shared ? `${ten} thấy gu của bạn; Nếp dùng nó khi phác tờ, và Rủ Đi AI dùng nó trong chat của hai bạn.` : "Gu của bạn đang để riêng.",
  };
}

/**
 * What turning my switch on promises (ADR-0048 §3.5): the other person sees my
 * taste, and Nếp and Rủ Đi AI in the two's chat use it. The chat is named in
 * the promise itself, because a consent given before it was named does not
 * cover the chat.
 */
export function cauBatGu(gu: GuSo | null | undefined, tenNguoiKia: string): string {
  const ten = tenNguoiKia.trim() || "Người ấy";
  return gu?.theirs_shared
    ? `Bật thì ${ten} thấy gu của bạn, hai bạn thấy mình cùng thích gì, và Nếp cùng Rủ Đi AI trong chat của hai bạn dùng gu đó.`
    : `Bật thì ${ten} thấy gu của bạn, Nếp và Rủ Đi AI trong chat của hai bạn dùng nó. Gu của ${ten} chỉ hiện khi chính họ bật.`;
}

/** `gu_chat` of chat-capabilities (ADR-0048 §3.2); null outside a couple. */
export interface GuChat {
  cua_toi: "tat" | "bat" | "can_bat_lai";
  nguoi_kia: boolean;
}

/**
 * Whether to offer «Bật lại cho chat»: my switch is on, but under the older
 * wording that promised only «Nếp dùng khi phác tờ», so the chat may not use
 * my taste until I turn it on again. Fails closed: no answer from the server,
 * no button.
 */
export function canBatLaiChoChat(gu: GuSo | null | undefined, guChat: GuChat | null | undefined): boolean {
  return gu?.mine_shared === true && guChat?.cua_toi === "can_bat_lai";
}

/** Why the button is there, in one line. */
export const CAU_BAT_LAI_CHO_CHAT = "Bạn bật từ trước, khi lời hứa chỉ là Nếp dùng gu khi phác tờ. Muốn Rủ Đi AI dùng gu của bạn trong chat thì bật lại.";

/**
 * Re-consent for the chat: off, then on again through the notebook's own two
 * commands; the second runs only if the first landed. If the second fails the
 * switch is left off -- closed, never on under words the person did not see.
 */
export async function batLaiChoChat(thuHoi: () => Promise<boolean>, bat: () => Promise<boolean>): Promise<boolean> {
  if (!(await thuHoi())) return false;
  return bat();
}
