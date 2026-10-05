/**
 * Amending an expense of a round already frozen or published (ADR-0056).
 *
 * What the server owns, and this module never does:
 *
 *   - Who is affected, and so who must agree: both ends of every transfer
 *     whose amount the correction changes. The server says it; the screen
 *     shows it. Nothing here decides an amendment applies.
 *   - The new transfers. They are computed on the server by the same
 *     arithmetic that froze the round; the screen shows old → new per pair.
 *   - Money law 1: the corrected shares are whole đồng, each checked here as a
 *     safe integer before it leaves the phone, and again on the server.
 *
 * A guest with no account answers on a review link the proposer shares, one
 * per guest, returned exactly once (like the round's own links); a member with
 * an account answers in the app, never through a link.
 */
import { BASE_URL, translatedAsActor, type Attempt, type Envelope } from "../../api";
import type { NghiaVu } from "./dot-thu";

/** The server's amount ceiling (allocator.MaxAmountVND): 10^12 đồng. */
export const MAX_AMOUNT_VND = 10 ** 12;

export type TrangThaiDieuChinh = "proposed" | "applied" | "rejected" | "expired";

/** One pair whose transfer changes: 0 before is a new transfer, 0 after a removed one. */
export type DongDieuChinh = { senderId: string; recipientId: string; cuVnd: number; moiVnd: number };

export type BenDieuChinh = {
  personId: string;
  traLoi: "dong_y" | "khong_dong_y" | null;
  qua: "app" | "guest_link" | "proposer" | null;
};

export type DieuChinh = {
  id: string;
  trangThai: TrangThaiDieuChinh;
  expenseId: string;
  nguoiDeXuat: string;
  lyDo: string;
  taoLuc: string;
  hetHan: string;
  xongLuc: string | null;
  dong: DongDieuChinh[];
  ben: BenDieuChinh[];
  choAi: string[];
};

/** One expense of the round as an amendment starts from: its shares now. */
export type KhoanTrongDot = {
  expenseId: string;
  moTa: string | null;
  nguoiTra: string;
  nguoiGhi: string;
  tongVnd: number;
  phanBo: { personId: string; vnd: number }[];
};

export type HoSoDieuChinh = {
  chuDot: string;
  trangThaiDot: string;
  khoan: KhoanTrongDot[];
  dieuChinh: DieuChinh[];
};

/** A review link, shown once to the proposer, to send to one guest. */
export type LinkDuyet = { senderId: string; url: string };

type Wire = {
  batch_owner_id: string;
  batch_status: string;
  expenses: {
    expense_id: string;
    description: string | null;
    paid_by_id: string;
    recorded_by_id: string;
    total_amount_vnd: number;
    allocations: { person_id: string; amount_vnd: number }[];
  }[];
  amendments: {
    id: string;
    status: TrangThaiDieuChinh;
    expense_id: string;
    proposed_by_id: string;
    reason: string;
    created_at: string;
    expires_at: string;
    resolved_at: string | null;
    lines: { sender_id: string; recipient_id: string; old_amount_vnd: number; new_amount_vnd: number }[];
    parties: { person_id: string; decision: "accept" | "reject" | null; via: BenDieuChinh["qua"] }[];
    pending_person_ids: string[];
  }[];
};

