/** An image already authenticated and opened by the native crypto module.
 * This source never carries bearer headers and never resolves a network URL.
 * Legacy/server images keep their separate URL and authorization policy.
 */
export function nguonAnhNiemDaMo(raw: string | null): { uri: string } | null {
  if (raw === null || raw.length > 35_000_000) return null;
  const head = /^data:image\/(?:jpeg|png|webp);base64,/.exec(raw);
  if (!head) return null;
  const body = raw.slice(head[0].length);
  if (body.length === 0 || body.length % 4 !== 0 || !/^[A-Za-z0-9+/]+={0,2}$/.test(body)) return null;
  return { uri: raw };
}
