/**
 * Who may comment on my posts (ADR-0022 §2.2): one setting on `PATCH /people/me`.
 *
 * Three words, the same three the server CHECKs. The screen shows them as
 * chips on one's own wall (and later in Settings, L5); the server answers
 * `can_comment` on every post from this, so nothing here decides anything --
 * it only names the choice and sends it.
 */
import { type Attempt, translatedAsActor } from "../../api";

export type ChinhSachBinhLuan = "readers" | "friends" | "nobody";

export const CHINH_SACH: { id: ChinhSachBinhLuan; nhan: string; giaiThich: string }[] = [
  { id: "readers", nhan: "Ai đọc được bài", giaiThich: "Ai đọc được bài thì bình luận được." },
  { id: "friends", nhan: "Chỉ bạn bè", giaiThich: "Người cùng nhóm đọc được vẫn chỉ xem, không bình luận." },
  { id: "nobody", nhan: "Không ai", giaiThich: "Tường chỉ để kể; không ai bình luận, kể cả bạn bè." },
];

export const MAC_DINH_CHINH_SACH: ChinhSachBinhLuan = "readers";

export function laChinhSach(value: unknown): value is ChinhSachBinhLuan {
  return CHINH_SACH.some((c) => c.id === value);
}

/** The label for a chip; an unknown word from an older client reads as the default. */
export function nhanChinhSach(id: string | undefined): string {
  const chon = CHINH_SACH.find((c) => c.id === id);
  if (chon === undefined) return CHINH_SACH[0].nhan;
  return chon.nhan;
}

export const LOI_CHINH_SACH: Record<string, string> = {
  person_not_found: "Máy chủ chưa có hồ sơ cho tài khoản này.",
  permission_denied: "Chỉ bạn mới đổi được cài đặt của tường mình.",
};

export type HoSoSauKhiDoi = { id: string; display_name: string; wall_comment_policy: ChinhSachBinhLuan };

export async function datChinhSachBinhLuan(
  policy: ChinhSachBinhLuan,
  actorId: string,
  attempt: Attempt,
): Promise<HoSoSauKhiDoi> {
  return translatedAsActor<HoSoSauKhiDoi>(LOI_CHINH_SACH, "/people/me", {
    method: "PATCH",
    body: { wall_comment_policy: policy },
    actorId,
    attempt,
  });
}
