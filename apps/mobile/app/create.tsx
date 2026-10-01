import { useLocalSearchParams, useNavigation, useRouter } from "expo-router";
import { useEffect } from "react";

import { CreateSheet } from "../src/rudi/screens/Create";
import { tabTu } from "../src/rudi/tao-moi";

/**
 * The create sheet opens over the tab its stamp was pressed on, which it
 * names as `?tu=` so the desk can put that tab's own kind of thing first.
 * Reached cold (a deep link, a notification) there is nothing under the
 * transparent route but grey, so the shell is put in place first -- the tab
 * named by `?tu=`, else Khám phá -- and the sheet re-opened over it.
 */
export default function CreateRoute() {
  const router = useRouter();
  const navigation = useNavigation();
  const tu = tabTu(useLocalSearchParams<{ tu?: string }>().tu);
  const lanh = !navigation.canGoBack();
  useEffect(() => {
    if (!lanh || dangMoLanh) return;
    dangMoLanh = true;
    // Spelled out, not `/${tu}`: the guide's extractor reads this route's exits
    // from literal strings (`tools/rut-huong-dan.mjs`), and every one is a tab.
    router.replace(
      (tu === "community" ? "/community" : tu === "plan" ? "/plan" : tu === "messages" ? "/messages" : tu === "profile" ? "/profile" : "/explore") as never,
    );
    // Not cleared on unmount: the replace above unmounts this very route, and
    // clearing the timer in that cleanup is what left a cold /create on Khám
    // phá with no tray at all (QA UI-010). The flag keeps a re-run effect
    // (Strict Mode) from pushing twice.
    setTimeout(() => {
      dangMoLanh = false;
      router.push((tu ? `/create?tu=${tu}` : "/create") as never);
    }, 0);
  }, [lanh, router, tu]);
  if (lanh) return null;
  return <CreateSheet />;
}

/** A cold open is putting the shell in place; one at a time. */
let dangMoLanh = false;
