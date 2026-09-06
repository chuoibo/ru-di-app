/**
 * Group settings on the wire (ADR-0021 §2.4): `PATCH /contexts/{id}`.
 *
 * Any active member may rename the group or choose its chat theme; the body
 * carries only the keys the person changed (`thanDoiNhom`), so a screen that
 * edits the name never resets the theme by accident. Roster actions (roles,
 * leaving) already have a wire in `src/screens/quan-tri/quan-tri.ts` and are
 * re-exported from here so the settings sheet imports one module.
 */
import { type Attempt, translatedAsActor } from "../../api";
import type { ThemeChat } from "../mau-chat";

export { datVaiTro, roiNhom } from "../../screens/quan-tri/quan-tri";

export type NhomChiTietWire = {
  id: string;
  display_name: string;
  created_by_id: string;
  created_at: string;
  theme: ThemeChat;
  /** Arrives with L6; absent on older servers. */
  ai_auto_suggest?: boolean;
};

export type ThayDoiNhom = {
  display_name?: string;
  theme?: ThemeChat;
  ai_auto_suggest?: boolean;
};

export const LOI_NHOM: Record<string, string> = {
  permission_denied: "Chỉ thành viên đang ở trong nhóm mới đổi được cài đặt của nhóm.",
  context_not_found: "Nhóm này không còn.",
  theme_unknown: "Bộ màu này bản app chưa có.",
  not_a_group: "Đây là cuộc trò chuyện riêng, không có cài đặt nhóm.",
};

/** Only the keys the person actually changed, with the name trimmed. */
export function thanDoiNhom(thay: ThayDoiNhom): Record<string, string | boolean> {
  const body: Record<string, string | boolean> = {};
  if (thay.display_name !== undefined) body.display_name = thay.display_name.trim();
  if (thay.theme !== undefined) body.theme = thay.theme;
  if (thay.ai_auto_suggest !== undefined) body.ai_auto_suggest = thay.ai_auto_suggest;
  return body;
}

/** True when there is something to send; an empty patch is refused server-side too. */
export function coGiDeDoi(thay: ThayDoiNhom): boolean {
  const body = thanDoiNhom(thay);
  if (typeof body.display_name === "string" && body.display_name === "") return false;
  return Object.keys(body).length > 0;
}

export async function doiNhom(
  contextId: string,
  personId: string,
  thay: ThayDoiNhom,
  attempt: Attempt,
): Promise<NhomChiTietWire> {
  return translatedAsActor<NhomChiTietWire>(LOI_NHOM, `/contexts/${contextId}`, {
    method: "PATCH",
    body: thanDoiNhom(thay),
    actorId: personId,
    roles: "group_admin,member",
    contexts: contextId,
    attempt,
  });
}
