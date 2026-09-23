import { useEffect, useState } from "react";
import { Pressable, StyleSheet, Text, useWindowDimensions } from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import Animated, {
  Extrapolation,
  interpolate,
  runOnJS,
  useAnimatedStyle,
  useSharedValue,
  withSpring,
  withTiming,
} from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import Svg, { Path } from "react-native-svg";

import { TAB_BAR_HEIGHT } from "../adaptive";
import { so } from "../art/net";
import { typography, useRudiTheme } from "../theme";
import { Nep } from "../ui/art/Nep";
import { useMotion } from "../ui/useMotion";
import { NEP_DIA, NEP_MEP_HEP, NEP_TO_CAO, TO_SAU_LO, ghimVaoRay, rayDoc, slopTrai, tyLeTuY, yTuTyLe } from "./dock-vi-tri";
import { useNep } from "./NepProvider";
import { hienToSau } from "./trang-thai";

/** How long one peeked line stays before it retracts on its own. */
const HE_MS = 4000;
/** Past this, a horizontal drag means «tuck Nếp away» rather than «move it». */
const KEO_AN_DP = 56;
const KEO_AN_TOC = 700;

/** Nếp stands on the slip at the size the old disc carried. */
const NEP_CO = 44;
/**
 * The folded corner. Small enough that the tucked edge (10dp) still shows a
 * sliver of straight paper under the fold, so the fold reads as a corner of
 * something rather than as the whole visible thing.
 */
const GOC = 8;
/** The second slip rides a little higher than Nếp's, so its top edge shows too. */
const TO_SAU_CAO_HON = 6;

/**
 * Nếp on the edge of the screen: a slip of paper tucked into the notebook.
 *
 * ## One object, in the margin
 *
 * Nếp is «mẩu lời hẹn gấp giấy» -- a folded slip with an appointment on it,
 * the one that keeps a seat for you. So Nếp here is not a button floating over
 * the page. It is a slip tucked into the page's right MARGIN, the way a slip
 * marks a place in a real notebook, and every state is the same slip tucked
 * more or less:
 *
 *   - `an`   (the default) tucked; its edge shows inside the margin, nothing
 *            more. Nếp is not drawn: on a money screen this edge is all ADR-0033
 *            §3 allows, «không mặt, không nhân vật».
 *   - `nghi` pulled out, because the person pulled it; Nếp stands on it.
 *   - `he`   pulled further, with the one line written ON the slip.
 *   - `mo`   the panel is open, and it covers the edge anyway.
 *
 * The margin is a measured budget, not a style: text in a conversation ends
 * exactly 16dp from the right edge, so the tucked slip, the second slip behind
 * it, and the tap area together stay inside those 16dp (`dock-vi-tri.ts`).
 * The first cut rested as a 57dp disc and failed exactly there: it cut «20|0»
 * on Explore, and its tap area caught the «Đồng ý» of an invitation.
 *
 * ## Paper, in this system's own grammar
 *
 * `paper` with a `lineStrong` hairline, the edge `ToGiay` uses; no shadow,
 * because DESIGN.md gives a shadow only to a print laid ON the page and this
 * slip is IN the page's edge. The top-left corner is FOLDED, drawn the way
 * `ToGiay` folds a letter: the corner is missing, the slip's edge turns along
 * the diagonal, and the back of the flap lies on the face in `paperShade`, the
 * tone of the fold across Nếp's own body. Not coral: under «Luật Góc Cắt» the
 * coral corner belongs to Nếp itself and to `dan`, and Nếp's own corner is
 * right there on the slip once it is out. The side that runs into the screen
 * edge has no stroke and no radius, because it continues inside the notebook.
 *
 * The slip is drawn in SVG rather than as a bordered View because a fold is a
 * missing corner: a View can only paint an erasing triangle in some guessed
 * ground colour, and this slip floats over cards, photos and chat.
 *
 * ## Something waiting is a second slip, not a dot
 *
 * DESIGN.md forbids Nếp acting as chrome, and a red dot is chrome. When there
 * is work, a second slip in `accentSoft` -- warmer paper, «ấm lên một nấc»
 * (ADR-0033 §7) -- slides out from behind Nếp's once, one beat, and stays.
 * It never shows on a money screen or beside an open sheet (`hienToSau`).
 *
 * ## «Chừa một chỗ cho nhau»
 *
 * When the page lays another sheet over itself -- a tray, a bottom sheet --
 * the slip tucks back into the edge and leaves the room to it, the first scene
 * ever drawn of Nếp, pulling out a chair for someone else. While the sheet is
 * up the edge is only a sign that Nếp is still there: not a button, and not
 * announced.
 */
