/**
 * A person's own photographs (ADR-0022 §2.1): upload, and the frame source.
 *
 * `POST /people/me/photos` stores a picture that only its owner may read until
 * a post showing it is readable; `GET /people/{id}/photos/{photo}` is the one
 * gate. A post's picture may also be a group photograph (only on a post
 * addressed to that group), so `nguonAnhBai` reads the url's shape and sends
 * the headers that gate expects -- one `BASE_URL`, never two.
 */
import type { ImageSource } from "expo-image";

import { BASE_URL, taiAnhCaNhanLen, type AnhDaTai } from "../../api";
import { headerNguoiGoi } from "../../danh-tinh";

const UUID = "[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}";
const ANH_NGUOI = new RegExp(`^/people/(${UUID})/photos/(${UUID})$`);
const ANH_NHOM = new RegExp(`^/contexts/(${UUID})/photos/(${UUID})$`);

/** The literal the server-routes gate reads: a personal photo's address. */
export function duongDanAnhNguoi(personId: string, photoId: string): string {
  return `/people/${personId}/photos/${photoId}`;
}

export function laAnhNguoi(url: string): boolean {
  return ANH_NGUOI.test(url);
}

export function laAnhNhom(url: string): boolean {
  return ANH_NHOM.test(url);
}

/** The group a group photograph belongs to, from its url; null for any other shape. */
export function nhomCuaAnh(url: string): string | null {
  const m = ANH_NHOM.exec(url);
  if (m === null) return null;
  return m[1];
}

/**
 * A frame source for a post's picture, or null when there is nothing to draw.
 * Relative urls only: the server never hands out absolute ones, and an
 * absolute url here would be a request to somebody else's host with this
 * reader's bearer attached.
 */
export function nguonAnhBai(imageUrl: string | null | undefined, actorId: string): ImageSource | null {
  if (imageUrl === null || imageUrl === undefined || imageUrl === "") return null;
  if (laAnhNguoi(imageUrl)) {
    return { uri: BASE_URL + imageUrl, headers: headerNguoiGoi(actorId, { roles: "member" }) };
  }
  const nhom = nhomCuaAnh(imageUrl);
  if (nhom !== null) {
    return { uri: BASE_URL + imageUrl, headers: headerNguoiGoi(actorId, { roles: "member", contexts: nhom }) };
  }
  return null;
}

/** Upload one's own picture; the answer carries the url a post may show. */
export async function taiAnhCaNhan(photo: { uri: string }, actorId: string): Promise<AnhDaTai> {
  return taiAnhCaNhanLen(photo, actorId);
}
