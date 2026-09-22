/**
 * The chat wire for the RuDi shell (M3): messages, reactions, read marks.
 *
 * Thin wrappers over `translatedAsActor` in `src/api.ts` so every call carries
 * the bearer (ADR-0014) and the `Idempotency-Key` for writes. The legacy chat
 * module (`src/screens/chat/tin-nhan.ts`) used its own `fetch` with actor
 * headers only, which a `prod` server ignores -- it cannot be reused here.
 *
 * Pure helpers (`gopTin`, `nhomTheoNgay`, `docTheAi`) are exported so the list
 * logic is testable without a device: an inverted FlatList wants newest-first,
 * a forward poll answers oldest-first, and mixing those up is how a chat shows
 * yesterday under today.
 */
import { type Attempt, newAttempt, translatedAsActor } from "../../api";
import {
  khoangGia,
  theTuCard,
  type DiaDiem,
  type KeHoach,
  type TheAi as TheKeHoach,
} from "../../screens/chat/ke-hoach";
import { docKhoiNhap, type KhoiNhapTrongThe } from "./to-hen-chung";

export type LoaiPhanUng = "heart" | "haha" | "like" | "wow" | "sad" | "fire";

export const PHAN_UNG: readonly { kind: LoaiPhanUng; glyph: string; nhan: string }[] = [
  { kind: "heart", glyph: "❤️", nhan: "Thích" },
  { kind: "haha", glyph: "😂", nhan: "Haha" },
  { kind: "like", glyph: "👍", nhan: "Đồng ý" },
  { kind: "wow", glyph: "😮", nhan: "Wow" },
  { kind: "sad", glyph: "😢", nhan: "Buồn" },
  { kind: "fire", glyph: "🔥", nhan: "Cháy" },
];

export function glyphPhanUng(kind: string): string {
  return PHAN_UNG.find((p) => p.kind === kind)?.glyph ?? "•";
}

export type PhanUngTomTat = { kind: LoaiPhanUng; count: number; mine: boolean };

export type LoaiTin = "text" | "image" | "ai_card" | "sticker" | "deleted";

/** The quoted message as the server reduced it (ADR-0021 §2.2): one line, no fetch. */
export type TrichDan = {
  id: string;
  kind: LoaiTin;
  author_id: string | null;
  preview: string;
};

export type Tin = {
  revision?: number;
  id: string;
  context_id: string;
  author_id: string | null;
  kind: LoaiTin;
  body: string | null;
  image_url: string | null;
  card: unknown | null;
  created_at: string;
  cursor: string;
  reactions?: PhanUngTomTat[];
  reply_to?: TrichDan | null;
  deleted_at?: string | null;
};

export type TrangTin = {
  context_id: string;
  messages: Tin[];
  next_cursor: string | null;
  has_more: boolean;
};

export type LuotAi = {
  context_id: string;
  spoke: boolean;
  reason: string;
  message: Tin | null;
};

/** `POST /messages` answers the stored message plus what the server did about a command. */
export type TinDaGui = Tin & {
  intent?: "plan" | "chia_bill" | "vote" | "mention" | null;
  companion?: LuotAi | null;
  vote?: { id: string; question: string } | null;
  expense_card?: Tin | null;
  intent_error?:
    | "vote_malformed"
    | "companion_rate_limited"
    | "chia_bill_not_available"
    | "chia_bill_no_expenses"
    | "chia_bill_refused"
    | null;
};

const LOI_CHAT: Record<string, string> = {
  permission_denied: "Bạn không còn ở trong nhóm này.",
  message_not_found: "Tin nhắn này không còn.",
  card_ungrounded: "Thẻ này không hợp lệ.",
  invalid_cursor: "Danh sách tin bị lệch, kéo để tải lại.",
  sticker_unknown: "Sticker này bản app chưa có.",
  reply_target_deleted: "Tin bạn muốn trả lời đã bị xoá.",
  reply_target_not_quotable: "Không trả lời được một thẻ; hãy trả lời một tin nhắn.",
  message_already_deleted: "Tin này đã bị xoá rồi.",
  message_kind_not_deletable: "Chỉ xoá được tin nhắn, ảnh hoặc sticker của chính bạn.",
  message_deleted: "Tin này đã bị xoá.",
};

