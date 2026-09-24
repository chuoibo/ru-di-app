/**
 * Which context a deep link may open a screen in.
 *
 * A pair is never the current group (ADR-0021 §2.5), so the plan a couple
 * agreed on, its bill and its ledger are opened with an explicit `?ctx=` /
 * `/settlements/{id}`. The link is a string anybody can type: the screen runs
 * in that context only when the person is an active member of it; otherwise
 * it is not honoured (`null`), and the caller falls back or redirects. The
 * server still refuses a context the person is not in -- this keeps the phone
 * from rendering somebody else's id as if it were theirs.
 */
export interface PhienNguCanh {
  context_id: string | null;
  contexts?: readonly { id: string; my_state: string }[] | null;
}

export type NguCanhMo = { kieu: "hien-tai" } | { kieu: "khac"; contextId: string } | { kieu: "tu-choi" };

export function nguCanhMo(phien: PhienNguCanh, ctx: unknown): NguCanhMo {
  if (typeof ctx !== "string" || ctx === "" || ctx === phien.context_id) return { kieu: "hien-tai" };
  return phien.contexts?.some((c) => c.id === ctx && c.my_state === "active") ? { kieu: "khac", contextId: ctx } : { kieu: "tu-choi" };
}
