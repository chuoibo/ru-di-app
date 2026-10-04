import { Ionicons } from "@expo/vector-icons";
import { NavigationContext } from "expo-router/build/react-navigation/core/NavigationContext";
import { useContext, useEffect, useRef, useState, type ReactNode } from "react";
import { BackHandler, KeyboardAvoidingView, Platform, Pressable, ScrollView, StyleSheet, View, useWindowDimensions, type StyleProp, type ViewStyle } from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import Animated, { interpolate, runOnJS, useAnimatedStyle, useSharedValue, withSpring, withTiming } from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { useNepGui } from "../nep/NepProvider";
import { dangKyLuiWeb } from "./lui-web";
import { lopPhu, useRudiTheme } from "../theme";
import { useMotion } from "./useMotion";
import { COT_DOC } from "../adaptive";

export interface SheetProps {
  open: boolean;
  onClose: () => void;
  /** An inline step may consume Back/Escape; dismiss/drag/blur still close. */
  onBack?: () => void;
  /** After the close animation has finished; a route that hosts the sheet navigates back from here. */
  onClosed?: () => void;
  children: ReactNode;
  /** Announced to assistive tech when the sheet opens. */
  accessibilityLabel: string;
  style?: StyleProp<ViewStyle>;
  /** Tallest the panel may grow (dp) before its content scrolls; default 82% of the window. */
  maxHeight?: number;
  /** A head for the page above its scrolling content: a small stage, a stamp (ADR-0037). */
  dauTrang?: ReactNode;
  /** Keep a form's editor/actions outside the reference content's scroll. */
  footer?: ReactNode;
  /** Opt-in for an editor whose panel must fit above the software keyboard. */
  avoidKeyboard?: boolean;
  /** Opt-in overflow cue for long content that must remain readable. */
  showsVerticalScrollIndicator?: boolean;
  testID?: string;
}

/** Open sheets in opening order; only the top one answers Escape and Tab. */
const webSheetStack: symbol[] = [];
/** Past this drag (dp) or this speed (dp/s) a release closes the sheet. */
const KEO_DONG_DP = 90;
const KEO_DONG_TOC = 900;
/** Widest a panel grows on a tablet; the reading column of DESIGN.md (QA UI-093). */
const RONG_TOI_DA = COT_DOC;
/**
 * How long after opening a tap on the sheet means nothing. A second tap of a
 * double tap lands ~60 ms after the first, on whatever has just appeared under
 * the finger: the scrim, which closed the sheet it had just opened (UI-088),
 * or a card of the desk sliding up, which navigated (UI-006).
 */
const CHAN_CHAM_MS = 250;

/**
 * A bottom sheet on the UI thread: scrim fades over `standard`, the panel
 * springs up and settles, Back and the scrim both close it, and the handle is
 * a real one: dragging it follows the finger and a release past `KEO_DONG_DP`
 * (or a flick) closes the sheet, a shorter release springs it back. The report
 * ruled that a handle may not imply drag-to-dismiss it does not do. The grab
 * zone is the handle row, not the whole panel, so the content's own scroll
 * never fights the drag. The whole panel is capped at `maxHeight` (82% of the
 * window), is at most 640 dp wide and centred on a tablet, travels by its own
 * height and fades over the last stretch of a close, takes no tap for the
 * first `CHAN_CHAM_MS` after opening, and is the top layer of its screen.
 * Not a modal route, so it can live inside a screen (create menu, picker,
 * filters) and be driven by state; `app/create.tsx` hosts it in a
 * transparent route. Under Reduce Motion the spring resolves instantly.
 */
