import { Ionicons } from "@expo/vector-icons";
import { BlurView } from "expo-blur";
import { Tabs } from "expo-router";
import type { ComponentProps } from "react";
import { useEffect } from "react";
import { Platform, Pressable, StyleSheet, Text, View, useWindowDimensions } from "react-native";
import Animated, { useAnimatedStyle, useSharedValue, withTiming } from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { TAB_BAR_HEIGHT, tabBarHeight } from "../adaptive";
import { typography, useRudiTheme } from "../theme";
import { TABLIST, tabState } from "../../ui/a11y";
import { laPair } from "../nhan-rieng/nhan-rieng";
import { useRudiSession } from "../session";
import { ConDauTao } from "./ConDauTao";
import { useAdaptiveLayout } from "./useAdaptiveLayout";
import { useMotion } from "./useMotion";

type TabBarProps = Parameters<NonNullable<ComponentProps<typeof Tabs>["tabBar"]>>[0];

/** Icon per destination, filled when current. */
const ICONS: Record<string, [keyof typeof Ionicons.glyphMap, keyof typeof Ionicons.glyphMap]> = {
  community: ["people-outline", "people"],
  explore: ["compass-outline", "compass"],
  plan: ["map-outline", "map"],
  messages: ["chatbubbles-outline", "chatbubbles"],
  profile: ["person-circle-outline", "person-circle"],
};

export { TAB_BAR_HEIGHT };
export const RAIL_WIDTH = 104;
/** Height of one rail row (a destination, or the stamp's slot at the top). */
const HANG_RAIL = 72;
const HANG_DAU_RAIL = 96;

/**
 * The notebook's edge strip: five destinations and the «Tạo mới» stamp
 * between where you look (Cộng đồng, Khám phá) and where you keep (Lên plan,
 * Tin nhắn, Cá nhân); on a tablet the same as a rail, the stamp at its head.
 *
 * The destinations are a `tablist` of real tabs (`aria-selected`, QA UI-003).
 * The stamp is a button, which a tablist may not own, so the strip keeps an
 * empty, hidden column for it and the stamp is laid over that column from
 * outside the list. Signed out, the demo's way in is the «Dữ liệu demo» badge
 * of each demo screen (a door), so the strip never needs a seventh column
 * (QA UI-082; a seventh made each column 46 dp at 320 dp). The active strip
 * follows reduced-motion preferences.
 */
