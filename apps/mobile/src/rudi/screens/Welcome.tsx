import { useFocusEffect, useRouter } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  type NativeScrollEvent,
  type NativeSyntheticEvent,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from "react-native";
import Animated, { useAnimatedStyle, useSharedValue, withTiming } from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { CUA_FIXTURE_DEV } from "../cua-fixture";
import { DAU_VAN_CAY } from "../dau-van-cay";
import { displayFace, lopPhu, mauSang, typography, useRudiTheme } from "../theme";
import { DemoBadge } from "../ui";
import { CoverButton } from "../ui/CoverButton";
import { Grain } from "../ui/Grain";
import { RouteLine } from "../ui/RouteLine";
import { StampButton } from "../ui/StampButton";
import { useAdaptiveLayout } from "../ui/useAdaptiveLayout";
import { useMotion } from "../ui/useMotion";
import { Washi } from "../ui/Washi";
import { Wordmark } from "../ui/Wordmark";

/**
 * The closed cover of the group's travel journal (FIRST VIEWPORT of the v2
 * contract): indigo cloth full-bleed, the wordmark pressed into the upper
 * third, one coral washi strip carrying the positioning line, and the
 * invitation as a coral seal at the foot. Pressing the seal lifts the cover
 * (`shared`, 300ms) and opens onto the bright Login page. No photograph: the
 * cover is the brand, and a stock photo of strangers was never ours to show.
 *
 * ## One line leads
 *
 * The 2026-09-06 review read the first cut as five voices at once: a very
 * large wordmark, the tape, the route, the page title and the body. The page
 * title is the sentence the person is meant to remember, so it is the one
 * thing at reading size; the wordmark is smaller than it was, the tape is a
 * caption-scale line, and the route draws itself in once and then stays
 * still. Nothing loops. On a short window the route is the first thing to go,
 * before any word or button; at a large font scale the cover scrolls instead
 * of clipping its own seal.
 */

export const WELCOME_PAGES = [
  {
    title: "Hẹn hội bạn. Rủ Đi lo phần còn lại.",
    body: "Khám phá, lên plan, chia bill và giữ trọn mọi kỷ niệm trong một nơi.",
  },
  {
    title: "Tìm nơi hợp cả hội",
    body: "Gợi ý theo gu nhóm, khoảng cách và ngân sách. Bạn luôn được sửa trước khi chốt.",
  },
  {
    title: "Chia bill từng đồng",
    body: "Gán món, xem ai nợ ai. Quyết toán và tài chính đọc cùng một sổ.",
  },
  {
    title: "Giữ kỷ niệm của hội",
    body: "Tường riêng, album chuyến đi, check-in khi tới nơi. Đây là không gian của nhóm bạn, không phải mạng xã hội mở.",
  },
];

/** One glyph per page above: friends, finding a place, the bill, the memories. */
const CHANG_GLYPHS = ["people", "compass", "receipt", "images"] as const;

