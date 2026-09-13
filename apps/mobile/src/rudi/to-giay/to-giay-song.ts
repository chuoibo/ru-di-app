/**
 * «Nếp truyền giấy» against the real server (ADR-0027, nineteen routes).
 *
 * Phase 2 built thirteen screens on a pure model with no network in them. This
 * module is the only thing between those screens and the server, and it exists
 * so the screens did not have to be rewritten: every function here hands back
 * the same `ToGiay` / `SoHaiNguoi` shapes the fixture build already renders.
 *
 * ## Why every refusal gets its own sentence
 *
 * `thongDiepNguoiDoc`'s generic 403 sentence says «bạn không có quyền». For
 * this feature that is wrong twice over. `pair_chat_consent_required` is not a
 * missing permission, it is a consent neither of them has given yet -- the
 * person reading it has every right they need and nothing to fix in settings.
 * And `paper_version_stale` is not a refusal at all from the reader's side; it
 * is news, that the other person wrote something while this screen was open.
 *
 * ## Why commands do not carry content back
 *
 * Every command answers `{id, state, version, outing_id?}` and nothing else, so
 * a retry after a lost reply replays exactly that (ADR-0027 §3). The screens
 * therefore re-read after a command rather than patching state from its answer;
 * `useToGiay` is what does that, once, in one place.
 */
import { type Attempt, translatedAsActor } from "../../api";
import type { NoiDungTo, ToGiay } from "./to-giay";

/** Four rungs of the consent ladder, of which slice 1 uses three. */
export type MucDich = "lap_so" | "bat_doi" | "doc_chat";
export type LoaiRangBuoc = "khong_an_duoc" | "dung";

export interface DongYCuaToi {
  purpose: MucDich;
  granted: boolean;
}

export interface DeNghiCho {
  id: string;
  purpose: MucDich;
  expires_at: string;
  proposed_by_id: string;
  my_granted: boolean;
}

export interface RangBuocSong {
  owner_id: string;
  kind: LoaiRangBuoc;
  content: string;
  version: number;
}

/** The notebook as one of the two sees it: `GET /contexts/{id}/notebook`. */
export interface SoHaiNguoi {
  context_id: string;
  cycle_state: "pending" | "active" | "closed" | null;
  participants: readonly string[];
  my_consents: readonly DongYCuaToi[];
  their_consents_granted: Readonly<Record<string, boolean>>;
  pending_proposals: readonly DeNghiCho[];
  constraints: readonly RangBuocSong[];
  nep_gui_ho: boolean;
  open_paper_id: string | null;
}

/** One row of `GET /contexts/{id}/papers`; the list carries no content. */
export interface ToTomTat {
  id: string;
  state: ToGiay["state"];
  version: number;
  tuan: string;
  ngay: string | null;
  expires_at: string;
}

/** What every command answers. Never content -- see the header. */
export interface LenhToGiay {
  id: string;
  state: ToGiay["state"];
  version: number;
  outing_id: string | null;
}

export interface XemTruocDongSo {
  revision: string;
  so_nhap_bo: number;
  so_to_huy: number;
  so_to_khoa: number;
  so_de_nghi_huy: number;
}

/**
 * Every refusal this feature can answer, in the language the person reading it
 * speaks. A code missing from here falls through to the generic sentence, which
 * is why the two that generic sentence describes wrongly are first.
 */
