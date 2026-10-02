/**
 * The words of the chip above the send button while a message asks Rủ Đi AI
 * (ADR-0046 §2.3, proposed; keeps ADR-0036 §2.5).
 *
 * The chip replaces the «Mình đang thấy» block of the old AI tray and keeps
 * its promise: it says how many messages go along, «Xem» lists exactly those,
 * and «Chỉ gửi lời nhờ» is one tap away. The count is read from the bundle
 * object that will be sent -- never from the list on screen, whose length is a
 * different number the moment the bundle is trimmed (`ganNgan`) or capped
 * (`GIOI_HAN_BOI_CANH.soLuot`). A chip that counts the screen is a chip that
 * lies about the payload, which is the one thing it exists not to do.
 *
 * Pure, so `tests/chip-boi-canh.test.mjs` can hold it to that.
 */
import { nhanVai, type BoiCanh, type LuotBoiCanh } from "../ai/boi-canh";

export type ChuChip = {
  /** The chip's sentence. */
  cau: string;
  /**
   * The same sentence for a screen reader when the chip leaves its subject to
   * the ✦: the eye reads the mark, the ear needs the name.
   */
  nhanDoc?: string;
  /** Whether «Xem» is offered: only while messages go along. */
  xem: boolean;
  /** The one-tap toggle's label, or null when there is nothing to toggle. */
  doi: string | null;
};

export const XEM = "Xem";
export const CHI_GUI_LOI_NHO = "Chỉ gửi lời nhờ";
export const CHUA_SAN_SANG = "Rủ Đi AI chưa sẵn sàng · Gửi như tin thường";

/**
 * @param goi the bundle that would go with the message, or null when this
 *   server takes no bundle at all.
 * @param kemTin false once the person tapped «Chỉ gửi lời nhờ».
 * @param sanSang whether the server can run the command now.
 * @param haiNguoi a two-person conversation: the words speak of two people,
 *   never of «nhóm» (design 2026-09-28).
 */
export function chuChip(goi: BoiCanh | null, kemTin: boolean, sanSang: boolean, haiNguoi = false): ChuChip {
  // The ✦ in the AI's colour already names whose readiness this is, so the
  // line keeps «AI» and drops «Rủ Đi»: it fits one line at 320dp (QA UI-167:
  // two lines, the mark alone on the first). The full sentence stays the
  // chip's spoken name, and the flows that look for it still find it.
  if (!sanSang) return { cau: "AI chưa sẵn sàng · gửi như tin thường", nhanDoc: CHUA_SAN_SANG, xem: false, doi: null };
  if (goi === null) return { cau: "Chỉ gửi lời nhờ, không kèm tin nào", xem: false, doi: null };
  const n = goi.luot.length;
  if (n === 0) return { cau: haiNguoi ? "Hai bạn chưa có tin nào, chỉ gửi lời nhờ" : "Nhóm chưa có tin nào, chỉ gửi lời nhờ", xem: false, doi: null };
  if (!kemTin) return { cau: "Chỉ gửi lời nhờ, không kèm tin nào", xem: false, doi: `Kèm lại ${n} tin` };
  return { cau: `Kèm ${n} tin gần đây`, xem: true, doi: CHI_GUI_LOI_NHO };
}

/** The bundle that actually travels: none at all after «Chỉ gửi lời nhờ». */
export function goiSeGui(goi: BoiCanh | null, kemTin: boolean): BoiCanh | undefined {
  return kemTin && goi !== null && goi.luot.length > 0 ? goi : undefined;
}

/** The head of the «Xem» sheet: the same count, and who else will read what. */
export function cauXem(goi: BoiCanh, haiNguoi = false): string {
  const ai = haiNguoi ? "cả hai bạn" : "cả nhóm";
  return `Rủ Đi AI sẽ đọc đúng ${goi.luot.length} tin dưới đây cùng lời nhờ trong tin của bạn. Lời nhờ và câu trả lời hiện cho ${ai}.`;
}

/** The sheet's second line: what goes along with each message, and whose names. */
export function cauXemCach(haiNguoi = false): string {
  const ten = haiNguoi ? "Tên hiển thị của hai bạn" : "Tên hiển thị của các thành viên";
  return `Ảnh đi bằng chú thích, sticker đi bằng chữ «Sticker», tin đã xoá đi bằng một dòng nói là đã xoá. ${ten} đi kèm để AI biết ai nói gì, còn chữ trong tin nhắn thì đi nguyên văn.`;
}

/** One speaker's consecutive turns in the «Xem» sheet. */
export type DoanXem = { key: string; nguoi: string; cuaToi: boolean; luot: LuotBoiCanh[] };

/**
 * The bundle as a transcript: consecutive turns of one speaker under one name,
 * in the order they go to the model. The sheet used to print «Name: text» for
 * every turn, forty lines of small type with nothing to rest the eye on; this
 * is the same list, said the way a chat says it. Nothing is merged, dropped or
 * reordered -- the count on the chip is still the count here.
 */
export function gomTheoNguoi(luot: readonly LuotBoiCanh[]): DoanXem[] {
  const ra: DoanXem[] = [];
  for (const l of luot) {
    const nguoi = nhanVai(l);
    const cuoi = ra[ra.length - 1];
    if (cuoi && cuoi.nguoi === nguoi) cuoi.luot.push(l);
    else ra.push({ key: l.id, nguoi, cuaToi: l.vai === "toi", luot: [l] });
  }
  return ra;
}