/** The server's refusals, in words a person reads. */
export const LOI_DIEU_CHINH: Record<string, string> = {
  amendment_already_open: "Đợt này đang có một đề xuất sửa chờ trả lời. Chờ nó xong (hoặc hết hạn) rồi đề xuất tiếp.",
  amendment_changes_nothing: "Số mới không làm ai phải chuyển khác đi, nên không có gì để sửa.",
  amendment_owes_nothing: "Sửa như vậy thì không còn ai phải chuyển tiền trong đợt này.",
  batch_not_amendable: "Đợt này không sửa theo cách này được.",
  expense_not_in_batch: "Khoản này không nằm trong đợt thu.",
  participant_not_member: "Mỗi người được chia phải còn ở trong nhóm.",
  allocation_total_invalid: "Tổng sau khi sửa phải lớn hơn 0 và không vượt trần số tiền.",
  allocation_amount_invalid: "Mỗi phần là số đồng nguyên, không âm.",
  amendment_reason_invalid: "Ghi lý do sửa (1 đến 500 ký tự).",
  amendment_not_allowed: "Chỉ người tạo đợt thu, người ghi hoặc người trả khoản này mới đề xuất sửa được.",
  amendment_closed: "Đề xuất này đã xong rồi.",
  amendment_already_answered: "Bạn đã trả lời đề xuất này.",
  amendment_not_affected: "Đề xuất này không đổi phần của bạn, nên không cần bạn trả lời.",
  amendment_answer_in_app: "Bạn trả lời trong ứng dụng.",
  amendment_stale: "Đợt thu đã đổi từ lúc đề xuất; đề xuất này không áp dụng được nữa.",
  amendment_not_found: "Không tìm thấy đề xuất này.",
  batch_not_found: "Không tìm thấy đợt thu này.",
};

/** `GET /batches/{id}/amendments`: the round's expenses and its amendments, newest first. */
export async function docHoSoDieuChinh(contextId: string, batchId: string, actorId: string): Promise<HoSoDieuChinh> {
  const w = await translatedAsActor<Wire>(LOI_DIEU_CHINH, `/batches/${batchId}/amendments`, { method: "GET", actorId, contexts: contextId });
  return {
    chuDot: w.batch_owner_id,
    trangThaiDot: w.batch_status,
    khoan: w.expenses.map((e) => ({
      expenseId: e.expense_id,
      moTa: e.description,
      nguoiTra: e.paid_by_id,
      nguoiGhi: e.recorded_by_id,
      tongVnd: e.total_amount_vnd,
      phanBo: e.allocations.map((a) => ({ personId: a.person_id, vnd: a.amount_vnd })),
    })),
    dieuChinh: w.amendments.map((a) => ({
      id: a.id,
      trangThai: a.status,
      expenseId: a.expense_id,
      nguoiDeXuat: a.proposed_by_id,
      lyDo: a.reason,
      taoLuc: a.created_at,
      hetHan: a.expires_at,
      xongLuc: a.resolved_at,
      dong: a.lines.map((l) => ({ senderId: l.sender_id, recipientId: l.recipient_id, cuVnd: l.old_amount_vnd, moiVnd: l.new_amount_vnd })),
      ben: a.parties.map((p) => ({
        personId: p.person_id,
        traLoi: p.decision === null ? null : p.decision === "accept" ? "dong_y" : "khong_dong_y",
        qua: p.via,
      })),
      choAi: a.pending_person_ids,
    })),
  };
}

/** Whole đồng within the ceiling, or the reason it is not (money law 1). */
export function loiPhan(vnd: number): string | null {
  if (!Number.isSafeInteger(vnd) || vnd < 0) return "Số đồng nguyên, không âm.";
  if (vnd > MAX_AMOUNT_VND) return "Vượt trần số tiền.";
  return null;
}

/** The corrected total, or null when any share is not whole đồng or the sum passes the ceiling. */
export function tongPhanBo(phanBo: Record<string, number>): number | null {
  let tong = 0;
  for (const vnd of Object.values(phanBo)) {
    if (loiPhan(vnd) !== null) return null;
    tong += vnd;
    if (!Number.isSafeInteger(tong) || tong > MAX_AMOUNT_VND) return null;
  }
  return tong;
}

/** `POST /batches/{id}/amendments`. Throws before the network on a share that is not whole đồng. */
export async function deXuatDieuChinh(input: {
  contextId: string;
  batchId: string;
  actorId: string;
  expenseId: string;
  lyDo: string;
  phanBo: Record<string, number>;
  attempt: Attempt;
}): Promise<{ id: string; trangThai: TrangThaiDieuChinh; links: LinkDuyet[] }> {
  if (tongPhanBo(input.phanBo) === null) throw new RangeError("amendment shares are not whole đồng within the ceiling");
  const r = await translatedAsActor<{ amendment_id: string; status: TrangThaiDieuChinh; review_links: { sender_id: string; path: string }[] }>(
    LOI_DIEU_CHINH,
    `/batches/${input.batchId}/amendments`,
    {
      body: { expense_id: input.expenseId, reason: input.lyDo, allocations: input.phanBo },
      actorId: input.actorId,
      attempt: input.attempt,
      contexts: input.contextId,
    },
  );
  return { id: r.amendment_id, trangThai: r.status, links: r.review_links.map((l) => ({ senderId: l.sender_id, url: BASE_URL + l.path })) };
}