const QUYEN = "group_admin,member";

export async function docTrangTin(
  contextId: string,
  personId: string,
  opts: { before?: string; after?: string; limit?: number } = {},
): Promise<TrangTin> {
  const params = new URLSearchParams();
  params.set("limit", String(opts.limit ?? 50));
  if (opts.before) params.set("before", opts.before);
  if (opts.after) params.set("after", opts.after);
  return translatedAsActor<TrangTin>(LOI_CHAT, `/contexts/${contextId}/messages?${params}`, {
    method: "GET",
    actorId: personId,
    roles: QUYEN,
    contexts: contextId,
  });
}

export async function guiTin(
  contextId: string,
  personId: string,
  body: string,
  attempt: Attempt,
  opts: { replyToId?: string | null } = {},
): Promise<TinDaGui> {
  return translatedAsActor<TinDaGui>(LOI_CHAT, `/contexts/${contextId}/messages`, {
    method: "POST",
    // `reply_to_id` only when there is one: a server older than L1 refuses an
    // unknown key (`extra=forbid`), and an ordinary message has no reply.
    body: { kind: "text", body, image_url: null, card: null, ...(opts.replyToId ? { reply_to_id: opts.replyToId } : {}) },
    actorId: personId,
    roles: QUYEN,
    contexts: contextId,
    attempt,
  });
}

/** Send one sticker by id (ADR-0021 §2.1). The picture lives on the phone; only the id travels. */
export async function guiSticker(
  contextId: string,
  personId: string,
  stickerId: string,
  attempt: Attempt,
  opts: { replyToId?: string | null } = {},
): Promise<TinDaGui> {
  return translatedAsActor<TinDaGui>(LOI_CHAT, `/contexts/${contextId}/messages`, {
    method: "POST",
    body: { kind: "sticker", body: stickerId, image_url: null, card: null, ...(opts.replyToId ? { reply_to_id: opts.replyToId } : {}) },
    actorId: personId,
    roles: QUYEN,
    contexts: contextId,
    attempt,
  });
}

/** Take back one's own message (ADR-0021 §2.3). 204: the row stays as «đã xoá». */
export async function xoaTin(contextId: string, messageId: string, personId: string): Promise<void> {
  await translatedAsActor<unknown>(LOI_CHAT, `/contexts/${contextId}/messages/${messageId}`, {
    method: "DELETE",
    actorId: personId,
    roles: QUYEN,
    contexts: contextId,
  });
}

/**
 * Send a photograph already uploaded to this group's storage (M8).
 *
 * `image_url` is the relative address `POST /contexts/{id}/photos` answered
 * with; the bytes never pass through this call. A caption is optional and
 * rides in `body`, which the server's payload CHECK allows for `image` (only
 * `card` has to be null) -- so one message carries the photo and the words
 * about it instead of two.
 */
export async function guiAnh(
  contextId: string,
  personId: string,
  imageUrl: string,
  caption: string | null,
  attempt: Attempt,
): Promise<TinDaGui> {
  const chu = caption === null ? null : caption.trim();
  return translatedAsActor<TinDaGui>(LOI_CHAT, `/contexts/${contextId}/messages`, {
    method: "POST",
    body: { kind: "image", body: chu === "" ? null : chu, image_url: imageUrl, card: null },
    actorId: personId,
    roles: QUYEN,
    contexts: contextId,
    attempt,
  });
}

export type DanhSachPhanUng = { message_id: string; reactions: PhanUngTomTat[] };

export async function themPhanUng(
  contextId: string,
  messageId: string,
  personId: string,
  kind: LoaiPhanUng,
): Promise<DanhSachPhanUng> {
  return translatedAsActor<DanhSachPhanUng>(
    LOI_CHAT,
    `/contexts/${contextId}/messages/${messageId}/reactions`,
    {
      method: "POST",
      body: { kind },
      actorId: personId,
      roles: QUYEN,
      contexts: contextId,
      attempt: newAttempt(),
    },
  );
}

