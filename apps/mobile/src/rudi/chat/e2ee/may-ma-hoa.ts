/**
 * One device's end of chat v2 (ADR-0057): enrolment, key packages, Welcomes,
 * keeping a room's MLS roster in line with the server's, reading and sending.
 *
 * What it never does: send plaintext anywhere, decide who belongs in a room
 * (the server attests the roster; the device refuses a commit that does not
 * match it), or hold a key -- the native module seals every change before it
 * answers, so nothing reaches the network that a crash could lose.
 *
 * What it writes down (`so-phong`): every decrypted message together with the
 * cursor past it, in one durable step, before the crypto may forget the
 * envelope (`settle_received`); and every own send before it leaves. A room's
 * work runs one step at a time (`trongPhong`), so two readers never race the
 * same cursor.
 *
 * The crypto and the HTTP lane are ports, so the orchestration is tested
 * against a fake lane and the real crypto (tests/e2ee-*.test.mjs).
 */
import { ApiError } from "../../../api";
import type { ApiV2 } from "./api-v2";
import type { Card, CommitBundle, Envelope, Event, Operation, Received, RosterView } from "./kieu";
import { PHONG_TRONG, canGuiLai, dungPhong, type BanGhi, type Nhan, type SoPhong } from "./so-phong";

export type CryptoPort = {
  open(actorId: string, deviceId: string): Promise<boolean>;
  createGroup(conversationId: string): Promise<string>;
  encrypt(conversationId: string, logicalSendId: string, operationJson: string): Promise<string>;
  receive(envelopeJson: string, rosterJson: string | null): Promise<string>;
  call(method: string, argsJson: string): Promise<string>;
};

/** A small durable key-value store (AsyncStorage in the app): ids and counters, never message content. */
export type KhoPort = { doc(key: string): Promise<string | null>; ghi(key: string, value: string): Promise<void> };

/**
 * A room's sealed record on this device (the native module's room store):
 * `noi` appends records and moves the cursor (when not null) in one durable
 * write, or throws having written nothing.
 */
export type KhoTinPort = {
  doc(room: string): Promise<SoPhong | null>;
  noi(room: string, cursor: number | null, ban: BanGhi[]): Promise<void>;
};

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

/** The crypto's refusal code (ERR_CHAT_CRYPTO_<CODE>), from the native module or the drill. */
function maCrypto(error: unknown): string {
  const e = error as { code?: unknown; message?: unknown } | null;
  const raw = String(e?.code ?? "").startsWith("ERR_CHAT_CRYPTO_") ? String(e?.code) : String(e?.message ?? "");
  return raw.startsWith("ERR_CHAT_CRYPTO_") ? raw.slice("ERR_CHAT_CRYPTO_".length) : "";
}

/**
 * An application envelope that will never open: forged, garbled, or for an
 * epoch that is gone. Skipping it (and saying so) keeps one bad sender from
 * stopping a room for everyone; a commit that will not verify still stops it.
 */
const KHONG_MO_DUOC = new Set(["AUTHENTICATION", "MLS", "INVALID"]);

/** Whether pressing «Thử lại» on a failed send could help. */
function thuLaiDuoc(error: unknown): boolean {
  if (!(error instanceof ApiError)) return maCrypto(error) === "";
  return error.status === 0 || error.status >= 500 || error.status === 408 || error.status === 429 || error.code === "chat_v2_stale_epoch" || error.code === "chat_v2_not_ready";
}

/**
 * A device that joined a room after this phone first saw the room
 * (ADR-0057 §1.3): the server vouches for who a device belongs to, so every
 * newcomer is said out loud in the room -- a ghost device cannot slip in
 * unseen (security review 05/10).
 */
export type ThietBiMoi = { room: string; card: Card };

export class MayMaHoa {
  private device: string | null = null;

  constructor(
    private readonly d: {
      actorId: string;
      crypto: CryptoPort;
      api: ApiV2;
      kho: KhoPort;
      so: KhoTinPort;
      uuid: () => string;
      label: string;
      /** Told of every device that joins a room this phone already knew. */
      onThietBiMoi?: (moi: ThietBiMoi) => void;
      /** Told whenever a room's record grew. */
      onPhong?: (room: string) => void;
    },
  ) {}

  private readonly phong = new Map<string, SoPhong>();
  private readonly hang = new Map<string, Promise<unknown>>();

  /** Runs `f` after every earlier step of the same room. */
  private trongPhong<T>(room: string, f: () => Promise<T>): Promise<T> {
    const sau = (this.hang.get(room) ?? Promise.resolve()).then(f, f);
    this.hang.set(room, sau.catch(() => undefined));
    return sau;
  }