export const LOI_TO_GIAY: Record<string, string> = {
  // 403, and NOT a permission. Both of them have to say yes before Nếp opens
  // the conversation; «bạn không có quyền» would send somebody looking through
  // settings for a switch that is not theirs alone to flip.
  pair_chat_consent_required: "Cả hai cùng đồng ý cho Nếp đọc tin nhắn thì Nếp mới nói được.",
  // 409, and news rather than a refusal: the other person wrote while this
  // screen was open. The screen re-reads; the sentence says why.
  paper_version_stale: "Người kia vừa gửi bản mới. Mở lại để xem đã.",
  paper_expired: "Tuần này hết rồi. Tuần sau mình rủ lại nhé.",
  paper_frozen: "Hai bạn chốt rồi, không sửa nữa.",
  paper_wrong_state: "Tờ giấy không ở trạng thái làm được việc này.",
  paper_not_withdrawable: "Người kia đã mở tờ này rồi, không rút lại được.",
  paper_self_response: "Đây là tờ bạn gửi, chờ người kia trả lời.",
  paper_needs_recorder: "Cần biết ai ghi là hai bạn đã đi.",
  paper_not_found: "Không có tờ giấy này.",
  notebook_not_found: "Không có sổ này.",
  notebook_revision_stale: "Sổ vừa đổi. Xem lại rồi đóng.",
  cycle_not_active: "Sổ chưa mở. Cả hai cùng đồng ý lập sổ trước đã.",
  cycle_closed: "Sổ này đã đóng.",
  consent_missing: "Cả hai cùng đồng ý lập sổ trước đã.",
  consent_proposal_expired: "Lời đề nghị này đã hết hạn.",
  consent_proposal_not_found: "Không có lời đề nghị này.",
  consent_purpose_unknown: "Không có mục đích này.",
  couple_slot_taken: "Một trong hai người đang là một đôi ở sổ khác.",
  constraint_kind_unknown: "Không có ô này.",
  // Hai mã dưới đây một client đúng đắn không chạm tới được: máy chủ tự dựng
  // ngày cho bản phác, và tên sự kiện do chính nó chọn. Vẫn có câu, vì thứ
  // người ta nhận khi một cái «không thể xảy ra» xảy ra không nên là câu 403
  // chung nói về một quyền họ vẫn có.
  paper_draft_needs_date: "Tờ giấy này thiếu ngày. Thử mở lại màn hình.",
  paper_event_unknown: "Không làm được việc này với tờ giấy.",
  paper_already_agreed: "Bạn đã đồng ý tờ này rồi.",
  paper_outing_exists: "Tờ này đã có buổi đi rồi.",
};

type Goi = { actorId: string; attempt?: Attempt };

function doc<T>(path: string, { actorId }: Goi): Promise<T> {
  return translatedAsActor<T>(LOI_TO_GIAY, path, { actorId, method: "GET" });
}

function ghi<T>(path: string, method: "POST" | "PATCH" | "PUT" | "DELETE", { actorId, attempt }: Goi, body?: unknown): Promise<T> {
  return translatedAsActor<T>(LOI_TO_GIAY, path, { actorId, method, attempt, body });
}

// --- sổ ---------------------------------------------------------------------

export function docSo(contextId: string, goi: Goi): Promise<SoHaiNguoi> {
  return doc<SoHaiNguoi>(`/contexts/${contextId}/notebook`, goi);
}

export function deNghiDongY(contextId: string, purpose: MucDich, goi: Goi): Promise<DeNghiCho> {
  return ghi<DeNghiCho>(`/contexts/${contextId}/notebook/proposals`, "POST", goi, { purpose });
}

export function dongYDeNghi(contextId: string, proposalId: string, goi: Goi): Promise<DeNghiCho> {
  return ghi<DeNghiCho>(`/contexts/${contextId}/notebook/proposals/${proposalId}/grant`, "POST", goi);
}

export function thuHoiDongY(contextId: string, purpose: MucDich, goi: Goi): Promise<void> {
  return ghi<void>(`/contexts/${contextId}/notebook/consents/${purpose}`, "DELETE", goi);
}

export function datRangBuoc(contextId: string, kind: LoaiRangBuoc, content: string, goi: Goi): Promise<RangBuocSong> {
  return ghi<RangBuocSong>(`/contexts/${contextId}/notebook/constraints/${kind}`, "PUT", goi, { content });
}

export function xoaRangBuoc(contextId: string, kind: LoaiRangBuoc, goi: Goi): Promise<void> {
  return ghi<void>(`/contexts/${contextId}/notebook/constraints/${kind}`, "DELETE", goi);
}

