/* `expo-router` for the node test build.
 *
 * `useTinNhan.ts` imports `useFocusEffect` from expo-router, and the real one
 * needs a navigation container that does not exist under node. This stub is
 * NOT a mock of navigation: it has exactly one switch, `CAU_HINH.focus`, which
 * decides whether the focus effect runs at all.
 *
 * Off by default, on purpose. With no DOM, react-native-web's
 * `AppState.addEventListener` returns `undefined`, so the hook's own cleanup
 * (`sub.remove()`) would throw the moment the effect ran. A test that needs the
 * focused path (the read mark, the poll) flips the switch AND patches `AppState`
 * first; the rest of the hook is exercised with the screen "not in front",
 * which is also where every late-response race in F42 lives.
 */
import { useEffect } from "react";

export const CAU_HINH = { focus: false };

export function useFocusEffect(effect) {
  useEffect(() => (CAU_HINH.focus ? effect() : undefined), [effect]);
}
