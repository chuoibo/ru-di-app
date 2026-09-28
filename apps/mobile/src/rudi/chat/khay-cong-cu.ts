/**
 * The words of the chat's «+» tray, which depend on who is in the room.
 *
 * QA 23/09 (couple, §11): in a two-person conversation the tray offered «Tờ
 * hẹn» (an AI-drafted plan) beside the pinned «Tờ giấy của hai mình» -- two
 * names for one job -- and asked «Bạn muốn rủ hội đi đâu?» of two people. In
 * a pair the paper is the plan: the tray's plan slot opens it instead.
 *
 * Design 2026-09-28 (Rủ Đi AI in a two-person chat): a pair can ask Rủ Đi AI
 * too, with `hoi` only (the plan, the expense sheet and «thành kèo» stay with
 * groups). Since the plan slot is the paper there, the pair's tray carries its
 * own way to the AI, which starts an `@Rủ Đi` message rather than a `/plan`.
 */
import type { LenhAi } from "./ai-invocations";

export interface ChuKhay {
  tieuDePoll: string;
  goiYPoll: string;
  nhanPlan: string;
  /** The tray's fourth tool: the pair's paper, or the AI plan panel. */
  congCuHen: { label: string; dich: "to-giay" | "plan" };
  /** The command the tray's AI path asks for: what its readiness is read from. */
  lenhAi: LenhAi;
  /** What the tray's «Hỏi Rủ Đi AI» puts at the start of the composer. */
  moDauHoiAi: string;
  /**
   * The pair's own way to the AI on the tools panel (its plan slot is the
   * paper), with the words shown when the server cannot answer here; null in
   * a group, whose AI path is the «Tờ hẹn» panel.
   */
  hoiAiTrenKhay: { label: string; chuaSanSang: string } | null;
}

export function chuKhay(haiNguoi: boolean): ChuKhay {
  return haiNguoi
    ? {
        tieuDePoll: "Hai mình chọn gì?",
        goiYPoll: "Tối nay mình ăn gì?",
        nhanPlan: "Hai bạn muốn đi đâu?",
        congCuHen: { label: "Tờ giấy", dich: "to-giay" },
        lenhAi: "hoi",
        moDauHoiAi: "@Rủ Đi ",
        hoiAiTrenKhay: { label: "Hỏi Rủ Đi AI", chuaSanSang: "Rủ Đi AI chưa trả lời được ở đây. Hai bạn vẫn nhắn tin như thường." },
      }
    : {
        tieuDePoll: "Hội mình chọn gì?",
        goiYPoll: "Tối nay hội mình ăn gì?",
        nhanPlan: "Bạn muốn rủ hội đi đâu?",
        congCuHen: { label: "Tờ hẹn", dich: "plan" },
        lenhAi: "plan",
        moDauHoiAi: "/plan ",
        hoiAiTrenKhay: null,
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
 * A pair is never offered `/plan` or `/chia-bill`: the server refuses them
 * there, and a suggestion that ends in a refusal is worse than none.
 */
export function lenhGoiY(haiNguoi: boolean): readonly LenhGoiY[] {
  return haiNguoi
    ? [
        { nhan: "/vote", goiY: "/vote", moTa: "Viết câu hỏi và lựa chọn" },
        { nhan: "@Rủ Đi", goiY: "@Rủ Đi ", moTa: "Hỏi Rủ Đi AI ngay trong cuộc trò chuyện của hai bạn" },
      ]
    : [
        { nhan: "/plan", goiY: "/plan ", moTa: "Rủ Đi AI phác lịch trình" },
        { nhan: "/vote", goiY: "/vote", moTa: "Viết câu hỏi và lựa chọn" },
        { nhan: "/chia-bill", goiY: "/chia-bill ", moTa: "Rủ Đi AI gom khoản chi để cả hội xác nhận" },
        { nhan: "@Rủ Đi", goiY: "@Rủ Đi ", moTa: "Hỏi Rủ Đi AI ngay trong nhóm" },
      ];
}
