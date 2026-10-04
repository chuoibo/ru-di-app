import { newAttempt, translatedAsActor, type Attempt, type PostAudience } from "../../api";

const LOI_XA_HOI: Record<string, string> = {
  post_not_found: "Bài này không còn hoặc không dành cho bạn.",
  comment_not_found: "Bình luận này không còn.",
  comments_closed: "Chủ tường không cho bình luận bài này.",
  invalid_parent: "Chỉ trả lời trực tiếp một bình luận gốc.",
  invalid_comment: "Bình luận cần có chữ và dưới 2.000 ký tự.",
  invalid_body: "Bình luận cần có chữ và dưới 2.000 ký tự.",
  community_unavailable: "Bài đã qua cộng đồng đang tạm không nhận bình luận. Thử lại sau nhé.",
  public_repost_needs_review: "Chia sẻ cho mọi người cần qua duyệt cộng đồng. Chọn bạn bè hoặc chỉ mình bạn.",
  rate_limited: "Bạn đang gửi hơi nhanh. Đợi một chút rồi thử lại.",
  invalid_cursor: "Không đọc tiếp được. Tải lại tường nhé.",
  social_temporarily_unavailable: "Tường đang tạm gián đoạn. Kéo xuống để thử lại.",
};

export type BaiTuong = {
  id: string;
  author_id: string;
  author_display_name: string;
  audience: PostAudience;
  context_id: string | null;
  body: string;
  image_url: string | null;
  created_at: string;
  like_count: number;
  liked: boolean;
  comment_count: number;
  can_comment: boolean;
  is_repost?: boolean;
  origin: null | Pick<BaiTuong, "id" | "author_id" | "author_display_name" | "body" | "image_url" | "created_at">;
};

export type TrangTuong = { person_id: string; posts: BaiTuong[]; next_cursor: string | null; has_more: boolean };
export type DoiTuong = { events: { id: string; post_id: string; kind: "post" | "comment" | "like" | "repost" }[]; next_cursor: string | null; has_more: boolean };
export type BinhLuanTuong = {
  id: string;
  post_id: string;
  parent_id: string | null;
  author_id: string;
  author_display_name: string;
  body: string;
  created_at: string;
  like_count: number;
  liked: boolean;
  replies: BinhLuanTuong[];
};
/** The reader's own comment waiting for review; nobody else sees it (ADR-0040). */
export type BinhLuanChoDuyet = {
  id: string;
  post_id: string;
  parent_id: string | null;
  body: string;
  status: "pending" | "review" | "rejected";
  created_at: string;
};
export type TrangBinhLuanTuong = {
  post_id: string;
  comments: BinhLuanTuong[];
  pending?: BinhLuanChoDuyet[];
  next_cursor: string | null;
  has_more: boolean;
};

/** A comment on a public post comes back as a draft with 202 until reviewed. */
export function dangChoDuyet(value: BinhLuanTuong | BinhLuanChoDuyet): value is BinhLuanChoDuyet {
  return "status" in value && value.status !== undefined;
}

export function ghepTrangTuong<T extends { id: string }>(oldPosts: T[], newPosts: T[]): T[] {
  const known = new Set(oldPosts.map((post) => post.id));
  return [...oldPosts, ...newPosts.filter((post) => !known.has(post.id))];
}

/**
 * A background refresh of a wall the reader may have paged through (QA
 * UI-155): every long-poll answer used to replace the list with the first
 * page, so the pages opened with «Xem những trang trước» vanished under the
 * reader's hand within twenty seconds.
 *
 * The fresh first page leads (new posts, edited ones, counts). What the
 * reader had below it stays, in its order, with the cursor that reaches past
 * it. A post that sat inside the old first page's window and is missing from
 * the new one is gone (deleted, or no longer shown to this reader); a post
 * further down is left alone, since no page read now says anything about it.
 */
