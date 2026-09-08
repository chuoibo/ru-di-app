/**
 * Quyền riêng tư: ai tìm được mình, ai bị chặn, và báo cáo (L5, ADR-0023).
 *
 * `chan`/`boChan` gọi thẳng cửa của máy chủ và không đoán trước kết quả: chặn
 * hai lần vẫn là 200 với cùng một thân, nên màn không cần trạng thái riêng để
 * biết mình đã chặn hay chưa — nó đọc lại danh sách.
 *
 * Danh sách chặn CHỈ có người mình chặn. Không có route nào trả «ai đang chặn
 * tôi», và sẽ không có: đó đúng là điều `BLOCKED_IS_SILENT` giấu.
 */
import { type Attempt, translatedAsActor } from "../../api";

export type LyDoBaoCao = "spam" | "harassment" | "inappropriate" | "impersonation" | "other";
export type LoaiBaoCao = "person" | "post" | "message" | "comment" | "story";

export type NguoiBiChan = { person_id: string; display_name: string; blocked_at: string };
export type DanhSachChanWire = { blocked: NguoiBiChan[] };
export type ChanWire = { person_id: string; state: "blocked" | "declined" };
export type BaoCaoWire = { id: string; created_at: string };

/** Cùng bộ từ với `app/domain/reports.py`; test node so hai danh sách. */
export const LY_DO_BAO_CAO: readonly { ma: LyDoBaoCao; nhan: string }[] = [
  { ma: "spam", nhan: "Rác, quảng cáo" },
  { ma: "harassment", nhan: "Làm phiền, quấy rối" },
  { ma: "inappropriate", nhan: "Nội dung không phù hợp" },
  { ma: "impersonation", nhan: "Giả mạo người khác" },
  { ma: "other", nhan: "Lý do khác" },
];

export const LOI_RIENG_TU: Record<string, string> = {
  person_not_found: "Không tìm thấy người này.",
  permission_denied: "Chỉ người đã chặn mới gỡ chặn được.",
  not_blocked: "Bạn chưa chặn người này.",
  only_blocker_may_unblock: "Chỉ người đã chặn mới gỡ chặn được.",
  unknown_reason: "Chọn một lý do trong danh sách.",
  note_too_long: "Ghi chú dài quá 500 ký tự.",
};

export async function chan(personId: string, actorId: string, attempt: Attempt): Promise<ChanWire> {
  return translatedAsActor<ChanWire>(LOI_RIENG_TU, `/people/${personId}/block`, {
    method: "POST",
    actorId,
    attempt,
  });
}

export async function boChan(personId: string, actorId: string, attempt: Attempt): Promise<ChanWire> {
  return translatedAsActor<ChanWire>(LOI_RIENG_TU, `/people/${personId}/block`, {
    method: "DELETE",
    actorId,
    attempt,
  });
}

export async function docDaChan(actorId: string): Promise<DanhSachChanWire> {
  return translatedAsActor<DanhSachChanWire>(LOI_RIENG_TU, "/people/me/blocked", {
    method: "GET",
    actorId,
  });
}

/** Thân báo cáo chỉ mang khoá có giá trị; ghi chú rỗng thì không gửi khoá. */
export function thanBaoCao(
  loai: LoaiBaoCao,
  targetId: string,
  lyDo: LyDoBaoCao,
  ghiChu: string,
): { target_type: LoaiBaoCao; target_id: string; reason: LyDoBaoCao; note?: string } {
  const gon = ghiChu.trim();
  const than = { target_type: loai, target_id: targetId, reason: lyDo };
  return gon === "" ? than : { ...than, note: gon };
}

export async function baoCao(
  loai: LoaiBaoCao,
  targetId: string,
  lyDo: LyDoBaoCao,
  ghiChu: string,
  actorId: string,
  attempt: Attempt,
): Promise<BaoCaoWire> {
  return translatedAsActor<BaoCaoWire>(LOI_RIENG_TU, "/reports", {
    method: "POST",
    body: thanBaoCao(loai, targetId, lyDo, ghiChu),
    actorId,
    attempt,
  });
}
