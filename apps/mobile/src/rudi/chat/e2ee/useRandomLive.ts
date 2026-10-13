import { useFocusEffect } from "expo-router";
import { useCallback, useEffect, useState } from "react";
import { AppState } from "react-native";

import { randomClockSample, randomLiveView, randomServerNow, type RandomClockAnchor, type RandomLiveRound, type RandomLiveView } from "./random-live";

export type RandomRoomClock = { anchor: RandomClockAnchor | null; status: "syncing" | "ready" | "unavailable" };
const UNAVAILABLE: RandomRoomClock = { anchor: null, status: "unavailable" };

function monotonicNow(): number | null {
  const now = globalThis.performance?.now?.();
  return typeof now === "number" && Number.isFinite(now) && now >= 0 ? now : null;
}

/** One bounded sampler per focused room, never one request per card or animation frame. */
export function useRandomRoomClock(sample: () => Promise<{ server_now: string }>, enabled: boolean): RandomRoomClock {
  const [stored, setStored] = useState<{ reader: typeof sample | null; clock: RandomRoomClock }>({ reader: null, clock: UNAVAILABLE });
  useFocusEffect(useCallback(() => {
    let disposed = false;
    let generation = 0;
    let refresh: ReturnType<typeof setTimeout> | null = null;
    const setClock = (clock: RandomRoomClock) => setStored({ reader: sample, clock });
    const stop = () => {
      generation += 1;
      if (refresh !== null) clearTimeout(refresh);
      refresh = null;
      if (!disposed) setClock({ anchor: null, status: "unavailable" });
    };
    const synchronize = async () => {
      stop();
      if (!enabled || AppState.currentState !== "active" || monotonicNow() === null) return;
      const version = generation;
      setClock({ anchor: null, status: "syncing" });
      let best: RandomClockAnchor | null = null;
      for (let attempt = 0; attempt < 3; attempt++) {
        if (disposed || version !== generation) return;
        try {
          const before = monotonicNow();
          if (before === null) break;
          const reply = await sample();
          const after = monotonicNow();
          const candidate = after === null ? null : randomClockSample(reply.server_now, before, after);
          if (candidate && (best === null || candidate.uncertaintyMs < best.uncertaintyMs)) best = candidate;
        } catch {
          // The other finite samples may still establish a usable bound.
        }
      }
      if (disposed || version !== generation) return;
      const now = monotonicNow();
      if (best === null || now === null || randomServerNow(best, now) === null) {
        setClock({ anchor: null, status: "unavailable" });
      } else {
        setClock({ anchor: best, status: "ready" });
      }
      // A failed sampler also gets one bounded refresh while this room is focused.
      refresh = setTimeout(() => void synchronize(), 30_000);
    };
    const subscription = AppState.addEventListener("change", (state) => {
      if (state === "active") void synchronize();
      else stop();
    });
    void synchronize();
    return () => { stop(); disposed = true; subscription.remove(); };
  }, [enabled, sample]));
  return enabled && stored.reader === sample && AppState.currentState === "active" ? stored.clock : UNAVAILABLE;
}

/** Local ticks only derive the current shared phase; no network calls or result selection. */
export function useRandomLiveTimeline(round: RandomLiveRound | null, acceptedAt: string | null, clock: RandomRoomClock): RandomLiveView | null {
  const derive = useCallback((): RandomLiveView | null => {
    if (round === null) return null;
    if (clock.status === "syncing") return { phase: "syncing", progress: 0, remainingMs: 0, reveal: false,
      reason: "Đang đồng bộ giờ của lượt…" };
    const now = monotonicNow();
    return randomLiveView(round, acceptedAt, clock.anchor, now ?? Number.NaN);
  }, [round, acceptedAt, clock]);
  const [, tick] = useState(0);
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | null = null;
    let disposed = false;
    const update = () => {
      if (disposed) return;
      const next = derive();
      tick((value) => value + 1);
      if (next?.phase === "countdown" || next?.phase === "racing") timer = setTimeout(update, 100);
    };
    update();
    return () => { disposed = true; if (timer !== null) clearTimeout(timer); };
  }, [derive]);
  // Ticks request a render; every render derives from the current round and anchor.
  return derive();
}