export function Sheet({ open, onClose, onBack, onClosed, children, accessibilityLabel, style, maxHeight, dauTrang, footer, avoidKeyboard = false, showsVerticalScrollIndicator = false, testID }: SheetProps) {
  const { colors, radius, space } = useRudiTheme();
  const insets = useSafeAreaInsets();
  const { height: windowHeight, width: windowWidth } = useWindowDimensions();
  // A page torn off a pad: the punched holes along its top edge (ADR-0037 D1).
  const soLo = Math.max(6, Math.min(40, Math.floor(windowWidth / 18)));
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
  // False for `CHAN_CHAM_MS` after each opening: the panel and the scrim take
  // no tap while the double tap that opened them is still landing.
  const [nhanCham, setNhanCham] = useState(false);
  // The panel's own height, so it travels exactly out of the window and not a
  // fixed 480 dp that left a third of a tall sheet showing (UI-013).
  const cao = useSharedValue(windowHeight);
  // The host's width, so a tablet gets a centred 640 dp panel (UI-093).
  const [rongKhung, setRongKhung] = useState(windowWidth);
  const [caoKhung, setCaoKhung] = useState(windowHeight);
  const panelRef = useRef<View>(null);
  const wrapperRef = useRef<View>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;
  const backRef = useRef(onBack ?? onClose);
  backRef.current = onBack ?? onClose;
  const nepGui = useNepGui();

  // Nếp is not drawn over an open sheet (`nep/trang-thai.ts` rule 3). Counted
  // per sheet, and the cleanup also runs when a screen unmounts with its sheet
  // still open, so the count cannot leak.
  useEffect(() => {
    if (!open || !nepGui) return;
    nepGui({ kieu: "mo-sheet" });
    return () => nepGui({ kieu: "dong-sheet" });
  }, [open, nepGui]);

  useEffect(() => {
    if (!open || !hien || Platform.OS !== "web") return;
    const identity = Symbol("sheet");
    webSheetStack.push(identity);
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const element = panelRef.current as unknown as HTMLElement | null;
    const hidden: { element: HTMLElement; inert: boolean; ariaHidden: string | null }[] = [];
    let branch = wrapperRef.current as unknown as HTMLElement | null;
    while (branch?.parentElement) {
      for (const sibling of Array.from(branch.parentElement.children)) {
        if (sibling !== branch && sibling instanceof HTMLElement && !["SCRIPT", "STYLE", "LINK"].includes(sibling.tagName)) {
          hidden.push({ element: sibling, inert: sibling.inert, ariaHidden: sibling.getAttribute("aria-hidden") });
          sibling.inert = true;
          sibling.setAttribute("aria-hidden", "true");
        }
      }
      if (branch.parentElement === document.body) break;
      branch = branch.parentElement;
    }
    const focusable = () => Array.from(element?.querySelectorAll<HTMLElement>('button,input,textarea,select,[tabindex]:not([tabindex="-1"])') ?? []).filter((node) => node.getAttribute("aria-disabled") !== "true" && !node.hasAttribute("disabled") && node.getClientRects().length > 0);
    const frame = requestAnimationFrame(() => focusable()[0]?.focus());
    const onKey = (event: KeyboardEvent) => {
      if (webSheetStack.at(-1) !== identity) return;
      if (event.key === "Escape") {
        event.preventDefault(); event.stopImmediatePropagation(); backRef.current();
      } else if (event.key === "Tab") {
        const targets = focusable();
        const current = targets.indexOf(document.activeElement as HTMLElement);
        if (targets.length && (current < 0 || (!event.shiftKey && current === targets.length - 1) || (event.shiftKey && current === 0))) {
          event.preventDefault(); targets[event.shiftKey ? targets.length - 1 : 0]?.focus();
        }
      }
    };
    document.addEventListener("keydown", onKey, true);
    return () => {
      cancelAnimationFrame(frame);
      document.removeEventListener("keydown", onKey, true);
      webSheetStack.splice(webSheetStack.indexOf(identity), 1);
      // Put back only what is still this sheet's doing. The navigator changes
      // these same attributes when a screen gains or loses focus, and Back
      // while the sheet was open runs that change first: restoring the values
      // saved at open time then re-hid the screen the person had just returned
      // to, tab bar included (QA UI-005, P1). An attribute that no longer reads
      // what this sheet wrote belongs to someone else now and is left alone.
      for (const saved of hidden) {
        if (!saved.element.isConnected) continue;
        if (saved.element.inert === true) saved.element.inert = saved.inert;
        if (saved.element.getAttribute("aria-hidden") === "true") {
          if (saved.ariaHidden === null) saved.element.removeAttribute("aria-hidden");
          else saved.element.setAttribute("aria-hidden", saved.ariaHidden);
        }
      }
      if (previous?.isConnected) previous.focus();
    };
  }, [open, hien]);

  const dongXong = () => {
    setHien(false);
    onClosed?.();
  };

  useEffect(() => {
    if (open) {
      setHien(true);
      setNhanCham(false);
      keo.value = 0;
      progress.value = withSpring(1, motion.spring.settle);
      const t = setTimeout(() => setNhanCham(true), CHAN_CHAM_MS);
      return () => clearTimeout(t);
    }
    progress.value = withTiming(0, motion.timing("standard"), (finished) => {
      if (finished) runOnJS(dongXong)();
    });
    // `dongXong` is recreated per render; the callback captured here is the
    // one from the render that closed the sheet, which is the one wanted.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, motion, progress, keo]);

  // The browser's Back closes the sheet, as Android's does (QA UI-038,
  // UI-117), and the page stays where it was (`lui-web.ts`).
  useEffect(() => {
    if (!open) return;
    return dangKyLuiWeb(() => backRef.current());
  }, [open]);

  // A sheet belongs to the screen it was opened on. When that screen loses
  // focus -- another tab, a route pushed from inside the sheet -- the sheet
  // closes, instead of staying open on a screen nobody can see and keeping
  // the rest of the app inert beneath it (QA UI-117, UI-087).
  const navigation = useContext(NavigationContext);
  useEffect(() => {
    if (!open || !navigation) return;
    return navigation.addListener("blur", () => closeRef.current());
  }, [open, navigation]);

  useEffect(() => {
    if (!open || Platform.OS !== "android") return;
    const sub = BackHandler.addEventListener("hardwareBackPress", () => {
      backRef.current();
      return true;
    });
    return () => sub.remove();
  }, [open]);

  const keoXuong = Gesture.Pan()
    .activeOffsetY(6)
    .onUpdate((e) => {
      keo.value = Math.max(0, e.translationY);
    })
    .onEnd((e) => {
      if (e.translationY > KEO_DONG_DP || e.velocityY > KEO_DONG_TOC) runOnJS(onClose)();
      else keo.value = withSpring(0, motion.spring.settle);
    });

  const scrim = useAnimatedStyle(() => ({ opacity: progress.value * Math.max(0, 1 - keo.value / Math.max(cao.value, 1)) }));
  // Out by its own height (and the shadow's reach), and faded over the last
  // stretch, so the final frame before unmount shows nothing: no panel left
  // standing a third out of the window and then gone (UI-013).
  const panel = useAnimatedStyle(() => ({
    opacity: interpolate(progress.value, [0, 0.12, 1], [0, 1, 1]),
    transform: [{ translateY: (1 - progress.value) * (cao.value + 24) + keo.value }],
  }));

  if (!hien) return null;
  const le = Math.max(0, (rongKhung - RONG_TOI_DA) / 2);

  // `collapsable={false}`: Fabric flattens a plain wrapper View and attaches
  // its children to the screen directly; toggling `pointerEvents` later makes
  // it un-flatten by re-parenting live Reanimated views, which crashed with
  // «addViewAt: child already has a parent» when a screen went away while its
  // sheet was closing (board 2026-09-06, flow 39). A real native view never
  // has to be re-parented.
  return (
    // zIndex: on the web a strip the screen pinned with its own zIndex (the
    // chat's pinned sheet, z 1) painted over the scrim and the panel (UI-070,
    // UI-166). The sheet is the top layer of its screen, by declaration.
    <View
      ref={wrapperRef}
      collapsable={false}
      onLayout={(e) => setRongKhung(Math.round(e.nativeEvent.layout.width))}
      pointerEvents={open ? "auto" : "none"}
      style={[StyleSheet.absoluteFill, styles.lop]}
      testID={testID}
    >
      <KeyboardAvoidingView
        enabled={avoidKeyboard}
        behavior={Platform.OS === "ios" ? "padding" : "height"}
        onLayout={(event) => setCaoKhung(event.nativeEvent.layout.height)}
        style={StyleSheet.absoluteFill}
      >
      <Animated.View style={[StyleSheet.absoluteFill, { backgroundColor: lopPhu.toi(0.42) }, scrim]}>
        <Pressable accessibilityLabel="Đóng" accessibilityRole="button" onPress={nhanCham ? onClose : undefined} style={StyleSheet.absoluteFill} />
      </Animated.View>
      <Animated.View
        ref={panelRef}
        role={Platform.OS === "web" ? "dialog" : undefined}
        aria-modal={Platform.OS === "web" ? true : undefined}
        onAccessibilityEscape={onClose}
        accessibilityViewIsModal
        accessibilityLabel={accessibilityLabel}
        onLayout={(e) => {
          cao.value = e.nativeEvent.layout.height;
        }}
        style={[
          styles.panel,
          {
            // The ceiling is the whole panel's: handle, head and content
            // together, not the scroll box alone with the handle and the
            // insets added on top (92% at 320×640, 96% at 390×460: UI-007, UI-040).
            maxHeight: avoidKeyboard ? Math.min(tran, Math.max(0, caoKhung - insets.top)) : tran,
            left: le,
            right: le,
            backgroundColor: colors.card,
            borderTopLeftRadius: radius.base,
            borderTopRightRadius: radius.base,
            paddingHorizontal: space.md,
            paddingBottom: Math.max(avoidKeyboard && caoKhung < windowHeight - insets.bottom ? 0 : insets.bottom, space.md),
          },
          panel,
          style,
        ]}
      >
        <View importantForAccessibility="no-hide-descendants" pointerEvents="none" style={[styles.loGiay, { paddingHorizontal: radius.base }]}>
          {Array.from({ length: soLo }, (_, i) => (
            <View key={i} style={[styles.lo, { backgroundColor: colors.paperShade }]} />
          ))}
        </View>
        <View style={styles.handleRow}>
          <View style={styles.closeSpace} />
          <GestureDetector gesture={keoXuong}>
            {/* A gesture for the hand, not a control for assistive tech: «Đóng
                bảng» beside it is the accessible way out. A label with no role
                here was axe's aria-prohibited-attr in every sheet (UI-089). */}
            <View aria-hidden accessibilityElementsHidden importantForAccessibility="no-hide-descendants" style={styles.vungKeo} testID="tay-cam">
              <View style={[styles.handle, { backgroundColor: colors.lineStrong }]} />
            </View>
          </GestureDetector>
          <Pressable accessibilityRole="button" accessibilityLabel="Đóng bảng" onPress={onClose} style={styles.closeSpace}>
            <Ionicons name="close" size={22} color={colors.ink} />
          </Pressable>
        </View>
        {dauTrang}
        <ScrollView
          bounces={false}
          keyboardShouldPersistTaps="handled"
          persistentScrollbar={showsVerticalScrollIndicator}
          showsVerticalScrollIndicator={showsVerticalScrollIndicator}
          style={[styles.cuon, showsVerticalScrollIndicator && Platform.OS === "web" ? { scrollbarWidth: "thin", scrollbarColor: `${colors.lineStrong} ${colors.card}` } as ViewStyle : null]}
        >
          {children}
        </ScrollView>
        {footer}
        {/* The opening's tap guard over the panel itself (UI-006). */}
        {nhanCham ? null : <View style={StyleSheet.absoluteFill} testID="sheet-chan-cham" />}
      </Animated.View>
      </KeyboardAvoidingView>
    </View>
  );
}

const styles = StyleSheet.create({
  lop: { zIndex: 10 },
  panel: { position: "absolute", bottom: 0 },
  // Shrinks inside the panel's ceiling and scrolls; grows no further than its content.
  cuon: { flexGrow: 0, flexShrink: 1 },
  // A grab zone the width of the panel and taller than the bar it shows.
  handleRow: { flexDirection: "row", alignItems: "center" },
  closeSpace: { width: 48, height: 48, alignItems: "center", justifyContent: "center" },
  vungKeo: { flex: 1, alignItems: "center", justifyContent: "center", minHeight: 36, marginBottom: 4 },
  handle: { width: 40, height: 4, borderRadius: 2 },
  loGiay: { position: "absolute", left: 0, right: 0, top: 6, flexDirection: "row", justifyContent: "space-between" },
  lo: { width: 5, height: 5, borderRadius: 2.5 },
});
