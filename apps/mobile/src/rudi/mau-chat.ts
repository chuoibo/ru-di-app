/**
 * Chat themes (ADR-0021 §2.4): five closed slugs, colours read from tokens.
 *
 * A leaf module: it imports `tokens.json` and nothing else, so `theme.ts` can
 * re-export it without a cycle and a node test can run it without a renderer.
 * No hex is spelled here -- `tests/rudi-khong-hex.test.mjs` and the repo-root
 * `tests/test_chat_theme_matches_tokens.py` both refuse a fourth copy of a
 * colour -- and an unknown slug falls back to `mac-dinh`, which is the brand
 * accent, so a stale client draws the old bubble rather than nothing.
 */
import tokens from "../../../../packages/shared/tokens.json";

export const THEME_CHAT = ["mac-dinh", "hoang-hon", "bien-dem", "rung-thong", "ruc-ro"] as const;
export type ThemeChat = (typeof THEME_CHAT)[number];

export type BangMauChat = { bubble: string; bubbleInk: string; accent: string };

type BangTheme = Record<string, { light: BangMauChat; dark: BangMauChat }>;

const NHAN: Record<ThemeChat, string> = {
  "mac-dinh": "Mặc định",
  "hoang-hon": "Hoàng hôn",
  "bien-dem": "Biển đêm",
  "rung-thong": "Rừng thông",
  "ruc-ro": "Rực rỡ",
};

export function laThemeChat(value: unknown): value is ThemeChat {
  return typeof value === "string" && (THEME_CHAT as readonly string[]).includes(value);
}

/** The bubble palette for a group's theme in the current scheme; unknown slugs draw the default. */
export function bangMauChat(slug: string | null | undefined, dark: boolean): BangMauChat {
  const bang = tokens.chatTheme as BangTheme;
  // Written as a statement, not `x ?? "mac-dinh"`: the fallback is a product
  // decision (the brand accent), not a raw value quietly standing in for a
  // missing one, and `tests/mac-dinh-am-tham-id.test.mjs` reads it that way.
  let chon: ThemeChat = "mac-dinh";
  if (laThemeChat(slug)) chon = slug;
  const scheme = dark ? "dark" : "light";
  return { ...bang[chon][scheme] };
}

export function nhanTheme(slug: string): string {
  return laThemeChat(slug) ? NHAN[slug] : NHAN["mac-dinh"];
}
