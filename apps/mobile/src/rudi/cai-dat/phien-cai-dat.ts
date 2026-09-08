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

/**
 * One table per door, read from two places.
 *
 * `nhan` is the noun a session row is labelled with; `qua` is the clause the
 * welcome line uses. They are different words for the same fact, so they live
 * in one row: a second hand-written list is a list that drifts, and the way it
 * drifts is that a door added later gets one of the two and not the other.
 *
 * `qua` is empty where no honest clause exists. A genesis session is seeded out
 * of band and has no story to tell a person, and saying nothing is better than
 * naming a door they did not walk through.
 */
const CUA: Record<CuaCapPhien, { nhan: string; qua: string }> = {
  otp: { nhan: "Số điện thoại", qua: "bằng số điện thoại của bạn" },
  google: { nhan: "Google", qua: "bằng tài khoản Google của bạn" },
  invite: { nhan: "Lời mời", qua: "từ lời mời bạn vừa nhận" },
  genesis: { nhan: "Bản dựng", qua: "" },
};

/** Câu dưới mỗi hàng: cửa nào đã cấp phiên này. */
export function nhanCua(cua: string): string {
  return CUA[cua as CuaCapPhien]?.nhan ?? "Cách khác";
}

/**
 * Câu chào hiện đúng một lần, ngay sau lần đăng nhập đầu của một tài khoản.
 *
 * Trước khi cửa Google chạy được thật, mọi tài khoản đều tới bằng số điện
 * thoại, nên câu ghi cứng «bằng số điện thoại của bạn» chưa ai thấy sai. Đo
 * trên máy 2026-09-09 với một tài khoản Google mới: nó nói sai ngay dòng đầu
 * tiên người ta đọc. Một cửa bản này chưa biết thì không khẳng định cửa nào.
 */
export function cauTaiKhoanVuaTao(cua: string): string {
  const qua = CUA[cua as CuaCapPhien]?.qua ?? "";
  const mo = qua ? `Tài khoản vừa được tạo ${qua}.` : "Tài khoản vừa được tạo.";
  return `${mo} Tên hiển thị sửa được ở mục Cá nhân.`;
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
