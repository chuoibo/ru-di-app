/**
 * A person's ink: the one colour that says «this is Minh Anh» wherever she
 * appears -- her avatar ring and initial, her seat at the bill table, her vote
 * dot, her name above a chat bubble (ADR-0037 D6, «Luật Mực Người»).
 *
 * Before this module every avatar took the screen's tone, on purpose: colour
 * meant «money» or «AI», never a person. That kept meaning clean and made a
 * group of eight read as eight identical coral circles, two of them «T». The
 * inks live in their own `tokens.json` section so they can never be confused
 * with the three meaning tones: each is at least 30 degrees of OKLCH hue away
 * from accent, split, ai and the brand coral/teal/violet, and each clears 4.5:1
 * on paper, card and ground of its own scheme (`tests/muc-nguoi.test.mjs`).
 *
 * The index is FNV-1a of the person id, so the same person has the same ink on
 * every screen and every device without storing anything. Pure: no React, no
 * React Native, so node tests import it as it ships.
 */
import tokens from "../../../../../packages/shared/tokens.json";

export const SO_MUC_NGUOI = 8;

export const BANG_MUC_NGUOI: Readonly<{ light: readonly string[]; dark: readonly string[] }> = Object.freeze({
  light: Object.freeze([...tokens.mucNguoi.light]),
  dark: Object.freeze([...tokens.mucNguoi.dark]),
});

/** 32-bit FNV-1a over the UTF-16 code units of `s`, as an unsigned integer. */
export function bamFnv1a(s: string): number {
  let h = 0x811c9dc5;
  for (let i = 0; i < s.length; i += 1) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return h >>> 0;
}

/**
 * The ink slot of a person. Ids are compared lowercase: the server's UUIDs are
 * canonical lowercase already, and a stray uppercase copy must not change the
 * colour someone is recognised by.
 */
export function chiSoMuc(personId: string): number {
  return bamFnv1a(personId.trim().toLowerCase()) % SO_MUC_NGUOI;
}

/** The ink itself, for the scheme on screen. */
export function mucNguoi(personId: string, dark: boolean): string {
  const bang = dark ? BANG_MUC_NGUOI.dark : BANG_MUC_NGUOI.light;
  return bang[chiSoMuc(personId)];
}
