/**
 * 24-hour stories (L4, ADR-0022 §2.3): the four routes and the pure copy.
 *
 * The server decides everything that matters -- who sees a story, whether it
 * is still live, whether THIS reader has seen it, and the order of the rail.
 * This module carries those answers to the screens and sends the reader's own
 * writes; the only arithmetic it does is the viewer's clock (five seconds per
 * story) and the strict «still live» check the viewer uses to stop showing a
 * story that expired while it was open.
 */
import { type Attempt, translatedAsActor } from "../../api";

export type StoryWire = {
  id: string;
  author_id: string;
  author_display_name: string;
  image_url: string;
  caption: string | null;
  audience: "friends";
  created_at: string;
  expires_at: string;
  /** Whether THIS reader has seen it. Server-decided. */
  seen: boolean;
};

export type NhomStoryWire = {
  author: { id: string; display_name: string };
  stories: StoryWire[];
  all_seen: boolean;
};

export type DaiStoryWire = { authors: NhomStoryWire[] };
export type DaXemWire = { story_id: string; seen_at: string };

/** How long one story stays on screen before the viewer moves on. */
export const THOI_LUONG_MS = 5000;

export const LOI_STORY: Record<string, string> = {
  story_not_found: "Story này không còn, hoặc không dành cho bạn.",
  photo_not_found: "Ảnh chưa gửi lên được. Chọn lại ảnh nhé.",
  photo_url_invalid: "Ảnh này không phải ảnh cá nhân của Rủ Đi.",
  permission_denied: "Chỉ đăng được ảnh của chính mình, và chỉ bạn mới xoá được story của mình.",
  caption_too_long: "Chú thích dài quá 200 ký tự.",
};

export async function docStories(actorId: string): Promise<DaiStoryWire> {
  return translatedAsActor<DaiStoryWire>(LOI_STORY, "/stories", { method: "GET", actorId });
}

/** The body carries `caption` only when there is one; an empty box sends none. */
export function thanDangStory(imageUrl: string, caption: string): { image_url: string; caption?: string } {
  const gon = caption.trim();
  return gon === "" ? { image_url: imageUrl } : { image_url: imageUrl, caption: gon };
}

export async function dangStory(imageUrl: string, caption: string, actorId: string, attempt: Attempt): Promise<StoryWire> {
  return translatedAsActor<StoryWire>(LOI_STORY, "/stories", {
    method: "POST",
    body: thanDangStory(imageUrl, caption),
    actorId,
    attempt,
  });
}

export async function danhDauDaXem(storyId: string, actorId: string): Promise<DaXemWire> {
  return translatedAsActor<DaXemWire>(LOI_STORY, `/stories/${storyId}/seen`, { method: "POST", actorId });
}

export async function xoaStory(storyId: string, actorId: string): Promise<void> {
  await translatedAsActor<void>(LOI_STORY, `/stories/${storyId}`, { method: "DELETE", actorId });
}

export function laCuaToi(nhom: Pick<NhomStoryWire, "author">, toi: string): boolean {
  return nhom.author.id === toi;
}

/** The rail's label for one author's ring, the way a screen reader says it. */
export function nhanVong(nhom: Pick<NhomStoryWire, "author" | "all_seen">, toi: string): string {
  if (laCuaToi(nhom, toi)) return "Story của bạn";
  return `Story của ${nhom.author.display_name}, ${nhom.all_seen ? "đã xem" : "chưa xem"}`;
}

/** Strict, like the server: at the deadline the story is gone. */
export function conHan(expiresAt: string, nowMs: number): boolean {
  const han = Date.parse(expiresAt);
  return Number.isFinite(han) && han > nowMs;
}

/** 0..1 of one story's five seconds, clamped. */
export function tienDo(batDauMs: number, nowMs: number): number {
  const phan = (nowMs - batDauMs) / THOI_LUONG_MS;
  if (!Number.isFinite(phan) || phan < 0) return 0;
  return phan > 1 ? 1 : phan;
}

/** Where the viewer opens: the first story this reader has not seen, else the first. */
export function chiSoBatDau(nhom: Pick<NhomStoryWire, "stories">): number {
  const i = nhom.stories.findIndex((s) => !s.seen);
  return i < 0 ? 0 : i;
}

export function nhomCua(dai: DaiStoryWire, authorId: string): NhomStoryWire | null {
  return dai.authors.find((n) => n.author.id === authorId) ?? null;
}

/** «2 giờ trước», «5 phút trước», «vừa xong»: the provenance line under the name. */
export function cauTuoi(createdAt: string, nowMs: number): string {
  const luc = Date.parse(createdAt);
  if (!Number.isFinite(luc)) return "";
  const phut = Math.max(0, Math.floor((nowMs - luc) / 60000));
  if (phut < 1) return "vừa xong";
  if (phut < 60) return `${phut} phút trước`;
  return `${Math.floor(phut / 60)} giờ trước`;
}
