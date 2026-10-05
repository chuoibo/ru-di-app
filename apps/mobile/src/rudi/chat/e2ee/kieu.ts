/**
 * Chat v2 wire shapes (ADR-0057), as the Go lane and the Rust crate speak them:
 * envelopes carry base64 bytes; an identity card carries its 32-byte keys as
 * arrays of numbers (the Rust IdentityCard).
 */

export type Card = {
  actor_id: string;
  device_id: string;
  mls_signature_key: number[];
  transport_signature_key: number[];
};

export type Envelope = {
  conversation_id: string;
  device_id: string;
  logical_send_id: string;
  protocol: string;
  epoch: number;
  ciphertext: string;
  signature: string;
};

export type CommitBody = { envelope: Envelope; roster: Card[] };

export type Event = {
  sequence: number;
  kind: "envelope" | "mark" | "commit";
  actor_id: string;
  envelope?: Envelope;
  commit?: CommitBody;
  created_at: string;
};

export type Page = { events: Event[]; next_sequence: number; has_more: boolean };

export type RosterView = {
  exists: boolean;
  epoch: number;
  last_sequence: number;
  ready: boolean;
  members: Card[];
  expected: Card[];
};

export type WelcomeView = { id: string; conversation_id: string; sequence: number; welcome: string; roster: Card[] };

export type MediaRef = { media_id: string; key: string; nonce: string; sha256: string; mime: string; size: number };

/** The operations a member sends inside MLS (the Rust Operation). */
export type Operation =
  | { type: "text"; body: string }
  | { type: "reply"; reply_to: string; body: string }
  | { type: "edit"; message_id: string; body: string }
  | { type: "delete"; message_id: string }
  | { type: "reaction"; message_id: string; emoji: string }
  | { type: "vote"; poll_id: string; option_id: string }
  | { type: "image"; media: MediaRef; caption: string | null; width: number; height: number }
  | { type: "sticker"; pack_id: string; sticker_id: string }
  | { type: "voice"; media: MediaRef; duration_ms: number };

/** What the device made of one event it received. */
export type Received =
  | { kind: "application"; actor_id: string; device_id: string; logical_send_id: string; operation: Operation }
  | { kind: "commit"; epoch: number }
  | { kind: "removed" };

/** A commit the device staged: the envelope to post, and a Welcome when it adds devices. */
export type CommitBundle = { envelope: Envelope; welcome: string | null };
