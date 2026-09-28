/**
 * The words of the chat's «+» tray, which depend on who is in the room.
 *
 * Owner decision 2026-09-28 (two classes): every group and every ordinary
 * two-person conversation is a friends' chat, with exactly a group's tools and
 * Rủ Đi AI -- «Tờ hẹn», `/plan`, `/chia-bill`, `@Rủ Đi`. A two-person
 * conversation where both people turned «Một đôi» on is a couple: the same
 * tools, plus the pair's paper («Tờ giấy»), which the tray adds and never
 * swaps in for «Tờ hẹn». That replaces the earlier rule (pull request 659) that a pair
 * could only ask `hoi` and had «Tờ giấy» in the plan slot.
 *
 * What still differs for two people is wording only: a room of two is never
 * called «hội» or «nhóm» (QA couple 23/09, §11).
 */

/**
 * What the tray's «Hỏi Rủ Đi AI» puts at the start of the composer: the same
 * in every room, since every room can ask for a plan.
 */
export const MO_DAU_HOI_AI = "/plan ";

/** The tray's extra tool for a couple: the pair's paper. */
export const CONG_CU_TO_GIAY = "Tờ giấy";

export interface ChuKhay {
  tieuDePoll: string;
  goiYPoll: string;
  nhanPlan: string;
  /** The «Tờ hẹn» panel's words pointing at the composer. */
  loiHoiAi: string;
}

export function chuKhay(haiNguoi: boolean): ChuKhay {
  return haiNguoi
    ? {
        tieuDePoll: "Hai mình chọn gì?",
        goiYPoll: "Tối nay mình ăn gì?",
        nhanPlan: "Hai bạn muốn đi đâu?",
        loiHoiAi:
          "Gõ @Rủ Đi hoặc /plan cùng lời nhờ ngay trong ô soạn. Tin của bạn hiện cho cả hai bạn như mọi tin khác, và Rủ Đi AI trả lời ngay dưới tin đó. Chip trên nút gửi cho bạn xem trước những tin đi kèm.",
      }
    : {
        tieuDePoll: "Hội mình chọn gì?",
        goiYPoll: "Tối nay hội mình ăn gì?",
        nhanPlan: "Bạn muốn rủ hội đi đâu?",
        loiHoiAi:
          "Gõ @Rủ Đi hoặc /plan cùng lời nhờ ngay trong ô soạn. Tin của bạn hiện cho cả nhóm như mọi tin khác, và Rủ Đi AI trả lời ngay dưới tin đó. Chip trên nút gửi cho bạn xem trước những tin đi kèm.",
      };
}

/** One command the composer suggests when the text starts with «/» or «@». */
export interface LenhGoiY {
  nhan: string;
  goiY: string;
  moTa: string;
}

/**
 * The commands the composer suggests. Choosing one fills the composer and
 * sends nothing (chat-ui-contract.md): an AI request is an ordinary message
 * the person finishes writing, with the preview chip above the send button
 * (ADR-0046). Only `/vote` opens a form, because a poll is not a message.
 *
 * The same four commands in every room; a room of two only words them for two.
 */
export function lenhGoiY(haiNguoi: boolean): readonly LenhGoiY[] {
  return [
    { nhan: "/plan", goiY: "/plan ", moTa: "Rủ Đi AI phác lịch trình" },
    { nhan: "/vote", goiY: "/vote", moTa: "Viết câu hỏi và lựa chọn" },
    {
      nhan: "/chia-bill",
      goiY: "/chia-bill ",
      moTa: haiNguoi ? "Rủ Đi AI gom khoản chi để hai bạn xác nhận" : "Rủ Đi AI gom khoản chi để cả hội xác nhận",
    },
    {
      nhan: "@Rủ Đi",
      goiY: "@Rủ Đi ",
      moTa: haiNguoi ? "Hỏi Rủ Đi AI ngay trong cuộc trò chuyện của hai bạn" : "Hỏi Rủ Đi AI ngay trong nhóm",
    },
  ];
}
