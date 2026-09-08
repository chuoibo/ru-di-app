/**
 * What is on its way, and what to do when it does not arrive.
 *
 * Sending a sticker used to be one awaited call with nothing on screen behind
 * it: the tray closed, no bubble appeared, and a failure printed a general
 * notice that did not say WHICH picture had been lost, let alone offer to send
 * it again (review delta 08/09, F32). This module is the missing half, kept
 * pure so the state machine can be proved without a renderer.
 *
 * The load-bearing decision is the key. `api.ts` says it in one line -- «mint
 * an attempt on the press, never inside a retry» -- and the chat hook did the
 * opposite, calling `newAttempt()` inside the action, so a retry after a lost
 * response wrote the message twice. Here the `Attempt` is minted once and then
 * CARRIED BY THE ROW: retrying replays the same key, which the server answers
 * from its store (`app/api/idempotency.py`, `Replay`), so a retry can only
 * ever produce the row that already exists. Sending the same sticker again on
 * purpose is a new row with a new key, and the two are different acts.
 */
import type { Attempt } from "../../api";
import type { TrichDan } from "./tin-song";

/** What a row on its way can be. There is no «sent»: a sent row leaves the queue. */
export type TrangThaiGui = "dang-gui" | "that-bai";

/** One logical send, held from the press until the server's row arrives. */
export interface TinChoGui {
  /** The attempt IS the identity: one logical send, one key, replayed on retry. */
  attempt: Attempt;
  kind: "sticker" | "text" | "image";
  /** The sticker id, the words, or the uploaded address. */
  than: string;
  /** The photo's caption. Part of the request body, so part of its identity. */
  phuDe: string | null;
  /** Kept so a retry answers the same message, not whatever is quoted now. */
  traLoi: TrichDan | null;
  trangThai: TrangThaiGui;
  /** The server's own sentence, already in Vietnamese; null while in flight. */
  loi: string | null;
  /** Whether pressing again could help. A vocabulary error never can. */
  thuLaiDuoc: boolean;
  luc: string;
}

/**
 * Codes where pressing again cannot help, and the one code that says so
 * outright.
 *
 * `sticker_unknown` means this build does not have the picture; `permission_denied`
 * and the reply-target codes mean the thing being answered is gone. Offering
 * «Thử lại» there is offering a button that will fail the same way.
 * `idempotency_request_in_flight` is the server saying the first attempt is
 * still running: its own sentence asks the person NOT to press again.
 */
const KHONG_THU_LAI = new Set([
  "sticker_unknown",
  "permission_denied",
  "reply_target_deleted",
  "reply_target_not_quotable",
  "message_not_found",
  "idempotency_request_in_flight",
  "idempotency_key_reuse",
  "invalid_idempotency_key",
]);

export function thuLaiDuoc(ma: string | null): boolean {
  return ma === null || !KHONG_THU_LAI.has(ma);
}

/** Put a fresh send at the head of the queue. Newest first, like the list it draws into. */
export function themVaoHang(hang: readonly TinChoGui[], tin: TinChoGui): TinChoGui[] {
  return [tin, ...hang.filter((t) => t.attempt.key !== tin.attempt.key)];
}

/** The server refused, or nothing answered. The row stays, holding its key. */
export function danhDauLoi(
  hang: readonly TinChoGui[],
  khoa: string,
  loi: string,
  ma: string | null,
): TinChoGui[] {
  return hang.map((t) =>
    t.attempt.key === khoa ? { ...t, trangThai: "that-bai" as const, loi, thuLaiDuoc: thuLaiDuoc(ma) } : t,
  );
}

/** Pressing «Thử lại»: the same row, the same key, in flight again. */
export function danhDauThuLai(hang: readonly TinChoGui[], khoa: string): TinChoGui[] {
  return hang.map((t) => (t.attempt.key === khoa ? { ...t, trangThai: "dang-gui" as const, loi: null } : t));
}

/** It arrived. The server's row is in the list now, so this one goes. */
export function boKhoiHang(hang: readonly TinChoGui[], khoa: string): TinChoGui[] {
  return hang.filter((t) => t.attempt.key !== khoa);
}

/** The row a retry needs, or null when that send is no longer queued. */
export function timTrongHang(hang: readonly TinChoGui[], khoa: string): TinChoGui | null {
  return hang.find((t) => t.attempt.key === khoa) ?? null;
}

/**
 * The key a failed send left behind, so pressing send again reuses it instead
 * of minting a new one.
 *
 * Only when the bytes are identical, and «identical» means EVERY field that
 * reaches the request body: the words, the quoted message AND the caption.
 * The server fingerprints method, path and canonical body, so the same key
 * with anything else changed is `422 idempotency_key_reuse` -- a refusal aimed
 * at somebody who did nothing wrong, and one this app classifies as permanent,
 * so it would tell them their message failed for good while it is already in
 * the thread. Anything different is a different send and gets its own key.
 */
export function khoaDungLai(
  cho: TinChoGui | null,
  than: string,
  traLoiId: string | null,
  phuDe: string | null = null,
): Attempt | null {
  if (cho === null || cho.trangThai !== "that-bai") return null;
  if (cho.than !== than) return null;
  if ((cho.traLoi?.id ?? null) !== traLoiId) return null;
  if ((cho.phuDe ?? null) !== phuDe) return null;
  return cho.attempt;
}