export async function boPhanUng(
  contextId: string,
  messageId: string,
  personId: string,
  kind: LoaiPhanUng,
): Promise<DanhSachPhanUng> {
  return translatedAsActor<DanhSachPhanUng>(
    LOI_CHAT,
    `/contexts/${contextId}/messages/${messageId}/reactions/${kind}`,
    { method: "DELETE", actorId: personId, roles: QUYEN, contexts: contextId },
  );
}

export async function danhDauDaDoc(contextId: string, personId: string, messageId: string): Promise<void> {
  await translatedAsActor<unknown>(LOI_CHAT, `/contexts/${contextId}/read-mark`, {
    method: "PUT",
    body: { message_id: messageId },
    actorId: personId,
    roles: QUYEN,
    contexts: contextId,
    attempt: newAttempt(),
  });
}

// ---- pure helpers ----------------------------------------------------------

/** Newest first, no duplicates. Both inputs may be in either order. */
export function gopTin(dangGiu: Tin[], them: Tin[]): Tin[] {
  const theoId = new Map<string, Tin>();
  for (const t of [...dangGiu, ...them]) {
    const held = theoId.get(t.id);
    if (held?.kind === "deleted" && t.kind !== "deleted") continue;
    if (held?.revision !== undefined && (t.revision ?? -1) < held.revision) continue;
    theoId.set(t.id, t);
  }
  const result = [...theoId.values()].map((t) => t.reply_to && theoId.get(t.reply_to.id)?.kind === "deleted"
    ? { ...t, reply_to: { ...t.reply_to, kind: "deleted" as const, preview: "Tin nhắn đã bị xoá" } } : t);
  return result.sort((a, b) => {
    if (a.created_at === b.created_at) return a.id < b.id ? 1 : -1;
    return a.created_at < b.created_at ? 1 : -1;
  });
}

/** Invalidations must not widen the loaded history past unread pages. */
export function apDungAnhChup(current: Tin[], incoming: Tin[]): Tin[] {
  if (current.length === 0) return gopTin(current, incoming);
  const known = new Set(current.map((message) => message.id));
  const oldest = current[current.length - 1];
  const admitted = incoming.filter((message) => known.has(message.id) || message.created_at > oldest.created_at ||
    message.created_at === oldest.created_at && message.id >= oldest.id);
  const deleted = new Set(incoming.filter((message) => message.kind === "deleted").map((message) => message.id));
  return gopTin(current, admitted).map((message) => message.reply_to && deleted.has(message.reply_to.id)
    ? { ...message, reply_to: { ...message.reply_to, kind: "deleted" as const, preview: "Tin nhắn đã bị xoá" } }
    : message);
}

/** The cursor of the newest message held, for `?after=` polling. */
export function cursorMoiNhat(tin: Tin[]): string | null {
  return tin.length === 0 ? null : tin[0].cursor;
}

/** The cursor of the oldest message held, for `?before=` paging. */
export function cursorCuNhat(tin: Tin[]): string | null {
  return tin.length === 0 ? null : tin[tin.length - 1].cursor;
}

/** Unversioned legacy ACKs cannot overwrite a versioned feed snapshot. */
export function thayPhanUng(tin: Tin[], messageId: string, reactions: PhanUngTomTat[]): Tin[] {
  return tin.map((t) => (t.id === messageId && t.revision === undefined ? { ...t, reactions } : t));
}

/**
 * The held list after the server said 204 to a deletion: the row flips to
 * `deleted` with no payload and no reactions, and every quote of it now reads
 * as the deletion -- the same shape the next poll will confirm.
 */
export function thayTinDaXoa(tin: Tin[], messageId: string, deletedAt: string): Tin[] {
  return tin.map((t) => {
    if (t.id === messageId) {
      return { ...t, kind: "deleted", body: null, image_url: null, card: null, reactions: [], deleted_at: deletedAt };
    }
    if (t.reply_to && t.reply_to.id === messageId) {
      return { ...t, reply_to: { ...t.reply_to, kind: "deleted", preview: "Tin nhắn đã bị xoá" } };
    }
    return t;
  });
}

/** What a conversation row shows for the newest message; labels for what has no words. */
export function xemTruocTinCuoi(cuoi: { kind: string; preview: string }): string {
  if (cuoi.kind === "sticker") return "Đã gửi một sticker";
  if (cuoi.kind === "deleted") return "Tin nhắn đã bị xoá";
  return cuoi.preview;
}

