import { Ionicons } from "@expo/vector-icons";
import { BlurView } from "expo-blur";
import { Image, ImageSource } from "expo-image";
import { LinearGradient } from "expo-linear-gradient";
import { StatusBar } from "expo-status-bar";
import { usePathname, useRouter } from "expo-router";
import { NavigationContext } from "expo-router/build/react-navigation/core/NavigationContext";
import { Children, createContext, useContext, useEffect, useMemo, useRef, useState, type ComponentProps, type ReactNode } from "react";
import { ActivityIndicator, DimensionValue, GestureResponderEvent, Keyboard, KeyboardAvoidingView, Platform, Pressable, RefreshControl, ScrollView, StyleProp, StyleSheet, Text, TextInput, TextInputProps, TextStyle, View, ViewStyle, useWindowDimensions, type LayoutChangeEvent } from "react-native";
import Animated, { useAnimatedScrollHandler, useSharedValue } from "react-native-reanimated";
import { SafeAreaView } from "react-native-safe-area-context";

import { DemoPerson } from "./fixtures";
import { duongDangNhap, laCuaVao } from "./duong-vao";
import { luiVe as luiVeAnToan } from "./lui-ve";
import { useRudiSession } from "./session";
import { cardShadow, lopPhu, mucTrenAnh, RudiTone, toneColor, toneSoftColor, typography, useRudiTheme, displayFace } from "./theme";
import { Field as FieldCore, type FieldCoreProps } from "./ui/Field";
import { Grain } from "./ui/Grain";
import { PressScale } from "./ui/PressScale";
import { useAdaptiveLayout } from "./ui/useAdaptiveLayout";
import { Wordmark } from "./ui/Wordmark";
import { CuonContext } from "./ui/cuon";
import { KheLopProvider, useKheLop } from "./ui/KheLop";
import { gridFor, tabBarHeight } from "./adaptive";
import { KHONG_VIEN_WEB } from "./ui/khong-vien-web";
import { TABLIST, giuState } from "../ui/a11y";

export type IconName = ComponentProps<typeof Ionicons>["name"];

type ScreenProps = {
  children: ReactNode;
  scroll?: boolean;
  tone?: RudiTone;
  padded?: boolean;
  /** Space under the content; `"tab"` is the tab bar's height at the current font scale plus a breath. */
  bottomInset?: number | "tab";
  footer?: ReactNode;
  footerInset?: number;
  contentStyle?: StyleProp<ViewStyle>;
  /** `cover` when the first child is a CoverBand: the status-bar area is indigo, not paper. */
  surface?: "page" | "cover";
  testID?: string;
  /** Only opt in when this screen owns the composer; live chat owns its own IME. */
  avoidKeyboard?: boolean;
  scrollEnabled?: boolean;
  /** Pull-to-refresh on a live list: the same read the screen does on focus, so stale data has a way out. */
  onRefresh?: () => Promise<void>;
  /** A sheet or scrim laid over the whole screen, outside the scroll box (a `Sheet` inside the content would scroll away with it). */
  overlay?: ReactNode;
  /** A thread reads from its end: keep the scroll at the bottom as content grows. */
  keepEnd?: boolean;
  /** Stays above the scroll box: a chat's top bar and pinned outing, which `keepEnd` would otherwise scroll away. */
  header?: ReactNode;
  /**
   * A paper stage heading the screen (ADR-0037, `ui/CanhGap`): first in the
   * list, folding flat as the list scrolls. The screen publishes its scroll
   * offset to the stage and to the header's compact title (`ui/cuon`), and the
   * header drops its fixed keyline for the one `ThanhCanh` fades in.
   */
  canh?: ReactNode;
  /**
   * A stepped flow's step: whenever it changes the list goes back to its top,
   * so a new page starts at its head and its title, not wherever the reader
   * had scrolled the last one to (a plain list only; a staged one folds).
   */
  cuonVeDau?: string | number;
  /**
   * The column the content keeps to on a tablet: `doc` 640 dp for a page to
   * read (a sheet of paper, a post), `form` 560 dp for a form (DESIGN.md),
   * `rong` 960 dp for grids and lists (the default). On a phone it is the
   * window. The header keeps to the same column (QA UI-093, UI-047).
   */
  cot?: "doc" | "form" | "rong";
};

/** The tablet column widths of `RudiScreen`'s `cot`. */
const RONG_COT = { doc: 640, form: 560, rong: 960 } as const;

