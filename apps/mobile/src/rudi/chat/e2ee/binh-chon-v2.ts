/** Polls interpreted only after MLS authenticated and decrypted the message. */

/** Display state from this device's sealed log, distinct from legacy API receipts. */
export type KetQuaPhieuV2 = {
  id: string; created_by_id: string; question: string; is_closed: boolean; closed_at: string | null;
  options: { id: string; label: string; ballot_count: number }[];
  total_ballots: number; leading_option_ids: string[]; is_tie: boolean; decided_option_id: string | null;
  my_option_id: string | null; revision: number; deleted: false;
};

export type BinhChonV2 = {
  question: string;
  options: { id: string; label: string }[];
  ballots: Record<string, string>;
  closed: boolean;
  closedAt: string | null;
  revision: number;
};

/** The existing /vote text remains readable by clients without a poll renderer. */
export function docBinhChonV2(body: string, id: string, sequence: number): BinhChonV2 | null {
  if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(id) || body.length > 900) return null;
  const match = /^\/vote[ \t]+([^?\n]+)\?[ \t]*(.+)$/.exec(body);
  if (!match) return null;
  const question = match[1].trim();
  const labels = match[2].split("|").map((label) => label.trim());
  if (!question || question.length > 180 || question.includes("|") || labels.length < 2 || labels.length > 6 ||
      labels.some((label) => !label || label.length > 100) || new Set(labels.map((label) => label.toLowerCase())).size !== labels.length) return null;
  return {
    question,
    // Option IDs are scoped by this poll ID. Derivation uses no random state,
    // so all members and a restarted device agree without a server poll table.
    options: labels.map((label, index) => ({ id: id.slice(0, -1) + (index + 1), label })),
    ballots: Object.create(null) as Record<string, string>, closed: false, closedAt: null, revision: sequence,
  };
}

/** Closing is an authenticated author edit, with no new crypto wire operation. */
export function chuDongBinhChon(body: string): string {
  return body + "\nBình chọn đã đóng.";
}

/** A view of the device's sealed record, never a response from /votes. */
export function xemBinhChonV2(t: { id: string; authorId: string; poll?: BinhChonV2; deleted: boolean }, personId: string): KetQuaPhieuV2 | null {
  const poll = t.poll;
  if (!poll || t.deleted) return null;
  const counts = new Map(poll.options.map((option) => [option.id, 0]));
  for (const option of Object.values(poll.ballots)) if (counts.has(option)) counts.set(option, counts.get(option)! + 1);
  const total = [...counts.values()].reduce((sum, count) => sum + count, 0);
  const highest = Math.max(0, ...counts.values());
  const leading = highest === 0 ? [] : poll.options.filter((option) => counts.get(option.id) === highest).map((option) => option.id);
  return {
    id: t.id, created_by_id: t.authorId, question: poll.question, closed_at: poll.closedAt,
    is_closed: poll.closed, options: poll.options.map((option) => ({ ...option, ballot_count: counts.get(option.id)! })),
    total_ballots: total, leading_option_ids: leading, is_tie: leading.length > 1,
    decided_option_id: poll.closed && leading.length === 1 ? leading[0] : null,
    my_option_id: Object.hasOwn(poll.ballots, personId) ? poll.ballots[personId] : null,
    revision: poll.revision, deleted: false,
  };
}
