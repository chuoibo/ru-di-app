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
  ai: { plan: { available: boolean; reason: string | null }; share_scope: "invocation_only" | "caller_attached" };
  media: { image: boolean; sticker: boolean; voice: boolean };
};
export type AiInvocation = {
  id: string;
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
 * Refusals this route can return, in words a person can act on.
 *
 * Without this table every 4xx lands on the generic sentence, which says the
 * fault is the app's. For a bundle that is too large that sentence is simply
 * wrong: the person can fix it, by sending fewer messages.
 */
export const LOI_GOI_AI: Record<string, string> = {
  boi_canh_qua_lon: "Đoạn chat gửi kèm dài quá. Bạn chọn «Chỉ gửi lời nhờ», hoặc thử lại để mình gửi ít tin hơn.",
  boi_canh_sai_dang: "Bản app này gửi bối cảnh theo kiểu máy chủ chưa đọc được. Cập nhật app rồi thử lại.",
  boi_canh_mismatch: "Có tin trong đoạn gửi kèm không thuộc nhóm này. Bạn thử lại nhé.",
  invocation_conflict: "Lời nhờ này đã gửi rồi với nội dung khác. Đợi kết quả cũ xong rồi gửi lại nhé.",
  invocation_rate_limited: "Bạn hỏi hơi nhanh. Chờ một chút rồi nhờ tiếp nhé.",
  invocation_not_retryable: "Lời nhờ này hết hạn chia sẻ rồi. Bạn viết lại một lời nhờ mới nhé.",
  provider_unavailable: "AI chưa sẵn sàng. Bạn vẫn có thể tự tạo kèo.",
  chat_ai_unavailable: "AI chưa sẵn sàng. Bạn vẫn có thể tự tạo kèo.",
  group_plan_only: "Chỗ này chưa nhờ AI phác kèo được.",
};

/**
 * @param boiCanh the bundle the person just saw above the send button. Omitted
 *   entirely when the server still declares `invocation_only`, so the body on
 *   the wire is byte for byte the old one.
 */
export function goiAi(contextId: string, personId: string, prompt: string, logicalId: string, boiCanh?: BoiCanh) {
  return translatedAsActor<AiInvocation>(LOI_GOI_AI, `/contexts/${contextId}/ai-invocations`, {
    ...options(contextId, personId), method: "POST",
    body: { logical_id: logicalId, command: "plan", prompt, ...(boiCanh ? { boi_canh: boiCanh } : {}) },
  });
}
export function thuLaiAi(contextId: string, personId: string, id: string) {
  return translatedAsActor<AiInvocation>({}, `/contexts/${contextId}/ai-invocations/${id}/retry`, {
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
