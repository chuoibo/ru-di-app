import { Tabs } from "expo-router";

import { RudiTabBar } from "../../src/rudi/ui/RudiTabBar";
import { useAdaptiveLayout } from "../../src/rudi/ui/useAdaptiveLayout";

/** Five destinations, rendered as a bottom strip or an adaptive left rail. */
export default function TabsLayout() {
  const layout = useAdaptiveLayout();
  return (
    <Tabs
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
