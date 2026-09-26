/**
 * Nếp's words: asking a question and reading the sealed answer back
 * (ADR-0036 §2.6-§2.8).
 *
 * What goes with a question is exactly three things, and the panel prints all
 * three above the send button:
 *   - the context slip the open screen declared (`phieu.ts`, through
 *     `donPhieu`), printed as `cauNguCanh`;
 *   - the turns of THIS panel session, oldest first, printed as a count;
 *   - the question itself.
 * The session lives in React state only. It is never written to disk and it
 * dies with the panel: ADR-0036 §4 forbids keeping it on the device or on the
 * server, and the server scrubs it with the question when the job ends.
 *
 * The answer comes back to the caller alone. It is not a card in any room.
 *
 * Pure apart from the two HTTP calls, so the bounds and the sentences run under
 * bare node (`tests/nep-hoi.test.mjs`).
 */
import { translatedAsActor } from "../../api";
import { nepPhaiLui, type PhieuNguCanh } from "./phieu";

export type VaiNep = "toi" | "nep";
export interface LuotNep {
  vai: VaiNep;
  chu: string;
}

/**
 * The server's bounds (`chatassist/nep.go`), counted the same way: in code
 * points, not UTF-16 units, so a Vietnamese sentence is measured the way the
 * server measures it.
 */
export const GIOI_HAN_PHIEN = Object.freeze({ luot: 24, chuMoiLuot: 2000, hoi: 2000, tong: 16000 });

const soChu = (s: string) => [...s].length;

/**
 * Which session turns go with the next question: the newest ones that fit,
 * oldest first. Dropping the oldest is the device's choice and it is printed
 * («kèm N lượt»), which is why the server refuses instead of trimming.
 */
export function gomPhien(luot: readonly LuotNep[], hoi: string): LuotNep[] {
  const out: LuotNep[] = [];
  let tong = soChu(hoi);
  for (let i = luot.length - 1; i >= 0 && out.length < GIOI_HAN_PHIEN.luot; i--) {
    const l = luot[i];
    const n = soChu(l.chu);
    if (!l.chu.trim() || n > GIOI_HAN_PHIEN.chuMoiLuot || tong + n > GIOI_HAN_PHIEN.tong) break;
    tong += n;
    out.push({ vai: l.vai, chu: l.chu });
  }
  return out.reverse();
}

/** The second line of «Mình đang thấy»: how much of this session goes along. */
export function cauPhienDiKem(soLuot: number): string {
  if (soLuot <= 0) return "Chưa có lượt hỏi đáp nào trước câu này. Mình không đọc chat, gu hay lịch sử của bạn.";
  return `Kèm ${soLuot} lượt hỏi đáp trước đó trong lần mở này. Mình không đọc chat, gu hay lịch sử của bạn.`;
}

/**
 * Whether this screen lets Nếp answer at all. The dock already cannot open on
 * a money screen (ADR-0035 §2.4); this is the same law for the case where the
 * panel somehow is open there, and the server holds it a third time.
 */
export function nepDuocHoi(phieu: PhieuNguCanh | null): boolean {
  return !(phieu && nepPhaiLui(phieu.man));
}

export type TrangThaiHoi = "queued" | "running" | "succeeded" | "failed" | "cancelled";
export interface CauHoiNep {
  id: string;
  status: TrangThaiHoi;
  code: string | null;
  /** The sealed answer; only on the caller's own read, only once it succeeded. */
  text: string | null;
  created_at: string;
  updated_at: string;
}

/**
 * Refusals the two Nếp routes can return, in words a person can act on.
 * `tests/cau-chu-goi-ai.test.mjs` derives the codes from the Go handlers:
 * a new refusal without a sentence here is red.
 */
