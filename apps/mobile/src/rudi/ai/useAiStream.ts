import { useEffect, useRef, useState } from "react";
import { AppState, Platform } from "react-native";

import { BASE_URL } from "../../api";
import { actorHeaders } from "../../danh-tinh";
import { TRA_LOI_DAU, theoDoiTraLoi, type DocLoiGoi, type TraLoiSong } from "./tra-loi-song";

/**
 * Follows one AI invocation on screen: the stream while it works, the
 * invocation read when it does not (`tra-loi-song.ts` holds every rule; this
 * is only the React and platform wiring).
 *
 * Scope `nep` reads `GET /me/nep/ai-invocations/{id}/events`; scope `nhom`
 * reads `GET /contexts/{c}/ai-invocations/{id}/events`, the requester's own
 * view (other members come with the WS `ai` frame, slice 12).
 *
 * `hoi` is the screen's EXISTING read of the invocation, so falling back
 * reuses what the screen already polls. It is read through a ref: a new
 * function identity on every render must not reopen the stream.
 */
export type PhamViLuong = { kieu: "nep" } | { kieu: "nhom"; contextId: string };

export interface TuyChonAiStream {
  /** The invocation to follow; null follows nothing and resets. */
  id: string | null;
  actorId: string | null;
  phamVi: PhamViLuong;
  hoi(id: string): Promise<DocLoiGoi | null>;
  nhipHoi(n: number): number;
  choToiDaMs: number;
}

export interface AiStream {
  traLoi: TraLoiSong;
  /** Polling ran out of time with no end. */
  hetGio: boolean;
}

/** Whether a fetch body can be read as it arrives on this runtime. */
export function coTheDocLuong(): boolean {
  return typeof ReadableStream !== "undefined" && typeof TextDecoder !== "undefined";
}

export function duongLuong(phamVi: PhamViLuong, id: string): string {
  const inv = encodeURIComponent(id);
  return phamVi.kieu === "nep"
    ? `/me/nep/ai-invocations/${inv}/events`
    : `/contexts/${encodeURIComponent(phamVi.contextId)}/ai-invocations/${inv}/events`;
}

export function useAiStream(o: TuyChonAiStream): AiStream {
  // Keyed by the invocation it belongs to, so a render between a new id and
  // the effect that resets never shows the previous answer under the new id.
  const [theo, datTheo] = useState<{ id: string | null; traLoi: TraLoiSong; hetGio: boolean }>({
    id: null,
    traLoi: TRA_LOI_DAU,
    hetGio: false,
  });
  const hoiRef = useRef(o.hoi);
  hoiRef.current = o.hoi;
  const nhipRef = useRef(o.nhipHoi);
  nhipRef.current = o.nhipHoi;
  const contextId = o.phamVi.kieu === "nhom" ? o.phamVi.contextId : null;

  useEffect(() => {
    const id = o.id;
    datTheo({ id, traLoi: TRA_LOI_DAU, hetGio: false });
    if (!id || !o.actorId) return;
    const phamVi: PhamViLuong = contextId === null ? { kieu: "nep" } : { kieu: "nhom", contextId };
    const theoDoi = theoDoiTraLoi({
      url: coTheDocLuong() ? BASE_URL + duongLuong(phamVi, id) : null,
      headers: actorHeaders(o.actorId, undefined, contextId ?? undefined),
      viTriQua: Platform.OS === "web" ? "query" : "header",
      hoi: () => hoiRef.current(id),
      nhipHoi: (n) => nhipRef.current(n),
      choToiDaMs: o.choToiDaMs,
      khiDoi: (traLoi) => datTheo((t) => (t.id === id ? { ...t, traLoi } : t)),
      khiHetGio: () => datTheo((t) => (t.id === id ? { ...t, hetGio: true } : t)),
      duocDoc: () => AppState.currentState === "active",
    });
    return () => theoDoi.dong();
  }, [o.id, o.actorId, contextId, o.choToiDaMs]);

  if (theo.id !== o.id) return { traLoi: TRA_LOI_DAU, hetGio: false };
  return { traLoi: theo.traLoi, hetGio: theo.hetGio };
}
