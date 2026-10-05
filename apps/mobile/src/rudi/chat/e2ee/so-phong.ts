/**
 * A room's durable record on this device (ADR-0057 §4.4): what was decrypted,
 * what this device sent, and how far the lane has been read -- one log, so a
 * message and the cursor past it are written in one step. MLS deletes a key
 * once it is used: a message that was decrypted and not written down is gone
 * for good, so nothing may move the cursor without writing what it passed.
 *
 * The log lives sealed on the device (the native module's room store), never
 * in plain storage, never in a backup. This module is pure: the same records
 * make the same room.
 */
import type { Received } from "./kieu";
import { SO_TRONG, apDung, type SoTin } from "./so-tin";

export type Nhan = Extract<Received, { kind: "application" }>;

export type BanGhi =
  /** Decrypted from another device, at its lane sequence. */
  | { t: "tin"; seq: number; r: Nhan }
  /** An envelope from `actor` that will never open (forged, garbled): skipped, and said. */
  | { t: "khong-mo"; seq: number; actor: string }
  /** This device's own operation, written before it is sent. */
  | { t: "cho"; r: Nhan; luc: number }
  /** The lane holds this device's send `id` at `seq`. */
  | { t: "da-gui"; id: string; seq: number }
  /** The send failed; `thuLai` says whether trying again can help. */
  | { t: "hong"; id: string; loi: string; thuLai: boolean }
  /** Tried again. */
  | { t: "thu"; id: string }
  /** The person gave up on a send that never reached the lane. */
  | { t: "bo"; id: string };

export type SoPhong = { cursor: number; ban: BanGhi[] };

export const PHONG_TRONG: SoPhong = { cursor: 0, ban: [] };

/** One own send not (yet) on the lane. */
export type Cho = { id: string; r: Nhan; luc: number; hong: boolean; loi: string | null; thuLai: boolean };

/** Operations that make a row of their own; the rest change an existing one. */
const TAO_HANG = new Set(["text", "reply", "image", "sticker", "voice"]);

/**
 * The room the records make: messages in lane order (each logical send once,
 * however often a replay wrote it) and this device's sends still on their way.
 */
export function dungPhong(ban: readonly BanGhi[]): { so: SoTin; cho: Cho[]; dangDi: Cho[]; khongMo: number } {
  const seq = new Map<string, number>();
  let khongMo = 0;
  const cua = new Map<string, Cho>();
  const bo = new Set<string>();
  const tin = new Map<string, { seq: number; r: Nhan }>();
  for (const b of ban) {
    switch (b.t) {
      case "tin":
        if (!tin.has(b.r.logical_send_id)) tin.set(b.r.logical_send_id, { seq: b.seq, r: b.r });
        break;
      case "cho":
        if (!cua.has(b.r.logical_send_id)) cua.set(b.r.logical_send_id, { id: b.r.logical_send_id, r: b.r, luc: b.luc, hong: false, loi: null, thuLai: true });
        break;
      case "da-gui":
        if (!seq.has(b.id)) seq.set(b.id, b.seq);
        break;
      case "hong": {
        const c = cua.get(b.id);
        if (c !== undefined) cua.set(b.id, { ...c, hong: true, loi: b.loi, thuLai: b.thuLai });
        break;
      }
      case "thu": {
        const c = cua.get(b.id);
        if (c !== undefined) cua.set(b.id, { ...c, hong: false, loi: null });
        break;
      }
      case "bo":
        bo.add(b.id);
        break;
      case "khong-mo":
        khongMo += 1;
        break;
    }
  }
  for (const [id, c] of cua) {
    const s = seq.get(id);
    if (s !== undefined && !tin.has(id)) tin.set(id, { seq: s, r: c.r });
  }
  let so = SO_TRONG;
  for (const t of [...tin.values()].sort((a, b) => a.seq - b.seq)) so = apDung(so, t.r, t.seq);
  // Every own operation not on the lane; the rows among them are drawn.
  const dangDi = [...cua.values()].filter((c) => !seq.has(c.id) && !bo.has(c.id));
  return { so, cho: dangDi.filter((c) => TAO_HANG.has(c.r.operation.type)), dangDi, khongMo };
}

/** Own sends to try again on their own: on their way, not failed, not given up. */
export function canGuiLai(ban: readonly BanGhi[]): Cho[] {
  return dungPhong(ban).dangDi.filter((c) => !c.hong);
}
