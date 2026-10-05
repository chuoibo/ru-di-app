/**
 * One device's end of chat v2 (ADR-0057): enrolment, key packages, Welcomes,
 * keeping a room's MLS roster in line with the server's, reading and sending.
 *
 * What it never does: send plaintext anywhere, decide who belongs in a room
 * (the server attests the roster; the device refuses a commit that does not
 * match it), or hold a key -- the native module seals every change before it
 * answers, so nothing reaches the network that a crash could lose.
 *
 * The crypto and the HTTP lane are ports, so the orchestration is tested
 * against a fake lane and the real crypto (tests/e2ee-*.test.mjs).
 */
import { ApiError } from "../../../api";
import type { ApiV2 } from "./api-v2";
import type { Card, CommitBundle, Envelope, Event, Operation, Received, RosterView } from "./kieu";

export type CryptoPort = {
  open(actorId: string, deviceId: string): Promise<boolean>;
  createGroup(conversationId: string): Promise<string>;
  encrypt(conversationId: string, logicalSendId: string, operationJson: string): Promise<string>;
  receive(envelopeJson: string, rosterJson: string | null): Promise<string>;
  call(method: string, argsJson: string): Promise<string>;
};

/** A small durable key-value store (AsyncStorage in the app). */
export type KhoPort = { doc(key: string): Promise<string | null>; ghi(key: string, value: string): Promise<void> };

/** Key packages kept published: refilled below the low mark. */
const KP_THAP = 3;
const KP_DAY = 8;

function bytesToBase64(bytes: number[]): string {
  let s = "";
  for (const b of bytes) s += String.fromCharCode(b);
  return btoa(s);
}

function code(error: unknown): string {
  return error instanceof ApiError ? error.code : "";
}

export type TinNhan = { sequence: number; received: Extract<Received, { kind: "application" }> };

export class MayMaHoa {
  private device: string | null = null;

  constructor(
    private readonly d: {
      actorId: string;
      crypto: CryptoPort;
      api: ApiV2;
      kho: KhoPort;
      uuid: () => string;
      label: string;
    },
  ) {}

  private khoa(ten: string): string {
    return `rudi.chat-v2.${ten}.${this.d.actorId}`;
  }

  private async call<T>(method: string, args: object = {}): Promise<T> {
    return JSON.parse(await this.d.crypto.call(method, JSON.stringify(args))) as T;
  }

  /** Opens (or creates and enrols) this phone's device identity; returns its id. */
  async moThietBi(): Promise<string> {
    let device = await this.d.kho.doc(this.khoa("device"));
    if (device === null) {
      device = this.d.uuid();
      await this.d.kho.ghi(this.khoa("device"), device);
    }
    await this.d.crypto.open(this.d.actorId, device);
    if ((await this.d.kho.doc(this.khoa("enrolled"))) !== device) {
      const e = await this.call<{ card: Card; proof: string }>("enrollment");
      await this.d.api.enroll(this.d.actorId, {
        device_id: device,
        mls_signature_key: bytesToBase64(e.card.mls_signature_key),
        transport_signature_key: bytesToBase64(e.card.transport_signature_key),
        proof: e.proof,
        label: this.d.label,
      });
      await this.d.kho.ghi(this.khoa("enrolled"), device);
      await this.d.kho.ghi(this.khoa("kp"), "0");
    }
    this.device = device;
    await this.boSungKeyPackage();
    return device;
  }

  private may(): string {
    if (this.device === null) throw new Error("chat_v2_device_closed");
    return this.device;
  }

  /** Keeps a few key packages published so others can add this device. */
  async boSungKeyPackage(): Promise<void> {
    const con = Number((await this.d.kho.doc(this.khoa("kp"))) ?? "0");
    if (con >= KP_THAP) return;
    const goi: string[] = [];
    for (let i = con; i < KP_DAY; i++) goi.push((await this.call<{ key_package: string }>("key_package")).key_package);
    const r = await this.d.api.publishKeyPackages(this.d.actorId, this.may(), goi);
    await this.d.kho.ghi(this.khoa("kp"), String(r.available));
  }

  /** Joins every room a Welcome waits for, each against its attested roster. */
  async nhanWelcome(): Promise<string[]> {
    const vao: string[] = [];
    for (const w of await this.d.api.welcomes(this.d.actorId, this.may())) {
      await this.call("join_group", { conversation_id: w.conversation_id, welcome: w.welcome, roster: w.roster });
      await this.d.kho.ghi(this.khoa(`cursor.${w.conversation_id}`), String(w.sequence));
      await this.d.api.ackWelcome(this.d.actorId, this.may(), w.id);
      vao.push(w.conversation_id);
    }
    if (vao.length > 0) {
      // Each Welcome consumed one of this device's key packages.
      const con = Number((await this.d.kho.doc(this.khoa("kp"))) ?? "0");
      await this.d.kho.ghi(this.khoa("kp"), String(Math.max(0, con - vao.length)));
      await this.boSungKeyPackage();
    }
    return vao;
  }

