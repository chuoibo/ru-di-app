/**
 * The invitation written as one sentence with blanks (plan S0.5, `CauRu`):
 * «Rủ {nhom} đi {ten} từ {tu} tới {den}». Pure: a template becomes words and
 * blanks, so the words wrap one at a time like any sentence and every blank is
 * a separate slot at least a finger wide.
 *
 * A blank keeps the punctuation that follows it («{den},» stays one piece: a
 * comma never starts a line), and a blank the template does not know is left
 * as literal text rather than silently dropped.
 */
export type PhanCau = { kieu: "chu"; chu: string } | { kieu: "o"; ten: string; sau: string };

export function tachCau(mau: string, oBiet: readonly string[]): PhanCau[] {
  return mau
    .trim()
    .split(/\s+/)
    .filter((tu) => tu !== "")
    .map((tu): PhanCau => {
      const m = /^\{([a-zA-Z0-9-]+)\}([,.;:!?]*)$/.exec(tu);
      if (m && oBiet.includes(m[1])) return { kieu: "o", ten: m[1], sau: m[2] };
      return { kieu: "chu", chu: tu };
    });
}
