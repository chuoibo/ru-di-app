import { Image, type ImageSource } from "expo-image";
import { useEffect, useState, type ReactNode } from "react";
import { StyleSheet, Text, View, type DimensionValue, type StyleProp, type ViewStyle } from "react-native";

import { MOTION_MS } from "../motion";
import { typography, useRudiTheme } from "../theme";
import { cauGhiCong, type Attribution } from "./ghi-cong";
import { useMotion } from "./useMotion";

/** The credit sentence and its type live in `ghi-cong.ts` (pure); re-exported so callers keep one import. */
export { cauGhiCong, type Attribution } from "./ghi-cong";

export interface MediaSlotProps {
  /** An authenticated source from `nguonAnh`, or null when there is no photo. */
  source: ImageSource | null;
  /** Width / height. 16/10 for a place, 1 for a tile, 4/5 for a polaroid. */
  ratio?: number;
  /** Fixed height instead of a ratio, when the parent sets the width. */
  height?: number;
  width?: DimensionValue;
  radius?: number;
  /** What a viewer with a screen reader hears; required, never decorative. */
  alt: string;
  /** Provenance line under a licensed photo. Required whenever `source` is not the group's own. */
  attribution?: Attribution;
  /** Authored artwork for the empty slot (an SVG per category). */
  fallback?: ReactNode;
  /** Content laid over the picture: a tag, a counter, a title on a scrim. */
  overlay?: ReactNode;
  contentFit?: "cover" | "contain";
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

/**
 * The one place a photograph may appear in the shell.
 *
 * Today live screens have no images on the wire, and the rule in DESIGN.md is
 * blunt: a stock photo standing in for a real place is a fabrication. This slot
 * exists so that rule can be kept *and* the layout can already be image-led:
 * the frame is drawn now, the fallback is authored artwork from the visual
 * world, and when M12 delivers licensed photos they drop into the same frame
 * with the author and licence printed beneath -- never a photo without its
 * provenance. Group photos (`nguonAnh`) come with request headers; a URL from
 * anywhere else is refused by that helper before it reaches here.
 */
export function MediaSlot({
  source,
  ratio = 16 / 10,
  height,
  width = "100%",
  radius,
  alt,
  attribution,
  fallback,
  overlay,
  contentFit = "cover",
  style,
  testID,
}: MediaSlotProps) {
  const { colors, radius: r, space } = useRudiTheme();
  const motion = useMotion();
  // The frame's ground is the theme's paper, never a fixed beige: on the dark
  // scheme a fixed light ground read as a slab (review 08/09 vòng 2 §4).
  // A picture that fails to load leaves the frame drawn and empty, which reads
  // as «this place looks like nothing» rather than as a broken address. It
  // stayed invisible for a whole board run: the credit under the frame was
  // correct, every assertion passed, and the two pictures were never there.
  // Saying it out loud costs one line and gives a flow something to assert.
  const [hong, setHong] = useState(false);
  useEffect(() => setHong(false), [source]);
  const frame: ViewStyle = height !== undefined ? { width, height } : { width, aspectRatio: ratio };
  return (
    <View testID={testID} style={style}>
      <View style={[frame, { borderRadius: radius ?? r.small, backgroundColor: colors.card, overflow: "hidden" }]}>
        {source && !hong ? (
          <Image
            accessibilityLabel={alt}
            source={source}
            contentFit={contentFit}
            onError={() => setHong(true)}
            transition={motion.reduced ? 0 : MOTION_MS.standard}
            style={StyleSheet.absoluteFill}
          />
        ) : (
          <View accessible accessibilityLabel={alt} style={[StyleSheet.absoluteFill, styles.center]}>
            {fallback}
          </View>
        )}
        {overlay ? <View style={StyleSheet.absoluteFill} pointerEvents="box-none">{overlay}</View> : null}
      </View>
      {source && hong ? (
        <Text style={[typography.caption, { color: colors.warn, marginTop: space.xs }]}>
          Chưa tải được ảnh
        </Text>
      ) : null}
      {source && attribution ? (
        // Two lines, not one: this credit is the condition on which the picture
        // above it is allowed to be here, so a long author name has to wrap
        // rather than end in an ellipsis.
        <Text
          numberOfLines={2}
          style={[typography.caption, { color: colors.inkFaint, marginTop: space.xs }]}
        >
          {cauGhiCong(attribution)}
        </Text>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  center: { alignItems: "center", justifyContent: "center" },
});
