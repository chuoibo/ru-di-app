import { createContext, useContext } from "react";

import type { RangBuoc } from "./so-fixture";
import type { GuSo } from "./gu-doi";
import type { ChonLo, VaiTuan } from "./to-giay-song";
import type { NoiDungTo, ToGiay } from "./to-giay";

/**
 * The two-person notebook as every screen of it reads it: `useSoDoi()`.
 *
 * `SoDoiSong.tsx` is the one provider, and it answers from the server
 * (ADR-0027). The state is the wire's shape (`ToGiay` from `to-giay.ts`) and
 * every action maps to one command.
 */

export interface TrangThaiSoDoi {
  /** Consent tier 2: the notebook exists (both agreed to open one). */
  lapSo: boolean;
  /** Consent tier 3: «Một đôi». Never implied by tier 2. */
  batDoi: boolean;
  /** Consent tier 4: Nếp may read the chat. Default OFF; slice 2 adds the switch. */
  docChat: boolean;
  /** Whose turn to open the week. Slice 1 alternates by who sent last. */
  luotCuaToi: boolean;
  /**
   * Whether `luotCuaToi` means anything here. The fixture keeps a turn; the
   * live notebook has none on the wire (ADR-0027 §6.3), so the screen must not
   * tell a person «Tuần này bạn mở lời» on a guess (QC 24/09, B1).
   */
  coLuot: boolean;
  /**
   * The last «Rủ đi chơi» was refused because this week already has a sheet
   * this person cannot see (the other person's private draft): the surface
   * waits for it instead of offering the same failing button (B1).
   */
  xinToBiChan: boolean;
  rangBuoc: { toi: RangBuoc; nguoiKia: RangBuoc };
  toGiay: readonly ToGiay[];
  /** Consent proposals still waiting for the other person. */
  /**
   * Lời đề nghị đang chờ, kèm AI đã đề nghị.
   *
   * `cuaToi` không phải trang trí: người ĐƯỢC đề nghị là người duy nhất bấm
   * đồng ý được, và bản đầu của màn hiện cùng một câu «chờ người ấy đồng ý»
   * cho cả hai phía — nên trên máy thật, người nhận lời đề nghị không có cửa
   * nào để trả lời. Đo được ở vòng native 14/09.
   */
  deNghiCho: readonly { id: string; purpose: "lap_so" | "bat_doi" | "doc_chat"; cuaToi: boolean }[];
  daDong: boolean;
  /**
   * The pair has stopped: one of the two blocked the other, or the other
   * account ended (ADR-0023 §2.3.2). The server refuses every outward write of
   * the notebook from then on (ADR-0027 §3, QA UI-120), so the screen stops
   * offering them and says ONE sentence for both causes -- which of the two it
   * was is not this screen's to tell. Sheets already written stay readable.
   */
  daDung: boolean;
  /** Taste in «Một đôi», per person (ADR-0034); null outside it. */
  gu: GuSo | null;
  /** «Người lo» of this week (ADR-0034 §2.4); null outside an open «Một đôi». */
  vai: VaiTuan | null;
  /**
   * The first read of the notebook has landed. Until then `toMo` being
   * `undefined` says nothing about whether a sheet is open. The fixture is
   * synchronous and always true.
   */
  daNap: boolean;
  /**
   * The first read failed, in the reader's words, or null (QA UI-083). While
   * set, nothing in the notebook is known: the screen says it could not read
   * it and offers «Thử lại», never «Chưa có sổ» and «Đề nghị lập sổ».
   */
  loiDoc: string | null;
  /** The command in flight, by name, or null. The fixture never waits. */
  dangLam: string | null;
  /**
   * Why the last command was refused, in the reader's words, or null.
   *
   * Before 23/09 nothing read this: a refused or dropped write left the screen
   * as if it had worked (the «Đừng» box lost every save without a word).
   */
  loiLenh: string | null;
  /**
   * A failed press of the taste sheet, worded for that sheet (`cauLoiGu`), or
   * null. Said inside the sheet: the body's `loiLenh` line sits under the
   * scrim, where nobody reads it (QA UI-129).
   */
  loiGu: string | null;
}

export interface SoDoiApi extends TrangThaiSoDoi {
  capId: string;
  toiId: string;
  nguoiKiaId: string;
  tenNguoiKia: string;
  /** THE open sheet of the paper surface (spec §15.1), or `undefined`. */
  toMo: ToGiay | undefined;
  /** Rows under the open sheet: everything else, newest first. */
  toKhac: readonly ToGiay[];
  /**
   * What closing would do, asked for rather than read.
   *
   * A promise because on the live build this is a POST: the count and the
   * `revision` that pins it are minted together, so that the rows somebody
   * agreed to close and the rows being closed are provably the same rows. The
   * fixture answers immediately; the screen holds `null` until either does.
   */
  xemTruocDongSo: () => Promise<{ revision: string; so_nhap_bo: number; so_to_huy: number; so_to_khoa: number; so_de_nghi_huy: number }>;

  deNghiLapSo: () => void;
  deNghiBatDoi: () => void;
  thuHoiBatDoi: () => void;
  /** My own `chia_gu` switch: on, and off again. */
  chiaGu: () => void;
  thoiChiaGu: () => void;
  /**
   * «Bật lại cho chat» (ADR-0048 §3.2): my switch off, then on again, so it is
   * granted under the wording that names the chat.
   */
  batLaiChiaGu: () => void;
  /** «Anh lo / Em lo / Hôm nay mình share» for this week. */
  chonLo: (lo: ChonLo) => void;
  /** Resolves true once every changed box has landed; false if any did not. */
  datRangBuoc: (rb: Partial<RangBuoc>) => Promise<boolean>;
  /** `revision` is the one the person just read. The fixture ignores it. */
  dongSo: (revision: string) => void;

  /** Đồng ý một lời đề nghị người kia vừa gửi. */
  dongYDeNghi: (id: string) => Promise<boolean>;

  /** «Rủ đi chơi»: Nếp drafts a sheet for me. */
  ruDiChoi: () => void;
  /** Read the notebook again now (a wait on the other person is news only they can make). */
  lamMoi: () => void;
  suaNhap: (id: string, content: NoiDungTo, lyDo: string | null) => void;
  gui: (id: string) => void;
  boNhap: (id: string) => void;
  dongY: (id: string) => void;
  deNghiSua: (id: string, content: NoiDungTo, lyDo: string | null) => void;
  rut: (id: string) => void;
  nghiTuan: (id: string) => void;
  daDi: (id: string) => void;
  giu: (id: string, line: string) => void;
  huy: (id: string) => void;

}

/** Provided by `SoDoiSong.tsx`. */
export const SoDoiContext = createContext<SoDoiApi | null>(null);

export function useSoDoi(): SoDoiApi {
  const api = useContext(SoDoiContext);
  if (!api) throw new Error("useSoDoi cần nằm trong SoDoiSongProvider");
  return api;
}
