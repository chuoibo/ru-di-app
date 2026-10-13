/** Versioned shared scheduling; the existing v1 result codec stays unchanged. */
import { createRandomRound, decodeRandomRound, encodeRandomRound, type RandomRound } from "./random-v2";

export type RandomLiveRound = Omit<RandomRound, "v"> & { v: 2; timing: "shared-v1" };
export type RandomPlayableRound = RandomRound | RandomLiveRound;
export type RandomLiveView = {
  phase: "syncing" | "countdown" | "racing" | "finished" | "unavailable";
  progress: number;
  remainingMs: number;
  reveal: boolean;
  reason?: string;
};
export type RandomClockAnchor = { serverMs: number; monotonicMs: number; uncertaintyMs: number };

export const RANDOM_COUNTDOWN_MS = 6000;
export const RANDOM_CLOCK_MAX_AGE_MS = 30_000;
export const RANDOM_CLOCK_MAX_UNCERTAINTY_MS = 250;
const MARKER = "\n/rudi-random-live-v2 ";
const KEYS = ["v", "id", "request", "mode", "options", "seed", "winner", "timing"];

export function randomLiveDuration(round: RandomLiveRound): number {
  return round.mode === "wheel" ? 6000 : 12_000;
}

export function randomLivePreview(round: RandomLiveRound): string {
  return `${round.mode === "wheel" ? "Vòng quay" : "Đua vịt"}: lượt đã khóa`;
}

function serialized(round: RandomLiveRound): string {
  return JSON.stringify({ v: 2, id: round.id, request: round.request, mode: round.mode, options: round.options,
    seed: round.seed, winner: round.winner, timing: round.timing });
}

function inBounds(body: string): boolean {
  if (body.length > 6000) return false;
  let bytes = 0;
  for (const character of body) {
    const point = character.codePointAt(0)!;
    bytes += point <= 0x7f ? 1 : point <= 0x7ff ? 2 : point <= 0xffff ? 3 : 4;
    if (bytes > 12 * 1024) return false;
  }
  return true;
}

/** Uses the same finite CSPRNG draw and source validation, with a separate wire version. */
export function createRandomLiveRound(round: RandomRound): RandomLiveRound {
  const checked = decodeRandomRound(encodeRandomRound(round));
  if (!checked) throw new Error("Invalid random live round");
  return { ...checked, v: 2, timing: "shared-v1" };
}

/** The fallback never reveals the winner before the shared deadline. */
export function encodeRandomLiveRound(round: RandomLiveRound): string {
  if (round === null || typeof round !== "object" || Object.keys(round).length !== KEYS.length ||
      !KEYS.every((key) => Object.hasOwn(round, key)) || round.v !== 2 || round.timing !== "shared-v1") throw new Error("Invalid random live round");
  const body = randomLivePreview(round) + MARKER + serialized(round);
  if (decodeRandomLiveRound(body) === null) throw new Error("Invalid random live round");
  return body;
}

/** Called only on authenticated MLS text/reply; logical-ID binding remains the reducer's job. */
export function decodeRandomLiveRound(body: string): RandomLiveRound | null {
  if (typeof body !== "string" || !inBounds(body)) return null;
  const marker = body.indexOf(MARKER);
  if (marker < 0) return null;
  try {
    const json = body.slice(marker + MARKER.length);
    const value: unknown = JSON.parse(json);
    if (value === null || typeof value !== "object" || Array.isArray(value)) return null;
    const raw = value as Record<string, unknown>;
    if (Object.keys(raw).length !== KEYS.length || !KEYS.every((key) => Object.hasOwn(raw, key)) ||
        raw.v !== 2 || raw.timing !== "shared-v1") return null;
    const round = createRandomRound({ id: raw.id, request: raw.request, mode: raw.mode, options: raw.options, seed: raw.seed } as Parameters<typeof createRandomRound>[0]);
    if (round.winner !== raw.winner) return null;
    const live: RandomLiveRound = { ...round, v: 2, timing: "shared-v1" };
    if (body.slice(0, marker) !== randomLivePreview(live) || json !== serialized(live)) return null;
    return live;
  } catch {
    return null;
  }
}

/** A DB time sampled somewhere inside the measured round trip, with explicit uncertainty. */
export function randomClockSample(serverNow: string, before: number, after: number): RandomClockAnchor | null {
  const serverMs = parseTimestamp(serverNow);
  if (serverMs === null || !Number.isFinite(before) || !Number.isFinite(after) || before < 0 || after < before) return null;
  const uncertaintyMs = (after - before) / 2 + 1; // Includes sub-millisecond timestamp truncation.
  if (uncertaintyMs > RANDOM_CLOCK_MAX_UNCERTAINTY_MS) return null;
  return { serverMs, monotonicMs: (before + after) / 2, uncertaintyMs };
}

function parseTimestamp(value: string | null): number | null {
  if (typeof value !== "string" || !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/.test(value)) return null;
  const ms = Date.parse(value);
  return Number.isFinite(ms) && ms > 0 && new Date(ms).toISOString().slice(0, 19) === value.slice(0, 19) ? ms : null;
}

export function randomServerNow(clock: RandomClockAnchor | null, monotonic: number): number | null {
  if (clock === null || !Number.isFinite(monotonic) || !Number.isFinite(clock.serverMs) || clock.serverMs <= 0 ||
      !Number.isFinite(clock.monotonicMs) || !Number.isFinite(clock.uncertaintyMs) || clock.uncertaintyMs < 0 ||
      clock.uncertaintyMs > RANDOM_CLOCK_MAX_UNCERTAINTY_MS) return null;
  const elapsed = monotonic - clock.monotonicMs;
  return elapsed < 0 || elapsed > RANDOM_CLOCK_MAX_AGE_MS ? null : clock.serverMs + elapsed;
}

/** No wall-clock or mount time enters the shared progress/reveal decision. */
export function randomLiveView(round: RandomLiveRound, acceptedAt: string | null, clock: RandomClockAnchor | null, monotonic: number): RandomLiveView {
  const at = parseTimestamp(acceptedAt);
  const now = randomServerNow(clock, monotonic);
  if (at === null || now === null || at > now + clock!.uncertaintyMs + 1000) return { phase: "unavailable", progress: 0, remainingMs: 0, reveal: false,
    reason: "Chưa đồng bộ được giờ. Mở lại lượt khi có mạng." };
  const start = at + RANDOM_COUNTDOWN_MS;
  const duration = randomLiveDuration(round);
  if (now < start) return { phase: "countdown", progress: 0, remainingMs: start - now, reveal: false };
  const elapsed = now - start;
  if (elapsed < duration) return { phase: "racing", progress: elapsed / duration, remainingMs: duration - elapsed, reveal: false };
  // The uncertainty interval must have passed the deadline before revealing.
  if (now - clock!.uncertaintyMs < start + duration) return { phase: "racing", progress: 1,
    remainingMs: start + duration + clock!.uncertaintyMs - now, reveal: false };
  return { phase: "finished", progress: 1, remainingMs: 0, reveal: true };
}
