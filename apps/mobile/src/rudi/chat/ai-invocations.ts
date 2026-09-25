import { newAttempt, translatedAsActor } from "../../api";
import type { BoiCanh } from "../ai/boi-canh";
import type { BodyTaoBuoiDi, ChangGui } from "../../screens/len-plan/buoi-di";

export type ChatCapabilities = {
  protocol: "legacy";
  realtime: { available: boolean };
  /**
   * `share_scope` is the GATE, not a label. While a server still says
   * `invocation_only` the client attaches no bundle at all, so an old server
   * keeps receiving exactly the old body and its tests stay green. That is the
   * whole rollout plan; there is no second feature flag.
   */
  ai: {
    plan: { available: boolean; reason: string | null };
    /** Absent on a server from before `command=chia_bill` existed: read as unavailable. */
    chia_bill?: { available: boolean; reason: string | null };
    share_scope: "invocation_only" | "caller_attached";
    /**
     * The server takes `trigger_message_id` and answers inside the thread, as a
     * reply to the `@Rủ Đi` message (ADR-0039). Absent on an older server,
     * which would refuse the unknown field: then no trigger is sent, and the
     * answer arrives as the card it always was.
     */
    mention?: boolean;
  };
  media: { image: boolean; sticker: boolean; voice: boolean };
};
/** The two things a person can ask the group AI for, on one queue (ADR-0036 §2.9). */
export type LenhAi = "plan" | "chia_bill";

/** Whether the server says this command can run now. Fails closed. */
export function lenhSanSang(capabilities: ChatCapabilities | null, lenh: LenhAi): boolean {
  return (lenh === "plan" ? capabilities?.ai.plan : capabilities?.ai.chia_bill)?.available === true;
}

/**
 * What a bare `/chia-bill` asks for. The server refuses an empty prompt, so a
 * command with no words after it still needs a plain request; words after it
 * are the caller's own and may carry an expense of their own («/chia-bill
 * mình trả 300k tiền nước»). Reading a typed message is `nhac-ai.ts`.
 */
export const LOI_NHO_CHIA_BILL = "Gom giúp các khoản chi trong đoạn chat";

export type AiInvocation = {
  id: string;
  /** Older servers did not echo it; treat a missing command as `plan`. */
  command?: LenhAi;
  status: "queued" | "running" | "succeeded" | "failed" | "cancelled";
  code: string | null;
  message_id: string | null;
  /** The `@Rủ Đi` message it answers; absent or null on an invocation without one. */
  trigger_message_id?: string | null;
  /** How many shared messages the server confirmed; absent on older servers. */
  so_tin_doc?: number | null;
  created_at: string;
  updated_at: string;
};
const options = (contextId: string, personId: string) => ({ actorId: personId, contexts: contextId, timeoutMs: 15000 });
export function docChatCapabilities(contextId: string, personId: string) {
  return translatedAsActor<ChatCapabilities>({}, `/contexts/${contextId}/chat-capabilities`, { ...options(contextId, personId), method: "GET" });
}
export function docAiInvocations(contextId: string, personId: string) {
  return translatedAsActor<{ invocations: AiInvocation[] }>({}, `/contexts/${contextId}/ai-invocations?limit=20`, { ...options(contextId, personId), method: "GET" });
}
/**
 * Refusals the invocation routes can return, in words a person can act on.
 *
 * Without this table every 4xx lands on the generic sentence, which says the
 * fault is the app's. For a bundle that is too large that sentence is simply
 * wrong: the person can fix it, by sending fewer messages.
 *
 * Both calls that reach the queue (`goiAi` and `thuLaiAi`) read this table,
 * and `tests/cau-chu-goi-ai.test.mjs` derives the codes those two routes can
 * emit from the Go handlers: a new refusal without a sentence here is red.
 */
