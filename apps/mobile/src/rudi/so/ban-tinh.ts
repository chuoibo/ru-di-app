/**
 * The character of a notebook: the ONE place the app spells out how the kinds
 * of notebook differ (spec «Nếp truyền giấy» §13.3, §17).
 *
 * There are no modes. There are notebooks, each of a kind, and a person opens
 * one or another. Everything a kind changes -- how a decision is reached, whether
 * there are two roles, how often Nếp speaks, what Nếp may do, the words on the
 * buttons, how money is shown -- is a field here, and nothing else in `src/`
 * branches on the kind. `tests/so-ban-tinh-mot-cho.test.mjs` counts.
 *
 * This is PRESENTATION POLICY. It says what a screen offers and how it words
 * it. It proves nothing about who may read or write what: authorization is
 * the server's, at every read and write, and a client that lets a button
 * appear has not granted anything (spec §13.3, Codex round two).
 */

export const LOAI_SO = ["hoi", "hai-nguoi", "doi"] as const;
export type LoaiSo = (typeof LOAI_SO)[number];

/** What Nếp is allowed to do in a notebook of this kind. Absent means silent. */
export type ViecNep = "phac-to" | "hoi-tuan" | "on-the" | "nhac-ngay" | "dan-truoc";

export interface BanTinhSo {
  /** How an outing is decided: the group votes; two people pass a sheet. */
  quyetDinh: "phieu" | "to-giay";
  /** Two named roles in an outing (Người lo / Người chấm), or none. */
  coVai: boolean;
  /** Nếp's speaking quota, per notebook (spec §6.3). Zero means never. */
  nhip: { toMoiTuan: number; lanLaMoiThang: number; nhacMoiThang: number };
  nepDuocLam: readonly ViecNep[];
  /** The words. Vietnamese, no em dash (`tests/dau-gach-dai.test.mjs`). */
  tuVung: { goiTapThe: string; cauMo: string; nutMoLoi: string; tenKhongGian: string };
  /** Money as a ledger of who owes whom, or as what the two spent together. */
  tienHien: "chia-bill" | "chi-tieu-chung";
}

export const BAN_TINH: Record<LoaiSo, BanTinhSo> = {
  // The group notebook: what ships today, unchanged. Nếp holds a seat and says
  // nothing on its own (spec §12.4: Nếp in the group keeps its old role).
  hoi: {
    quyetDinh: "phieu",
    coVai: false,
    nhip: { toMoiTuan: 0, lanLaMoiThang: 0, nhacMoiThang: 0 },
    nepDuocLam: [],
    tuVung: { goiTapThe: "cả hội", cauMo: "Đi đâu cả hội?", nutMoLoi: "Rủ cả hội", tenKhongGian: "Kế hoạch" },
    tienHien: "chia-bill",
  },
  // Two friends who opened a notebook: sheets are passed, nothing else is on.
  // Consent tier 2 of spec §7.2; tier 3 turns this into `doi`.
  "hai-nguoi": {
    quyetDinh: "to-giay",
    coVai: false,
    nhip: { toMoiTuan: 1, lanLaMoiThang: 0, nhacMoiThang: 0 },
    nepDuocLam: ["phac-to"],
    tuVung: { goiTapThe: "hai bạn", cauMo: "Đi đâu không?", nutMoLoi: "Rủ đi chơi", tenKhongGian: "Tờ giấy của hai mình" },
    tienHien: "chi-tieu-chung",
  },
  // A couple: two roles, the weekly sheet, the private notebook, Nếp's four jobs.
  doi: {
    quyetDinh: "to-giay",
    coVai: true,
    nhip: { toMoiTuan: 1, lanLaMoiThang: 1, nhacMoiThang: 2 },
    nepDuocLam: ["phac-to", "hoi-tuan", "on-the", "nhac-ngay", "dan-truoc"],
    tuVung: { goiTapThe: "hai bạn", cauMo: "Tối nay tụi mình làm gì?", nutMoLoi: "Rủ đi chơi", tenKhongGian: "Tờ giấy của hai mình" },
    tienHien: "chi-tieu-chung",
  },
};

/**
 * Which kind a conversation is, from the two facts the server states: its
 * `kind` (`group` or `pair`, ADR-0021 §2.5) and whether the couple tier is on
 * for it (consent tier 3, both people, in force). A `pair` is never a couple
 * by itself (ADR-0027, spec §3.3).
 */
export function loaiSoCua(nhom: { kind?: "group" | "pair" }, doi: { bat: boolean } | null): LoaiSo {
  if (nhom.kind !== "pair") return "hoi";
  return doi?.bat ? "doi" : "hai-nguoi";
}

export function banTinhCua(loai: LoaiSo): BanTinhSo {
  return BAN_TINH[loai];
}
