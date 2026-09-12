/**
 * The experience build's two-person notebook: one pair conversation and the
 * papers in it, hand-written in the wire's shape so every screen renders the
 * same fields it will read from the server in Phase 4.
 *
 * Names are ROLE names («Người ấy», «Bạn»), never a real person's: fixtures
 * ship in the binary and land in screenshots pinned into the repo.
 *
 * Kept apart from `fixtures.ts` on purpose: `tests/art-ky-hoa.test.mjs` reads
 * that file as text and counts its places by a `name:`/`tags:`/`category:`
 * pattern, so new fixtures with similar keys would change what it counts.
 */
import type { RangBuoc, NepDuLieuPhac } from "./so-fixture";
import type { ToGiay } from "./to-giay";

/** Who I am on the fixture build, and who the other person is. */
export const TOI_DEMO = { id: "toi", ten: "Bạn" } as const;
export const NGUOI_KIA_DEMO = { id: "nguoi-ay", ten: "Người ấy" } as const;

/** The pair conversation. Its id is how the group chat route recognises the fixture notebook. */
export const CAP_DEMO = {
  id: "cap-demo",
  kind: "pair" as const,
  display_name: NGUOI_KIA_DEMO.ten,
} as const;

/** What Nếp knows this week: a routine of three dinners in one area, and one place they have not tried. */
export const NEP_PHAC_MAU: NepDuLieuPhac = {
  ngay: "Thứ Bảy 20/09",
  choMoi: { viec: "Ăn tối, một quán chưa đi", gio: "18:30" },
  diTiep: { viec: "Đi bộ, rồi chè", gio: "20:00" },
  lyDo: "Ba tuần liền hai bạn ăn ở cùng một khu.",
};

export const RANG_BUOC_MAU: { toi: RangBuoc; nguoiKia: RangBuoc } = {
  toi: { khong_an_duoc: "Hải sản", dung: "Đừng rủ sau 21:00 ngày thường." },
  nguoiKia: { khong_an_duoc: "Cay", dung: "Đừng chọn chỗ phải xếp hàng." },
};

const CHANG = (gio: string, viec: string) => ({ gio, viec, place_id: null, can_kiem: true });
const NOI_DUNG = (ngay: string, ...chang: ReturnType<typeof CHANG>[]) => ({ ngay, chang });

/**
 * Papers from earlier weeks, so the paper space has a past to show under the
 * one open sheet: a plan that happened and kept a line, a week that expired.
 * Dates are fixture dates; nothing reads the clock.
 */
export const TO_GIAY_CU: readonly ToGiay[] = [
  {
    id: "to-w36",
    state: "da_giu",
    version: 2,
    author_type: "human",
    sent_by: NGUOI_KIA_DEMO.id,
    versions: [
      {
        version: 1,
        content: NOI_DUNG("Thứ Bảy 06/09", CHANG("18:00", "Bún chả, quán góc phố")),
        ly_do: null,
        sent_at: "2026-09-03T12:00:00Z",
        sent_by: TOI_DEMO.id,
        author_type: "human",
        my_response: null,
        their_agreed: false,
        viewed_by_recipient_at: "2026-09-03T13:00:00Z",
      },
      {
        version: 2,
        content: NOI_DUNG("Thứ Bảy 06/09", CHANG("18:30", "Bún chả, quán góc phố"), CHANG("20:00", "Đi bộ bờ kè")),
        ly_do: "Thêm đi bộ cho đỡ no.",
        sent_at: "2026-09-03T14:00:00Z",
        sent_by: NGUOI_KIA_DEMO.id,
        author_type: "human",
        my_response: "dong_y",
        their_agreed: true,
        viewed_by_recipient_at: null,
      },
    ],
    outing_id: "outing-w36",
    keeps: [{ id: "to-w36-giu-1", line: "Hàng chè đầu hẻm, lần sau lại.", created_at: "2026-09-06T15:00:00Z" }],
  },
  {
    id: "to-w37",
    state: "het_han",
    version: 1,
    author_type: "human",
    sent_by: TOI_DEMO.id,
    versions: [
      {
        version: 1,
        content: NOI_DUNG("Chủ nhật 14/09", CHANG("10:00", "Cà phê sáng, chỗ có sân")),
        ly_do: null,
        sent_at: "2026-09-10T09:00:00Z",
        sent_by: TOI_DEMO.id,
        author_type: "human",
        my_response: null,
        their_agreed: false,
        viewed_by_recipient_at: "2026-09-10T20:00:00Z",
      },
    ],
    outing_id: null,
    keeps: [],
  },
];
