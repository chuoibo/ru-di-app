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
  /**
   * The line under the tools: a photo sent here can be picked into a public
   * trip diary (any member's diary of an outing in this room reads the room's
   * photos). In a group the diary's keeper is one of many; in a pair it is the
   * other person.
   */
  ghiChuAnh: string;
  /** The action on the pinned shared-plan row while its draft is open. */
  suaChung: string;
}

export function chuKhay(haiNguoi: boolean): ChuKhay {
  return haiNguoi
    ? {
        tieuDePoll: "Hai mình chọn gì?",
        goiYPoll: "Tối nay mình ăn gì?",
        nhanPlan: "Hai bạn muốn đi đâu?",
        loiHoiAi:
          "Gõ @Rủ Đi hoặc /plan cùng lời nhờ ngay trong ô soạn. Tin của bạn hiện cho cả hai bạn như mọi tin khác, và Rủ Đi AI trả lời ngay dưới tin đó. Chip trên nút gửi cho bạn xem trước những tin đi kèm.",
        ghiChuAnh: "Ảnh gửi vào đây có thể được người kia chọn vào một sổ chuyến đi công khai. Nếu muốn gỡ, hãy nhắn người ấy nhé.",
        suaChung: "Sửa cùng nhau",
      }
    : {
        tieuDePoll: "Hội mình chọn gì?",
        goiYPoll: "Tối nay hội mình ăn gì?",
        nhanPlan: "Bạn muốn rủ hội đi đâu?",
        loiHoiAi:
          "Gõ @Rủ Đi hoặc /plan cùng lời nhờ ngay trong ô soạn. Tin của bạn hiện cho cả nhóm như mọi tin khác, và Rủ Đi AI trả lời ngay dưới tin đó. Chip trên nút gửi cho bạn xem trước những tin đi kèm.",
        ghiChuAnh: "Ảnh gửi vào đây có thể được bạn đồng hành chọn vào sổ chuyến đi công khai. Nếu muốn gỡ, hãy nhắn người giữ sổ nhé.",
        suaChung: "Sửa cùng hội",
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

/** Space between two tools of the tray, across and down (dp). */
export const KHOANG_CONG_CU = 8;
/** The tool's paper square at its full size, and the smallest it may shrink to (dp). */
export const ICON_CONG_CU = 56;
export const ICON_CONG_CU_NHO = 44;
/**
 * The widest single word a tool label carries, at caption size and font
 * scale 1 (dp). A label may take two lines, but a word cannot break, so a
 * tool must be at least this wide. Measured in Chromium 2026-09-28 at the
 * caption style (600, 13 px, system sans): «Sticker» 43.4, «chọn» 31.1,
 * «Bình» 28.9, «giấy» 26.0; 50 keeps about 15% of room for another face.
 */
export const CHU_CONG_CU_RONG_NHAT = 50;
/** One label line at caption size and font scale 1 (dp), `typography.caption.lineHeight`. */
const DONG_CHU = 18;
/** Padding above and below a tool, and between its square and its label (dp). */
const DEM_DOC = 10;
const KHOANG_ICON_CHU = 7;

export interface BoCucKhay {
  /** Tools per row. */
  cot: number;
  hang: number;
  /** Width of one tool (dp); every tool is this wide, a lone last-row tool too. */
  oRong: number;
  /** Side of the tool's paper square (dp). */
  icon: number;
  /** Height the tool grid needs to show every label whole (dp). */
  caoNoiDung: number;
}

/**
 * How the tray lays out `soCongCu` tools in a row `rong` dp wide at the
 * system `fontScale`. Bug 2026-09-28 (lab screenshots, 360 dp): five tools of
 * a fixed 62 dp minimum wrapped 4 + 1, the fifth stretched across the row and
 * the height cap cut its label. The rule now:
 *  - one row whenever each tool is still as wide as its widest word (and its
 *    shrunk square), which holds five tools at 320 dp and at 360 dp;
 *  - otherwise the fewest balanced rows (five: 3 + 2, never 4 + 1), every tool
 *    the same width, so no tool is stretched alone;
 *  - the height is computed from the rows and the label lines at this font
 *    scale, so the caller can make the cap at least that tall (`tranKhay`).
 * Four tools at 360 and 390 dp come out as before: one row, each a quarter of
 * the row.
 */
export function boCucKhay(rong: number, soCongCu: number, fontScale = 1): BoCucKhay {
  const n = Math.max(1, Math.floor(soCongCu));
  const tiLe = Number.isFinite(fontScale) ? Math.min(Math.max(fontScale, 1), 2) : 1;
  const r = Number.isFinite(rong) ? Math.max(0, rong) : 0;
  const can = Math.max(ICON_CONG_CU_NHO, Math.ceil(CHU_CONG_CU_RONG_NHAT * tiLe));
  const vuaMotHang = Math.max(1, Math.floor((r + KHOANG_CONG_CU) / (can + KHOANG_CONG_CU)));
  const hang = Math.ceil(n / Math.min(n, vuaMotHang));
  const cot = Math.ceil(n / hang);
  const oRong = Math.max(0, Math.floor(((r - (cot - 1) * KHOANG_CONG_CU) / cot) * 10) / 10);
  const icon = Math.min(ICON_CONG_CU, Math.floor(oRong));
  const oCao = DEM_DOC * 2 + icon + KHOANG_ICON_CHU + 2 * DONG_CHU * tiLe;
  const caoNoiDung = Math.ceil(hang * oCao + (hang - 1) * KHOANG_CONG_CU);
  return { cot, hang, oRong, icon, caoNoiDung };
}

/**
 * The tallest the tray's scrolling box may be. A form (the poll) keeps the old
 * cap, a quarter of the window between 130 and 260 dp, so its send button
 * stays in view. The tool grid is never cut by it: the cap grows to the
 * grid's own height, up to 60% of the window, past which the grid scrolls.
 */
export function tranKhay(caoCuaSo: number, caoLuoi: number | null): number {
  const goc = Math.max(130, Math.min(260, caoCuaSo * 0.25));
  return caoLuoi === null ? goc : Math.max(goc, Math.min(caoLuoi, Math.round(caoCuaSo * 0.6)));
}