  /**
   * Brings a room's MLS roster in line with the server's: opens the lane if
   * nobody has, adds the devices it expects, removes the ones it no longer
   * does. Answers whether the room is ready to send.
   */
  async chuanBi(room: string): Promise<boolean> {
    const device = this.may();
    let r: RosterView = await this.d.api.roster(this.d.actorId, room, device);
    if (!r.exists) {
      r = await this.d.api.bootstrap(this.d.actorId, room, device);
      await this.d.crypto.createGroup(room);
      await this.d.kho.ghi(this.khoa(`cursor.${room}`), "0");
    }
    if (!r.members.some((c) => c.device_id === device)) return false; // waiting for a Welcome
    for (let lan = 0; lan < 3 && !r.ready; lan++) {
      await this.dongBo(room, () => undefined);
      r = await this.d.api.roster(this.d.actorId, room, device);
      if (r.ready) break;
      const co = new Set(r.members.map((c) => c.device_id));
      const can = new Set(r.expected.map((c) => c.device_id));
      const them = [...can].filter((id) => !co.has(id));
      const bo = [...co].filter((id) => !can.has(id) && id !== device);
      try {
        if (bo.length > 0) {
          await this.ghiCommit(room, await this.call<CommitBundle>("stage_remove", { conversation_id: room, logical_send_id: this.d.uuid(), device_id: bo[0] }), [], [bo[0]]);
        } else if (them.length > 0) {
          const claims = await this.d.api.claim(this.d.actorId, room, device, them);
          const bundle = await this.call<CommitBundle>("stage_add", {
            conversation_id: room,
            logical_send_id: this.d.uuid(),
            members: claims.map((c) => ({ card: c.card, key_package: c.key_package })),
          });
          await this.ghiCommit(room, bundle, them, []);
        }
      } catch (error) {
        if (code(error) !== "chat_v2_stale_epoch" && code(error) !== "chat_v2_key_package_unavailable") throw error;
      }
      r = await this.d.api.roster(this.d.actorId, room, device);
    }
    return r.ready;
  }

  /** Posts a staged commit; on a lost epoch race it is abandoned and the winner read. */
  private async ghiCommit(room: string, bundle: CommitBundle, added: string[], removed: string[]): Promise<void> {
    try {
      await this.d.api.commit(this.d.actorId, room, bundle, added, removed);
    } catch (error) {
      if (code(error) === "chat_v2_stale_epoch") {
        await this.call("abandon_commit", { conversation_id: room });
        await this.dongBo(room, () => undefined);
      }
      throw error;
    }
    await this.call("acknowledge_commit", { envelope: bundle.envelope });
  }

  /**
   * Reads the room's new events and hands every decrypted message to `tin`.
   * The device's own sends are already known to the app; its own commits were
   * acknowledged when posted. A commit by another device is verified against
   * the roster the server attests in the event. Returns the last sequence read.
   */
  async dongBo(room: string, tin: (t: TinNhan) => void): Promise<number> {
    const device = this.may();
    let after = Number((await this.d.kho.doc(this.khoa(`cursor.${room}`))) ?? "0");
    for (;;) {
      const page = await this.d.api.events(this.d.actorId, room, device, after);
      for (const e of page.events) {
        await this.motSuKien(room, e, tin);
        after = e.sequence;
        await this.d.kho.ghi(this.khoa(`cursor.${room}`), String(after));
      }
      if (!page.has_more) return after;
    }
  }

  private async motSuKien(room: string, e: Event, tin: (t: TinNhan) => void): Promise<void> {
    const device = this.may();
    if (e.envelope !== undefined && e.envelope.device_id !== device) {
      const r = JSON.parse(await this.d.crypto.receive(JSON.stringify(e.envelope), null)) as Received;
      if (r.kind === "application") tin({ sequence: e.sequence, received: r });
    } else if (e.commit !== undefined && e.commit.envelope.device_id !== device) {
      const r = JSON.parse(await this.d.crypto.receive(JSON.stringify(e.commit.envelope), JSON.stringify(e.commit.roster))) as Received;
      if (r.kind === "removed") await this.call("forget", { conversation_id: room });
    }
  }

  /**
   * Sends one operation. A moved epoch re-encrypts the same logical send
   * under the new one after reading the commit; a room not yet ready is
   * brought in line first. Never falls back to anything unencrypted.
   */
  async gui(room: string, operation: Operation, logical: string = this.d.uuid()): Promise<Event> {
    for (let lan = 0; ; lan++) {
      const envelope = JSON.parse(await this.d.crypto.encrypt(room, logical, JSON.stringify(operation))) as Envelope;
      try {
        const r = await this.d.api.send(this.d.actorId, room, envelope);
        await this.call("acknowledge_sent", { envelope });
        return r.event;
      } catch (error) {
        const c = code(error);
        if (lan >= 2 || (c !== "chat_v2_stale_epoch" && c !== "chat_v2_not_ready")) throw error;
        await this.dongBo(room, () => undefined);
        if (c === "chat_v2_not_ready") await this.chuanBi(room);
        await this.call("abandon_send", { envelope }).catch(() => undefined);
      }
    }
  }
}
