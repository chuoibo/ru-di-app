/**
 * A paper stage as the head of a screen (ADR-0037 D1, plan S0.3).
 *
 * Two pieces, one scroll:
 *   - `CanhGap` goes in `RudiScreen`'s `canh` slot, first in the list: the
 *     stage stands up once when the screen opens, and folds back into the page
 *     as the list scrolls (near layers first, as a pop-up page closing), with
 *     the screen's title printed on the page under it;
 *   - `ThanhCanh` goes in the `header` slot, above the list: the back button,
 *     which never moves, and a compact title that only appears once the big
 *     one has gone under the bar -- so the title is on screen exactly once.
 *
 * The title and subtitle are React Native text on the page, never in the
 * canvas (ADR-0037 D9); the stage itself is decoration unless `coMoTa` asks a
 * screen reader to read its one sentence. With no room for a stage (a phone on
 * its side) the page keeps only its words.
 */
import { useEffect, useRef, useState, type ReactNode } from "react";
import { StyleSheet, Text, View, useWindowDimensions, type LayoutChangeEvent } from "react-native";
import { GestureDetector } from "react-native-gesture-handler";
import Animated, { useAnimatedStyle, useDerivedValue, useSharedValue } from "react-native-reanimated";

import type { SanKhau as SanKhauData } from "../art/san-khau";
import { kichThuocSanKhau } from "../san-khau/kich-thuoc";
import { typography, useRudiTheme } from "../theme";
import { TopBar } from "../ui";
import { useCuonManHinh } from "./cuon";
import { SanKhau } from "./SanKhau";
import { useAdaptiveLayout } from "./useAdaptiveLayout";
import { useThiSaiKeo } from "./useThiSai";

/** The list has folded the stage flat once it has scrolled this share of the stage's height. */
const GAP_HET = 0.7;
/** The compact title fades in over the last stretch of the big title going under the bar. */
const CHU_VAO = 24;
/** The gap between the stage and the words under it (`styles.khoi.gap`). */
const KHOANG = 10;

export interface CanhGapProps {
  /** The stage, or a builder for it that takes the art's compact reading. */
  san: SanKhauData | ((gon: boolean) => SanKhauData);
  tieuDe: string;
  phuDe?: string;
  /** A short stamp-cased line above the title (the group, the city). */
  nhanTren?: string;
  /** Let the reader lean the stage sideways with a finger (parallax). */
  keo?: boolean;
  /** Read the stage's sentence to a screen reader. */
  coMoTa?: boolean;
  /** Anything that belongs under the title on the page (a stamp, a line of state). */
  children?: ReactNode;
  testID?: string;
}

