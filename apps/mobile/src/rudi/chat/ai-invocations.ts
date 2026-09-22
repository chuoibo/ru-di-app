import { newAttempt, translatedAsActor } from "../../api";
import type { BodyTaoBuoiDi, ChangGui } from "../../screens/len-plan/buoi-di";

export type ChatCapabilities = {
  protocol: "legacy";
  realtime: { available: boolean };
  ai: { plan: { available: boolean; reason: string | null }; share_scope: "invocation_only" };
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
export function goiAi(contextId: string, personId: string, prompt: string, logicalId: string) {
  return translatedAsActor<AiInvocation>({}, `/contexts/${contextId}/ai-invocations`, {
    ...options(contextId, personId), method: "POST", body: { logical_id: logicalId, command: "plan", prompt },
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
