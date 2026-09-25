/**
 * Finding `@Rủ Đi` in what a person typed, and what goes with it (ADR-0039,
 * proposed).
 *
 * Since the group AI answers inside the thread, `@Rủ Đi …` is an ordinary
 * message: it is posted through the send queue like any other, and only THEN
 * does the client invoke the AI, naming that message. The server never starts
 * anything from message text. So this module decides one thing locally --
 * «is this message also a request to the AI» -- and it is pure so the answer
 * can be checked without a device (`tests/nhac-ai.test.mjs`).
 *
 * The mention counts anywhere in the message («tối nay @Rủ Đi gợi ý quán»),
 * in either Unicode form a keyboard produces (NFC, or NFD with combining
 * accents), with or without the accents («@ru di», «@rudi»), and never inside
 * an email address (an address that happens to end in the app's name is
 * somebody's mailbox, not a question). The
 * two slash commands count only at the start, as they always have.
 */
import { LOI_NHO_CHIA_BILL, type LenhAi } from "./ai-invocations";

/** What a bare `@Rủ Đi` or `/plan` asks for when nothing else was typed. */
export const LOI_NHO_PLAN = "Phác giúp nhóm một kèo đi chơi";

export type NhacAi = {
  lenh: LenhAi;
  /** The request without the mention or the command: what the model reads as the ask. */
  loiNho: string;
};

// `i` with `u` folds case the Unicode way, so «Đ» meets «đ» and «Ủ» meets «ủ».
const MAU_NHAC = /@(rủ\s+đi|ru\s+di|rudi)/giu;
const MAU_LENH = /^\s*\/(plan|chia-?bill)(?=$|\s)/iu;

/** A letter or a digit in any script (a letter is anything with case). */
function laChuHoacSo(ch: string | undefined): boolean {
  if (ch === undefined) return false;
  if (ch >= "0" && ch <= "9") return true;
  return ch.toLowerCase() !== ch.toUpperCase();
}

/**
 * Where each real mention sits in `s` (already NFC), as [start, end).
 *
 * Before the `@`: nothing that can end an email's local part. After the name:
 * nothing that continues a word («@rudivn») or a domain («@rudi.vn»); a full
 * stop or a comma that ends the sentence is fine.
 */
function viTriNhac(s: string): Array<[number, number]> {
  const out: Array<[number, number]> = [];
  for (const m of s.matchAll(MAU_NHAC)) {
    const dau = m.index ?? 0;
    const cuoi = dau + m[0].length;
    const truoc = s[dau - 1];
    if (truoc !== undefined && (laChuHoacSo(truoc) || "._+-".includes(truoc))) continue;
    const sau = s[cuoi];
    if (laChuHoacSo(sau) || sau === "_") continue;
    if ((sau === "." || sau === "-") && laChuHoacSo(s[cuoi + 1])) continue;
    out.push([dau, cuoi]);
  }
  return out;
}

/**
 * The request in a message that calls on the AI: the leading command and every
 * mention taken out, the spacing and the punctuation they leave behind tidied.
 * An empty request gets the command's plain default, because the server
 * refuses an empty prompt and a bare `@Rủ Đi` still means «help us».
 */
export function tachLoiNho(body: string): string {
  let s = body.normalize("NFC");
  const lenh = MAU_LENH.exec(s);
  const chiaBill = lenh !== null && /^chia/i.test(lenh[1]);
  if (lenh) s = s.slice(lenh[0].length);
  const cat = viTriNhac(s);
  for (let i = cat.length - 1; i >= 0; i--) s = s.slice(0, cat[i][0]) + " " + s.slice(cat[i][1]);
  const gon = s.replace(/\s+/g, " ").replace(/\s+([,.!?;:])/g, "$1").replace(/^[\s,:;.!?-]+/, "").trim();
  if (gon !== "") return gon;
  return chiaBill ? LOI_NHO_CHIA_BILL : LOI_NHO_PLAN;
}

/**
 * Whether a message also asks the AI, and for what. Null for an ordinary
 * message, which is sent and nothing else.
 *
 * `/chia-bill` is the only command that asks for something other than a
 * plan; a mention and `/plan` are both `plan` until the engine gains `hoi`.
 */
export function timNhacAi(text: string): NhacAi | null {
  const s = text.normalize("NFC");
  const lenh = MAU_LENH.exec(s);
  if (lenh) return { lenh: /^chia/i.test(lenh[1]) ? "chia_bill" : "plan", loiNho: tachLoiNho(s) };
  if (viTriNhac(s).length > 0) return { lenh: "plan", loiNho: tachLoiNho(s) };
  return null;
}
