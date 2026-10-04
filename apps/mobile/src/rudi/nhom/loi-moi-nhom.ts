/**
 * A group invitation, seen by the person it invites (QA UI-080): who invited
 * them, how many are already in, and a way to say no.
 *
 * Wire: `GET` / `DELETE /contexts/{id}/invitation`, Go-only routes (ADR-0031):
 * the invited person cannot read the roster, and declining closes only their
 * own invited row (invited -> left). A later invitation starts again.
 */
import { type Attempt, translatedAsActor } from "../../api";

export type LoiMoiNhom = {
  context_id: string;
  display_name: string;
  /** Null when the inviter's account is gone. */
  invited_by: { id: string; display_name: string } | null;
  member_count: number;
  invited_at: string;
};

export const LOI_LOI_MOI: Record<string, string> = {
  invitation_not_found: "Lời mời này không còn: có thể bạn đã trả lời trên máy khác, hoặc nhóm đã rút lời mời.",
  invalid_context_id: "Lời mời này không đọc được.",
  groups_unavailable: "Chưa đọc được lời mời. Thử lại sau một chút.",
};

export async function docLoiMoiNhom(contextId: string, actorId: string): Promise<LoiMoiNhom> {
  return translatedAsActor<LoiMoiNhom>(LOI_LOI_MOI, `/contexts/${contextId}/invitation`, {
    method: "GET",
    actorId,
  });
}

export async function tuChoiLoiMoiNhom(contextId: string, actorId: string, attempt: Attempt): Promise<void> {
  await translatedAsActor<unknown>(LOI_LOI_MOI, `/contexts/${contextId}/invitation`, {
    method: "DELETE",
    actorId,
    attempt,
  });
}

/**
 * The invitation row's second line: who invited, then how many are in.
 * «Chat Test 01 mời bạn · 2 người trong nhóm»; without a known inviter it
 * still says it is an invitation, never a bare count.
 */
export function cauLoiMoi(loiMoi: Pick<LoiMoiNhom, "invited_by" | "member_count"> | null, soNguoiDuPhong: number): string {
  const so = loiMoi?.member_count ?? soNguoiDuPhong;
  const ai = loiMoi?.invited_by?.display_name?.trim();
  return `${ai ? `${ai} mời bạn` : "Bạn được mời"} · ${so} người trong nhóm`;
}

/** Invitations first, each list keeping its own order: an answer is waited on. */
export function loiMoiTruoc<T extends { my_state: string }>(ds: readonly T[]): T[] {
  return [...ds.filter((n) => n.my_state === "invited"), ...ds.filter((n) => n.my_state !== "invited")];
}
