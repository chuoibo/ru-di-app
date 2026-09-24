/**
 * The context bundle: the one thing the caller hands the model, and the one
 * shape both AI surfaces use (ADR-0036 §2.2).
 *
 * The server does not read the conversation. It cannot: chat v2 is end to end
 * encrypted and the server holds no key. So context reaches a model exactly
 * one way, by the client putting it there. That makes this file a privacy
 * boundary, not a serialiser, and it is why every field below is a decision
 * rather than a mapping.
 *
 * ## What never travels, and why each one is a rule rather than an oversight
 *
 * **Account ids.** A turn carries `vai` ("me", "a friend", "the AI") and,
 * for other people, a `biDanh`. Never a person id.
 *
 * `biDanh` used to be a pseudonym minted per bundle («Bạn 1», «Bạn 2»). Since
 * 2026-09-24 it is the member's display name, the one the room already sees
 * above each bubble (ADR-0036 §5, product lead's decision): an answer that
 * says «Lan dị ứng hải sản» is one the group can act on, and «Bạn 1 dị ứng hải
 * sản» makes everybody work out who Bạn 1 was. Two members with the same name
 * are «Lan» and «Lan (2)» so they stay two speakers; a member whose name is not
 * known yet is «Bạn N». The words above the send button say names go along,
 * and they changed in the same change, because a stated privacy promise is not
 * withdrawn from one side (ADR-0036 §2.5). The server runs every label through
 * the prompt-safety test before a model reads it, since a display name is text
 * a person typed. «Chỉ gửi lời nhờ» still sends no chat at all; the roster
 * the server adds still names the members, and `cauBoiCanh(null)` says so.
 *
 * **`image_url`.** Never, under any circumstance. It is an authorised read
 * route, and handing one to a server-side model hands over an entry point
 * rather than a picture. An image travels as its caption or as the word
 * «Ảnh», and nothing else.
 *
 * **Card innards.** An `ai_card` travels as a label plus the one line that
 * carries the group's decision (a poll's question, an itinerary's title). Not
 * the card. Cards hold `budget_per_person_vnd` and a list of catalogue places;
 * money the person typed into the prompt is their own choice, money leaking
 * out of a card is automatic, and the two are not the same thing.
 *
 * **A deleted message's old body.** It stays a turn, because it is still a row
 * on screen and dropping it would splice two unrelated turns into a false
 * exchange. But its text is a constant, and this module does not read `body`
 * for that kind even if a server left one behind.
 */
import type { PhieuNguCanh } from "../nep/phieu";

/**
 * The client's own ceiling. The server refuses above a higher one; this is the
 * number that actually decides what gets sent, and it is set so the two can
 * never disagree: 40 x 300 runes plus a 4000-rune prompt stays under the
 * server's total-rune refusal by construction.
 */
export const GIOI_HAN_BOI_CANH = Object.freeze({ soLuot: 40, chuMoiLuot: 300, byte: 24_000 });

/** Who spoke. Never a name, never an id. */
export type VaiLuot = "toi" | "ban" | "ai";

/** What the row was on screen, so the model reads a rhythm and not a gap. */
export type LoaiLuot = "chu" | "anh" | "sticker" | "the" | "da-xoa";

export type LuotBoiCanh = {
  /**
   * The server message id, and the only field here the server can check.
   *
   * It is not a privacy cost: the server already owns these ids, and they never
   * reach the model. It buys one thing, and that thing matters because other
   * people read it. The published card says «Nếp đã đọc N tin», and everyone in
   * the room sees that line. Without an id per turn, N is a number the caller
   * asserted about itself; with one, the server can confirm every turn is a real
   * message of THIS room before it lets that sentence be published.
   *
   * It does not make the text truthful -- a member can still paste anything into
   * the prompt -- and the server does not pretend otherwise. It makes the COUNT
   * truthful, which is the part addressed to third parties.
   */
  id: string;
  vai: VaiLuot;
  /**
   * The speaker's display name, deduplicated inside ONE bundle («Lan (2)»), or
   * «Bạn N» when unknown. A label, not an identity: never a person id.
   */
  biDanh?: string;
  loai: LoaiLuot;
  /** ISO instant, so the model can read pacing. */
  luc: string;
  chu: string;
};

export type BoiCanh = {
  /** Wire version. The server refuses a shape it does not know rather than guessing. */
  ban: 1;
  nguon: "chat-nhom" | "nep-rieng";
  luot: LuotBoiCanh[];
  /**
   * How many turns the person could see before anything was dropped. Without
   * it the screen can only say "24 turns" and never "24 of 130", which is the
   * difference between a count and an honest count.
   */
  tongLuot: number;
  daCat: boolean;
  /** Nếp only: what the open screen declared about itself (ADR-0033 §2.5). */
  phieu?: PhieuNguCanh;
};

