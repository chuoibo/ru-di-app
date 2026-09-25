/**
 * Which avatar each frame shows, kept consistent across readers.
 *
 * The server is the one source of truth: `GET /people/avatars?ids=...` answers
 * each person's current avatar id (or null for none) for the people this
 * reader may see, and `/people/avatars/stream` pushes a hint when one changes.
 * A frame's address carries that id -- `/people/{id}/avatar?v=<avatar id>` --
 * so the address changes exactly when the picture does. That is what makes
 * every cache honest: the browser's HTTP cache (`max-age=300`) and expo-image's
 * native disk cache are both keyed by URL, and a URL that never changed kept
 * showing a replaced picture for minutes on web and indefinitely on a phone
 * (measured on web 2026-09-24: B saw A's old picture until max-age ran out).
 *
 * Rules this module holds, all tested in `tests/anh-dai-dien.test.mjs`:
 *
 *   - **Unknown means ask, not guess.** A person whose version is not known
 *     draws initials and is queued; queued ids go out in one request of at
 *     most 100. Known-null draws initials with no request at all, so a roster
 *     of people without pictures costs one call, not one 404 each.
 *   - **Newer beats older.** A push or one's own upload bumps that person's
 *     generation; a versions answer that was asked for before the bump is
 *     not allowed to overwrite it.
 *   - **The same frame gets the same object.** expo-image on web refetches a
 *     headered source whenever the source OBJECT changes, so an unchanged
 *     address and header set hands back the object handed out last time.
 *   - **A picture that would not load stays initials for that version only.**
 *   - **One account at a time.** A different `actorId` wipes everything.
 *
 * Pure module, no React: `AvatarNguoi` subscribes, the stream feeds it.
 */
import type { ImageSource } from "expo-image";

import { BASE_URL, duongDanAnhDaiDien, translatedAsActor } from "../../api";
import { headerNguoiGoi } from "../../danh-tinh";

/** One versions request asks for at most this many people (the server's limit). */
export const TOI_DA_MOT_LAN = 100;

export type PhienBan = string | null;
export type LayPhienBan = (ids: string[], actorId: string) => Promise<Record<string, PhienBan>>;

async function layTuMayChu(ids: string[], actorId: string): Promise<Record<string, PhienBan>> {
  const tra = await translatedAsActor<{ avatars: Record<string, PhienBan> }>(
    {},
    `/people/avatars?ids=${ids.map(encodeURIComponent).join(",")}`,
    // A hung request would keep these people "being asked" forever; the
    // timeout turns it into a failure, which leaves them unknown and askable.
    { method: "GET", actorId, timeoutMs: 10_000 },
  );
  return tra.avatars ?? {};
}

let lay: LayPhienBan = layTuMayChu;
let hen: (f: () => void) => void = (f) => {
  setTimeout(f, 20);
};

let chu: string | null = null;
const phienBan = new Map<string, PhienBan>();
const doi = new Map<string, number>();
const choHoi = new Set<string>();
const dangHoi = new Set<string>();
const hong = new Set<string>();
const daDua = new Map<string, { khoa: string; nguon: ImageSource }>();
const nghe = new Set<() => void>();
let daHen = false;
let nhip = 0;

function phat(): void {
  nhip += 1;
  for (const f of nghe) f();
}

function theoChu(actorId: string): void {
  if (chu === actorId) return;
  chu = actorId;
  phienBan.clear();
  doi.clear();
  choHoi.clear();
  dangHoi.clear();
  hong.clear();
  daDua.clear();
}

function xin(personId: string): void {
  if (choHoi.has(personId) || dangHoi.has(personId)) return;
  choHoi.add(personId);
  if (daHen) return;
  daHen = true;
  hen(() => {
    daHen = false;
    void guiDi();
  });
}