export function RudiScreen({
  children,
  scroll = true,
  tone: _tone = "accent",
  padded = true,
  bottomInset = 32,
  footer,
  footerInset = 0,
  contentStyle,
  surface = "page",
  testID,
  avoidKeyboard = false,
  scrollEnabled = true,
  overlay,
  keepEnd = false,
  header,
  onRefresh,
  canh,
  cuonVeDau,
  cot = "rong",
}: ScreenProps) {
  const { colors, dark, space } = useRudiTheme();
  const layout = useAdaptiveLayout();
  const { fontScale } = useWindowDimensions();
  const [keyboardOpen, setKeyboardOpen] = useState(false);
  const cuon = useRef<ScrollView>(null);
  // The scroll a stage folds with; published only when the screen has one.
  const coCanh = canh !== undefined && canh !== null && canh !== false;
  const cuonY = useSharedValue(0);
  const nguongTieuDe = useSharedValue(1e6);
  const theoCuon = useAnimatedScrollHandler((e) => {
    cuonY.value = e.contentOffset.y;
  });
  const cuonMan = useMemo(() => (coCanh ? { cuonY, nguongTieuDe } : null), [coCanh, cuonY, nguongTieuDe]);
  // Layers a component deep in the screen draws over all of it (`LenLop`).
  const { khe, lop } = useKheLop();
  // Pull-to-refresh runs the screen's own read; the spinner is the only state
  // the shell adds, and it ends whether the read succeeded or threw.
  const [dangKeo, setDangKeo] = useState(false);
  const keoLamMoi = onRefresh
    ? async () => {
        setDangKeo(true);
        try {
          await onRefresh();
        } finally {
          setDangKeo(false);
        }
      }
    : undefined;
  useEffect(() => {
    if (cuonVeDau === undefined) return;
    cuon.current?.scrollTo({ y: 0, animated: false });
  }, [cuonVeDau]);
  useEffect(() => {
    if (!avoidKeyboard) return;
    const show = Keyboard.addListener("keyboardDidShow", () => setKeyboardOpen(true));
    const hide = Keyboard.addListener("keyboardDidHide", () => setKeyboardOpen(false));
    return () => { show.remove(); hide.remove(); };
  }, [avoidKeyboard]);
  const tablet = layout.sizeClass !== "compact";
  const inner = [
    styles.screenInner,
    // A cover band is the first child and paints under the status bar itself
    // (`CoverBand underStatusBar`); the screen adds no top inset, or a paper
    // strip shows between the two.
    surface === "cover" && { paddingTop: 0 },
    padded && { paddingHorizontal: tablet ? space.lg : space.md },
    { paddingBottom: bottomInset === "tab" ? tabBarHeight(fontScale) + 48 : bottomInset },
    tablet && [styles.tabletInner, { maxWidth: RONG_COT[cot] }],
    contentStyle,
  ];

  return (
    <SafeAreaView
      edges={surface === "cover" ? ["left", "right"] : ["top", "left", "right"]}
      style={[styles.safeArea, { backgroundColor: surface === "cover" ? colors.cover : colors.ground }]}
      testID={testID}
    >
      {/* Light icons over the indigo cover, dark over paper; every screen re-asserts
          so leaving a cover screen never leaves pale icons on cream. */}
      <StatusBar style={surface === "cover" || dark ? "light" : "dark"} />
      <View pointerEvents="none" style={[styles.paper, { backgroundColor: colors.ground }]}>
        {/* Day: a page of paper with its grain. Night: the notebook is closed on the
            table — the ground is the cover cloth (its weave measures ≈ 8 grey levels at
            0.30; the paper tile measured ≈ 2 on the dark ground, i.e. flat), and every
            drawn sheet sits on it in the `paper` tone (review 11/09, A3). */}
        {dark ? <Grain material="vaiBia" opacity={0.3} /> : <Grain material="giayTrang" opacity={0.45} />}
      </View>
      <KheLopProvider khe={khe}>
      <CuonContext.Provider value={cuonMan}>
      <KeyboardAvoidingView style={styles.flex} enabled={avoidKeyboard} behavior={Platform.OS === "ios" ? "padding" : "height"}>
      {header ? (
        // A keyline under the fixed header: content scrolling beneath it reads
        // as paper under a rule, not as a rendering fault. A staged screen's
        // bar draws its own, only once the stage has folded under it.
        <View style={[styles.screenHeader, { paddingHorizontal: tablet ? space.lg : space.md, borderBottomColor: colors.line }, coCanh && styles.screenHeaderTrong, tablet && [styles.tabletInner, { maxWidth: RONG_COT[cot] }]]}>{header}</View>
      ) : null}
      {scroll && coCanh ? (
        <Animated.ScrollView
          scrollEnabled={scrollEnabled}
          contentContainerStyle={inner}
          keyboardShouldPersistTaps="handled"
          onScroll={theoCuon}
          scrollEventThrottle={16}
          refreshControl={
            keoLamMoi ? (
              <RefreshControl colors={[colors.accent]} onRefresh={() => void keoLamMoi()} progressBackgroundColor={colors.card} refreshing={dangKeo} tintColor={colors.accent} />
            ) : undefined
          }
          showsVerticalScrollIndicator={false}
          style={styles.flex}
        >
          {canh}
          {children}
        </Animated.ScrollView>
      ) : scroll ? (
        <ScrollView
          ref={cuon}
          scrollEnabled={scrollEnabled}
          contentContainerStyle={inner}
          keyboardShouldPersistTaps="handled"
          onContentSizeChange={keepEnd ? () => cuon.current?.scrollToEnd({ animated: false }) : undefined}
          refreshControl={
            keoLamMoi ? (
              <RefreshControl colors={[colors.accent]} onRefresh={() => void keoLamMoi()} progressBackgroundColor={colors.card} refreshing={dangKeo} tintColor={colors.accent} />
            ) : undefined
          }
          showsVerticalScrollIndicator={false}
          style={styles.flex}
        >
          {children}
        </ScrollView>
      ) : (
        <View style={[inner, styles.flex]}>{canh}{children}</View>
      )}
      {footer ? (
        <View
          style={[
            styles.screenFooter,
            { paddingHorizontal: tablet ? space.lg : space.md, paddingBottom: keyboardOpen ? 8 : footerInset },
            tablet && [styles.tabletInner, { maxWidth: RONG_COT[cot] }],
          ]}
        >
          {footer}
        </View>
      ) : null}
      </KeyboardAvoidingView>
      </CuonContext.Provider>
      </KheLopProvider>
      {overlay}
      {lop}
    </SafeAreaView>
  );
}

/**
 * Inside a TopBar: a DemoBadge there shortens its label so the title can stay
 * centred, and steps aside entirely where the bar draws the demo door itself.
 */
const TrongTopBar = createContext<{ trong: boolean; cua: boolean }>({ trong: false, cua: false });

export function TopBar({
  title,
  subtitle,
  back = true,
  onBack,
  right,
}: {
  title?: string;
  subtitle?: string;
  back?: boolean;
  /** What the chevron does instead of leaving the route: a stepper's own step back. */
  onBack?: () => void;
  right?: ReactNode;
}) {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const { cheDo } = useRudiSession();
  // The screen's own navigator, not the app's current route: under the
  // transparent «Tạo mới» route the current route is `/create`, and a tab's
  // header behind the tray started drawing the demo door (seen on Android).
  const navigation = useContext(NavigationContext);
  const trongTab = navigation?.getState?.()?.type === "tab";
  const pathname = usePathname();
  // A demo screen outside the tabs (the tabs carry «Đăng nhập» in the bar)
  // says it is one and where the way in is, on every such screen, with ONE
  // element: the demo door (QA UI-082). A badge and a separate sign-in icon
  // side by side widened the right side so far that the centred title of the
  // demo notebook was 6 px wide. Never on the doors themselves.
  const cuaDemo = cheDo !== "live" && !trongTab && !laCuaVao(pathname);
  const { width: rongCuaSo } = useWindowDimensions();
  // Back through history when there is any; a screen a link opened cold goes
  // to its own tab instead of standing still (QA UI-018).
  const luiVe = () => {
    if (onBack !== undefined) onBack();
    else luiVeAnToan(router as never, pathname);
  };
  // Web: a screen that gains navigation focus puts the keyboard and the
  // screen reader on its title, not on `body` (QA UI-112: 7 of 7 screens
  // measured left focus on `body`). Never over a field the person is typing
  // in, never under an open sheet that owns focus.
  const tieuDeRef = useRef<Text>(null);
  useEffect(() => {
    if (Platform.OS !== "web" || !title) return;
    const dua = () => {
      requestAnimationFrame(() => duaFocusVaoTieuDe(tieuDeRef.current));
    };
    if (!navigation || navigation.isFocused()) dua();
    return navigation?.addListener("focus", dua);
  }, [navigation, title]);
  // Both sides take the wider side's natural width, so the title is centred on
  // the screen and not on whatever is left between a chevron and a badge. The
  // natural width is measured on an inner view; measuring the slot itself would
  // read back the minimum we set and never shrink again. Centring gives way
  // when it would cut the title: then each side keeps its own width and the
  // title takes what is between them, a little off centre but whole.
  const [benRong, setBenRong] = useState({ trai: 0, phai: 0 });
  const [rongThanh, setRongThanh] = useState(0);
  const [rongChu, setRongChu] = useState(0);
  const canBang = Math.max(52, benRong.trai, benRong.phai);
  const vuaKhiCanBang = rongThanh === 0 || rongChu === 0 || rongThanh - 2 * canBang - 16 >= rongChu;
  const rongTrai = vuaKhiCanBang ? canBang : Math.max(52, benRong.trai);
  const rongPhai = vuaKhiCanBang ? canBang : Math.max(52, benRong.phai);
  const doBen = (ben: "trai" | "phai") => (e: LayoutChangeEvent) => {
    const w = Math.ceil(e.nativeEvent.layout.width);
    setBenRong((cu) => (cu[ben] === w ? cu : { ...cu, [ben]: w }));
  };

  return (
    <TrongTopBar.Provider value={{ trong: true, cua: cuaDemo }}>
    <View onLayout={(e) => setRongThanh(Math.round(e.nativeEvent.layout.width))} style={styles.topBar}>
      {/* The title's and subtitle's natural width, measured off-screen for the rule above. */}
      <View aria-hidden importantForAccessibility="no-hide-descendants" onLayout={(e) => setRongChu(Math.ceil(e.nativeEvent.layout.width))} pointerEvents="none" style={styles.doChu}>
        {title ? <Text style={typography.title}>{title}</Text> : null}
        {subtitle ? <Text style={typography.caption}>{subtitle}</Text> : null}
      </View>
      <View style={[styles.topBarSide, { minWidth: rongTrai }]}>
        <View onLayout={doBen("trai")} style={styles.topBarSideInner}>
        {back ? (
          <IconButton
            accessibilityLabel="Quay lại"
            icon="chevron-back"
            onPress={luiVe}
            quiet
          />
        ) : (
          // The wordmark alone: the gradient tile repeated in every tab header
          // was the app icon wearing itself as a hat (finish review, taste note).
          <Wordmark color={colors.ink} height={18} />
        )}
        </View>
      </View>
      <View style={styles.topBarTitleWrap}>
        {title ? (
          // Two lines before an ellipsis: a long title next to a wide right
          // side (the demo notebook's gear and door) reflows instead of
          // losing its last word.
          <Text
            accessibilityRole="header"
            numberOfLines={2}
            ref={tieuDeRef}
            style={[typography.title, styles.topBarTitle, KHONG_VIEN_WEB, { color: colors.ink }]}
            // Focusable by script only (react-native-web forwards it), so the
            // title can take focus when its screen does, and Tab never stops on it.
            {...({ tabIndex: -1 } as object)}
          >
            {title}
          </Text>
        ) : null}
        {subtitle ? (
          <Text
            numberOfLines={1}
            style={[typography.caption, { color: colors.inkFaint }]}
          >
            {subtitle}
          </Text>
        ) : null}
      </View>
      <View style={[styles.topBarSide, styles.topBarRight, { minWidth: rongPhai }]}>
        <View onLayout={doBen("phai")} style={styles.topBarSideInnerRight}>
          {right}
          {cuaDemo ? <CuaDemo nhan={rongCuaSo < 360 ? null : "Demo"} onPress={() => router.push(duongDangNhap(pathname) as never)} /> : null}
        </View>
      </View>
    </View>
    </TrongTopBar.Provider>
  );
}

