/**
 * Xoá tài khoản (L5, ADR-0023 §2.1) — hai bước và một từ gõ tay.
 *
 * Không có nút «Xoá» đứng một mình ở đâu trong app này. Người dùng phải đọc
 * một màn nói thẳng điều gì mất và điều gì ở lại, rồi gõ đúng một từ. Cái giá
 * của một cú chạm nhầm ở đây không lấy lại được.
 */
import { type Attempt, translatedAsActor } from "../../api";

/**
 * Từ phải gõ, viết như mọi chữ «Xoá» trên màn (QA UI-110: câu nhắc «XOA» không
 * dấu, gõ «XOÁ» thì nút tắt mà không nói vì sao). Có dấu hay không đều nhận:
 * bàn phím nào cũng gõ được.
 */
export const TU_XAC_NHAN = "XOÁ";

export const LOI_XOA_TAI_KHOAN: Record<string, string> = {
  confirm_required: "Cần xác nhận rõ ràng để xoá tài khoản.",
  person_not_found: "Không còn hồ sơ cho tài khoản này.",
  authentication_required: "Bạn cần đăng nhập lại.",
};

/** Bốn câu màn xoá nói ra trước khi hỏi. Sự thật, không phải trấn an. */
export const DIEU_SE_XAY_RA: readonly string[] = [
  "Tên, giới thiệu, ảnh đại diện và ảnh cá nhân của bạn bị xoá.",
  "Bài đăng, story, bình luận và phản ứng của bạn bị xoá.",
  "Bạn rời mọi nhóm; tin nhắn cũ ở lại với tên «Người dùng đã rời».",
  "Sổ tiền của các nhóm giữ nguyên: ai nợ ai bao nhiêu không đổi.",
];

const boDau = (chu: string) => chu.normalize("NFD").replace(/[\u0300-\u036f]/g, "").replace(/đ/gi, "d");

export function xacNhanHopLe(da_go: string): boolean {
  return boDau(da_go.trim().toUpperCase()) === boDau(TU_XAC_NHAN);
}

export async function xoaTaiKhoan(actorId: string, attempt: Attempt): Promise<void> {
  await translatedAsActor<void>(LOI_XOA_TAI_KHOAN, "/people/me", {
    method: "DELETE",
    body: { confirm: true },
    actorId,
    attempt,
  });
}