async function guiDi(): Promise<void> {
  const actorId = chu;
  if (actorId === null) return;
  while (choHoi.size > 0) {
    const lo = [...choHoi].slice(0, TOI_DA_MOT_LAN);
    const truoc = new Map(lo.map((id) => [id, doi.get(id) ?? 0]));
    for (const id of lo) {
      choHoi.delete(id);
      dangHoi.add(id);
    }
    let tra: Record<string, PhienBan> | null = null;
    try {
      tra = await lay(lo, actorId);
    } catch {
      tra = null;
    }
    if (chu !== actorId) return;
    for (const id of lo) {
      dangHoi.delete(id);
      // A failed request leaves the person unknown; the next resync asks again.
      if (tra === null || (doi.get(id) ?? 0) !== truoc.get(id)) continue;
      // Absent means this reader may not see them: initials, same as the image route's 403.
      phienBan.set(id, Object.prototype.hasOwnProperty.call(tra, id) ? tra[id] : null);
    }
    phat();
  }
}

/**
 * The frame source for `personId` as seen by `actorId`, or null for initials.
 * Null while the version is being asked for, for "no picture", for a reader
 * the server does not let see it, and for a version whose picture would not load.
 */
export function nguonAvatar(personId: string | null | undefined, actorId: string | null | undefined): ImageSource | null {
  if (!personId || !actorId) return null;
  theoChu(actorId);
  const v = phienBan.get(personId);
  if (v === undefined) {
    xin(personId);
    return null;
  }
  if (v === null || hong.has(`${personId} ${v}`)) return null;
  const moi: ImageSource = {
    uri: `${BASE_URL}${duongDanAnhDaiDien(personId)}?v=${encodeURIComponent(v)}`,
    headers: headerNguoiGoi(actorId, { roles: "member" }),
  };
  // The bearer is in the headers, so a new sign-in as the same person is a new key too.
  const khoa = `${moi.uri} ${JSON.stringify(moi.headers)}`;
  const cu = daDua.get(personId);
  if (cu !== undefined && cu.khoa === khoa) return cu.nguon;
  daDua.set(personId, { khoa, nguon: moi });
  return moi;
}

/** The picture for this person's current version would not load: initials until it changes. */
export function baoAnhHong(personId: string, actorId: string): void {
  theoChu(actorId);
  const v = phienBan.get(personId);
  if (typeof v !== "string" || hong.has(`${personId} ${v}`)) return;
  hong.add(`${personId} ${v}`);
  phat();
}

/**
 * `personId` has a new picture. With the id the upload answered, every frame
 * switches at once; without it, the person is asked for again.
 */
export function baoDaDoiAnh(personId: string, actorId: string, avatarId?: string | null): void {
  theoChu(actorId);
  doi.set(personId, (doi.get(personId) ?? 0) + 1);
  if (avatarId === undefined) {
    phienBan.delete(personId);
    xin(personId);
  } else {
    phienBan.set(personId, avatarId);
  }
  phat();
}

/** Ask again for everyone this account has drawn: after a reconnect or coming back to the foreground. */
export function lamMoiTatCa(actorId: string): void {
  theoChu(actorId);
  for (const id of phienBan.keys()) xin(id);
}

/** One frame from the stream. Anything unrecognised is ignored, never trusted. */
export function nhanSuKien(suKien: unknown, actorId: string): void {
  if (!suKien || typeof suKien !== "object") return;
  const e = suKien as { type?: unknown; person_id?: unknown; avatar_id?: unknown };
  if (e.type === "ready") {
    lamMoiTatCa(actorId);
    return;
  }
  if (e.type !== "avatar" || typeof e.person_id !== "string") return;
  if (e.avatar_id !== undefined && e.avatar_id !== null && typeof e.avatar_id !== "string") return;
  baoDaDoiAnh(e.person_id, actorId, (e.avatar_id as string | null | undefined) ?? null);
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

/** Test seam: where versions come from and how the batch is deferred. */
export function datNguonPhienBan(f: LayPhienBan, henMoi?: (f: () => void) => void): void {
  lay = f;
  if (henMoi) hen = henMoi;
}