export function Logo({ compact = false, ink }: { compact?: boolean; ink?: string }) {
  const { brand, colors } = useRudiTheme();
  return (
    <View style={styles.logoRow} accessibilityLabel="Rủ Đi">
      <LinearGradient
        colors={[brand.logoGradient.from, brand.logoGradient.to]}
        end={{ x: 1, y: 1 }}
        start={{ x: 0, y: 0 }}
        style={[styles.logoMark, compact && styles.logoMarkCompact]}
      >
        <Text style={[styles.logoMarkType, compact && styles.logoMarkTypeCompact]}>
          Rủ{"\n"}Đi
        </Text>
      </LinearGradient>
      <Wordmark height={compact ? 18 : 26} color={ink ?? colors.ink} />
    </View>
  );
}

export function Eyebrow({ children, tone = "accent" }: { children: ReactNode; tone?: RudiTone }) {
  const { colors } = useRudiTheme();
  return (
    <View style={[styles.eyebrow, { backgroundColor: toneSoftColor(colors, tone) }]}>
      <View style={[styles.eyebrowDot, { backgroundColor: toneColor(colors, tone) }]} />
      <Text style={[typography.caption, { color: toneColor(colors, tone) }]}>{children}</Text>
    </View>
  );
}

/**
 * "Dữ liệu demo". A claim about where the numbers came from, so it reads the
 * mode rather than being placed by hand on the screens somebody remembered.
 *
 * Renders NOTHING in live mode. A badge saying "demo" over real money would be
 * the same lie as the reverse, pointed the other way.
 */
export function DemoBadge({
  label = "Dữ liệu demo",
  compactLabel,
  cua = true,
}: {
  label?: string;
  /** Short form used inside a TopBar; default «Demo». */
  compactLabel?: string;
  /**
   * The badge is the way out of the demo: pressing it goes to sign-in with
   * this screen as `?tiep=` (QA UI-082). `false` for a label that names a
   * part of a demo screen, not the screen («AI nháp» on the AI sheet).
   */
  cua?: boolean;
}) {
  const { colors } = useRudiTheme();
  const { cheDo } = useRudiSession();
  const topBar = useContext(TrongTopBar);
  const router = useRouter();
  const pathname = usePathname();
  if (cheDo === "live") return null;
  // The bar draws the door, which says «Demo» itself; a second label beside it
  // would be the same word twice and the width the title needs.
  if (topBar.cua) return null;
  const trongTopBar = topBar.trong;
  // In a title bar the full label cannot share a 360dp row with a centred title
  // at font 1.3; the flask plus «Demo» keeps the honesty, the accessibility
  // label keeps the full sentence for screen readers and the native gate.
  const chu = trongTopBar ? compactLabel ?? "Demo" : label;
  if (cua) return <CuaDemo label={label} nhan={chu} onPress={() => router.push(duongDangNhap(pathname) as never)} />;
  return (
    <View accessibilityLabel={label} style={[styles.demoBadge, { backgroundColor: colors.card, borderColor: colors.line }]}>
      <Ionicons color={colors.inkFaint} name="flask-outline" size={12} />
      <Text numberOfLines={1} style={[styles.demoText, { color: colors.inkFaint }]}>{chu}</Text>
    </View>
  );
}

/** Focus the screen's title unless someone is typing or a sheet holds focus. */
function duaFocusVaoTieuDe(node: unknown): void {
  if (typeof document === "undefined") return;
  const tieuDe = node as HTMLElement | null;
  if (!tieuDe?.isConnected || typeof tieuDe.focus !== "function") return;
  const dangO = document.activeElement as HTMLElement | null;
  const dangGo =
    dangO !== null &&
    dangO.isConnected &&
    (dangO.tagName === "INPUT" || dangO.tagName === "TEXTAREA" || dangO.isContentEditable) &&
    dangO.closest('[aria-hidden="true"],[inert]') === null;
  if (dangGo || document.querySelector('[role="dialog"][aria-modal="true"]')) return;
  tieuDe.focus({ preventScroll: true });
}

/**
 * The demo door of a TopBar: «Demo» and the way out of it, as one control.
 * The label is 12 sp, not the passive badge's 10, because it is pressed.
 * Under 360 dp it is the flask and the door alone (`nhan={null}`), so a long title
 * still has two lines' room; its name says the whole sentence either way.
 */
