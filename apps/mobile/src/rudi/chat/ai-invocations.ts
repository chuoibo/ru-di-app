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
 * What a typed command asks for, and the words that go with it. `/chia-bill`
 * alone still needs a request the server will accept (it refuses an empty
 * prompt), so it gets a plain one; the words after it are the caller's own and
 * may carry an expense of their own («/chia-bill mình trả 300k tiền nước»).
 */
export function docLenhAi(body: string): { lenh: LenhAi; prompt: string } {
  const chia = /^\/chia-?bill\b\s*/i.exec(body);
  if (chia) return { lenh: "chia_bill", prompt: body.slice(chia[0].length).trim() || LOI_NHO_CHIA_BILL };
  return { lenh: "plan", prompt: body.replace(/^\/plan\s*|^@(rủ đi|ru di|rudi)\s*/i, "") };
}
export const LOI_NHO_CHIA_BILL = "Gom giúp các khoản chi trong đoạn chat";

export type AiInvocation = {
  id: string;
  /** Older servers did not echo it; treat a missing command as `plan`. */
  command?: LenhAi;
  status: "queued" | "running" | "succeeded" | "failed" | "cancelled";
  code: string | null;
  message_id: string | null;
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
};

/**
 * Why a job the worker picked up ended without a card, for the codes a person
 * can do something about. These never come back as an HTTP refusal, so they
 * are not `LOI_GOI_AI` (whose every key must be one), and a code missing here
 * falls back to the row's own sentence.
 */
export const LOI_KET_QUA_AI: Record<string, string> = {
  chia_bill_no_expenses: "Rủ Đi AI chưa thấy khoản chi nào có số tiền trong đoạn chat gửi kèm. Bạn gửi kèm tin có số tiền, hoặc thêm khoản chi ở mục Chia bill.",
};

/** A job whose answer would be the same on retry offers no «Thử lại». */
export function thuLaiDuoc(request: AiInvocation): boolean {
  return request.status === "failed" && request.code !== "chia_bill_no_expenses";
}

/** The words on a pending or failed invocation row, per command. */
export function chuHangLoiGoi(request: AiInvocation): { tieuDe: string; cau: string } {
  const chia = request.command === "chia_bill";
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
 */
export function goiAi(contextId: string, personId: string, prompt: string, logicalId: string, boiCanh?: BoiCanh, lenh: LenhAi = "plan") {
  return translatedAsActor<AiInvocation>(LOI_GOI_AI, `/contexts/${contextId}/ai-invocations`, {
    ...options(contextId, personId), method: "POST",
    body: { logical_id: logicalId, command: lenh, prompt, ...(boiCanh ? { boi_canh: boiCanh } : {}) },
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