/** The quote strip drawn while composing a reply: who said it, one short line. */
export function trichTu(tin: Tin, tenNguoi: (id: string | null) => string): TrichDan {
  let preview: string;
  if (tin.kind === "image") preview = tin.body ? `Ảnh: ${tin.body}` : "Ảnh";
  else if (tin.kind === "sticker") preview = "Sticker";
  else if (tin.kind === "deleted") preview = "Tin nhắn đã bị xoá";
  else preview = (tin.body ?? "").replace(/\s+/g, " ").trim();
  if (preview.length > 80) preview = preview.slice(0, 79) + "…";
  return { id: tin.id, kind: tin.kind, author_id: tin.author_id, preview: preview || tenNguoi(tin.author_id) };
}

export type HangHienThi =
  | { loai: "tin"; tin: Tin }
  | { loai: "ngay"; nhan: string; key: string };

/**
 * Day dividers for an inverted list: the divider for a day sits AFTER (below,
 * in list order) the newest-first messages of that day, i.e. visually above
 * them. Labels are «Hôm nay», «Hôm qua», or dd/MM.
 */
export function nhomTheoNgay(tin: Tin[], homNay: Date = new Date()): HangHienThi[] {
  const ra: HangHienThi[] = [];
  let ngayHienTai: string | null = null;
  for (const t of tin) {
    const ngay = t.created_at.slice(0, 10);
    if (ngayHienTai !== null && ngay !== ngayHienTai) {
      ra.push({ loai: "ngay", nhan: nhanNgay(ngayHienTai, homNay), key: `ngay-${ngayHienTai}` });
    }
    ra.push({ loai: "tin", tin: t });
    ngayHienTai = ngay;
  }
  if (ngayHienTai !== null) {
    ra.push({ loai: "ngay", nhan: nhanNgay(ngayHienTai, homNay), key: `ngay-${ngayHienTai}` });
  }
  return ra;
}

export function nhanNgay(yyyyMmDd: string, homNay: Date): string {
  const nay = homNay.toISOString().slice(0, 10);
  const qua = new Date(homNay.getTime() - 86_400_000).toISOString().slice(0, 10);
  if (yyyyMmDd === nay) return "Hôm nay";
  if (yyyyMmDd === qua) return "Hôm qua";
  const [, mm, dd] = yyyyMmDd.split("-");
  return `${dd}/${mm}`;
}

export function gioPhut(iso: string): string {
  const d = new Date(iso);
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  return `${hh}:${mm}`;
}

export type TheAi =
  | { loai: "text"; text: string }
  | { loai: "places"; the: Extract<TheKeHoach, { kind: "places" }> }
  | { loai: "itinerary"; the: KeHoach; outingId?: string; nhapChung?: KhoiNhapTrongThe }
  | { loai: "poll"; vote_id: string; question: string; options: { id: string; label: string }[] }
  | {
      loai: "expense_draft";
      drafts: { title: string; amount_vnd: number; paid_by_id: string; shared_by: string[]; needs_review: boolean }[];
    }
  | { loai: "khac" };

function laBanGhi(v: unknown): v is Record<string, unknown> {
  return v !== null && typeof v === "object" && !Array.isArray(v);
}

function chuoiNeuCo(v: unknown): string | undefined {
  return typeof v === "string" ? v : undefined;
}

/** A stable list key per row; a message's key is prefixed so it is a key, not an id shown. */
export function khoaHang(hang: HangHienThi): string {
  if (hang.loai === "ngay") return hang.key;
  return `tin-${hang.tin.id}`;
}

/** Hide only a poll command that has a matching persisted poll card. */
export function tinChoHoiThoai(messages: readonly Tin[]): Tin[] {
  const polls = new Map<string, number[]>();
  for (const message of messages) {
    const card = docTheAi(message.card);
    if (card.loai !== "poll") continue;
    const question = card.question.replace(/\?$/, "").trim();
    const times = polls.get(question) ?? [];
    times.push(Date.parse(message.created_at));
    polls.set(question, times);
  }
  return messages.filter((message) => {
    if (message.kind !== "text" || !message.body?.startsWith("/vote ")) return true;
    const question = message.body.slice(6).split("?")[0].trim();
    return !(polls.get(question) ?? []).some((at) => Math.abs(at - Date.parse(message.created_at)) < 30000);
  });
}

