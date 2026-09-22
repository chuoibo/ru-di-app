/** Session-memory drafts never write plaintext conversation content to disk. */
export type BanNhapCongCu = { question: string; choices: string[]; prompt: string };
const drafts = new Map<string, BanNhapCongCu>();
let owner: string | null = null;
const blank = (): BanNhapCongCu => ({ question: "", choices: ["", ""], prompt: "" });
const clone = (value: BanNhapCongCu): BanNhapCongCu => ({ ...value, choices: [...value.choices] });

/** Switching identities or signing out destroys the previous account's vault. */
export function datChuSoHuuBanNhap(personId: string | null): void {
  if (owner !== personId) drafts.clear();
  owner = personId;
}

export function docBanNhapCongCu(personId: string, contextId: string): BanNhapCongCu {
  return owner === personId ? clone(drafts.get(contextId) ?? blank()) : blank();
}

export function ghiBanNhapCongCu(personId: string, contextId: string, value: BanNhapCongCu): void {
  if (owner !== personId) return;
  if (!value.question && value.choices.every((choice) => !choice) && !value.prompt) drafts.delete(contextId);
  else drafts.set(contextId, clone(value));
}

/**
 * Per-field poll errors.
 *
 * One sentence under the whole form told a person something was wrong but not
 * where, so fixing it meant re-reading every box. Each message here names the
 * box it belongs to, and `loiBinhChon` below folds them back into the single
 * line the live region still announces.
 */
export type LoiBinhChonTheoO = {
  question: string | null;
  choices: (string | null)[];
};

export function loiBinhChonTheoO(question: string, choices: readonly string[]): LoiBinhChonTheoO {
  const out: LoiBinhChonTheoO = { question: null, choices: choices.map(() => null) };
  const trimmed = question.trim();
  if (!trimmed) out.question = "Cần một câu hỏi.";
  else if (trimmed.includes("|")) out.question = "Câu hỏi không chứa dấu |.";
  else if (trimmed.replace(/\?$/, "").includes("?")) out.question = "Đặt dấu hỏi ở cuối câu hỏi.";

  const cleaned = choices.map((choice) => choice.trim());
  const filled = cleaned.filter(Boolean);
  const seen = new Map<string, number>();
  cleaned.forEach((choice, index) => {
    if (!choice) return;
    if (choice.includes("|")) { out.choices[index] = "Lựa chọn không chứa dấu |."; return; }
    const key = choice.toLocaleLowerCase();
    const first = seen.get(key);
    if (first === undefined) seen.set(key, index);
    else out.choices[index] = "Trùng với lựa chọn " + (first + 1) + ".";
  });
  if (filled.length < 2) {
    // Point at the first empty box rather than at the form: that is the one
    // the person has to touch next.
    const blank = cleaned.findIndex((choice) => !choice);
    if (blank >= 0) out.choices[blank] = "Cần ít nhất hai lựa chọn.";
  }
  return out;
}

/** The one-line summary, derived from the per-field answer so the two agree. */
export function loiBinhChon(question: string, choices: readonly string[]): string | null {
  const cleaned = choices.map((choice) => choice.trim()).filter(Boolean);
  if (!question.trim() || cleaned.length < 2) return "Thêm câu hỏi và ít nhất hai lựa chọn.";
  if ([question, ...cleaned].some((value) => value.includes("|")) || new Set(cleaned.map((value) => value.toLocaleLowerCase())).size !== cleaned.length) {
    return "Mỗi lựa chọn cần khác nhau và không chứa dấu |.";
  }
  if (question.trim().replace(/\?$/, "").includes("?")) return "Đặt dấu hỏi ở cuối câu hỏi.";
  return null;
}
