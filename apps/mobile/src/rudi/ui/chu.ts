/**
 * Small typographic helpers for text the kit sets itself. Pure.
 */

/** A no-break space: two words joined by it wrap together or not at all. */
export const NBSP = " ";

/**
 * Join the last two words of a sentence with a no-break space, so a wrapped
 * line never ends with one short word alone («…rồi thử / lại.» -- re-audit
 * 10/09, ảnh 20). Only sentences of at least four words are touched; a
 * two-word label is not an orphan problem, and the caller's own no-break
 * spaces are kept.
 */
export function khongMoCoi(text: string): string {
  const tu = text.trim().split(" ");
  if (tu.length < 4) return text;
  const cuoi = tu.pop() as string;
  return `${tu.join(" ")}${NBSP}${cuoi}`;
}
