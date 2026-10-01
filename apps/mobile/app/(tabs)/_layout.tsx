import { Tabs } from "expo-router";

import { RudiTabBar } from "../../src/rudi/ui/RudiTabBar";
import { useAdaptiveLayout } from "../../src/rudi/ui/useAdaptiveLayout";

/**
 * Five destinations, rendered as a bottom strip or an adaptive left rail.
 *
 * `backBehavior="history"`: Back returns to the tab the person came from. The
 * default («firstRoute») kept only [first tab, current tab], so on the web two
 * moves between tabs other than the first never grew the browser history and
 * Back left the app altogether once Cộng đồng became the first tab (QA UI-123).
 * On Android the system Back now walks the tabs the person visited, then exits.
 */
export default function TabsLayout() {
  const layout = useAdaptiveLayout();
  return (
    <Tabs
      backBehavior="history"
      screenOptions={{ headerShown: false, tabBarPosition: layout.rail ? "left" : "bottom" }}
      tabBar={(props) => <RudiTabBar {...props} />}
    >
      <Tabs.Screen name="community" options={{ title: "Cộng đồng" }} />
      <Tabs.Screen name="explore" options={{ title: "Khám phá" }} />
      <Tabs.Screen name="plan" options={{ title: "Lên plan" }} />
      <Tabs.Screen name="messages" options={{ title: "Tin nhắn" }} />
      <Tabs.Screen name="profile" options={{ title: "Cá nhân" }} />
    </Tabs>
  );
}
