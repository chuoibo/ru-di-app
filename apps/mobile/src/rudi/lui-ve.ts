/**
 * «Quay lại» that always goes somewhere.
 *
 * `router.back()` with nothing behind it is a no-op: a screen opened cold from
 * a link -- a shared post, an invite, a pasted address -- has an empty stack,
 * and its chevron did nothing at all (QA UI-018: «Quay lại» on a cold `/login`
 * stood still). With history, Back is history; without it, the screen goes to
 * the tab its route belongs to, so the person lands where that thing lives.
 * `diary/DiaryScreen.tsx` already did this by hand for one case.
 */

/** The tab each top-level route belongs to; anything unlisted goes to the app's door. */
const TAB_CUA: Record<string, string> = {
  community: "/community",
  posts: "/community",
  places: "/explore",
  destinations: "/explore",
  "ai-match": "/explore",
  outings: "/plan",
  trips: "/plan",
  votes: "/plan",
  "check-ins": "/plan",
  batches: "/plan",
  settlements: "/plan",
  "smart-split": "/plan",
  groups: "/messages",
  people: "/messages",
  friends: "/messages",
  "hai-nguoi": "/messages",
  stories: "/messages",
  diaries: "/profile",
  moments: "/profile",
  achievements: "/profile",
  finance: "/profile",
  settings: "/profile",
};

/** Where Back goes when there is no history: the tab of the route's first segment. */
export function duongCha(pathname: string): string {
  const dau = pathname.split("?")[0]?.split("/").filter(Boolean)[0] ?? "";
  return TAB_CUA[dau] ?? "/";
}

type RouterLui = { canGoBack: () => boolean; back: () => void; replace: (href: never) => void };

/** Back through history when there is any, else to `cha` (default: the route's own tab). */
export function luiVe(router: RouterLui, pathname: string, cha?: string): void {
  if (router.canGoBack()) router.back();
  else router.replace((cha ?? duongCha(pathname)) as never);
}

/** For a screen that knows its parent: Back through history, else to `cha`. */
export function luiVeVe(router: RouterLui, cha: string): void {
  if (router.canGoBack()) router.back();
  else router.replace(cha as never);
}
