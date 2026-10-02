/**
 * The community feed's small decisions, pure so they are tested without a
 * screen (QA N14, B7 of the UI/UX upgrade).
 */
import type { Post } from "./api";

/**
 * Following is about a person, not a card: every card by that author takes
 * the new state at once (QA UI-140: one card said «Bỏ theo dõi», the next
 * card of the same author still said «Theo dõi»).
 */
export function doiTheoDoiTacGia(posts: readonly Post[], authorId: string, following: boolean): Post[] {
  return posts.map((p) => (p.author_id === authorId ? { ...p, following } : p));
}

/**
 * Whether a card's body is cut on the feed: the same test that prints
 * «Đọc tiếp». A body that already shows whole opens the post on the first
 * tap; only a cut one spends the first tap on showing the rest (QA UI-139).
 */
export function thanBiCat(body: string): boolean {
  return body.length > 240 || (body.match(/\n/g)?.length ?? 0) >= 6;
}

/** One line of the notifications list, as the server sends it (QA UI-147). */
export type ThongBao = {
  id: string;
  post_id: string;
  kind: string;
  created_at: string;
  /** Who mentioned you; null for a notification older than the field. */
  actor?: string | null;
  /** True when the mention is in a comment, not in the post itself. */
  from_comment?: boolean;
  /** The first words of what mentions you, already cut by the server. */
  excerpt?: string | null;
};

/** «Lan nhắc bạn trong một bình luận»; who and where, never a fixed sentence for every line. */
export function cauThongBao(t: ThongBao): string {
  const noi = t.from_comment ? "một bình luận" : "một câu chuyện";
  if (t.kind === "mention") return t.actor ? `${t.actor} nhắc bạn trong ${noi}` : `Bạn được nhắc trong ${noi}`;
  return t.actor ? `${t.actor} có điều mới cho bạn` : "Có điều mới cho bạn";
}

/** Where the notifications list was last opened on this phone, per person. */
export const khoaThongBaoDaXem = (person: string) => `cong-dong-thong-bao-da-xem:${person}`;

/**
 * Whether the bell carries a dot: a notification newer than the moment this
 * phone last opened the list. Per phone, by design: the server keeps no
 * «seen» mark the app reads yet.
 */
export function coThongBaoMoi(ds: readonly ThongBao[], daXemLuc: string | null): boolean {
  if (ds.length === 0) return false;
  if (daXemLuc === null) return true;
  const moc = Date.parse(daXemLuc);
  return ds.some((t) => Date.parse(t.created_at) > moc);
}

/**
 * A review row's state in words (QA UI-144: «· pending» printed raw). The
 * queue holds `pending` (waiting its first reading) and `review` (sent to a
 * person); the others are named so a new state never prints its code.
 */
export function nhanTrangThaiDuyet(status: string): string {
  switch (status) {
    case "pending":
      return "Chờ duyệt";
    case "review":
      return "Cần người xem lại";
    case "approved":
      return "Đã duyệt";
    case "rejected":
      return "Chưa phù hợp";
    case "private":
      return "Riêng tư";
    default:
      return "Đang xử lý";
  }
}
