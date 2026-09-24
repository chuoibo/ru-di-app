/**
 * Turning the conversation on screen into the bundle that goes with an AI call.
 *
 * Pure on purpose. Everything that can be wrong here -- the order, what a
 * deleted row becomes, what an image is allowed to carry -- is a privacy
 * property, and a privacy property checked by looking at a phone is not
 * checked. `tests/ai-boi-canh.test.mjs` is where it is checked.
 *
 * ## The caller passes what is on screen, not what is in memory
 *
 * `gomBoiCanhChat` takes the list the screen is drawing, which is
 * `tinChoHoiThoai(chat.tin)` and not `chat.tin`. The difference matters:
 * `tinChoHoiThoai` hides a `/vote` command once its poll card exists, so that
 * command is NOT on screen. The promise under the send button is "this is what
 * you are looking at", and it has to be true in the literal sense. One place
 * decides what is visible; this file does not get a second opinion.
 *
 * Unsent rows in `chat.hangCho` stay out too. They are drawn, but they have no
 * server id, no cursor and no server `created_at` (`useTinNhan.ts` keeps them
 * deliberately outside `tin`), and whatever the person is typing is already in
 * the prompt.
 */
import { docTheAi, type Tin } from "./tin-song";
import { GIOI_HAN_BOI_CANH, chuGon, ganNgan, type BoiCanh, type LuotBoiCanh, type VaiLuot } from "../ai/boi-canh";

/** A deleted row is still a turn. Its old text is never what travels. */
const CHU_DA_XOA = "Tin nhắn đã bị xoá";

function vaiCua(tin: Tin, personId: string): VaiLuot {
  if (tin.author_id === null) return "ai";
  return tin.author_id === personId ? "toi" : "ban";
}

/** A card travels as a label plus the one line carrying the group's decision. */
function chuTheAi(card: unknown): string {
  const the = docTheAi(card);
  if (the.loai === "poll") return `Thẻ bình chọn: ${chuGon(the.question)}`;
  if (the.loai === "itinerary") return `Tờ hẹn: ${chuGon(the.the.tieuDe)}`;
  if (the.loai === "text") return chuGon(the.text);
  return "Một thẻ";
}

/**
 * One label per author for the whole bundle, handed out by first appearance.
 *
 * The label is the member's display name (ADR-0036 §5), because a plan that
 * says «Lan dị ứng hải sản» is one the room can act on. Two members with the
 * same name must not become one speaker, so a repeat is «Lan (2)». A member
 * whose name the screen does not know yet is «Bạn N», never the account id.
 * The server checks every label again before a model reads it.
 */
function nhanCua(
  authorId: string,
  biDanh: Map<string, string>,
  daDung: Set<string>,
  tenCua?: (personId: string) => string | undefined,
): string {
  const co = biDanh.get(authorId);
  if (co !== undefined) return co;
  const ten = tenCua?.(authorId)?.trim();
  let nhan: string;
  if (ten) {
    nhan = ten;
    for (let k = 2; daDung.has(nhan); k++) nhan = `${ten} (${k})`;
  } else {
    let n = 1;
    do nhan = `Bạn ${n++}`;
    while (daDung.has(nhan));
  }
  daDung.add(nhan);
  biDanh.set(authorId, nhan);
  return nhan;
}

function luotCua(
  tin: Tin,
  personId: string,
  biDanh: Map<string, string>,
  daDung: Set<string>,
  tenCua?: (personId: string) => string | undefined,
): LuotBoiCanh {
  const vai = vaiCua(tin, personId);
  const chung = { id: tin.id, vai, luc: tin.created_at } as const;
  if (vai === "ban" && tin.author_id !== null) nhanCua(tin.author_id, biDanh, daDung, tenCua);
  const nhan = vai === "ban" && tin.author_id !== null ? { biDanh: biDanh.get(tin.author_id) } : {};
  switch (tin.kind) {
    case "deleted":
      return { ...chung, ...nhan, loai: "da-xoa", chu: CHU_DA_XOA };
    case "image":
      // The caption, or the bare word. Never `image_url`.
      return { ...chung, ...nhan, loai: "anh", chu: tin.body ? `Ảnh: ${chuGon(tin.body)}` : "Ảnh" };
    case "sticker":
      // A sticker id is this app's private vocabulary; a model reads nothing
      // from it. Say a sticker happened and keep the rhythm honest.
      return { ...chung, ...nhan, loai: "sticker", chu: "Sticker" };
    case "ai_card":
      return { ...chung, ...nhan, loai: "the", chu: chuTheAi(tin.card) };
    default:
      return { ...chung, ...nhan, loai: "chu", chu: chuGon(tin.body) };
  }
}

/**
 * @param tin the rows the screen is drawing, newest first (an inverted list).
 * @param tenCua the display name the screen shows for a member, or undefined
 *   when it does not know one; the label then falls back to «Bạn N».
 */
export function gomBoiCanhChat(opts: {
  tin: readonly Tin[];
  personId: string;
  tenCua?: (personId: string) => string | undefined;
  soLuot?: number;
  hanByte?: number;
}): BoiCanh {
  const soLuot = opts.soLuot ?? GIOI_HAN_BOI_CANH.soLuot;
  // `chat.tin` is newest first, because the list is inverted. A transcript is
  // read forwards, and handing a model a reversed conversation is a quality
  // fault nobody can see on screen. Take from the head, then flip.
  const moiNhat = opts.tin.slice(0, soLuot).reverse();
  const biDanh = new Map<string, string>();
  const daDung = new Set<string>();
  const bc: BoiCanh = {
    ban: 1,
    nguon: "chat-nhom",
    luot: moiNhat.map((t) => luotCua(t, opts.personId, biDanh, daDung, opts.tenCua)),
    tongLuot: opts.tin.length,
    daCat: opts.tin.length > soLuot,
  };
  return ganNgan(bc, opts.hanByte ?? GIOI_HAN_BOI_CANH.byte);
}