export const LOI_NEP: Record<string, string> = {
  nep_lui_man_tien: "Ở màn tiền Nếp không trả lời, để bạn tự xem số liệu cho rõ. Ra màn khác rồi hỏi Nếp nhé.",
  invalid_invocation: "Câu hỏi đang trống hoặc dài quá. Bạn viết gọn lại rồi hỏi Nếp nhé.",
  boi_canh_sai_dang: "Bản app này đã cũ nên Nếp chưa đọc được câu hỏi. Cập nhật app rồi thử lại.",
  boi_canh_qua_lon: "Lần mở này đã hỏi đáp khá dài. Bạn đóng bảng Nếp rồi mở lại để bắt đầu lượt mới nhé.",
  authentication_required: "Phiên đăng nhập đã hết. Bạn đăng nhập lại rồi hỏi Nếp tiếp nhé.",
  invocation_conflict: "Câu này vừa gửi đi với nội dung khác. Đợi Nếp trả lời xong rồi hỏi lại nhé.",
  invocation_rate_limited: "Bạn hỏi hơi nhanh. Chờ một chút rồi hỏi Nếp tiếp nhé.",
  provider_unavailable: "Nếp chưa trả lời được lúc này. Bạn thử lại sau ít phút nhé.",
  chat_ai_unavailable: "Nếp chưa trả lời được lúc này. Bạn thử lại sau ít phút nhé.",
  invocation_not_found: "Không còn thấy câu hỏi này nữa. Bạn hỏi lại Nếp nhé.",
};

/**
 * Why a question the worker picked up ended without an answer. These never
 * come back as an HTTP refusal, so they are not in `LOI_NEP`.
 */
export const LOI_KET_QUA_NEP: Record<string, string> = {
  provider_unavailable: "Nếp chưa trả lời được lúc này. Bạn thử lại sau ít phút nhé.",
  invalid_ai_result: "Nếp nghĩ chưa ra câu trả lời gọn. Bạn hỏi lại theo cách khác nhé.",
  sharing_unavailable: "Phiên đăng nhập vừa đổi nên Nếp dừng câu này. Bạn hỏi lại nhé.",
  sharing_expired: "Câu hỏi chờ lâu quá nên Nếp đã bỏ đi. Bạn hỏi lại nhé.",
  worker_interrupted: "Nếp bị ngắt giữa chừng. Bạn hỏi lại nhé.",
};

export function cauKetQuaNep(code: string | null): string {
  return (code ? LOI_KET_QUA_NEP[code] : undefined) ?? "Nếp chưa trả lời được câu này. Bạn hỏi lại nhé.";
}

/** Whether to keep reading. A finished job, any kind of finished, stops it. */
export function conCho(status: TrangThaiHoi): boolean {
  return status === "queued" || status === "running";
}

/**
 * How long to wait before the next read. A text answer takes seconds, not the
 * minutes a drawing takes, so it starts fast and caps low.
 */
export function nhipHoiNepMs(lanThu: number): number {
  const n = Number.isFinite(lanThu) ? Math.max(0, Math.floor(lanThu)) : 0;
  return Math.min(4_000, 800 + n * 400);
}

/** After this long, stop reading and say so rather than spin. */
export const CHO_TOI_DA_MS = 90_000;

/**
 * The key one attempt is remembered by. It covers the session and the slip as
 * well as the words: the server digests all three, so the same question over
 * a longer session is a new question, never a replay of the old answer.
 */
export function khoaLanHoi(hoi: string, luot: readonly LuotNep[], phieu: PhieuNguCanh | null): string {
  return JSON.stringify([hoi, luot, phieu]);
}

export function goiNep(
  actorId: string,
  hoi: string,
  logicalId: string,
  luot: readonly LuotNep[],
  phieu: PhieuNguCanh | null,
) {
  return translatedAsActor<CauHoiNep>(LOI_NEP, `/me/nep/ai-invocations`, {
    method: "POST",
    actorId,
    timeoutMs: 15000,
    body: { logical_id: logicalId, prompt: hoi, phieu, luot },
  });
}

export function docNep(actorId: string, id: string) {
  return translatedAsActor<CauHoiNep>(LOI_NEP, `/me/nep/ai-invocations/${encodeURIComponent(id)}`, {
    method: "GET",
    actorId,
    timeoutMs: 15000,
  });
}
