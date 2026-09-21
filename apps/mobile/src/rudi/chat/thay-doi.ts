/** Durable invalidations are ordered separately from message timestamps. */
import { translatedAsActor, type CuocBinhChonWire } from "../../api";
import type { Tin } from "./tin-song";

export type ThayDoi = { sequence: number; type: "message" | "vote"; entity_id: string; revision: number };
export type TrangThayDoi = { context_id: string; changes: ThayDoi[]; next_sequence: number; watermark: number; has_more: boolean };
export type BinhChonSong = CuocBinhChonWire & { revision: number; deleted?: boolean };
export type AnhChupChat = { context_id: string; watermark: number; messages: (Tin & { revision: number })[]; votes: BinhChonSong[] };

export function docThayDoi(contextId: string, personId: string, after: number) {
  return translatedAsActor<TrangThayDoi>({}, `/contexts/${contextId}/changes?after=${after}&limit=100`, {
    method: "GET", actorId: personId, contexts: contextId, timeoutMs: 10000,
  });
}

export function docAnhChupChat(contextId: string, personId: string, changes?: readonly ThayDoi[]) {
  const message_ids = [...new Set(changes?.filter((c) => c.type === "message").map((c) => c.entity_id))];
  const vote_ids = [...new Set(changes?.filter((c) => c.type === "vote").map((c) => c.entity_id))];
  return translatedAsActor<AnhChupChat>({}, `/contexts/${contextId}/changes/snapshot`, {
    method: "POST", actorId: personId, contexts: contextId, timeoutMs: 10000,
    body: changes === undefined ? {} : { message_ids, vote_ids },
  });
}

/** Ignore snapshots that completed out of order, including an older own vote. */
export function gopBinhChon(current: Record<string, BinhChonSong>, incoming: readonly BinhChonSong[]) {
  const next = { ...current };
  for (const vote of incoming) {
    if (!next[vote.id] || next[vote.id].revision <= vote.revision) next[vote.id] = vote;
  }
  return next;
}

export function docTrangThayDoi(raw: unknown, contextId: string, after: number): TrangThayDoi | null {
  if (!raw || typeof raw !== "object") return null;
  const p = raw as TrangThayDoi;
  if (p.context_id !== contextId || !Array.isArray(p.changes) || p.changes.length > 100 ||
      !Number.isSafeInteger(p.next_sequence) || p.next_sequence < after ||
      !Number.isSafeInteger(p.watermark) || p.watermark < p.next_sequence || typeof p.has_more !== "boolean") return null;
  let previous = after;
  for (const change of p.changes) {
    if (!Number.isSafeInteger(change.sequence) || change.sequence !== previous + 1 ||
        !Number.isSafeInteger(change.revision) || change.revision < 1 ||
        !["message", "vote"].includes(change.type) || typeof change.entity_id !== "string") return null;
    previous = change.sequence;
  }
  return previous === p.next_sequence ? p : null;
}
