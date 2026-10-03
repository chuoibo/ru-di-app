import { useCallback, useEffect, useRef, useState } from "react";

import { ApiError, newAttempt } from "../../api";
import { useAiStream } from "../ai/useAiStream";
import type { TraLoiSong } from "../ai/tra-loi-song";
import {
  CHO_TOI_DA_MS,
  LOI_NEP,
  docNep,
  goiNep,
  gomPhien,
  ketCucNep,
  khoaLanHoi,
  nepDuocHoi,
  nhipHoiNepMs,
  type LuotNep,
} from "./hoi";
import type { PhieuNguCanh } from "./phieu";

/**
 * One panel session with Nếp.
 *
 * The session is `useState` and nothing else: no AsyncStorage, no provider
 * above the panel. It ends when the panel closes (`mo` goes false) or the app
 * restarts, which is the whole of ADR-0036 §4 «không giữ phiên Nếp ở máy chủ
 * hay xuống đĩa thiết bị» on this side.
 *
 * Since slice 11 the answer is read while it is written: after the `202` the
 * invocation is followed by `useAiStream` (the SSE stream, and the same
 * `docNep` read as before whenever the stream is not usable). `song` is that
 * answer in flight; when it ends, `ketCucNep` decides what stays: the sealed
 * text becomes a turn, or the words that showed stay beside the sentence.
 *
 * Every late result of an older question is dropped: a question is followed
 * by its invocation id, and a newer question (or closing the panel) replaces
 * that id, which closes the older stream.
 */
export interface PhienNep {
  luot: LuotNep[];
  dangHoi: boolean;
  loi: string | null;
  /** The question out now, drawn as the next turn until its answer lands. */
  cauDangHoi: string | null;
  /** The answer being written, while a question is out. */
  song: TraLoiSong | null;
  /** Words that had shown before the answer failed; printed above `loi`. */
  dangDo: string | null;
  /** The last answer's chips (xong.chips); tapping one fills the input. */
  chips: string[];
  hoi(cau: string): Promise<boolean>;
}

type DangCho = { id: string; cau: string };

export function useNepHoi(actorId: string | null, phieu: PhieuNguCanh | null, mo: boolean): PhienNep {
  const [luot, datLuot] = useState<LuotNep[]>([]);
  const [dangHoi, datDangHoi] = useState(false);
  const [loi, datLoi] = useState<string | null>(null);
  const [cho, datCho] = useState<DangCho | null>(null);
  const [dangDo, datDangDo] = useState<string | null>(null);
  const [chips, datChips] = useState<string[]>([]);
  const doi = useRef(0);
  // The pending question's settle function, so a question that is abandoned
  // (the panel closed, a newer question) still settles instead of hanging.
  const treo = useRef<((v: boolean) => void) | null>(null);
  const lanThu = useRef<{ khoa: string; id: string } | null>(null);
  const luotRef = useRef(luot);
  luotRef.current = luot;

  const docLai = useCallback(
    async (id: string) => (actorId ? docNep(actorId, id) : null),
    [actorId],
  );
  const { traLoi, hetGio } = useAiStream({
    id: cho?.id ?? null,
    actorId,
    phamVi: { kieu: "nep" },
    hoi: docLai,
    nhipHoi: nhipHoiNepMs,
    choToiDaMs: CHO_TOI_DA_MS,
  });

  const xongCau = useCallback((v: boolean) => {
    const f = treo.current;
    treo.current = null;
    f?.(v);
  }, []);

  // Closing the panel ends the session: the turns, the pending question and
  // any error go with it.
  useEffect(() => {
    if (mo) return;
    doi.current += 1;
    xongCau(false);
    lanThu.current = null;
    datLuot([]);
    datDangHoi(false);
    datLoi(null);
    datCho(null);
    datDangDo(null);
    datChips([]);
  }, [mo, xongCau]);

  useEffect(() => () => xongCau(false), [xongCau]);

  // The followed answer ended: keep what the contract says stays.
  useEffect(() => {
    if (!cho) return;
    if (hetGio) {
      datCho(null);
      datDangHoi(false);
      datLoi("Nếp nghĩ lâu quá. Bạn hỏi lại sau một lát nhé.");
      xongCau(false);
      return;
    }
    const ket = ketCucNep(traLoi);
    if (!ket) return;
    const cau = cho.cau;
    datCho(null);
    datDangHoi(false);
    if (ket.kieu === "tra-loi") {
      datLuot((truoc) => [...truoc, { vai: "toi", chu: cau }, { vai: "nep", chu: ket.chu }]);
      datChips(ket.chips);
      xongCau(true);
    } else {
      datDangDo(ket.conLai);
      datLoi(ket.cau);
      xongCau(false);
    }
  }, [cho, traLoi, hetGio, xongCau]);

  const hoi = useCallback(
    async (cau: string) => {
      const chu = cau.trim();
      if (!chu || dangHoi) return false;
      if (!actorId) {
        datLoi("Cần đăng nhập để hỏi Nếp. Đăng nhập rồi thử lại nhé.");
        return false;
      }
      if (!nepDuocHoi(phieu)) {
        datLoi(LOI_NEP.nep_lui_man_tien);
        return false;
      }
      const kem = gomPhien(luotRef.current, chu);
      const khoa = khoaLanHoi(chu, kem, phieu);
      if (!lanThu.current || lanThu.current.khoa !== khoa) lanThu.current = { khoa, id: newAttempt().key };
      const luotNay = ++doi.current;
      xongCau(false);
      datCho(null);
      datLoi(null);
      datDangDo(null);
      datChips([]);
      datDangHoi(true);

      try {
        const job = await goiNep(actorId, chu, lanThu.current.id, kem, phieu);
        if (luotNay !== doi.current) return false;
        lanThu.current = null;
        if (job.status === "succeeded" && job.text) {
          const traLoiNgay = job.text;
          datLuot((truoc) => [...truoc, { vai: "toi", chu }, { vai: "nep", chu: traLoiNgay }]);
          datDangHoi(false);
          return true;
        }
        return await new Promise<boolean>((ketThuc) => {
          treo.current = ketThuc;
          datCho({ id: job.id, cau: chu });
        });
      } catch (error) {
        if (luotNay !== doi.current) return false;
        datDangHoi(false);
        datLoi(error instanceof ApiError ? error.message : "Nếp chưa trả lời được lúc này. Bạn thử lại sau ít phút nhé.");
        return false;
      }
    },
    [actorId, phieu, dangHoi, xongCau],
  );

  return { luot, dangHoi, loi, cauDangHoi: cho?.cau ?? null, song: cho ? traLoi : null, dangDo, chips, hoi };
}
