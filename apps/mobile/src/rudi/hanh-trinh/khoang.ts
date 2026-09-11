/** Integer metres and urban-motorbike seconds. No places, no fetch. */

export type ToaDo = { lat: number; lng: number };

const BAN_KINH_DAT_M = 6_371_000;
/** Default movement between stops: 25 km/h, typical urban motorbike. */
const VAN_TOC_M_GIO = 25_000;

function toRad(deg: number): number {
  return (deg * Math.PI) / 180;
}

export function haversineMet(a: ToaDo, b: ToaDo): number {
  const dLat = toRad(b.lat - a.lat);
  const dLng = toRad(b.lng - a.lng);
  const lat1 = toRad(a.lat);
  const lat2 = toRad(b.lat);
  const h =
    Math.sin(dLat / 2) ** 2 + Math.cos(lat1) * Math.cos(lat2) * Math.sin(dLng / 2) ** 2;
  return Math.round(2 * BAN_KINH_DAT_M * Math.asin(Math.min(1, Math.sqrt(h))));
}

export function giayTuMet(met: number): number {
  if (met <= 0) return 0;
  return Math.round((met * 3600) / VAN_TOC_M_GIO);
}
