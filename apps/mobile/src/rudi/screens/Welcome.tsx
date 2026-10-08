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
import Svg, { Path } from "react-native-svg";

import { AssetIllustration } from "../asset-pack/react-native/AssetIllustration";
import { DAU_VAN_CAY } from "../dau-van-cay";
import { giuState } from "../../ui/a11y";
import { displayFace, lopPhu, typography, useRudiTheme } from "../theme";
import { CoverButton } from "../ui/CoverButton";
import { Nep } from "../ui/art/Nep";
import { Grain } from "../ui/Grain";
import { PressScale } from "../ui/PressScale";
import { StampButton } from "../ui/StampButton";
import { useAdaptiveLayout } from "../ui/useAdaptiveLayout";
import { useMotion } from "../ui/useMotion";
import { Logo } from "../ui";

/** The indigo journal cover keeps four accessible chapters and one opening action.
 * Local scenery is illustration; the chapter copy carries the product promise.
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

export function WelcomeScreen() {
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const { width: windowWidth, height: windowHeight, fontScale } = useWindowDimensions();
  const layout = useAdaptiveLayout();
  const motion = useMotion();
  const { brand, colors, dark } = useRudiTheme();
  const pager = useRef<ScrollView>(null);
  const [page, setPage] = useState(0);
  const pageNow = useRef(0);
  const [pageWidth, setPageWidth] = useState(0);
  // The cover art arrives once. Under Reduce Motion it is simply there.
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

  // Android exposes the system's 1.3 setting as a float close to 1.3.
  const short = layout.heightClass === "short" || windowHeight - insets.top - insets.bottom < 660 || Math.round(fontScale * 100) >= 130;
  const compact = layout.sizeClass === "compact";
  const narrow = windowWidth < 360;
  const contentWidth = Math.min(windowWidth - 48, 640);
  const textWidth = short ? contentWidth : Math.round(contentWidth * 0.6);
  const artWidth = Math.round(contentWidth * 0.5);
  const heroHeight = short ? 160 : Math.max(370, Math.min(520, windowHeight - insets.top - insets.bottom - 296));
  const artHeight = heroHeight + 72;
  const scene = page === 3 ? "scene-coast" : page === 1 ? "scene-cafe" : "scene-street";

  return (
    <View role="main" style={[styles.root, { backgroundColor: colors.ground }]} testID="welcome-screen">
      <StatusBar style="light" />
      <Grain material={dark ? "vaiBia" : "giayTrang"} opacity={dark ? 0.3 : 0.45} />
      <View style={[styles.flex, { backgroundColor: colors.cover }]}>
        <Grain material="vaiBia" opacity={0.3} />
        {/* The cover grows for its words. Short windows keep the illustration
            below the chapter, with larger type and controls in the scroll flow. */}
        <ScrollView
          bounces={false}
          contentContainerStyle={[styles.cover, { paddingTop: insets.top + 12, paddingBottom: Math.max(insets.bottom, 16) + 6 }]}
          showsVerticalScrollIndicator={false}
          style={styles.flex}
        >
          <View style={styles.identity}>
            <Logo compact ink={colors.coverInk} />
          </View>
          <View style={[styles.hero, { minHeight: heroHeight }]}>
            {!short ? (
              <Animated.View pointerEvents="none" style={[styles.coverArt, { width: artWidth, height: artHeight }, routeStyle]}>
                <AssetIllustration name={scene} width={artWidth} height={artHeight} fit="cover" />
                <Svg width={72} height={artHeight} style={styles.pageTurn} accessible={false}>
                  <Path d={`M0 0 C8 ${artHeight * 0.32} 64 ${artHeight * 0.66} 8 ${artHeight} L0 ${artHeight} Z`} fill={colors.cover} />
                  <Path d={`M9 0 C17 ${artHeight * 0.32} 73 ${artHeight * 0.66} 17 ${artHeight} L8 ${artHeight} C64 ${artHeight * 0.66} 8 ${artHeight * 0.32} 0 0 Z`} fill={colors.paper} />
                </Svg>
              </Animated.View>
            ) : null}
            <View style={[styles.chapter, { width: textWidth }, narrow && styles.chapterNarrow, short && styles.chapterShort]}>
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
                  <Text style={[styles.pageTitle, narrow && styles.titleNarrow, { color: colors.coverInk }]}>{item.title}</Text>
                  <Text style={[typography.body, styles.pageBody, { color: colors.coverInkSoft }]}>{item.body}</Text>
                </View>
              ))}
            </ScrollView>
              {!short && !narrow ? <AssetIllustration name="spot-memory" width={Math.min(textWidth, 175)} height={116} resolution="small" style={styles.memory} /> : null}
            </View>
            {short ? (
              <Animated.View pointerEvents="none" style={[styles.shortArt, { width: contentWidth }, routeStyle]} testID="welcome-compact-art">
                <AssetIllustration name={scene} width={contentWidth} height={112} fit="cover" />
                <Svg width={96} height={112} style={styles.pageTurn} accessible={false}>
                  {/* repo-guard: allow=long-number reason=synthetic-paper-fold-svg-coordinates */}
                  <Path d="M0 0 L64 0 C88 32 76 80 40 112 L0 112 Z" fill={colors.cover} />
                  {/* repo-guard: allow=long-number reason=synthetic-paper-fold-svg-coordinates */}
                  <Path d="M64 0 C88 32 76 80 40 112 L52 112 C88 80 100 32 76 0 Z" fill={colors.paper} />
                </Svg>
                <Nep pose="moi" size={76} style={styles.shortNep} />
              </Animated.View>
            ) : null}
          </View>
          <View style={[styles.bottom, !compact && styles.bottomWide]}>
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
              <StampButton label="Rủ Đi thôi!" onPress={openCover} testID="welcome-cta" tilt={0} style={styles.openButton} />
              <CoverButton
                icon="chevron-forward"
                label={page < WELCOME_PAGES.length - 1 ? "Tìm hiểu thêm" : "Xem lại từ đầu"}
                onPress={learnMore}
                variant="link"
                style={styles.learnMoreButton}
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
  cover: { flexGrow: 1, paddingHorizontal: 24, width: "100%", maxWidth: 688, alignSelf: "center" },
  identity: { alignItems: "flex-start", paddingTop: 12, zIndex: 1 },
  hero: { position: "relative", flexGrow: 1 },
  coverArt: { position: "absolute", top: -72, right: -24, overflow: "hidden" },
  pageTurn: { position: "absolute", left: 0, top: 0 },
  chapter: { paddingTop: 56 },
  chapterNarrow: { paddingTop: 40 },
  chapterShort: { paddingTop: 24, paddingBottom: 8 },
  shortArt: { height: 112, overflow: "hidden" },
  shortNep: { position: "absolute", left: 4, bottom: 0 },
  memory: { marginTop: 28, transform: [{ rotate: "-5deg" }] },
  bottom: { gap: 4, paddingTop: 8, zIndex: 1 },
  bottomWide: { alignSelf: "center", width: "100%", maxWidth: 640 },
  page: { paddingRight: 8 },
  pageTitle: { fontFamily: displayFace.extraBold, fontSize: 34, lineHeight: 40, letterSpacing: -0.7 },
  titleNarrow: { fontSize: 30, lineHeight: 36 },
  pageBody: { marginTop: 14, maxWidth: 360 },
  dots: { flexDirection: "row", alignItems: "center", justifyContent: "center", gap: 8 },
  dotTarget: { minWidth: 48, minHeight: 48, alignItems: "center", justifyContent: "center" },
  progress: { textAlign: "center", marginTop: -10, marginBottom: 12 },
  dot: { width: 6, height: 6, borderRadius: 3 },
  dotActive: { width: 22, borderRadius: 3 },
  actions: { gap: 8 },
  openButton: { alignSelf: "stretch", width: "100%" },
  learnMoreButton: { width: "100%" },
  dauVanCay: { textAlign: "center", paddingTop: 4 },
});
