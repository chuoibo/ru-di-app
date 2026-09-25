/**
 * «Người lo» of the week, as sentences (ADR-0034 §2.4).
 *
 * The server decides who leads: inferred from what the two did in this cycle
 * (a sheet sent first counts two, a «đề nghị sửa» one; a tie goes to whoever
 * opened the notebook), or what one of them chose for the week. Nothing about
 * who they are goes in -- there is no gender anywhere. This module only
 * words it, from the reader's side.
 */
import type { VaiTuan } from "./to-giay-song";

export interface CauVai {
  /** «Tuần này bạn lo» / «Tuần này Minh lo» / «Tuần này hai bạn cùng lo». */
  nhan: string;
  /** Why, in one short line. */
  vi: string;
  /** Is the reader one of this week's leads? */
  laToi: boolean;
}

export function cauVaiTuan(vai: VaiTuan | null | undefined, toiId: string, tenNguoiKia: string): CauVai | null {
  if (!vai || vai.nguoi_lo.length === 0) return null;
  const ten = tenNguoiKia.trim() || "Người ấy";
  const laToi = vai.nguoi_lo.includes(toiId);
  const caHai = vai.nguoi_lo.length > 1;
  const nhan = caHai ? "Tuần này hai bạn cùng lo" : laToi ? "Tuần này bạn lo" : `Tuần này ${ten} lo`;
  if (vai.cach === "chon") {
    return { nhan, vi: caHai ? "Đã chọn «Hôm nay mình share»." : "Đã chọn cho tuần này.", laToi };
  }
  const diemCua = (id: string) => vai.diem.find((d) => d.person_id === id)?.score ?? 0;
  const lo = vai.nguoi_lo[0];
  const khac = vai.diem.find((d) => d.person_id !== lo);
  const vi =
    diemCua(lo) === 0
      ? "Chưa ai gửi tờ nào, nên người lập sổ lo trước."
      : khac !== undefined && khac.score === diemCua(lo)
        ? "Hai bạn chủ động như nhau, nên người lập sổ lo trước."
        : laToi
          ? "Bạn hay gửi tờ trước và đề nghị sửa."
          : `${ten} hay gửi tờ trước và đề nghị sửa.`;
  return { nhan, vi, laToi };
}