function CuaDemo({ onPress, nhan = "Demo", label = "Dữ liệu demo" }: { onPress: () => void; /** `null`: the flask and the door alone. */ nhan?: string | null; label?: string }) {
  const { colors } = useRudiTheme();
  return (
    <Pressable accessibilityLabel={`${label}. Đăng nhập`} accessibilityRole="button" onPress={onPress} style={styles.cuaDemo} testID="cua-demo">
      <View style={[styles.cuaDemoChip, { backgroundColor: colors.card, borderColor: colors.line }]}>
        <Ionicons color={colors.inkFaint} name="flask-outline" size={13} />
        {nhan === null ? null : <Text numberOfLines={1} style={[styles.cuaDemoChu, { color: colors.inkSoft }]}>{nhan}</Text>}
        <Ionicons color={colors.accent} name="log-in-outline" size={15} />
      </View>
    </Pressable>
  );
}

export function Heading({
  title,
  subtitle,
  align = "left",
  size = "h1",
}: {
  title: string;
  subtitle?: string;
  align?: "left" | "center";
  size?: "display" | "h1" | "h2";
}) {
  const { colors } = useRudiTheme();
  return (
    <View style={[styles.heading, align === "center" && styles.center]}>
      <Text
        style={[
          typography[size],
          { color: colors.ink, textAlign: align },
        ]}
      >
        {title}
      </Text>
      {subtitle ? (
        <Text
          style={[
            typography.body,
            styles.headingSubtitle,
            { color: colors.inkSoft, textAlign: align },
          ]}
        >
          {subtitle}
        </Text>
      ) : null}
    </View>
  );
}

export function SectionHeader({
  title,
  action,
  onAction,
}: {
  title: string;
  action?: string;
  onAction?: () => void;
}) {
  const { colors } = useRudiTheme();
  return (
    <View style={styles.sectionHeader}>
      <Text style={[typography.h2, { color: colors.ink }]}>{title}</Text>
      {action ? (
        <Pressable
          accessibilityRole="button"
          hitSlop={8}
          onPress={onAction}
          style={({ pressed }) => [styles.sectionAction, pressed && styles.pressed]}
        >
          <Text style={[typography.label, { color: colors.accent }]}>{action}</Text>
        </Pressable>
      ) : null}
    </View>
  );
}

export function Card({
  children,
  style,
  tone,
  onPress,
  accessibilityLabel,
}: {
  children: ReactNode;
  style?: StyleProp<ViewStyle>;
  tone?: RudiTone;
  onPress?: (event: GestureResponderEvent) => void;
  accessibilityLabel?: string;
}) {
  const { colors, radius } = useRudiTheme();
  const cardStyle = [
    styles.card,
    cardShadow,
    {
      backgroundColor: tone ? toneSoftColor(colors, tone) : colors.card,
      borderColor: tone ? toneSoftColor(colors, tone) : colors.line,
      borderRadius: radius.base,
    },
    style,
  ];

  if (onPress) {
    return (
      <Pressable
        accessibilityLabel={accessibilityLabel}
        accessibilityRole="button"
        onPress={onPress}
        style={({ pressed }) => [cardStyle, pressed && styles.cardPressed]}
      >
        {children}
      </Pressable>
    );
  }
  return <View style={cardStyle}>{children}</View>;
}

type ButtonProps = {
  label: string;
  onPress?: () => void;
  icon?: IconName;
  /**
   * `warn` is the destructive action: «Xoá cuốn sổ», «Bỏ bản phác». Outline
   * or ghost only, in the `warn` ink that already marks an error -- there is
   * no warn fill in the palette, and a solid request draws as outline. Until
   * this tone, «Xoá cuốn sổ» was a solid coral button and «Bỏ bản phác» looked
   * like «Tuần này nghỉ» (QA pattern notes).
   */
  tone?: RudiTone | "warn";
  variant?: "solid" | "soft" | "outline" | "ghost";
  disabled?: boolean;
  loading?: boolean;
  compact?: boolean;
  full?: boolean;
  style?: StyleProp<ViewStyle>;
  /** When the visible label is not enough on its own («Nhắn tin» on a row
   *  that names somebody): the sentence a screen reader, and Maestro, get. */
  accessibilityLabel?: string;
  /**
   * ADR-0038 §2.2: why a disabled button cannot be used yet («Chọn một tấm
   * ảnh trước đã.»), printed under it and given to a screen reader as the
   * hint. A disabled button without a reason should usually not be shown.
   */
  lyDo?: string;
};

declare const __DEV__: boolean | undefined;

const nutTatDaBao = new Set<string>();

/** One development warning per label for a disabled RudiButton with no `lyDo`. */
function baoNutTatThieuLyDo(label: string): void {
  if (nutTatDaBao.has(label)) return;
  nutTatDaBao.add(label);
  console.warn(`[RudiButton] "${label}" is disabled without lyDo (ADR-0038 §2.2): say why, or hide it until it can be used.`);
}

export function RudiButton({
  label,
  onPress,
  icon,
  tone = "accent",
  variant = "solid",
  disabled = false,
  loading = false,
  compact = false,
  full = true,
  style,
  accessibilityLabel,
  lyDo,
}: ButtonProps) {
  const { colors, radius } = useRudiTheme();
  // A disabled button with no reason under it is what ADR-0038 §2.2 rules out
  // (QA UI-091). Development only: say which one, once per label.
  if (typeof __DEV__ !== "undefined" && __DEV__ && disabled && !loading && !lyDo) baoNutTatThieuLyDo(label);
  const phaHuy = tone === "warn";
  if (phaHuy && (variant === "solid" || variant === "soft")) variant = "outline";
  const mauTone = phaHuy ? colors.warn : toneColor(colors, tone as RudiTone);
  // ADR-0038 §2.2: not yet usable is not faded. The button keeps a readable
  // label and a dashed edge, both measured (`test_contrast_floor.py`); while
  // it loads it keeps its own face.
  const tat = disabled && !loading;
  const vienTat = colors.lineStrong;
  const chuTat = colors.inkSoft;
  const solid = variant === "solid" && !tat;
  const foreground = tat ? chuTat : solid ? colors[`${tone as RudiTone}Ink` as const] : mauTone;
  const base = [
    styles.button,
    compact && styles.buttonCompact,
    full && styles.buttonFull,
    { borderRadius: radius.control },
    variant === "soft" && { backgroundColor: toneSoftColor(colors, tone as RudiTone), borderColor: "transparent" },
    // The outline is the button's own tone on split/ai screens; `lineStrong`
    // (a warm neutral) only on accent, where it is the brand world's line.
    variant === "outline" && { backgroundColor: colors.card, borderColor: tone === "accent" ? colors.lineStrong : mauTone },
    variant === "ghost" && { backgroundColor: "transparent", borderColor: "transparent" },
    tat && { backgroundColor: colors.card, borderColor: vienTat, borderStyle: "dashed" as const, borderWidth: 1.5 },
    style,
  ];
  const body = (
    <>
      {loading ? <ActivityIndicator color={foreground} size="small" /> : null}
      {!loading && icon ? <Ionicons color={foreground} name={icon} size={compact ? 18 : 20} /> : null}
      <Text numberOfLines={1} style={[typography.label, styles.buttonLabel, { color: foreground }]}>
        {label}
      </Text>
    </>
  );

  const nut = (
    // Press feedback is a spring on the UI thread (scale 1 -> 0.98), the
    // `instant` step of the motion vocabulary; the old opacity dim ran on the
    // JS thread and could not honour Reduce Motion.
    <PressScale
      accessibilityHint={tat && lyDo ? lyDo : undefined}
      accessibilityLabel={accessibilityLabel}
      accessibilityRole="button"
      accessibilityState={{ disabled: disabled || loading, busy: loading }}
      disabled={disabled || loading}
      onPress={onPress}
      pressedScale={0.98}
      style={base}
    >
      {solid ? (
        <LinearGradient
          colors={
            // Scheme tokens, not the brand's light-only pair: in dark the
            // primary must be the brightest actionable thing on the screen.
            tone === "accent" ? [colors.accent, colors.accentEnd] : [mauTone, mauTone]
          }
          end={{ x: 1, y: 0.6 }}
          start={{ x: 0, y: 0 }}
          style={[StyleSheet.absoluteFill, { borderRadius: radius.control }]}
        />
      ) : null}
      {body}
    </PressScale>
  );
  if (!tat || !lyDo) return nut;
  return (
    <View style={[styles.nutCoLyDo, full ? styles.nutCoLyDoFull : null]}>
      {nut}
      <View importantForAccessibility="no-hide-descendants" style={styles.lyDo}>
        <Ionicons color={chuTat} name="information-circle-outline" size={16} />
        <Text style={[typography.caption, styles.lyDoChu, { color: chuTat }]}>{lyDo}</Text>
      </View>
    </View>
  );
}