export function NepDock() {
  const { dock, tyLe, gui, datTyLe } = useNep();
  const { colors, radius } = useRudiTheme();
  const motion = useMotion();
  const insets = useSafeAreaInsets();
  const { height, width } = useWindowDimensions();
  const [kich, datKich] = useState({ w: NEP_DIA, h: NEP_TO_CAO });

  const ray = rayDoc({
    cao: height,
    dinh: insets.top,
    // Reserved even on routes without tabs: sitting a little high costs
    // nothing, sitting on the tab bar costs the create button.
    day: TAB_BAR_HEIGHT + insets.bottom,
  });

  const dangAn = dock.trangThai === "an";
  const dangHe = dock.trangThai === "he";
  const coToSau = hienToSau(dock);
  // How much of the slip is tucked into the edge right now.
  const cai = dangAn ? NEP_DIA - NEP_MEP_HEP : 0;

  const y = useSharedValue(yTuTyLe(tyLe, ray));
  const keoX = useSharedValue(0);
  const caiX = useSharedValue(cai);
  // 0 = still hidden behind Nếp's slip, 1 = out by its full sliver.
  const sauRa = useSharedValue(0);

  // Follow the stored position once the disk has answered, and follow the rail
  // when the window changes (rotation, split screen).
  useEffect(() => {
    y.value = withSpring(yTuTyLe(tyLe, ray), motion.spring.settle);
  }, [tyLe, ray.tren, ray.duoi, y, motion, ray]);

  // The slip sliding into and out of the edge. It decelerates, because a slip
  // pushed into a notebook stops against the spine; Reduce Motion makes it a cut.
  useEffect(() => {
    caiX.value = withTiming(cai, motion.timing("standard", "decelerate"));
  }, [cai, caiX, motion]);

  // The second slip arrives once, in one beat, and then simply stays. Leaving
  // is a cut: work that is done does not need a farewell.
  useEffect(() => {
    sauRa.value = coToSau ? withTiming(1, motion.timing("standard", "decelerate")) : 0;
  }, [coToSau, sauRa, motion]);

  // One peeked line retracts by itself. A line that waited for a tap would
  // become a second permanent element on every screen.
  useEffect(() => {
    if (!dangHe) return;
    const t = setTimeout(() => gui({ kieu: "het-gio-he" }), HE_MS);
    return () => clearTimeout(t);
  }, [dangHe, gui]);

  // Only outward and vertical drags mean anything here. Under gesture
  // navigation the outer ~30dp of the edge belongs to the system's Back swipe,
  // which is an INWARD drag: a tucked slip that needed one to come out would
  // hand the person Back instead. So out is a tap, and the inward half of the
  // drag is clamped away rather than competed for.
  const keo = Gesture.Pan()
    .enabled(!dock.nhuongCho)
    .onUpdate((e) => {
      y.value = ghimVaoRay(yTuTyLe(tyLe, ray) + e.translationY, ray);
      keoX.value = Math.max(0, e.translationX);
    })
    .onEnd((e) => {
      runOnJS(datTyLe)(tyLeTuY(y.value, ray));
      if (e.translationX > KEO_AN_DP || e.velocityX > KEO_AN_TOC) runOnJS(gui)({ kieu: "vuot-ra" });
      keoX.value = withSpring(0, motion.spring.settle);
    });

  const kieuRay = useAnimatedStyle(() => ({ transform: [{ translateY: y.value }] }));
  const kieuTo = useAnimatedStyle(() => ({ transform: [{ translateX: caiX.value + keoX.value }] }));
  // It rises and slides out together, from exactly behind Nếp's slip, so
  // nothing of it shows before the beat.
  const kieuSau = useAnimatedStyle(() => ({
    transform: [{ translateX: (1 - sauRa.value) * TO_SAU_LO }, { translateY: (1 - sauRa.value) * TO_SAU_CAO_HON }],
  }));
  // Nếp comes into view with the slip and leaves with it, rather than popping.
  const kieuNep = useAnimatedStyle(() => ({
    opacity: interpolate(caiX.value, [0, NEP_DIA - NEP_MEP_HEP], [1, 0], Extrapolation.CLAMP),
  }));

  // The panel covers the edge anyway, and a slip sliding under a sheet reads
  // as a bug rather than as depth.
  if (dock.trangThai === "mo") return null;

  const nhan = dangAn
    ? coToSau
      ? "Nếp đang cài trong mép sổ và có tin mới, chạm để kéo ra"
      : "Nếp đang cài trong mép sổ, chạm để kéo ra"
    : dangHe
      ? "Nếp có việc cho bạn, chạm để xem"
      : "Mở Nếp";

  return (
    <Animated.View
      // While another sheet is up the edge is a sign, not a control: it cannot
      // be pressed, dragged or announced, or it would pull Nếp back over the
      // words the person is reading.
      accessibilityElementsHidden={dock.nhuongCho}
      importantForAccessibility={dock.nhuongCho ? "no-hide-descendants" : "auto"}
      pointerEvents={dock.nhuongCho ? "none" : "box-none"}
      style={[styles.lop, { width }, kieuRay]}
    >
      <GestureDetector gesture={keo}>
        <Animated.View style={[styles.cum, kieuTo]}>
          {coToSau ? (
            // The second slip, behind Nếp's. It is never text and never a
            // count; it is one more slip in the notebook.
            <Animated.View
              pointerEvents="none"
              style={[
                styles.toSau,
                {
                  backgroundColor: colors.accentSoft,
                  borderColor: colors.lineStrong,
                  borderTopLeftRadius: radius.small,
                  borderBottomLeftRadius: radius.small,
                },
                kieuSau,
              ]}
              testID="nep-to-sau"
            />
          ) : null}
          <Pressable
            accessibilityLabel={nhan}
            accessibilityRole="button"
            // Tucked, the slip borrows the rest of the margin and not one dp of
            // the page; out, it borrows nothing (`slopTrai`).
            hitSlop={{ top: 12, bottom: 12, left: slopTrai(dangAn), right: 12 }}
            onLayout={(e) => {
              const { width: w, height: h } = e.nativeEvent.layout;
              if (w !== kich.w || h !== kich.h) datKich({ w, h });
            }}
            onPress={() => gui({ kieu: "cham" })}
            style={styles.to}
            testID={dangAn ? "nep-mep" : "nep-dia"}
          >
            {({ pressed }) => (
              <>
                <MatTo
                  bong={colors.paperShade}
                  giay={pressed ? colors.paperShade : colors.paper}
                  h={kich.h}
                  muc={colors.ink}
                  r={radius.small}
                  vien={colors.lineStrong}
                  w={kich.w}
                />
                {dangHe ? (
                  <Text
                    numberOfLines={2}
                    style={[typography.body, styles.dong, { color: colors.ink }]}
                    testID="nep-bong-bong"
                  >
                    Mình có việc này, xem không?
                  </Text>
                ) : null}
                <Animated.View style={[styles.oNep, kieuNep]}>
                  <Nep pose="doi" size={NEP_CO} testID="nep-hinh" />
                </Animated.View>
              </>
            )}
          </Pressable>
        </Animated.View>
      </GestureDetector>
    </Animated.View>
  );
}