export const LOI_GOI_AI: Record<string, string> = {
  boi_canh_qua_lon: "Đoạn chat gửi kèm dài quá. Bạn chọn «Chỉ gửi lời nhờ», hoặc thử lại để mình gửi ít tin hơn.",
  boi_canh_sai_dang: "Bản app này đã cũ nên Rủ Đi chưa đọc được yêu cầu. Cập nhật app rồi thử lại.",
  boi_canh_mismatch: "Có tin trong đoạn gửi kèm không thuộc nhóm này. Bạn thử lại nhé.",
  invocation_conflict: "Lời nhờ này đã gửi rồi với nội dung khác. Đợi kết quả cũ xong rồi gửi lại nhé.",
  invocation_rate_limited: "Bạn hỏi hơi nhanh. Chờ một chút rồi nhờ tiếp nhé.",
  invocation_not_retryable: "Lời nhờ này hết hạn chia sẻ rồi. Bạn viết lại một lời nhờ mới nhé.",
  provider_unavailable: "AI chưa sẵn sàng. Bạn vẫn có thể tự tạo kèo.",
  chat_ai_unavailable: "AI chưa sẵn sàng. Bạn vẫn có thể tự tạo kèo.",
  group_plan_only: "Chỗ này chưa nhờ AI phác kèo được.",
  invalid_invocation: "Lời nhờ đang trống hoặc dài quá. Bạn viết gọn lại rồi gửi nhé.",
  authentication_required: "Phiên đăng nhập đã hết. Bạn đăng nhập lại rồi nhờ AI tiếp nhé.",
  membership_required: "Bạn không còn ở trong nhóm này nên chưa nhờ AI ở đây được.",
  encrypted_invocation_required: "Nhóm này đã chuyển sang chat mã hoá, nên cách nhờ AI này chưa dùng được ở đây.",
  invocation_not_found: "Không còn thấy lời nhờ này nữa. Bạn gửi một lời nhờ mới nhé.",
  // ADR-0039: the answer is a reply to the `@Rủ Đi` message, so the message
  // itself can be the reason a request is refused.
  trigger_khong_hop_le: "Rủ Đi AI chỉ trả lời tin nhờ của chính bạn trong nhóm này, gửi trong một ngày qua và chưa xoá. Bạn gửi một tin mới có @Rủ Đi nhé.",
  invocation_trigger_taken: "Tin này đã được nhờ Rủ Đi AI trả lời rồi. Câu trả lời sẽ hiện ngay dưới tin.",
  invocation_room_busy: "Rủ Đi AI đang trả lời ba lời nhờ trong nhóm. Đợi một câu xong rồi nhờ tiếp nhé.",
  invocation_room_rate_limited: "Nhóm đã nhờ Rủ Đi AI nhiều trong một giờ qua. Nghỉ tay một chút rồi nhờ tiếp nhé.",
};

/**
 * Why a job the worker picked up ended without a card, for the codes a person
 * can do something about. These never come back as an HTTP refusal, so they
 * are not `LOI_GOI_AI` (whose every key must be one), and a code missing here
 * falls back to the row's own sentence.
 */
export const LOI_KET_QUA_AI: Record<string, string> = {
  chia_bill_no_expenses: "Rủ Đi AI chưa thấy khoản chi nào có số tiền trong đoạn chat gửi kèm. Bạn gửi kèm tin có số tiền, hoặc thêm khoản chi ở mục Chia bill.",
  trigger_deleted: "Tin nhờ Rủ Đi AI không còn nữa, nên câu trả lời không được gửi. Bạn gửi một tin mới có @Rủ Đi nhé.",
};

/** A job whose answer would be the same on retry offers no «Thử lại». */
export function thuLaiDuoc(request: AiInvocation): boolean {
  return request.status === "failed" && request.code !== "chia_bill_no_expenses" && request.code !== "trigger_deleted";
}

