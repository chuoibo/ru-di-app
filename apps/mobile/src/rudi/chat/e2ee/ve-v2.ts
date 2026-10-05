/**
 * How a v2 room's record is drawn by the chat screen built for the legacy
 * lane: messages in the legacy `Tin` shape, own sends on their way as pending
 * rows. Pure, so what an `ai_card` becomes (drawn as the assistant's only once
 * checked) is proved without a device.
 */
import type { TinChoGui } from "../hang-cho";
import type { LoaiPhanUng, PhanUngTomTat, Tin } from "../tin-song";
import { PHAN_UNG } from "../tin-song";
import type { Cho } from "./so-phong";
import type { TinV2 } from "./so-tin";

const GLYPH: Record<string, LoaiPhanUng> = Object.fromEntries(PHAN_UNG.map((p) => [p.glyph, p.kind]));
export const KIND_GLYPH: Record<LoaiPhanUng, string> = Object.fromEntries(PHAN_UNG.map((p) => [p.kind, p.glyph])) as Record<LoaiPhanUng, string>;

/**
 * Which `ai_card` a check is about: this very message (its logical send id),
 * who sealed it, and the invocation it claims. Per message, never per
 * invocation: a check of one card must not vouch for a second card the same
 * sender seals under the same invocation with other bytes (security review
 * 06/10).
 */
export const khoaXacMinh = (t: TinV2): string => `${t.id}|${t.authorId}|${t.ai?.invocationId ?? ""}`;

/**
 * A v2 message in the legacy `Tin` shape the screen draws, or null for one
 * not to draw yet. An `ai_card` is the assistant's only once checked against
 * the server's receipt (`xacMinh` true); until then it is not drawn at all,
 * and a card that failed the check is the sender's own words, said so.
 */
export function sangTin(t: TinV2, contextId: string, personId: string, anhDaMo: Record<string, string>, xacMinh: Record<string, boolean> = {}): Tin | null {
  if (t.ai !== null && !t.deleted) {
    const dung = xacMinh[khoaXacMinh(t)];
    if (dung === undefined) return null;
    let card: unknown = null;
    try {
      card = JSON.parse(t.ai.card);
    } catch {
      card = null;
    }
    if (dung && card !== null) {
      return {
        id: t.id, context_id: contextId, author_id: null, kind: "ai_card", body: null, image_url: null, card,
        created_at: t.at ?? new Date(0).toISOString(), cursor: String(t.sequence), reactions: [],
        reply_to: t.replyTo === null ? null : { id: t.replyTo, kind: "text", author_id: null, preview: "" }, deleted_at: null,
      };
    }
    return {
      id: t.id, context_id: contextId, author_id: t.authorId, kind: "text",
      body: "Tin này tự nhận là câu trả lời của Rủ Đi AI nhưng không khớp với câu trả lời máy chủ ghi nhận.",
      image_url: null, card: null, created_at: t.at ?? new Date(0).toISOString(), cursor: String(t.sequence), reactions: [], reply_to: null, deleted_at: null,
    };
  }
  return sangTinThuong(t, contextId, personId, anhDaMo);
}

function sangTinThuong(t: TinV2, contextId: string, personId: string, anhDaMo: Record<string, string>): Tin {
  const reactions: PhanUngTomTat[] = Object.keys(t.reactions)
    .filter((g) => Object.hasOwn(GLYPH, g))
    .map((g) => ({ kind: GLYPH[g], count: t.reactions[g].length, mine: t.reactions[g].includes(personId) }));
  // A sticker travels as its id in `body`, exactly as the legacy wire carries
  // one (tin-song.ts guiSticker): the screen draws the picture from that id.
  let body = t.body;
  if (t.sticker !== null) body = t.sticker.stickerId;
  return {
    id: t.id,
    context_id: contextId,
    author_id: t.authorId,
    kind: t.deleted ? "deleted" : t.sticker !== null ? "sticker" : t.media?.type === "image" ? "image" : "text",
    body,
    image_url: t.media?.type === "image" ? (anhDaMo[t.media.media.media_id] ?? null) : null,
    card: null,
    created_at: t.at ?? new Date(0).toISOString(),
    cursor: String(t.sequence),
    reactions,
    reply_to: t.replyTo === null ? null : { id: t.replyTo, kind: "text", author_id: null, preview: "" },
    deleted_at: t.deleted ? (t.at ?? new Date(0).toISOString()) : null,
  };
}

/** An own send still on its way, as the screen's pending row. */
export function sangCho(c: Cho, tinTheoId: Record<string, TinV2>): TinChoGui {
  const op = c.r.operation;
  const goc = op.type === "reply" ? tinTheoId[op.reply_to] : undefined;
  return {
    attempt: { key: c.id, at: c.luc },
    kind: op.type === "sticker" ? "sticker" : op.type === "image" ? "image" : "text",
    than: op.type === "sticker" ? op.sticker_id : op.type === "text" || op.type === "reply" ? op.body : "",
    phuDe: op.type === "image" ? op.caption : null,
    traLoi: op.type === "reply" ? { id: op.reply_to, kind: "text", author_id: goc?.authorId ?? null, preview: goc?.body ?? "" } : null,
    trangThai: c.hong ? "that-bai" : "dang-gui",
    loi: c.loi,
    thuLaiDuoc: c.thuLai,
    luc: new Date(c.luc).toISOString(),
  };
}

