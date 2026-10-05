/** Google is an optional native door; an absent configuration is not an error UI. */
export function googleConfigured(platform, webClientId, iosClientId) {
    const valid = (value) => /^[a-zA-Z0-9-]+\.apps\.googleusercontent\.com$/.test(value?.trim() ?? "");
    return (platform === "android" || platform === "ios") && valid(webClientId)
        && (platform !== "ios" || valid(iosClientId));
}
/** Cancellation never reaches our API; Google profile/email are never identity keys. */
export async function googleSession(choose, exchange) {
    const response = await choose();
    if (response.type === "cancelled")
        return null;
    const token = response.data.idToken;
    if (!token)
        throw new Error("Google chưa trả mã đăng nhập. Hãy thử lại hoặc dùng số điện thoại.");
    return exchange(token);
}
