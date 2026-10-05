/**
 * The chat v2 lane's HTTP routes (services/core/internal/chatv2http). Bearer
 * sessions only; every byte field base64; refusals keep the server's code
 * (`chat_v2_stale_epoch`, `chat_v2_not_ready`, ...) on the ApiError so the
 * engine can branch on it.
 */
import { translatedAsActor } from "../../../api";
import type { Card, CommitBundle, Envelope, Event, Page, RosterView, WelcomeView } from "./kieu";

const LOI: Record<string, string> = {
  chat_v2_device_limit: "Tài khoản đã có 5 thiết bị nhắn tin mã hoá. Gỡ một thiết bị cũ trong Cài đặt rồi thử lại.",
  chat_v2_not_ready: "Phòng đang thiết lập mã hoá. Đợi một chút.",
  chat_v2_forbidden: "Thiết bị này không còn quyền trong phòng.",
};

export type ApiV2 = {
  enroll(actorId: string, body: { device_id: string; mls_signature_key: string; transport_signature_key: string; proof: string; label: string }): Promise<Card>;
  publishKeyPackages(actorId: string, device: string, packages: string[]): Promise<{ available: number; max: number }>;
  welcomes(actorId: string, device: string): Promise<WelcomeView[]>;
  ackWelcome(actorId: string, device: string, welcome: string): Promise<void>;
  roster(actorId: string, room: string, device: string): Promise<RosterView>;
  bootstrap(actorId: string, room: string, device: string): Promise<RosterView>;
  claim(actorId: string, room: string, device: string, targets: string[]): Promise<{ card: Card; key_package: string }[]>;
  commit(actorId: string, room: string, bundle: CommitBundle, added: string[], removed: string[]): Promise<{ event: Event; ready: boolean }>;
  send(actorId: string, room: string, envelope: Envelope): Promise<{ event: Event; replayed: boolean }>;
  events(actorId: string, room: string, device: string, after: number): Promise<Page>;
};

export const apiV2: ApiV2 = {
  enroll: (actorId, body) => translatedAsActor<Card>(LOI, "/v2/chat/devices", { body, actorId }),
  publishKeyPackages: (actorId, device, packages) =>
    translatedAsActor(LOI, `/v2/chat/devices/${device}/key-packages`, { body: { key_packages: packages }, actorId }),
  welcomes: async (actorId, device) =>
    (await translatedAsActor<{ welcomes: WelcomeView[] }>(LOI, `/v2/chat/devices/${device}/welcomes`, { method: "GET", actorId })).welcomes,
  ackWelcome: async (actorId, device, welcome) => {
    await translatedAsActor(LOI, `/v2/chat/devices/${device}/welcomes/${welcome}`, { method: "DELETE", actorId });
  },
  roster: (actorId, room, device) =>
    translatedAsActor<RosterView>(LOI, `/v2/chat/${room}/roster?device_id=${device}`, { method: "GET", actorId, contexts: room }),
  bootstrap: (actorId, room, device) =>
    translatedAsActor<RosterView>(LOI, `/v2/chat/${room}/bootstrap`, { body: { device_id: device }, actorId, contexts: room }),
  claim: async (actorId, room, device, targets) =>
    (await translatedAsActor<{ claims: { card: Card; key_package: string }[] }>(LOI, `/v2/chat/${room}/key-packages/claim`, {
      body: { device_id: device, targets },
      actorId,
      contexts: room,
    })).claims,
  commit: (actorId, room, bundle, added, removed) =>
    translatedAsActor(LOI, `/v2/chat/${room}/commits`, {
      body: { envelope: bundle.envelope, added, removed, ...(bundle.welcome === null ? {} : { welcome: bundle.welcome }) },
      actorId,
      contexts: room,
    }),
  send: (actorId, room, envelope) => translatedAsActor(LOI, `/v2/chat/${room}/events`, { body: envelope, actorId, contexts: room }),
  events: (actorId, room, device, after) =>
    translatedAsActor<Page>(LOI, `/v2/chat/${room}/events?device_id=${device}&after=${after}&limit=100`, { method: "GET", actorId, contexts: room }),
};
