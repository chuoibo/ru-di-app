import { newAttempt, translatedAsActor } from "../../api";

/**
 * The shared sheet a group edits together between deciding and committing.
 *
 * It is the object the whole "cùng chọn → cùng sửa → chốt" chain was missing:
 * a poll decided something, and until now there was nowhere for that decision
 * to land except one person's private form. The sheet is anchored to a real
 * message, so every edit arrives on everyone's screen through the change feed
 * they already listen on.
 *
 * `revision` is not bookkeeping. Sending it back is how the server can tell an
 * edit from an overwrite: two people typing at once is the normal case, and
 * the one who started from an older copy is told so (409) instead of silently
 * erasing what the other just decided.
 */
export type ChangToHen = {
  time_text: string;
  label: string;
  place_id?: string;
};

export type ToHenChung = {
  id: string;
  context_id: string;
  message_id: string;
  source_vote_id: string | null;
  revision: number;
  title: string;
  starts_on: string | null;
  ends_on: string | null;
  headcount: number | null;
  budget_per_person_vnd: number | null;
  stops: ChangToHen[];
  status: "open" | "promoted" | "discarded";
  created_by: string;
};

export type BanNhapToHen = {
  title: string;
  starts_on?: string | null;
  ends_on?: string | null;
  headcount?: number | null;
  budget_per_person_vnd?: number | null;
  stops: ChangToHen[];
  from_vote_id?: string;
};

const options = (contextId: string, personId: string) => ({
  actorId: personId,
  contexts: contextId,
  timeoutMs: 15000,
});

export function moToHenChung(contextId: string, personId: string, draft: BanNhapToHen) {
  return translatedAsActor<ToHenChung>({}, `/contexts/${contextId}/shared-drafts`, {
    ...options(contextId, personId),
    method: "POST",
    body: draft,
    attempt: newAttempt(),
  });
}

export function docToHenChung(contextId: string, personId: string, id: string) {
  return translatedAsActor<ToHenChung>({}, `/contexts/${contextId}/shared-drafts/${id}`, {
    ...options(contextId, personId),
    method: "GET",
  });
}

/** `revision` is the copy the editor was looking at, never the one we want. */
export function suaToHenChung(
  contextId: string,
  personId: string,
  id: string,
  revision: number,
  patch: Partial<BanNhapToHen>,
) {
  return translatedAsActor<ToHenChung>({}, `/contexts/${contextId}/shared-drafts/${id}`, {
    ...options(contextId, personId),
    method: "PATCH",
    body: { ...patch, revision },
    attempt: newAttempt(),
  });
}

export function boToHenChung(contextId: string, personId: string, id: string) {
  return translatedAsActor<ToHenChung>({}, `/contexts/${contextId}/shared-drafts/${id}/discard`, {
    ...options(contextId, personId),
    method: "POST",
    attempt: newAttempt(),
  });
}

/**
 * The draft block a shared sheet carries inside its itinerary card. An AI card
 * has no such block, which is exactly how a screen tells the two apart: one
 * can still be edited by the group, the other is a suggestion to accept.
 */
export type KhoiNhapTrongThe = {
  id: string;
  revision: number;
  status: ToHenChung["status"];
  source_vote_id: string | null;
  starts_on: string | null;
  ends_on: string | null;
  headcount: number | null;
  budget_per_person_vnd: number | null;
};

export function docKhoiNhap(card: unknown): KhoiNhapTrongThe | null {
  if (!card || typeof card !== "object") return null;
  const payload = (card as { payload?: unknown }).payload;
  if (!payload || typeof payload !== "object") return null;
  const draft = (payload as { draft?: unknown }).draft;
  if (!draft || typeof draft !== "object") return null;
  const row = draft as Record<string, unknown>;
  if (typeof row.id !== "string" || typeof row.revision !== "number") return null;
  const status = row.status;
  if (status !== "open" && status !== "promoted" && status !== "discarded") return null;
  return {
    id: row.id,
    revision: row.revision,
    status,
    source_vote_id: typeof row.source_vote_id === "string" ? row.source_vote_id : null,
    starts_on: typeof row.starts_on === "string" ? row.starts_on : null,
    ends_on: typeof row.ends_on === "string" ? row.ends_on : null,
    headcount: typeof row.headcount === "number" ? row.headcount : null,
    budget_per_person_vnd:
      typeof row.budget_per_person_vnd === "number" ? row.budget_per_person_vnd : null,
  };
}

/**
 * Vietnamese reads a date day first. The wire stays ISO because the server and
 * every comparison depend on it; only what a person reads changes.
 */
export function ngayKieuViet(iso: string | null | undefined): string {
  if (!iso) return "";
  const parts = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  if (!parts) return iso;
  return `${parts[3]}/${parts[2]}/${parts[1]}`;
}

/** The inverse, for a field a person typed into. Returns null when incomplete. */
export function ngayVeISO(text: string): string | null {
  const parts = /^(\d{1,2})\/(\d{1,2})\/(\d{4})$/.exec(text.trim());
  if (!parts) return null;
  const day = parts[1].padStart(2, "0");
  const month = parts[2].padStart(2, "0");
  const iso = `${parts[3]}-${month}-${day}`;
  const parsed = new Date(`${iso}T00:00:00Z`);
  if (Number.isNaN(parsed.getTime())) return null;
  // Reject a date the calendar does not have, such as 31/02.
  if (parsed.toISOString().slice(0, 10) !== iso) return null;
  return iso;
}
