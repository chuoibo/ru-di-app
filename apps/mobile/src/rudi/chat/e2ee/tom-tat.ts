/**
 * End-to-end rooms in the conversation list (ADR-0057 §8.4). The list's own
 * route reads the legacy `messages` table, which a v2 room no longer writes:
 * without this a v2 room's row froze at its last legacy message, kept its
 * legacy unread count for ever, and sank in the order. The lane's summary
 * (last message time and sender, unread count) replaces those fields; the
 * words come only from this device's own sealed record, or the row says the
 * message is encrypted.
 */
import type { NhomTomTat } from "../../../phien";

export type TomTatV2 = {
  conversation_id: string;
  last_sequence: number;
  last_at: string | null;
  last_actor_id: string | null;
  unread: number;
  read_sequence: number;
};

/** What a v2 row says when this device has no words for its last message. */
export const CHU_MA_HOA = "Tin nhắn mã hoá đầu cuối";

/**
 * The list with every v2 room's row taken from the lane: its unread count
 * (the legacy one can never be cleared there), its last message (this
 * device's words for it when it has them), and the rows in order of their
 * last message, newest first -- the order the server keeps for legacy rows.
 */
export function gopTomTatV2(nhom: NhomTomTat[], rooms: TomTatV2[], xemTruoc: Record<string, string>): NhomTomTat[] {
  const theo = new Map(rooms.map((r) => [r.conversation_id, r]));
  const gop = nhom.map((n) => {
    const r = theo.get(n.id);
    if (r === undefined) return n;
    const cu = n.last_message ?? null;
    const moiHon = r.last_at !== null && (cu === null || r.last_at > cu.created_at);
    return {
      ...n,
      unread_count: r.unread,
      last_message: moiHon
        ? {
            id: `v2:${r.last_sequence}`,
            kind: "text" as const,
            preview: xemTruoc[n.id] ?? CHU_MA_HOA,
            author_id: r.last_actor_id,
            author_display_name: n.counterpart !== null && n.counterpart !== undefined && n.counterpart.id === r.last_actor_id ? n.counterpart.display_name : null,
            created_at: r.last_at as string,
          }
        : cu,
    };
  });
  const luc = (n: NhomTomTat) => n.last_message?.created_at ?? "";
  return gop
    .map((n, i) => ({ n, i }))
    .sort((a, b) => {
      const la = luc(a.n);
      const lb = luc(b.n);
      if (la === lb) return a.i - b.i;
      if (la === "") return 1;
      if (lb === "") return -1;
      return lb.localeCompare(la);
    })
    .map((x) => x.n);
}
