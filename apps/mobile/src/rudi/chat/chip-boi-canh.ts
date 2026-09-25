/**
 * The words of the chip above the send button while a message asks Rủ Đi AI
 * (ADR-0039 §2.3, proposed; keeps ADR-0036 §2.5).
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
import type { BoiCanh } from "../ai/boi-canh";

export type ChuChip = {
  /** The chip's sentence. */
  cau: string;
  /** Whether «Xem» is offered: only while messages go along. */
  xem: boolean;
  /** The one-tap toggle's label, or null when there is nothing to toggle. */
  doi: string | null;
};

export const XEM = "Xem";
export const CHI_GUI_LOI_NHO = "Chỉ gửi lời nhờ";

/**
 * @param goi the bundle that would go with the message, or null when this
 *   server takes no bundle at all.
 * @param kemTin false once the person tapped «Chỉ gửi lời nhờ».
 * @param sanSang whether the server can run the command now.
 */
export function chuChip(goi: BoiCanh | null, kemTin: boolean, sanSang: boolean): ChuChip {
  if (!sanSang) return { cau: "Rủ Đi AI chưa sẵn sàng · Gửi như tin thường", xem: false, doi: null };
  if (goi === null) return { cau: "Chỉ gửi lời nhờ, không kèm tin nào", xem: false, doi: null };
  const n = goi.luot.length;
  if (n === 0) return { cau: "Nhóm chưa có tin nào, chỉ gửi lời nhờ", xem: false, doi: null };
  if (!kemTin) return { cau: "Chỉ gửi lời nhờ, không kèm tin nào", xem: false, doi: `Kèm lại ${n} tin` };
  return { cau: `Kèm ${n} tin gần đây`, xem: true, doi: CHI_GUI_LOI_NHO };
}

/** The bundle that actually travels: none at all after «Chỉ gửi lời nhờ». */
export function goiSeGui(goi: BoiCanh | null, kemTin: boolean): BoiCanh | undefined {
  return kemTin && goi !== null && goi.luot.length > 0 ? goi : undefined;
}

/** The head of the «Xem» sheet: the same count, and who else will read what. */
export function cauXem(goi: BoiCanh): string {
  return `Rủ Đi AI sẽ đọc đúng ${goi.luot.length} tin dưới đây cùng lời nhờ trong tin của bạn. Lời nhờ và câu trả lời hiện cho cả nhóm.`;
}
