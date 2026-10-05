/**
 * This account's devices on the chat v2 lane (ADR-0057 §1.3–§1.4): which
 * phones hold its end-to-end keys, a key mark to compare out of band, and
 * taking one out. A device taken out leaves every room at the next commit and
 * reads nothing sent after it.
 */
import { translatedAsActor } from "../../../api";
import type { Card } from "./kieu";

export type ThietBiWire = { card: Card; label: string; created_at: string; revoked_at: string | null };

const LOI: Record<string, string> = {
  chat_v2_forbidden: "Thiết bị này không thuộc tài khoản của bạn.",
};

export async function docThietBi(actorId: string): Promise<{ devices: ThietBiWire[]; max_devices: number }> {
  return translatedAsActor(LOI, "/v2/chat/devices", { method: "GET", actorId });
}

export async function goThietBi(actorId: string, deviceId: string): Promise<void> {
  await translatedAsActor(LOI, `/v2/chat/devices/${deviceId}`, { method: "DELETE", actorId });
}

/**
 * The device's key mark: the first 10 bytes of its transport key and of its
 * MLS key, in groups of four hex digits. The same device shows the same mark
 * on every phone that lists it; a mark that differs between two phones means
 * the server handed one of them another key.
 */
export function dauKhoa(card: Card): string {
  const hex = [...card.transport_signature_key.slice(0, 10), ...card.mls_signature_key.slice(0, 10)]
    .map((b) => (b & 0xff).toString(16).padStart(2, "0"))
    .join("");
  return (hex.match(/.{1,4}/g) ?? []).join(" ");
}

/** Whether two cards carry the same keys for the same device. */
export function cungKhoa(a: Card, b: Card): boolean {
  const bang = (x: number[], y: number[]) => x.length === y.length && x.every((v, i) => v === y[i]);
  return a.actor_id === b.actor_id && a.device_id === b.device_id && bang(a.mls_signature_key, b.mls_signature_key) && bang(a.transport_signature_key, b.transport_signature_key);
}