export function WelcomeScreen() {
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const layout = useAdaptiveLayout();
  const motion = useMotion();
  const { brand, colors } = useRudiTheme();
  const pager = useRef<ScrollView>(null);
  const [page, setPage] = useState(0);
  const [pageWidth, setPageWidth] = useState(0);
  const [routeBox, setRouteBox] = useState({ w: 0, h: 0 });
  const lift = useSharedValue(0);
  // The route is drawn once, on the first frame the cover is up: a plan being
  // pencilled in, not a loop. Under Reduce Motion it is simply there.
  const routeIn = useSharedValue(0);
  useEffect(() => {
    routeIn.value = withTiming(1, motion.timing("shared", "decelerate"));
  }, [motion, routeIn]);
  // One press opens one Login: a second tap inside the 300ms lift used to queue a
  // second push. Reset when the cover regains focus (back from Login), together
  // with the lift, so the cover is whole again instead of staying faded.
  const dangMo = useRef(false);
  useFocusEffect(
    useCallback(() => {
      dangMo.current = false;
      lift.value = 0;
    }, [lift]),
  );

  const onScroll = (event: NativeSyntheticEvent<NativeScrollEvent>) => {
    const next = Math.round(event.nativeEvent.contentOffset.x / Math.max(pageWidth, 1));
    if (next !== page && next >= 0 && next < WELCOME_PAGES.length) setPage(next);
  };

  const learnMore = () => {
    const next = page < WELCOME_PAGES.length - 1 ? page + 1 : 0;
    pager.current?.scrollTo({ x: next * pageWidth, animated: !motion.reduced });
    if (motion.reduced) setPage(next);
  };

  const openCover = () => {
    if (dangMo.current) return;
    dangMo.current = true;
    // The cover lifts before the page shows; under Reduce Motion the route
    // changes at once. `router.push` waits on the animation, never on data.
    const ms = motion.ms("shared");
    lift.value = withTiming(1, motion.timing("shared", "accelerate"));
    setTimeout(() => router.push("/login"), ms);
  };

  const coverStyle = useAnimatedStyle(() => ({
    opacity: 1 - lift.value * 0.35,
    transform: [{ translateY: -lift.value * 48 }],
  }));
  const routeStyle = useAnimatedStyle(() => ({
    opacity: routeIn.value,
    transform: [{ translateY: (1 - routeIn.value) * 12 }],
  }));

  const short = layout.heightClass === "short";
  const compact = layout.sizeClass === "compact";
  const markHeight = short ? 64 : compact ? 100 : 136;

  return (
    <View style={[styles.root, { backgroundColor: colors.cover }]} testID="welcome-screen">
      <StatusBar style="light" />
      <Grain material="vaiBia" opacity={0.3} />
      <Animated.View style={[styles.flex, coverStyle]}>
        {/* A scroll view with a growing content box: at font scale 1.0 the route
            absorbs the slack and nothing moves; at 2.0 the words and the seal
            keep their size and the cover scrolls. */}
        <ScrollView
          bounces={false}
          contentContainerStyle={[styles.cover, { paddingTop: insets.top + 12, paddingBottom: Math.max(insets.bottom, 16) + 6 }]}
          showsVerticalScrollIndicator={false}
          style={styles.flex}
        >
          <View style={styles.top}>
            {CUA_FIXTURE_DEV ? <DemoBadge label="Bản trải nghiệm" /> : <View />}
          </View>

          <View style={[styles.mark, short && styles.markShort]}>
            <Wordmark height={markHeight} color={colors.coverInk} />
            <Washi tone="accent" tilt={-2} height={34} style={styles.tape}>
              {/* Static dark ink: the tape is coral in both schemes, and the scheme's light ink on coral would read 2.4:1. */}
              <Text style={[styles.tagline, { color: mauSang.ink }]}>AI đi chơi, chia bill thông minh</Text>
            </Washi>
          </View>

          <Animated.View
            onLayout={(e) => setRouteBox({ w: e.nativeEvent.layout.width, h: e.nativeEvent.layout.height })}
            style={[styles.route, routeStyle]}
            pointerEvents="none"
          >
            {routeBox.h > 72 && !short ? (
              <RouteLine
                width={routeBox.w}
                height={routeBox.h}
                color={colors.coverInk}
                opacity={0.72}
                stops={WELCOME_PAGES.length}
                activeStop={page}
                glyphs={CHANG_GLYPHS}
                activeColor={brand.coral}
                activeInk={mauSang.ink}
                accessibilityLabel={`Chặng ${page + 1} trên ${WELCOME_PAGES.length}`}
              />
            ) : null}
          </Animated.View>

          <View style={[styles.bottom, !compact && styles.bottomWide]}>
            <ScrollView
              ref={pager}
              horizontal
              onLayout={(e) => setPageWidth(e.nativeEvent.layout.width)}
              onMomentumScrollEnd={onScroll}
              pagingEnabled
              showsHorizontalScrollIndicator={false}
            >
              {WELCOME_PAGES.map((item) => (
                <View key={item.title} style={{ width: pageWidth || undefined, paddingHorizontal: 20 }}>
                  <Text style={[styles.pageTitle, { color: colors.coverInk }]}>{item.title}</Text>
                  <Text style={[typography.body, styles.pageBody, { color: colors.coverInkSoft }]}>{item.body}</Text>
                </View>
              ))}
            </ScrollView>
            {/* The pager's own indicator: the current page is a short coral bar, the
                same coral as the route's active stop, so the two say one thing. */}
            <View style={styles.dots} accessibilityLabel={`Trang ${page + 1} trên ${WELCOME_PAGES.length}`}>
              {WELCOME_PAGES.map((item, index) => (
                <View
                  key={item.title}
                  style={[styles.dot, { backgroundColor: index === page ? brand.coral : lopPhu.trang(0.38) }, index === page && styles.dotActive]}
                />
              ))}
            </View>
            <View style={styles.actions}>
              <StampButton label="Rủ Đi thôi!" onPress={openCover} testID="welcome-cta" tilt={-3} />
              <CoverButton
                icon="chevron-forward"
                label={page < WELCOME_PAGES.length - 1 ? "Tìm hiểu thêm" : "Xem lại từ đầu"}
                onPress={learnMore}
                variant="link"
              />
              {DAU_VAN_CAY ? (
                // Native gate anchor (NEO 2b): the harness inlines a per-run value and
                // asserts it on screen. Absent outside the harness, so nothing ships.
                <Text accessibilityLabel="dau-van-cay" style={[typography.caption, styles.dauVanCay, { color: colors.coverInkSoft }]}>
                  {DAU_VAN_CAY}
                </Text>
              ) : null}
            </View>
          </View>
        </ScrollView>
      </Animated.View>
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  flex: { flex: 1 },
  cover: { flexGrow: 1, paddingHorizontal: 20 },
  top: { flexDirection: "row", alignItems: "center", justifyContent: "flex-end", minHeight: 28 },
  mark: { alignItems: "center", gap: 18, marginTop: 28 },
  markShort: { gap: 10, marginTop: 8 },
  // Never wider than a page: on a tablet the S-curve across 1600px read as a wire.
  // `minHeight: 0` lets it give way first when the words need the room.
  route: { flex: 1, minHeight: 0, marginVertical: 8, marginHorizontal: 12, alignSelf: "center", width: "100%", maxWidth: 560 },
  tape: { alignSelf: "center", paddingHorizontal: 16 },
  tagline: { fontFamily: displayFace.bold, fontSize: 15, lineHeight: 19, letterSpacing: -0.1 },
  bottom: { gap: 12, marginHorizontal: -20 },
  bottomWide: { alignSelf: "center", width: "100%", maxWidth: 640, marginHorizontal: 0 },
  pageTitle: { fontFamily: displayFace.extraBold, fontSize: 28, lineHeight: 33, letterSpacing: -0.7, minHeight: 68 },
  pageBody: { marginTop: 6, minHeight: 60, maxWidth: 520 },
  dots: { flexDirection: "row", alignItems: "center", justifyContent: "center", gap: 7 },
  dot: { width: 6, height: 6, borderRadius: 3 },
  dotActive: { width: 22, borderRadius: 3 },
  actions: { paddingHorizontal: 20, gap: 10 },
  dauVanCay: { textAlign: "center", paddingTop: 4 },
});
