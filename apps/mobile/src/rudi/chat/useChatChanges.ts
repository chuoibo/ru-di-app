import { useFocusEffect } from "expo-router";
import { useCallback, useRef, useState } from "react";
import { AppState } from "react-native";
import { ApiError, BASE_URL, tokenPhienHienTai } from "../../api";
import { docAnhChupChat, docThayDoi, docTrangThayDoi, gopBinhChon, type AnhChupChat, type BinhChonSong, type TrangThayDoi } from "./thay-doi";

/** The legacy feed is explicit; it is never a plaintext fallback for v2. */
export function useChatChanges(contextId: string, personId: string, apply: (snapshot: AnhChupChat) => void) {
  const [votes, setVotes] = useState<Record<string, BinhChonSong>>({});
  const [connection, setConnection] = useState<"connecting" | "live" | "recovering" | "unsupported">("connecting");
  const applyRef = useRef(apply);
  applyRef.current = apply;
  useFocusEffect(useCallback(() => {
    let disposed = false;
    let generation = 0;
    let after: number | null = null;
    let busy = false;
    let socket: WebSocket | null = null;
    let timer: ReturnType<typeof setInterval> | null = null;
    let reconnectAfter = 0;
    let failures = 0;
    let unsupported = false;
    let lastReconcile = 0;
    setVotes({});
    const active = (version: number) => !disposed && version === generation && AppState.currentState === "active";
    const accept = (snapshot: AnhChupChat) => {
      if (snapshot.context_id !== contextId) throw new Error("Wrong conversation snapshot");
      applyRef.current(snapshot);
      setVotes((current) => gopBinhChon(current, snapshot.votes));
    };
    const page = async (raw: unknown, version: number) => {
      const parsed = docTrangThayDoi(raw, contextId, after ?? 0);
      if (!parsed) throw new Error("Invalid change sequence");
      if (parsed.changes.length) {
        const snapshot = await docAnhChupChat(contextId, personId, parsed.changes);
        if (!active(version)) return false;
        accept(snapshot);
      }
      // A hydration watermark can include unrelated future changes. Only the
      // page we actually applied advances this cursor.
      after = parsed.next_sequence;
      return true;
    };
    const openSocket = () => {
      if (socket || unsupported || Date.now() < reconnectAfter || after === null || typeof WebSocket === "undefined") return;
      const token = tokenPhienHienTai();
      if (!token) return;
      const version = generation;
      const current = new WebSocket(`${BASE_URL.replace(/^http/, "ws")}/contexts/${encodeURIComponent(contextId)}/changes/stream?after=${after}`);
      socket = current;
      current.onopen = () => {
        if (!active(version)) { current.close(); return; }
        current.send(JSON.stringify({ type: "authenticate", token }));
      };
      current.onmessage = async (event) => {
        if (!active(version) || socket !== current) return;
        // HTTP reconciliation and the socket share one ordered apply lane.
        if (busy) { current.close(); return; }
        busy = true;
        try {
          if (await page(JSON.parse(String(event.data)), version)) {
            if (current.readyState === WebSocket.OPEN) current.send(JSON.stringify({ type: "ack", sequence: after }));
            failures = 0;
            setConnection("live");
          }
        } catch { current.close(); }
        finally { if (version === generation) busy = false; }
      };
      current.onerror = () => current.close();
      current.onclose = () => {
        if (socket !== current) return;
        socket = null;
        failures += 1;
        reconnectAfter = Date.now() + Math.min(15000, 500 * 2 ** Math.min(failures, 5)) + Math.random() * 250;
        if (active(version)) setConnection("recovering");
      };
    };
    const reconcile = async () => {
      if (disposed || unsupported || busy || AppState.currentState !== "active") return;
      if (socket?.readyState === WebSocket.OPEN) return;
      const version = generation;
      busy = true;
      try {
        if (after === null) {
          const snapshot = await docAnhChupChat(contextId, personId);
          if (!active(version)) return;
          accept(snapshot);
          after = snapshot.watermark;
        }
        // Drain all available pages, yielding to the event loop at every I/O.
        let next: TrangThayDoi;
        do {
          next = await docThayDoi(contextId, personId, after);
          if (!active(version) || !await page(next, version)) return;
        } while (next.has_more);
        lastReconcile = Date.now();
        openSocket();
      } catch (error) {
        if (!active(version)) return;
        if (error instanceof ApiError && [404, 501].includes(error.status)) {
          unsupported = true;
          setConnection("unsupported");
        } else setConnection("recovering");
      } finally { if (version === generation) busy = false; }
    };
    const start = () => {
      void reconcile();
      if (!timer) timer = setInterval(() => {
        if (Date.now() - lastReconcile >= 1000) void reconcile();
      }, 1000);
    };
    const stop = () => {
      generation += 1;
      busy = false;
      if (timer) clearInterval(timer);
      timer = null;
      const old = socket;
      socket = null;
      old?.close();
    };
    if (AppState.currentState === "active") start();
    const subscription = AppState.addEventListener("change", (state) => state === "active" ? start() : stop());
    return () => { disposed = true; stop(); subscription.remove(); };
  }, [contextId, personId]));
  return { votes, connection };
}
