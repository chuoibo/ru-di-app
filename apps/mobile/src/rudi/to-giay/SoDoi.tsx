import { createContext, useContext, useMemo, useState, type ReactNode } from "react";

import { CUA_FIXTURE_DEV } from "../cua-fixture";
import { CAP_DEMO, NEP_PHAC_MAU, NGUOI_KIA_DEMO, RANG_BUOC_MAU, TOI_DEMO, TO_GIAY_CU } from "./fixtures-doi";
import {
  type RangBuoc,
  boNhap,
  deNghiSua,
  dongSo,
  ghiDaDi,
  giuMotDieu,
  guiTo,
  huyBuoi,
  nghiTuan,
  nguoiKiaDongY,
  nguoiNhanXem,
  phacToGiay,
  rutTo,
  suaNhap,
  toiDongY,
} from "./so-fixture";
import { type NoiDungTo, type ToGiay, demHauQuaDongSo, toUuTien } from "./to-giay";

/**
 * The two-person notebook of the EXPERIENCE build, held in memory.
 *
 * Phase 2 of «Nếp truyền giấy» builds the thirteen surfaces of slice 1 against
 * this provider; Phase 4 swaps it for the routes of ADR-0027 without touching
 * the screens, because the state it hands out is already the wire's shape
 * (`ToGiay` from `to-giay.ts`) and every action maps to one command.
 *
 * Deliberately NOT persisted: `luu-tru.ts` whitelists what the fixture session
 * keeps between launches, and a notebook of fixture papers restored days later
 * would be a plan about nothing (the same reason `profileNotice` is kept out).
 * The Maestro table starts from a clean app anyway.
 *
 * The clock lives here, at the edge: the pure transitions in `so-fixture.ts`
 * take `now` as a parameter, and this is the one place that reads it.
 *
 * The «other person» actions (`nguoiKia*`) exist so a single phone can play
 * both sides of the round trip. They are exposed only under `CUA_FIXTURE_DEV`
 * (a development build with `EXPO_PUBLIC_RUDI_FIXTURE=1`), the same door the
 * fixture login uses; on any other build `nguoiKia` is `null` and no screen
 * can offer them.
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
   * The first read of the notebook has landed. Until then `toMo` being
   * `undefined` says nothing about whether a sheet is open. The fixture is
   * synchronous and always true.
   */
  daNap: boolean;
  /** The command in flight, by name, or null. The fixture never waits. */
  dangLam: string | null;
  /**
   * Why the last command was refused, in the reader's words, or null.
   *
   * Before 23/09 nothing read this: a refused or dropped write left the screen
   * as if it had worked (the «Đừng» box lost every save without a word).
   */
  loiLenh: string | null;
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

  /** The other side of the table, development fixture only. */
  nguoiKia: null | {
    dongYDeNghi: (id: string) => void;
    xem: (toId: string) => void;
    dongY: (toId: string) => void;
    deNghiSua: (toId: string, content: NoiDungTo, lyDo: string | null) => void;
  };
}

/**
 * Exported so a second implementation can stand behind the same door.
 *
 * `SoDoiSong` provides this context from the server instead of from the
 * fixture store, which is why Phase 4 rewrote no screen: every screen asks
 * `useSoDoi()` and neither knows nor needs to know which one answered.
 */
export const SoDoiContext = createContext<SoDoiApi | null>(null);

function seed(): TrangThaiSoDoi {
  return {
    lapSo: true,
    batDoi: false,
    docChat: false,
    luotCuaToi: true,
    coLuot: true,
    xinToBiChan: false,
    rangBuoc: RANG_BUOC_MAU,
    toGiay: TO_GIAY_CU,
    deNghiCho: [],
    daDong: false,
    daNap: true,
    dangLam: null,
    loiLenh: null,
  };
}

const bayGio = () => new Date().toISOString();

