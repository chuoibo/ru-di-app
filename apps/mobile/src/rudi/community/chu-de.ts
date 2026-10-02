/**
 * The composer's topics, checked as the server checks them (QA UI-133).
 *
 * The server refuses more than five topics (`too_many_topics`) and a topic
 * shorter than two or longer than forty characters, or one holding / \ < > @
 * (`invalid_topic`; `services/core/internal/community/posts.go`
 * `normalizeTopics`). The composer used to learn that only after «Gửi», and
 * then said «lỗi của app» under the bottom edge. Checked here, while typing,
 * the box says what to fix next to itself. Pure, so the rules are tested
 * against the server's.
 */

/** The server's ceiling. */
export const TOI_DA_CHU_DE = 5;

/** The topics as typed: comma separated, trimmed, empty pieces dropped. */
export function tachChuDe(go: string): string[] {
  return go.split(",").map((s) => s.trim()).filter((s) => s !== "");
}

/** One topic as the server normalises it: no leading «#», lower case, single spaces. */
export function chuanChuDe(s: string): string {
  return s.trim().replace(/^#/, "").trim().toLocaleLowerCase("vi").normalize("NFC").split(/\s+/).filter(Boolean).join(" ");
}

/**
 * What is wrong with the topics, in the words the box shows, or null when the
 * server will accept them. The first problem only: one sentence at a time.
 */
export function loiChuDe(go: string): string | null {
  const ds = tachChuDe(go);
  if (ds.length > TOI_DA_CHU_DE) return `Tối đa ${TOI_DA_CHU_DE} chủ đề. Bỏ bớt ${ds.length - TOI_DA_CHU_DE} chủ đề nhé.`;
  for (const g of ds) {
    const s = chuanChuDe(g);
    const n = [...s].length;
    if (n < 2) return `Chủ đề «${g}» ngắn quá: cần ít nhất 2 ký tự.`;
    if (n > 40) return `Chủ đề «${[...g].slice(0, 16).join("")}…» dài quá: tối đa 40 ký tự.`;
    if (/[/\\<>@]/.test(s)) return `Chủ đề «${g}» có ký tự / \\ < > hoặc @: bỏ ký tự đó đi.`;
  }
  return null;
}
