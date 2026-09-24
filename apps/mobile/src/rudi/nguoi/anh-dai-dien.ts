/**
 * Which avatar frames may ask the server, and what they ask for.
 *
 * Every live screen that shows a person draws the same frame: the photograph
 * that person uploaded (M8), or their initials. The address never changes
 * (`/people/{id}/avatar`), and the server answers 404 for "no picture yet" and
 * 403 for "you share no group with them" -- both ordinary. So two things have
 * to live somewhere shared rather than in each screen:
 *
 *   - **A refusal is remembered.** A roster of thirty people, most without a
 *     photograph, would otherwise send thirty requests on every render and
 *     every scroll back. A refused frame stays initials for `NHO_HONG_MS`, then
 *     may ask again (somebody may have uploaded since).
 *   - **An upload is announced.** The address is stable, so a frame already
 *     pointed at it keeps drawing the old bytes out of cache. `baoDaDoiAnh`
 *     bumps a per-person counter that goes into the query string, and every
 *     mounted frame hears about it -- not only the settings screen that did
 *     the upload.
 *   - **The same frame gets the same object.** expo-image on web fetches a
 *     headered source in an effect keyed on the source OBJECT, so a fresh
 *     `{uri, headers}` per render is a fresh request per render (measured: 43
 *     avatar requests for an eight-person roster). An unchanged address and
 *     header set hands back the object it handed out last time.
 *
 * All of it belongs to one signed-in account: a different `actorId` wipes the
 * memory, because what one account may see says nothing about another's.
 *
 * Pure module, no React: `AvatarNguoi` subscribes to it, the node suite runs it.
 */
import type { ImageSource } from "expo-image";

import { nguonAnhDaiDien } from "./anh-ca-nhan";

/** How long a refused frame stays initials before it may ask again. */
export const NHO_HONG_MS = 10 * 60 * 1000;

let chu: string | null = null;
const hongLuc = new Map<string, number>();
const phienBan = new Map<string, number>();
const daDua = new Map<string, { khoa: string; nguon: ImageSource }>();
const nghe = new Set<() => void>();
let nhip = 0;

function phat(): void {
  nhip += 1;
  for (const f of nghe) f();
}

/** Another account (or none) wipes what the previous one learnt. */
function theoChu(actorId: string): void {
  if (chu === actorId) return;
  chu = actorId;
  hongLuc.clear();
  phienBan.clear();
  daDua.clear();
}

/**
 * The frame source for `personId` as seen by `actorId`, or null for initials.
 *
 * Null without asking when either id is missing or the server refused this
 * frame recently; otherwise the address with this person's upload counter.
 */
export function nguonAvatar(
  personId: string | null | undefined,
  actorId: string | null | undefined,
  bayGio: number = Date.now(),
): ImageSource | null {
  if (!personId || !actorId) return null;
  theoChu(actorId);
  const luc = hongLuc.get(personId);
  if (luc !== undefined) {
    if (bayGio - luc < NHO_HONG_MS) return null;
    hongLuc.delete(personId);
  }
  const moi = nguonAnhDaiDien(personId, actorId, phienBan.get(personId) ?? 0);
  // The bearer is in the headers, so a new sign-in as the same person is a new key too.
  const khoa = `${moi.uri} ${JSON.stringify(moi.headers)}`;
  const cu = daDua.get(personId);
  if (cu !== undefined && cu.khoa === khoa) return cu.nguon;
  daDua.set(personId, { khoa, nguon: moi });
  return moi;
}

/** The frame for `personId` would not load (404, 403, network): draw initials. */
export function baoAnhHong(personId: string, actorId: string, bayGio: number = Date.now()): void {
  theoChu(actorId);
  if (hongLuc.has(personId)) return;
  hongLuc.set(personId, bayGio);
  phat();
}

/** `personId` has a new picture: forget any refusal and bust every frame's cache. */
export function baoDaDoiAnh(personId: string, actorId: string): void {
  theoChu(actorId);
  hongLuc.delete(personId);
  phienBan.set(personId, (phienBan.get(personId) ?? 0) + 1);
  phat();
}

/** Subscribe to changes; returns the unsubscribe. Shape of `useSyncExternalStore`. */
export function dangKyAnhDaiDien(f: () => void): () => void {
  nghe.add(f);
  return () => {
    nghe.delete(f);
  };
}

/** Changes on every announcement; the snapshot `useSyncExternalStore` compares. */
export function nhipAnhDaiDien(): number {
  return nhip;
}
