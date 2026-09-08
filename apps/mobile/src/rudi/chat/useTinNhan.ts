/**
 * One group's conversation, kept fresh while the screen is in front.
 *
 * - First page (newest 50) on mount; older pages on demand (`napCuHon`).
 * - Forward poll every 4 s while focused and the app is active, and once
 *   right after a send, using the newest cursor held. The server echoes the
 *   cursor on an empty page (BE6), so a quiet group polls in place instead of
 *   re-reading the top.
 * - Read mark: the newest message id goes to `PUT /read-mark` whenever the
 *   held list changes while focused, so the conversation list's unread counts
 *   fall as the person reads.
 *
 * State is a plain object rather than a reducer: five fields, one owner.
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

export function useTinNhan(contextId: string, personId: string) {
  const [trang, setTrang] = useState<TrangThaiChat>({
    tin: [],
    dangNap: true,
    dangNapCu: false,
    hetTinCu: false,
    loi: null,
    hangCho: [],
  });
  const tinRef = useRef<Tin[]>([]);
  // Held in a ref as well as in state, for the same reason `tin` is: a
  // re-render between the press and the reply must not be able to lose the
  // key and turn a retry into a second write (`api.ts`, `attemptFor`).
  const hangRef = useRef<TinChoGui[]>([]);
  // The words of a failed text send, held only for their key. Not in `hangCho`:
  // the composer gets the words back and the notice says why, so a row would be
  // the same news twice -- and an invisible one used to eat the empty state.
  const banNhapRef = useRef<TinChoGui | null>(null);
  const dangFocus = useRef(false);
  const daDanhDau = useRef<string | null>(null);

  const dat = useCallback((tin: Tin[], phan: Partial<TrangThaiChat> = {}) => {
    tinRef.current = tin;
    setTrang((cu) => ({ ...cu, tin, ...phan }));
  }, []);

  const datHang = useCallback((hang: TinChoGui[]) => {
    hangRef.current = hang;
    setTrang((cu) => ({ ...cu, hangCho: hang }));
  }, []);


  // A key stands for one request to one path, so nothing in flight may cross a
  // conversation. `navigate()` to the same route with different params updates
  // this instance instead of remounting it, which is exactly how a draft could
  // have been carried into another group (finish review 09/09, R2).
  useEffect(() => {
    hangRef.current = [];
    banNhapRef.current = null;
    setTrang((cu) => ({ ...cu, hangCho: [] }));
  }, [contextId, personId]);

  const napDau = useCallback(async () => {
    try {
      const page = await docTrangTin(contextId, personId);
      dat(gopTin([], page.messages), { dangNap: false, hetTinCu: !page.has_more, loi: null });
    } catch (error) {
      setTrang((cu) => ({ ...cu, dangNap: false, loi: loiRaChu(error) }));
    }
  }, [contextId, personId, dat]);

  const napMoi = useCallback(async () => {
    const after = cursorMoiNhat(tinRef.current);
    try {
      const page = after === null
        ? await docTrangTin(contextId, personId)
        : await docTrangTin(contextId, personId, { after });
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
   */
  const chay = useCallback(
    async (cho: TinChoGui, goi: (attempt: Attempt) => Promise<TinDaGui>, hienHang = true): Promise<TinDaGui> => {
      if (hienHang) datHang(themVaoHang(hangRef.current, cho));
      try {
        const daGui = await goi(cho.attempt);
        const them: Tin[] = [daGui];
        if (daGui.companion?.message) them.push(daGui.companion.message);
        if (daGui.expense_card) them.push(daGui.expense_card);
        if (hienHang) datHang(boKhoiHang(hangRef.current, cho.attempt.key));
        dat(gopTin(tinRef.current, them), { loi: null });
        void napMoi();
        return daGui;
      } catch (error) {
        if (hienHang) datHang(danhDauLoi(hangRef.current, cho.attempt.key, loiRaChu(error), maLoi(error)));
        throw error;
      }
    },
    [dat, datHang, napMoi],
  );

  const napCuHon = useCallback(async () => {
    const before = cursorCuNhat(tinRef.current);
    if (before === null || trang.hetTinCu || trang.dangNapCu) return;
    setTrang((cu) => ({ ...cu, dangNapCu: true }));
    try {
      const page = await docTrangTin(contextId, personId, { before });
      dat(gopTin(tinRef.current, page.messages), { dangNapCu: false, hetTinCu: !page.has_more });
    } catch (error) {
      setTrang((cu) => ({ ...cu, dangNapCu: false, loi: loiRaChu(error) }));
    }
  }, [contextId, personId, dat, trang.hetTinCu, trang.dangNapCu]);

  useEffect(() => {
    void napDau();
  }, [napDau]);

  // Poll while focused and the app is in the foreground.
  useFocusEffect(
    useCallback(() => {
      dangFocus.current = true;
      let hen: ReturnType<typeof setInterval> | null = null;
      const bat = () => {
        if (hen === null) hen = setInterval(() => void napMoi(), NHIP_POLL_MS);
      };
      const tat = () => {
        if (hen !== null) clearInterval(hen);
        hen = null;
      };
      if (AppState.currentState === "active") bat();
      const sub = AppState.addEventListener("change", (s) => (s === "active" ? bat() : tat()));
      void napMoi();
      return () => {
        dangFocus.current = false;
        tat();
        sub.remove();
      };
    }, [napMoi]),
  );

  // Read mark follows the newest message the person has in front of them.
  useEffect(() => {
    const moiNhat = trang.tin[0]?.id;
    if (!moiNhat || !dangFocus.current || daDanhDau.current === moiNhat) return;
    daDanhDau.current = moiNhat;
    void danhDauDaDoc(contextId, personId, moiNhat).catch(() => undefined);
  }, [trang.tin, contextId, personId]);

  const gui = useCallback(
    async (body: string, traLoi: TrichDan | null = null): Promise<TinDaGui> => {
      // Pressing send again after a failure, with the same words and the same
      // quoted message, is the SAME send: it reuses the key, so if the first
      // request actually landed the second one replays it instead of writing a
      // second message. Anything different is a different send and mints a new
      // key -- the same key with different bytes would be a 422 aimed at
      // somebody who did nothing wrong.
      //
      // The draft lives in a ref rather than in the visible queue, because the
      // composer already holds the words and the notice already says why: a
      // row here would be the same news twice.
      const replyToId = traLoi?.id ?? null;
      const attempt = khoaDungLai(banNhapRef.current, body, replyToId) ?? newAttempt();
      const nhap: TinChoGui = { attempt, kind: "text", than: body, phuDe: null, traLoi, trangThai: "dang-gui", loi: null, thuLaiDuoc: true, luc: new Date().toISOString() };
      banNhapRef.current = nhap;
      try {
        const daGui = await chay(nhap, (a) => guiTin(contextId, personId, body, a, { replyToId }), false);
        banNhapRef.current = null;
        return daGui;
      } catch (error) {
        banNhapRef.current = { ...nhap, trangThai: "that-bai" };
        throw error;
      }
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
    async (imageUrl: string, caption: string | null): Promise<TinDaGui> => {
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
    async (stickerId: string, traLoi: TrichDan | null = null): Promise<TinDaGui> => {
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
      if (cho === null || cho.trangThai !== "that-bai") return null;
      datHang(danhDauThuLai(hangRef.current, khoa));
      // Only stickers and pictures are ever queued: the composer holds the
      // words, so there is no text row here to retry.
      const goi = (a: Attempt): Promise<TinDaGui> =>
        cho.kind === "image"
          ? guiAnh(contextId, personId, cho.than, cho.phuDe, a)
          : guiSticker(contextId, personId, cho.than, a, { replyToId: cho.traLoi?.id ?? null });
      return chay({ ...cho, trangThai: "dang-gui", loi: null }, goi);
    },
    [contextId, personId, chay, datHang],
  );

  /** Give up on a row that failed: it leaves the queue and nothing was written. */
  const boQua = useCallback((khoa: string) => datHang(boKhoiHang(hangRef.current, khoa)), [datHang]);

  /**
   * Take back one's own message. The held row flips at once from the 204
   * (`thayTinDaXoa`), then the next poll confirms the server's row; a refusal
   * surfaces as the server's sentence and the row stays as it was.
   */
  const xoaTinCuaToi = useCallback(
    async (messageId: string): Promise<void> => {
      await xoaTin(contextId, messageId, personId);
      dat(thayTinDaXoa(tinRef.current, messageId, new Date().toISOString()), { loi: null });
      void napMoi();
    },
    [contextId, personId, dat, napMoi],
  );

  const doiPhanUng = useCallback(
    async (messageId: string, kind: LoaiPhanUng, dangCoCuaToi: boolean) => {
      const ket = dangCoCuaToi
        ? await boPhanUng(contextId, messageId, personId, kind)
        : await themPhanUng(contextId, messageId, personId, kind);
      dat(thayPhanUng(tinRef.current, messageId, ket.reactions));
    },
    [contextId, personId, dat],
  );

  return { ...trang, napCuHon, napMoi, gui, guiAnhMoi, guiSticker: guiStickerMoi, thuLaiMot, boQua, xoaTin: xoaTinCuaToi, doiPhanUng, taiLai: napDau };
}
