import { useFocusEffect } from "expo-router";
import { useCallback, useRef, useState } from "react";
import { AppState } from "react-native";
import { ApiError, newAttempt, thongDiepNguoiDoc } from "../../api";
import { docAiInvocations, docChatCapabilities, goiAi, gopAiInvocations, thuLaiAi, type AiInvocation, type ChatCapabilities } from "./ai-invocations";

export function useChatAi(contextId: string, personId: string) {
  const [capabilities, setCapabilities] = useState<ChatCapabilities | null>(null);
  const [requests, setRequests] = useState<AiInvocation[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const generation = useRef(0);
  const attempt = useRef<{ prompt: string; id: string } | null>(null);
  const sending = useRef(false);
  const requestsRef = useRef(requests);
  requestsRef.current = requests;
  useFocusEffect(useCallback(() => {
    let disposed = false;
    let reading = false;
    generation.current += 1;
    setCapabilities(null); setRequests([]); setError(null); setBusy(false);
    attempt.current = null;
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
    void refresh(true);
    const timer = setInterval(() => {
      if (requestsRef.current.some((request) => request.status === "queued" || request.status === "running")) void refresh();
    }, 2000);
    const sub = AppState.addEventListener("change", (state) => { if (state === "active") void refresh(true); });
    return () => { disposed = true; generation.current += 1; clearInterval(timer); sub.remove(); };
  }, [contextId, personId]));
  const send = async (prompt: string) => {
    if (sending.current) return false;
    if (!capabilities?.ai.plan.available) { setError("AI chưa sẵn sàng. Bạn vẫn có thể tự tạo kèo."); return false; }
    if (!attempt.current || attempt.current.prompt !== prompt) attempt.current = { prompt, id: newAttempt().key };
    const version = generation.current;
    sending.current = true; setBusy(true); setError(null);
    try {
      const request = await goiAi(contextId, personId, prompt, attempt.current.id);
      if (version !== generation.current) return false;
      setRequests((held) => gopAiInvocations(held, [request]));
      attempt.current = null;
      return true;
    } catch (cause) {
      if (version === generation.current) setError(cause instanceof ApiError ? cause.message : thongDiepNguoiDoc(0, null));
      return false;
    } finally { sending.current = false; if (version === generation.current) setBusy(false); }
  };
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
  return { capabilities, requests, busy, error, send, retry };
}
