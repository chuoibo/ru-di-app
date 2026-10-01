/**
 * The browser's Back, for whatever layer is on top: a `Sheet`, or a tray a
 * screen draws itself (the chat's tools and shared-plan trays).
 *
 * One listener for all of them, added when this module loads, which is before
 * the router mounts and adds its own. Listeners on `window` run in the order
 * they were added -- the capture flag does not move one ahead of another at
 * the target -- so a listener a layer added when it opened ran AFTER the
 * router's: the router had already left the screen, and the layer closed only
 * because its screen was gone (QA UI-117, UI-038: Back from a tray in a chat
 * landed on Tin nhắn). Running first, this stops the router from seeing that
 * Back, puts the entry the person was on back on top (same state, same url),
 * and closes the top layer. Nothing is pushed when a layer opens: an extra
 * entry is buried the moment the layer's own action navigates, and becomes a
 * Back that does nothing.
 */
import { Platform } from "react-native";

type MucLui = { giu: { state: unknown; url: string }; dong: () => void };

const ngan: MucLui[] = [];

if (Platform.OS === "web" && typeof window !== "undefined") {
  window.addEventListener(
    "popstate",
    (event) => {
      const tren = ngan.at(-1);
      if (!tren) return;
      event.stopImmediatePropagation();
      window.history.pushState(tren.giu.state, "", tren.giu.url);
      tren.dong();
    },
    true,
  );
}

/**
 * Register an open layer; the returned function unregisters it. `dong` is
 * read at Back time, so a caller may pass a ref-backed function.
 */
export function dangKyLuiWeb(dong: () => void): () => void {
  if (Platform.OS !== "web" || typeof window === "undefined") return () => undefined;
  const muc: MucLui = { giu: { state: window.history.state as unknown, url: window.location.href }, dong };
  ngan.push(muc);
  return () => {
    const i = ngan.indexOf(muc);
    if (i >= 0) ngan.splice(i, 1);
  };
}
