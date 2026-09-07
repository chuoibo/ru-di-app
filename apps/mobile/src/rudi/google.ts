/** Google is an optional native door; an absent configuration is not an error UI. */
export function googleConfigured(platform: string, webClientId?: string, iosClientId?: string): boolean {
  const valid = (value?: string) => /^[a-zA-Z0-9-]+\.apps\.googleusercontent\.com$/.test(value?.trim() ?? "");
  return (platform === "android" || platform === "ios") && valid(webClientId)
    && (platform !== "ios" || valid(iosClientId));
}

type IdentityResponse = { type: "cancelled" } | { type: "success"; data: { idToken: string | null } };

/** Cancellation never reaches our API; Google profile/email are never identity keys. */
export async function googleSession<T>(
  choose: () => Promise<IdentityResponse>,
  exchange: (idToken: string) => Promise<T>,
): Promise<T | null> {
  const response = await choose();
  if (response.type === "cancelled") return null;
  const token = response.data.idToken;
  if (!token) throw new Error("Google chưa trả mã đăng nhập. Hãy thử lại hoặc dùng số điện thoại.");
  return exchange(token);
}
