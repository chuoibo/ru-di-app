import { Tabs } from "expo-router";

import { RudiTabBar } from "../../src/rudi/ui/RudiTabBar";
import { useAdaptiveLayout } from "../../src/rudi/ui/useAdaptiveLayout";

/**
 * Four columns and the «Tạo» stamp in the middle, rendered as a bottom strip or
 * an adaptive left rail (owner's mockup, 01/10: an even column count had no
 * middle). Cộng đồng is a tab route with no column — Khám phá's second
 * section, `href: null` here and hosted by the Khám phá column in
 * `src/rudi/ui/thanh-tab.ts` — so it keeps the strip, its URL and its links.
 *
 * `backBehavior="history"`: Back returns to the tab the person came from. The
 * default («firstRoute») kept only [first tab, current tab], so on the web two
 * moves between tabs other than the first never grew the browser history and
 * Back left the app altogether (QA UI-123). On Android the system Back now
 * walks the tabs and sections the person visited, then exits.
 */
export default function TabsLayout() {
  const layout = useAdaptiveLayout();
  return (
    <Tabs
      backBehavior="history"
      screenOptions={{ headerShown: false, tabBarPosition: layout.rail ? "left" : "bottom" }}
      tabBar={(props) => <RudiTabBar {...props} />}
    >
      <Tabs.Screen name="explore" options={{ title: "Khám phá" }} />
      <Tabs.Screen name="community" options={{ href: null, title: "Cộng đồng" }} />
      <Tabs.Screen name="plan" options={{ title: "Lên plan" }} />
      <Tabs.Screen name="messages" options={{ title: "Tin nhắn" }} />
      <Tabs.Screen name="profile" options={{ title: "Cá nhân" }} />
    </Tabs>
  );
}
