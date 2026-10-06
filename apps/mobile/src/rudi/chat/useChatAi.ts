import { useFocusEffect } from "expo-router";
import { useCallback, useRef, useState } from "react";
import { AppState } from "react-native";
import { ApiError, thongDiepNguoiDoc } from "../../api";
import { docAiInvocations, docChatCapabilities, goiAi, gopAiInvocations, thuLaiAi, type AiInvocation, type ChatCapabilities, type LenhAi } from "./ai-invocations";
import type { BoiCanh } from "../ai/boi-canh";

/**
 * The second half of an `@Rủ Đi` send (ADR-0046, proposed): the message is
 * already stored, and this is the explicit call that asks the AI to answer it.
 *
 * Held in memory only, like the send queue it follows. The key is the
 * message's own attempt key, and the bundle is the one frozen when the person
 * pressed send, so «Thử lại» replays the same call byte for byte and the
 * server answers from its idempotency record instead of refusing a conflict.
 */
export type CapChoAi = {
  /** The send's attempt key, reused as the invocation's logical id. */
  khoa: string;
  lenh: LenhAi;
  loiNho: string;
  /** The stored `@Rủ Đi` message. */
  trigger: string;
  /** Frozen at the press; undefined after «Chỉ gửi lời nhờ». */
  goi: BoiCanh | undefined;
  /** The server's sentence when the call failed; null while in flight. */
  loi: string | null;
};

/**
 * How often a two-person room re-reads what it is. Only the other person's
 * «Một đôi» can turn a friends' pair into a couple, and no message or change
 * frame says so; five seconds keeps the pinned paper row and the fifth tool
 * within a breath of their agreement without leaving the chat (QA UI-125).
 */
export const NHIP_DOC_LAI_PHONG_DOI_MS = 5000;

export function useChatAi(contextId: string, personId: string, { haiNguoi = false }: { haiNguoi?: boolean } = {}) {
  const [capabilities, setCapabilities] = useState<ChatCapabilities | null>(null);
  // Which room the held answers belong to. Coming back to the same room keeps
  // them while they are re-read: blanking them on every focus drew the room as
  // a friends' pair for a beat and the pinned row jumped 78dp (QA UI-125).
  const phongDaDoc = useRef<string | null>(null);
  const [requests, setRequests] = useState<AiInvocation[]>([]);
  const [cho, setCho] = useState<CapChoAi[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const generation = useRef(0);
  const sending = useRef(false);
  const requestsRef = useRef(requests);
  requestsRef.current = requests;
  const capabilitiesRef = useRef(capabilities);
  capabilitiesRef.current = capabilities;
  const refreshRef = useRef<(() => Promise<void>) | null>(null);
  useFocusEffect(useCallback(() => {
    let disposed = false;
    let reading = false;
    generation.current += 1;
    const phong = `${contextId}:${personId}`;
    if (phongDaDoc.current !== phong) {
      phongDaDoc.current = phong;
      setCapabilities(null); setRequests([]); setCho([]);
    }
    setError(null); setBusy(false);
    const refresh = async (capabilitiesToo = false) => {
      if (reading || disposed || AppState.currentState !== "active") return;
      reading = true;
      try {
        if (capabilitiesToo) {
          const caps = await docChatCapabilities(contextId, personId);
          if (disposed) return;
          setCapabilities(caps);
        }
        const result = await docAiInvocations(contextId, personId);
        if (!disposed) setRequests((held) => gopAiInvocations(held, result.invocations));
      } catch { /* Capabilities fail closed; older servers cannot enable AI. */ }
      finally { reading = false; }
    };
    refreshRef.current = () => refresh();
    void refresh(true);
    const timer = setInterval(() => {
      if (requestsRef.current.some((request) => request.status === "queued" || request.status === "running")) void refresh();
    }, 2000);
    const doiPhong = haiNguoi ? setInterval(() => { void refresh(true); }, NHIP_DOC_LAI_PHONG_DOI_MS) : null;
    const sub = AppState.addEventListener("change", (state) => { if (state === "active") void refresh(true); });
    return () => { disposed = true; refreshRef.current = null; generation.current += 1; clearInterval(timer); if (doiPhong) clearInterval(doiPhong); sub.remove(); };
  }, [contextId, personId, haiNguoi]));

  /**
   * Ask the AI to answer a stored message. The bundle goes only when the
   * server reads one, and the trigger only when the server says it answers in
   * the thread: an older server keeps receiving exactly the old body.
   */
  const goiCap = async (cap: Omit<CapChoAi, "loi">) => {
    const caps = capabilitiesRef.current;
    const version = generation.current;
    setCho((held) => [{ ...cap, loi: null }, ...held.filter((c) => c.khoa !== cap.khoa)]);
    try {
      const goi = caps?.ai.share_scope === "caller_attached" ? cap.goi : undefined;
      const trigger = caps?.ai.mention === true ? cap.trigger : undefined;
      const request = await goiAi(contextId, personId, cap.loiNho, cap.khoa, goi, cap.lenh, trigger);
      if (version !== generation.current) return;
      setRequests((held) => gopAiInvocations(held, [request]));
      setCho((held) => held.filter((c) => c.khoa !== cap.khoa));
    } catch (cause) {
      if (version !== generation.current) return;
      const loi = cause instanceof ApiError ? cause.message : thongDiepNguoiDoc(0, null);
      setCho((held) => held.map((c) => (c.khoa === cap.khoa ? { ...c, loi } : c)));
    }
  };
  /** «Thử lại» on a failed pair: the same key, the same frozen bundle. */
  const thuLaiCap = (khoa: string) => {
    const cap = cho.find((c) => c.khoa === khoa);
    if (cap && cap.loi !== null) void goiCap(cap);
  };
  /** «Bỏ»: the message stays in the thread; only the request is dropped. */
  const boCap = (khoa: string) => setCho((held) => held.filter((c) => c.khoa !== khoa));

  const retry = async (id: string) => {
    if (sending.current) return;
    const version = generation.current;
    sending.current = true; setBusy(true); setError(null);
    try {
      const request = await thuLaiAi(contextId, personId, id);
      if (version === generation.current) setRequests((held) => gopAiInvocations(held, [request]));
    } catch (cause) {
      if (version === generation.current) setError(cause instanceof ApiError ? cause.message : thongDiepNguoiDoc(0, null));
    } finally { sending.current = false; if (version === generation.current) setBusy(false); }
  };
  /**
   * One invocation as the list this hook already polls last saw it: the
   * streamed reply's fallback reads here instead of adding a second poll.
   */
  const docMot = useCallback(async (id: string) => requestsRef.current.find((r) => r.id === id) ?? null, []);
  /** Read the list now (a stream just ended, so its row is about to change). */
  const lamMoi = useCallback(() => { void refreshRef.current?.(); }, []);
  return { capabilities, requests, cho, busy, error, goiCap, thuLaiCap, boCap, retry, docMot, lamMoi };
}
