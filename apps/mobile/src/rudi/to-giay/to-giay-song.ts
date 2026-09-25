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
 * `thongDiepNguoiDoc`'s generic sentences are written for refusals in general.
 * `paper_version_stale` is not a refusal at all from the reader's side; it is
 * news, that the other person wrote something while this screen was open.
 *
 * ## Why commands do not carry content back
 *
 * Every command answers `{id, state, version, outing_id?}` and nothing else, so
 * a retry after a lost reply replays exactly that (ADR-0027 §3). The screens
 * therefore re-read after a command rather than patching state from its answer;
 * `useToGiay` is what does that, once, in one place.
 */
import { type Attempt, translatedAsActor } from "../../api";
import type { GuSo } from "./gu-doi";
import type { NoiDungTo, ToGiay } from "./to-giay";

/** The rungs both climb: only these are ever pending for the other person. */
export type MucDichBac = "lap_so" | "bat_doi" | "doc_chat";
/** The ladder, plus `chia_gu`: each person's own taste switch (ADR-0034). */
export type MucDich = MucDichBac | "chia_gu";
export type LoaiRangBuoc = "khong_an_duoc" | "dung";

export interface DongYCuaToi {
  purpose: MucDich;
  granted: boolean;
}

export interface DeNghiCho {
  id: string;
  /** A `chia_gu` proposal completes as it is filed, so it is never pending. */
  purpose: MucDichBac;
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
  /**
   * What BOTH agreed to on one proposal, in ladder order. Absent on a server
   * older than 23/09, where the client falls back to the per-person maps.
   */
  granted_purposes?: readonly MucDich[];
  /** Null outside «Một đôi»; absent on a server older than 25/09 (ADR-0034). */
  taste?: GuSo | null;
  /** «Người lo» of this week; null outside an open «Một đôi» (ADR-0034 §2.4). */
  week_role?: VaiTuan | null;
}

/** `PairWeekRoleResponse`: inferred from the notebook (`suy`) or chosen (`chon`). */
export interface VaiTuan {
  tuan: string;
  nguoi_lo: readonly string[];
  cach: "suy" | "chon";
  diem: readonly { person_id: string; score: number }[];
}

export type ChonLo = "toi" | "nguoi_kia" | "ca_hai";

/**
 * One row of `GET /contexts/{id}/papers`; no versions, no responses.
 *
 * `chang_dau` and `dong_giu_dau` are the two facts a closed row shows besides
 * the date. They ride on the list because the list is that row's only read:
 * fetching each closed sheet whole to render one line would be one request per
 * row on every poll. They arrive as pieces, not as a sentence -- the row writes
 * «19:00 Ăn tối» or «Giữ lại: …» in its own words.
 */
