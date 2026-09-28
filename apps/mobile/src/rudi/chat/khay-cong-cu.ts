/**
 * The words of the chat's «+» tray, which depend on who is in the room.
 *
 * QA 23/09 (couple, §11): in a two-person conversation the tray offered «Tờ
 * hẹn» (an AI-drafted plan) beside the pinned «Tờ giấy của hai mình» -- two
 * names for one job -- and asked «Bạn muốn rủ hội đi đâu?» of two people. In
 * a pair the paper is the plan: the tray's plan slot opens it instead, and the
 * explicit AI path (`@Rủ Đi …`) keeps its panel, worded for two.
 */
export interface ChuKhay {
  tieuDePoll: string;
  goiYPoll: string;
  nhanPlan: string;
  /** The tray's fourth tool: the pair's paper, or the AI plan panel. */
  congCuHen: { label: string; dich: "to-giay" | "plan" };
}

export function chuKhay(haiNguoi: boolean): ChuKhay {
  return haiNguoi
    ? {
        tieuDePoll: "Hai mình chọn gì?",
        goiYPoll: "Tối nay mình ăn gì?",
        nhanPlan: "Hai bạn muốn đi đâu?",
        congCuHen: { label: "Tờ giấy", dich: "to-giay" },
      }
    : {
        tieuDePoll: "Hội mình chọn gì?",
        goiYPoll: "Tối nay hội mình ăn gì?",
        nhanPlan: "Bạn muốn rủ hội đi đâu?",
        congCuHen: { label: "Tờ hẹn", dich: "plan" },
      };
}
