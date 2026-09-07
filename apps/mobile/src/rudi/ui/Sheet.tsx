import { useEffect, useState, type ReactNode } from "react";
import { BackHandler, Platform, Pressable, ScrollView, StyleSheet, View, useWindowDimensions, type StyleProp, type ViewStyle } from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import Animated, { runOnJS, useAnimatedStyle, useSharedValue, withSpring, withTiming } from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { lopPhu, useRudiTheme } from "../theme";
import { useMotion } from "./useMotion";

export interface SheetProps {
  open: boolean;
  onClose: () => void;
  /** After the close animation has finished; a route that hosts the sheet navigates back from here. */
  onClosed?: () => void;
  children: ReactNode;
  /** Announced to assistive tech when the sheet opens. */
  accessibilityLabel: string;
  style?: StyleProp<ViewStyle>;
  /** Tallest the panel may grow (dp) before its content scrolls; default 82% of the window. */
  maxHeight?: number;
  testID?: string;
}

/** Past this drag (dp) or this speed (dp/s) a release closes the sheet. */
const KEO_DONG_DP = 90;
const KEO_DONG_TOC = 900;

/**
 * A bottom sheet on the UI thread: scrim fades over `standard`, the panel
 * springs up and settles, Back and the scrim both close it, and the handle is
 * a real one: dragging it follows the finger and a release past `KEO_DONG_DP`
 * (or a flick) closes the sheet, a shorter release springs it back. The report
 * ruled that a handle may not imply drag-to-dismiss it does not do. The grab
 * zone is the handle row, not the whole panel, so the content's own scroll
 * never fights the drag. Not a modal route, so it can live inside a screen
 * (create menu, picker, filters) and be driven by state; `app/create.tsx`
 * hosts it in a transparent route. Under Reduce Motion the spring resolves
 * instantly.
 */
export function Sheet({ open, onClose, onClosed, children, accessibilityLabel, style, maxHeight, testID }: SheetProps) {
  const { colors, radius, space } = useRudiTheme();
  const insets = useSafeAreaInsets();
  const { height: windowHeight } = useWindowDimensions();
  const tran = maxHeight ?? Math.round(windowHeight * 0.82);
  const motion = useMotion();
  // Starts closed even when mounted open, so a sheet that arrives with its
  // screen (or inside a Modal) still slides in instead of appearing cut.
  const progress = useSharedValue(0);
  // How far the handle has been dragged down; the panel follows it.
  const keo = useSharedValue(0);
  // Mounted while open, and until the close animation has finished: React
  // state, not a shared value read during render (Reanimated strict mode).
  const [hien, setHien] = useState(open);

  const dongXong = () => {
    setHien(false);
    onClosed?.();
  };

  useEffect(() => {
    if (open) {
      setHien(true);
      keo.value = 0;
      progress.value = withSpring(1, motion.spring.settle);
      return;
    }
    progress.value = withTiming(0, motion.timing("standard"), (finished) => {
      if (finished) runOnJS(dongXong)();
    });
    // `dongXong` is recreated per render; the callback captured here is the
    // one from the render that closed the sheet, which is the one wanted.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, motion, progress, keo]);

  useEffect(() => {
    if (!open || Platform.OS !== "android") return;
    const sub = BackHandler.addEventListener("hardwareBackPress", () => {
      onClose();
      return true;
    });
    return () => sub.remove();
  }, [open, onClose]);

  const keoXuong = Gesture.Pan()
    .activeOffsetY(6)
    .onUpdate((e) => {
      keo.value = Math.max(0, e.translationY);
    })
    .onEnd((e) => {
      if (e.translationY > KEO_DONG_DP || e.velocityY > KEO_DONG_TOC) runOnJS(onClose)();
      else keo.value = withSpring(0, motion.spring.settle);
    });

  const scrim = useAnimatedStyle(() => ({ opacity: progress.value * Math.max(0, 1 - keo.value / 480) }));
  const panel = useAnimatedStyle(() => ({
    transform: [{ translateY: (1 - progress.value) * 480 + keo.value }],
  }));

  if (!hien) return null;
  // The scroll box inside caps the panel so a long editor at font 2.0 scrolls
  // instead of pushing its own submit button off the window.

  // `collapsable={false}`: Fabric flattens a plain wrapper View and attaches
  // its children to the screen directly; toggling `pointerEvents` later makes
  // it un-flatten by re-parenting live Reanimated views, which crashed with
  // «addViewAt: child already has a parent» when a screen went away while its
  // sheet was closing (board 2026-09-06, flow 39). A real native view never
  // has to be re-parented.
  return (
    <View collapsable={false} style={StyleSheet.absoluteFill} pointerEvents={open ? "auto" : "none"} testID={testID}>
      <Animated.View style={[StyleSheet.absoluteFill, { backgroundColor: lopPhu.toi(0.42) }, scrim]}>
        <Pressable accessibilityLabel="Đóng" accessibilityRole="button" onPress={onClose} style={StyleSheet.absoluteFill} />
      </Animated.View>
      <Animated.View
        accessibilityViewIsModal
        accessibilityLabel={accessibilityLabel}
        style={[
          styles.panel,
          {
            backgroundColor: colors.card,
            borderTopLeftRadius: radius.base,
            borderTopRightRadius: radius.base,
            paddingHorizontal: space.md,
            paddingBottom: Math.max(insets.bottom, space.md),
          },
          panel,
          style,
        ]}
      >
        <GestureDetector gesture={keoXuong}>
          <View accessibilityHint="Kéo xuống để đóng" accessibilityLabel="Tay cầm" style={styles.vungKeo}>
            <View style={[styles.handle, { backgroundColor: colors.lineStrong }]} />
          </View>
        </GestureDetector>
        <ScrollView bounces={false} keyboardShouldPersistTaps="handled" showsVerticalScrollIndicator={false} style={{ maxHeight: tran }}>
          {children}
        </ScrollView>
      </Animated.View>
    </View>
  );
}

const styles = StyleSheet.create({
  panel: { position: "absolute", left: 0, right: 0, bottom: 0 },
  // A grab zone the width of the panel and taller than the bar it shows.
  vungKeo: { alignItems: "center", justifyContent: "center", minHeight: 36, marginBottom: 4 },
  handle: { width: 40, height: 4, borderRadius: 2 },
});
