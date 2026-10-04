/** Personal memory books. Only the previewed bundle reaches the inference API. */
import { ApiError, BASE_URL, newAttempt, translatedAsActor } from "../../api";
import { laTuChoiVinhVien } from "../../cau-loi-theo-ma";
import { headerNguoiGoi } from "../../danh-tinh";

export type DiaryKind = "moment" | "trip";
export type DiaryPhoto = { id: string; url?: string; caption: string; day: string };
export type DiarySource = { title: string; starts_on: string; ends_on: string; kind: DiaryKind; photos: DiaryPhoto[]; places: string[]; excerpts: string[] };
export type DiaryPage = { layout: "photo" | "collage" | "note"; heading: string; text: string; photo_ids: string[] };
export type DiaryDocument = { title: string; subtitle: string; cover_id: string; pages: DiaryPage[]; ai_generated: boolean };
export type Diary = { id: string; outing_id: string; owner_id: string; kind: DiaryKind; audience: "private" | "public"; revision: number; document: DiaryDocument; created_at: string; updated_at: string };
export type Ending = { title: string; starts_on: string; ends_on: string; kind: DiaryKind; ended_at: string | null; can_end: boolean; diary_id: string | null };
export type DiaryJob = { id: string; status: "queued" | "running" | "succeeded" | "failed"; code: string | null; result: DiaryDocument | null };
const ERRORS: Record<string, string> = {
  diary_unavailable: "Rủ Đi đang gặp sự cố. Bạn thử lại sau một chút nhé.",
  outing_not_started: "Cuộc đi còn ở phía trước. Mình giữ trang cuối cho hôm trở về nhé.",
  outing_not_ended: "Người tổ chức chưa khép cuộc đi. Bạn quay lại sau nhé.",
  organizer_required: "Người tổ chức sẽ khép cuộc đi cho cả hội.",
  ending_already_confirmed: "Cuộc đi vừa được khép ở máy khác. Mở lại để xem nhé.",
  diary_revision_conflict: "Sổ vừa được sửa ở máy khác. Bản bạn đang viết vẫn ở đây; mở bản đã lưu để đối chiếu.",
  diary_source_changed: "Lịch trình đã đổi. Mở lại phần chất liệu trước khi dựng sổ nhé.",
  diary_rate_limited: "Mình đã dựng khá nhiều bản trong giờ này. Bạn vẫn có thể tự xếp trang, hoặc thử AI sau.",
  diary_manual_rate_limited: "Đã có nhiều lần xếp trang trong giờ này. Bản đang sửa vẫn ở đây; bạn quay lại dựng mới sau nhé.",
  diary_photo_not_found: "Có ảnh không còn dùng được. Bỏ ảnh đó rồi lưu lại nhé.",
  diary_not_found: "Cuốn sổ này đang được giữ riêng, hoặc đã được cất đi.",
  diary_job_not_found: "Lần dựng này đã hết hạn. Chọn lại chất liệu để dựng một bản mới nhé.",
};
export function readEnding(person: string, outing: string) {
  return translatedAsActor<Ending>(ERRORS, `/outings/${outing}/ending`, { actorId: person, method: "GET" });
}
export function endOuting(person: string, outing: string, kind: DiaryKind) {
  return translatedAsActor<Ending>(ERRORS, `/outings/${outing}/ending`, { actorId: person, method: "POST", body: { kind }, attempt: newAttempt() });
}
export function readSources(person: string, outing: string) {
  return translatedAsActor<DiarySource>(ERRORS, `/outings/${outing}/diary-sources`, { actorId: person, method: "GET" });
}
export function buildDiary(person: string, outing: string, source: DiarySource, useAI: boolean, logicalId: string) {
  return translatedAsActor<DiaryJob>(ERRORS, `/outings/${outing}/diary-jobs`, { actorId: person, method: "POST", body: { logical_id: logicalId, confirmed: true, use_ai: useAI, source }, attempt: newAttempt() });
}
export function readJob(person: string, job: string) {
  return translatedAsActor<DiaryJob>(ERRORS, `/diary-jobs/${job}`, { actorId: person, method: "GET" });
}
export function readDiary(person: string, id: string) {
  return translatedAsActor<Diary>(ERRORS, `/diaries/${id}`, { actorId: person, method: "GET" });
}
export function saveDiary(person: string, outing: string, document: DiaryDocument, revision: number, audience: Diary["audience"]) {
  return translatedAsActor<Diary>(ERRORS, `/outings/${outing}/diary`, { actorId: person, method: "PUT", body: { document, revision, audience }, attempt: newAttempt() });
}
export function removeDiary(person: string, id: string) {
  return translatedAsActor<void>(ERRORS, `/diaries/${id}`, { actorId: person, method: "DELETE", attempt: newAttempt() });
}
export function listDiaries(person: string, owner: string, before = "") {
  return translatedAsActor<{ diaries: Diary[]; next_cursor: string | null }>(ERRORS, `/people/${owner}/diaries?before=${before}`, { actorId: person, method: "GET" });
}
export function diaryImage(person: string, path: string) { return { uri: BASE_URL + path, headers: headerNguoiGoi(person), cacheKey: undefined }; }
export const publishedPhoto = (id: string, photo: string) => `/diaries/${id}/photos/${photo}`;

