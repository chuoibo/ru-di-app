import { useFocusEffect, useRouter } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  type NativeScrollEvent,
  type NativeSyntheticEvent,
  Platform,
  ScrollView,
  StyleSheet,
  Text,
  useWindowDimensions,
  View,
  type ViewStyle,
} from "react-native";
import Animated, { useAnimatedStyle, useSharedValue, withTiming } from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { DAU_VAN_CAY } from "../dau-van-cay";
import { giuState } from "../../ui/a11y";
import { displayFace, lopPhu, mauSang, typography, useRudiTheme } from "../theme";
import { CoverButton } from "../ui/CoverButton";
import { Grain } from "../ui/Grain";
import { RouteLine } from "../ui/RouteLine";
import { PressScale } from "../ui/PressScale";
import { StampButton } from "../ui/StampButton";
import { useAdaptiveLayout } from "../ui/useAdaptiveLayout";
import { useMotion } from "../ui/useMotion";
import { Washi } from "../ui/Washi";
import { Wordmark } from "../ui/Wordmark";

/**
 * The closed cover of the group's travel journal (FIRST VIEWPORT of the v2
 * contract): indigo cloth full-bleed, the wordmark pressed into the upper
 * third, one coral washi strip carrying the positioning line, and the
 * invitation as a coral seal at the foot. Pressing the seal hands the cover
 * to the native page transition onto Login. No photograph: the
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
    title: "Một lời rủ. Nhiều ngày đáng nhớ.",
    body: "Hội bạn hay người thương, từ lúc chưa biết đi đâu đến khi có chuyện mang về.",
  },
  {
    title: "Hẹn ở nơi ai cũng muốn tới",
    body: "Một quán quen, một góc mới. Chọn theo gu, đường đi và khoản cả hội muốn dành.",
  },
  {
    title: "Vui cùng nhau, rõ phần mỗi người",
    body: "Ai dùng món nào, phần người ấy ở đó. Từng đồng rõ ràng, để lời hẹn sau vẫn nhẹ tênh.",
  },
  {
    title: "Đi rồi, còn điều để nhớ",
    body: "Một buổi hẹn thành khoảnh khắc. Những ngày đi xa thành cuốn sổ. Giữ riêng, hoặc mở cho mọi người.",
  },
];

/** One glyph per page above: friends, finding a place, the bill, the memories. */
const CHANG_GLYPHS = ["people", "compass", "receipt", "images"] as const;

