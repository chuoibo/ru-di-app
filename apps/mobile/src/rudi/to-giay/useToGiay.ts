/**
 * One notebook and the sheet in front of it, kept fresh while the screen is.
 *
 * Shaped after `chat/useTinNhan.ts`, and for the same reasons: a poll every
 * four seconds while focused and the app is active, and a generation stamp so a
 * reply that lands after the screen has moved on is dropped rather than merged.
 *
 * ## Why nothing moves before the server answers
 *
 * Section 3.3 rule 6: the client never decides what a sheet has become. An
 * optimistic «Đã gửi» would be a promise this screen cannot keep -- the other
 * person may have sent a new version a second ago, and the answer to the send
 * is `paper_version_stale`. So every command runs, and only its answer moves
 * the screen. The cost is one round trip of «đang gửi»; what it buys is that
 * the two of them are never looking at different weeks.
 *
 * ## Why one command at a time
 *
 * `dangLam` holds the name of the command in flight and the screen disables the
 * rest. Two commands on one sheet is how a sheet gets agreed and withdrawn in
 * the same second, and the server would then answer one of them with a refusal
 * that reads as a bug.
 *
 * ## Why a re-read after every command
 *
 * A command answers `{id, state, version, outing_id?}` and never content
 * (ADR-0027 §3), so patching the held sheet from it would leave the screen
 * showing v1's content beside v2's number. The re-read is one GET and it is the
 * only place the sheet is replaced.
 */
import { useFocusEffect } from "expo-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { AppState } from "react-native";

import { ApiError, attemptFor, thongDiepNguoiDoc, type Attempt, quenLuot } from "../../api";
import type { NoiDungTo, ToGiay } from "./to-giay";
import {
  type LoaiRangBuoc,
  type MucDich,
  type SoHaiNguoi,
  type ToTomTat,
  type XemTruocDongSo,
  danhDauDaXem,
  datRangBuoc,
  deNghiDongY,
  deNghiSua,
  docDanhSachTo,
  docSo,
  docTo,
  dongSo,
  dongY,
  dongYDeNghi,
  ghiDaDi,
  giuMotDong,
  guiTo,
  nghiTuan,
  rutTo,
  suaNhapSong,
  tenLuot,
  thuHoiDongY,
  xemTruocDongSo,
  xinToMoi,
  xoaRangBuoc,
} from "./to-giay-song";

export const NHIP_SO_MS = 4000;

export type TrangThaiSo = {
  /** `dang-nap` only on the first read; a poll never takes the screen away. */
  pha: "dang-nap" | "san-sang" | "loi";
  so: SoHaiNguoi | null;
  /** The sheet the paper surface is showing, whole. `null` when there is none. */
  to: ToGiay | null;
  /** Every sheet this person may see, newest first; summaries, no content. */
  ds: readonly ToTomTat[];
  /** The read that failed, in the language of whoever is reading it. */
  loi: string | null;
  /** The name of the command in flight, or null. */
  dangLam: string | null;
  /** Why the last command was refused. Cleared when the next one starts. */
  loiLenh: string | null;
  /**
   * The last refused command and the server's code for it (`paper_wrong_state`),
   * when there was one: what the refusal MEANS can depend on it (B1), and the
   * sentence alone cannot be read back into a code. Cleared with `loiLenh`.
   */
  lenhBiChan: { ten: string; ma: string | null } | null;
};

const DAU: TrangThaiSo = {
  pha: "dang-nap",
  so: null,
  to: null,
  ds: [],
  loi: null,
  dangLam: null,
  loiLenh: null,
  lenhBiChan: null,
};

function loiRaChu(error: unknown): string {
  return error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null);
}

/**
 * Which sheet the paper surface shows when the notebook names none.
 *
 * The server's `open_paper_id` answers «what is in play», which is the whole
 * question while something is. When nothing is, the surface shows the most
 * recent thing that happened rather than an empty page -- the list arrives
 * newest first, so the first plan or memory in it is that.
 */
