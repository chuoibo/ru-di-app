import { useFocusEffect } from "expo-router";
import { useCallback, useRef, useState } from "react";
import { AppState } from "react-native";
import { ApiError, newAttempt, thongDiepNguoiDoc } from "../../api";
import { docAiInvocations, docChatCapabilities, goiAi, gopAiInvocations, lenhSanSang, thuLaiAi, type AiInvocation, type ChatCapabilities, type LenhAi } from "./ai-invocations";
import { vanTay, type BoiCanh } from "../ai/boi-canh";

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
  const send = async (prompt: string, boiCanh?: BoiCanh, lenh: LenhAi = "plan") => {
    if (sending.current) return false;
    if (!capabilities || !lenhSanSang(capabilities, lenh)) {
      setError(lenh === "chia_bill" ? "AI chưa gom khoản chi được lúc này. Bạn vẫn có thể thêm khoản chi ở mục Chia bill." : "AI chưa sẵn sàng. Bạn vẫn có thể tự tạo kèo.");
      return false;
    }
    // Only attach when the server says it reads a bundle. An older server gets
    // the old body, and nothing needs a flag.
    const dinhKem = capabilities.ai.share_scope === "caller_attached" ? boiCanh : undefined;
    // The key covers the BUNDLE as well as the words. Keyed on the prompt
    // alone, the same question asked again over newer messages reuses the old
    // logical id, the server finds a matching digest and answers 200 with the
    // OLD card, and the person believes the AI just read what they just said.
    // The command is part of it too: the server digests command + prompt +
    // bundle, so one key reused across two commands would be a 409.
    const khoa = `${lenh}\u0000${prompt}\u0000${dinhKem ? vanTay(dinhKem) : ""}`;
    if (!attempt.current || attempt.current.prompt !== khoa) attempt.current = { prompt: khoa, id: newAttempt().key };
    const version = generation.current;
    sending.current = true; setBusy(true); setError(null);
    try {
      const request = await goiAi(contextId, personId, prompt, attempt.current.id, dinhKem, lenh);
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