export interface ToTomTat {
  id: string;
  state: ToGiay["state"];
  version: number;
  tuan: string;
  ngay: string | null;
  expires_at: string;
  chang_dau: { gio: string; viec: string } | null;
  dong_giu_dau: string | null;
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
 * speaks. A code missing from here falls through to the generic sentence.
 * `pair_chat_consent_required` used to be first here; its only raise site was
 * the automatic companion turn, deleted by ADR-0036 §2.1.
 */
export const LOI_TO_GIAY: Record<string, string> = {
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
  // The other person already proposed the same thing: answer theirs, do not
  // file a second one (a second one used to light the rung with no agreement).
  consent_proposal_pending: "Người ấy vừa đề nghị đúng việc này. Mở lại để đồng ý lời đề nghị của họ.",
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

/**
 * Every call below writes its path inline, and that is deliberate.
 *
 * The first cut routed all nineteen through two helpers taking `path: string`.
 * `tests/test_api_contract_unresolved_pin.py` went red on both: the contract
 * gate reads the literal handed to `translatedAsActor`, so a helper hid every
 * route in this feature behind one unreadable argument. Pinning the two would
 * have made the gate green while blinding it to all nineteen -- the same trade
 * the migration parity gate refused when a column helper hid column names.
 *
 * So the repetition is the gate's price, paid once per route.
 */

// --- sổ ---------------------------------------------------------------------

export function docSo(contextId: string, goi: Goi): Promise<SoHaiNguoi> {
  return translatedAsActor<SoHaiNguoi>(LOI_TO_GIAY, `/contexts/${contextId}/notebook`, { actorId: goi.actorId, method: "GET" });
}

export function deNghiDongY(contextId: string, purpose: MucDich, goi: Goi): Promise<DeNghiCho> {
  return translatedAsActor<DeNghiCho>(LOI_TO_GIAY, `/contexts/${contextId}/notebook/proposals`, { actorId: goi.actorId, method: "POST", attempt: goi.attempt, body: { purpose } });
}

export function dongYDeNghi(contextId: string, proposalId: string, goi: Goi): Promise<DeNghiCho> {
  return translatedAsActor<DeNghiCho>(LOI_TO_GIAY, `/contexts/${contextId}/notebook/proposals/${proposalId}/grant`, { actorId: goi.actorId, method: "POST", attempt: goi.attempt });
}

export function thuHoiDongY(contextId: string, purpose: MucDich, goi: Goi): Promise<void> {
  return translatedAsActor<void>(LOI_TO_GIAY, `/contexts/${contextId}/notebook/consents/${purpose}`, { actorId: goi.actorId, method: "DELETE", attempt: goi.attempt });
}

export function datVaiTuan(contextId: string, lo: ChonLo, goi: Goi): Promise<VaiTuan> {
  return translatedAsActor<VaiTuan>(LOI_TO_GIAY, `/contexts/${contextId}/notebook/week-role`, { actorId: goi.actorId, method: "PUT", attempt: goi.attempt, body: { lo } });
}

export function datRangBuoc(contextId: string, kind: LoaiRangBuoc, content: string, goi: Goi): Promise<RangBuocSong> {
  return translatedAsActor<RangBuocSong>(LOI_TO_GIAY, `/contexts/${contextId}/notebook/constraints/${kind}`, { actorId: goi.actorId, method: "PUT", attempt: goi.attempt, body: { content } });
}

export function xoaRangBuoc(contextId: string, kind: LoaiRangBuoc, goi: Goi): Promise<void> {
  return translatedAsActor<void>(LOI_TO_GIAY, `/contexts/${contextId}/notebook/constraints/${kind}`, { actorId: goi.actorId, method: "DELETE", attempt: goi.attempt });
}

export function xemTruocDongSo(contextId: string, goi: Goi): Promise<XemTruocDongSo> {
  return translatedAsActor<XemTruocDongSo>(LOI_TO_GIAY, `/contexts/${contextId}/notebook/close/preview`, { actorId: goi.actorId, method: "POST", attempt: goi.attempt });
}

/** The revision is what makes the counts somebody read and the rows being closed the same rows. */
export function dongSo(contextId: string, revision: string, goi: Goi): Promise<void> {
  return translatedAsActor<void>(LOI_TO_GIAY, `/contexts/${contextId}/notebook/close`, { actorId: goi.actorId, method: "POST", attempt: goi.attempt, body: { revision } });
}

// --- tờ giấy ----------------------------------------------------------------

export function docDanhSachTo(contextId: string, goi: Goi): Promise<{ papers: readonly ToTomTat[] }> {
  return translatedAsActor<{ papers: readonly ToTomTat[] }>(LOI_TO_GIAY, `/contexts/${contextId}/papers`, { actorId: goi.actorId, method: "GET" });
}

export function xinToMoi(contextId: string, goi: Goi): Promise<LenhToGiay> {
  return translatedAsActor<LenhToGiay>(LOI_TO_GIAY, `/contexts/${contextId}/papers/draft`, { actorId: goi.actorId, method: "POST", attempt: goi.attempt });
}

export function docTo(paperId: string, goi: Goi): Promise<ToGiay> {
  return translatedAsActor<ToGiay>(LOI_TO_GIAY, `/papers/${paperId}`, { actorId: goi.actorId, method: "GET" });
}

export function suaNhapSong(paperId: string, content: NoiDungTo, lyDo: string | null, goi: Goi): Promise<LenhToGiay> {
  return translatedAsActor<LenhToGiay>(LOI_TO_GIAY, `/papers/${paperId}/draft`, { actorId: goi.actorId, method: "PATCH", attempt: goi.attempt, body: { content, ly_do: lyDo } });
}

/** Pressing send is agreeing to what was sent, so the version is pinned here. */
export function guiTo(paperId: string, version: number, goi: Goi): Promise<LenhToGiay> {
  return translatedAsActor<LenhToGiay>(LOI_TO_GIAY, `/papers/${paperId}/send`, { actorId: goi.actorId, method: "POST", attempt: goi.attempt, body: { version } });
}

export function danhDauDaXem(paperId: string, version: number, goi: Goi): Promise<void> {
  return translatedAsActor<void>(LOI_TO_GIAY, `/papers/${paperId}/versions/${version}/viewed`, { actorId: goi.actorId, method: "POST", attempt: goi.attempt });
}

export function dongY(paperId: string, version: number, goi: Goi): Promise<LenhToGiay> {
  return translatedAsActor<LenhToGiay>(LOI_TO_GIAY, `/papers/${paperId}/versions/${version}/responses`, { actorId: goi.actorId, method: "POST", attempt: goi.attempt, body: { kind: "dong_y" } });
}

export function deNghiSua(paperId: string, version: number, content: NoiDungTo, lyDo: string | null, goi: Goi): Promise<LenhToGiay> {
  return translatedAsActor<LenhToGiay>(LOI_TO_GIAY, `/papers/${paperId}/versions/${version}/responses`, { actorId: goi.actorId, method: "POST", attempt: goi.attempt, body: {
    kind: "de_nghi_sua",
    content,
    ly_do: lyDo,
  } });
}

export function rutTo(paperId: string, version: number, goi: Goi): Promise<LenhToGiay> {
  return translatedAsActor<LenhToGiay>(LOI_TO_GIAY, `/papers/${paperId}/withdraw`, { actorId: goi.actorId, method: "POST", attempt: goi.attempt, body: { version } });
}

export function nghiTuan(paperId: string, goi: Goi): Promise<LenhToGiay> {
  return translatedAsActor<LenhToGiay>(LOI_TO_GIAY, `/papers/${paperId}/skip`, { actorId: goi.actorId, method: "POST", attempt: goi.attempt });
}

export function ghiDaDi(paperId: string, goi: Goi): Promise<LenhToGiay> {
  return translatedAsActor<LenhToGiay>(LOI_TO_GIAY, `/papers/${paperId}/done`, { actorId: goi.actorId, method: "POST", attempt: goi.attempt });
}

export function giuMotDong(paperId: string, line: string, goi: Goi): Promise<{ id: string; line: string; created_at: string }> {
  return translatedAsActor<{ id: string; line: string; created_at: string }>(LOI_TO_GIAY, `/papers/${paperId}/keeps`, { actorId: goi.actorId, method: "POST", attempt: goi.attempt, body: { line } });
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