/** The words on a pending or failed invocation row, per command. */
export function chuHangLoiGoi(request: AiInvocation): { tieuDe: string; cau: string } {
  const chia = request.command === "chia_bill";
  // An answer in the thread says what it is reading, with the count the
  // server confirmed (design 03 §5: «Rủ Đi AI đang đọc {n} tin…»).
  if (request.trigger_message_id && (request.status === "queued" || request.status === "running")) {
    const n = request.so_tin_doc ?? 0;
    return {
      tieuDe: n > 0 ? `Rủ Đi AI đang đọc ${n} tin…` : "Rủ Đi AI đang đọc lời nhờ…",
      cau: "Câu trả lời sẽ hiện ngay dưới tin của bạn.",
    };
  }
  if (request.status === "failed") {
    return {
      tieuDe: chia ? "Chưa gom được khoản chi" : "Chưa phác được tờ hẹn",
      cau: (request.code ? LOI_KET_QUA_AI[request.code] : undefined)
        ?? (chia ? "Lời nhờ vẫn được giữ. Bạn có thể thử lại hoặc thêm khoản chi ở mục Chia bill." : "Lời nhờ vẫn được giữ. Bạn có thể thử lại hoặc tự tạo kèo."),
    };
  }
  return {
    tieuDe: request.status === "queued" ? "Lời nhờ đang chờ" : chia ? "Đang gom khoản chi…" : "Đang phác tờ hẹn…",
    cau: "Bạn cứ trò chuyện, kết quả sẽ về đây.",
  };
}

/**
 * @param boiCanh the bundle the person just saw above the send button. Omitted
 *   entirely when the server still declares `invocation_only`, so the body on
 *   the wire is byte for byte the old one.
 * @param lenh `chia_bill` rides the same queue, digest, limits and preview as
 *   `plan`; only the server's inference step differs.
 * @param triggerMessageId the `@Rủ Đi` message this answers, already stored.
 *   Omitted entirely (not sent as null) when the server does not declare
 *   `mention`, so an older server keeps receiving exactly the old body.
 */
export function goiAi(contextId: string, personId: string, prompt: string, logicalId: string, boiCanh?: BoiCanh, lenh: LenhAi = "plan", triggerMessageId?: string) {
  return translatedAsActor<AiInvocation>(LOI_GOI_AI, `/contexts/${contextId}/ai-invocations`, {
    ...options(contextId, personId), method: "POST",
    body: { logical_id: logicalId, command: lenh, prompt, ...(boiCanh ? { boi_canh: boiCanh } : {}), ...(triggerMessageId ? { trigger_message_id: triggerMessageId } : {}) },
  });
}
export function thuLaiAi(contextId: string, personId: string, id: string) {
  return translatedAsActor<AiInvocation>(LOI_GOI_AI, `/contexts/${contextId}/ai-invocations/${id}/retry`, {
    ...options(contextId, personId), method: "POST", attempt: newAttempt(),
  });
}
export function gopAiInvocations(current: AiInvocation[], incoming: AiInvocation[]) {
  const byId = new Map(current.map((item) => [item.id, item]));
  for (const item of incoming) {
    const previous = byId.get(item.id);
    if (!previous || previous.updated_at <= item.updated_at) byId.set(item.id, item);
  }
  return [...byId.values()].sort((a, b) => b.created_at.localeCompare(a.created_at)).slice(0, 20);
}

export type PlanPromotion = { outing_id: string; timeline_revision: number; source_message_id: string };
export function taoKeoTuChat(contextId: string, personId: string, sourceMessageId: string, details: BodyTaoBuoiDi, stops?: ChangGui[]) {
  return translatedAsActor<PlanPromotion>({ plan_already_promoted: "Hội đã tạo kèo từ tờ hẹn này. Mở kèo để xem bản mới nhất." }, `/contexts/${contextId}/plan-promotions`, {
    ...options(contextId, personId), method: "POST", body: { source_message_id: sourceMessageId, ...details, ...(stops ? { stops } : {}) },
  });
}
export function docKeoTuChat(contextId: string, personId: string, sourceMessageId: string) {
  return translatedAsActor<PlanPromotion>({}, `/contexts/${contextId}/plan-promotions/${sourceMessageId}`, { ...options(contextId, personId), method: "GET" });
}