export function SoDoiProvider({ children }: { children: ReactNode }) {
  const [s, setS] = useState<TrangThaiSoDoi>(seed);
  const toi = TOI_DEMO.id, kia = NGUOI_KIA_DEMO.id;
  const doi = (f: (ds: readonly ToGiay[]) => ToGiay[]) => setS((c) => (c.daDong ? c : { ...c, toGiay: f(c.toGiay) }));

  const api = useMemo<SoDoiApi>(() => {
    const toMo = s.daDong ? undefined : toUuTien(s.toGiay, toi);
    // Newest sheet first by when it was CREATED (list order), not by its last
    // `sent_at`: a dropped draft has none and sank to the bottom, a withdrawn
    // sheet jumped to the top, and two screens showed the same list in two
    // orders (blind read 12/09).
    const toKhac = [...s.toGiay].filter((t) => t.id !== toMo?.id).reverse();
    const daCoToMo = s.toGiay.some((t) => ["nhap", "da_gui", "da_xem", "de_nghi_sua", "dong_y"].includes(t.state));
    return {
      ...s,
      capId: CAP_DEMO.id,
      toiId: toi,
      nguoiKiaId: kia,
      tenNguoiKia: NGUOI_KIA_DEMO.ten,
      toMo,
      toKhac,
      // The fixture has no clock and no other writer, so its revision is a
      // constant: nothing can change between the count and the close.
      xemTruocDongSo: async () => ({ revision: "fixture", ...demHauQuaDongSo(s.toGiay, s.deNghiCho.length) }),

      deNghiLapSo: () => setS((c) => (c.lapSo || c.deNghiCho.some((d) => d.purpose === "lap_so") ? c : { ...c, deNghiCho: [...c.deNghiCho, { id: `dn-lap-so-${c.deNghiCho.length + 1}`, purpose: "lap_so", cuaToi: true }] })),
      deNghiBatDoi: () => setS((c) => (!c.lapSo || c.batDoi || c.deNghiCho.some((d) => d.purpose === "bat_doi") ? c : { ...c, deNghiCho: [...c.deNghiCho, { id: `dn-bat-doi-${c.deNghiCho.length + 1}`, purpose: "bat_doi", cuaToi: true }] })),
      thuHoiBatDoi: () => setS((c) => ({ ...c, batDoi: false, deNghiCho: c.deNghiCho.filter((d) => d.purpose !== "bat_doi") })),
      datRangBuoc: async (rb) => {
        setS((c) => ({ ...c, rangBuoc: { ...c.rangBuoc, toi: { ...c.rangBuoc.toi, ...rb } } }));
        return true;
      },
      dongSo: () => setS((c) => ({ ...c, daDong: true, deNghiCho: [], toGiay: dongSo(c.toGiay) })),

      // Trên bản trải nghiệm, lời đề nghị luôn là của tôi, nên «đồng ý» ở đây
      // là việc của người kia — cùng một đường với nút dưới `nguoiKia`.
      dongYDeNghi: async (id: string) => {
        api.nguoiKia?.dongYDeNghi(id);
        return true;
      },

      ruDiChoi: () => {
        if (s.daDong || daCoToMo) return null;
        const id = `to-${s.toGiay.length + 1}`;
        doi((ds) => [...ds, phacToGiay(id, NEP_PHAC_MAU)]);
        return id;
      },
      // The fixture has no other writer: there is nothing new to read.
      lamMoi: () => undefined,
      suaNhap: (id, content, lyDo) => doi((ds) => suaNhap(ds, id, content, lyDo)),
      gui: (id) => {
        const now = bayGio();
        setS((c) => (c.daDong ? c : { ...c, luotCuaToi: false, toGiay: guiTo(c.toGiay, id, toi, now) }));
      },
      boNhap: (id) => doi((ds) => boNhap(ds, id)),
      dongY: (id) => doi((ds) => toiDongY(ds, id, toi, `outing-${id}`)),
      deNghiSua: (id, content, lyDo) => {
        const now = bayGio();
        doi((ds) => deNghiSua(ds, id, toi, toi, content, lyDo, now));
      },
      rut: (id) => doi((ds) => rutTo(ds, id, toi)),
      nghiTuan: (id) => doi((ds) => nghiTuan(ds, id)),
      daDi: (id) => doi((ds) => ghiDaDi(ds, id, toi)),
      giu: (id, line) => {
        const now = bayGio();
        doi((ds) => giuMotDieu(ds, id, line, now));
      },
      huy: (id) => doi((ds) => huyBuoi(ds, id)),

      nguoiKia: CUA_FIXTURE_DEV
        ? {
            dongYDeNghi: (id) =>
              setS((c) => {
                const dn = c.deNghiCho.find((d) => d.id === id);
                if (!dn) return c;
                return {
                  ...c,
                  deNghiCho: c.deNghiCho.filter((d) => d.id !== id),
                  lapSo: c.lapSo || dn.purpose === "lap_so",
                  batDoi: c.batDoi || dn.purpose === "bat_doi",
                  docChat: c.docChat || dn.purpose === "doc_chat",
                };
              }),
            xem: (toId) => {
              const now = bayGio();
              doi((ds) => nguoiNhanXem(ds, toId, now));
            },
            dongY: (toId) => doi((ds) => nguoiKiaDongY(ds, toId, toi, `outing-${toId}`)),
            deNghiSua: (toId, content, lyDo) => {
              const now = bayGio();
              setS((c) => (c.daDong ? c : { ...c, luotCuaToi: true, toGiay: deNghiSua(c.toGiay, toId, kia, toi, content, lyDo, now) }));
            },
          }
        : null,
    };
  }, [s, toi, kia]);

  return <SoDoiContext.Provider value={api}>{children}</SoDoiContext.Provider>;
}

export function useSoDoi(): SoDoiApi {
  const api = useContext(SoDoiContext);
  if (!api) throw new Error("useSoDoi cần nằm trong SoDoiProvider");
  return api;
}
