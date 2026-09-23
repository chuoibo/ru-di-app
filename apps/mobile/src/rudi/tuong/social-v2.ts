import { newAttempt, translatedAsActor, type Attempt, type PostAudience } from "../../api";

const LOI_XA_HOI: Record<string, string> = {
  post_not_found: "Bài này không còn hoặc không dành cho bạn.",
  comment_not_found: "Bình luận này không còn.",
  comments_closed: "Chủ tường không cho bình luận bài này.",
  reply_depth_exceeded: "Chỉ trả lời trực tiếp một bình luận gốc.",
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
export type TrangBinhLuanTuong = { post_id: string; comments: BinhLuanTuong[]; next_cursor: string | null; has_more: boolean };

export function ghepTrangTuong<T extends { id: string }>(oldPosts: T[], newPosts: T[]): T[] {
  const known = new Set(oldPosts.map((post) => post.id));
  return [...oldPosts, ...newPosts.filter((post) => !known.has(post.id))];
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

export async function guiTraLoi(postId: string, parentId: string | null, body: string, actorId: string, attempt: Attempt): Promise<BinhLuanTuong> {
  const content = parentId ? { body: body.trim(), parent_id: parentId } : { body: body.trim() };
  return translatedAsActor<BinhLuanTuong>(LOI_XA_HOI, `/social/v2/posts/${postId}/comments`, { method: "POST", actorId, attempt, body: content });
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
