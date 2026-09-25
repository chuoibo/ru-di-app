/**
 * A notebook seen closed on the table (ADR-0037 D1, plan S2): cloth cover in
 * the cover indigo, a darker spine, an elastic band in coral, and one paper
 * label per name pasted on the front. The two-person notebook before both
 * have agreed is exactly this: a book with their two names on it, shut.
 *
 * `mo` swings the cover open round its spine (M6, «bìa sổ mở»): 0 shut, 1
 * open to `BIA_MO.gocToiDa`. Past upright the reader sees the inside of the
 * cover, an endpaper shaded toward the fold, and beside it the first page,
 * ruled like a school notebook. A book given `mo` keeps the room its cover
 * swings through (`choLatBia`), so it never paints over its neighbours.
 * Decoration: the names are text, the book is read as one image.
 */
import { LinearGradient } from "expo-linear-gradient";
import type { ReactNode } from "react";
import { StyleSheet, Text, View, type StyleProp, type ViewStyle } from "react-native";
import Animated, { useAnimatedStyle, type SharedValue } from "react-native-reanimated";

import { BIA_MO, choLatBia } from "../san-khau/dong-hoc";
import { bongGiay, phuMau, typography, useRudiTheme } from "../theme";

/** The ruled page: first rule under a header band, then one every `DONG` dp. */
const DONG = 14;
const DAU_TRANG = 20;

export function SoBia({
  ten,
  nhan,
  mo,
  rong = 132,
  cao: caoDat,
  trang,
  style,
  testID,
}: {
  /** One label per name on the cover. */
  ten: readonly string[];
  /**
   * A label of the caller's own in place of the name labels -- a new group's
   * notebook takes its name typed straight onto the cover. The book is then no
   * longer one image: what the label holds stays reachable on its own.
   */
  nhan?: ReactNode;
  /** The cover's height; a book's proportion (1.32 of the width) by default. */
  cao?: number;
  /** 0 shut .. 1 open; a shared value for the opening moment, else shut. */
  mo?: SharedValue<number>;
  rong?: number;
  /** What lies on the first page, shown as the cover opens. */
  trang?: ReactNode;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}) {
  const { brand, colors, dark } = useRudiTheme();
  const cao = caoDat ?? Math.round(rong * 1.32);
  const cho = mo ? choLatBia(rong, cao) : { trai: 0, doc: 0 };
  const soDong = Math.max(0, Math.floor((cao - DAU_TRANG - 8) / DONG));
  const lat = useAnimatedStyle(() => {
    const m = mo ? mo.value : 0;
    return { transform: [{ perspective: BIA_MO.phoiCanh }, { rotateY: `${-m * BIA_MO.gocToiDa}deg` }] };
  });
  // The inside of the cover: the same swing seen from the other face, hinged
  // on the same spine, turned away (hidden) until the cover passes upright.
  const latTrong = useAnimatedStyle(() => {
    const m = mo ? mo.value : 0;
    return { transform: [{ perspective: BIA_MO.phoiCanh }, { rotateY: `${180 - m * BIA_MO.gocToiDa}deg` }] };
  });
  return (
    <View
      accessibilityLabel={nhan ? undefined : `Sổ của ${ten.join(" và ")}`}
      accessibilityRole={nhan ? undefined : "image"}
      accessible={!nhan}
      style={[{ width: rong + cho.trai, height: cao + 2 * cho.doc, paddingLeft: cho.trai, paddingTop: cho.doc }, style]}
      testID={testID}
    >
      <View style={{ width: rong, height: cao }}>
        {/* The first page, under the cover. */}
        <View style={[StyleSheet.absoluteFill, styles.trang, { backgroundColor: colors.card, borderColor: colors.lineStrong }, bongGiay(1, dark)]}>
          {mo
            ? Array.from({ length: soDong }, (_, i) => (
                <View key={i} pointerEvents="none" style={[styles.dongKe, { top: DAU_TRANG + i * DONG, backgroundColor: colors.line }]} />
              ))
            : null}
          {mo ? <View pointerEvents="none" style={[styles.le, { backgroundColor: phuMau(brand.coral, 0.45) }]} /> : null}
          {trang}
        </View>
        {mo ? (
          <Animated.View style={[styles.biaTrong, { left: -rong, width: rong, backgroundColor: colors.paper, borderColor: colors.lineStrong, transformOrigin: "right center" }, latTrong]}>
            <LinearGradient
              colors={[phuMau(colors.ink, 0), phuMau(colors.ink, dark ? 0.3 : 0.12)]}
              end={{ x: 1, y: 0.5 }}
              pointerEvents="none"
              start={{ x: 0.6, y: 0.5 }}
              style={StyleSheet.absoluteFill}
            />
          </Animated.View>
        ) : null}
        <Animated.View style={[StyleSheet.absoluteFill, styles.bia, { backgroundColor: colors.cover, transformOrigin: "left center" }, lat]}>
          <View style={[styles.gay, { backgroundColor: phuMau(colors.ink, dark ? 0.35 : 0.45) }]} />
          <View style={styles.nhan}>
            {nhan ?? ten.map((t) => (
              <View key={t} style={[styles.the, { backgroundColor: colors.card, borderColor: colors.coverLineStrong }]}>
                <Text numberOfLines={1} style={[typography.label, { color: colors.ink }]}>
                  {t}
                </Text>
              </View>
            ))}
          </View>
          <View style={[styles.thun, { backgroundColor: brand.coral }]} />
        </Animated.View>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  trang: { borderRadius: 4, borderWidth: StyleSheet.hairlineWidth, padding: 12, justifyContent: "center", alignItems: "center", overflow: "hidden" },
  dongKe: { position: "absolute", left: 0, right: 0, height: StyleSheet.hairlineWidth },
  // The margin rule of a school notebook.
  le: { position: "absolute", top: 0, bottom: 0, left: 16, width: 1 },
  biaTrong: {
    position: "absolute",
    top: 0,
    bottom: 0,
    borderWidth: StyleSheet.hairlineWidth,
    borderTopLeftRadius: 6,
    borderBottomLeftRadius: 6,
    backfaceVisibility: "hidden",
    overflow: "hidden",
  },
  bia: { borderTopRightRadius: 6, borderBottomRightRadius: 6, borderTopLeftRadius: 2, borderBottomLeftRadius: 2, backfaceVisibility: "hidden", overflow: "hidden", justifyContent: "center" },
  gay: { position: "absolute", left: 0, top: 0, bottom: 0, width: 12 },
  nhan: { marginLeft: 22, marginRight: 22, gap: 6, alignItems: "stretch" },
  the: { borderWidth: 1, borderRadius: 3, paddingHorizontal: 8, paddingVertical: 4, alignItems: "center" },
  thun: { position: "absolute", right: 14, top: 0, bottom: 0, width: 6 },
});
