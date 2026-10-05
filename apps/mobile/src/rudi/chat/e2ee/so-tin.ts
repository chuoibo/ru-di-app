/**
 * The room's messages as the decrypted operations make them (ADR-0057 §5).
 * Pure: the same operations in the same order make the same list. Only an
 * author edits or deletes their own message; a reply keeps its target even if
 * that message is later deleted (it reads as deleted).
 */
import type { Operation, Received } from "./kieu";

export type TinV2 = {
  id: string;
  authorId: string;
  sequence: number;
  body: string | null;
  replyTo: string | null;
  edited: boolean;
  deleted: boolean;
  reactions: Record<string, string[]>;
  media: Extract<Operation, { type: "image" | "voice" }> | null;
  sticker: { packId: string; stickerId: string } | null;
};

export type SoTin = { order: string[]; byId: Record<string, TinV2> };

export const SO_TRONG: SoTin = { order: [], byId: {} };

function them(so: SoTin, tin: TinV2): SoTin {
  if (so.byId[tin.id] !== undefined) return so;
  return { order: [...so.order, tin.id], byId: { ...so.byId, [tin.id]: tin } };
}

function sua(so: SoTin, id: string, f: (t: TinV2) => TinV2 | null): SoTin {
  const cu = so.byId[id];
  if (cu === undefined) return so;
  const moi = f(cu);
  return moi === null ? so : { ...so, byId: { ...so.byId, [id]: moi } };
}

/** Applies one received operation, decrypted at `sequence`. */
export function apDung(so: SoTin, nhan: Extract<Received, { kind: "application" }>, sequence: number): SoTin {
  const op = nhan.operation;
  const tac = nhan.actor_id;
  const moi = (patch: Partial<TinV2>): TinV2 => ({
    id: nhan.logical_send_id, authorId: tac, sequence, body: null, replyTo: null, edited: false, deleted: false,
    reactions: {}, media: null, sticker: null, ...patch,
  });
  switch (op.type) {
    case "text":
      return them(so, moi({ body: op.body }));
    case "reply":
      return them(so, moi({ body: op.body, replyTo: op.reply_to }));
    case "image":
      return them(so, moi({ body: op.caption, media: op }));
    case "voice":
      return them(so, moi({ media: op }));
    case "sticker":
      return them(so, moi({ sticker: { packId: op.pack_id, stickerId: op.sticker_id } }));
    case "edit":
      return sua(so, op.message_id, (t) => (t.authorId !== tac || t.deleted || t.media?.type === "voice" ? null : { ...t, body: op.body, edited: true }));
    case "delete":
      return sua(so, op.message_id, (t) => (t.authorId !== tac ? null : { ...t, body: null, media: null, sticker: null, deleted: true, reactions: {} }));
    case "reaction":
      return sua(so, op.message_id, (t) => {
        if (t.deleted) return null;
        const ai = t.reactions[op.emoji] ?? [];
        const lai = ai.includes(tac) ? ai.filter((x) => x !== tac) : [...ai, tac];
        const reactions = { ...t.reactions, [op.emoji]: lai };
        if (lai.length === 0) delete reactions[op.emoji];
        return { ...t, reactions };
      });
    case "vote":
      return so;
  }
}

export function danhSach(so: SoTin): TinV2[] {
  return so.order.map((id) => so.byId[id]);
}