export function RudiTabBar({ state, descriptors, navigation }: TabBarProps) {
  const { colors, dark } = useRudiTheme();
  const insets = useSafeAreaInsets();
  const layout = useAdaptiveLayout();
  const { fontScale } = useWindowDimensions();
  const motion = useMotion();
  const { phien } = useRudiSession();
  const coCap = (phien?.contexts ?? []).some((nhom) => laPair(nhom) && nhom.my_state === "active");

  const routes = state.routes;
  const count = routes.length;
  // The stamp's column: third on the strip, the head of the rail.
  const viTriDau = layout.rail ? 0 : Math.min(2, count);
  const columns = count + 1;
  const tabDangMo = routes[state.index]?.name ?? "";

  // Where the rail's rows begin: its own top padding. The indicator is laid
  // out from the rail's edge, and measuring its rows from 0 put it beside the
  // wrong tab (QA UI-004).
  const dauRail = insets.top + 12;
  const indicator = useSharedValue(state.index);
  useEffect(() => {
    indicator.value = withTiming(state.index, motion.timing("standard"));
  }, [state.index, indicator, motion]);

  const indicatorStyle = useAnimatedStyle(() => {
    if (layout.rail) return { transform: [{ translateY: dauRail + HANG_DAU_RAIL + indicator.value * HANG_RAIL }] };
    const column = indicator.value >= viTriDau ? indicator.value + 1 : indicator.value;
    return { left: `${(column / columns) * 100}%` as const };
  });

  const items = routes.map((route, index) => {
    const { options } = descriptors[route.key];
    const focused = state.index === index;
    const [outline, filled] = ICONS[route.name] ?? ["ellipse-outline", "ellipse"];
    const label = typeof options.title === "string" ? options.title : route.name;
    const onPress = () => {
      const event = navigation.emit({ type: "tabPress", target: route.key, canPreventDefault: true });
      if (!focused && !event.defaultPrevented) {
        motion.haptic.select();
        navigation.navigate(route.name);
      }
    };
    return (
      <Pressable
        key={route.key}
        {...tabState(focused)}
        accessibilityLabel={label}
        onPress={onPress}
        style={layout.rail ? styles.railItem : styles.item}
      >
        <Ionicons color={focused ? colors.accent : colors.inkFaint} name={focused ? filled : outline} size={24} />
        {/* Two lines before an ellipsis: at 2.0 «Khám …» stopped naming the
            tab (review 08/09 F04); the bar grows with the text (`tabBarHeight`). */}
        <Text numberOfLines={2} style={[typography.caption, styles.label, { color: focused ? colors.accent : colors.inkFaint }]}>
          {label}
        </Text>
      </Pressable>
    );
  });

  // The stamp's slot inside the list: empty and hidden, so the list owns tabs only.
  items.splice(
    viTriDau,
    0,
    <View aria-hidden importantForAccessibility="no-hide-descendants" key="o-dau" pointerEvents="none" style={layout.rail ? styles.oDauRail : styles.item} />,
  );

  const bottom = Math.max(insets.bottom, 10);
  const glass = Platform.OS === "ios" && !layout.rail;

  const bar = (
    <View
      style={[
        layout.rail ? styles.rail : styles.bar,
        {
          backgroundColor: glass ? "transparent" : colors.card,
          borderColor: colors.line,
          ...(layout.rail
            ? { width: RAIL_WIDTH, paddingTop: dauRail, paddingBottom: Math.max(insets.bottom, 12) }
            : { height: tabBarHeight(fontScale) + bottom, paddingBottom: bottom }),
        },
      ]}
    >
      {glass ? <BlurView intensity={78} tint={dark ? "dark" : "light"} style={StyleSheet.absoluteFill} /> : null}
      <Animated.View
        pointerEvents="none"
        style={[
          layout.rail ? styles.railIndicator : styles.indicator,
          layout.rail ? { backgroundColor: colors.accent, width: 4 } : { width: `${100 / columns}%` },
          indicatorStyle,
        ]}
      >
        {layout.rail ? null : <View style={[styles.tape, { backgroundColor: colors.accent }]} />}
      </Animated.View>
      <View {...TABLIST} style={layout.rail ? styles.danhSachRail : styles.danhSach}>
        {items}
      </View>
      {/* Over the empty slot, from outside the list. */}
      <View
        pointerEvents="box-none"
        style={
          layout.rail
            ? [styles.dauRail, { top: dauRail }]
            : [styles.dauThanh, { left: `${(viTriDau / columns) * 100}%`, width: `${100 / columns}%`, bottom }]
        }
      >
        <ConDauTao coCap={coCap} rail={layout.rail} tab={tabDangMo} />
      </View>
    </View>
  );
  return bar;
}

const styles = StyleSheet.create({
  bar: {
    flexDirection: "row",
    alignItems: "stretch",
    borderTopWidth: StyleSheet.hairlineWidth,
  },
  rail: {
    flexDirection: "column",
    alignItems: "stretch",
    borderRightWidth: StyleSheet.hairlineWidth,
    height: "100%",
  },
  danhSach: { flex: 1, flexDirection: "row", alignItems: "stretch" },
  danhSachRail: { flexDirection: "column", alignItems: "stretch" },
  item: { flex: 1, minHeight: 48, alignItems: "center", justifyContent: "center", gap: 2, paddingTop: 8 },
  // No `flex` here: react-native-web writes `flex: 0` as `flex-basis: 0`,
  // which outranks `height`, so every rail tab was 48 dp on the web while the
  // indicator stepped by 72 (QA UI-004).
  railItem: { height: HANG_RAIL, alignItems: "center", justifyContent: "center", gap: 2 },
  oDauRail: { height: HANG_DAU_RAIL },
  label: { fontSize: 12, lineHeight: 14, textAlign: "center" },
  dauThanh: { position: "absolute", top: 0, alignItems: "stretch" },
  dauRail: { position: "absolute", left: 0, right: 0, height: HANG_DAU_RAIL, alignItems: "stretch" },
  indicator: { position: "absolute", top: 0, height: 6, alignItems: "center", backgroundColor: "transparent" },
  tape: { width: 28, height: 4, borderBottomLeftRadius: 4, borderBottomRightRadius: 4 },
  railIndicator: { position: "absolute", left: 0, top: 0, height: HANG_RAIL },
});
