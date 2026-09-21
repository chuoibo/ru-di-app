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

export function loiBinhChon(question: string, choices: readonly string[]): string | null {
  const cleaned = choices.map((choice) => choice.trim()).filter(Boolean);
  if (!question.trim() || cleaned.length < 2) return "Thêm câu hỏi và ít nhất hai lựa chọn.";
  if ([question, ...cleaned].some((value) => value.includes("|")) || new Set(cleaned.map((value) => value.toLocaleLowerCase())).size !== cleaned.length) {
    return "Mỗi lựa chọn cần khác nhau và không chứa dấu |.";
  }
  if (question.trim().replace(/\?$/, "").includes("?")) return "Đặt dấu hỏi ở cuối câu hỏi.";
  return null;
}
