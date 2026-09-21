/**
 * One group's conversation, kept fresh while the screen is in front.
 *
 * - First page (newest 50) on mount; older pages on demand (`napCuHon`).
 * - Forward poll every 4 s while focused and the app is active, and once
 *   right after a send, using the newest cursor held. The server echoes the
 *   cursor on an empty page (BE6), so a quiet group polls in place instead of
 *   re-reading the top.
 * - Read marks follow viewport reports, never fetched messages. Writes are
 *   serialized and acknowledged only after success while active and focused.
 *
 * State is a plain object rather than a reducer: five fields, one owner.
 *
 * Generations: every request is stamped with the conversation it was made for,
 * and a reply that arrives after the hook has moved on -- another group,
 * another person, or unmounted -- is dropped, never merged (audit native
 * 09/09, F42). See `theHeRef`.
 */
import { useFocusEffect } from "expo-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { AppState } from "react-native";

import { ApiError, newAttempt, thongDiepNguoiDoc, type Attempt } from "../../api";
import {
  boKhoiHang,
  danhDauLoi,
  danhDauThuLai,
  khoaDungLai,
  themVaoHang,
  timTrongHang,
  type TinChoGui,
} from "./hang-cho";
import {
  boPhanUng,
  cursorCuNhat,
  cursorMoiNhat,
  danhDauDaDoc,
  docTrangTin,
  gopTin,
  guiAnh,
  guiSticker,
  guiTin,
  thayPhanUng,
  thayTinDaXoa,
  themPhanUng,
  xoaTin,
  type LoaiPhanUng,
  type Tin,
  type TinDaGui,
  type TrichDan,
} from "./tin-song";

export const NHIP_POLL_MS = 4000;

export type TrangThaiChat = {
  tin: Tin[];
  dangNap: boolean;
  dangNapCu: boolean;
  hetTinCu: boolean;
  loi: string | null;
  /**
   * What is on its way or has failed, newest first. Deliberately NOT part of
   * `tin`: that list is the server's, and `cursorMoiNhat` polls from its head,
   * so a row with an invented cursor at the front would poison every poll.
   */
  hangCho: TinChoGui[];
};

function loiRaChu(error: unknown): string {
  return error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null);
}

const TRANG_DAU: TrangThaiChat = { tin: [], dangNap: true, dangNapCu: false, hetTinCu: false, loi: null, hangCho: [] };

