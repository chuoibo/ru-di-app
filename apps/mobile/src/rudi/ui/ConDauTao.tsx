/**
 * «Tạo mới», the coral stamp of the tab strip, on every tab.
 *
 * The stamp is a slot of its own, raised out of the strip's top edge the way a
 * rubber stamp stands on the page, in the middle of five equal slots: four
 * columns (Khám phá, Lên plan | Tin nhắn, Cá nhân) and the one thing that
 * makes something new between them (owner's mockup, 01/10; until then five
 * columns put it third of six, off centre). Cộng đồng is Khám phá's second
 * section, a route with no column (`thanh-tab.ts`).
 *
 * It knows the tab it was pressed on:
 *
 * - a press opens the desk with that tab's own kind of thing first (`?tu=`,
 *   `tao-moi.ts`): a post on Cộng đồng, an outing on Lên plan, a memory on
 *   Cá nhân;
 * - a long press goes straight to that thing, for the person who already
 *   knows; the accessibility action says the same in words;
 * - a press leaves the stamp's mark: one ink ring that opens and fades over
 *   `standard`, the only motion here, and none under Reduce Motion.
 *
 * On the rail it is the first item, labelled, under nothing but the top inset.
 */
import { Ionicons } from "@expo/vector-icons";
import { useRouter } from "expo-router";
import { StyleSheet, Text, View } from "react-native";
import Animated, { useAnimatedStyle, useSharedValue, withSequence, withTiming } from "react-native-reanimated";

import { ACTIONS } from "../screens/Create";
import { tabTu, thuTuViec } from "../tao-moi";
import { typography, useRudiTheme } from "../theme";
import { PressScale } from "./PressScale";
import { useMotion } from "./useMotion";

const DAU = 56;

export function ConDauTao({ tab, rail = false, coCap = false }: { tab: string; rail?: boolean; coCap?: boolean }) {
  const { colors, brand } = useRudiTheme();
  const router = useRouter();
  const motion = useMotion();
  const tu = tabTu(tab);
  const dauTien = thuTuViec({ viec: ACTIONS, tu, coCap, coCongDong: true }).viec[0];
  const vet = useSharedValue(0);

  const dong = () => {
    // The mark: a ring at the stamp's edge that opens to 1.4x as it fades.
    if (!motion.reduced) vet.value = withSequence(withTiming(1, { duration: 0 }), withTiming(0, motion.timing("standard", "decelerate")));
  };
  const mo = () => {
    dong();
    motion.haptic.impact();
    router.push((tu ? `/create?tu=${tu}` : "/create") as never);
  };
  const thangToi = () => {
    if (!dauTien) return mo();
    motion.haptic.success();
    router.push(dauTien.href as never);
  };

  const vetStyle = useAnimatedStyle(() => ({
    opacity: vet.value * 0.5,
    transform: [{ scale: 1.4 - vet.value * 0.4 }],
  }));

  return (
    // One button for the stamp and its word: a tap on «Tạo» opens the desk as
    // a tap on the coral does, the way each neighbouring column opens on its
    // word (critique 02/10). Pressed, the whole column gives a little; the
    // stamp sits near its middle, so it barely moves.
    <PressScale
      accessibilityActions={dauTien ? [{ name: "longpress", label: dauTien.title }] : undefined}
      accessibilityHint={dauTien ? `Giữ để ${dauTien.title.toLowerCase()} ngay` : undefined}
      accessibilityLabel="Tạo mới"
      accessibilityRole="button"
      aria-haspopup="dialog"
      haptic="none"
      onAccessibilityAction={(e) => {
        if (e.nativeEvent.actionName === "longpress") thangToi();
      }}
      onLongPress={thangToi}
      onPress={mo}
      pressedScale={0.92}
      style={rail ? styles.cotRail : styles.cot}
      testID="con-dau-tao"
    >
      <View style={[styles.oDau, rail ? null : styles.noiLen]}>
        <Animated.View pointerEvents="none" style={[styles.vet, { borderColor: brand.coral }, vetStyle]} />
        <View style={[styles.dau, { backgroundColor: brand.coral, borderColor: colors.ground, shadowColor: colors.accent }]}>
          <Ionicons color={brand.coralInk} name="add" size={30} />
        </View>
      </View>
      {/* Named like its neighbours, so the row reads as one strip of words. */}
      <Text importantForAccessibility="no" numberOfLines={1} style={[typography.caption, styles.nhan, { color: colors.accent }]}>
        {/* One stamp, one name on the strip and the rail (owner's mockup,
            01/10); a screen reader hears «Tạo mới» from the button itself. */}
        Tạo
      </Text>
    </PressScale>
  );
}

const styles = StyleSheet.create({
  // Laid out like a tab column (centred, 8 dp top padding) with the stamp
  // counting as a 24 dp icon, so «Tạo» sits on the same line as the other
  // labels; the rest of the stamp stands above the strip (`noiLen`).
  cot: { flex: 1, minHeight: 48, alignItems: "center", justifyContent: "center", gap: 2, paddingTop: 8 },
  // No `flex` on the rail (react-native-web turns `flex: 0` into a zero basis
  // that outranks `height`; see RudiTabBar).
  cotRail: { height: 96, alignItems: "center", justifyContent: "center", gap: 2 },
  oDau: { width: DAU, height: DAU, alignItems: "center", justifyContent: "center" },
  // Out of the strip's top edge by all but an icon's height: the stamp stands
  // on the page above, its foot where the other columns draw their icon.
  noiLen: { marginTop: -(DAU - 24) },
  dau: {
    width: DAU,
    height: DAU,
    borderRadius: DAU / 2,
    borderWidth: 4,
    alignItems: "center",
    justifyContent: "center",
    elevation: 6,
    shadowOffset: { width: 0, height: 6 },
    shadowOpacity: 0.22,
    shadowRadius: 10,
  },
  vet: { position: "absolute", width: DAU, height: DAU, borderRadius: DAU / 2, borderWidth: 2 },
  nhan: { fontSize: 12, lineHeight: 14, textAlign: "center" },
});