export function lamMoiDauTuong<T extends { id: string }>(
  hienCo: { bai: T[]; conTro: string | null; conNua: boolean },
  dau: { posts: T[]; next_cursor: string | null; has_more: boolean },
): { bai: T[]; conTro: string | null; conNua: boolean } {
  const trongDau = new Set(dau.posts.map((b) => b.id));
  const moc = dau.posts.at(-1)?.id;
  const viTriMoc = moc === undefined ? -1 : hienCo.bai.findIndex((b) => b.id === moc);
  // The new page's last post is one the reader had: everything above it in
  // the old list was covered by this read. Not found (the page is all new, or
  // empty): nothing in the old list was covered past what the page repeats.
  const conLai = (viTriMoc >= 0 ? hienCo.bai.slice(viTriMoc + 1) : dau.has_more ? hienCo.bai : []).filter((b) => !trongDau.has(b.id));
  if (conLai.length === 0) return { bai: dau.posts, conTro: dau.next_cursor, conNua: dau.has_more };
  return { bai: [...dau.posts, ...conLai], conTro: hienCo.conTro, conNua: hienCo.conNua };
}

export async function docTrangTuong(personId: string, actorId: string, cursor: string | null = null): Promise<TrangTuong> {
  const query = `limit=20${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`;
  return translatedAsActor<TrangTuong>(LOI_XA_HOI, `/social/v2/people/${personId}/posts?${query}`, { method: "GET", actorId });
}

export async function docBaiTuong(postId: string, actorId: string): Promise<BaiTuong> {
  return translatedAsActor<BaiTuong>(LOI_XA_HOI, `/social/v2/posts/${postId}`, { method: "GET", actorId });
}

export async function docDoiTuong(personId: string, actorId: string, cursor: string | null, wait = 20): Promise<DoiTuong> {
  const query = `${cursor ? `after=${encodeURIComponent(cursor)}&` : ""}wait=${wait}`;
  return translatedAsActor<DoiTuong>(LOI_XA_HOI, `/social/v2/people/${personId}/changes?${query}`, { method: "GET", actorId, timeoutMs: (wait + 5) * 1000 });
}

export async function docBinhLuanTuong(postId: string, actorId: string, cursor: string | null = null): Promise<TrangBinhLuanTuong> {
  const query = `limit=20${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`;
  return translatedAsActor<TrangBinhLuanTuong>(LOI_XA_HOI, `/social/v2/posts/${postId}/comments?${query}`, { method: "GET", actorId });
}

export async function guiTraLoi(postId: string, parentId: string | null, body: string, actorId: string, attempt: Attempt): Promise<BinhLuanTuong | BinhLuanChoDuyet> {
  const content = parentId ? { body: body.trim(), parent_id: parentId } : { body: body.trim() };
  return translatedAsActor<BinhLuanTuong | BinhLuanChoDuyet>(LOI_XA_HOI, `/social/v2/posts/${postId}/comments`, { method: "POST", actorId, attempt, body: content });
}

export async function thichBaiTuong(postId: string, actorId: string, shouldLike: boolean): Promise<{ post_id: string; liked: boolean; like_count: number }> {
  return translatedAsActor(LOI_XA_HOI, `/social/v2/posts/${postId}/like`, { method: shouldLike ? "PUT" : "DELETE", actorId, attempt: newAttempt() });
}

export async function thichBinhLuan(commentId: string, actorId: string, shouldLike: boolean): Promise<{ comment_id: string; liked: boolean; like_count: number }> {
  return translatedAsActor(LOI_XA_HOI, `/social/v2/comments/${commentId}/like`, { method: shouldLike ? "PUT" : "DELETE", actorId, attempt: newAttempt() });
}

export async function dangLaiBai(postId: string, audience: PostAudience, actorId: string, attempt: Attempt, contextId?: string): Promise<{ id: string; origin_id: string }> {
  const body = contextId ? { audience, context_id: contextId } : { audience };
  return translatedAsActor(LOI_XA_HOI, `/social/v2/posts/${postId}/repost`, { method: "POST", actorId, attempt, body });
}

/** Chat v2 sends this reference only after client-side E2EE enrollment succeeds. */
export type EncryptedPostReference = { kind: "post_reference"; post_id: string };
export function nhanBaiChoChatV2(postId: string): EncryptedPostReference {
  // TODO(chat-v2-E2EE): encrypt this payload in the recipient conversation;
  // the receiving client must re-check the post ACL before opening it.
  return { kind: "post_reference", post_id: postId };
}
