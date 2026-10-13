/** A finite casual draw, interpreted only after MLS authenticates its author. */
export type RandomMode = "wheel" | "ducks";

/** A place ID is sender-supplied attribution, never server-verified evidence. */
export type RandomOption = { label: string; placeId?: string };

export type RandomRound = {
  v: 1;
  id: string;
  request: string;
  mode: RandomMode;
  options: RandomOption[];
  seed: string;
  winner: number;
};

type RandomInput = Omit<RandomRound, "v" | "winner">;

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const SEED = /^[0-9a-f]{32}$/;
const MARKER = "\n/rudi-random-v1 ";
const WORD_RANGE = 0x100000000;
const MAX_BODY = 6000;
const MAX_BYTES = 12 * 1024;
// Keep the readable fallback a single unambiguous line, including bidi controls.
const CONTROLS = /[\p{Cc}\p{Cf}\p{Cs}\u2028\u2029]/u;

function record(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function exactKeys(value: Record<string, unknown>, required: readonly string[], optional: readonly string[] = []): boolean {
  const keys = Object.keys(value);
  return required.every((key) => Object.hasOwn(value, key)) &&
    keys.every((key) => required.includes(key) || optional.includes(key));
}

function text(value: unknown, max: number, canonical: boolean, nonempty = false): string | null {
  if (typeof value !== "string" || CONTROLS.test(value)) return null;
  const normalized = value.trim().normalize("NFC");
  return normalized.length > max || (nonempty && normalized.length === 0) || (canonical && normalized !== value)
    ? null : normalized;
}

/** Expand Unicode caseless forms without a locale-dependent comparison. */
function labelKey(label: string): string {
  return [...label.toLowerCase()].map((letter) => letter === "\u0131" ? letter : letter.toUpperCase().toLowerCase()).join("").normalize("NFC");
}

function options(value: unknown, canonical: boolean): RandomOption[] | null {
  if (!Array.isArray(value) || value.length < 2 || value.length > 8) return null;
  const result: RandomOption[] = [];
  const labels = new Set<string>();
  for (const item of value) {
    if (!record(item) || !exactKeys(item, ["label"], ["placeId"])) return null;
    const label = text(item.label, 80, canonical, true);
    if (label === null || labels.has(labelKey(label))) return null;
    if (Object.hasOwn(item, "placeId") && (typeof item.placeId !== "string" || !UUID.test(item.placeId))) return null;
    labels.add(labelKey(label));
    result.push({ label, ...(typeof item.placeId === "string" ? { placeId: item.placeId } : {}) });
  }
  return result;
}

/** Four big-endian uint32 words; reject the remainder instead of biasing modulo. */
export function randomWinner(seed: string, count: number): number | null {
  if (typeof seed !== "string" || !SEED.test(seed) || !Number.isInteger(count) || count < 2 || count > 8) return null;
  const limit = Math.floor(WORD_RANGE / count) * count;
  for (let offset = 0; offset < seed.length; offset += 8) {
    const word = Number.parseInt(seed.slice(offset, offset + 8), 16);
    if (word < limit) return word % count;
  }
  return null;
}

function readRound(value: unknown, canonical: boolean): RandomRound | null {
  if (!record(value) || !exactKeys(value, ["v", "id", "request", "mode", "options", "seed", "winner"]) ||
      value.v !== 1 || typeof value.id !== "string" || !UUID.test(value.id) ||
      (value.mode !== "wheel" && value.mode !== "ducks") || typeof value.seed !== "string") return null;
  const request = text(value.request, 300, canonical);
  const choices = options(value.options, canonical);
  if (request === null || choices === null) return null;
  const winner = randomWinner(value.seed, choices.length);
  if (winner === null || value.winner !== winner) return null;
  return { v: 1, id: value.id, request, mode: value.mode, options: choices, seed: value.seed, winner };
}

/** The caller supplies CSPRNG words; this codec never creates fresh draw entropy. */
export function createRandomRound(input: RandomInput): RandomRound {
  if (!record(input) || !exactKeys(input, ["id", "request", "mode", "options", "seed"])) throw new Error("Invalid random round");
  const choices = options(input.options, false);
  const winner = choices === null ? null : randomWinner(input.seed, choices.length);
  const round = readRound({ ...input, options: choices, v: 1, winner }, false);
  if (round === null) throw new Error("Invalid random round");
  return round;
}

function serialized(round: RandomRound): string {
  return JSON.stringify({ v: round.v, id: round.id, request: round.request, mode: round.mode, options: round.options, seed: round.seed, winner: round.winner });
}

function fallback(round: RandomRound): string {
  return `${round.mode === "wheel" ? "Vòng quay" : "Đua vịt"} chọn: ${round.options[round.winner].label}`;
}

function inBounds(body: string): boolean {
  if (body.length > MAX_BODY) return false;
  let bytes = 0;
  for (const character of body) {
    const point = character.codePointAt(0)!;
    bytes += point <= 0x7f ? 1 : point <= 0x7ff ? 2 : point <= 0xffff ? 3 : 4;
    if (bytes > MAX_BYTES) return false;
  }
  return true;
}

/** Stable readable text for old clients, followed by the exact canonical snapshot. */
export function encodeRandomRound(round: RandomRound): string {
  const checked = readRound(round, true);
  if (checked === null) throw new Error("Invalid random round");
  const body = fallback(checked) + MARKER + serialized(checked);
  if (!inBounds(body)) throw new Error("Random round is too large");
  return body;
}

/** Fail closed on malformed/noncanonical JSON, changed results, or fallback mismatch. */
export function decodeRandomRound(body: string): RandomRound | null {
  if (typeof body !== "string" || !inBounds(body)) return null;
  const marker = body.indexOf(MARKER);
  if (marker < 0) return null;
  try {
    const json = body.slice(marker + MARKER.length);
    const round = readRound(JSON.parse(json), true);
    if (round === null || body.slice(0, marker) !== fallback(round) || json !== serialized(round)) return null;
    return round;
  } catch {
    return null;
  }
}