/**
 * The face of the slip: paper with its top-left corner folded down.
 *
 * Same order as `ToGiay`'s fold, minus the erasing triangle, because here the
 * corner is simply not part of the outline: fill, outline (open on the right,
 * where the slip runs into the notebook), flap, and the flap's two free edges
 * in ink. Paths use only M/L/C/Z with plain decimals, the grammar Android's
 * PathParser accepts at mount (`art/net.ts`).
 */
function MatTo({ w, h, r, giay, bong, muc, vien }: { w: number; h: number; r: number; giay: string; bong: string; muc: string; vien: string }) {
  // Inset strokes by half their width so a hairline on the outer edge is drawn
  // whole instead of clipped to half by the viewport.
  const o = 0.5;
  const k = 0.5523 * r;
  const vienMo = [
    `M ${so(w)} ${so(o)}`,
    `L ${so(GOC)} ${so(o)}`,
    `L ${so(o)} ${so(GOC)}`,
    `L ${so(o)} ${so(h - r)}`,
    `C ${so(o)} ${so(h - r + k)} ${so(r - k)} ${so(h - o)} ${so(r)} ${so(h - o)}`,
    `L ${so(w)} ${so(h - o)}`,
  ].join(" ");
  const lat = `M ${so(GOC)} ${so(o)} L ${so(GOC)} ${so(GOC)} L ${so(o)} ${so(GOC)} Z`;
  const canhLat = `M ${so(GOC)} ${so(o)} L ${so(GOC)} ${so(GOC)} L ${so(o)} ${so(GOC)}`;
  return (
    <Svg height={h} pointerEvents="none" style={StyleSheet.absoluteFill} width={w}>
      <Path d={`${vienMo} Z`} fill={giay} />
      <Path d={vienMo} fill="none" stroke={vien} strokeWidth={StyleSheet.hairlineWidth} />
      <Path d={lat} fill={bong} />
      <Path d={canhLat} fill="none" stroke={muc} strokeLinejoin="round" strokeWidth={1} />
    </Svg>
  );
}

const styles = StyleSheet.create({
  lop: {
    position: "absolute",
    top: 0,
    left: 0,
    flexDirection: "row",
    justifyContent: "flex-end",
  },
  // Nếp's slip and the slip behind it travel together.
  cum: {
    flexDirection: "row",
    alignItems: "center",
  },
  to: {
    minHeight: NEP_TO_CAO,
    flexDirection: "row",
    alignItems: "center",
  },
  oNep: {
    width: NEP_DIA,
    alignItems: "center",
    justifyContent: "center",
  },
  // Written on the slip, left of Nếp, in the voice of the rest of the app.
  dong: {
    maxWidth: 176,
    paddingLeft: 14,
  },
  toSau: {
    position: "absolute",
    left: -TO_SAU_LO,
    top: -TO_SAU_CAO_HON,
    bottom: TO_SAU_CAO_HON,
    width: NEP_DIA,
    borderTopWidth: StyleSheet.hairlineWidth,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderLeftWidth: StyleSheet.hairlineWidth,
    borderRightWidth: 0,
  },
});