  /** The room's record so far, read from the sealed store once. */
  async soPhong(room: string): Promise<SoPhong> {
    let p = this.phong.get(room);
    if (p === undefined) {
      p = (await this.d.so.doc(room)) ?? PHONG_TRONG;
      if (!this.phong.has(room)) this.phong.set(room, p);
      p = this.phong.get(room) as SoPhong;
    }
    return p;
  }

  /** Appends to the room's record; the cursor only moves forward. */
  private async ghi(room: string, cursor: number | null, ban: BanGhi[]): Promise<void> {
    const cu = await this.soPhong(room);
    const toi = cursor === null || cursor <= cu.cursor ? null : cursor;
    if (toi === null && ban.length === 0) return;
    await this.d.so.noi(room, toi, ban);
    this.phong.set(room, { cursor: toi ?? cu.cursor, ban: [...cu.ban, ...ban] });
    this.d.onPhong?.(room);
  }

  /**
   * Remembers the devices of a room and reports any it had not seen. The
   * first sighting of a room only records it: there is no "before" to compare.
   */
  private async ghiNhanRoster(room: string, roster: Card[]): Promise<void> {
    const khoa = this.khoa(`devices.${room}`);
    const cu = await this.d.kho.doc(khoa);
    const daBiet = new Set<string>(cu === null ? [] : (JSON.parse(cu) as string[]));
    for (const c of roster) {
      if (cu !== null && !daBiet.has(c.device_id) && c.device_id !== this.device) this.d.onThietBiMoi?.({ room, card: c });
      daBiet.add(c.device_id);
    }
    await this.d.kho.ghi(khoa, JSON.stringify([...daBiet]));
  }

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

  /** This phone's device id, once open. */
  async thietBi(): Promise<string> {
    return this.may();
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
      await this.trongPhong(w.conversation_id, async () => {
        try {
          await this.call("join_group", { conversation_id: w.conversation_id, welcome: w.welcome, roster: w.roster });
        } catch (error) {
          // Joined before the app died, Welcome not yet acknowledged: its key
          // package is spent, and the room is already here.
          const co = await this.call<{ conversations: string[] }>("conversations");
          if (!co.conversations.includes(w.conversation_id)) throw error;
        }
        await this.ghiNhanRoster(w.conversation_id, w.roster);
        await this.ghi(w.conversation_id, w.sequence, []);
        await this.d.api.ackWelcome(this.d.actorId, this.may(), w.id);
      });
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
    return this.trongPhong(room, () => this.chuanBiTrong(room));
  }

  private async chuanBiTrong(room: string): Promise<boolean> {
    const device = this.may();
    let r: RosterView = await this.d.api.roster(this.d.actorId, room, device);
    if (!r.exists) {
      r = await this.d.api.bootstrap(this.d.actorId, room, device);
      await this.d.crypto.createGroup(room);
      await this.ghiNhanRoster(room, r.members);
    }
    if (!r.members.some((c) => c.device_id === device)) return false; // waiting for a Welcome
    for (let lan = 0; lan < 3 && !r.ready; lan++) {
      await this.dongBoTrong(room);
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
      const r = await this.d.api.commit(this.d.actorId, room, bundle, added, removed);
      if (r.event.commit !== undefined) await this.ghiNhanRoster(room, r.event.commit.roster);
    } catch (error) {
      if (code(error) === "chat_v2_stale_epoch") {
        await this.call("abandon_commit", { conversation_id: room });
        await this.dongBoTrong(room);
      }
      throw error;
    }
    await this.call("acknowledge_commit", { envelope: bundle.envelope });
  }

  /**
   * Reads the room's new events into its record: every decrypted message with
   * the cursor past it, in one write per page, and only then lets the crypto
   * forget the envelopes. The device's own sends are known from what it wrote
   * before sending; the lane's copy confirms where they landed. A commit by
   * another device is verified against the roster the server attests in it.
   */
  async dongBo(room: string): Promise<SoPhong> {
    return this.trongPhong(room, async () => {
      await this.dongBoTrong(room);
      return this.soPhong(room);
    });
  }

  private async dongBoTrong(room: string): Promise<void> {
    const device = this.may();
    for (;;) {
      const truoc = await this.soPhong(room);
      const page = await this.d.api.events(this.d.actorId, room, device, truoc.cursor);
      const ban: BanGhi[] = [];
      let cursor = Math.max(truoc.cursor, page.next_sequence);
      let mo = false;
      let roi = false;
      for (const e of page.events) {
        if (e.envelope !== undefined && e.envelope.device_id === device) {
          ban.push({ t: "da-gui", id: e.envelope.logical_send_id, seq: e.sequence, at: e.created_at });
        } else if (e.envelope !== undefined) {
          mo = true;
          const r = await this.moThu(e.envelope);
          if (r === null) ban.push({ t: "khong-mo", seq: e.sequence, actor: e.actor_id });
          else if (r.kind === "application") ban.push({ t: "tin", seq: e.sequence, r, at: e.created_at });
        } else if (e.commit !== undefined && e.commit.envelope.device_id !== device) {
          mo = true;
          const r = JSON.parse(await this.d.crypto.receive(JSON.stringify(e.commit.envelope), JSON.stringify(e.commit.roster))) as Received;
          if (r.kind === "removed") {
            roi = true;
            cursor = Math.max(cursor, e.sequence);
            break;
          }
          await this.ghiNhanRoster(room, e.commit.roster);
        }
        cursor = Math.max(cursor, e.sequence);
      }
      await this.ghi(room, cursor, ban);
      if (roi) {
        await this.call("forget", { conversation_id: room });
        return;
      }
      if (mo) await this.call("settle_received", { conversation_id: room });
      if (!page.has_more) return;
    }
  }