const BO_MA = new TextEncoder();

export function soByte(s: string): number {
  return BO_MA.encode(s).length;
}

/**
 * Cut to a byte ceiling without splitting a character.
 *
 * Binary search on code points rather than walking continuation bytes: fewer
 * places to be wrong, and the cost is a handful of encodes.
 */
export function catTheoByte(s: string, han: number): string {
  if (soByte(s) <= han) return s;
  const ky = Array.from(s);
  let thap = 0;
  let cao = ky.length;
  while (thap < cao) {
    const giua = Math.ceil((thap + cao) / 2);
    if (soByte(ky.slice(0, giua).join("")) <= han) thap = giua;
    else cao = giua - 1;
  }
  return ky.slice(0, thap).join("");
}

/** One line of prose, capped by code point so a cap means the same in any script. */
export function chuGon(raw: string | null | undefined, han: number = GIOI_HAN_BOI_CANH.chuMoiLuot): string {
  const mot = (raw ?? "").replace(/\s+/g, " ").trim();
  const ky = Array.from(mot);
  return ky.length <= han ? mot : ky.slice(0, han).join("");
}

export function soByteBoiCanh(bc: BoiCanh): number {
  return soByte(JSON.stringify(bc));
}

/**
 * Bring a bundle under the byte ceiling by dropping whole turns from the OLD
 * end. The newest turn is never mangled: it is the one the person was looking
 * at when they pressed send. Only when a single turn is itself too large does
 * its text get cut.
 */
export function ganNgan(bc: BoiCanh, han: number = GIOI_HAN_BOI_CANH.byte): BoiCanh {
  const ra: BoiCanh = { ...bc, luot: [...bc.luot] };
  while (ra.luot.length > 1 && soByteBoiCanh(ra) > han) {
    ra.luot.shift();
    ra.daCat = true;
  }
  if (ra.luot.length === 1 && soByteBoiCanh(ra) > han) {
    const thua = soByteBoiCanh(ra) - han;
    const mot = ra.luot[0];
    ra.luot = [{ ...mot, chu: catTheoByte(mot.chu, Math.max(0, soByte(mot.chu) - thua)) }];
    ra.daCat = true;
  }
  return ra;
}

/**
 * A cheap stable string standing for this exact bundle.
 *
 * It exists for idempotency, and that is not a detail. The client keys an
 * attempt so a double tap does not run twice; the server refuses a repeat of
 * the same logical call. Key either side on the prompt alone and the same
 * question asked again over NEWER messages collides with the old digest, so
 * the server answers 200 with the OLD card and the person believes the AI just
 * read what they just said. That failure is silent, which is why the bundle
 * has to be part of the key on both sides.
 */
export function vanTay(bc: BoiCanh): string {
  const cuoi = bc.luot.length > 0 ? bc.luot[bc.luot.length - 1].luc : "";
  return `${bc.ban}:${bc.nguon}:${bc.luot.length}:${bc.tongLuot}:${cuoi}:${soByteBoiCanh(bc)}`;
}

/**
 * The sentence above the send button.
 *
 * Counted at call time, never written into a constant: a number that drifts
 * out of step with the payload is the exact lie this block exists to prevent.
 */
export function cauBoiCanh(bc: BoiCanh | null): string {
  // The server's roster names the members even without a bundle (ADR-0036 §5),
  // so this sentence says so rather than implying nothing but the prompt goes.
  if (bc === null) return "Chỉ lời nhờ trong ô này, cùng tên hiển thị của các thành viên. Không tin nhắn nào đi kèm.";
  if (bc.luot.length === 0) return "Nhóm chưa có tin nào, nên mình chỉ gửi lời nhờ trong ô này.";
  if (bc.daCat || bc.luot.length < bc.tongLuot) {
    return `${bc.luot.length} tin gần nhất trong ${bc.tongLuot} tin bạn đang thấy, kèm lời nhờ trong ô này. Phần cũ hơn mình để lại.`;
  }
  return `${bc.luot.length} tin gần nhất bạn đang thấy, kèm lời nhờ trong ô này.`;
}

/**
 * The speaker label the SCREEN shows. Deliberately not the same string the
 * server puts in front of the model: on screen "toi" is the person reading, so
 * it reads «Bạn»; in the payload it reads the caller's own display name (or
 * «Mình» when that name cannot be used), because there the reader is the
 * model. Two audiences, two words, one source of truth for the role.
 */
export function nhanVai(l: LuotBoiCanh): string {
  if (l.vai === "toi") return "Bạn";
  if (l.vai === "ai") return "Rủ Đi AI";
  return l.biDanh ?? "Một người trong nhóm";
}
