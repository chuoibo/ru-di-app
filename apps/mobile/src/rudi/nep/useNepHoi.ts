import { useCallback, useEffect, useRef, useState } from "react";
import { AppState } from "react-native";

import { ApiError, newAttempt } from "../../api";
import {
  CHO_TOI_DA_MS,
  LOI_NEP,
  cauKetQuaNep,
  conCho,
  docNep,
  goiNep,
  gomPhien,
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
 * Every late result of an older question is dropped by a generation number,
 * the same way `useNepAnh` drops an older drawing.
 */
export interface PhienNep {
  luot: LuotNep[];
  dangHoi: boolean;
  loi: string | null;
  hoi(cau: string): Promise<boolean>;
}

export function useNepHoi(actorId: string | null, phieu: PhieuNguCanh | null, mo: boolean): PhienNep {
  const [luot, datLuot] = useState<LuotNep[]>([]);
  const [dangHoi, datDangHoi] = useState(false);
  const [loi, datLoi] = useState<string | null>(null);
  const doi = useRef(0);
  const hen = useRef<ReturnType<typeof setTimeout> | null>(null);
  // The pending read's settle function, so a question that is abandoned (the
  // panel closed, a newer question) still settles instead of hanging its caller.
  const treo = useRef<((v: boolean) => void) | null>(null);
  const lanThu = useRef<{ khoa: string; id: string } | null>(null);
  const luotRef = useRef(luot);
  luotRef.current = luot;

  const dung = useCallback(() => {
    if (hen.current) clearTimeout(hen.current);
    hen.current = null;
    treo.current?.(false);
    treo.current = null;
  }, []);

  // Closing the panel ends the session: the turns, the pending question and
  // any error go with it.
  useEffect(() => {
    if (mo) return;
    doi.current += 1;
    dung();
    lanThu.current = null;
    datLuot([]);
    datDangHoi(false);
    datLoi(null);
  }, [mo, dung]);

  useEffect(() => dung, [dung]);

  const hoi = useCallback(
    async (cau: string) => {
      const chu = cau.trim();
      if (!chu || dangHoi) return false;
      if (!actorId) {
        datLoi("Bản trải nghiệm chưa hỏi Nếp được. Đăng nhập rồi thử lại nhé.");
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
      dung();
      datLoi(null);
      datDangHoi(true);

      let id: string;
      try {
        const job = await goiNep(actorId, chu, lanThu.current.id, kem, phieu);
        if (luotNay !== doi.current) return false;
        id = job.id;
        if (job.status === "succeeded" && job.text) {
          lanThu.current = null;
          datLuot((truoc) => [...truoc, { vai: "toi", chu }, { vai: "nep", chu: job.text as string }]);
          datDangHoi(false);
          return true;
        }
      } catch (error) {
        if (luotNay !== doi.current) return false;
        datDangHoi(false);
        datLoi(error instanceof ApiError ? error.message : "Nếp chưa trả lời được lúc này. Bạn thử lại sau ít phút nhé.");
        return false;
      }
      lanThu.current = null;

      const batDau = Date.now();
      let n = 0;
      return new Promise<boolean>((ketThuc) => {
        const xong = (v: boolean) => {
          if (treo.current === ketThuc) treo.current = null;
          ketThuc(v);
        };
        treo.current = ketThuc;
        const doc = async () => {
          if (luotNay !== doi.current) return xong(false);
          if (Date.now() - batDau > CHO_TOI_DA_MS) {
            datDangHoi(false);
            datLoi("Nếp nghĩ lâu quá. Bạn hỏi lại sau một lát nhé.");
            return xong(false);
          }
          if (AppState.currentState === "active") {
            try {
              const tin = await docNep(actorId, id);
              if (luotNay !== doi.current) return xong(false);
              if (tin.status === "succeeded" && tin.text) {
                const traLoi = tin.text;
                datLuot((truoc) => [...truoc, { vai: "toi", chu }, { vai: "nep", chu: traLoi }]);
                datDangHoi(false);
                return xong(true);
              }
              if (!conCho(tin.status)) {
                datDangHoi(false);
                datLoi(cauKetQuaNep(tin.code));
                return xong(false);
              }
            } catch {
              // One missed read is not a failed question: the job is still
              // running on the server, so read again.
            }
          }
          n += 1;
          hen.current = setTimeout(doc, nhipHoiNepMs(n));
        };
        hen.current = setTimeout(doc, nhipHoiNepMs(0));
      });
    },
    [actorId, phieu, dangHoi, dung],
  );

  return { luot, dangHoi, loi, hoi };
}