export function useTinNhan(contextId: string, personId: string) {
  const [trang, setTrang] = useState<TrangThaiChat>(TRANG_DAU);
  const tinRef = useRef<Tin[]>(TRANG_DAU.tin);
  // Held in a ref as well as in state, for the same reason `tin` is: a
  // re-render between the press and the reply must not be able to lose the
  // key and turn a retry into a second write (`api.ts`, `attemptFor`).
  const hangRef = useRef<TinChoGui[]>([]);
  const dangFocus = useRef(false);
  const daDanhDau = useRef<string | null>(null);
  const daThay = useRef<string | null>(null);
  const dangGhiDoc = useRef<number | null>(null);
  // Which conversation, and which mount, the requests in flight belong to.
  // Read before the first await of every async path below and compared after
  // each one; a mismatch means the reply is for a conversation that has left
  // the screen, and it is dropped in full -- success, failure, and the poll a
  // success would have triggered.
  const theHeRef = useRef(0);
  const chuSoHuu = useRef({ contextId, personId, active: true });

  const dat = useCallback((tin: Tin[], phan: Partial<TrangThaiChat> = {}) => {
    tinRef.current = tin;
    setTrang((cu) => ({ ...cu, tin, ...phan }));
  }, []);

  const datHang = useCallback((hang: TinChoGui[]) => {
    hangRef.current = hang;
    setTrang((cu) => ({ ...cu, hangCho: hang }));
  }, []);


  // One conversation, one generation. `navigate()` to the same route with
  // different params updates this instance instead of remounting it (the route
  // now keys the screen by id as well, but a person change and an unmount still
  // arrive here), and a reply for the previous conversation must be dropped
  // rather than merged into this one: a sticker sent in group A that landed
  // after the switch to B used to be written into B's list, and the poll that
  // followed read A with B's cursor (audit native 09/09, F42; finish review
  // 09/09, R2 for the draft). The cleanup is the ONE place the generation
  // moves. React runs it before the next conversation's first read is issued,
  // and on unmount, so both are the same door.
  //
  // The queue does not follow the person: a row of A that fails after the
  // switch loses its retry button, and coming back to A re-reads the first
  // page, where anything that did land is already waiting.
  useEffect(() => {
    chuSoHuu.current = { contextId, personId, active: true };
    tinRef.current = TRANG_DAU.tin;
    hangRef.current = [];
    daThay.current = null;
    dangGhiDoc.current = null;
    daDanhDau.current = null;
    setTrang(TRANG_DAU);
    return () => {
      chuSoHuu.current.active = false;
      theHeRef.current += 1;
    };
  }, [contextId, personId]);

  const napDau = useCallback(async () => {
    const theHe = theHeRef.current;
    try {
      const page = await docTrangTin(contextId, personId);
      if (theHe !== theHeRef.current) return;
      dat(gopTin(tinRef.current, page.messages), { dangNap: false, hetTinCu: !page.has_more, loi: null });
    } catch (error) {
      if (theHe !== theHeRef.current) return;
      setTrang((cu) => ({ ...cu, dangNap: false, loi: loiRaChu(error) }));
    }
  }, [contextId, personId, dat]);

  const napMoi = useCallback(async () => {
    const theHe = theHeRef.current;
    const after = cursorMoiNhat(tinRef.current);
    try {
      const page = after === null
        ? await docTrangTin(contextId, personId)
        : await docTrangTin(contextId, personId, { after });
      if (theHe !== theHeRef.current) return;
      if (page.messages.length > 0) dat(gopTin(tinRef.current, page.messages), { loi: null });
    } catch {
      // A missed poll is not an error the person needs to read; the next tick
      // tries again and a send surfaces its own failure.
    }
  }, [contextId, personId, dat]);

  /** The code behind a refusal, so a permanent one offers no retry. */
  const maLoi = (error: unknown): string | null => (error instanceof ApiError ? error.code : null);

  /**
   * One logical send, from the press to the server's row.
   *
   * `attempt` is minted by the caller on the press and then carried by the
   * queued row, so `thuLaiMot` below hands the SAME key back to the server.
   * A replay returns the original 201 and `gopTin` dedupes it against the row
   * already held, so a retry cannot leave two messages behind.
   *
   * Resolves to `null` when the reply is for a conversation that has since left
   * the screen: nothing is written, nothing is thrown, and the poll is NOT run
   * -- `napMoi` here is closed over the old context, so it would read the old
   * group with the new group's cursor. Callers treat `null` as «not ours».
   */
  const chay = useCallback(
    async (cho: TinChoGui, goi: (attempt: Attempt) => Promise<TinDaGui>): Promise<TinDaGui | null> => {
      const chu = chuSoHuu.current;
      if (!chu.active || chu.contextId !== contextId || chu.personId !== personId) return null;
      const theHe = theHeRef.current;
      datHang(themVaoHang(hangRef.current, cho));
      try {
        const daGui = await goi(cho.attempt);
        if (theHe !== theHeRef.current) return null;
        const them: Tin[] = [daGui];
        if (daGui.companion?.message) them.push(daGui.companion.message);
        if (daGui.expense_card) them.push(daGui.expense_card);
        datHang(boKhoiHang(hangRef.current, cho.attempt.key));
        dat(gopTin(tinRef.current, them), { loi: null });
        void napMoi();
        return daGui;
      } catch (error) {
        if (theHe !== theHeRef.current) return null;
        datHang(danhDauLoi(hangRef.current, cho.attempt.key, loiRaChu(error), maLoi(error)));
        throw error;
      }
    },
    [contextId, personId, dat, datHang, napMoi],
  );

  const napCuHon = useCallback(async () => {
    const before = cursorCuNhat(tinRef.current);
    if (before === null || trang.hetTinCu || trang.dangNapCu) return;
    const theHe = theHeRef.current;
    setTrang((cu) => ({ ...cu, dangNapCu: true }));
    try {
      const page = await docTrangTin(contextId, personId, { before });
      if (theHe !== theHeRef.current) return;
      dat(gopTin(tinRef.current, page.messages), { dangNapCu: false, hetTinCu: !page.has_more });
    } catch (error) {
      if (theHe !== theHeRef.current) return;
      setTrang((cu) => ({ ...cu, dangNapCu: false, loi: loiRaChu(error) }));
    }
  }, [contextId, personId, dat, trang.hetTinCu, trang.dangNapCu]);

  // One in-flight read acknowledgement per conversation. A failed request
  // leaves the target pending; polling/focus/visibility retries it. Never
  // advance the local acknowledged watermark before the server answers.
  const ghiDaDoc = useCallback(async () => {
    if (dangGhiDoc.current !== null || !dangFocus.current || AppState.currentState !== "active") return;
    const theHe = theHeRef.current;
    dangGhiDoc.current = theHe;
    try {
      while (theHe === theHeRef.current && dangFocus.current && AppState.currentState === "active") {
        const id = daThay.current;
        if (id === null || id === daDanhDau.current) break;
        await danhDauDaDoc(contextId, personId, id);
        if (theHe !== theHeRef.current) return;
        daDanhDau.current = id;
      }
    } catch {
      // Retain the target for the next active tick, without blocking reading.
    } finally {
      if (theHe === theHeRef.current) dangGhiDoc.current = null;
    }
  }, [contextId, personId]);

  const danhDauHienThi = useCallback((ids: readonly string[]) => {
    if (!dangFocus.current || AppState.currentState !== "active") return;
    const theHeTin = tinRef.current;
    const thay = new Set(ids);
    const moi = theHeTin.findIndex((t) => thay.has(t.id));
    if (moi < 0) return;
    const cu = theHeTin.findIndex((t) => t.id === daThay.current);
    if (cu < 0 || moi < cu) daThay.current = theHeTin[moi].id;
    void ghiDaDoc();
  }, [ghiDaDoc]);

  useEffect(() => {
    void napDau();
  }, [napDau]);

  useFocusEffect(
    useCallback(() => {
      dangFocus.current = true;
      let hen: ReturnType<typeof setInterval> | null = null;
      const dongBo = () => { void napMoi(); void ghiDaDoc(); };
      const bat = () => {
        if (hen === null) hen = setInterval(dongBo, NHIP_POLL_MS);
        dongBo();
      };
      const tat = () => {
        if (hen !== null) clearInterval(hen);
        hen = null;
      };
      if (AppState.currentState === "active") bat();
      const sub = AppState.addEventListener("change", (s) => (s === "active" ? bat() : tat()));
      return () => {
        dangFocus.current = false;
        tat();
        sub.remove();
      };
    }, [napMoi, ghiDaDoc]),
  );

  const gui = useCallback(
    async (body: string, traLoi: TrichDan | null = null): Promise<TinDaGui | null> => {
      // A deliberate send is a new logical message. Failed messages keep
      // their original bytes and key in the visible queue, where retry lives.
      const nhap: TinChoGui = {
        attempt: newAttempt(), kind: "text", than: body, phuDe: null, traLoi,
        trangThai: "dang-gui", loi: null, thuLaiDuoc: true, luc: new Date().toISOString(),
      };
      return chay(nhap, (a) => guiTin(contextId, personId, body, a, { replyToId: traLoi?.id ?? null }));
    },
    [contextId, personId, chay],
  );

  /**
   * Upload one picked photo into this group, then say it in the feed.
   *
   * Two server calls with one press, in this order on purpose: the message
   * cannot exist before the address it points at. A failed upload therefore
   * leaves no half-message behind, and the caller sees the upload's own words.
   */
  const guiAnhMoi = useCallback(
    async (imageUrl: string, caption: string | null): Promise<TinDaGui | null> => {
      // The caption is part of the request body, so it is part of what the key
      // stands for: retrying with it dropped would send the same key with
      // different bytes and earn a 422 for a message that is already there.
      const cu = hangRef.current.find((t) => t.kind === "image" && t.than === imageUrl && t.trangThai === "that-bai") ?? null;
      const attempt = khoaDungLai(cu, imageUrl, null, caption) ?? newAttempt();
      if (cu !== null && cu.attempt.key !== attempt.key) datHang(boKhoiHang(hangRef.current, cu.attempt.key));
      return chay(
        { attempt, kind: "image", than: imageUrl, phuDe: caption, traLoi: null, trangThai: "dang-gui", loi: null, thuLaiDuoc: true, luc: new Date().toISOString() },
        (a) => guiAnh(contextId, personId, imageUrl, caption, a),
      );
    },
    [contextId, personId, chay, datHang],
  );

  /** One sticker, by id; the server refuses an id outside the vocabulary. */
  const guiStickerMoi = useCallback(
    async (stickerId: string, traLoi: TrichDan | null = null): Promise<TinDaGui | null> => {
      // Every press is its own send: choosing the same sticker twice on
      // purpose is two messages, and each carries its own key.
      return chay(
        { attempt: newAttempt(), kind: "sticker", than: stickerId, phuDe: null, traLoi, trangThai: "dang-gui", loi: null, thuLaiDuoc: true, luc: new Date().toISOString() },
        (a) => guiSticker(contextId, personId, stickerId, a, { replyToId: traLoi?.id ?? null }),
      );
    },
    [contextId, personId, chay],
  );

  /**
   * Send that row again, with the key it was minted with.
   *
   * This is the difference between a retry and a second send: the server
   * recognises the key, and either writes the message once or replays the
   * answer it already gave. Nothing here mints a new attempt.
   */
  const thuLaiMot = useCallback(
    async (khoa: string): Promise<TinDaGui | null> => {
      const cho = timTrongHang(hangRef.current, khoa);
      if (cho === null || cho.trangThai !== "that-bai" || !cho.thuLaiDuoc) return null;
      datHang(danhDauThuLai(hangRef.current, khoa));
      const goi = (a: Attempt): Promise<TinDaGui> =>
        cho.kind === "image"
          ? guiAnh(contextId, personId, cho.than, cho.phuDe, a)
          : cho.kind === "text"
            ? guiTin(contextId, personId, cho.than, a, { replyToId: cho.traLoi?.id ?? null })
            : guiSticker(contextId, personId, cho.than, a, { replyToId: cho.traLoi?.id ?? null });
      return chay({ ...cho, trangThai: "dang-gui", loi: null }, goi);
    },
    [contextId, personId, chay, datHang],
  );

  /** Dismiss a failed row locally. A lost response may still have committed. */
  const boQua = useCallback((khoa: string) => datHang(boKhoiHang(hangRef.current, khoa)), [datHang]);

  /**
   * Take back one's own message. The held row flips at once from the 204
   * (`thayTinDaXoa`), then the next poll confirms the server's row; a refusal
   * surfaces as the server's sentence and the row stays as it was.
   */
  const xoaTinCuaToi = useCallback(
    async (messageId: string): Promise<void> => {
      const theHe = theHeRef.current;
      try {
        await xoaTin(contextId, messageId, personId);
      } catch (error) {
        if (theHe !== theHeRef.current) return;
        throw error;
      }
      if (theHe !== theHeRef.current) return;
      dat(thayTinDaXoa(tinRef.current, messageId, new Date().toISOString()), { loi: null });
      void napMoi();
    },
    [contextId, personId, dat, napMoi],
  );

  const doiPhanUng = useCallback(
    async (messageId: string, kind: LoaiPhanUng, dangCoCuaToi: boolean) => {
      const theHe = theHeRef.current;
      let ket;
      try {
        ket = dangCoCuaToi
          ? await boPhanUng(contextId, messageId, personId, kind)
          : await themPhanUng(contextId, messageId, personId, kind);
      } catch (error) {
        if (theHe !== theHeRef.current) return;
        throw error;
      }
      if (theHe !== theHeRef.current) return;
      dat(thayPhanUng(tinRef.current, messageId, ket.reactions));
    },
    [contextId, personId, dat],
  );

  return { ...trang, danhDauHienThi, napCuHon, napMoi, gui, guiAnhMoi, guiSticker: guiStickerMoi, thuLaiMot, boQua, xoaTin: xoaTinCuaToi, doiPhanUng, taiLai: napDau };
}
