/**
 * Phiên đăng nhập của chính mình (L5, ADR-0023 §2.5).
 *
 * Máy chủ không lưu nhãn thiết bị, nên màn này không bịa ra một cái. Cái nó
 * nói được là cửa nào đã cấp phiên, cấp lúc nào, và phiên nào là phiên đang
 * dùng — cờ `current` do máy chủ đặt từ chính bearer của request, không phải
 * do client đoán.
 */
import { type Attempt, translatedAsActor } from "../../api";

export type CuaCapPhien = "invite" | "otp" | "google" | "genesis";

export type PhienWire = {
  id: string;
  issued_via: CuaCapPhien;
  created_at: string;
  expires_at: string;
  current: boolean;
};

export type DanhSachPhienWire = { sessions: PhienWire[] };

export const LOI_PHIEN: Record<string, string> = {
  session_not_found: "Phiên này không còn.",
  authentication_required: "Bạn cần đăng nhập lại.",
};

/** Câu dưới mỗi hàng: cửa nào đã cấp phiên này. */
export function nhanCua(cua: string): string {
  if (cua === "otp") return "Số điện thoại";
  if (cua === "google") return "Google";
  if (cua === "invite") return "Lời mời";
  if (cua === "genesis") return "Bản dựng";
  return "Cách khác";
}

/** «Đang dùng máy này» hoặc ngày mở phiên; không có nhãn thiết bị để nói. */
export function cauPhien(phien: PhienWire): string {
  if (phien.current) return "Phiên này, đang dùng";
  const luc = new Date(phien.created_at);
  if (Number.isNaN(luc.getTime())) return nhanCua(phien.issued_via);
  return `${nhanCua(phien.issued_via)} · từ ${luc.toLocaleDateString("vi-VN")}`;
}

export async function docPhien(actorId: string): Promise<DanhSachPhienWire> {
  return translatedAsActor<DanhSachPhienWire>(LOI_PHIEN, "/sessions", {
    method: "GET",
    actorId,
  });
}

export async function thuHoiPhien(sessionId: string, actorId: string, attempt: Attempt): Promise<void> {
  await translatedAsActor<void>(LOI_PHIEN, `/sessions/${sessionId}`, {
    method: "DELETE",
    actorId,
    attempt,
  });
}
