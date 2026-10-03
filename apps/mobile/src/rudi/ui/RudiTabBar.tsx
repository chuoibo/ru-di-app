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
import { dichCuaCot, oCuaCot, xepThanh } from "./thanh-tab";
import { useAdaptiveLayout } from "./useAdaptiveLayout";
import { useMotion } from "./useMotion";
import { Wordmark } from "./Wordmark";

type TabBarProps = Parameters<NonNullable<ComponentProps<typeof Tabs>["tabBar"]>>[0];

/** Icon per column, filled when current. */
const ICONS: Record<string, [keyof typeof Ionicons.glyphMap, keyof typeof Ionicons.glyphMap]> = {
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
/** The wordmark's row at the head of the rail, above the stamp (owner's mockup, 01/10). */
const HANG_LOGO_RAIL = 56;

/**
 * The notebook's edge strip: four columns and the «Tạo mới» stamp in the
 * middle of the five slots, between where you look and plan (Khám phá, Lên
 * plan) and where you talk and keep (Tin nhắn, Cá nhân); on a tablet the same
 * as a rail, the wordmark and then the stamp at its head. The arithmetic is
 * `thanh-tab.ts`: a route with no column (Cộng đồng, Khám phá's second
 * section) lights its host column, and the Khám phá column reopens the
 * section last in view.
 *
 * The destinations are a `tablist` of real tabs (`aria-selected`, QA UI-003).
 * The stamp is a button, which a tablist may not own, so the strip keeps an
 * empty, hidden column for it and the stamp is laid over that column from
 * outside the list. Signed out, every tab is the sign-in door, so the strip
 * never needs a seventh column (QA UI-082; a seventh made each column 46 dp
 * at 320 dp). The active strip
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
  const tabDangMo = routes[state.index]?.name ?? "";
  const thanh = xepThanh(
    routes.map((r) => r.name),
    tabDangMo,
    layout.rail,
  );
  // The stamp's slot: the middle of five on the strip, the head of the rail.
  const { viTriDau, soCot: columns } = thanh;
  // The indicator follows the lit column, not the route index: on Cộng đồng
  // (a route with no column) the Khám phá column stays lit.
  const viTriSang = Math.max(thanh.cotChon, 0);

  // Where the rail's rows begin: its own top padding. The indicator is laid
  // out from the rail's edge, and measuring its rows from 0 put it beside the
  // wrong tab (QA UI-004).
  const dauRail = insets.top + 12;
  // On the strip the indicator moves by slot (oCuaCot counts the stamp's), so
  // a move past the stamp slides over it instead of jumping a slot at the end;
  // the rail lists the columns under the stamp, so there it moves by column.
  const viTriVach = layout.rail ? viTriSang : oCuaCot(thanh, viTriSang);
  const indicator = useSharedValue(viTriVach);
  useEffect(() => {
    indicator.value = withTiming(viTriVach, motion.timing("standard"));
  }, [viTriVach, indicator, motion]);

  const indicatorStyle = useAnimatedStyle(() => {
    if (layout.rail) return { transform: [{ translateY: dauRail + HANG_LOGO_RAIL + HANG_DAU_RAIL + indicator.value * HANG_RAIL }] };
    return { left: `${(indicator.value / columns) * 100}%` as const };
  });

  const items = thanh.cot.flatMap((ten, index) => {
    const route = routes.find((r) => r.name === ten);
    if (!route) return [];
    const { options } = descriptors[route.key];
    const focused = thanh.cotChon === index;
    const [outline, filled] = ICONS[route.name] ?? ["ellipse-outline", "ellipse"];
    const label = typeof options.title === "string" ? options.title : route.name;
    const onPress = () => {
      // The Khám phá column reopens the section last in view; a lit column
      // (Cộng đồng open counts as Khám phá) does not navigate: its screen
      // hears the press and goes back to its top (`useChamLaiTab`).
      const dich = dichCuaCot(route.name);
      const dichRoute = routes.find((r) => r.name === dich) ?? route;
      const event = navigation.emit({ type: "tabPress", target: dichRoute.key, canPreventDefault: true });
      if (!focused && !event.defaultPrevented) {
        motion.haptic.select();
        navigation.navigate(dichRoute.name);
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
        testID="vach-thanh"
        style={[
          layout.rail ? styles.railIndicator : styles.indicator,
          layout.rail ? { backgroundColor: colors.accent, width: 4 } : { width: `${100 / columns}%` },
          indicatorStyle,
        ]}
      >
        {layout.rail ? null : <View style={[styles.tape, { backgroundColor: colors.accent }]} />}
      </Animated.View>
      {/* The rail's head: the wordmark, outside the list, above the stamp. */}
      {layout.rail ? (
        <View style={styles.logoRail}>
          <Wordmark color={colors.ink} height={18} />
        </View>
      ) : null}
      <View {...TABLIST} style={layout.rail ? styles.danhSachRail : styles.danhSach}>
        {items}
      </View>
      {/* Over the empty slot, from outside the list. */}
      <View
        pointerEvents="box-none"
        style={
          layout.rail
            ? [styles.dauRail, { top: dauRail + HANG_LOGO_RAIL }]
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
  logoRail: { height: HANG_LOGO_RAIL, alignItems: "center", justifyContent: "center" },
  label: { fontSize: 12, lineHeight: 14, textAlign: "center" },
  dauThanh: { position: "absolute", top: 0, alignItems: "stretch" },
  dauRail: { position: "absolute", left: 0, right: 0, height: HANG_DAU_RAIL, alignItems: "stretch" },
  indicator: { position: "absolute", top: 0, height: 6, alignItems: "center", backgroundColor: "transparent" },
  tape: { width: 28, height: 4, borderBottomLeftRadius: 4, borderBottomRightRadius: 4 },
  railIndicator: { position: "absolute", left: 0, top: 0, height: HANG_RAIL },
});
