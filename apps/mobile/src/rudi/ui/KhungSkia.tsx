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
 * Each branch marks itself with `data-renderer` on the web (`dataSet`), inner
 * to the branch that actually painted, so an evidence run can prove which one
 * drew (`tools/xem-san-khau.mjs` fails unless it reads `skia`).
 */
import { Component, Suspense, lazy, useEffect, type ComponentType, type ReactNode } from "react";
import { View, type StyleProp, type ViewStyle } from "react-native";

import { napSkia, useTrangThaiSkia } from "./skia/nap-skia";

/** A lazy Skia component that only loads once `napSkia()` says Skia can draw. */
export function luoiSkia<P extends object>(nap: () => Promise<{ default: ComponentType<P> }>): ComponentType<P> {
  return lazy(async () => {
    const coSkia = await napSkia();
    if (!coSkia) throw new Error("skia-khong-ve-duoc");
    return nap();
  });
}

class VeLaiBangSvg extends Component<{ svg: ReactNode; children: ReactNode }, { hong: boolean }> {
  state = { hong: false };

  static getDerivedStateFromError() {
    return { hong: true };
  }

  render() {
    return this.state.hong ? this.props.svg : this.props.children;
  }
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
  useEffect(() => {
    void napSkia();
  }, []);
  const banSvg = (
    <Dau renderer="svg">
      {svg}
    </Dau>
  );
  return (
    <View pointerEvents="box-none" style={style} testID={testID}>
      {trangThai === "san-sang" ? (
        <VeLaiBangSvg svg={banSvg}>
          <Suspense fallback={banSvg}>
            <Dau renderer="skia">
              <VeSkia {...props} />
            </Dau>
          </Suspense>
        </VeLaiBangSvg>
      ) : (
        banSvg
      )}
    </View>
  );
}