/** Defaults are suggestions only. Out-of-period photographs remain available. */
export function initialPhotos(source: DiarySource): string[] { return source.photos.filter((p) => p.day >= source.starts_on && p.day <= source.ends_on).slice(0, 40).map((p) => p.id); }
export function selectedBundle(source: DiarySource, ids: readonly string[], excerpts: string[]): DiarySource {
  return { ...source, photos: source.photos.filter((p) => ids.includes(p.id)), excerpts: [...excerpts] };
}
export function movePage(document: DiaryDocument, from: number, to: number): DiaryDocument {
  if (from < 0 || to < 0 || from >= document.pages.length || to >= document.pages.length) return document;
  const pages = [...document.pages]; const [page] = pages.splice(from, 1); pages.splice(to, 0, page); return { ...document, pages };
}

export const privatizeDiary = (person: string, diary: Diary) => translatedAsActor<Diary>(ERRORS, `/diaries/${diary.id}/audience`, { actorId: person, method: "PATCH", attempt: newAttempt(), body: { revision: diary.revision, audience: "private" } });

/** Apply a bounded selection without ever dropping an existing choice silently. */
export function togglePhoto(ids: readonly string[], id: string, limit: number): string[] {
  if (ids.includes(id)) return ids.filter((item) => item !== id);
  if (ids.length >= limit) return [...ids];
  return [...ids, id];
}

/** Reopening an edition keeps explicitly imported photos available to its editor. */
export function includeSavedPhotos(source: DiarySource, saved: Diary): DiarySource {
  const photos = [...source.photos];
  const known = new Set(photos.map((p) => p.id));
  const ids = [saved.document.cover_id, ...saved.document.pages.flatMap((p) => p.photo_ids)];
  for (const id of ids) {
    if (!id || known.has(id)) continue;
    known.add(id);
    photos.push({ id, url: publishedPhoto(saved.id, id), caption: "", day: source.ends_on });
  }
  return { ...source, photos };
}

/** A waiting message is presentation, never an authorization check. */
export function endingWait(ending: Ending, now: Date = new Date()): "future" | "organizer" | null {
  if (ending.ended_at || ending.can_end) return null;
  const vietnamDay = new Date(now.getTime() + 7 * 60 * 60 * 1000).toISOString().slice(0, 10);
  return ending.starts_on > vietnamDay ? "future" : "organizer";
}

/** Only transient failures offer the same request again; a conflict needs review. */
export function diaryFailure(error: unknown): { message: string; retryable: boolean; code: string | null } {
  const api = error instanceof ApiError ? error : null;
  const blocked = new Set(["outing_not_started", "outing_not_ended", "organizer_required", "ending_already_confirmed", "diary_revision_conflict", "diary_source_changed", "diary_photo_not_found"]);
  return {
    message: error instanceof Error ? error.message : "Chưa giữ được trang này. Bạn thử lại nhé.",
    retryable: !api || (!blocked.has(api.code.toLowerCase()) && !laTuChoiVinhVien(api.status, api.code)),
    code: api?.code.toLowerCase() ?? null,
  };
}