export function IconButton({
  icon,
  onPress,
  accessibilityLabel,
  selected = false,
  quiet = false,
  solid = false,
  dim = false,
  loading = false,
  disabled = false,
  tone = "accent",
}: {
  icon: IconName;
  onPress?: () => void;
  accessibilityLabel: string;
  selected?: boolean;
  quiet?: boolean;
  /** The surface's primary action: tone fill, ink-on-tone glyph. */
  solid?: boolean;
  /** Nothing to act on yet: faint glyph, no border. */
  dim?: boolean;
  loading?: boolean;
  disabled?: boolean;
  tone?: RudiTone;
}) {
  const { colors } = useRudiTheme();
  // ADR-0038 §2.2: a primary icon with nothing to act on yet (send, with the
  // box empty) is not a full coral disc. It is the same outline every button
  // takes when it cannot be used yet: dashed edge, soft ink glyph.
  const tatSolid = solid && disabled && !loading;
  const background = tatSolid
    ? colors.card
    : solid
    ? toneColor(colors, tone)
    : selected
      ? toneSoftColor(colors, tone)
      : quiet || dim
        ? "transparent"
        : colors.card;
  const glyph = tatSolid
    ? colors.inkSoft
    : solid
    ? colors[`${tone}Ink` as const]
    : selected
      ? toneColor(colors, tone)
      : dim
        ? colors.inkFaint
        : colors.ink;
  return (
    <PressScale
      accessibilityLabel={accessibilityLabel}
      accessibilityRole="button"
      aria-busy={loading}
      aria-disabled={disabled || loading}
      {...giuState(Boolean(selected))}
      disabled={disabled || loading}
      hitSlop={4}
      onPress={onPress}
      pressedScale={0.94}
      style={[
        styles.iconButton,
        { backgroundColor: background, borderColor: quiet || dim || solid ? "transparent" : colors.line },
        tatSolid && { borderColor: colors.lineStrong, borderStyle: "dashed" as const, borderWidth: 1.5 },
      ]}
    >
      {loading ? <ActivityIndicator color={glyph} size="small" /> : <Ionicons color={glyph} name={icon} size={22} />}
    </PressScale>
  );
}

/**
 * The kit's text field. The box, the input and the placeholder live in
 * `ui/Field.tsx`, which imports nothing native so a node test can render it
 * (audit native 09/09, F44); this wrapper only turns an icon NAME into the
 * `Ionicons` element that file cannot import.
 *
 * A `const`, not a `function` declaration, on purpose: the non-text contrast
 * gate (`services/api/tests/web/test_contrast_floor.py`) reads the kit as text
 * and takes the FIRST exported function named Field as the component that owns
 * the control boundary. That is the core in `ui/Field.tsx`, where the
 * `lineStrong` border is declared; this glue owns no border and must not be
 * the block the gate measures -- nor may this comment spell the marker out.
 */
export const Field = ({ icon, ...props }: FieldCoreProps & { icon?: IconName }) => {
  const { colors } = useRudiTheme();
  return <FieldCore {...props} leading={icon ? <Ionicons color={colors.inkFaint} name={icon} size={20} /> : undefined} />;
};

export function SearchField({ placeholder = "Tìm quán, món…", ...props }: TextInputProps & { oRef?: FieldCoreProps["oRef"] }) {
  return <Field {...props} icon="search-outline" placeholder={placeholder} returnKeyType="search" />;
}

/**
 * Six boxes for a one-time code, one real input behind them.
 *
 * The boxes are paint; the `TextInput` stretched over them is what has focus,
 * receives the SMS autofill (`autoComplete="sms-otp"` / `oneTimeCode`) and
 * what a driver types into. One input rather than six keeps paste, autofill and
 * backspace ordinary, and keeps the value a single string the caller submits
 * when it reaches `length`. Its text is transparent, not its opacity: an
 * element with opacity 0 is also invisible to the accessibility tree.
 */
export function OtpBoxes({
  value,
  onChange,
  length = 6,
  disabled = false,
}: {
  value: string;
  onChange: (next: string) => void;
  length?: number;
  disabled?: boolean;
}) {
  const { colors, radius } = useRudiTheme();
  const oHienTai = Math.min(value.length, length - 1);
  return (
    <View style={styles.otpWrap}>
      <View pointerEvents="none" style={styles.otpRow}>
        {Array.from({ length }, (_, i) => (
          <View
            key={i}
            style={[
              styles.otpBox,
              {
                backgroundColor: colors.card,
                borderColor: i === oHienTai && !disabled ? colors.accent : colors.lineStrong,
                borderRadius: radius.control,
              },
            ]}
          >
            <Text style={[typography.title, { color: colors.ink }]}>{value[i] ?? ""}</Text>
          </View>
        ))}
      </View>
      <TextInput
        accessibilityLabel="Ô nhập mã"
        autoComplete="sms-otp"
        autoFocus
        caretHidden
        editable={!disabled}
        keyboardType="number-pad"
        maxLength={length}
        onChangeText={(text) => onChange(text.replace(/\D/g, "").slice(0, length))}
        style={[styles.otpInput, KHONG_VIEN_WEB]}
        testID="otp-input"
        textContentType="oneTimeCode"
        value={value}
      />
    </View>
  );
}

