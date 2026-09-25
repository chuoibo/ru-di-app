import { useState } from "react";
import { ActivityIndicator, StyleSheet, Text, View, type StyleProp, type ViewStyle } from "react-native";
import Svg, { Path } from "react-native-svg";

import { displayFace, mauSang, phuMau, useRudiTheme } from "../theme";
import { duongVienDau } from "./duong-svg";
import { Grain } from "./Grain";
import { PressScale } from "./PressScale";

export interface StampButtonProps {
  label: string;
  onPress: () => void;
  disabled?: boolean;
  loading?: boolean;
  /** Rotation of a seal pressed by hand: -3 on the cover, -1 on a form, 0 in a table. */
  tilt?: -3 | -2 | -1.5 | -1 | 0 | 1 | 1.5 | 2 | 3;
  /** `lon` is the cover's ask; `vua` is the same seal on a form. */
  size?: "lon" | "vua";
  /**
   * The seal's ink: coral for the ask (default), teal for a decision about
   * money (ADR-0037 D14: «Ghi vào sổ», «Phát đợt thu»). The lettering stays
   * the static dark ink on both: 5.41:1 on coral, 5.33:1 on teal. There is no
   * violet seal -- the dark ink reads 3.34:1 on it.
   */
  tone?: "accent" | "split";
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

const CO = {
  lon: { minHeight: 60, paddingHorizontal: 30, fontSize: 21, lineHeight: 26, radius: 16 },
  vua: { minHeight: 52, paddingHorizontal: 24, fontSize: 17, lineHeight: 22, radius: 14 },
} as const;

/**
 * The ask, as a rubber stamp pressed into the page.
 *
 * Not a pill, and not a bar: the seal is as wide as its words (a stamp is cut
 * to its lettering, a button stretches to its container), it leans the way a
 * hand leaves it, and its rim is drawn as a path whose edge breaks by a dp or
 * so all the way round (`duongVienDau`) instead of a geometric radius. One rim
 * only; the 2026-09-06 review read a second inner edge as three rings around
 * the words. No arrow glyph: the stamp is the verb. Coral ink whose fill is
 * broken by the ink tile, lettering in the display face in static dark ink
 * (`mauSang.ink`, 5.41:1 on coral in either scheme). The same seal, one size
 * down, is the primary action on Login, so the ask has one language.
 */
export function StampButton({ label, onPress, disabled, loading, tilt = 0, size = "lon", tone = "accent", style, testID }: StampButtonProps) {
  const { brand } = useRudiTheme();
  const mauDau = tone === "split" ? brand.teal : brand.coral;
  // Static dark ink: the seal is coral in both schemes, so its lettering never
  // follows the scheme (the dark scheme's light ink on coral read 2.4:1).
  const muc = mauSang.ink;
  const busy = !!loading;
  const co = CO[size];
  const [box, setBox] = useState({ w: 0, h: 0 });
  return (
    <PressScale
      accessibilityLabel={label}
      accessibilityRole="button"
      accessibilityState={{ disabled: !!disabled, busy }}
      disabled={disabled || busy}
      haptic="impact"
      onPress={onPress}
      pressedScale={0.97}
      testID={testID}
      style={[
        styles.seal,
        { opacity: disabled ? 0.55 : 1, ...(tilt === 0 ? {} : { transform: [{ rotate: `${tilt}deg` }] }) },
        style,
      ]}
    >
      <View
        onLayout={(e) => setBox({ w: Math.round(e.nativeEvent.layout.width), h: Math.round(e.nativeEvent.layout.height) })}
        style={[
          styles.body,
          { minHeight: co.minHeight, paddingHorizontal: co.paddingHorizontal, borderRadius: co.radius },
          // Until the first layout the rim has no size; plain coral for that one frame, never a hole.
          box.w === 0 && { backgroundColor: mauDau },
        ]}
      >
        {box.w > 0 ? (
          <Svg height={box.h} pointerEvents="none" style={StyleSheet.absoluteFill} viewBox={`0 0 ${box.w} ${box.h}`} width={box.w}>
            <Path d={duongVienDau(box.w, box.h, co.radius)} fill={mauDau} stroke={phuMau(muc, 0.88)} strokeWidth={2} />
          </Svg>
        ) : null}
        {/* Ink, not paper: the sparse paper tile measured flat on coral (stddev 2); this one lands at ~8 levels at 1x, like the cloth. Clipped a hair inside the rim so it never shows past a broken edge. */}
        <View pointerEvents="none" style={[styles.muc, { borderRadius: co.radius - 2 }]}>
          <Grain material="mucIn" opacity={0.26} />
        </View>
        <View style={styles.row}>
          {busy ? <ActivityIndicator color={muc} /> : null}
          <Text style={[styles.label, { color: muc, fontSize: co.fontSize, lineHeight: co.lineHeight }]}>{label}</Text>
        </View>
      </View>
    </PressScale>
  );
}

const styles = StyleSheet.create({
  seal: { alignSelf: "center" },
  body: { justifyContent: "center", overflow: "hidden" },
  muc: { position: "absolute", left: 3, right: 3, top: 3, bottom: 3, overflow: "hidden" },
  row: { flexDirection: "row", alignItems: "center", justifyContent: "center", gap: 10 },
  label: { fontFamily: displayFace.bold, letterSpacing: 0.4 },
});
