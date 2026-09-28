import { useEffect } from "react";
import { AppState } from "react-native";
import { newAttempt, translatedAsActor } from "../../api";
import { COMMUNITY_ERRORS } from "./api";

/** Only visible, foreground time contributes; disabled consent sends no event. */
export function useViewSignal(person: string | undefined, post: string | null, enabled: boolean) {
  useEffect(() => {
    if (!person || !post || !enabled) return;
    let started = Date.now(); let foreground = AppState.currentState === "active"; let total = 0;
    const send = (kind: "impression" | "view" | "skip", dwell: number) => { void translatedAsActor<void>(COMMUNITY_ERRORS, "/v2/community/interactions", { actorId: person, method: "POST", attempt: newAttempt(), body: { id: newAttempt().key, post_id: post, kind, dwell_ms: Math.min(180000, Math.max(0, dwell)) } }).catch(() => {}); };
    send("impression", 0);
    const subscription = AppState.addEventListener("change", (state) => { if (foreground) total += Date.now() - started; foreground = state === "active"; started = Date.now(); });
    return () => { subscription.remove(); if (foreground) total += Date.now() - started; send(total >= 3000 ? "view" : "skip", total); };
  }, [person, post, enabled]);
}
