/**
 * One post and what sits under it (ADR-0022 §2.2).
 *
 * The server counts reactions and comments from its rows and decides
 * `can_comment` for THIS reader from the wall owner's policy; this module
 * carries those answers to the screen and sends the reader's own writes. It
 * never re-derives who may comment from the relation -- `coTheBinhLuan` reads
 * the flag and nothing else, so the screen and the server cannot disagree.
 */
import { type Attempt, translatedAsActor, type PostWire } from "../../api";

export type LoaiPhanUng = "heart" | "haha" | "like" | "wow" | "sad" | "fire";
export type DemPhanUng = { kind: LoaiPhanUng; count: number };

/** `GET /posts/{id}` after L3; the five new keys are absent on an older server. */
export type BaiWire = PostWire & {
  author_display_name?: string;
  reactions?: DemPhanUng[];
  my_reactions?: LoaiPhanUng[];
  comment_count?: number;
  can_comment?: boolean;
};

export type BinhLuanBai = {
  id: string;
  post_id: string;
  author_id: string;
  author_display_name: string;
  body: string;
  created_at: string;
};

export type TrangBinhLuan = {
  post_id: string;
  comments: BinhLuanBai[];
  next_cursor: string | null;
  has_more: boolean;
};

export type PhanUngBai = { post_id: string; reactions: DemPhanUng[]; my_reactions: LoaiPhanUng[] };

export const CAU_KHONG_BINH_LUAN = "Chủ tường không cho bình luận bài này.";

export const LOI_BAI: Record<string, string> = {
  post_not_found: "Bài này không có, hoặc không dành cho bạn.",
  comments_closed: CAU_KHONG_BINH_LUAN,
  comment_not_found: "Bình luận này không còn.",
  permission_denied: "Bạn chỉ xoá được bình luận của mình, hoặc bình luận trên bài của mình.",
  invalid_cursor: "Không đọc tiếp được. Mở lại bài nhé.",
};

export async function docBaiChiTiet(postId: string, actorId: string): Promise<BaiWire> {
  return translatedAsActor<BaiWire>(LOI_BAI, `/posts/${postId}`, { method: "GET", actorId });
}

export async function docBinhLuanBai(
  postId: string,
  actorId: string,
  after: string | null = null,
  limit = 50,
): Promise<TrangBinhLuan> {
  const truyVan = after === null ? `limit=${limit}` : `limit=${limit}&after=${encodeURIComponent(after)}`;
  return translatedAsActor<TrangBinhLuan>(LOI_BAI, `/posts/${postId}/comments?${truyVan}`, {
    method: "GET",
    actorId,
  });
}

export async function guiBinhLuanBai(
  postId: string,
  body: string,
  actorId: string,
  attempt: Attempt,
): Promise<BinhLuanBai> {
  return translatedAsActor<BinhLuanBai>(LOI_BAI, `/posts/${postId}/comments`, {
    method: "POST",
    body: { body: body.trim() },
    actorId,
    attempt,
  });
}

export async function xoaBinhLuanBai(
  postId: string,
  commentId: string,
  actorId: string,
  attempt: Attempt,
): Promise<void> {
  await translatedAsActor<void>(LOI_BAI, `/posts/${postId}/comments/${commentId}`, {
    method: "DELETE",
    actorId,
    attempt,
  });
}

export async function themPhanUngBai(
  postId: string,
  kind: LoaiPhanUng,
  actorId: string,
  attempt: Attempt,
): Promise<PhanUngBai> {
  return translatedAsActor<PhanUngBai>(LOI_BAI, `/posts/${postId}/reactions`, {
    method: "POST",
    body: { kind },
    actorId,
    attempt,
  });
}

export async function boPhanUngBai(
  postId: string,
  kind: LoaiPhanUng,
  actorId: string,
  attempt: Attempt,
): Promise<PhanUngBai> {
  return translatedAsActor<PhanUngBai>(LOI_BAI, `/posts/${postId}/reactions/${kind}`, {
    method: "DELETE",
    actorId,
    attempt,
  });
}

/* ------------------------------------------------------------ pure copy */

/** The server's answer, and only that. A post from an older server has no flag: closed. */
export function coTheBinhLuan(bai: Pick<BaiWire, "can_comment">): boolean {
  return bai.can_comment === true;
}

export function demLoai(bai: Pick<BaiWire, "reactions">, kind: LoaiPhanUng): number {
  const dem = (bai.reactions ?? []).find((r) => r.kind === kind);
  if (dem === undefined) return 0;
  return dem.count;
}

export function tongPhanUng(bai: Pick<BaiWire, "reactions">): number {
  return (bai.reactions ?? []).reduce((tong, r) => tong + r.count, 0);
}

export function daPhanUng(bai: Pick<BaiWire, "my_reactions">, kind: LoaiPhanUng): boolean {
  return (bai.my_reactions ?? []).includes(kind);
}

/** «1 tim · 2 bình luận», the way the memory wall says it. */
export function cauTuongTacBai(bai: Pick<BaiWire, "reactions" | "comment_count">): string {
  return `${demLoai(bai, "heart")} tim · ${bai.comment_count ?? 0} bình luận`;
}

/** The post's counts after a reaction write, without a second read. */
export function apPhanUng<T extends Pick<BaiWire, "reactions" | "my_reactions">>(bai: T, moi: PhanUngBai): T {
  return { ...bai, reactions: moi.reactions, my_reactions: moi.my_reactions };
}

export function coTheXoaBinhLuan(
  binhLuan: Pick<BinhLuanBai, "author_id">,
  bai: Pick<BaiWire, "author_id">,
  toi: string,
): boolean {
  return binhLuan.author_id === toi || bai.author_id === toi;
}

/** Older page + newer page, oldest first, no id twice. */
export function ghepBinhLuan(cu: BinhLuanBai[], moi: BinhLuanBai[]): BinhLuanBai[] {
  const daCo = new Set(cu.map((c) => c.id));
  return [...cu, ...moi.filter((c) => !daCo.has(c.id))];
}

export function boBinhLuan(danhSach: BinhLuanBai[], id: string): BinhLuanBai[] {
  return danhSach.filter((c) => c.id !== id);
}
