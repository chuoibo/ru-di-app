/**
 * The ground every paper object stands on (ADR-0037 D1, D2): a shape cut to
 * the size the object measured, drawn behind React Native content that never
 * waits for it.
 *
 * The object lays itself out like any view; once it knows its size it draws
 * its outline (`hinh(w, h)` from `art/giay.ts`: teeth, notches, a torn edge,
 * a scalloped rim) in the paper's tone with an edge of `lineStrong` -- not
 * `line`: on the dark ground `card` is 1.06:1 against the page, so the edge is
 * what makes the object an object.
 *
 * The drop shadow of its paper height is an OUTSET `boxShadow` on an empty
 * body the size of the object (`boGoc` rounds it like the shape's corners).
 * Outset shadows are not drawn inside their box, so a notch or the gap between
 * two teeth shows the page, never a slab of paper or of shadow; `chen` pulls
 * the body in where a shape is cut deeper than its box edge.
 */
import { useState, type ReactNode } from "react";
import { StyleSheet, View, type LayoutChangeEvent, type StyleProp, type ViewStyle } from "react-native";
import Svg, { Path } from "react-native-svg";

import type { LopVe } from "../art/net";
import { bongGiay, useRudiTheme, type CaoGiay } from "../theme";
import { mauLop } from "./art/VeLop";

export interface HinhGiay {
  /** The paper (a closed fill). */
  nen: string;
  /** The edge, drawn as a stroke. */
  vien?: string;
  /** Marks printed on the paper: perforations, folds, a pocket. */
  them?: readonly LopVe[];
}

export interface NenGiayProps {
  hinh: (w: number, h: number) => HinhGiay;
  /** Paper height: 1 pasted on the page (default), 2 standing, 3 lifted. */
  cao?: CaoGiay;
  /** How far inside each edge the shadow's body stays. */
  chen?: { tren?: number; duoi?: number; trai?: number; phai?: number };
  /** Corner radius of the shadow's body, to follow the shape's corners. */
  boGoc?: number;
  /** The paper's tone: `card` (default), `paper`, or `trong` (an outline only: a locked stamp). */
  to?: "card" | "paper" | "trong";
  /** The edge drawn dashed (a stamp not yet earned). */
  vienDut?: boolean;
  children?: ReactNode;
  style?: StyleProp<ViewStyle>;
  onLayout?: (e: LayoutChangeEvent) => void;
  testID?: string;
  accessibilityLabel?: string;
}

export function NenGiay({ hinh, cao = 1, chen = {}, boGoc = 0, to = "card", vienDut = false, children, style, onLayout, testID, accessibilityLabel }: NenGiayProps) {
  const { colors, dark } = useRudiTheme();
  const [kt, setKt] = useState({ w: 0, h: 0 });
  const mauGiay = to === "paper" ? colors.paper : to === "trong" ? "none" : colors.card;
  const ve = kt.w > 0 && kt.h > 0 ? hinh(kt.w, kt.h) : null;
  const than = { top: chen.tren ?? 0, bottom: chen.duoi ?? 0, left: chen.trai ?? 0, right: chen.phai ?? 0 };
  return (
    <View
      accessibilityLabel={accessibilityLabel}
      onLayout={(e) => {
        const w = Math.round(e.nativeEvent.layout.width);
        const h = Math.round(e.nativeEvent.layout.height);
        setKt((cu) => (cu.w === w && cu.h === h ? cu : { w, h }));
        onLayout?.(e);
      }}
      style={style}
      testID={testID}
    >
      {to !== "trong" ? <View pointerEvents="none" style={[styles.than, than, { borderRadius: boGoc }, bongGiay(cao, dark)]} /> : null}
      {ve ? (
        <Svg height={kt.h} pointerEvents="none" style={StyleSheet.absoluteFill} viewBox={`0 0 ${kt.w} ${kt.h}`} width={kt.w}>
          <Path d={ve.nen} fill={mauGiay} />
          {ve.them?.map((l, i) =>
            l.net && l.net > 0 ? (
              <Path d={l.d} fill="none" key={i} stroke={mauLop(colors, l.mau)} strokeLinecap="round" strokeLinejoin="round" strokeWidth={l.net} />
            ) : (
              <Path d={l.d} fill={mauLop(colors, l.mau)} key={i} />
            ),
          )}
          {ve.vien ? (
            <Path d={ve.vien} fill="none" stroke={colors.lineStrong} strokeDasharray={vienDut ? [4, 3] : undefined} strokeLinejoin="round" strokeWidth={vienDut ? 1.4 : 1} />
          ) : null}
        </Svg>
      ) : null}
      {children}
    </View>
  );
}

const styles = StyleSheet.create({
  than: { position: "absolute" },
});
