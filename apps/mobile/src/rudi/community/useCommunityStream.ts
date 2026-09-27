import { useEffect, useRef, useState } from "react";
import { AppState } from "react-native";
import { BASE_URL } from "../../api";
import { tokenPhienHienTai } from "../../danh-tinh";

export type CommunityEvent = { kind: "sync" | "feed.changed" | "post.changed"; cursor: number; post_id?: string };
/** Reconnects with jitter and suspends in the background. Tokens never enter URLs. */
export function useCommunityStream(person: string | undefined, posts: readonly string[], onEvent: (event: CommunityEvent) => void) {
  const callback = useRef(onEvent); callback.current = onEvent;
  const [connected, setConnected] = useState(false);
  const key = posts.slice(0, 40).join(",");
  useEffect(() => {
    if (!person) return;
    let stopped = false; let socket: WebSocket | null = null; let timer: ReturnType<typeof setTimeout> | undefined; let attempt = 0; let cursor = 0;
    const connect = () => {
      const token = tokenPhienHienTai(); if (stopped || !token || AppState.currentState === "background") return;
      const connection = new WebSocket(BASE_URL.replace(/^http/, "ws") + "/v2/community/stream");
      socket = connection;
      connection.onopen = () => { if (socket !== connection || stopped) return; attempt = 0; connection.send(JSON.stringify({ token, posts: key ? key.split(",") : [], after: cursor })); };
      connection.onmessage = (message) => {
        if (socket !== connection || stopped) return;
        try { const event = JSON.parse(String(message.data)) as CommunityEvent;
          if (!Number.isSafeInteger(event.cursor) || !["sync", "feed.changed", "post.changed"].includes(event.kind)) return;
          if (event.kind === "sync") { setConnected(true); cursor = event.cursor; callback.current(event); }
          else if (event.cursor >= cursor) { cursor = event.cursor; callback.current(event); }
        } catch { /* Ignore malformed frames; the next snapshot repairs state. */ }
      };
      connection.onclose = () => { if (socket !== connection || stopped) return; setConnected(false); if (AppState.currentState !== "background") timer = setTimeout(connect, Math.min(30000, 1000 * 2 ** Math.min(attempt++, 5)) + Math.random() * 500); };
      connection.onerror = () => connection.close();
    };
    connect();
    const app = AppState.addEventListener("change", (state) => { clearTimeout(timer); if (state === "active") { socket?.close(); connect(); } else socket?.close(); });
    return () => { stopped = true; clearTimeout(timer); app.remove(); socket?.close(); };
  }, [person, key]);
  return connected;
}