export function xemTruocDongSo(contextId: string, goi: Goi): Promise<XemTruocDongSo> {
  return ghi<XemTruocDongSo>(`/contexts/${contextId}/notebook/close/preview`, "POST", goi);
}

/** The revision is what makes the counts somebody read and the rows being closed the same rows. */
export function dongSo(contextId: string, revision: string, goi: Goi): Promise<void> {
  return ghi<void>(`/contexts/${contextId}/notebook/close`, "POST", goi, { revision });
}

// --- tờ giấy ----------------------------------------------------------------

export function docDanhSachTo(contextId: string, goi: Goi): Promise<{ papers: readonly ToTomTat[] }> {
  return doc<{ papers: readonly ToTomTat[] }>(`/contexts/${contextId}/papers`, goi);
}

export function xinToMoi(contextId: string, goi: Goi): Promise<LenhToGiay> {
  return ghi<LenhToGiay>(`/contexts/${contextId}/papers/draft`, "POST", goi);
}

export function docTo(paperId: string, goi: Goi): Promise<ToGiay> {
  return doc<ToGiay>(`/papers/${paperId}`, goi);
}

export function suaNhapSong(paperId: string, content: NoiDungTo, lyDo: string | null, goi: Goi): Promise<LenhToGiay> {
  return ghi<LenhToGiay>(`/papers/${paperId}/draft`, "PATCH", goi, { content, ly_do: lyDo });
}

/** Pressing send is agreeing to what was sent, so the version is pinned here. */
export function guiTo(paperId: string, version: number, goi: Goi): Promise<LenhToGiay> {
  return ghi<LenhToGiay>(`/papers/${paperId}/send`, "POST", goi, { version });
}

export function danhDauDaXem(paperId: string, version: number, goi: Goi): Promise<void> {
  return ghi<void>(`/papers/${paperId}/versions/${version}/viewed`, "POST", goi);
}

export function dongY(paperId: string, version: number, goi: Goi): Promise<LenhToGiay> {
  return ghi<LenhToGiay>(`/papers/${paperId}/versions/${version}/responses`, "POST", goi, { kind: "dong_y" });
}

export function deNghiSua(paperId: string, version: number, content: NoiDungTo, lyDo: string | null, goi: Goi): Promise<LenhToGiay> {
  return ghi<LenhToGiay>(`/papers/${paperId}/versions/${version}/responses`, "POST", goi, {
    kind: "de_nghi_sua",
    content,
    ly_do: lyDo,
  });
}

export function rutTo(paperId: string, version: number, goi: Goi): Promise<LenhToGiay> {
  return ghi<LenhToGiay>(`/papers/${paperId}/withdraw`, "POST", goi, { version });
}

export function nghiTuan(paperId: string, goi: Goi): Promise<LenhToGiay> {
  return ghi<LenhToGiay>(`/papers/${paperId}/skip`, "POST", goi);
}

export function ghiDaDi(paperId: string, goi: Goi): Promise<LenhToGiay> {
  return ghi<LenhToGiay>(`/papers/${paperId}/done`, "POST", goi);
}

export function giuMotDong(paperId: string, line: string, goi: Goi): Promise<{ id: string; line: string; created_at: string }> {
  return ghi<{ id: string; line: string; created_at: string }>(`/papers/${paperId}/keeps`, "POST", goi, { line });
}

/**
 * The name an attempt is filed under, so a retry keeps its key and a different
 * target gets its own (`attemptFor`). The version is part of the name wherever
 * the server pins one: pressing «Ừ» on v2 after a failed «Ừ» on v1 is a
 * different write, and reusing v1's key would have the server replay v1's
 * answer for it.
 */
export function tenLuot(viec: string, paperId: string, version?: number): string {
  return version === undefined ? `${viec}:${paperId}` : `${viec}:${paperId}:${version}`;
}
