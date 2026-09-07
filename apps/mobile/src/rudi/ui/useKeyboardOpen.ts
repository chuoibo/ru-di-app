import { useEffect, useState } from "react";
import { Keyboard, Platform } from "react-native";

/**
 * Whether the software keyboard is up.
 *
 * A form under a cover band has two jobs at once: say who is asking (the
 * band) and take the answer (the field). When the IME opens on a compact
 * phone the band can no longer afford its full height, so the screens that own
 * a cover read this and let the band contract; the field and its action stay
 * above the keyboard instead of below the fold. Android fires `keyboardDid*`
 * only; iOS gets the `Will*` pair so the band moves with the keyboard, not
 * after it.
 */
export function useKeyboardOpen(): boolean {
  const [open, setOpen] = useState(false);
  useEffect(() => {
    const show = Platform.OS === "ios" ? "keyboardWillShow" : "keyboardDidShow";
    const hide = Platform.OS === "ios" ? "keyboardWillHide" : "keyboardDidHide";
    const a = Keyboard.addListener(show, () => setOpen(true));
    const b = Keyboard.addListener(hide, () => setOpen(false));
    return () => {
      a.remove();
      b.remove();
    };
  }, []);
  return open;
}
