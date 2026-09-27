/**
 * Choose the renderer for one piece of art: Skia when it can draw, the SVG
 * drawing of the same geometry until then and whenever it cannot (ADR-0037 D10).
 *
 * The Skia component is always reached through `luoiSkia(() => import(...))`,
 * never a static import: on the web the library may not be evaluated before
 * CanvasKit has loaded, and on native a dev client built before the campaign
 * has no Skia module at all. Loading failure, a missing module or a render
 * error all land on the SVG drawing, never on a blank or a crash.
 *
 * Animation state is NOT owned by either renderer: the caller keeps the shared
 * values (a stage's `mo`, a puppet's clock) and hands them to both, so a swap
 * from SVG to Skia in the middle of a pop-up continues the same motion instead
 * of restarting it.
 *
 * The hand-over is a cross-fade, never a cut: the SVG drawing stays in place
 * while the Skia canvas mounts on top of it at opacity 0, and only once that
 * canvas has had two frames to paint does it fade in (`standard`) and the SVG
 * leave. A canvas's first frame can land a frame or two after it mounts
 * (CanvasKit surface set-up on the web), and a straight swap showed exactly
 * that as a blank stage (evidence run 24/09). Under Reduce Motion the fade is
 * a cut, still after the paint.
 *
 * Each branch marks itself with `data-renderer` on the web (`dataSet`), inner
 * to the branch that actually painted, so an evidence run can prove which one
 * drew (`tools/xem-san-khau.mjs` fails unless it reads `skia`).
 */
import { Component, Suspense, lazy, useEffect, useState, type ComponentType, type ReactNode } from "react";
import { StyleSheet, View, type StyleProp, type ViewStyle } from "react-native";
import Animated, { runOnJS, useAnimatedStyle, useSharedValue, withTiming } from "react-native-reanimated";

import { napSkia, useTrangThaiSkia } from "./skia/nap-skia";
import { useMotion } from "./useMotion";

/** A lazy Skia component that only loads once `napSkia()` says Skia can draw. */
export function luoiSkia<P extends object>(nap: () => Promise<{ default: ComponentType<P> }>): ComponentType<P> {
  return lazy(async () => {
    const coSkia = await napSkia();
    if (!coSkia) throw new Error("skia-khong-ve-duoc");
    return nap();
  });
}

/** Catches a Skia render error; the SVG drawing, still mounted under it, stays. */
class BatLoiSkia extends Component<{ onLoi: () => void; children: ReactNode }, { hong: boolean }> {
  state = { hong: false };

  static getDerivedStateFromError() {
    return { hong: true };
  }

  componentDidCatch() {
    this.props.onLoi();
  }

  render() {
    return this.state.hong ? null : this.props.children;
  }
}

/**
 * The Skia layer, mounted invisible over the SVG drawing: two frames for the
 * canvas to paint, then a fade in, then `onXong` lets the SVG go.
 */
function HienSauKhiVe({ children, onXong, phu }: { children: ReactNode; onXong: () => void; phu: boolean }) {
  const motion = useMotion();
  const op = useSharedValue(0);
  useEffect(() => {
    let huy = false;
    let a = 0;
    let b = 0;
    a = requestAnimationFrame(() => {
      b = requestAnimationFrame(() => {
        if (huy) return;
        op.value = withTiming(1, motion.timing("standard"), (xong) => {
          if (xong) runOnJS(onXong)();
        });
      });
    });
    return () => {
      huy = true;
      cancelAnimationFrame(a);
      cancelAnimationFrame(b);
    };
    // Once per mount: the fade is the hand-over, not a response to props.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  const kieu = useAnimatedStyle(() => ({ opacity: op.value }));
  return (
    <Animated.View pointerEvents="box-none" style={[phu ? StyleSheet.absoluteFill : null, kieu]}>
      {children}
    </Animated.View>
  );
}

/** Web-only marker; react-native-web renders `dataSet` as `data-*`, native ignores it. */
function Dau({ renderer, children, style }: { renderer: "skia" | "svg"; children: ReactNode; style?: StyleProp<ViewStyle> }) {
  return (
    <View pointerEvents="box-none" style={style} {...({ dataSet: { renderer } } as object)}>
      {children}
    </View>
  );
}

export function KhungSkia<P extends object>({
  skia: VeSkia,
  props,
  svg,
  style,
  testID,
}: {
  skia: ComponentType<P>;
  props: P;
  /** The same art drawn with react-native-svg: the fallback and the loading frame. */
  svg: ReactNode;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}) {
  const trangThai = useTrangThaiSkia();
  // `boSvg`: the Skia canvas has painted and faded in, the SVG may go.
  // `hong`: Skia failed to render; the SVG stays for good.
  const [boSvg, setBoSvg] = useState(false);
  const [hong, setHong] = useState(false);
  useEffect(() => {
    void napSkia();
  }, []);
  const coSkia = trangThai === "san-sang" && !hong;
  return (
    <View pointerEvents="box-none" style={style} testID={testID}>
      {!boSvg || !coSkia ? <Dau renderer="svg">{svg}</Dau> : null}
      {coSkia ? (
        <BatLoiSkia onLoi={() => setHong(true)}>
          <Suspense fallback={null}>
            <HienSauKhiVe onXong={() => setBoSvg(true)} phu={!boSvg}>
              <Dau renderer="skia">
                <VeSkia {...props} />
              </Dau>
            </HienSauKhiVe>
          </Suspense>
        </BatLoiSkia>
      ) : null}
    </View>
  );
}

