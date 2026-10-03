import { NavigationContext } from "expo-router/build/react-navigation/core/NavigationContext";
import { useContext, useEffect, useRef } from "react";

/**
 * Runs `khiCham` when the person taps the tab-strip column of the screen that
 * is already open — the column that is lit (owner's choice, 02/10: a re-tap
 * goes back to the top, as in the phone's own apps).
 *
 * The strip (`RudiTabBar`) emits `tabPress` at the route a column opens
 * (`dichCuaCot`: on Cộng đồng the Khám phá column opens Cộng đồng) and
 * navigates only when that route is not the open one, so a press that finds
 * its screen focused is a re-tap. Whether it was focused is read when the
 * press arrives; the work waits a frame, so a listener that prevents the
 * default still wins. Null opts out, and so does a screen outside a tab
 * navigator (a stack page), where nothing emits the event.
 */
export function useChamLaiTab(khiCham: (() => void) | null): void {
  const navigation = useContext(NavigationContext);
  const goi = useRef(khiCham);
  goi.current = khiCham;
  const bat = khiCham !== null;
  useEffect(() => {
    if (!bat || !navigation || navigation.getState()?.type !== "tab") return undefined;
    // `tabPress` belongs to tab navigators, so it is not in the base event map.
    return navigation.addListener("tabPress" as never, ((e: { defaultPrevented?: boolean }) => {
      const dangMo = navigation.isFocused();
      requestAnimationFrame(() => {
        if (dangMo && !e.defaultPrevented) goi.current?.();
      });
    }) as never);
  }, [navigation, bat]);
}
