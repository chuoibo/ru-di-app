/**
 * Nhắn riêng 1:1 (ADR-0021 §2.5): one door, `POST /people/{id}/dm`.
 *
 * A private conversation is a `context` of kind `pair`, so the chat screen,
 * the message wire, reactions, read marks and the theme are the group ones,
 * unchanged. This module is the small part that differs: the door that opens
 * (or finds) the pair, how a pair is NAMED on screen -- after the other person,
 * never after its empty stored name and never after an id -- and how the
 * session's own group list takes the new row so the chat can open at once.
 *
 * The server answers every refusal with the same 404 and the same sentence
 * (not friends, blocked, deleted, nobody). The table below repeats that
 * sentence for the codes the door can answer, so a more specific message here
 * cannot turn the screen into the oracle the server refuses to be.
 */
import { type Attempt, translatedAsActor } from "../../api";
import type { NhomTomTat } from "../../phien";

export type NguoiKia = { id: string; display_name: string };

/** The part of a conversation row that naming reads; a group has no counterpart. */
export type CuocTroChuyen = {
  display_name: string;
  kind?: string;
  counterpart?: NguoiKia | null;
};

export const LOI_NHAN_RIENG: Record<string, string> = {
  person_not_found: "Chưa thể nhắn riêng với người này.",
  permission_denied: "Chưa thể nhắn riêng với người này.",
  self_direct_message: "Không thể nhắn riêng với chính mình.",
};

/** What a pair is called when nobody's name is there to call it by. A word, never an id. */
export const TEN_NGUOI_KHUYET = "Thành viên";

/**
 * The pair with this person: created (201) or found (200). Same row either
 * way, shaped like `GET /people/me/contexts`, so the caller can open the chat
 * without a second read.
 */
export async function moNhanRieng(personId: string, actorId: string, attempt: Attempt): Promise<NhomTomTat> {
  return translatedAsActor<NhomTomTat>(LOI_NHAN_RIENG, `/people/${personId}/dm`, {
    method: "POST",
    actorId,
    attempt,
  });
}

export function laPair(n: { kind?: string } | undefined): boolean {
  return n?.kind === "pair";
}

/**
 * What the conversation is called at the top of the screen and in the list.
 *
 * A group: its own name. A pair: the other person's name. The server already
 * fills `display_name` with that name, but a pair whose counterpart has no
 * name must still read as a person rather than as an empty title, and a group
 * the session does not know yet reads as «Nhóm» rather than as nothing.
 */
export function tenCuocTroChuyen(n: CuocTroChuyen | undefined): string {
  if (n === undefined) return "Nhóm";
  if (!laPair(n)) return n.display_name;
  const ten = (n.counterpart?.display_name ?? n.display_name).trim();
  if (ten === "") return TEN_NGUOI_KHUYET;
  return ten;
}

/**
 * The session's group list with this pair in it, newest first. The chat screen
 * names a conversation from `phien.contexts`, so a pair opened a moment ago
 * must be there before that screen mounts; the next `GET /people/me/contexts`
 * replaces the whole list anyway.
 */
export function ghepVaoDanhSach(contexts: NhomTomTat[] | undefined, moi: NhomTomTat): NhomTomTat[] {
  const conLai = (contexts ?? []).filter((n) => n.id !== moi.id);
  return [moi, ...conLai];
}