export function WelcomeScreen() {
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const { height: windowHeight, fontScale } = useWindowDimensions();
  const layout = useAdaptiveLayout();
  const motion = useMotion();
  const { brand, colors, dark } = useRudiTheme();
  const pager = useRef<ScrollView>(null);
  const [page, setPage] = useState(0);
  const pageNow = useRef(0);
  const [pageWidth, setPageWidth] = useState(0);
  const [routeBox, setRouteBox] = useState({ w: 0, h: 0 });
  const [taglineHeight, setTaglineHeight] = useState(19);
  // The route is drawn once, on the first frame the cover is up: a plan being
  // pencilled in, not a loop. Under Reduce Motion it is simply there.
  const routeIn = useSharedValue(0);
  useEffect(() => {
    routeIn.value = withTiming(1, motion.timing("shared", "decelerate"));
  }, [motion, routeIn]);
  // One press opens one Login. Keep the lock while this page loses focus;
  // reset only on return, so late taps during the native transition do nothing.
  const dangMo = useRef(false);
  useFocusEffect(
    useCallback(() => {
      dangMo.current = false;
      // Prepare the paper page while the cover is still closed. Login makes
      // no authentication request until its user presses a sign-in action.
      router.prefetch("/login");
    }, [router]),
  );

  // Width changes reflow the same chapter instead of changing its meaning.
  useEffect(() => {
    if (pageWidth > 0) pager.current?.scrollTo({ x: pageNow.current * pageWidth, animated: false });
  }, [pageWidth]);

  const onScroll = (event: NativeSyntheticEvent<NativeScrollEvent>) => {
    const next = Math.round(event.nativeEvent.contentOffset.x / Math.max(pageWidth, 1));
    if (next >= 0 && next < WELCOME_PAGES.length && next !== pageNow.current) {
      pageNow.current = next;
      setPage(next);
    }
  };

  const showPage = (requested: number) => {
    const next = Math.max(0, Math.min(WELCOME_PAGES.length - 1, requested));
    pager.current?.scrollTo({ x: next * pageWidth, animated: !motion.reduced });
    if (motion.reduced) { pageNow.current = next; setPage(next); }
  };

  const learnMore = () => showPage(pageNow.current < WELCOME_PAGES.length - 1 ? pageNow.current + 1 : 0);
  const openCover = () => {
    if (dangMo.current) return;
    dangMo.current = true;
    // A single transaction paints the actual next page before moving it.
    // The stack owns reduced motion too; there is no outgoing 3D frame or
    // artificial wait exposing an empty sheet while Login prepares.
    router.push("/login");
  };
  const routeStyle = useAnimatedStyle(() => ({
    opacity: routeIn.value,
    transform: [{ translateY: (1 - routeIn.value) * 12 }],
  }));

  const short = layout.heightClass === "short" || windowHeight - insets.top - insets.bottom < 660 || fontScale >= 1.3;
  const compact = layout.sizeClass === "compact";
  const markHeight = short ? 64 : compact ? 100 : 136;

  return (
    <View role="main" style={[styles.root, { backgroundColor: colors.ground }]} testID="welcome-screen">
      <StatusBar style="light" />
      <Grain material={dark ? "vaiBia" : "giayTrang"} opacity={dark ? 0.3 : 0.45} />
      <View style={[styles.flex, { backgroundColor: colors.cover }]}>
        <Grain material="vaiBia" opacity={0.3} />
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
            <View />
          </View>

          <View style={[styles.mark, short && styles.markShort]}>
            <Wordmark height={markHeight} color={colors.coverInk} />
            <Washi tone="accent" tilt={-2} height={Math.max(34, taglineHeight + 16)} style={styles.tape}>
              {/* Static dark ink: the tape is coral in both schemes, and the scheme's light ink on coral would read 2.4:1. */}
              <Text onLayout={(e) => setTaglineHeight(Math.ceil(e.nativeEvent.layout.height))} style={[styles.tagline, { color: mauSang.ink }]}>Từ lời rủ đến trang kỷ niệm</Text>
            </Washi>
          </View>

          <Animated.View
            onLayout={(e) => setRouteBox({ w: e.nativeEvent.layout.width, h: e.nativeEvent.layout.height })}
            style={[styles.route, short && styles.routeShort, routeStyle]}
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
              testID="welcome-pager"
              role="region"
              accessibilityLabel={`Giới thiệu Rủ Đi, trang ${page + 1} trên ${WELCOME_PAGES.length}`}
              {...(Platform.OS === "web" ? { tabIndex: 0, onKeyDown: (event: { key: string; preventDefault(): void }) => {
                if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
                event.preventDefault();
                showPage(event.key === "Home" ? 0 : event.key === "End" ? WELCOME_PAGES.length - 1 : pageNow.current + (event.key === "ArrowRight" ? 1 : -1));
              } } : {})}
              horizontal
              onLayout={(e) => setPageWidth(e.nativeEvent.layout.width)}
              onScroll={onScroll}
              onMomentumScrollEnd={onScroll}
              scrollEventThrottle={16}
              disableIntervalMomentum
              decelerationRate="fast"
              // Web paging wrappers become the snap targets, so their inner
              // page's snap-stop cannot constrain a fling. Own the web targets.
              pagingEnabled={Platform.OS !== "web"}
              style={Platform.OS === "web" ? ({ scrollSnapType: "x mandatory" } as ViewStyle) : undefined}
              showsHorizontalScrollIndicator={false}
            >
              {WELCOME_PAGES.map((item, index) => (
                <View key={item.title} aria-hidden={index !== page} accessibilityElementsHidden={index !== page} importantForAccessibility={index === page ? "auto" : "no-hide-descendants"} style={[styles.page, { width: pageWidth || undefined }, Platform.OS === "web" && ({ scrollSnapAlign: "start", scrollSnapStop: "always" } as ViewStyle)]}>
                  <Text style={[styles.pageTitle, { color: colors.coverInk }]}>{item.title}</Text>
                  <Text style={[typography.body, styles.pageBody, { color: colors.coverInkSoft }]}>{item.body}</Text>
                </View>
              ))}
            </ScrollView>
            {/* The pager's own indicator: the current page is a short coral bar, the
                same coral as the route's active stop, so the two say one thing. */}
            <View style={styles.dots} role="group" accessibilityLabel="Chọn trang giới thiệu">
              {WELCOME_PAGES.map((item, index) => (
                <PressScale
                  key={item.title}
                  accessibilityLabel={`Trang ${index + 1}: ${item.title}`}
                  accessibilityRole="button"
                  {...giuState(index === page)}
                  haptic="select"
                  onPress={() => showPage(index)}
                  style={styles.dotTarget}
                ><View style={[styles.dot, { backgroundColor: index === page ? brand.coral : lopPhu.trang(0.38) }, index === page && styles.dotActive]} /></PressScale>
              ))}
            </View>
            <Text accessibilityLiveRegion="polite" aria-live="polite" style={[typography.caption, styles.progress, { color: colors.coverInkSoft }]}>Trang {page + 1} trên {WELCOME_PAGES.length}</Text>
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
      </View>
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
  routeShort: { flex: 0, height: 20 },
  tape: { alignSelf: "center", paddingHorizontal: 16, maxWidth: "100%" },
  tagline: { fontFamily: displayFace.bold, fontSize: 15, lineHeight: 19, letterSpacing: -0.1, textAlign: "center" },
  bottom: { gap: 12, marginHorizontal: -20 },
  bottomWide: { alignSelf: "center", width: "100%", maxWidth: 640, marginHorizontal: 0 },
  page: { paddingHorizontal: 20 },
  pageTitle: { fontFamily: displayFace.extraBold, fontSize: 28, lineHeight: 33, letterSpacing: -0.7, minHeight: 68 },
  pageBody: { marginTop: 6, minHeight: 60, maxWidth: 520 },
  dots: { flexDirection: "row", alignItems: "center", justifyContent: "center", gap: 8 },
  dotTarget: { minWidth: 48, minHeight: 48, alignItems: "center", justifyContent: "center" },
  progress: { textAlign: "center", marginTop: -12 },
  dot: { width: 6, height: 6, borderRadius: 3 },
  dotActive: { width: 22, borderRadius: 3 },
  actions: { paddingHorizontal: 20, gap: 10 },
  dauVanCay: { textAlign: "center", paddingTop: 4 },
});
