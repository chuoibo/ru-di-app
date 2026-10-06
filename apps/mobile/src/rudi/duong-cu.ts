/** Keep a legacy link's resource and context until the live route checks access. */
function query(ctx: unknown, dip?: unknown): string {
  const params: string[] = [];
  if (typeof ctx === "string" && ctx !== "") params.push(`ctx=${encodeURIComponent(ctx)}`);
  if (typeof dip === "string" && dip !== "") params.push(`dip=${encodeURIComponent(dip)}`);
  return params.length ? `?${params.join("&")}` : "";
}

export function duongKeoCu(id: unknown, ctx: unknown): string {
  return typeof id === "string" && id !== ""
    ? `/outings/${encodeURIComponent(id)}${query(ctx)}`
    : "/(tabs)/plan";
}

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function duongChiaBillCu(id: unknown, ctx: unknown, dip: unknown): string {
  // The current review route recognizes outing UUIDs; other historic slugs start a new bill.
  const outing = typeof id === "string" && UUID.test(id) ? id.toLowerCase() : "moi";
  return `/smart-split/${outing}/review${query(ctx, dip)}`;
}