  /** Opens one application envelope; null when it never will. */
  private async moThu(envelope: Envelope): Promise<Received | null> {
    try {
      return JSON.parse(await this.d.crypto.receive(JSON.stringify(envelope), null)) as Received;
    } catch (error) {
      if (KHONG_MO_DUOC.has(maCrypto(error))) return null;
      throw error;
    }
  }

  /**
   * Sends one operation: written down first, then encrypted and posted. A
   * moved epoch re-encrypts the same logical send under the new one after
   * reading the commit; a room not yet ready is brought in line first. Never
   * falls back to anything unencrypted. A failure stays in the record as a
   * failed send the person can try again or drop.
   */
  async gui(room: string, operation: Operation, logical: string = this.d.uuid()): Promise<Event | null> {
    return this.trongPhong(room, async () => {
      const r: Nhan = { kind: "application", actor_id: this.d.actorId, device_id: this.may(), logical_send_id: logical, operation };
      await this.ghi(room, null, [{ t: "cho", r, luc: Date.now() }]);
      return this.guiTrong(room, operation, logical);
    });
  }

  /** Tries a failed send again, the same logical send. */
  async thuLai(room: string, logical: string): Promise<Event | null> {
    return this.trongPhong(room, async () => {
      const c = dungPhong((await this.soPhong(room)).ban).dangDi.find((x) => x.id === logical);
      if (c === undefined) return null;
      await this.ghi(room, null, [{ t: "thu", id: logical }]);
      return this.guiTrong(room, c.r.operation, logical);
    });
  }

  /** Drops a send that never reached the lane. */
  async boQua(room: string, logical: string): Promise<void> {
    await this.trongPhong(room, async () => {
      const c = dungPhong((await this.soPhong(room)).ban).dangDi.find((x) => x.id === logical);
      if (c === undefined) return;
      await this.ghi(room, null, [{ t: "bo", id: logical }]);
      try {
        const envelope = JSON.parse(await this.d.crypto.encrypt(room, logical, JSON.stringify(c.r.operation))) as Envelope;
        await this.call("abandon_send", { envelope });
      } catch {
        // Nothing queued in the crypto for it: nothing to drop there.
      }
    });
  }

  /** Sends what was written down and never confirmed (the app died on the way). */
  async guiLai(room: string): Promise<void> {
    await this.trongPhong(room, async () => {
      for (const c of canGuiLai((await this.soPhong(room)).ban)) {
        try {
          await this.guiTrong(room, c.r.operation, c.id);
        } catch {
          // Recorded as failed by guiTrong; the person decides.
        }
      }
    });
  }

  private async guiTrong(room: string, operation: Operation, logical: string): Promise<Event | null> {
    for (let lan = 0; ; lan++) {
      let envelope: Envelope;
      try {
        envelope = JSON.parse(await this.d.crypto.encrypt(room, logical, JSON.stringify(operation))) as Envelope;
      } catch (error) {
        // Already accepted by the lane before the app died: reading the room
        // confirms where it landed.
        if (maCrypto(error) === "CONFLICT") return null;
        await this.ghi(room, null, [{ t: "hong", id: logical, loi: "Thiết bị chưa mã hoá được tin này.", thuLai: false }]);
        throw error;
      }
      try {
        const r = await this.d.api.send(this.d.actorId, room, envelope);
        await this.call("acknowledge_sent", { envelope });
        await this.ghi(room, null, [{ t: "da-gui", id: logical, seq: r.event.sequence, at: r.event.created_at }]);
        return r.event;
      } catch (error) {
        const c = code(error);
        if (lan >= 2 || (c !== "chat_v2_stale_epoch" && c !== "chat_v2_not_ready")) {
          await this.ghi(room, null, [{ t: "hong", id: logical, loi: error instanceof ApiError ? error.message : "Chưa gửi được.", thuLai: thuLaiDuoc(error) }]);
          throw error;
        }
        await this.dongBoTrong(room);
        if (c === "chat_v2_not_ready") await this.chuanBiTrong(room);
        await this.call("abandon_send", { envelope }).catch(() => undefined);
      }
    }
  }
}