/** Read a server card without trusting its shape. Anything odd is `khac`. */
export function docTheAi(card: unknown): TheAi {
  if (!laBanGhi(card) || !laBanGhi(card.payload)) return { loai: "khac" };
  const p = card.payload;
  switch (card.kind) {
    case "text":
      return typeof p.text === "string" ? { loai: "text", text: p.text } : { loai: "khac" };
    case "places":
    case "itinerary": {
      // Same wire shape App B drew (`ke-hoach.ts`): the server sends catalogue
      // places, not a list the client invents. An unreadable card is `khac`.
      const the = theTuCard(card);
      if (the === null || the.kind === "text") return { loai: "khac" };
      if (the.kind === "places") return { loai: "places", the };
      // A shared sheet is an itinerary card too; the `draft` block is what
      // says the group may still edit it, rather than accept it.
      const nhapChung = docKhoiNhap(card);
      return { loai: "itinerary", the, ...(typeof p.outing_id === "string" && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(p.outing_id) ? { outingId: p.outing_id } : {}), ...(nhapChung ? { nhapChung } : {}) };
    }
    case "poll": {
      const options = Array.isArray(p.options)
        ? p.options
            .filter(laBanGhi)
            .map((o) => ({ id: String(o.id ?? ""), label: String(o.label ?? "") }))
            .filter((o) => o.id !== "" && o.label !== "")
        : [];
      if (typeof p.vote_id !== "string" || typeof p.question !== "string" || options.length < 2) {
        return { loai: "khac" };
      }
      return { loai: "poll", vote_id: p.vote_id, question: p.question, options };
    }
    case "expense_draft": {
      const drafts = Array.isArray(p.drafts)
        ? p.drafts.filter(laBanGhi).map((d) => ({
            title: String(d.title ?? ""),
            amount_vnd: typeof d.amount_vnd === "number" ? d.amount_vnd : 0,
            paid_by_id: String(d.paid_by_id ?? ""),
            shared_by: Array.isArray(d.shared_by) ? d.shared_by.map(String) : [],
            needs_review: d.needs_review !== false,
          }))
        : [];
      return { loai: "expense_draft", drafts };
    }
    default:
      return { loai: "khac" };
  }
}

/** Copy for what the server said about a command, or null when nothing to say. */
export function cauYDinh(gui: TinDaGui): string | null {
  switch (gui.intent_error) {
    case "companion_rate_limited":
      return "Hết lượt hỏi Rủ Đi AI trong phút này. Tin của bạn vẫn được gửi.";
    case "vote_malformed":
      return "Bình chọn cần dạng: /vote Câu hỏi? Lựa chọn A | Lựa chọn B";
    case "chia_bill_not_available":
      return "Chia bill từ chat chưa sẵn sàng trên máy chủ này.";
    case "chia_bill_no_expenses":
      return "Không thấy khoản chi nào trong các tin gần đây.";
    case "chia_bill_refused":
      return "Máy chủ từ chối bản đọc lần này. Thử lại sau.";
    default:
      break;
  }
  if (gui.companion && !gui.companion.spoke) {
    switch (gui.companion.reason) {
      case "unavailable":
        return "Rủ Đi AI chưa nối được mô hình trên máy chủ này.";
      case "ungrounded":
        return "Rủ Đi AI có ý nhưng không nêu được địa điểm trong danh mục, nên im lặng.";
      default:
        return "Rủ Đi AI đang im lặng (" + gui.companion.reason + ").";
    }
  }
  return null;
}

/** One line under a place on an AI card: price band, hours, distance -- what is known. */
export function moTaDiaDiem(d: DiaDiem): string {
  const phan: string[] = [];
  const gia = khoangGia(d);
  if (gia !== null) phan.push(gia);
  if (d.gioMo !== undefined) phan.push(`mở ${d.gioMo}`);
  if (d.cachKm !== undefined) phan.push(`${d.cachKm} km`);
  return phan.length === 0 ? "Trong danh mục Rủ Đi" : phan.join(" · ");
}