export function Chip({
  label,
  icon,
  leading,
  selected = false,
  tone = "accent",
  onPress,
  accessibilityLabel,
}: {
  label: string;
  icon?: IconName;
  /** Authored artwork in the icon's place (a `GuGlyph`), so a taxonomy is drawn with one pen everywhere. */
  leading?: ReactNode;
  selected?: boolean;
  tone?: RudiTone;
  onPress?: () => void;
  /** When the same label appears on several rows, say which row this one is. */
  accessibilityLabel?: string;
}) {
  const { colors, radius } = useRudiTheme();
  const foreground = selected ? toneColor(colors, tone) : colors.inkSoft;
  if (onPress === undefined) {
    // A fact, not a control: no role, no pressed state, no 48dp box. A chip that
    // cannot be tapped must not announce itself as a button.
    return (
      <View
        accessibilityLabel={accessibilityLabel}
        style={[
          styles.chipTinh,
          {
            backgroundColor: selected ? toneSoftColor(colors, tone) : colors.card,
            borderColor: selected ? toneColor(colors, tone) : colors.lineStrong,
            borderRadius: radius.small,
          },
        ]}
      >
        {leading ?? (icon ? <Ionicons color={foreground} name={icon} size={14} /> : null)}
        <Text numberOfLines={1} style={[typography.caption, styles.chipChu, { color: foreground }]}>
          {label}
        </Text>
      </View>
    );
  }
  return (
    <PressScale
      accessibilityLabel={accessibilityLabel}
      accessibilityRole="button"
      aria-pressed={selected}
      onPress={onPress}
      pressedScale={0.96}
      style={[
        styles.chip,
        {
          backgroundColor: selected ? toneSoftColor(colors, tone) : colors.card,
          // DESIGN.md: an unselected chip's boundary is `lineStrong` (>= 3:1 on
          // card and ground); `line` is a hairline for dividers, not a control edge.
          borderColor: selected ? toneColor(colors, tone) : colors.lineStrong,
          borderRadius: radius.pill,
        },
      ]}
    >
      {/* Selected is said twice: fill and a check, so it does not rest on color alone. */}
      {leading ?? (icon ? <Ionicons color={foreground} name={icon} size={16} /> : null)}
      {icon === undefined && leading === undefined && selected ? <Ionicons color={foreground} name="checkmark" size={16} /> : null}
      <Text numberOfLines={1} style={[typography.caption, styles.chipChu, { color: foreground }]}>
        {label}
      </Text>
    </PressScale>
  );
}

export function Avatar({
  person,
  size = 44,
  ring = false,
}: {
  person: DemoPerson;
  size?: number;
  ring?: boolean;
}) {
  const { colors } = useRudiTheme();
  return (
    <View
      accessibilityLabel={person.name}
      style={[
        styles.avatar,
        {
          width: size,
          height: size,
          borderRadius: size / 2,
          backgroundColor: person.color,
          borderColor: ring ? colors.accent : colors.card,
          borderWidth: ring ? 2 : 2,
        },
      ]}
    >
      <Text style={[styles.avatarText, { fontSize: Math.max(10, size * 0.3) }]}>{person.initials}</Text>
    </View>
  );
}

export function AvatarStack({ people, max = 4 }: { people: DemoPerson[]; max?: number }) {
  const { colors } = useRudiTheme();
  const visible = people.slice(0, max);
  const remaining = people.length - visible.length;
  return (
    <View style={styles.avatarStack}>
      {visible.map((person, index) => (
        <View key={person.id} style={{ marginLeft: index ? -10 : 0, zIndex: visible.length - index }}>
          <Avatar person={person} size={34} />
        </View>
      ))}
      {remaining > 0 ? (
        <View style={[styles.avatarMore, { backgroundColor: colors.ink, borderColor: colors.card }]}>
          <Text style={styles.avatarMoreText}>+{remaining}</Text>
        </View>
      ) : null}
    </View>
  );
}

/**
 * A picture the group owns: an authored asset, or a photograph the group took
 * and the server released to a member. Never a catalogue photograph -- those
 * are `AnhCoGhiCong`, which does not hand out an address on its own, so one
 * cannot reach this frame and lose its credit on the way (F31).
 */
export function Photo({
  source,
  height = 190,
  ratio,
  radius = 20,
  overlay,
  style,
  contentFit = "cover",
}: {
  source: ImageSource;
  height?: number;
  /** Width-driven frame (aspect ratio) instead of a fixed height, so a tile stays square in any column width. */
  ratio?: number;
  radius?: number;
  overlay?: ReactNode;
  style?: StyleProp<ViewStyle>;
  contentFit?: "cover" | "contain";
}) {
  const { colors } = useRudiTheme();
  // The ground before the picture arrives is the theme's paper, so a slow or
  // failed load on the dark scheme is a dark frame, not a light slab.
  return (
    <View style={[styles.photo, ratio !== undefined ? { aspectRatio: ratio } : { height }, { borderRadius: radius, backgroundColor: colors.card }, style]}>
      <Image contentFit={contentFit} source={source} style={StyleSheet.absoluteFill} transition={180} />
      {overlay}
    </View>
  );
}

export function PhotoShade({ children }: { children: ReactNode }) {
  return (
    <LinearGradient
      colors={["transparent", lopPhu.xam(0.78)]}
      end={{ x: 0.5, y: 1 }}
      start={{ x: 0.5, y: 0.3 }}
      style={[StyleSheet.absoluteFill, styles.photoShade]}
    >
      {children}
    </LinearGradient>
  );
}

export function Stat({
  value,
  label,
  icon,
  tone = "accent",
}: {
  value: string;
  label: string;
  icon?: IconName;
  tone?: RudiTone;
}) {
  const { colors } = useRudiTheme();
  return (
    <View style={styles.stat}>
      {icon ? (
        <View style={[styles.statIcon, { backgroundColor: toneSoftColor(colors, tone) }]}>
          <Ionicons color={toneColor(colors, tone)} name={icon} size={19} />
        </View>
      ) : null}
      {/* Never shrunk to fit: a figure that does not fit wraps to its own line (report §7.1). */}
      <Text style={[typography.money, { color: colors.ink }]}>{value}</Text>
      <Text style={[typography.caption, { color: colors.inkFaint }]}>{label}</Text>
    </View>
  );
}

/**
 * A note in the margin, in violet ink: what the assistant would add, said in
 * one sentence and signed. No fill and no label over the sentence (a kicker):
 * a hairline above and below in the AI tone marks it as an aside on the page.
 */
export function AiNote({ children }: { children: ReactNode }) {
  const { colors } = useRudiTheme();
  return (
    <View style={[styles.aiNote, { borderColor: colors.ai }]}>
      <Ionicons color={colors.ai} name="sparkles" size={17} style={styles.aiIcon} />
      <View style={styles.flex}>
        <Text style={[typography.label, styles.aiText, { color: colors.ink }]}>{children}</Text>
        <Text style={[typography.caption, { color: colors.ai }]}>Rủ Đi AI gợi ý</Text>
      </View>
    </View>
  );
}

export function ProgressBar({ value, tone = "accent" }: { value: number; tone?: RudiTone }) {
  const { colors } = useRudiTheme();
  return (
    <View style={[styles.progressTrack, { backgroundColor: colors.line }]}>
      <View
        style={[
          styles.progressFill,
          { backgroundColor: toneColor(colors, tone), width: `${Math.min(100, Math.max(0, value))}%` },
        ]}
      />
    </View>
  );
}