export function CanhGap({ san, tieuDe, phuDe, nhanTren, keo = false, coMoTa = false, children, testID }: CanhGapProps) {
  const { colors } = useRudiTheme();
  const layout = useAdaptiveLayout();
  const { height: caoCuaSo, fontScale } = useWindowDimensions();
  const cuon = useCuonManHinh();
  const [rong, setRong] = useState(0);
  // Where the big title ends in the list, built from parts that each only
  // change with a SIZE: the block's top, the stage's height and the title's
  // bottom inside the words. A view that only MOVES reports no new layout on
  // the web (react-native-web measures with a ResizeObserver), so reading the
  // title's y after the stage appeared above it kept the stale, stage-less
  // position (evidence run 24/09: the bar took the title 130dp early).
  const viTri = useRef({ yKhoi: 0, cuoiTieuDe: 0 });
  const { thiSai, cuChi } = useThiSaiKeo(keo);

  const mau = typeof san === "function" ? san(false) : san;
  const kt = rong > 0
    ? kichThuocSanKhau({ rongCho: rong, caoCuaSo, tiLe: mau.khung.w / mau.khung.h, sizeClass: layout.sizeClass, heightClass: layout.heightClass, fontScale })
    : null;
  const sanVe = kt && typeof san === "function" && kt.gon ? san(true) : mau;
  const caoSan = kt?.h ?? 0;

  const gap = useDerivedValue(() => {
    if (!cuon || caoSan <= 0) return 0;
    return Math.min(1, Math.max(0, cuon.cuonY.value / (caoSan * GAP_HET)));
  });

  const baoNguong = () => {
    if (cuon) cuon.nguongTieuDe.value = viTri.current.yKhoi + (caoSan > 0 ? caoSan + KHOANG : 0) + viTri.current.cuoiTieuDe;
  };
  const doKhoi = (e: LayoutChangeEvent) => {
    const { width, y } = e.nativeEvent.layout;
    setRong((cu) => (Math.abs(cu - width) < 1 ? cu : width));
    viTri.current.yKhoi = y;
    baoNguong();
  };
  const doTieuDe = (e: LayoutChangeEvent) => {
    const { y, height } = e.nativeEvent.layout;
    viTri.current.cuoiTieuDe = y + height;
    baoNguong();
  };
  // The stage's height arrives after the first layout (it needs the width).
  useEffect(baoNguong, [caoSan]); // eslint-disable-line react-hooks/exhaustive-deps

  const sanKhau = kt ? (
    <View style={styles.giuaSan}>
      <SanKhau coMoTa={coMoTa} gap={gap} san={sanVe} testID={testID ? `${testID}-san` : undefined} thiSai={thiSai} width={kt.w} />
    </View>
  ) : null;

  return (
    <View onLayout={doKhoi} style={styles.khoi} testID={testID}>
      {sanKhau && keo ? <GestureDetector gesture={cuChi}>{sanKhau}</GestureDetector> : sanKhau}
      <View style={styles.chu}>
        {/* Condensed caps stack two marks over a capital («SỔ»): one clipped line needs the room above. */}
        {nhanTren ? (
          <Text numberOfLines={1} style={[typography.stamp, { lineHeight: 18, color: colors.inkSoft }]}>
            {nhanTren}
          </Text>
        ) : null}
        <Text accessibilityRole="header" onLayout={doTieuDe} style={[typography.display, { color: colors.ink }]}>
          {tieuDe}
        </Text>
        {phuDe ? <Text style={[typography.body, { color: colors.inkSoft }]}>{phuDe}</Text> : null}
        {children}
      </View>
    </View>
  );
}

/**
 * The bar above a staged screen: the back button, still, and the title, which
 * only shows once the stage's big title has scrolled under it.
 */
export function ThanhCanh({ tieuDe, back = true, onBack, phai }: { tieuDe: string; back?: boolean; onBack?: () => void; phai?: ReactNode }) {
  const { colors } = useRudiTheme();
  const cuon = useCuonManHinh();
  const hien = useDerivedValue(() => {
    if (!cuon) return 1;
    return Math.min(1, Math.max(0, (cuon.cuonY.value - (cuon.nguongTieuDe.value - CHU_VAO)) / CHU_VAO));
  });
  const kieuChu = useAnimatedStyle(() => ({ opacity: hien.value }));
  const kieuVach = useAnimatedStyle(() => ({ opacity: hien.value }));
  // A stable width for the side slots is TopBar's job; the compact title is
  // laid over its empty centre, so the back button keeps TopBar's exact place.
  const [rongBen, setRongBen] = useState(56);
  return (
    <View>
      <TopBar back={back} onBack={onBack} right={phai ? <View onLayout={(e) => setRongBen(Math.max(56, Math.ceil(e.nativeEvent.layout.width)))}>{phai}</View> : undefined} />
      <View pointerEvents="none" style={[StyleSheet.absoluteFill, styles.giuaThanh, { paddingHorizontal: rongBen }]}>
        <Animated.Text
          accessibilityElementsHidden
          importantForAccessibility="no-hide-descendants"
          numberOfLines={1}
          style={[typography.title, { color: colors.ink }, kieuChu]}
          testID="thanh-canh-tieu-de"
        >
          {tieuDe}
        </Animated.Text>
      </View>
      <Animated.View pointerEvents="none" style={[styles.vach, { backgroundColor: colors.line }, kieuVach]} />
    </View>
  );
}

const styles = StyleSheet.create({
  khoi: { gap: KHOANG },
  giuaSan: { alignItems: "center" },
  chu: { gap: 4 },
  giuaThanh: { alignItems: "center", justifyContent: "center" },
  vach: { position: "absolute", left: 0, right: 0, bottom: 0, height: StyleSheet.hairlineWidth },
});
