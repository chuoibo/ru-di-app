/** Public OSM-ish coordinates for the fixture Đà Lạt catalogue, not GPS.
 *
 * Kept out of `fixtures.ts` so the pure journey layer never imports
 * `expo-image`. Numbers are of the places, rounded to public map precision.
 */

import type { ChoChieu } from "./mo-hinh";

export const TOA_DO_MAU: Record<string, { lat: number; lng: number }> = {
  "banh-can-le": { lat: 11.9419, lng: 108.4376 },
  "ho-tuyen-lam-dem": { lat: 11.8967, lng: 108.4403 },
  "cho-dem": { lat: 11.9414, lng: 108.4372 },
  "doi-thien-phuc": { lat: 11.9806, lng: 108.4481 },
  "still-cafe": { lat: 11.9478, lng: 108.4383 },
  "lau-ga-la-e": { lat: 11.9432, lng: 108.4358 },
};

/** Đà Lạt centre when a day has no mapped stop (do not plot HCMC). */
export const TAM_DA_LAT = { lat: 11.9404, lng: 108.4583 };

export function choTuId(places: readonly { id: string; name: string; address?: string | null }[]): ChoChieu[] {
  return places.flatMap((p) => {
    const t = TOA_DO_MAU[p.id];
    if (!t) return [];
    return [{ id: p.id, name: p.name, lat: t.lat, lng: t.lng, address: p.address ?? null }];
  });
}