export function Segmented({
  items,
  selected,
  onSelect,
  tone = "accent",
  testIDs,
}: {
  items: string[];
  selected: number;
  onSelect: (index: number) => void;
  tone?: RudiTone;
  testIDs?: (string | undefined)[];
}) {
  const { colors, radius } = useRudiTheme();
  return (
    <View {...TABLIST} style={[styles.segmented, { backgroundColor: colors.card, borderColor: colors.line, borderRadius: radius.control }]}>
      {items.map((item, index) => {
        const active = selected === index;
        return (
          <Pressable
            key={item}
            accessibilityLabel={item}
            accessibilityRole="tab"
            aria-selected={active}
            onPress={() => onSelect(index)}
            testID={testIDs?.[index]}
            style={[
              styles.segment,
              active && { backgroundColor: toneSoftColor(colors, tone), borderRadius: radius.small },
            ]}
          >
            <Text style={[typography.caption, styles.segmentLabel, { color: active ? toneColor(colors, tone) : colors.inkFaint }]}>
              {item}
            </Text>
          </Pressable>
        );
      })}
    </View>
  );
}

export function ListRow({
  icon,
  title,
  subtitle,
  trailing,
  tone = "accent",
  onPress,
  chevron = true,
}: {
  icon: IconName;
  title: string;
  subtitle?: string;
  trailing?: ReactNode;
  tone?: RudiTone;
  onPress?: () => void;
  /** False for a row that acts on the spot (copy, reply): a chevron promises somewhere to go. */
  chevron?: boolean;
}) {
  const { colors } = useRudiTheme();
  return (
    <PressScale
      accessibilityRole={onPress ? "button" : undefined}
      disabled={onPress === undefined}
      onPress={onPress}
      pressedScale={0.985}
      style={styles.listRow}
    >
      <View style={[styles.listIcon, { backgroundColor: toneSoftColor(colors, tone) }]}>
        <Ionicons color={toneColor(colors, tone)} name={icon} size={20} />
      </View>
      <View style={styles.listText}>
        <Text style={[typography.label, { color: colors.ink }]}>{title}</Text>
        {subtitle ? <Text style={[typography.caption, { color: colors.inkFaint }]}>{subtitle}</Text> : null}
      </View>
      {trailing ?? (onPress && chevron ? <Ionicons color={colors.inkFaint} name="chevron-forward" size={19} /> : null)}
    </PressScale>
  );
}

export function FloatingGlass({ children, style }: { children: ReactNode; style?: StyleProp<ViewStyle> }) {
  const { dark, radius } = useRudiTheme();
  return (
    <BlurView
      intensity={Platform.OS === "android" ? 35 : 65}
      tint={dark ? "dark" : "light"}
      style={[styles.floatingGlass, { borderRadius: radius.base }, style]}
    >
      {children}
    </BlurView>
  );
}

export function ResponsiveRow({
  children,
  minItemWidth = 250,
  gap = 12,
  maxColumns = 3,
}: {
  children: ReactNode;
  minItemWidth?: number;
  gap?: number;
  /** Most items per row; thumbnails may go past the three-card default. */
  maxColumns?: number;
}) {
  const [width, setWidth] = useState(0);
  const { itemWidth } = gridFor(width, minItemWidth, gap, maxColumns);
  return (
    <View onLayout={(event) => setWidth(event.nativeEvent.layout.width)} style={{ flexDirection: "row", flexWrap: "wrap", gap }}>
      {Children.toArray(children).map((child, index) => (
        <View key={typeof child === "object" && child !== null && "key" in child ? child.key ?? index : index} style={{ width: width > 0 ? itemWidth : "100%", minWidth: 0 }}>{child}</View>
      ))}
    </View>
  );
}

export function Divider() {
  const { colors } = useRudiTheme();
  return <View style={[styles.divider, { backgroundColor: colors.line }]} />;
}

/**
 * Rows on paper. Each child sits on a hairline `line`; there is no card, no
 * shadow, no radius -- «Hàng + kẻ tóc là container mặc định» (DESIGN.md), the
 * shape `Profile`, `DiemDenScreen` and `HangDiaDiem` already draw by hand.
 * Extracted (11/09, re-audit R5) when the Settings family was still stacking
 * eight `Card`s next to those rows, two surface systems on neighbouring screens.
 */
export function NhomHang({ children, style }: { children: ReactNode; style?: StyleProp<ViewStyle> }) {
  const { colors } = useRudiTheme();
  const muc = Children.toArray(children).filter(Boolean);
  return (
    <View style={style}>
      {muc.map((con, i) => (
        <View key={i} style={[styles.nhomHangMuc, { borderBottomColor: colors.line }]}>{con}</View>
      ))}
    </View>
  );
}

export function Inline({
  children,
  gap = 8,
  wrap = false,
  style,
}: {
  children: ReactNode;
  gap?: number;
  wrap?: boolean;
  style?: StyleProp<ViewStyle>;
}) {
  return <View style={[styles.inline, { gap }, wrap && styles.wrap, style]}>{children}</View>;
}

export function Spacer({ size = 16 }: { size?: number }) {
  return <View style={{ height: size }} />;
}

export function SurfaceLabel({ children }: { children: ReactNode }) {
  const { colors } = useRudiTheme();
  return <Text style={[typography.caption, styles.surfaceLabel, { color: colors.inkFaint }]}>{children}</Text>;
}

export function widthPercent(value: number): DimensionValue {
  return (value + "%") as DimensionValue;
}

