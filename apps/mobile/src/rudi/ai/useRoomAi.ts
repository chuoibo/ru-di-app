import { useEffect, useMemo, useState } from "react";

import { buocPhong, donPhong, KHO_PHONG_TRONG, type KhoPhongAi, type KhungAi } from "./phong-ai";

/**
 * The room's answers as the change feed's `ai` frames bring them (slice 12).
 * Every rule is in `phong-ai.ts`; this is the React wiring: a store the
 * frames fold into, emptied on each new socket, pruned every few seconds.
 *
 * `nhan` is what `useChatChanges` hands the frames to. Its identity is
 * stable, so passing it never reopens the socket.
 */
export interface NhanKhungAi {
  /** One frame, already read by `docKhungAi`. */
  khung(k: KhungAi): void;
  /** A new socket: the server replays the room, so start from empty. */
  moi(): void;
}

const NHIP_DON_MS = 5_000;

export function useRoomAi(contextId: string): { kho: KhoPhongAi; nhan: NhanKhungAi } {
  const [kho, datKho] = useState<KhoPhongAi>(KHO_PHONG_TRONG);
  useEffect(() => {
    datKho(KHO_PHONG_TRONG);
  }, [contextId]);
  useEffect(() => {
    const h = setInterval(() => datKho((k) => donPhong(k, Date.now())), NHIP_DON_MS);
    return () => clearInterval(h);
  }, []);
  const nhan = useMemo<NhanKhungAi>(
    () => ({
      khung: (k) => datKho((cu) => buocPhong(cu, k, Date.now())),
      moi: () => datKho(KHO_PHONG_TRONG),
    }),
    [],
  );
  return { kho, nhan };
}