/** `POST /batches/{id}/amendments/{aid}/decision`, the app's answer. */
export async function traLoiDieuChinh(input: {
  contextId: string;
  batchId: string;
  amendmentId: string;
  actorId: string;
  dongY: boolean;
  attempt: Attempt;
}): Promise<TrangThaiDieuChinh> {
  const r = await translatedAsActor<{ status: TrangThaiDieuChinh }>(LOI_DIEU_CHINH, `/batches/${input.batchId}/amendments/${input.amendmentId}/decision`, {
    body: { accept: input.dongY },
    actorId: input.actorId,
    attempt: input.attempt,
    contexts: input.contextId,
  });
  return r.status;
}

/* ------------------------------------------------------------ pure helpers */

export function dieuChinhDangMo(hoSo: HoSoDieuChinh): DieuChinh | null {
  return hoSo.dieuChinh.find((d) => d.trangThai === "proposed") ?? null;
}

/** ADR-0056 §2.2: the round's owner, or whoever recorded or paid the expense. */
export function coTheDeXuat(hoSo: HoSoDieuChinh, khoan: KhoanTrongDot, personId: string): boolean {
  return personId === hoSo.chuDot || personId === khoan.nguoiTra || personId === khoan.nguoiGhi;
}

export function toiCanTraLoi(d: DieuChinh, personId: string): boolean {
  return d.trangThai === "proposed" && d.choAi.includes(personId);
}

export function cauTrangThaiDieuChinh(d: DieuChinh): string {
  switch (d.trangThai) {
    case "proposed":
      return d.choAi.length === 1 ? "Chờ 1 người trả lời" : `Chờ ${d.choAi.length} người trả lời`;
    case "applied":
      return "Đã điều chỉnh";
    case "rejected":
      return "Không được đồng ý";
    case "expired":
      return "Hết hạn, giữ số cũ";
  }
}

/** The shares an amendment starts from: every member listed, the expense's own shares filled in. */
export function phanBoBanDau(khoan: KhoanTrongDot, thanhVien: readonly string[]): Record<string, number> {
  const out: Record<string, number> = {};
  for (const id of thanhVien) out[id] = 0;
  for (const p of khoan.phanBo) out[p.personId] = p.vnd;
  return out;
}

/**
 * After an amendment applied, a guest it touched holds the review URL, which
 * now opens their new share; the round's stored link for them is dead. The
 * stored envelopes are brought up to date: the URL from the review link, the
 * amount from the board the server now draws.
 */
export function linkSauDieuChinh(envelopes: readonly Envelope[], links: readonly LinkDuyet[], bang: readonly NghiaVu[]): Envelope[] {
  const moi = new Map(links.map((l) => [l.senderId, l.url.replace(/\/dieu-chinh$/, "")]));
  const out = envelopes.map((e) => {
    const url = moi.get(e.senderId);
    if (url === undefined) return e;
    const cua = bang.filter((n) => n.senderId === e.senderId);
    return {
      ...e,
      url,
      amountVnd: cua.reduce((s, n) => s + n.amountVnd, 0),
      obligations: cua.map((n) => ({ obligationId: n.id, amountVnd: n.amountVnd })),
    };
  });
  return out;
}

/** The share message for a review link: neutral, attributed, no amount (it may be forwarded). */
export function loiNhanLinkDuyet(tenNguoiNhan: string, linkDuyet: string): string {
  return `Đợt thu có đề xuất sửa số tiền phần của ${tenNguoiNhan}. Mở link để xem số cũ, số mới và trả lời:\n${linkDuyet}\n\nLink này dành cho ${tenNguoiNhan}.`;
}