function toDeMo(so: SoHaiNguoi | null, ds: readonly ToTomTat[]): string | null {
  if (so?.open_paper_id) return so.open_paper_id;
  const kyUc = ds.find((row) => row.state === "chot" || row.state === "da_di" || row.state === "da_giu");
  return kyUc?.id ?? null;
}

/**
 * `nhip: 0` reads on focus and never on a timer.
 *
 * The pinned line in the pair chat is the caller that needs it: it shows one
 * sentence about the sheet, and a second four-second poll running beside the
 * conversation's own would double that screen's traffic to say the same thing
 * a beat sooner.
 */
export function useToGiay(contextId: string, personId: string, { nhip = NHIP_SO_MS }: { nhip?: number } = {}) {
  const [trang, setTrang] = useState<TrangThaiSo>(DAU);
  const theHeRef = useRef(0);
  const dangFocus = useRef(false);
  // Minted on the press and kept, so a retry carries the same key and the
  // server replays its first answer instead of writing twice (`attemptFor`).
  const luotRef = useRef<Record<string, Attempt>>({});
  const dangLamRef = useRef<string | null>(null);

  useEffect(() => {
    setTrang(DAU);
    luotRef.current = {};
    dangLamRef.current = null;
    // The one place the generation moves: React runs this before the next
    // notebook's first read and on unmount, so both leave by the same door.
    return () => {
      theHeRef.current += 1;
    };
  }, [contextId, personId]);

  /**
   * One read of everything the surface shows: the notebook, the list, and the
   * open sheet whole. Three requests rather than one, because that is what the
   * wire is -- a command answer carries no content and the list carries no
   * versions, so the sheet has to be asked for by name.
   */
  const doc = useCallback(
    async (dauTien: boolean) => {
      const theHe = theHeRef.current;
      const goi = { actorId: personId };
      try {
        const [so, danhSach] = await Promise.all([docSo(contextId, goi), docDanhSachTo(contextId, goi)]);
        if (theHe !== theHeRef.current) return;
        const id = toDeMo(so, danhSach.papers);
        const to = id === null ? null : await docTo(id, goi);
        if (theHe !== theHeRef.current) return;
        setTrang((cu) => ({ ...cu, pha: "san-sang", so, ds: danhSach.papers, to, loi: null }));
      } catch (error) {
        if (theHe !== theHeRef.current) return;
        // A failed poll is not news; a failed first read is the screen.
        if (dauTien) setTrang((cu) => ({ ...cu, pha: "loi", loi: loiRaChu(error) }));
      }
    },
    [contextId, personId],
  );

  useEffect(() => {
    void doc(true);
  }, [doc]);

  useFocusEffect(
    useCallback(() => {
      dangFocus.current = true;
      void doc(false);
      if (nhip <= 0) {
        return () => {
          dangFocus.current = false;
        };
      }
      const dongHo = setInterval(() => {
        if (dangFocus.current && AppState.currentState === "active") void doc(false);
      }, nhip);
      return () => {
        dangFocus.current = false;
        clearInterval(dongHo);
      };
    }, [doc, nhip]),
  );

  /**
   * Run one command, then re-read. Returns true when the command landed.
   *
   * `ten` is both the button's lock and the attempt's name, so pressing «Ừ» on
   * v2 after a failed «Ừ» on v1 is a different write with its own key rather
   * than a replay of v1's answer.
   */
  const lam = useCallback(
    async (ten: string, chay: (goi: { actorId: string; attempt: Attempt }) => Promise<unknown>): Promise<boolean> => {
      if (dangLamRef.current !== null) return false;
      const theHe = theHeRef.current;
      dangLamRef.current = ten;
      setTrang((cu) => ({ ...cu, dangLam: ten, loiLenh: null, lenhBiChan: null }));
      try {
        await chay({ actorId: personId, attempt: attemptFor(luotRef.current, ten) });
        if (theHe !== theHeRef.current) return false;
        // Landed: the next command under this name is a new write, not a retry.
        quenLuot(luotRef.current, ten);
        dangLamRef.current = null;
        setTrang((cu) => ({ ...cu, dangLam: null }));
        await doc(false);
        return true;
      } catch (error) {
        if (theHe !== theHeRef.current) return false;
        dangLamRef.current = null;
        setTrang((cu) => ({ ...cu, dangLam: null, loiLenh: loiRaChu(error), lenhBiChan: { ten, ma: error instanceof ApiError ? error.code : null } }));
        // A refusal is usually news about what the other person did, so the
        // screen re-reads before the person decides what to do about it.
        await doc(false);
        return false;
      }
    },
    [doc, personId],
  );

  const toId = trang.to?.id ?? null;
  const phienBanHienTai = trang.to?.version ?? 0;

  return {
    ...trang,
    lamMoi: () => doc(false),

    // --- sổ ---
    xinLapSo: () => lam("lap-so", (goi) => deNghiDongY(contextId, "lap_so", goi)),
    xinBac: (purpose: MucDich) => lam(`de-nghi:${purpose}`, (goi) => deNghiDongY(contextId, purpose, goi)),
    dongYDeNghiNay: (proposalId: string) =>
      lam(`dong-y-de-nghi:${proposalId}`, (goi) => dongYDeNghi(contextId, proposalId, goi)),
    thuHoi: (purpose: MucDich) => lam(`thu-hoi:${purpose}`, (goi) => thuHoiDongY(contextId, purpose, goi)),
    datRangBuocNay: (kind: LoaiRangBuoc, content: string) =>
      lam(`rang-buoc:${kind}`, (goi) => datRangBuoc(contextId, kind, content, goi)),
    xoaRangBuocNay: (kind: LoaiRangBuoc) => lam(`xoa-rang-buoc:${kind}`, (goi) => xoaRangBuoc(contextId, kind, goi)),
    xemTruocDong: (): Promise<XemTruocDongSo> => xemTruocDongSo(contextId, { actorId: personId }),
    dongSoNay: (revision: string) => lam("dong-so", (goi) => dongSo(contextId, revision, goi)),

    // --- tờ giấy ---
    xinTo: () => lam("xin-to", (goi) => xinToMoi(contextId, goi)),
    suaNhap: (content: NoiDungTo, lyDo: string | null) =>
      toId === null ? Promise.resolve(false) : lam(tenLuot("sua", toId), (goi) => suaNhapSong(toId, content, lyDo, goi)),
    gui: () =>
      toId === null
        ? Promise.resolve(false)
        : lam(tenLuot("gui", toId, phienBanHienTai), (goi) => guiTo(toId, phienBanHienTai, goi)),
    daXem: () =>
      toId === null
        ? Promise.resolve(false)
        : lam(tenLuot("xem", toId, phienBanHienTai), (goi) => danhDauDaXem(toId, phienBanHienTai, goi)),
    dongYTo: () =>
      toId === null
        ? Promise.resolve(false)
        : lam(tenLuot("dong-y", toId, phienBanHienTai), (goi) => dongY(toId, phienBanHienTai, goi)),
    deNghiSuaTo: (content: NoiDungTo, lyDo: string | null) =>
      toId === null
        ? Promise.resolve(false)
        : lam(tenLuot("de-nghi-sua", toId, phienBanHienTai), (goi) =>
            deNghiSua(toId, phienBanHienTai, content, lyDo, goi),
          ),
    rut: () =>
      toId === null
        ? Promise.resolve(false)
        : lam(tenLuot("rut", toId, phienBanHienTai), (goi) => rutTo(toId, phienBanHienTai, goi)),
    nghiTuanNay: () => (toId === null ? Promise.resolve(false) : lam(tenLuot("nghi", toId), (goi) => nghiTuan(toId, goi))),
    ghiDaDiRoi: () => (toId === null ? Promise.resolve(false) : lam(tenLuot("da-di", toId), (goi) => ghiDaDi(toId, goi))),
    giuDong: (line: string) =>
      toId === null ? Promise.resolve(false) : lam(tenLuot("giu", toId), (goi) => giuMotDong(toId, line, goi)),
  };
}