const styles = StyleSheet.create({
  otpWrap: { position: "relative", alignSelf: "center" },
  otpRow: { flexDirection: "row", gap: 8, justifyContent: "center" },
  otpBox: { width: 44, height: 54, borderWidth: 1.5, alignItems: "center", justifyContent: "center" },
  otpInput: {
    position: "absolute",
    left: 0,
    right: 0,
    top: 0,
    bottom: 0,
    color: "transparent",
    backgroundColor: "transparent",
    fontSize: 1,
  },
  flex: { flex: 1 },
  safeArea: { flex: 1, overflow: "hidden" },
  paper: { position: "absolute", left: 0, right: 0, top: 0, bottom: 0 },
  screenInner: { width: "100%", gap: 18, paddingTop: 8 },
  screenHeader: { borderBottomWidth: StyleSheet.hairlineWidth },
  screenHeaderTrong: { borderBottomWidth: 0 },
  screenFooter: { width: "100%", paddingTop: 8, zIndex: 2 },
  // `width: 100%` so the column fills up to its `maxWidth`; centred without it,
  // the column shrank to whatever its widest child happened to be.
  tabletInner: { alignSelf: "center", width: "100%", maxWidth: 960, paddingTop: 22 },
  topBar: { minHeight: 52, flexDirection: "row", alignItems: "center", justifyContent: "space-between" },
  // Sides are at least the 48dp target wide and always equal (see TopBar), so
  // the title is centred on the screen even next to a badge.
  topBarSide: { minWidth: 52, flexShrink: 0, alignItems: "flex-start" },
  topBarSideInner: { alignSelf: "flex-start" },
  // The right content hugs the right edge when the left side is the wider one (Logo instead of a chevron).
  topBarSideInnerRight: { alignItems: "center", alignSelf: "flex-end", flexDirection: "row", gap: 4 },
  topBarRight: { alignItems: "flex-end" },
  topBarTitleWrap: { flex: 1, alignItems: "center", paddingHorizontal: 8 },
  topBarTitle: { textAlign: "center" },
  doChu: { position: "absolute", left: 0, top: 0, opacity: 0 },
  cuaDemo: { minHeight: 48, minWidth: 48, justifyContent: "center" },
  cuaDemoChip: { flexDirection: "row", alignItems: "center", gap: 4, borderWidth: 1, borderRadius: 999, paddingHorizontal: 8, paddingVertical: 5 },
  cuaDemoChu: { fontSize: 12, lineHeight: 14, fontWeight: "700", letterSpacing: 0.2 },
  logoRow: { flexDirection: "row", alignItems: "center", flexShrink: 0, gap: 9 },
  logoMark: { width: 48, height: 48, borderRadius: 17, alignItems: "center", justifyContent: "center", transform: [{ rotate: "-4deg" }] },
  logoMarkCompact: { width: 40, height: 40, borderRadius: 14 },
  logoMarkType: { color: mucTrenAnh, fontFamily: displayFace.extraBold, fontSize: 14, lineHeight: 13, letterSpacing: -0.6, textAlign: "center", transform: [{ skewX: "-9deg" }] },
  logoMarkTypeCompact: { fontSize: 12, lineHeight: 11 },
  eyebrow: { alignSelf: "flex-start", flexDirection: "row", alignItems: "center", gap: 7, paddingHorizontal: 11, paddingVertical: 7, borderRadius: 999 },
  eyebrowDot: { width: 6, height: 6, borderRadius: 3 },
  demoBadge: { flexDirection: "row", alignItems: "center", alignSelf: "flex-start", gap: 5, borderWidth: 1, borderRadius: 999, paddingHorizontal: 8, paddingVertical: 5 },
  demoText: { fontSize: 10, lineHeight: 12, fontWeight: "700", letterSpacing: 0.2 },
  heading: { gap: 8, maxWidth: 620 },
  center: { alignSelf: "center", alignItems: "center" },
  headingSubtitle: { maxWidth: 560 },
  sectionHeader: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", gap: 12, marginTop: 3 },
  // A text action is still a target: 48dp tall, so a 13dp glyph is not the whole hit box.
  sectionAction: { minHeight: 48, justifyContent: "center", paddingHorizontal: 6 },
  card: { borderWidth: 1, padding: 16 },
  cardPressed: { opacity: 0.94, transform: [{ scale: 0.992 }] },
  pressed: { opacity: 0.68 },
  // `minWidth` pairs with `minHeight`: a content-sized button («Bỏ», `full={false}`)
  // measured 47.2dp wide on device once it stopped being full-width (finish
  // review 10/09, F41). The floor lives here so every future short label gets it.
  button: { minHeight: 52, minWidth: 48, flexShrink: 0, overflow: "hidden", borderWidth: 1, borderColor: "transparent", paddingHorizontal: 18, flexDirection: "row", alignItems: "center", justifyContent: "center", gap: 9 },
  buttonFull: { width: "100%" },
  buttonCompact: { minHeight: 48, paddingHorizontal: 14 },
  buttonLabel: { zIndex: 1 },
  buttonPressed: { opacity: 0.82, transform: [{ scale: 0.98 }] },
  nutCoLyDo: { gap: 6, alignSelf: "flex-start" },
  nutCoLyDoFull: { alignSelf: "stretch" },
  lyDo: { flexDirection: "row", alignItems: "flex-start", gap: 6, paddingHorizontal: 4 },
  lyDoChu: { flexShrink: 1 },
  iconButton: { width: 48, height: 48, borderRadius: 16, borderWidth: 1, alignItems: "center", justifyContent: "center" },
  // `maxWidth`: a chip never grows past the row that holds it; a long place
  // name ellipsizes inside it instead of running off the sheet (QA UI-099).
  chipTinh: { minHeight: 30, maxWidth: "100%", flexShrink: 0, borderWidth: 1, flexDirection: "row", alignItems: "center", gap: 5, paddingHorizontal: 9, paddingVertical: 5 },
  chip: { minHeight: 48, maxWidth: "100%", flexShrink: 0, borderWidth: 1, flexDirection: "row", alignItems: "center", justifyContent: "center", gap: 6, paddingHorizontal: 12, paddingVertical: 10 },
  chipChu: { flexShrink: 1 },
  avatar: { alignItems: "center", justifyContent: "center" },
  avatarText: { color: mucTrenAnh, fontWeight: "800", letterSpacing: -0.2 },
  avatarStack: { flexDirection: "row", alignItems: "center" },
  avatarMore: { width: 34, height: 34, marginLeft: -10, borderRadius: 17, borderWidth: 2, alignItems: "center", justifyContent: "center" },
  avatarMoreText: { color: mucTrenAnh, fontSize: 10, fontWeight: "800" },
  photo: { position: "relative", overflow: "hidden" },
  photoShade: { justifyContent: "flex-end", padding: 16 },
  stat: { flex: 1, minWidth: 88, alignItems: "center", gap: 4, paddingVertical: 5 },
  statIcon: { width: 38, height: 38, borderRadius: 13, alignItems: "center", justifyContent: "center", marginBottom: 2 },
  aiNote: { flexDirection: "row", gap: 11, paddingVertical: 12, borderTopWidth: StyleSheet.hairlineWidth, borderBottomWidth: StyleSheet.hairlineWidth },
  aiIcon: { width: 32, height: 32, borderRadius: 11, alignItems: "center", justifyContent: "center" },
  aiText: { marginTop: 2 },
  progressTrack: { height: 8, borderRadius: 999, overflow: "hidden" },
  progressFill: { height: "100%", borderRadius: 999 },
  segmented: { flexDirection: "row", padding: 4, borderWidth: 1 },
  // Vertical padding so a label that wraps at large text («Theo hệ thống» at 2.0) keeps air above and below instead of filling the segment edge to edge (finish review 11/09).
  segment: { flex: 1, minHeight: 48, paddingHorizontal: 6, paddingVertical: 8, alignItems: "center", justifyContent: "center" },
  segmentLabel: { textAlign: "center" },
  listRow: { minHeight: 58, flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 7 },
  listIcon: { width: 40, height: 40, borderRadius: 13, alignItems: "center", justifyContent: "center" },
  listText: { flex: 1, gap: 2 },
  floatingGlass: { overflow: "hidden", borderWidth: 1, borderColor: lopPhu.trang(0.7) },
  responsiveRow: { flexDirection: "row", alignItems: "stretch" },
  responsiveColumn: { flexDirection: "column" },
  divider: { width: "100%", height: StyleSheet.hairlineWidth },
  nhomHangMuc: { paddingVertical: 6, borderBottomWidth: StyleSheet.hairlineWidth },
  inline: { flexDirection: "row", alignItems: "center" },
  wrap: { flexWrap: "wrap" },
  surfaceLabel: { textTransform: "uppercase", letterSpacing: 0.7 },
});
