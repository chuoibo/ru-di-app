import { useState } from "react";
import { Image, PixelRatio, StyleSheet, View, type LayoutChangeEvent, type StyleProp, type ViewStyle } from "react-native";

import { chatLieu } from "../textures";
import { luoiChatLieu } from "./luoi-chat-lieu";

export interface GrainProps {
  material: keyof typeof chatLieu;
  /**
   * Cloth 0.3 on the cover and on the dark page ground, paper 0.45 on light
   * pages, ink 0.26 inside a stamp. Measured on the emulator at 1x: 0.11 / 0.07
   * read as flat colour. The tiles are black-and-white noise with a neutral
   * mean, so on a mid-tone ground the token colour underneath still measures
   * the same to within a level -- but NOT on the darkest ground: the cloth tile
   * (grey 127.5, alpha 45/255) at 0.30 lifts `ground` dark #151830 to a measured
   * #1c1f36 (+7 grey levels, L* 9.2 -> 12; finish review 11/09). Contrast on
   * the dark page is therefore ~8% lower than the token table says (`inkFaint`
   * 6.33 -> 5.87, still over 4.5), and `card` on the real ground is 1.06:1, so
   * a dark container keeps its `line` edge -- the fill alone is no longer a step.
   */
  opacity?: number;
  style?: StyleProp<ViewStyle>;
}

/**
 * A material laid over a surface: cloth on the cover, paper on the pages,
 * paper again inside a stamp so its fill is imperfect like ink on fibre.
 *
 * Laid as a grid of plain Images, not one `resizeMode="repeat"` Image. On
 * Android the tiled image is rasterised once into a bitmap of the view's size
 * at request time; on a full-height cover that bitmap came out shorter than
 * the view and the weave stopped a third of the way down (2026-09-05 board:
 * pixel runs uniform below y = 700 while the top read 2.7 levels of grain).
 * Plain tiles never depend on that. One alpha layer for the whole grid; the
 * tile is the PNG at device pixels so nothing is blurred by scaling.
 */
export function Grain({ material, opacity = 0.3, style }: GrainProps) {
  const [box, setBox] = useState({ w: 0, h: 0 });
  const onLayout = (e: LayoutChangeEvent) => {
    const w = Math.round(e.nativeEvent.layout.width);
    const h = Math.round(e.nativeEvent.layout.height);
    if (w !== box.w || h !== box.h) setBox({ w, h });
  };
  const { tile, cols, rows } = luoiChatLieu(box.w, box.h, PixelRatio.get());
  return (
    <View onLayout={onLayout} pointerEvents="none" style={[StyleSheet.absoluteFill, styles.layer, { opacity }, style]}>
      {Array.from({ length: cols * rows }, (_, i) => (
        <Image
          key={i}
          accessibilityElementsHidden
          importantForAccessibility="no"
          resizeMode="stretch"
          source={chatLieu[material]}
          style={{ position: "absolute", left: (i % cols) * tile, top: Math.floor(i / cols) * tile, width: tile, height: tile }}
        />
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  layer: { overflow: "hidden" },
});
