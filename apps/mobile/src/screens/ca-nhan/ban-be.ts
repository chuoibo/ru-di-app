/** Username lookup, friendship requests and their public refusal messages. */
import { BASE_URL, translatedAsActor, type Attempt } from "../../api";

/** A discoverable account returned by username lookup. */
export type NguoiTimDuoc = {
  person_id: string;
  display_name: string;
  username?: string;
};

/** One friend edge, as whichever party is reading it sees it. */
export type LoiMoi = {
  id: string;
  requester_id: string;
  addressee_id: string;
  /** Whoever the reader is not. Resolved by the server, not by this app. */
  other_person_id: string;
  other_display_name: string;
  state: "pending" | "accepted" | "declined" | "blocked";
  created_at: string;
  decided_at: string | null;
};

export type Ban = {
  person_id: string;
  display_name: string;
  friends_since: string;
};

export type TraLoi = "accept" | "decline" | "block";

/* ------------------------------------------------------ what refusals mean */

/** Username discovery never reveals hidden, blocked or deleted accounts. */
export const LOI_TIM: Record<string, string> = {
  person_not_found: "Chưa tìm thấy tài khoản này. Kiểm tra username hoặc nhờ bạn ấy bật cho phép tìm kiếm.",
  username_invalid: "Tên tài khoản gồm 3–32 chữ, số, dấu chấm hoặc gạch dưới.",
  auth_rate_limited: "Bạn vừa tìm hơi nhiều lần. Thử lại sau một phút. App vẫn hoạt động.",
  authentication_required: "Hãy đăng nhập lại để tìm bạn.",
  managed_account_required: "Tìm bạn theo username cần đăng nhập bằng tài khoản Rủ Đi của bạn.",
  auth_temporarily_unavailable: "Tìm bạn đang gián đoạn. Hãy thử lại sau.",
};

/**
 * Refusals of `POST /friends/requests`.
 *
 * `request_not_open` is the interesting one, and the sentence is short on
 * purpose. The server answers with that one code for three different
 * situations -- already friends, a request already pending, and blocked -- and
 * it does that deliberately: `service.py` gives the race arm the same status
 * and the same code as the read arm so that "a blocked person cannot tell a
 * block from a duplicate by timing". Writing "hai bạn đã là bạn rồi, hoặc lời
 * mời trước còn đang chờ" here would undo that on the client, because it names
 * two of the three states and so tells whoever is blocked that they are not.
 *
 * So the sentence says what is true of all three -- it did not go -- and sends
 * the reader to the two lists further down the screen, which are their own and
 * which they are entitled to read.
 */
export const LOI_GUI: Record<string, string> = {
  person_not_found:
    "Người này không còn trong Rủ Đi nữa. Tìm lại bằng tên tài khoản.",
  request_not_open:
    "Lời mời này chưa gửi được. Kéo xuống xem \"Lời mời đã gửi\" và \"Bạn bè\" bên dưới để biết hai bạn đang ở đâu.",
  self_edge: "Đây là tài khoản của chính bạn. Không tự kết bạn với mình được.",
  permission_denied: "Tài khoản đang dùng chưa được phép gửi lời mời kết bạn.",
};

/** Refusals of `POST /friends/requests/{id}/respond`. */
export const LOI_TRA_LOI: Record<string, string> = {
  friend_request_not_found:
    "Lời mời này không còn nữa. Có thể bạn đã trả lời rồi ở lần mở trước. Tải lại danh sách để xem trạng thái mới nhất.",
  // 403. The server sends its domain code as the detail here rather than a
  // sentence (`only_addressee_may_answer`), so without this line a machine
  // string would reach the screen.
  permission_denied: "Chỉ người được mời mới trả lời được lời mời này.",
  not_pending:
    "Lời mời này đã được trả lời rồi. Tải lại danh sách để xem trạng thái mới nhất.",
  already_blocked: "Lời mời này đã bị chặn từ trước.",
  not_a_party: "Lời mời này không phải của bạn.",
};

/** Refusals of the two read routes. Both are self-only at the server. */
export const LOI_DOC: Record<string, string> = {
  permission_denied: "Chỉ chính chủ mới xem được danh sách bạn bè của mình.",
};

/* ------------------------------------------------------------- the calls */

/** POST the username in a body, keeping it out of access-log URLs. */
export async function timBanTheoUsername(
  username: string,
  actorId: string,
): Promise<NguoiTimDuoc> {
  return translatedAsActor<NguoiTimDuoc>(LOI_TIM, "/friends/lookup", {
    method: "POST",
    body: { username },
    actorId,
  });
}

/**
 * Ask somebody to be friends.
 *
 * 201 means asked. It does not mean friends, and the screen that calls this
 * has to say so -- see `KetBan.tsx`, where the returned `state` is rendered as
 * a wait rather than as a result.
 */
export async function guiLoiMoi(
  addresseeId: string,
  actorId: string,
  attempt: Attempt,
): Promise<LoiMoi> {
  return translatedAsActor<LoiMoi>(LOI_GUI, "/friends/requests", {
    method: "POST",
    body: { addressee_id: addresseeId },
    actorId,
    attempt,
  });
}

/** Answer one. Only the person it was addressed to may accept or decline. */
export async function traLoiLoiMoi(
  requestId: string,
  quyetDinh: TraLoi,
  actorId: string,
  attempt: Attempt,
): Promise<LoiMoi> {
  return translatedAsActor<LoiMoi>(
    LOI_TRA_LOI,
    `/friends/requests/${requestId}/respond`,
    { method: "POST", body: { decision: quyetDinh }, actorId, attempt },
  );
}

/**
 * Pending requests in one direction.
 *
 * The server reads anything other than the literal `outgoing` as `incoming`,
 * so an unknown value narrows the result rather than widening it. This sends
 * one of the two words and nothing else.
 */
export async function docLoiMoi(
  personId: string,
  actorId: string,
  huong: "incoming" | "outgoing",
): Promise<LoiMoi[]> {
  const wire = await translatedAsActor<{ requests: LoiMoi[] }>(
    LOI_DOC,
    `/people/${personId}/friend-requests?direction=${huong}`,
    { method: "GET", actorId },
  );
  return wire.requests;
}

/** This person's friends. Self-only, enforced at the server. */
export async function docDanhSachBan(
  personId: string,
  actorId: string,
): Promise<Ban[]> {
  const wire = await translatedAsActor<{ friends: Ban[] }>(
    LOI_DOC,
    `/people/${personId}/friends`,
    { method: "GET", actorId },
  );
  return wire.friends;
}

/* -------------------------------------------------------------- for the eye */

/** The monogram an avatar frame falls back to. No photos of real people. */
export function chuDau(ten: string): string {
  const word = ten.trim().split(/\s+/).pop() ?? "";
  return word === "" ? "?" : word.slice(0, 1).toUpperCase();
}

/** The address this screen talks to, printed on every dead end. */
export const DIA_CHI_API = BASE_URL;
